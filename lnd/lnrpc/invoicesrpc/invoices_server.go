package invoicesrpc

import (
	"context"

	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/lnd/channeldb"
	"github.com/pkt-cash/pktd/lnd/lnrpc"
	"github.com/pkt-cash/pktd/lnd/lntypes"
	"github.com/pkt-cash/pktd/pktlog/log"
)

const (
	// subServerName is the name of the sub rpc server. We'll use this name
	// to register ourselves, and we also require that the main
	// SubServerConfigDispatcher instance recognize it as the name of our
	// RPC service.
	subServerName = "InvoicesRPC"
)

// DefaultInvoicesMacFilename is the default name of the invoices
// macaroon that we expect to find via a file handle within the main
// configuration file in this package.
var DefaultInvoicesMacFilename = "invoices.macaroon"

// Server is a sub-server of the main RPC server: the invoices RPC. This sub
// RPC server allows external callers to access the status of the invoices
// currently active within lnd, as well as configuring it at runtime.
type Server struct {
	quit chan struct{}

	cfg *Config
}

// A compile time check to ensure that Server fully implements the
// InvoicesServer gRPC service.
var _ InvoicesServer = (*Server)(nil)

// New returns a new instance of the invoicesrpc Invoices sub-server. We also
// return the set of permissions for the macaroons that we may create within
// this method. If the macaroons we need aren't found in the filepath, then
// we'll create them on start up. If we're unable to locate, or create the
// macaroons we need, then we'll return with an error.
func New(cfg *Config) (*Server, er.R) {
	server := &Server{
		cfg:  cfg,
		quit: make(chan struct{}, 1),
	}
	return server, nil
}

// Start launches any helper goroutines required for the Server to function.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) Start() er.R {
	return nil
}

// Stop signals any active goroutines for a graceful closure.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) Stop() er.R {
	close(s.quit)

	return nil
}

// Name returns a unique string representation of the sub-server. This can be
// used to identify the sub-server and also de-duplicate them.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) Name() string {
	return subServerName
}

// RegisterWithRootServer will be called by the root gRPC server to direct a sub
// RPC server to register itself with the main gRPC root server. Until this is
// called, each sub-server won't be able to have requests routed towards it.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) RegisterWithRootServer(grpcServer *grpc.Server) er.R {
	// We make sure that we register it with the main gRPC server to ensure
	// all our methods are routed properly.
	RegisterInvoicesServer(grpcServer, s)

	log.Debugf("Invoices RPC server successfully registered with root " +
		"gRPC server")

	return nil
}

// RegisterWithRestServer will be called by the root REST mux to direct a sub
// RPC server to register itself with the main REST mux server. Until this is
// called, each sub-server won't be able to have requests routed towards it.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) RegisterWithRestServer(ctx context.Context,
	mux *runtime.ServeMux, dest string, opts []grpc.DialOption) er.R {

	// We make sure that we register it with the main REST server to ensure
	// all our methods are routed properly.
	err := RegisterInvoicesHandlerFromEndpoint(ctx, mux, dest, opts)
	if err != nil {
		log.Errorf("Could not register Invoices REST server "+
			"with root REST server: %v", err)
		return er.E(err)
	}

	log.Debugf("Invoices REST server successfully registered with " +
		"root REST server")
	return nil
}

// SubscribeSingleInvoice returns a uni-directional stream (server -> client)
// for notifying the client of state changes for a specified invoice.
func (s *Server) SubscribeSingleInvoice(
	in *SubscribeSingleInvoiceRequest,
	updateStream Invoices_SubscribeSingleInvoiceServer,
) error {
	return er.Native(s.SubscribeSingleInvoice0(in, updateStream))
}
func (s *Server) SubscribeSingleInvoice0(req *SubscribeSingleInvoiceRequest,
	updateStream Invoices_SubscribeSingleInvoiceServer) er.R {

	hash, err := lntypes.MakeHash(req.RHash)
	if err != nil {
		return err
	}

	invoiceClient, err := s.cfg.InvoiceRegistry.SubscribeSingleInvoice(hash)
	if err != nil {
		return err
	}
	defer invoiceClient.Cancel()

	for {
		select {
		case newInvoice := <-invoiceClient.Updates:
			rpcInvoice, err := CreateRPCInvoice(
				newInvoice, s.cfg.ChainParams,
			)
			if err != nil {
				return err
			}

			if err := updateStream.Send(rpcInvoice); err != nil {
				return er.E(err)
			}

		case <-s.quit:
			return nil
		}
	}
}

