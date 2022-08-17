package wtclientrpc

import (
	"context"
	"net"
	"strconv"

	"github.com/pkt-cash/pktd/btcec"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/wtclientrpc_pb"
	"github.com/pkt-cash/pktd/lnd/lncfg"
	"github.com/pkt-cash/pktd/lnd/lnwire"
	"github.com/pkt-cash/pktd/lnd/watchtower"
	"github.com/pkt-cash/pktd/lnd/watchtower/wtclient"
)

// ErrWtclientNotActive signals that RPC calls cannot be processed
// because the watchtower client is not active.
var ErrWtclientNotActive = er.GenericErrorType.CodeWithDetail("ErrWtclientNotActive",
	"watchtower client not active")

// WatchtowerClient is the RPC server we'll use to interact with the backing
// active watchtower client.
//
// TODO(wilmer): better name?
type WatchtowerClient struct {
	cfg Config
}

// New returns a new instance of the wtclientrpc WatchtowerClient sub-server.
// We also return the set of permissions for the macaroons that we may create
// within this method. If the macaroons we need aren't found in the filepath,
// then we'll create them on start up. If we're unable to locate, or create the
// macaroons we need, then we'll return with an error.
func New(cfg *Config) (*WatchtowerClient, er.R) {
	return &WatchtowerClient{cfg: *cfg}, nil
}

// isActive returns nil if the watchtower client is initialized so that we can
// process RPC requests.
func (c *WatchtowerClient) isActive() er.R {
	if c.cfg.Active {
		return nil
	}
	return ErrWtclientNotActive.Default()
}

// AddTower adds a new watchtower reachable at the given address and considers
// it for new sessions. If the watchtower already exists, then any new addresses
// included will be considered when dialing it for session negotiations and
// backups.
func (c *WatchtowerClient) AddTower(ctx context.Context,
	req *wtclientrpc_pb.AddTowerRequest) (*rpc_pb.Null, er.R) {

	if err := c.isActive(); err != nil {
		return nil, err
	}

	pubKey, err := btcec.ParsePubKey(req.Pubkey, btcec.S256())
	if err != nil {
		return nil, err
	}
	addr, errr := lncfg.ParseAddressString(
		req.Address, strconv.Itoa(watchtower.DefaultPeerPort),
		c.cfg.Resolver,
	)
	if errr != nil {
		return nil, er.Errorf("invalid address %v: %v", req.Address, errr)
	}

	towerAddr := &lnwire.NetAddress{
		IdentityKey: pubKey,
		Address:     addr,
	}
	if err := c.cfg.Client.AddTower(towerAddr); err != nil {
		return nil, err
	}

	return nil, nil
}

// RemoveTower removes a watchtower from being considered for future session
// negotiations and from being used for any subsequent backups until it's added
// again. If an address is provided, then this RPC only serves as a way of
// removing the address from the watchtower instead.
func (c *WatchtowerClient) RemoveTower(ctx context.Context,
	req *wtclientrpc_pb.RemoveTowerRequest) (*rpc_pb.Null, er.R) {

	if err := c.isActive(); err != nil {
		return nil, err
	}

	pubKey, err := btcec.ParsePubKey(req.Pubkey, btcec.S256())
	if err != nil {
		return nil, err
	}

	var addr net.Addr
	if req.Address != "" {
		addr, err = lncfg.ParseAddressString(
			req.Address, strconv.Itoa(watchtower.DefaultPeerPort),
			c.cfg.Resolver,
		)
		if err != nil {
			return nil, er.Errorf("unable to parse tower "+
				"address %v: %v", req.Address, err)
		}
	}

	if err := c.cfg.Client.RemoveTower(pubKey, addr); err != nil {
		return nil, err
	}

	return nil, nil
}

