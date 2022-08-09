package walletrpc

import (
	"bytes"
	"context"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkt-cash/pktd/btcutil"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/psbt"
	"github.com/pkt-cash/pktd/chaincfg/chainhash"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/signrpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/walletrpc_pb"
	"github.com/pkt-cash/pktd/lnd/input"
	"github.com/pkt-cash/pktd/lnd/keychain"
	"github.com/pkt-cash/pktd/lnd/labels"
	"github.com/pkt-cash/pktd/lnd/lnrpc"
	"github.com/pkt-cash/pktd/lnd/lnwallet"
	"github.com/pkt-cash/pktd/lnd/lnwallet/chainfee"
	"github.com/pkt-cash/pktd/lnd/sweep"
	"github.com/pkt-cash/pktd/pktlog/log"
	"github.com/pkt-cash/pktd/pktwallet/wtxmgr"
	"github.com/pkt-cash/pktd/txscript"
	"github.com/pkt-cash/pktd/txscript/params"
	"github.com/pkt-cash/pktd/wire"
	"google.golang.org/grpc"
)

// subServerName is the name of the sub rpc server. We'll use this name
// to register ourselves, and we also require that the main
// SubServerConfigDispatcher instance recognize as the name of our
const subServerName = "WalletKitRPC"

// LndInternalLockID is the binary representation of the SHA256 hash of
// the string "lnd-internal-lock-id" and is used for UTXO lock leases to
// identify that we ourselves are locking an UTXO, for example when
// giving out a funded PSBT. The ID corresponds to the hex value of
// ede19a92ed321a4705f8a1cccc1d4f6182545d4bb4fae08bd5937831b7e38f98.
var LndInternalLockID = wtxmgr.LockID{
	0xed, 0xe1, 0x9a, 0x92, 0xed, 0x32, 0x1a, 0x47,
	0x05, 0xf8, 0xa1, 0xcc, 0xcc, 0x1d, 0x4f, 0x61,
	0x82, 0x54, 0x5d, 0x4b, 0xb4, 0xfa, 0xe0, 0x8b,
	0xd5, 0x93, 0x78, 0x31, 0xb7, 0xe3, 0x8f, 0x98,
}

var Err = er.NewErrorType("walletrpc")

// ErrZeroLabel is returned when an attempt is made to label a transaction with
// an empty label.
var ErrZeroLabel = Err.CodeWithDetail("ErrZeroLabel", "cannot label transaction with empty label")

// WalletKit is a sub-RPC server that exposes a tool kit which allows clients
// to execute common wallet operations. This includes requesting new addresses,
// keys (for contracts!), and publishing transactions.
type WalletKit struct {
	cfg *Config
	walletrpc_pb.UnimplementedWalletKitServer
}

// A compile time check to ensure that WalletKit fully implements the
// WalletKitServer gRPC service.
var _ walletrpc_pb.WalletKitServer = (*WalletKit)(nil)

// New creates a new instance of the WalletKit sub-RPC server.
func New(cfg *Config) (*WalletKit, er.R) {
	walletKit := &WalletKit{
		cfg: cfg,
	}
	return walletKit, nil
}

// Start launches any helper goroutines required for the sub-server to function.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (w *WalletKit) Start() er.R {
	return nil
}

// Stop signals any active goroutines for a graceful closure.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (w *WalletKit) Stop() er.R {
	return nil
}

// Name returns a unique string representation of the sub-server. This can be
// used to identify the sub-server and also de-duplicate them.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (w *WalletKit) Name() string {
	return subServerName
}

// RegisterWithRootServer will be called by the root gRPC server to direct a
// sub RPC server to register itself with the main gRPC root server. Until this
// is called, each sub-server won't be able to have requests routed towards it.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (w *WalletKit) RegisterWithRootServer(grpcServer *grpc.Server) er.R {
	// We make sure that we register it with the main gRPC server to ensure
	// all our methods are routed properly.
	walletrpc_pb.RegisterWalletKitServer(grpcServer, w)

	log.Debugf("WalletKit RPC server successfully registered with " +
		"root gRPC server")

	return nil
}

// RegisterWithRestServer will be called by the root REST mux to direct a sub
// RPC server to register itself with the main REST mux server. Until this is
// called, each sub-server won't be able to have requests routed towards it.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (w *WalletKit) RegisterWithRestServer(ctx context.Context,
	mux *runtime.ServeMux, dest string, opts []grpc.DialOption) er.R {

	// We make sure that we register it with the main REST server to ensure
	// all our methods are routed properly.
	// err := RegisterWalletKitHandlerFromEndpoint(ctx, mux, dest, opts)
	// if err != nil {
	// 	log.Errorf("Could not register WalletKit REST server "+
	// 		"with root REST server: %v", err)
	// 	return er.E(err)
	// }

	log.Debugf("WalletKit REST server successfully registered with " +
		"root REST server")
	return nil
}

