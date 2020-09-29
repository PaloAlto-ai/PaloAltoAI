// Copyright 2014 The go-PaloAltoAi Authors
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

// Package paa implements the PaloAltoAi protocol.
package paa

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/PaloAltoAi/go-PaloAltoAi/accounts"
	"github.com/PaloAltoAi/go-PaloAltoAi/common"
	"github.com/PaloAltoAi/go-PaloAltoAi/common/hexutil"
	"github.com/PaloAltoAi/go-PaloAltoAi/consensus"
	"github.com/PaloAltoAi/go-PaloAltoAi/consensus/clique"
	"github.com/PaloAltoAi/go-PaloAltoAi/consensus/paaash"
	"github.com/PaloAltoAi/go-PaloAltoAi/core"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/bloombits"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/rawdb"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/types"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/vm"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/downloader"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/filters"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/gasprice"
	"github.com/PaloAltoAi/go-PaloAltoAi/paadb"
	"github.com/PaloAltoAi/go-PaloAltoAi/event"
	"github.com/PaloAltoAi/go-PaloAltoAi/internal/paaapi"
	"github.com/PaloAltoAi/go-PaloAltoAi/log"
	"github.com/PaloAltoAi/go-PaloAltoAi/miner"
	"github.com/PaloAltoAi/go-PaloAltoAi/node"
	"github.com/PaloAltoAi/go-PaloAltoAi/p2p"
	"github.com/PaloAltoAi/go-PaloAltoAi/params"
	"github.com/PaloAltoAi/go-PaloAltoAi/rlp"
	"github.com/PaloAltoAi/go-PaloAltoAi/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// PaloAltoAi implements the PaloAltoAi full node service.
type PaloAltoAi struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the PaloAltoAi

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb paadb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	APIBackend *PaaAPIBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	paaerbase common.Address

	networkID     uint64
	netRPCService *paaapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and paaerbase)
}

