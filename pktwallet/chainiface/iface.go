package chainiface

import (
	"time"

	"github.com/pkt-cash/pktd/btcutil"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/chaincfg/chainhash"
	"github.com/pkt-cash/pktd/pktwallet/waddrmgr"
	"github.com/pkt-cash/pktd/pktwallet/wtxmgr"
	"github.com/pkt-cash/pktd/wire"
)

// FilterBlocksRequest specifies a range of blocks and the set of
// internal and external addresses of interest, indexed by corresponding
// scoped-index of the child address. A global set of watched outpoints
// is also included to monitor for spends.
type FilterBlocksRequest struct {
	Blocks           []wtxmgr.BlockMeta
	ExternalAddrs    map[waddrmgr.ScopedIndex]btcutil.Address
	InternalAddrs    map[waddrmgr.ScopedIndex]btcutil.Address
	ImportedAddrs    []btcutil.Address
	WatchedOutPoints map[wire.OutPoint]btcutil.Address
}

// FilterBlocksResponse reports the set of all internal and external
// addresses found in response to a FilterBlockRequest, any outpoints
// found that correspond to those addresses, as well as the relevant
// transactions that can modify the wallet's balance. The index of the
// block within the FilterBlocksRequest is returned, such that the
// caller can reinitiate a request for the subsequent block after
// updating the addresses of interest.
type FilterBlocksResponse struct {
	BatchIndex         uint32
	BlockMeta          wtxmgr.BlockMeta
	FoundExternalAddrs map[waddrmgr.KeyScope]map[uint32]struct{}
	FoundInternalAddrs map[waddrmgr.KeyScope]map[uint32]struct{}
	FoundOutPoints     map[wire.OutPoint]btcutil.Address
	RelevantTxns       []*wire.MsgTx
}

// Interface allows more than one backing blockchain source, such as a
// pktd RPC chain server, or an SPV library, as long as we write a driver for
// it.
type Interface interface {
	Start() er.R
	Stop()
	BestBlock() (*waddrmgr.BlockStamp, er.R)
	GetBlock(*chainhash.Hash) (*wire.MsgBlock, er.R)
	GetBlockHash(int64) (*chainhash.Hash, er.R)
	GetBlockHeader(*chainhash.Hash) (*wire.BlockHeader, er.R)
	IsCurrent() bool
	FilterBlocks(*FilterBlocksRequest) (*FilterBlocksResponse, er.R)
	SendRawTransaction(*wire.MsgTx, bool) (*chainhash.Hash, er.R)
	BackEnd() string
	GetBlockHeight(hash *chainhash.Hash) (int32, er.R)
}

type Mock struct {
}

var _ Interface = (*Mock)(nil)

func (m *Mock) Start() er.R {
	return nil
}

func (m *Mock) Stop() {
}

func (m *Mock) WaitForShutdown() {}

func (m *Mock) GetBlock(*chainhash.Hash) (*wire.MsgBlock, er.R) {
	return nil, nil
}

func (m *Mock) GetBlockHash(int64) (*chainhash.Hash, er.R) {
	return nil, nil
}

func (m *Mock) GetBlockHeader(*chainhash.Hash) (*wire.BlockHeader,
	er.R) {
	return nil, nil
}

func (m *Mock) IsCurrent() bool {
	return false
}

func (m *Mock) FilterBlocks(*FilterBlocksRequest) (
	*FilterBlocksResponse, er.R) {
	return nil, nil
}

func (m *Mock) BestBlock() (*waddrmgr.BlockStamp, er.R) {
	return &waddrmgr.BlockStamp{
		Height:    500000,
		Hash:      chainhash.Hash{},
		Timestamp: time.Unix(1234, 0),
	}, nil
}

func (m *Mock) SendRawTransaction(*wire.MsgTx, bool) (
	*chainhash.Hash, er.R) {
	return nil, nil
}

func (m *Mock) Rescan(*chainhash.Hash, []btcutil.Address,
	map[wire.OutPoint]btcutil.Address) er.R {
	return nil
}

func (m *Mock) NotifyReceived([]btcutil.Address) er.R {
	return nil
}

func (m *Mock) NotifyBlocks() er.R {
	return nil
}

func (m *Mock) Notifications() <-chan interface{} {
	return nil
}

func (m *Mock) BackEnd() string {
	return "mock"
}

func (m *Mock) GetBlockHeight(hash *chainhash.Hash) (int32, er.R) {
	return -1, er.New("unsupported in mock")
}
