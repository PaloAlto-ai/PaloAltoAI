// Copyright 2016 The go-PaloAltoAi Authors
// This file is part of the go-PaloAltoAi library.
//
// The go-PaloAltoAi library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-PaloAltoAi library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-PaloAltoAi library. If not, see <http://www.gnu.org/licenses/>.

// Package les implements the Light PaloAltoAi Subprotocol.
package les

import (
	"fmt"
	"sync"
	"time"

	"github.com/PaloAltoAi/go-PaloAltoAi/accounts"
	"github.com/PaloAltoAi/go-PaloAltoAi/common"
	"github.com/PaloAltoAi/go-PaloAltoAi/common/hexutil"
	"github.com/PaloAltoAi/go-PaloAltoAi/consensus"
	"github.com/PaloAltoAi/go-PaloAltoAi/core"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/bloombits"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/rawdb"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/types"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/downloader"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/filters"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/gasprice"
	"github.com/PaloAltoAi/go-PaloAltoAi/event"
	"github.com/PaloAltoAi/go-PaloAltoAi/internal/paaapi"
	"github.com/PaloAltoAi/go-PaloAltoAi/light"
	"github.com/PaloAltoAi/go-PaloAltoAi/log"
	"github.com/PaloAltoAi/go-PaloAltoAi/node"
	"github.com/PaloAltoAi/go-PaloAltoAi/p2p"
	"github.com/PaloAltoAi/go-PaloAltoAi/p2p/discv5"
	"github.com/PaloAltoAi/go-PaloAltoAi/params"
	rpc "github.com/PaloAltoAi/go-PaloAltoAi/rpc"
)

type LightPaloAltoAi struct {
	lesCommons

	odr         *LesOdr
	relay       *LesTxRelay
	chainConfig *params.ChainConfig
	// Channel for shutting down the service
	shutdownChan chan bool

	// Handlers
	peers      *peerSet
	txPool     *light.TxPool
	blockchain *light.LightChain
	serverPool *serverPool
	reqDist    *requestDistributor
	retriever  *retrieveManager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer

	ApiBackend *LesApiBackend

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	networkId     uint64
	netRPCService *paaapi.PublicNetAPI

	wg sync.WaitGroup
}

func New(ctx *node.ServiceContext, config *paa.Config) (*LightPaloAltoAi, error) {
	chainDb, err := paa.CreateDB(ctx, config, "lightchaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.ConstantinopleOverride)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newPeerSet()
	quitSync := make(chan struct{})

	lpaa := &LightPaloAltoAi{
		lesCommons: lesCommons{
			chainDb: chainDb,
			config:  config,
			iConfig: light.DefaultClientIndexerConfig,
		},
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		peers:          peers,
		reqDist:        newRequestDistributor(peers, quitSync),
		accountManager: ctx.AccountManager,
		engine:         paa.CreateConsensusEngine(ctx, chainConfig, &config.Paaash, nil, false, chainDb),
		shutdownChan:   make(chan bool),
		networkId:      config.NetworkId,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   paa.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
	}

	lpaa.relay = NewLesTxRelay(peers, lpaa.reqDist)
	lpaa.serverPool = newServerPool(chainDb, quitSync, &lpaa.wg)
	lpaa.retriever = newRetrieveManager(peers, lpaa.reqDist, lpaa.serverPool)

	lpaa.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, lpaa.retriever)
	lpaa.chtIndexer = light.NewChtIndexer(chainDb, lpaa.odr, params.CHTFrequencyClient, params.HelperTrieConfirmations)
	lpaa.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, lpaa.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency)
	lpaa.odr.SetIndexers(lpaa.chtIndexer, lpaa.bloomTrieIndexer, lpaa.bloomIndexer)

	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if lpaa.blockchain, err = light.NewLightChain(lpaa.odr, lpaa.chainConfig, lpaa.engine); err != nil {
		return nil, err
	}
	// Note: AddChildIndexer starts the update process for the child
	lpaa.bloomIndexer.AddChildIndexer(lpaa.bloomTrieIndexer)
	lpaa.chtIndexer.Start(lpaa.blockchain)
	lpaa.bloomIndexer.Start(lpaa.blockchain)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		lpaa.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	lpaa.txPool = light.NewTxPool(lpaa.chainConfig, lpaa.blockchain, lpaa.relay)
	if lpaa.protocolManager, err = NewProtocolManager(lpaa.chainConfig, light.DefaultClientIndexerConfig, true, config.NetworkId, lpaa.eventMux, lpaa.engine, lpaa.peers, lpaa.blockchain, nil, chainDb, lpaa.odr, lpaa.relay, lpaa.serverPool, quitSync, &lpaa.wg); err != nil {
		return nil, err
	}
	lpaa.ApiBackend = &LesApiBackend{lpaa, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.MinerGasPrice
	}
	lpaa.ApiBackend.gpo = gasprice.NewOracle(lpaa.ApiBackend, gpoParams)
	return lpaa, nil
}

func lesTopic(genesisHash common.Hash, protocolVersion uint) discv5.Topic {
	var name string
	switch protocolVersion {
	case lpv1:
		name = "LES"
	case lpv2:
		name = "LES2"
	default:
		panic(nil)
	}
	return discv5.Topic(name + "@" + common.Bytes2Hex(genesisHash.Bytes()[0:8]))
}

type LightDummyAPI struct{}

// Paaerbase is the address that mining rewards will be send to
func (s *LightDummyAPI) Paaerbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}

// Coinbase is the address that mining rewards will be send to (alias for Paaerbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the PaloAltoAi package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *LightPaloAltoAi) APIs() []rpc.API {
	return append(paaapi.GetAPIs(s.ApiBackend), []rpc.API{
		{
			Namespace: "paa",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "paa",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "paa",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *LightPaloAltoAi) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *LightPaloAltoAi) BlockChain() *light.LightChain      { return s.blockchain }
func (s *LightPaloAltoAi) TxPool() *light.TxPool              { return s.txPool }
func (s *LightPaloAltoAi) Engine() consensus.Engine           { return s.engine }
func (s *LightPaloAltoAi) LesVersion() int                    { return int(ClientProtocolVersions[0]) }
func (s *LightPaloAltoAi) Downloader() *downloader.Downloader { return s.protocolManager.downloader }
func (s *LightPaloAltoAi) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *LightPaloAltoAi) Protocols() []p2p.Protocol {
	return s.makeProtocols(ClientProtocolVersions)
}

// Start implements node.Service, starting all internal goroutines needed by the
// PaloAltoAi protocol implementation.
func (s *LightPaloAltoAi) Start(srvr *p2p.Server) error {
	log.Warn("Light client mode is an experimental feature")
	s.startBloomHandlers(params.BloomBitsBlocksClient)
	s.netRPCService = paaapi.NewPublicNetAPI(srvr, s.networkId)
	// clients are searching for the first advertised protocol in the list
	protocolVersion := AdvertiseProtocolVersions[0]
	s.serverPool.start(srvr, lesTopic(s.blockchain.Genesis().Hash(), protocolVersion))
	s.protocolManager.Start(s.config.LightPeers)
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// PaloAltoAi protocol.
func (s *LightPaloAltoAi) Stop() error {
	s.odr.Stop()
	s.bloomIndexer.Close()
	s.chtIndexer.Close()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	s.txPool.Stop()
	s.engine.Close()

	s.eventMux.Stop()

	time.Sleep(time.Millisecond * 200)
	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}