func (s *PaloAltoAi) AddLesServer(ls LesServer) {
	s.lesServer = ls
	ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new PaloAltoAi object (including the
// initialisation of the common PaloAltoAi object)
func New(ctx *node.ServiceContext, config *Config) (*PaloAltoAi, error) {
	// Ensure configuration values are compatible and sane
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run paa.PaloAltoAi in light sync mode, use les.LightPaloAltoAi")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	if config.MinerGasPrice == nil || config.MinerGasPrice.Cmp(common.Big0) <= 0 {
		log.Warn("Sanitizing invalid miner gas price", "provided", config.MinerGasPrice, "updated", DefaultConfig.MinerGasPrice)
		config.MinerGasPrice = new(big.Int).Set(DefaultConfig.MinerGasPrice)
	}
	// Assemble the PaloAltoAi object
	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.ConstantinopleOverride)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	paa := &PaloAltoAi{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, chainConfig, &config.Paaash, config.MinerNotify, config.MinerNoverify, chainDb),
		shutdownChan:   make(chan bool),
		networkID:      config.NetworkId,
		gasPrice:       config.MinerGasPrice,
		paaerbase:      config.Paaerbase,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
	}

	log.Info("Initialising PaloAltoAi protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := rawdb.ReadDatabaseVersion(chainDb)
		if bcVersion != nil && *bcVersion > core.BlockChainVersion {
			return nil, fmt.Errorf("database version is v%d, Gpaa %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
		} else if bcVersion != nil && *bcVersion < core.BlockChainVersion {
			log.Warn("Upgrade blockchain database version", "from", *bcVersion, "to", core.BlockChainVersion)
		}
		rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
	}
	var (
		vmConfig = vm.Config{
			EnablePreimageRecording: config.EnablePreimageRecording,
			EWASMInterpreter:        config.EWASMInterpreter,
			EVMInterpreter:          config.EVMInterpreter,
		}
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieCleanLimit: config.TrieCleanCache, TrieDirtyLimit: config.TrieDirtyCache, TrieTimeLimit: config.TrieTimeout}
	)
	paa.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, paa.chainConfig, paa.engine, vmConfig, paa.shouldPreserve)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		paa.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	paa.bloomIndexer.Start(paa.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	paa.txPool = core.NewTxPool(config.TxPool, paa.chainConfig, paa.blockchain)

	if paa.protocolManager, err = NewProtocolManager(paa.chainConfig, config.SyncMode, config.NetworkId, paa.eventMux, paa.txPool, paa.engine, paa.blockchain, chainDb, config.Whitelist); err != nil {
		return nil, err
	}

	paa.miner = miner.New(paa, paa.chainConfig, paa.EventMux(), paa.engine, config.MinerRecommit, config.MinerGasFloor, config.MinerGasCeil, paa.isLocalBlock)
	paa.miner.SetExtra(makeExtraData(config.MinerExtraData))

	paa.APIBackend = &PaaAPIBackend{paa, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.MinerGasPrice
	}
	paa.APIBackend.gpo = gasprice.NewOracle(paa.APIBackend, gpoParams)

	return paa, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"gpaa",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (paadb.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	if db, ok := db.(*paadb.LDBDatabase); ok {
		db.Meter("paa/db/chaindata/")
	}
	return db, nil
}

// CreateConsensusEngine creates the required type of consensus engine instance for an PaloAltoAi service
func CreateConsensusEngine(ctx *node.ServiceContext, chainConfig *params.ChainConfig, config *paaash.Config, notify []string, noverify bool, db paadb.Database) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	// Otherwise assume proof-of-work
	switch config.PowMode {
	case paaash.ModeFake:
		log.Warn("Paaash used in fake mode")
		return paaash.NewFaker()
	case paaash.ModeTest:
		log.Warn("Paaash used in test mode")
		return paaash.NewTester(nil, noverify)
	case paaash.ModeShared:
		log.Warn("Paaash used in shared mode")
		return paaash.NewShared()
	default:
		engine := paaash.New(paaash.Config{
			CacheDir:       ctx.ResolvePath(config.CacheDir),
			CachesInMem:    config.CachesInMem,
			CachesOnDisk:   config.CachesOnDisk,
			DatasetDir:     config.DatasetDir,
			DatasetsInMem:  config.DatasetsInMem,
			DatasetsOnDisk: config.DatasetsOnDisk,
		}, notify, noverify)
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs return the collection of RPC services the PaloAltoAi package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *PaloAltoAi) APIs() []rpc.API {
	apis := paaapi.GetAPIs(s.APIBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "paa",
			Version:   "1.0",
			Service:   NewPublicPaloAltoAiAPI(s),
			Public:    true,
		}, {
			Namespace: "paa",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "paa",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "paa",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.APIBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s.chainConfig, s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *PaloAltoAi) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *PaloAltoAi) Paaerbase() (eb common.Address, err error) {
	s.lock.RLock()
	paaerbase := s.paaerbase
	s.lock.RUnlock()

	if paaerbase != (common.Address{}) {
		return paaerbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			paaerbase := accounts[0].Address

			s.lock.Lock()
			s.paaerbase = paaerbase
			s.lock.Unlock()

			log.Info("Paaerbase automatically configured", "address", paaerbase)
			return paaerbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("paaerbase must be explicitly specified")
}

// isLocalBlock checks whpaaer the specified block is mined
// by local miner accounts.
//
// We regard two types of accounts as local miner account: paaerbase
// and accounts specified via `txpool.locals` flag.
func (s *PaloAltoAi) isLocalBlock(block *types.Block) bool {
	author, err := s.engine.Author(block.Header())
	if err != nil {
		log.Warn("Failed to retrieve block author", "number", block.NumberU64(), "hash", block.Hash(), "err", err)
		return false
	}
	// Check whpaaer the given address is paaerbase.
	s.lock.RLock()
	paaerbase := s.paaerbase
	s.lock.RUnlock()
	if author == paaerbase {
		return true
	}
	// Check whpaaer the given address is specified by `txpool.local`
	// CLI flag.
	for _, account := range s.config.TxPool.Locals {
		if account == author {
			return true
		}
	}
	return false
}

// shouldPreserve checks whpaaer we should preserve the given block
// during the chain reorg depending on whpaaer the author of block
// is a local account.
func (s *PaloAltoAi) shouldPreserve(block *types.Block) bool {
	// The reason we need to disable the self-reorg preserving for clique
	// is it can be probable to introduce a deadlock.
	//
	// e.g. If there are 7 available signers
	//
	// r1   A
	// r2     B
	// r3       C
	// r4         D
	// r5   A      [X] F G
	// r6    [X]
	//
	// In the round5, the inturn signer E is offline, so the worst case
	// is A, F and G sign the block of round5 and reject the block of opponents
	// and in the round6, the last available signer B is offline, the whole
	// network is stuck.
	if _, ok := s.engine.(*clique.Clique); ok {
		return false
	}
	return s.isLocalBlock(block)
}

// SetPaaerbase sets the mining reward address.
func (s *PaloAltoAi) SetPaaerbase(paaerbase common.Address) {
	s.lock.Lock()
	s.paaerbase = paaerbase
	s.lock.Unlock()

	s.miner.SetPaaerbase(paaerbase)
}

// StartMining starts the miner with the given number of CPU threads. If mining
// is already running, this method adjust the number of threads allowed to use
// and updates the minimum price required by the transaction pool.
func (s *PaloAltoAi) StartMining(threads int) error {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		log.Info("Updated mining threads", "threads", threads)
		if threads == 0 {
			threads = -1 // Disable the miner from within
		}
		th.SetThreads(threads)
	}
	// If the miner was not running, initialize it
	if !s.IsMining() {
		// Propagate the initial price point to the transaction pool
		s.lock.RLock()
		price := s.gasPrice
		s.lock.RUnlock()
		s.txPool.SetGasPrice(price)

		// Configure the local mining address
		eb, err := s.Paaerbase()
		if err != nil {
			log.Error("Cannot start mining without paaerbase", "err", err)
			return fmt.Errorf("paaerbase missing: %v", err)
		}
		if clique, ok := s.engine.(*clique.Clique); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Paaerbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			clique.Authorize(eb, wallet.SignHash)
		}
		// If mining is started, we can disable the transaction rejection mechanism
		// introduced to speed sync times.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)

		go s.miner.Start(eb)
	}
	return nil
}