// ListTowers returns the list of watchtowers registered with the client.
func (c *WatchtowerClient) ListTowers(ctx context.Context,
	req *wtclientrpc_pb.ListTowersRequest) (*wtclientrpc_pb.ListTowersResponse, er.R) {

	if err := c.isActive(); err != nil {
		return nil, err
	}

	towers, err := c.cfg.Client.RegisteredTowers()
	if err != nil {
		return nil, err
	}

	rpcTowers := make([]*wtclientrpc_pb.Tower, 0, len(towers))
	for _, tower := range towers {
		rpcTower := marshallTower(tower, req.IncludeSessions)
		rpcTowers = append(rpcTowers, rpcTower)
	}

	return &wtclientrpc_pb.ListTowersResponse{Towers: rpcTowers}, nil
}

// GetTowerInfo retrieves information for a registered watchtower.
func (c *WatchtowerClient) GetTowerInfo(ctx context.Context,
	req *wtclientrpc_pb.GetTowerInfoRequest) (*wtclientrpc_pb.Tower, er.R) {

	if err := c.isActive(); err != nil {
		return nil, err
	}

	pubKey, err := btcec.ParsePubKey(req.Pubkey, btcec.S256())
	if err != nil {
		return nil, err
	}

	tower, err := c.cfg.Client.LookupTower(pubKey)
	if err != nil {
		return nil, err
	}

	return marshallTower(tower, req.IncludeSessions), nil
}

// Stats returns the in-memory statistics of the client since startup.
func (c *WatchtowerClient) Stats(ctx context.Context,
	req *rpc_pb.Null) (*wtclientrpc_pb.StatsResponse, er.R) {

	if err := c.isActive(); err != nil {
		return nil, err
	}

	stats := c.cfg.Client.Stats()
	return &wtclientrpc_pb.StatsResponse{
		NumBackups:           uint32(stats.NumTasksAccepted),
		NumFailedBackups:     uint32(stats.NumTasksIneligible),
		NumPendingBackups:    uint32(stats.NumTasksReceived),
		NumSessionsAcquired:  uint32(stats.NumSessionsAcquired),
		NumSessionsExhausted: uint32(stats.NumSessionsExhausted),
	}, nil
}

// Policy returns the active watchtower client policy configuration.
func (c *WatchtowerClient) Policy(ctx context.Context,
	req *wtclientrpc_pb.PolicyRequest) (*wtclientrpc_pb.PolicyResponse, er.R) {

	if err := c.isActive(); err != nil {
		return nil, err
	}

	policy := c.cfg.Client.Policy()
	return &wtclientrpc_pb.PolicyResponse{
		MaxUpdates:      uint32(policy.MaxUpdates),
		SweepSatPerByte: uint32(policy.SweepFeeRate.FeePerKVByte() / 1000),
	}, nil
}

// marshallTower converts a client registered watchtower into its corresponding
// RPC type.
func marshallTower(tower *wtclient.RegisteredTower, includeSessions bool) *wtclientrpc_pb.Tower {
	rpcAddrs := make([]string, 0, len(tower.Addresses))
	for _, addr := range tower.Addresses {
		rpcAddrs = append(rpcAddrs, addr.String())
	}

	var rpcSessions []*wtclientrpc_pb.TowerSession
	if includeSessions {
		rpcSessions = make([]*wtclientrpc_pb.TowerSession, 0, len(tower.Sessions))
		for _, session := range tower.Sessions {
			satPerByte := session.Policy.SweepFeeRate.FeePerKVByte() / 1000
			rpcSessions = append(rpcSessions, &wtclientrpc_pb.TowerSession{
				NumBackups:        uint32(len(session.AckedUpdates)),
				NumPendingBackups: uint32(len(session.CommittedUpdates)),
				MaxBackups:        uint32(session.Policy.MaxUpdates),
				SweepSatPerByte:   uint32(satPerByte),
			})
		}
	}

	return &wtclientrpc_pb.Tower{
		Pubkey:                 tower.IdentityKey.SerializeCompressed(),
		Addresses:              rpcAddrs,
		ActiveSessionCandidate: tower.ActiveSessionCandidate,
		NumSessions:            uint32(len(tower.Sessions)),
		Sessions:               rpcSessions,
	}
}
