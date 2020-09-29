// Copyright 2015 The go-PaloAltoAi Authors
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

package paa

import (
	"context"
	"math/big"

	"github.com/PaloAltoAi/go-PaloAltoAi/accounts"
	"github.com/PaloAltoAi/go-PaloAltoAi/common"
	"github.com/PaloAltoAi/go-PaloAltoAi/common/math"
	"github.com/PaloAltoAi/go-PaloAltoAi/core"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/bloombits"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/state"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/types"
	"github.com/PaloAltoAi/go-PaloAltoAi/core/vm"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/downloader"
	"github.com/PaloAltoAi/go-PaloAltoAi/paa/gasprice"
	"github.com/PaloAltoAi/go-PaloAltoAi/paadb"
	"github.com/PaloAltoAi/go-PaloAltoAi/event"
	"github.com/PaloAltoAi/go-PaloAltoAi/params"
	"github.com/PaloAltoAi/go-PaloAltoAi/rpc"
)

// PaaAPIBackend implements paaapi.Backend for full nodes
type PaaAPIBackend struct {
	paa *PaloAltoAi
	gpo *gasprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *PaaAPIBackend) ChainConfig() *params.ChainConfig {
	return b.paa.chainConfig
}

func (b *PaaAPIBackend) CurrentBlock() *types.Block {
	return b.paa.blockchain.CurrentBlock()
}

func (b *PaaAPIBackend) SetHead(number uint64) {
	b.paa.protocolManager.downloader.Cancel()
	b.paa.blockchain.SetHead(number)
}

func (b *PaaAPIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.paa.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.paa.blockchain.CurrentBlock().Header(), nil
	}
	return b.paa.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *PaaAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.paa.blockchain.GetHeaderByHash(hash), nil
}

func (b *PaaAPIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.paa.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.paa.blockchain.CurrentBlock(), nil
	}
	return b.paa.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *PaaAPIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.paa.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.paa.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *PaaAPIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.paa.blockchain.GetBlockByHash(hash), nil
}

func (b *PaaAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.paa.blockchain.GetReceiptsByHash(hash), nil
}

func (b *PaaAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.paa.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *PaaAPIBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.paa.blockchain.GetTdByHash(blockHash)
}

func (b *PaaAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.paa.BlockChain(), nil)
	return vm.NewEVM(context, state, b.paa.chainConfig, *b.paa.blockchain.GetVMConfig()), vmError, nil
}

func (b *PaaAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.paa.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *PaaAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.paa.BlockChain().SubscribeChainEvent(ch)
}

func (b *PaaAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.paa.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *PaaAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.paa.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *PaaAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.paa.BlockChain().SubscribeLogsEvent(ch)
}

func (b *PaaAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.paa.txPool.AddLocal(signedTx)
}

func (b *PaaAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.paa.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *PaaAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.paa.txPool.Get(hash)
}

func (b *PaaAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.paa.txPool.State().GetNonce(addr), nil
}

func (b *PaaAPIBackend) Stats() (pending int, queued int) {
	return b.paa.txPool.Stats()
}

func (b *PaaAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.paa.TxPool().Content()
}

func (b *PaaAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.paa.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *PaaAPIBackend) Downloader() *downloader.Downloader {
	return b.paa.Downloader()
}

func (b *PaaAPIBackend) ProtocolVersion() int {
	return b.paa.PaaVersion()
}

func (b *PaaAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *PaaAPIBackend) ChainDb() paadb.Database {
	return b.paa.ChainDb()
}

func (b *PaaAPIBackend) EventMux() *event.TypeMux {
	return b.paa.EventMux()
}

func (b *PaaAPIBackend) AccountManager() *accounts.Manager {
	return b.paa.AccountManager()
}

func (b *PaaAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.paa.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *PaaAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.paa.bloomRequests)
	}
}