// StopMining terminates the miner, both at the consensus engine level as well as
// at the block creation level.
func (s *PaloAltoAi) StopMining() {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		th.SetThreads(-1)
	}
	// Stop the block creating itself
	s.miner.Stop()
}

func (s *PaloAltoAi) IsMining() bool      { return s.miner.Mining() }
func (s *PaloAltoAi) Miner() *miner.Miner { return s.miner }

func (s *PaloAltoAi) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *PaloAltoAi) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *PaloAltoAi) TxPool() *core.TxPool               { return s.txPool }
func (s *PaloAltoAi) EventMux() *event.TypeMux           { return s.eventMux }
func (s *PaloAltoAi) Engine() consensus.Engine           { return s.engine }
func (s *PaloAltoAi) ChainDb() paadb.Database            { return s.chainDb }
func (s *PaloAltoAi) IsListening() bool                  { return true } // Always listening
func (s *PaloAltoAi) PaaVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *PaloAltoAi) NetVersion() uint64                 { return s.networkID }
func (s *PaloAltoAi) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *PaloAltoAi) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// PaloAltoAi protocol implementation.
func (s *PaloAltoAi) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	s.startBloomHandlers(params.BloomBitsBlocks)

	// Start the RPC service
	s.netRPCService = paaapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// PaloAltoAi protocol.
func (s *PaloAltoAi) Stop() error {
	s.bloomIndexer.Close()
	s.blockchain.Stop()
	s.engine.Close()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)
	return nil
}