// ListUnspent returns useful information about each unspent output owned by the
// wallet, as reported by the underlying `ListUnspentWitness`; the information
// returned is: outpoint, amount in satoshis, address, address type,
// scriptPubKey in hex and number of confirmations.  The result is filtered to
// contain outputs whose number of confirmations is between a
// minimum and maximum number of confirmations specified by the user, with 0
// meaning unconfirmed.
func (w *WalletKit) ListUnspent(ctx context.Context,
	in *walletrpc_pb.ListUnspentRequest) (*walletrpc_pb.ListUnspentResponse, error) {
	out, err := w.ListUnspent0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) ListUnspent0(ctx context.Context,
	req *walletrpc_pb.ListUnspentRequest) (*walletrpc_pb.ListUnspentResponse, er.R) {

	// Validate the confirmation arguments.
	minConfs, maxConfs, err := lnrpc.ParseConfs(req.MinConfs, req.MaxConfs)
	if err != nil {
		return nil, err
	}

	// With our arguments validated, we'll query the internal wallet for
	// the set of UTXOs that match our query.
	//
	// We'll acquire the global coin selection lock to ensure there aren't
	// any other concurrent processes attempting to lock any UTXOs which may
	// be shown available to us.
	var utxos []*lnwallet.Utxo
	err = w.cfg.CoinSelectionLocker.WithCoinSelectLock(func() er.R {
		utxos, err = w.cfg.Wallet.ListUnspentWitness(minConfs, maxConfs)
		return err
	})
	if err != nil {
		return nil, err
	}

	rpcUtxos, err := lnrpc.MarshalUtxos(utxos, w.cfg.ChainParams)
	if err != nil {
		return nil, err
	}

	return &walletrpc_pb.ListUnspentResponse{
		Utxos: rpcUtxos,
	}, nil
}