// SettleInvoice settles an accepted invoice. If the invoice is already settled,
// this call will succeed.
func (s *Server) SettleInvoice(
	ctx context.Context,
	in *SettleInvoiceMsg,
) (*SettleInvoiceResp, error) {
	out, err := s.SettleInvoice0(ctx, in)
	return out, er.Native(err)
}
func (s *Server) SettleInvoice0(ctx context.Context,
	in *SettleInvoiceMsg) (*SettleInvoiceResp, er.R) {

	preimage, err := lntypes.MakePreimage(in.Preimage)
	if err != nil {
		return nil, err
	}

	err = s.cfg.InvoiceRegistry.SettleHodlInvoice(preimage)
	if err != nil && !channeldb.ErrInvoiceAlreadySettled.Is(err) {
		return nil, err
	}

	return &SettleInvoiceResp{}, nil
}

// CancelInvoice cancels a currently open invoice. If the invoice is already
// canceled, this call will succeed. If the invoice is already settled, it will
// fail.
func (s *Server) CancelInvoice(
	ctx context.Context,
	in *CancelInvoiceMsg,
) (*CancelInvoiceResp, error) {
	out, err := s.CancelInvoice0(ctx, in)
	return out, er.Native(err)
}
func (s *Server) CancelInvoice0(ctx context.Context,
	in *CancelInvoiceMsg) (*CancelInvoiceResp, er.R) {

	paymentHash, err := lntypes.MakeHash(in.PaymentHash)
	if err != nil {
		return nil, err
	}

	err = s.cfg.InvoiceRegistry.CancelInvoice(paymentHash)
	if err != nil {
		return nil, err
	}

	log.Infof("Canceled invoice %v", paymentHash)

	return &CancelInvoiceResp{}, nil
}

// AddHoldInvoice attempts to add a new hold invoice to the invoice database.
// Any duplicated invoices are rejected, therefore all invoices *must* have a
// unique payment hash.
func (s *Server) AddHoldInvoice(
	ctx context.Context,
	invoice *AddHoldInvoiceRequest,
) (*AddHoldInvoiceResp, error) {
	out, err := s.AddHoldInvoice0(ctx, invoice)
	return out, er.Native(err)
}
func (s *Server) AddHoldInvoice0(
	ctx context.Context,
	invoice *AddHoldInvoiceRequest,
) (*AddHoldInvoiceResp, er.R) {

	addInvoiceCfg := &AddInvoiceConfig{
		AddInvoice:         s.cfg.InvoiceRegistry.AddInvoice,
		IsChannelActive:    s.cfg.IsChannelActive,
		ChainParams:        s.cfg.ChainParams,
		NodeSigner:         s.cfg.NodeSigner,
		DefaultCLTVExpiry:  s.cfg.DefaultCLTVExpiry,
		ChanDB:             s.cfg.ChanDB,
		GenInvoiceFeatures: s.cfg.GenInvoiceFeatures,
	}

	hash, err := lntypes.MakeHash(invoice.Hash)
	if err != nil {
		return nil, err
	}

	value, err := lnrpc.UnmarshallAmt(invoice.Value, invoice.ValueMsat)
	if err != nil {
		return nil, err
	}

	// Convert the passed routing hints to the required format.
	routeHints, err := CreateZpay32HopHints(invoice.RouteHints)
	if err != nil {
		return nil, err
	}
	addInvoiceData := &AddInvoiceData{
		Memo:            invoice.Memo,
		Hash:            &hash,
		Value:           value,
		DescriptionHash: invoice.DescriptionHash,
		Expiry:          invoice.Expiry,
		FallbackAddr:    invoice.FallbackAddr,
		CltvExpiry:      invoice.CltvExpiry,
		Private:         invoice.Private,
		HodlInvoice:     true,
		Preimage:        nil,
		RouteHints:      routeHints,
	}

	_, dbInvoice, err := AddInvoice(ctx, addInvoiceCfg, addInvoiceData)
	if err != nil {
		return nil, err
	}

	return &AddHoldInvoiceResp{
		PaymentRequest: string(dbInvoice.PaymentRequest),
	}, nil
}