// LeaseOutput locks an output to the given ID, preventing it from being
// available for any future coin selection attempts. The absolute time of the
// lock's expiration is returned. The expiration of the lock can be extended by
// successive invocations of this call. Outputs can be unlocked before their
// expiration through `ReleaseOutput`.
//
// If the output is not known, wtxmgr.ErrUnknownOutput is returned. If the
// output has already been locked to a different ID, then
// wtxmgr.ErrOutputAlreadyLocked is returned.
func (w *WalletKit) LeaseOutput(ctx context.Context,
	in *walletrpc_pb.LeaseOutputRequest) (*walletrpc_pb.LeaseOutputResponse, error) {
	out, err := w.LeaseOutput0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) LeaseOutput0(ctx context.Context,
	req *walletrpc_pb.LeaseOutputRequest) (*walletrpc_pb.LeaseOutputResponse, er.R) {

	if len(req.Id) != 32 {
		return nil, er.New("id must be 32 random bytes")
	}
	var lockID wtxmgr.LockID
	copy(lockID[:], req.Id)

	// Don't allow ID's of 32 bytes, but all zeros.
	if lockID == (wtxmgr.LockID{}) {
		return nil, er.New("id must be 32 random bytes")
	}

	// Don't allow our internal ID to be used externally for locking. Only
	// unlocking is allowed.
	if lockID == LndInternalLockID {
		return nil, er.New("reserved id cannot be used")
	}

	op, err := unmarshallOutPoint(req.Outpoint)
	if err != nil {
		return nil, err
	}

	// Acquire the global coin selection lock to ensure there aren't any
	// other concurrent processes attempting to lease the same UTXO.
	var expiration time.Time
	err = w.cfg.CoinSelectionLocker.WithCoinSelectLock(func() er.R {
		expiration, err = w.cfg.Wallet.LeaseOutput(lockID, *op)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &walletrpc_pb.LeaseOutputResponse{
		Expiration: uint64(expiration.Unix()),
	}, nil
}

// ReleaseOutput unlocks an output, allowing it to be available for coin
// selection if it remains unspent. The ID should match the one used to
// originally lock the output.
func (w *WalletKit) ReleaseOutput(ctx context.Context,
	in *walletrpc_pb.ReleaseOutputRequest) (*walletrpc_pb.ReleaseOutputResponse, error) {
	out, err := w.ReleaseOutput0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) ReleaseOutput0(ctx context.Context,
	req *walletrpc_pb.ReleaseOutputRequest) (*walletrpc_pb.ReleaseOutputResponse, er.R) {

	if len(req.Id) != 32 {
		return nil, er.New("id must be 32 random bytes")
	}
	var lockID wtxmgr.LockID
	copy(lockID[:], req.Id)

	op, err := unmarshallOutPoint(req.Outpoint)
	if err != nil {
		return nil, err
	}

	// Acquire the global coin selection lock to maintain consistency as
	// it's acquired when we initially leased the output.
	err = w.cfg.CoinSelectionLocker.WithCoinSelectLock(func() er.R {
		return w.cfg.Wallet.ReleaseOutput(lockID, *op)
	})
	if err != nil {
		return nil, err
	}

	return &walletrpc_pb.ReleaseOutputResponse{}, nil
}

// DeriveNextKey attempts to derive the *next* key within the key family
// (account in BIP43) specified. This method should return the next external
// child within this branch.
func (w *WalletKit) DeriveNextKey(ctx context.Context,
	in *walletrpc_pb.KeyReq) (*signrpc_pb.KeyDescriptor, error) {
	out, err := w.DeriveNextKey0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) DeriveNextKey0(ctx context.Context,
	req *walletrpc_pb.KeyReq) (*signrpc_pb.KeyDescriptor, er.R) {

	nextKeyDesc, err := w.cfg.KeyRing.DeriveNextKey(
		keychain.KeyFamily(req.KeyFamily),
	)
	if err != nil {
		return nil, err
	}

	return &signrpc_pb.KeyDescriptor{
		KeyLoc: &signrpc_pb.KeyLocator{
			KeyFamily: int32(nextKeyDesc.Family),
			KeyIndex:  int32(nextKeyDesc.Index),
		},
		RawKeyBytes: nextKeyDesc.PubKey.SerializeCompressed(),
	}, nil
}

// DeriveKey attempts to derive an arbitrary key specified by the passed
// KeyLocator.
func (w *WalletKit) DeriveKey(ctx context.Context,
	in *signrpc_pb.KeyLocator) (*signrpc_pb.KeyDescriptor, error) {
	out, err := w.DeriveKey0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) DeriveKey0(ctx context.Context,
	req *signrpc_pb.KeyLocator) (*signrpc_pb.KeyDescriptor, er.R) {

	keyDesc, err := w.cfg.KeyRing.DeriveKey(keychain.KeyLocator{
		Family: keychain.KeyFamily(req.KeyFamily),
		Index:  uint32(req.KeyIndex),
	})
	if err != nil {
		return nil, err
	}

	return &signrpc_pb.KeyDescriptor{
		KeyLoc: &signrpc_pb.KeyLocator{
			KeyFamily: int32(keyDesc.Family),
			KeyIndex:  int32(keyDesc.Index),
		},
		RawKeyBytes: keyDesc.PubKey.SerializeCompressed(),
	}, nil
}

// NextAddr returns the next unused address within the wallet.
func (w *WalletKit) NextAddr(ctx context.Context,
	in *walletrpc_pb.AddrRequest) (*walletrpc_pb.AddrResponse, error) {
	out, err := w.NextAddr0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) NextAddr0(ctx context.Context,
	req *walletrpc_pb.AddrRequest) (*walletrpc_pb.AddrResponse, er.R) {

	addr, err := w.cfg.Wallet.NewAddress(lnwallet.WitnessPubKey, false)
	if err != nil {
		return nil, err
	}

	return &walletrpc_pb.AddrResponse{
		Addr: addr.String(),
	}, nil
}

// Attempts to publish the passed transaction to the network. Once this returns
// without an error, the wallet will continually attempt to re-broadcast the
// transaction on start up, until it enters the chain.
func (w *WalletKit) PublishTransaction(ctx context.Context,
	in *walletrpc_pb.Transaction) (*walletrpc_pb.PublishResponse, error) {
	out, err := w.PublishTransaction0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) PublishTransaction0(ctx context.Context,
	req *walletrpc_pb.Transaction) (*walletrpc_pb.PublishResponse, er.R) {

	switch {
	// If the client doesn't specify a transaction, then there's nothing to
	// publish.
	case len(req.TxHex) == 0:
		return nil, er.Errorf("must provide a transaction to " +
			"publish")
	}

	tx := &wire.MsgTx{}
	txReader := bytes.NewReader(req.TxHex)
	if err := tx.Deserialize(txReader); err != nil {
		return nil, err
	}

	label, err := labels.ValidateAPI(req.Label)
	if err != nil {
		return nil, err
	}

	err = w.cfg.Wallet.PublishTransaction(tx, label)
	if err != nil {
		return nil, err
	}

	return &walletrpc_pb.PublishResponse{}, nil
}

// SendOutputs is similar to the existing sendmany call in Bitcoind, and allows
// the caller to create a transaction that sends to several outputs at once.
// This is ideal when wanting to batch create a set of transactions.
func (w *WalletKit) SendOutputs(ctx context.Context,
	in *walletrpc_pb.SendOutputsRequest) (*walletrpc_pb.SendOutputsResponse, error) {
	out, err := w.SendOutputs0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) SendOutputs0(ctx context.Context,
	req *walletrpc_pb.SendOutputsRequest) (*walletrpc_pb.SendOutputsResponse, er.R) {

	switch {
	// If the client didn't specify any outputs to create, then  we can't
	// proceed .
	case len(req.Outputs) == 0:
		return nil, er.Errorf("must specify at least one output " +
			"to create")
	}

	// Before we can request this transaction to be created, we'll need to
	// amp the protos back into the format that the internal wallet will
	// recognize.
	outputsToCreate := make([]*wire.TxOut, 0, len(req.Outputs))
	for _, output := range req.Outputs {
		outputsToCreate = append(outputsToCreate, &wire.TxOut{
			Value:    output.Value,
			PkScript: output.PkScript,
		})
	}

	// Then, we'll extract the minimum number of confirmations that each
	// output we use to fund the transaction should satisfy.
	minConfs, err := lnrpc.ExtractMinConfs(req.MinConfs, req.SpendUnconfirmed)
	if err != nil {
		return nil, err
	}

	label, err := labels.ValidateAPI(req.Label)
	if err != nil {
		return nil, err
	}

	// Now that we have the outputs mapped, we can request that the wallet
	// attempt to create this transaction.
	tx, err := w.cfg.Wallet.SendOutputs(
		outputsToCreate, chainfee.SatPerKWeight(req.SatPerKw), minConfs, label,
	)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	if err := tx.Serialize(&b); err != nil {
		return nil, err
	}

	return &walletrpc_pb.SendOutputsResponse{
		RawTx: b.Bytes(),
	}, nil
}

// EstimateFee attempts to query the internal fee estimator of the wallet to
// determine the fee (in sat/kw) to attach to a transaction in order to achieve
// the confirmation target.
func (w *WalletKit) EstimateFee(ctx context.Context,
	in *walletrpc_pb.EstimateFeeRequest) (*walletrpc_pb.EstimateFeeResponse, error) {
	out, err := w.EstimateFee0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) EstimateFee0(ctx context.Context,
	req *walletrpc_pb.EstimateFeeRequest) (*walletrpc_pb.EstimateFeeResponse, er.R) {

	switch {
	// A confirmation target of zero doesn't make any sense. Similarly, we
	// reject confirmation targets of 1 as they're unreasonable.
	case req.ConfTarget == 0 || req.ConfTarget == 1:
		return nil, er.Errorf("confirmation target must be greater " +
			"than 1")
	}

	satPerKw, err := w.cfg.FeeEstimator.EstimateFeePerKW(
		uint32(req.ConfTarget),
	)
	if err != nil {
		return nil, err
	}

	return &walletrpc_pb.EstimateFeeResponse{
		SatPerKw: int64(satPerKw),
	}, nil
}

// PendingSweeps returns lists of on-chain outputs that lnd is currently
// attempting to sweep within its central batching engine. Outputs with similar
// fee rates are batched together in order to sweep them within a single
// transaction. The fee rate of each sweeping transaction is determined by
// taking the average fee rate of all the outputs it's trying to sweep.
func (w *WalletKit) PendingSweeps(ctx context.Context,
	in *walletrpc_pb.PendingSweepsRequest) (*walletrpc_pb.PendingSweepsResponse, error) {
	out, err := w.PendingSweeps0(ctx, in)
	return out, er.Native(err)
}
func (w *WalletKit) PendingSweeps0(ctx context.Context,
	in *walletrpc_pb.PendingSweepsRequest) (*walletrpc_pb.PendingSweepsResponse, er.R) {

	// Retrieve all of the outputs the UtxoSweeper is currently trying to
	// sweep.
	pendingInputs, err := w.cfg.Sweeper.PendingInputs()
	if err != nil {
		return nil, err
	}

	// Convert them into their respective RPC format.
	rpcPendingSweeps := make([]*walletrpc_pb.PendingSweep, 0, len(pendingInputs))
	for _, pendingInput := range pendingInputs {
		var witnessType walletrpc_pb.WitnessType
		switch pendingInput.WitnessType {
		case input.CommitmentTimeLock:
			witnessType = walletrpc_pb.WitnessType_COMMITMENT_TIME_LOCK
		case input.CommitmentNoDelay:
			witnessType = walletrpc_pb.WitnessType_COMMITMENT_NO_DELAY
		case input.CommitmentRevoke:
			witnessType = walletrpc_pb.WitnessType_COMMITMENT_REVOKE
		case input.HtlcOfferedRevoke:
			witnessType = walletrpc_pb.WitnessType_HTLC_OFFERED_REVOKE
		case input.HtlcAcceptedRevoke:
			witnessType = walletrpc_pb.WitnessType_HTLC_ACCEPTED_REVOKE
		case input.HtlcOfferedTimeoutSecondLevel:
			witnessType = walletrpc_pb.WitnessType_HTLC_OFFERED_TIMEOUT_SECOND_LEVEL
		case input.HtlcAcceptedSuccessSecondLevel:
			witnessType = walletrpc_pb.WitnessType_HTLC_ACCEPTED_SUCCESS_SECOND_LEVEL
		case input.HtlcOfferedRemoteTimeout:
			witnessType = walletrpc_pb.WitnessType_HTLC_OFFERED_REMOTE_TIMEOUT
		case input.HtlcAcceptedRemoteSuccess:
			witnessType = walletrpc_pb.WitnessType_HTLC_ACCEPTED_REMOTE_SUCCESS
		case input.HtlcSecondLevelRevoke:
			witnessType = walletrpc_pb.WitnessType_HTLC_SECOND_LEVEL_REVOKE
		case input.WitnessKeyHash:
			witnessType = walletrpc_pb.WitnessType_WITNESS_KEY_HASH
		case input.NestedWitnessKeyHash:
			witnessType = walletrpc_pb.WitnessType_NESTED_WITNESS_KEY_HASH
		case input.CommitmentAnchor:
			witnessType = walletrpc_pb.WitnessType_COMMITMENT_ANCHOR
		default:
			log.Warnf("Unhandled witness type %v for input %v",
				pendingInput.WitnessType, pendingInput.OutPoint)
		}

		op := &rpc_pb.OutPoint{
			TxidBytes:   pendingInput.OutPoint.Hash[:],
			OutputIndex: pendingInput.OutPoint.Index,
		}
		amountSat := uint32(pendingInput.Amount)
		satPerByte := uint32(pendingInput.LastFeeRate.FeePerKVByte() / 1000)
		broadcastAttempts := uint32(pendingInput.BroadcastAttempts)
		nextBroadcastHeight := uint32(pendingInput.NextBroadcastHeight)

		requestedFee := pendingInput.Params.Fee
		requestedFeeRate := uint32(requestedFee.FeeRate.FeePerKVByte() / 1000)

		rpcPendingSweeps = append(rpcPendingSweeps, &walletrpc_pb.PendingSweep{
			Outpoint:            op,
			WitnessType:         witnessType,
			AmountSat:           amountSat,
			SatPerByte:          satPerByte,
			BroadcastAttempts:   broadcastAttempts,
			NextBroadcastHeight: nextBroadcastHeight,
			RequestedSatPerByte: requestedFeeRate,
			RequestedConfTarget: requestedFee.ConfTarget,
			Force:               pendingInput.Params.Force,
		})
	}

	return &walletrpc_pb.PendingSweepsResponse{
		PendingSweeps: rpcPendingSweeps,
	}, nil
}

// unmarshallOutPoint converts an outpoint from its lnrpc type to its canonical
// type.
func unmarshallOutPoint(op *rpc_pb.OutPoint) (*wire.OutPoint, er.R) {
	if op == nil {
		return nil, er.Errorf("empty outpoint provided")
	}

	var hash chainhash.Hash
	switch {
	case len(op.TxidBytes) == 0 && len(op.TxidStr) == 0:
		fallthrough

	case len(op.TxidBytes) != 0 && len(op.TxidStr) != 0:
		return nil, er.Errorf("either TxidBytes or TxidStr must be " +
			"specified, but not both")

	// The hash was provided as raw bytes.
	case len(op.TxidBytes) != 0:
		copy(hash[:], op.TxidBytes)

	// The hash was provided as a hex-encoded string.
	case len(op.TxidStr) != 0:
		h, err := chainhash.NewHashFromStr(op.TxidStr)
		if err != nil {
			return nil, err
		}
		hash = *h
	}

	return &wire.OutPoint{
		Hash:  hash,
		Index: op.OutputIndex,
	}, nil
}

// BumpFee allows bumping the fee rate of an arbitrary input. A fee preference
// can be expressed either as a specific fee rate or a delta of blocks in which
// the output should be swept on-chain within. If a fee preference is not
// explicitly specified, then an error is returned. The status of the input
// sweep can be checked through the PendingSweeps RPC.
func (w *WalletKit) BumpFee(ctx context.Context,
	in *walletrpc_pb.BumpFeeRequest) (*walletrpc_pb.BumpFeeResponse, error) {
	res, err := w.BumpFee0(ctx, in)
	return res, er.Native(err)
}
func (w *WalletKit) BumpFee0(ctx context.Context,
	in *walletrpc_pb.BumpFeeRequest) (*walletrpc_pb.BumpFeeResponse, er.R) {

	// Parse the outpoint from the request.
	op, err := unmarshallOutPoint(in.Outpoint)
	if err != nil {
		return nil, err
	}

	// Construct the request's fee preference.
	satPerKw := chainfee.SatPerKVByte(in.SatPerByte * 1000).FeePerKWeight()
	feePreference := sweep.FeePreference{
		ConfTarget: uint32(in.TargetConf),
		FeeRate:    satPerKw,
	}

	// We'll attempt to bump the fee of the input through the UtxoSweeper.
	// If it is currently attempting to sweep the input, then it'll simply
	// bump its fee, which will result in a replacement transaction (RBF)
	// being broadcast. If it is not aware of the input however,
	// lnwallet.ErrNotMine is returned.
	sweepParams := sweep.ParamsUpdate{
		Fee:   feePreference,
		Force: in.Force,
	}

	_, err = w.cfg.Sweeper.UpdateParams(*op, sweepParams)
	switch {
	case err == nil:
		return &walletrpc_pb.BumpFeeResponse{}, nil
	case lnwallet.ErrNotMine.Is(err):
		break
	default:
		return nil, err
	}

	log.Debugf("Attempting to CPFP outpoint %s", op)

	// Since we're unable to perform a bump through RBF, we'll assume the
	// user is attempting to bump an unconfirmed transaction's fee rate by
	// sweeping an output within it under control of the wallet with a
	// higher fee rate, essentially performing a Child-Pays-For-Parent
	// (CPFP).
	//
	// We'll gather all of the information required by the UtxoSweeper in
	// order to sweep the output.
	utxo, err := w.cfg.Wallet.FetchInputInfo(op)
	if err != nil {
		return nil, err
	}

	// We're only able to bump the fee of unconfirmed transactions.
	if utxo.Confirmations > 0 {
		return nil, er.New("unable to bump fee of a confirmed " +
			"transaction")
	}

	var witnessType input.WitnessType
	switch utxo.AddressType {
	case lnwallet.WitnessPubKey:
		witnessType = input.WitnessKeyHash
	case lnwallet.NestedWitnessPubKey:
		witnessType = input.NestedWitnessKeyHash
	default:
		return nil, er.Errorf("unknown input witness %v", op)
	}

	signDesc := &input.SignDescriptor{
		Output: &wire.TxOut{
			PkScript: utxo.PkScript,
			Value:    int64(utxo.Value),
		},
		HashType: params.SigHashAll,
	}

	// We'll use the current height as the height hint since we're dealing
	// with an unconfirmed transaction.
	currentBs, err := w.cfg.Chain.BestBlock()
	if err != nil {
		return nil, er.Errorf("unable to retrieve current height: %v",
			err)
	}

	input := input.NewBaseInput(op, witnessType, signDesc, uint32(currentBs.Height))
	if _, err = w.cfg.Sweeper.SweepInput(input, sweep.Params{Fee: feePreference}); err != nil {
		return nil, err
	}

	return &walletrpc_pb.BumpFeeResponse{}, nil
}

/*
// ListSweeps returns a list of the sweeps that our node has published.
func (w *WalletKit) ListSweeps(c context.Context,
	req *walletrpc_pb.ListSweepsRequest) (*walletrpc_pb.ListSweepsResponse, error) {
	out, err := w.ListSweeps0(c, req)
	return out, er.Native(err)
}
func (w *WalletKit) ListSweeps0(ctx context.Context,
	in *walletrpc_pb.ListSweepsRequest) (*walletrpc_pb.ListSweepsResponse, er.R) {

	sweeps, err := w.cfg.Sweeper.ListSweeps()
	if err != nil {
		return nil, err
	}

	sweepTxns := make(map[string]bool)
	for _, sweep := range sweeps {
		sweepTxns[sweep.String()] = true
	}

	// Some of our sweeps could have been replaced by fee, or dropped out
	// of the mempool. Here, we lookup our wallet transactions so that we
	// can match our list of sweeps against the list of transactions that
	// the wallet is still tracking.
	transactions, err := w.cfg.Wallet.ListTransactionDetails(
		0, btcwallet.UnconfirmedHeight, 0, 0, 0, false,
	)
	if err != nil {
		return nil, err
	}

	var (
		txids     []string
		txDetails []*lnwallet.TransactionDetail
	)

	for _, tx := range transactions {
		_, ok := sweepTxns[tx.Hash.String()]
		if !ok {
			continue
		}

		// Add the txid or full tx details depending on whether we want
		// verbose output or not.
		if in.Verbose {
			txDetails = append(txDetails, tx)
		} else {
			txids = append(txids, tx.Hash.String())
		}
	}

	if in.Verbose {
		return &walletrpc_pb.ListSweepsResponse{
			Sweeps: &walletrpc_pb.ListSweepsResponse_TransactionDetails{
				TransactionDetails: lnrpc.RPCTransactionDetails(
					txDetails,
					false,
				),
			},
		}, nil
	}

	return &walletrpc_pb.ListSweepsResponse{
		Sweeps: &walletrpc_pb.ListSweepsResponse_TransactionIds{
			TransactionIds: &walletrpc_pb.ListSweepsResponse_TransactionIDs{
				TransactionIds: txids,
			},
		},
	}, nil
}
*/

// LabelTransaction adds a label to a transaction.
func (w *WalletKit) LabelTransaction(ctx context.Context,
	req *walletrpc_pb.LabelTransactionRequest) (*walletrpc_pb.LabelTransactionResponse, error) {
	out, err := w.LabelTransaction0(ctx, req)
	return out, er.Native(err)
}
func (w *WalletKit) LabelTransaction0(ctx context.Context,
	req *walletrpc_pb.LabelTransactionRequest) (*walletrpc_pb.LabelTransactionResponse, er.R) {

	// Check that the label provided in non-zero.
	if len(req.Label) == 0 {
		return nil, ErrZeroLabel.Default()
	}

	// Validate the length of the non-zero label. We do not need to use the
	// label returned here, because the original is non-zero so will not
	// be replaced.
	if _, err := labels.ValidateAPI(req.Label); err != nil {
		return nil, err
	}

	hash, err := chainhash.NewHash(req.Txid)
	if err != nil {
		return nil, err
	}

	err = w.cfg.Wallet.LabelTransaction(*hash, req.Label, req.Overwrite)
	return &walletrpc_pb.LabelTransactionResponse{}, err
}

// FundPsbt creates a fully populated PSBT that contains enough inputs to fund
// the outputs specified in the template. There are two ways of specifying a
// template: Either by passing in a PSBT with at least one output declared or
// by passing in a raw TxTemplate message. If there are no inputs specified in
// the template, coin selection is performed automatically. If the template does
// contain any inputs, it is assumed that full coin selection happened
// externally and no additional inputs are added. If the specified inputs aren't
// enough to fund the outputs with the given fee rate, an error is returned.
// After either selecting or verifying the inputs, all input UTXOs are locked
// with an internal app ID.
//
// NOTE: If this method returns without an error, it is the caller's
// responsibility to either spend the locked UTXOs (by finalizing and then
// publishing the transaction) or to unlock/release the locked UTXOs in case of
// an error on the caller's side.
func (w *WalletKit) FundPsbt(c context.Context,
	req *walletrpc_pb.FundPsbtRequest) (*walletrpc_pb.FundPsbtResponse, error) {
	out, err := w.FundPsbt0(c, req)
	return out, er.Native(err)
}
func (w *WalletKit) FundPsbt0(_ context.Context,
	req *walletrpc_pb.FundPsbtRequest) (*walletrpc_pb.FundPsbtResponse, er.R) {

	var (
		err         er.R
		packet      *psbt.Packet
		feeSatPerKW chainfee.SatPerKWeight
		locks       []*utxoLock
		rawPsbt     bytes.Buffer
	)

	// There are two ways a user can specify what we call the template (a
	// list of inputs and outputs to use in the PSBT): Either as a PSBT
	// packet directly or as a special RPC message. Find out which one the
	// user wants to use, they are mutually exclusive.
	switch {
	// The template is specified as a PSBT. All we have to do is parse it.
	case req.GetPsbt() != nil:
		r := bytes.NewReader(req.GetPsbt())
		packet, err = psbt.NewFromRawBytes(r, false)
		if err != nil {
			return nil, er.Errorf("could not parse PSBT: %v", err)
		}

	// The template is specified as a RPC message. We need to create a new
	// PSBT and copy the RPC information over.
	case req.GetRaw() != nil:
		tpl := req.GetRaw()
		if len(tpl.Outputs) == 0 {
			return nil, er.Errorf("no outputs specified")
		}

		txOut := make([]*wire.TxOut, 0, len(tpl.Outputs))
		for addrStr, amt := range tpl.Outputs {
			addr, err := btcutil.DecodeAddress(
				addrStr, w.cfg.ChainParams,
			)
			if err != nil {
				return nil, er.Errorf("error parsing address "+
					"%s for network %s: %v", addrStr,
					w.cfg.ChainParams.Name, err)
			}
			pkScript, err := txscript.PayToAddrScript(addr)
			if err != nil {
				return nil, er.Errorf("error getting pk "+
					"script for address %s: %v", addrStr,
					err)
			}

			txOut = append(txOut, &wire.TxOut{
				Value:    int64(amt),
				PkScript: pkScript,
			})
		}

		txIn := make([]*wire.OutPoint, len(tpl.Inputs))
		for idx, in := range tpl.Inputs {
			op, err := unmarshallOutPoint(in)
			if err != nil {
				return nil, er.Errorf("error parsing "+
					"outpoint: %v", err)
			}
			txIn[idx] = op
		}

		sequences := make([]uint32, len(txIn))
		packet, err = psbt.New(txIn, txOut, 2, 0, sequences)
		if err != nil {
			return nil, er.Errorf("could not create PSBT: %v", err)
		}

	default:
		return nil, er.Errorf("transaction template missing, need " +
			"to specify either PSBT or raw TX template")
	}

	// Determine the desired transaction fee.
	switch {
	// Estimate the fee by the target number of blocks to confirmation.
	case req.GetTargetConf() != 0:
		targetConf := req.GetTargetConf()
		if targetConf < 2 {
			return nil, er.Errorf("confirmation target must be " +
				"greater than 1")
		}

		feeSatPerKW, err = w.cfg.FeeEstimator.EstimateFeePerKW(
			targetConf,
		)
		if err != nil {
			return nil, er.Errorf("could not estimate fee: %v",
				err)
		}

	// Convert the fee to sat/kW from the specified sat/vByte.
	case req.GetSatPerVbyte() != 0:
		feeSatPerKW = chainfee.SatPerKVByte(
			req.GetSatPerVbyte() * 1000,
		).FeePerKWeight()

	default:
		return nil, er.Errorf("fee definition missing, need to " +
			"specify either target_conf or set_per_vbyte")
	}

	// The RPC parsing part is now over. Several of the following operations
	// require us to hold the global coin selection lock so we do the rest
	// of the tasks while holding the lock. The result is a list of locked
	// UTXOs.
	changeIndex := int32(-1)
	err = w.cfg.CoinSelectionLocker.WithCoinSelectLock(func() er.R {
		// In case the user did specify inputs, we need to make sure
		// they are known to us, still unspent and not yet locked.
		if len(packet.UnsignedTx.TxIn) > 0 {
			// Get a list of all unspent witness outputs.
			utxos, err := w.cfg.Wallet.ListUnspentWitness(
				defaultMinConf, defaultMaxConf,
			)
			if err != nil {
				return err
			}

			// Validate all inputs against our known list of UTXOs
			// now.
			err = verifyInputsUnspent(packet.UnsignedTx.TxIn, utxos)
			if err != nil {
				return err
			}
		}

		// We made sure the input from the user is as sane as possible.
		// We can now ask the wallet to fund the TX. This will not yet
		// lock any coins but might still change the wallet DB by
		// generating a new change address.
		changeIndex, err = w.cfg.Wallet.FundPsbt(packet, feeSatPerKW)
		if err != nil {
			return er.Errorf("wallet couldn't fund PSBT: %v", err)
		}

		// Make sure we can properly serialize the packet. If this goes
		// wrong then something isn't right with the inputs and we
		// probably shouldn't try to lock any of them.
		err = packet.Serialize(&rawPsbt)
		if err != nil {
			return er.Errorf("error serializing funded PSBT: %v",
				err)
		}

		// Now we have obtained a set of coins that can be used to fund
		// the TX. Let's lock them to be sure they aren't spent by the
		// time the PSBT is published. This is the action we do here
		// that could cause an error. Therefore if some of the UTXOs
		// cannot be locked, the rollback of the other's locks also
		// happens in this function. If we ever need to do more after
		// this function, we need to extract the rollback needs to be
		// extracted into a defer.
		locks, err = lockInputs(w.cfg.Wallet, packet)
		if err != nil {
			return er.Errorf("could not lock inputs: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert the lock leases to the RPC format.
	rpcLocks := make([]*walletrpc_pb.UtxoLease, len(locks))
	for idx, lock := range locks {
		rpcLocks[idx] = &walletrpc_pb.UtxoLease{
			Id: lock.lockID[:],
			Outpoint: &rpc_pb.OutPoint{
				TxidBytes:   lock.outpoint.Hash[:],
				TxidStr:     lock.outpoint.Hash.String(),
				OutputIndex: lock.outpoint.Index,
			},
			Expiration: uint64(lock.expiration.Unix()),
		}
	}

	return &walletrpc_pb.FundPsbtResponse{
		FundedPsbt:        rawPsbt.Bytes(),
		ChangeOutputIndex: changeIndex,
		LockedUtxos:       rpcLocks,
	}, nil
}

// FinalizePsbt expects a partial transaction with all inputs and outputs fully
// declared and tries to sign all inputs that belong to the wallet. Lnd must be
// the last signer of the transaction. That means, if there are any unsigned
// non-witness inputs or inputs without UTXO information attached or inputs
// without witness data that do not belong to lnd's wallet, this method will
// fail. If no error is returned, the PSBT is ready to be extracted and the
// final TX within to be broadcast.
//
// NOTE: This method does NOT publish the transaction once finalized. It is the
// caller's responsibility to either publish the transaction on success or
// unlock/release any locked UTXOs in case of an error in this method.
func (w *WalletKit) FinalizePsbt(c context.Context,
	req *walletrpc_pb.FinalizePsbtRequest) (*walletrpc_pb.FinalizePsbtResponse, error) {
	out, err := w.FinalizePsbt0(c, req)
	return out, er.Native(err)
}
func (w *WalletKit) FinalizePsbt0(_ context.Context,
	req *walletrpc_pb.FinalizePsbtRequest) (*walletrpc_pb.FinalizePsbtResponse, er.R) {

	// Parse the funded PSBT. No additional checks are required at this
	// level as the wallet will perform all of them.
	packet, err := psbt.NewFromRawBytes(
		bytes.NewReader(req.FundedPsbt), false,
	)
	if err != nil {
		return nil, er.Errorf("error parsing PSBT: %v", err)
	}

	// Let the wallet do the heavy lifting. This will sign all inputs that
	// we have the UTXO for. If some inputs can't be signed and don't have
	// witness data attached, this will fail.
	err = w.cfg.Wallet.FinalizePsbt(packet)
	if err != nil {
		return nil, er.Errorf("error finalizing PSBT: %v", err)
	}

	var (
		finalPsbtBytes bytes.Buffer
		finalTxBytes   bytes.Buffer
	)

	// Serialize the finalized PSBT in both the packet and wire format.
	err = packet.Serialize(&finalPsbtBytes)
	if err != nil {
		return nil, er.Errorf("error serializing PSBT: %v", err)
	}
	finalTx, err := psbt.Extract(packet)
	if err != nil {
		return nil, er.Errorf("unable to extract final TX: %v", err)
	}
	err = finalTx.Serialize(&finalTxBytes)
	if err != nil {
		return nil, er.Errorf("error serializing final TX: %v", err)
	}

	return &walletrpc_pb.FinalizePsbtResponse{
		SignedPsbt: finalPsbtBytes.Bytes(),
		RawFinalTx: finalTxBytes.Bytes(),
	}, nil
}
