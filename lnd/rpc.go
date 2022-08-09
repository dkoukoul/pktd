package lnd

import (
	"context"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/pkt-cash/pktd/btcjson"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/connmgr/banmgr"
	"github.com/pkt-cash/pktd/generated/proto/meta_pb"
	"github.com/pkt-cash/pktd/generated/proto/restrpc_pb/help_pb"
	"github.com/pkt-cash/pktd/generated/proto/routerrpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/walletunlocker_pb"
	"github.com/pkt-cash/pktd/lnd/chainreg"
	"github.com/pkt-cash/pktd/lnd/lnrpc"
	"github.com/pkt-cash/pktd/lnd/lnrpc/apiv1"
	"github.com/pkt-cash/pktd/lnd/lnrpc/routerrpc"
	"github.com/pkt-cash/pktd/lnd/lnrpc/verrpc"
	"github.com/pkt-cash/pktd/lnd/lnrpc/wtclientrpc"
	"github.com/pkt-cash/pktd/lnd/walletunlocker"
	"github.com/pkt-cash/pktd/neutrino"
	"github.com/pkt-cash/pktd/pktwallet/waddrmgr"
	"github.com/pkt-cash/pktd/pktwallet/wallet"
)

// type RpcFunc struct {
// 	command string
// 	req     proto.Message
// 	res     proto.Message
// 	f       func(c *RpcContext, m proto.Message) (proto.Message, er.R)
// }

func (c *RpcContext) RegisterFunctions(a *apiv1.Apiv1) {
	lightning := apiv1.DefineCategory(
		a,
		"lightning",
		`
		The Lightning Network component of the wallet
		`,
	)
	lightningChannel := apiv1.DefineCategory(
		lightning,
		"channel",
		`
		Management of lightning channels to direct peers of this pld node
		`,
	)
	apiv1.Endpoint(
		lightningChannel,
		"",
		`
		List all open channels

		ListChannels returns a description of all the open channels that this node
		is a participant in.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.ListChannelsRequest) (*rpc_pb.ListChannelsResponse, er.R) {
			return er.E1(rs.ListChannels(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningChannel,
		"open",
		`
		Open a channel to a node or an existing peer
		
		OpenChannel attempts to open a singly funded channel specified in the
		request to a remote peer. Users are able to specify a target number of
		blocks that the funding transaction should be confirmed in, or a manual fee
		rate to us for the funding transaction. If neither are specified, then a
		lax block confirmation target is used. Each OpenStatusUpdate will return
		the pending channel ID of the in-progress channel. Depending on the
		arguments specified in the OpenChannelRequest, this pending channel ID can
		then be used to manually progress the channel funding flow.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.OpenChannelRequest) (*rpc_pb.ChannelPoint, er.R) {
			return er.E1(rs.OpenChannelSync(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningChannel,
		"close",
		`
		Close an existing channel

		CloseChannel attempts to close an active channel identified by its channel
		outpoint (ChannelPoint). The actions of this method can additionally be
		augmented to attempt a force close after a timeout period in the case of an
		inactive peer. If a non-force close (cooperative closure) is requested,
		then the user can specify either a target number of blocks until the
		closure transaction is confirmed, or a manual fee rate. If neither are
		specified, then a default lax, block confirmation target is used.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.CloseChannelRequest) (*rpc_pb.Null, er.R) {
			// TODO streaming
			return nil, er.E(rs.CloseChannel(req, nil))
		}),
	)
	apiv1.Endpoint(
		lightningChannel,
		"abandon",
		`
		Abandons an existing channel

		AbandonChannel removes all channel state from the database except for a
		close summary. This method can be used to get rid of permanently unusable
		channels due to bugs fixed in newer versions of lnd. This method can also be
		used to remove externally funded channels where the funding transaction was
		never broadcast. Only available for non-externally funded channels in dev
		build.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.AbandonChannelRequest) (*rpc_pb.AbandonChannelResponse, er.R) {
			return er.E1(rs.AbandonChannel(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningChannel,
		"balance",
		`
		Returns the sum of the total available channel balance across all open channels

		ChannelBalance returns a report on the total funds across all open channels,
		categorized in local/remote, pending local/remote and unsettled local/remote
		balances.
		`,
		withRpc(c, func(rs *LightningRPCServer, _ *rpc_pb.Null) (*rpc_pb.ChannelBalanceResponse, er.R) {
			return er.E1(rs.ChannelBalance(context.TODO(), nil))
		}),
	)
	apiv1.Endpoint(
		lightningChannel,
		"pending",
		`
		Display information pertaining to pending channels

		PendingChannels returns a list of all the channels that are currently
		considered "pending". A channel is pending if it has finished the funding
		workflow and is waiting for confirmations for the funding txn, or is in the
		process of closure, either initiated cooperatively or non-cooperatively.
		`,
		withRpc(c, func(rs *LightningRPCServer, _ *rpc_pb.Null) (*rpc_pb.PendingChannelsResponse, er.R) {
			return er.E1(rs.PendingChannels(context.TODO(), nil))
		}),
		help_pb.F_ALLOW_GET,
	)
	apiv1.Endpoint(
		lightningChannel,
		"closed",
		`
		List all closed channels

		ClosedChannels returns a description of all the closed channels that
		this node was a participant in.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.ClosedChannelsRequest) (*rpc_pb.ClosedChannelsResponse, er.R) {
			return er.E1(rs.ClosedChannels(context.TODO(), req))
		}),
		help_pb.F_ALLOW_GET,
	)
	apiv1.Endpoint(
		lightningChannel,
		"networkinfo",
		`
		Get statistical information about the current state of the network

		GetNetworkInfo returns some basic stats about the known channel graph from
		the point of view of the node.
		`,
		withRpc(c, func(rs *LightningRPCServer, _ *rpc_pb.Null) (*rpc_pb.NetworkInfo, er.R) {
			return er.E1(rs.GetNetworkInfo(context.TODO(), nil))
		}),
		help_pb.F_ALLOW_GET,
	)
	apiv1.Endpoint(
		lightningChannel,
		"feereport",
		`
		Display the current fee policies of all active channels

		FeeReport allows the caller to obtain a report detailing the current fee
		schedule enforced by the node globally for each channel.
		`,
		withRpc(c, func(rs *LightningRPCServer, _ *rpc_pb.Null) (*rpc_pb.FeeReportResponse, er.R) {
			return er.E1(rs.FeeReport(context.TODO(), nil))
		}),
	)
	apiv1.Endpoint(
		lightningChannel,
		"policy",
		`
		Display the current fee policies of all active channels

		FeeReport allows the caller to obtain a report detailing the current fee
		schedule enforced by the node globally for each channel.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.PolicyUpdateRequest) (*rpc_pb.PolicyUpdateResponse, er.R) {
			return er.E1(rs.UpdateChannelPolicy(context.TODO(), nil))
		}),
	)

	//	>>> lightning/channel/backup subCategory commands
	lightningChannelBackup := apiv1.DefineCategory(
		lightningChannel,
		"backup",
		`
		Backup and recovery of the state of active Lightning Channels
		`,
	)
	apiv1.Endpoint(
		lightningChannelBackup,
		"export",
		`
		Obtain a static channel back up for a selected channels, or all known channels

		ExportChannelBackup attempts to return an encrypted static channel backup
		for the target channel identified by it channel point. The backup is
		encrypted with a key generated from the aezeed seed of the user. The
		returned backup can either be restored using the RestoreChannelBackup
		method once lnd is running, or via the InitWallet and UnlockWallet methods
		from the WalletUnlocker service.
		`,
		func(req *rpc_pb.ExportChannelBackupRequest) (*rpc_pb.ChannelBackup, er.R) {
			//	invoke Lightning export chan backup command
			cc, errr := c.withRpcServer()
			if cc != nil {
				channelBackupResp, err := cc.ExportChannelBackup(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return channelBackupResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)

	apiv1.Endpoint(
		lightningChannelBackup,
		"verify",
		`
		Verify an existing channel backup

		VerifyChanBackup allows a caller to verify the integrity of a channel backup
		snapshot. This method will accept either a packed Single or a packed Multi.
		Specifying both will result in an error.
		`,
		func(req *rpc_pb.ChanBackupSnapshot) (*rpc_pb.VerifyChanBackupResponse, er.R) {
			//	invoke Lightning verify chan backup command
			cc, errr := c.withRpcServer()
			if cc != nil {
				var verifyChanBackupResp *rpc_pb.VerifyChanBackupResponse

				verifyChanBackupResp, err := cc.VerifyChanBackup(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return verifyChanBackupResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)

	apiv1.Endpoint(
		lightningChannelBackup,
		"restore",
		`
		Restore an existing single or multi-channel static channel backup

		RestoreChannelBackups accepts a set of singular channel backups, or a
		single encrypted multi-chan backup and attempts to recover any funds
		remaining within the channel. If we are able to unpack the backup, then the
		new channel will be shown under listchannels, as well as pending channels.
		`,
		func(req *rpc_pb.RestoreChanBackupRequest) (*rpc_pb.RestoreBackupResponse, er.R) {
			//	invoke Lightning restore chan backup command
			cc, errr := c.withRpcServer()
			if cc != nil {
				var restoreBackupResp *rpc_pb.RestoreBackupResponse

				restoreBackupResp, err := cc.RestoreChannelBackups(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return restoreBackupResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)

	//	>>> lightning/graph subCategory commands
	lightningGraph := apiv1.DefineCategory(
		lightning, "graph", "Information about the global known Lightning Network")

	apiv1.Endpoint(
		lightningGraph,
		"",
		`
		Describe the network graph

		DescribeGraph returns a description of the latest graph state from the
		point of view of the node. The graph information is partitioned into two
		components: all the nodes/vertexes, and all the edges that connect the
		vertexes themselves. As this is a directed graph, the edges also contain
		the node directional specific routing policy which includes: the time lock
		delta, fee information, etc.
		`,
		func(req *rpc_pb.ChannelGraphRequest) (*rpc_pb.ChannelGraph, er.R) {
			//	get graph description info
			cc, errr := c.withRpcServer()
			if cc != nil {
				var channelGraphResp *rpc_pb.ChannelGraph

				channelGraphResp, err := cc.DescribeGraph(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return channelGraphResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)
	apiv1.Endpoint(
		lightningGraph,
		"nodemetrics",
		`
		Get node metrics

		Returns node metrics calculated from the graph. Currently
		the only supported metric is betweenness centrality of individual nodes.
		`,
		func(req *rpc_pb.NodeMetricsRequest) (*rpc_pb.NodeMetricsResponse, er.R) {
			//	get node metrics info
			cc, errr := c.withRpcServer()
			if cc != nil {
				nodeMetricsResp, err := cc.GetNodeMetrics(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return nodeMetricsResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)
	apiv1.Endpoint(
		lightningGraph,
		"channel",
		`
		Get the state of a channel

		GetChanInfo returns the latest authenticated network announcement for the
		given channel identified by its channel ID: an 8-byte integer which
		uniquely identifies the location of transaction's funding output within the
		blockchain.
		`,
		func(req *rpc_pb.ChanInfoRequest) (*rpc_pb.ChannelEdge, er.R) {
			//	get chan info
			cc, errr := c.withRpcServer()
			if cc != nil {
				channelEdgeResp, err := cc.GetChanInfo(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return channelEdgeResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)
	apiv1.Endpoint(
		lightningGraph,
		"nodeinfo",
		`
		Get information on a specific node

		Returns the latest advertised, aggregated, and authenticated
		channel information for the specified node identified by its public key.
		`,
		func(req *rpc_pb.NodeInfoRequest) (*rpc_pb.NodeInfo, er.R) {
			//	get node info
			cc, errr := c.withRpcServer()
			if cc != nil {
				nodeInfoResp, err := cc.GetNodeInfo(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return nodeInfoResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)

	//	>>> lightning/invoice subCategory commands
	lightningInvoice := apiv1.DefineCategory(
		lightning, "invoice", "Management of invoices which are used to request payment over Lightning")
	apiv1.Endpoint(
		lightningInvoice,
		"create",
		`
		Add a new invoice

		AddInvoice attempts to add a new invoice to the invoice database. Any
		duplicated invoices are rejected, therefore all invoices *must* have a
		unique payment preimage.
		`,
		func(req *rpc_pb.Invoice) (*rpc_pb.AddInvoiceResponse, er.R) {
			//	add an invoice
			cc, errr := c.withRpcServer()
			if cc != nil {
				addInvoiceResp, err := cc.AddInvoice(context.TODO(), req)
				if err != nil {
					return nil, er.E(err)
				} else {
					return addInvoiceResp, nil
				}
			} else {
				return nil, errr
			}
		},
	)
	apiv1.Endpoint(
		lightningInvoice,
		"lookup",
		`
		Lookup an existing invoice by its payment hash

		LookupInvoice attempts to look up an invoice according to its payment hash.
		The passed payment hash *must* be exactly 32 bytes, if not, an error is
		returned.
		`,
		func(req *rpc_pb.PaymentHash) (*rpc_pb.Invoice, er.R) {
			if cc, err := c.withRpcServer(); cc != nil {
				return er.E1(cc.LookupInvoice(context.TODO(), req))
			} else {
				return nil, err
			}
		},
	)
	apiv1.Endpoint(
		lightningInvoice,
		"",
		`
		List all invoices currently stored within the database. Any active debug invoices are ignored

		ListInvoices returns a list of all the invoices currently stored within the
		database. Any active debug invoices are ignored. It has full support for
		paginated responses, allowing users to query for specific invoices through
		their add_index. This can be done by using either the first_index_offset or
		last_index_offset fields included in the response as the index_offset of the
		next request. By default, the first 100 invoices created will be returned.
		Backwards pagination is also supported through the Reversed flag.
		`,
		withRpc(c, func(cc *LightningRPCServer, req *rpc_pb.ListInvoiceRequest) (*rpc_pb.ListInvoiceResponse, er.R) {
			return er.E1(cc.ListInvoices(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningInvoice,
		"decodepayreq",
		`
		Decode a payment request

		DecodePayReq takes an encoded payment request string and attempts to decode
		it, returning a full description of the conditions encoded within the
		payment request.
		`,
		withRpc(c, func(cc *LightningRPCServer, req *rpc_pb.PayReqString) (*rpc_pb.PayReq, er.R) {
			return er.E1(cc.DecodePayReq(context.TODO(), req))
		}),
	)

	//	>>> lightning/payment subCategory command
	lightningPayment := apiv1.DefineCategory(lightning, "payment",
		"Lightning network payments which have been made, or have been forwarded, through this node")
	apiv1.Endpoint(
		lightningPayment,
		"send",
		`
		SendPayment sends payments through the Lightning Network
		`,
		withRpc(c, func(cc *LightningRPCServer, req *rpc_pb.SendRequest) (*rpc_pb.SendResponse, er.R) {
			return er.E1(cc.SendPaymentSync(context.TODO(), req))
		}),
	)
	// TODO(cjd): Streaming
	// apiv1.Register(
	// 	a,
	// 	"/lightning/payment/payinvoice",
	// 	`
	// 	Send a payment over lightning

	// 	SendPaymentV2 attempts to route a payment described by the passed
	// 	PaymentRequest to the final destination. The call returns a stream of
	// 	payment updates.
	// 	`,
	// 	false,
	// 	withRouter(c, func(rs *routerrpc.Server, req *routerrpc_pb.SendPaymentRequest) (*rpc_pb.Null, er.R) {
	// 		return er.E1(rs.SendPaymentV2(context.TODO(), req))
	// 	}),
	// )
	apiv1.Endpoint(
		lightningPayment,
		"sendtoroute",
		`
		Send a payment over a predefined route

		SendToRouteV2 attempts to make a payment via the specified route. This
		method differs from SendPayment in that it allows users to specify a full
		route manually. This can be used for things like rebalancing, and atomic
		swaps.
		`,
		withRouter(c, func(rs *routerrpc.Server, req *routerrpc_pb.SendToRouteRequest) (*rpc_pb.HTLCAttempt, er.R) {
			return er.E1(rs.SendToRouteV2(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningPayment,
		"",
		`
		List all outgoing payments

    	ListPayments returns a list of all outgoing payments.
		`,
		withRpc(c, func(cc *LightningRPCServer, req *rpc_pb.ListPaymentsRequest) (*rpc_pb.ListPaymentsResponse, er.R) {
			return er.E1(cc.ListPayments(context.TODO(), req))
		}),
	)
	// TODO(cjd): Streaming only
	// apiv1.Register(
	// 	a,
	// 	"/lightning/payment/track",
	// 	`
	// 	Track payment

	// 	TrackPaymentV2 returns an update stream for the payment identified by the
	// 	payment hash.
	// 	`,
	// 	false,
	// 	withRouter(c, func(rs *routerrpc.Server, req *routerrpc_pb.TrackPaymentRequest) (*rpc_pb.HTLCAttempt, er.R) {
	// 		return er.E1(rs.TrackPaymentV2(context.TODO(), req))
	// 	}),
	// )
	apiv1.Endpoint(
		lightningPayment,
		"queryroutes",
		`
		Query a route to a destination

		QueryRoutes attempts to query the daemon's Channel Router for a possible
		route to a target destination capable of carrying a specific amount of
		satoshis. The returned route contains the full details required to craft and
		send an HTLC, also including the necessary information that should be
		present within the Sphinx packet encapsulated within the HTLC.
		`,
		withRpc(c, func(cc *LightningRPCServer, req *rpc_pb.QueryRoutesRequest) (*rpc_pb.QueryRoutesResponse, er.R) {
			return er.E1(cc.QueryRoutes(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningPayment,
		"fwdinghistory",
		`
		Query the history of all forwarded HTLCs

		ForwardingHistory allows the caller to query the htlcswitch for a record of
		all HTLCs forwarded within the target time range, and integer offset
		within that time range. If no time-range is specified, then the first chunk
		of the past 24 hrs of forwarding history are returned.
	
		A list of forwarding events are returned. Each response has the index offset
		of the last entry. The index offset can be provided to the request to allow
		the caller to skip a series of records.
		`,
		withRpc(c, func(cc *LightningRPCServer, req *rpc_pb.ForwardingHistoryRequest) (*rpc_pb.ForwardingHistoryResponse, er.R) {
			return er.E1(cc.ForwardingHistory(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningPayment,
		"querymc",
		`
		Query the internal mission control state

		QueryMissionControl exposes the internal mission control state to callers.
		It is a development feature.
		`,
		withRouter(c, func(rs *routerrpc.Server, _ *rpc_pb.Null) (*routerrpc_pb.QueryMissionControlResponse, er.R) {
			return er.E1(rs.QueryMissionControl(context.TODO(), nil))
		}),
	)
	apiv1.Endpoint(
		lightningPayment,
		"queryprob",
		`
		Estimate a success probability

		QueryProbability returns the current success probability estimate for a
		given node pair and amount.
		`,
		withRouter(c, func(rs *routerrpc.Server, req *routerrpc_pb.QueryProbabilityRequest) (*routerrpc_pb.QueryProbabilityResponse, er.R) {
			return er.E1(rs.QueryProbability(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		lightningPayment,
		"resetmc",
		`
		Reset internal mission control state

		ResetMissionControl clears all mission control state and starts with a clean slate.
		`,
		withRouter(c, func(rs *routerrpc.Server, req *rpc_pb.Null) (*rpc_pb.Null, er.R) {
			_, err := rs.ResetMissionControl(context.TODO(), nil)
			return nil, er.E(err)
		}),
	)
	apiv1.Endpoint(
		lightningPayment,
		"buildroute",
		`
		Build a route from a list of hop pubkeys

		BuildRoute builds a fully specified route based on a list of hop public
		keys. It retrieves the relevant channel policies from the graph in order to
		calculate the correct fees and time locks.
		`,
		withRouter(c, func(rs *routerrpc.Server, req *routerrpc_pb.BuildRouteRequest) (*routerrpc_pb.BuildRouteResponse, er.R) {
			return er.E1(rs.BuildRoute(context.TODO(), req))
		}),
	)

	//	>>> lightning/peer subCategory command

	lightningPeer := apiv1.DefineCategory(lightning, "peer", "Lightning nodes to which we are directly connected")
	apiv1.Endpoint(
		lightningPeer,
		"connect",
		`
		Connect to a remote pld peer

		ConnectPeer attempts to establish a connection to a remote peer. This is at
		the networking level, and is used for communication between nodes. This is
		distinct from establishing a channel with a peer.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.ConnectPeerRequest) (*rpc_pb.Null, er.R) {
			_, err := er.E1(rs.ConnectPeer(context.TODO(), req))
			return nil, err
		}),
	)
	apiv1.Endpoint(
		lightningPeer,
		"disconnect",
		`
		Disconnect a remote pld peer identified by public key

		DisconnectPeer attempts to disconnect one peer from another identified by a
		given pubKey. In the case that we currently have a pending or active channel
		with the target peer, then this action will be not be allowed.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.DisconnectPeerRequest) (*rpc_pb.Null, er.R) {
			_, err := er.E1(rs.DisconnectPeer(context.TODO(), req))
			return nil, err
		}),
	)
	apiv1.Endpoint(
		lightningPeer,
		"",
		`
		List all active, currently connected peers

		ListPeers returns a verbose listing of all currently active peers.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.Null) (*rpc_pb.ListPeersResponse, er.R) {
			_, err := er.E1(rs.ListPeers(context.TODO(), nil))
			return nil, err
		}),
	)

	//	>>> meta category command
	meta := apiv1.DefineCategory(a, "meta",
		"API endpoints which are relevant to the entire pld node, not any specific module")
	apiv1.Endpoint(
		meta,
		"debuglevel",
		`
		Set the debug level

		DebugLevel allows a caller to programmatically set the logging verbosity of
		lnd. The logging can be targeted according to a coarse daemon-wide logging
		level, or in a granular fashion to specify the logging for a target
		sub-system.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.DebugLevelRequest) (*rpc_pb.DebugLevelResponse, er.R) {
			return er.E1(rs.DebugLevel(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		meta,
		"getinfo",
		`
		Returns basic information related to the active daemon

		GetInfo returns general information concerning the lightning node including
		it's identity pubkey, alias, the chains it is connected to, and information
		concerning the number of open+pending channels.
		`,
		func(m *rpc_pb.Null) (*meta_pb.GetInfo2Response, er.R) {
			var ni rpc_pb.NeutrinoInfo
			if n, _ := c.withNeutrino(); n != nil {
				neutrinoPeers := n.Peers()
				for i := range neutrinoPeers {
					var peerDesc rpc_pb.PeerDesc
					neutrinoPeer := neutrinoPeers[i]

					peerDesc.BytesReceived = neutrinoPeer.BytesReceived()
					peerDesc.BytesSent = neutrinoPeer.BytesSent()
					peerDesc.LastRecv = neutrinoPeer.LastRecv().String()
					peerDesc.LastSend = neutrinoPeer.LastSend().String()
					peerDesc.Connected = neutrinoPeer.Connected()
					peerDesc.Addr = neutrinoPeer.Addr()
					peerDesc.Inbound = neutrinoPeer.Inbound()
					na := neutrinoPeer.NA()
					if na != nil {
						peerDesc.Na = na.IP.String() + ":" + strconv.Itoa(int(na.Port))
					}
					peerDesc.Id = neutrinoPeer.ID()
					peerDesc.UserAgent = neutrinoPeer.UserAgent()
					peerDesc.Services = neutrinoPeer.Services().String()
					peerDesc.VersionKnown = neutrinoPeer.VersionKnown()
					peerDesc.AdvertisedProtoVer = neutrinoPeer.Describe().AdvertisedProtoVer
					peerDesc.ProtocolVersion = neutrinoPeer.ProtocolVersion()
					peerDesc.SendHeadersPreferred = neutrinoPeer.Describe().SendHeadersPreferred
					peerDesc.VerAckReceived = neutrinoPeer.VerAckReceived()
					peerDesc.WitnessEnabled = neutrinoPeer.Describe().WitnessEnabled
					peerDesc.WireEncoding = strconv.Itoa(int(neutrinoPeer.Describe().WireEncoding))
					peerDesc.TimeOffset = neutrinoPeer.TimeOffset()
					peerDesc.TimeConnected = neutrinoPeer.Describe().TimeConnected.String()
					peerDesc.StartingHeight = neutrinoPeer.StartingHeight()
					peerDesc.LastBlock = neutrinoPeer.LastBlock()
					if neutrinoPeer.LastAnnouncedBlock() != nil {
						peerDesc.LastAnnouncedBlock = neutrinoPeer.LastAnnouncedBlock().CloneBytes()
					}
					peerDesc.LastPingNonce = neutrinoPeer.LastPingNonce()
					peerDesc.LastPingTime = neutrinoPeer.LastPingTime().String()
					peerDesc.LastPingMicros = neutrinoPeer.LastPingMicros()

					ni.Peers = append(ni.Peers, &peerDesc)
				}
				n.BanMgr().ForEachIp(func(bi banmgr.BanInfo) er.R {
					ban := rpc_pb.NeutrinoBan{}
					ban.Addr = bi.Addr
					ban.Reason = bi.Reason
					ban.EndTime = bi.BanExpiresTime.String()
					ban.BanScore = bi.BanScore

					ni.Bans = append(ni.Bans, &ban)
					return nil
				})

				neutrionoQueries := n.GetActiveQueries()
				for i := range neutrionoQueries {
					nq := rpc_pb.NeutrinoQuery{}
					query := neutrionoQueries[i]
					if query.Peer != nil {
						nq.Peer = query.Peer.String()
					} else {
						nq.Peer = "<nil>"
					}
					nq.Command = query.Command
					nq.ReqNum = query.ReqNum
					nq.CreateTime = query.CreateTime
					nq.LastRequestTime = query.LastRequestTime
					nq.LastResponseTime = query.LastResponseTime

					ni.Queries = append(ni.Queries, &nq)
				}

				bb, err := n.BestBlock()
				if err != nil {
					return nil, err
				}
				ni.BlockHash = bb.Hash.String()
				ni.Height = bb.Height
				ni.BlockTimestamp = bb.Timestamp.String()
				ni.IsSyncing = !n.IsCurrent()
			}

			var walletInfo *rpc_pb.WalletInfo
			if w, _ := c.withWallet(); w != nil {
				mgrStamp := w.Manager.SyncedTo()
				walletStats := &rpc_pb.WalletStats{}
				w.ReadStats(func(ws *btcjson.WalletStats) {
					walletStats.MaintenanceInProgress = ws.MaintenanceInProgress
					walletStats.MaintenanceName = ws.MaintenanceName
					walletStats.MaintenanceCycles = int32(ws.MaintenanceCycles)
					walletStats.MaintenanceLastBlockVisited = int32(ws.MaintenanceLastBlockVisited)
					walletStats.Syncing = ws.Syncing
					if ws.SyncStarted != nil {
						walletStats.SyncStarted = ws.SyncStarted.String()
					}
					walletStats.SyncRemainingSeconds = ws.SyncRemainingSeconds
					walletStats.SyncCurrentBlock = ws.SyncCurrentBlock
					walletStats.SyncFrom = ws.SyncFrom
					walletStats.SyncTo = ws.SyncTo
					walletStats.BirthdayBlock = ws.BirthdayBlock
				})
				walletInfo = &rpc_pb.WalletInfo{
					CurrentBlockHash:      mgrStamp.Hash.String(),
					CurrentHeight:         mgrStamp.Height,
					CurrentBlockTimestamp: mgrStamp.Timestamp.String(),
					WalletVersion:         int32(waddrmgr.LatestMgrVersion),
					WalletStats:           walletStats,
				}
			}

			// Get Lightning info
			var lightning *rpc_pb.GetInfoResponse
			if cc, _ := c.withRpcServer(); cc != nil {
				if l, err := cc.GetInfo(context.TODO(), nil); err != nil {
					return nil, er.E(err)
				} else {
					lightning = l
				}
			}

			return &meta_pb.GetInfo2Response{
				Neutrino:  &ni,
				Wallet:    walletInfo,
				Lightning: lightning,
			}, nil
		},
	)
	apiv1.Endpoint(
		meta,
		"stop",
		`
		Stop and shutdown the daemon

		StopDaemon will send a shutdown request to the interrupt handler, triggering
		a graceful shutdown of the daemon.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.Null) (*rpc_pb.Null, er.R) {
			_, err := er.E1(rs.StopDaemon(context.TODO(), nil))
			return nil, err
		}),
	)

	/*
		//	service daemon version  -  URI /meta/version
		{
			command: help.CommandVersion,
			req:     nil,
			res:     (*verrpc_pb.Version)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get daemon version
				cc, errr := c.withVerRPCServer()
				if cc != nil {
					var versionResp *verrpc_pb.Version

					versionResp, err := cc.GetVersion(context.TODO(), nil)
					if err != nil {
						return nil, er.E(err)
					} else {
						return versionResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service force pld crash  -  URI /meta/crash
		{
			command: help.CommandCrash,
			req:     nil,
			res:     (*rest_pb.RestEmptyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				var someVariable *string = nil

				//	dereference o nil pointer to force a core dump
				if len(*someVariable) == 0 {
					return nil, nil
				}

				return &rest_pb.RestEmptyResponse{}, nil
			},
		},
	*/

	// wallet category commands
	wallet := apiv1.DefineCategory(a, "wallet", "APIs for management of on-chain (non-Lightning) payments")
	apiv1.Endpoint(
		wallet,
		"balance",
		`
		Compute and display the wallet's current balance

		WalletBalance returns total unspent outputs(confirmed and unconfirmed), all
		confirmed unspent outputs and all unconfirmed unspent outputs under control
		of the wallet.
		`,
		withRpc(c, func(rs *LightningRPCServer, _ *rpc_pb.Null) (*rpc_pb.WalletBalanceResponse, er.R) {
			return er.E1(rs.WalletBalance(context.TODO(), nil))
		}),
	)
	apiv1.Endpoint(
		wallet,
		"changepassphrase",
		`
		Change an encrypted wallet's password at startup

		ChangePassword changes the password of the encrypted wallet. This will
		automatically unlock the wallet database if successful.
		`,
		withMeta(c, func(rs *lnrpc.MetaService, req *meta_pb.ChangePasswordRequest) (*rpc_pb.Null, er.R) {
			_, err := rs.ChangePassword0(context.TODO(), req)
			return nil, err
		}),
	)
	apiv1.Endpoint(
		wallet,
		"checkpassphrase",
		`
		Check the wallet's password

    	CheckPassword verify that the password in the request is valid for the wallet.
		`,
		withMeta(c, func(rs *lnrpc.MetaService, req *meta_pb.CheckPasswordRequest) (*meta_pb.CheckPasswordResponse, er.R) {
			return rs.CheckPassword0(context.TODO(), req)
		}),
	)
	apiv1.Endpoint(
		wallet,
		"create",
		`
		Initialize a wallet when starting lnd for the first time

		This is used when lnd is starting up for the first time to fully
		initialize the daemon and its internal wallet. At the very least a wallet
		password must be provided. This will be used to encrypt sensitive material
		on disk.
	
		In the case of a recovery scenario, the user can also specify their aezeed
		mnemonic and passphrase. If set, then the daemon will use this prior state
		to initialize its internal wallet.
	
		Alternatively, this can be used along with the /util/seed/create RPC to
		obtain a seed, then present it to the user. Once it has been verified by
		the user, the seed can be fed into this RPC in order to commit the new
		wallet.
		`,
		withUnlocker(c, func(rs *walletunlocker.UnlockerService, req *walletunlocker_pb.InitWalletRequest) (*rpc_pb.Null, er.R) {
			_, err := rs.InitWallet0(context.TODO(), req)
			return nil, err
		}),
	)
	apiv1.Endpoint(
		wallet,
		"getsecret",
		`
		Get a secret

    	This provides which is generated using the wallet's private keys,
		this can be used as a password for another application. It will be
		the same as long as this wallet exists, even if it is re-recovered from seed.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.GetSecretRequest) (*rpc_pb.GetSecretResponse, er.R) {
			return er.E1(rs.GetSecret(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		wallet,
		"seed",
		`
		Get the wallet seed words for this wallet

    	Get the wallet seed words for this wallet, this seed is returned in an
		ENCRYPTED form (using the wallet passphrase as key). The output is 15 words.
		`,
		withRpc(c, func(rs *LightningRPCServer, _ *rpc_pb.Null) (*rpc_pb.GetWalletSeedResponse, er.R) {
			return er.E1(rs.GetWalletSeed(context.TODO(), nil))
		}),
	)
	apiv1.Endpoint(
		wallet,
		"unlock",
		`
		Unlock an encrypted wallet at startup

		Required at startup of pld to provide a password to unlock the wallet database.
		`,
		withUnlocker(c, func(rs *walletunlocker.UnlockerService, req *walletunlocker_pb.UnlockWalletRequest) (*rpc_pb.Null, er.R) {
			return rs.UnlockWallet0(context.TODO(), req)
		}),
	)

	//	>>> wallet/networkstewardvote subCategory command
	walletNetworkStewardVote := apiv1.DefineCategory(wallet, "networkstewardvote",
		"Control how this wallet votes on PKT Network Steward")
	apiv1.Endpoint(
		walletNetworkStewardVote,
		"",
		`
		Find out how the wallet's currently configured to vote

		Find out how the wallet is currently configured to vote in a network steward election.
		`,
		withRpc(c, func(rs *LightningRPCServer, _ *rpc_pb.Null) (*rpc_pb.GetNetworkStewardVoteResponse, er.R) {
			return er.E1(rs.GetNetworkStewardVote(context.TODO(), nil))
		}),
	)
	apiv1.Endpoint(
		walletNetworkStewardVote,
		"set",
		`
		Configure the wallet to vote for a network steward

		Configure the wallet to vote for a network steward when making payments (note: payments to segwit addresses cannot vote)
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.SetNetworkStewardVoteRequest) (*rpc_pb.Null, er.R) {
			_, e := er.E1(rs.SetNetworkStewardVote(context.TODO(), req))
			return nil, e
		}),
	)

	//	>>> wallet/transaction subCategory command
	walletTransaction := apiv1.DefineCategory(wallet, "transaction",
		"Create and manage on-chain transactions with the wallet")
	apiv1.Endpoint(
		walletTransaction,
		"",
		`
		Get details regarding a transaction

    	Returns a JSON object with details regarding a transaction relevant to this wallet.
		If the transaction is not known to be relevant to at least one address in this wallet
		it will appear as "not found" even if the transaction is real.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.GetTransactionRequest) (*rpc_pb.GetTransactionResponse, er.R) {
			return er.E1(rs.GetTransaction(context.TODO(), req))
		}),
	)
	apiv1.Endpoint(
		walletTransaction,
		"create",
		`
		Create a transaction but do not send it to the chain

		This does not store the transaction as existing in the wallet so
		/wallet/transaction/query will not return a transaction created by this
		endpoint. In order to make multiple transactions concurrently, prior to
		the first transaction being submitted to the chain, you must specify the
		autolock field.
		`,
		withRpc(c, func(rs *LightningRPCServer, req *rpc_pb.CreateTransactionRequest) (*rpc_pb.CreateTransactionResponse, er.R) {
			return er.E1(rs.CreateTransaction(context.TODO(), req))
		}),
	)

	/*
		//	service sendcoins  -  URI /wallet/transaction/sendcoins
		{
			command: help.CommandSendCoins,
			req:     (*rpc_pb.SendCoinsRequest)(nil),
			res:     (*rpc_pb.SendCoinsResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				sendCoinsReq, ok := m.(*rpc_pb.SendCoinsRequest)
				if !ok {
					return nil, er.New("Argument is not a SendCoinsRequest")
				}

				//	send coins to one addresses
				cc, errr := c.withRpcServer()
				if cc != nil {
					var sendCoinsResp *rpc_pb.SendCoinsResponse

					sendCoinsResp, err := cc.SendCoins(context.TODO(), sendCoinsReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return sendCoinsResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	Sendfrom  -  URI /wallet/transaction/sendfrom
		{
			command: help.CommandSendFrom,
			req:     (*rpc_pb.SendFromRequest)(nil),
			res:     (*rpc_pb.SendFromResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				req, ok := m.(*rpc_pb.SendFromRequest)
				if !ok {
					return nil, er.New("Argument is not a SendFromRequest")
				}
				if server, err := c.withRpcServer(); server != nil {
					if l, err := server.SendFrom(context.TODO(), req); err != nil {
						return nil, er.E(err)
					} else {
						return l, nil
					}
				} else {
					return nil, err
				}
			},
		},
		//	service sendmany  -  URI /wallet/transaction/sendmany
		{
			command: help.CommandSendMany,
			req:     (*rpc_pb.SendManyRequest)(nil),
			res:     (*rpc_pb.SendManyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				sendManyReq, ok := m.(*rpc_pb.SendManyRequest)
				if !ok {
					return nil, er.New("Argument is not a SendManyRequest")
				}

				//	send coins to many addresses
				cc, errr := c.withRpcServer()
				if cc != nil {
					var sendManyResp *rpc_pb.SendManyResponse

					sendManyResp, err := cc.SendMany(context.TODO(), sendManyReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return sendManyResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},

		//	>>> wallet/unspent subCategory command

		// URI /wallet/loosetxn/watch
		{
			command: "LooseTransactionsWatch",
			req:     nil,
			res:     nil,
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				w, err := c.withWallet()
				if err != nil {
					return nil, err
				}
				w.WatchLooseTransactions()
				return nil, nil
			},
		},
		// URI /wallet/loosetxn/stopwatch
		{
			command: "LooseTransactionsStopWatch",
			req:     nil,
			res:     nil,
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				w, err := c.withWallet()
				if err != nil {
					return nil, err
				}
				w.StopWatchLooseTransactions()
				return nil, nil
			},
		},
		// URI /wallet/loosetxn
		{
			command: "LooseTransactions",
			req:     nil,
			res:     (*rpc_pb.LooseTxnRes)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				w, err := c.withWallet()
				if err != nil {
					return nil, err
				}
				ret := w.WatchingLooseTransactions()
				return &rpc_pb.LooseTxnRes{IsWatching: ret}, nil
			},
		},

		//	service listunspent  -  URI /wallet/unspent
		{
			command: help.CommandListUnspent,
			req:     (*rpc_pb.ListUnspentRequest)(nil),
			res:     (*rpc_pb.ListUnspentResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				listUnspentReq, ok := m.(*rpc_pb.ListUnspentRequest)
				if !ok {
					return nil, er.New("Argument is not a ListUnspentRequest")
				}

				//	get a list of available utxos
				cc, errr := c.withRpcServer()
				if cc != nil {
					var listUnspentResp *rpc_pb.ListUnspentResponse

					listUnspentResp, err := cc.ListUnspent(context.TODO(), listUnspentReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return listUnspentResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	Resync  -  URI /wallet/unspent/resync
		{
			command: help.CommandResync,
			req:     (*rpc_pb.ReSyncChainRequest)(nil),
			res:     (*rest_pb.RestEmptyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				req, ok := m.(*rpc_pb.ReSyncChainRequest)
				if !ok {
					return nil, er.New("Argument is not a ReSyncChainRequest")
				}
				if server, err := c.withRpcServer(); server != nil {
					if _, err := server.ReSync(context.TODO(), req); err != nil {
						return nil, er.E(err)
					} else {
						return &rest_pb.RestEmptyResponse{}, nil
					}
				} else {
					return nil, err
				}
			},
		},
		//	StopResync  -  URI /wallet/unspent/stopresync
		{
			command: help.CommandStopResync,
			req:     nil,
			res:     (*rpc_pb.StopReSyncResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				if server, err := c.withRpcServer(); server != nil {
					if l, err := server.StopReSync(context.TODO(), nil); err != nil {
						return nil, er.E(err)
					} else {
						return l, nil
					}
				} else {
					return nil, err
				}
			},
		},

		//	>>> wallet/unspent/lock subCategory command

		//	service listlockunspent  -  URI /wallet/unspent/lock
		{
			command: help.CommandListLockUnspent,
			req:     nil,
			res:     (*rpc_pb.ListLockUnspentResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	invoke wallet list lock unspent command
				cc, errr := c.withRpcServer()
				if cc != nil {
					var listLockUnspentResp *rpc_pb.ListLockUnspentResponse

					listLockUnspentResp, err := cc.ListLockUnspent(context.TODO(), nil)
					if err != nil {
						return nil, er.E(err)
					} else {
						return listLockUnspentResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service lockunspent  -  URI /wallet/unspent/lock/create
		{
			command: help.CommandLockUnspent,
			req:     (*rpc_pb.LockUnspentRequest)(nil),
			res:     (*rpc_pb.LockUnspentResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				lockUnspentReq, ok := m.(*rpc_pb.LockUnspentRequest)
				if !ok {
					return nil, er.New("Argument is not a LockUnspentRequest")
				}

				//	invoke wallet lock unspent command
				cc, errr := c.withRpcServer()
				if cc != nil {
					var lockUnspentResp *rpc_pb.LockUnspentResponse

					lockUnspentResp, err := cc.LockUnspent(context.TODO(), lockUnspentReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return lockUnspentResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},

		//	>>> wallet/address subCategory command

		//	GetAddressBalances  -  URI /wallet/address/balances
		{
			command: help.CommandGetAddressBalances,
			req:     (*rpc_pb.GetAddressBalancesRequest)(nil),
			res:     (*rpc_pb.GetAddressBalancesResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				req, ok := m.(*rpc_pb.GetAddressBalancesRequest)
				if !ok {
					return nil, er.New("Argument is not a GetAddressBalancesRequest")
				}
				if server, err := c.withRpcServer(); server != nil {
					if l, err := server.GetAddressBalances(context.TODO(), req); err != nil {
						return nil, er.E(err)
					} else {
						return l, nil
					}
				} else {
					return nil, err
				}
			},
		},
		//	New wallet address  -  URI /wallet/address/create
		//	requires unlocked wallet -> access to rpcServer
		{
			command: help.CommandNewAddress,
			req:     (*rpc_pb.GetNewAddressRequest)(nil),
			res:     (*rpc_pb.GetNewAddressResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {
				req, ok := m.(*rpc_pb.GetNewAddressRequest)
				if !ok {
					return nil, er.New("Argument is not a GetNewAddressRequest")
				}
				if server, err := c.withRpcServer(); server != nil {
					if l, err := server.GetNewAddress(context.TODO(), req); err != nil {
						return nil, er.E(err)
					} else {
						return l, nil
					}
				} else {
					return nil, err
				}
			},
		},
		//	service dumpprivkey  -  URI /wallet/address/dumpprivkey
		{
			command: help.CommandDumpPrivkey,
			req:     (*rpc_pb.DumpPrivKeyRequest)(nil),
			res:     (*rpc_pb.DumpPrivKeyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				dumpPrivKeyReq, ok := m.(*rpc_pb.DumpPrivKeyRequest)
				if !ok {
					return nil, er.New("Argument is not a DumpPrivKeyRequest")
				}

				//	invoke wallet dump private key command
				cc, errr := c.withRpcServer()
				if cc != nil {
					var dumpPrivKeyResp *rpc_pb.DumpPrivKeyResponse

					dumpPrivKeyResp, err := cc.DumpPrivKey(context.TODO(), dumpPrivKeyReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return dumpPrivKeyResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service importprivkey  -  URI /wallet/address/import
		{
			command: help.CommandImportPrivkey,
			req:     (*rpc_pb.ImportPrivKeyRequest)(nil),
			res:     (*rpc_pb.ImportPrivKeyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				importPrivKeyReq, ok := m.(*rpc_pb.ImportPrivKeyRequest)
				if !ok {
					return nil, er.New("Argument is not a ImportPrivKeyRequest")
				}

				//	invoke wallet import private key command
				cc, errr := c.withRpcServer()
				if cc != nil {
					var importPrivKeyResp *rpc_pb.ImportPrivKeyResponse

					importPrivKeyResp, err := cc.ImportPrivKey(context.TODO(), importPrivKeyReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return importPrivKeyResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service signmessage  -  URI /wallet/address/signmessage
		{
			command: help.CommandSignMessage,
			req:     (*rpc_pb.SignMessageRequest)(nil),
			res:     (*rpc_pb.SignMessageResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				signMessageReq, ok := m.(*rpc_pb.SignMessageRequest)
				if !ok {
					return nil, er.New("Argument is not a SignMessageRequest")
				}

				//	invoke wallet sign message command
				cc, errr := c.withRpcServer()
				if cc != nil {
					var signMessageResp *rpc_pb.SignMessageResponse

					signMessageResp, err := cc.SignMessage(context.TODO(), signMessageReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return signMessageResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	Decode transaction service  -  URI /wallet/transaction/decode
		{
			command: help.CommandDecodeRawTransaction,
			req:     (*rpc_pb.DecodeRawTransactionRequest)(nil),
			res:     (*rpc_pb.TransactionInfo)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				decodeReq, ok := m.(*rpc_pb.DecodeRawTransactionRequest)
				if !ok {
					return nil, er.New("Argument is not a DecodeRawTransactionRequest")
				}

				//	generate a new seed
				cc, errr := c.withRpcServer()
				if cc != nil {
					var decodeResp *rpc_pb.TransactionInfo

					decodeResp, err := cc.DecodeRawTransaction(context.TODO(), decodeReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return decodeResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	>>> neutrino category command

		//	service bcasttransaction  -  URI /neutrino/bcasttransaction
		{
			command: help.CommandBcastTransaction,
			req:     (*rpc_pb.BcastTransactionRequest)(nil),
			res:     (*rpc_pb.BcastTransactionResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				bcastTransactionReq, ok := m.(*rpc_pb.BcastTransactionRequest)
				if !ok {
					return nil, er.New("Argument is not a BcastTransactionRequest")
				}

				//	invoke Lightning broadcast transaction in chain command
				cc, errr := c.withRpcServer()
				if cc != nil {
					var bcastTransactionResp *rpc_pb.BcastTransactionResponse

					bcastTransactionResp, err := cc.BcastTransaction(context.TODO(), bcastTransactionReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return bcastTransactionResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service estimatefee  -  URI /neutrino/estimatefee
		{
			command: help.CommandEstimateFee,
			req:     (*rpc_pb.EstimateFeeRequest)(nil),
			res:     (*rpc_pb.EstimateFeeResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				estimateFeeReq, ok := m.(*rpc_pb.EstimateFeeRequest)
				if !ok {
					return nil, er.New("Argument is not a EstimateFeeRequest")
				}

				//	get estimate fee info
				cc, errr := c.withRpcServer()
				if cc != nil {
					var estimateFeeResp *rpc_pb.EstimateFeeResponse

					estimateFeeResp, err := cc.EstimateFee(context.TODO(), estimateFeeReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return estimateFeeResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},

		//	>>> util/seed subCategory command

		//	Change Passphrase service  -  URI /util/seed/changepassphrase
		{
			command: help.CommandChangeSeedPassphrase,
			req:     (*rpc_pb.ChangeSeedPassphraseRequest)(nil),
			res:     (*rpc_pb.ChangeSeedPassphraseResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				changeSeedPassphraseReq, ok := m.(*rpc_pb.ChangeSeedPassphraseRequest)
				if !ok {
					return nil, er.New("Argument is not a ChangeSeedPassphraseRequest")
				}

				//	invoke Lightning change seed passphrase command
				cc, errr := c.withRpcServer()
				if cc != nil {
					var changeSeedPassphraseResp *rpc_pb.ChangeSeedPassphraseResponse

					changeSeedPassphraseResp, err := cc.ChangeSeedPassphrase(context.TODO(), changeSeedPassphraseReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return changeSeedPassphraseResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	GenSeed service  -  URI /util/seed/create
		{
			command: help.CommandGenSeed,
			req:     (*walletunlocker_pb.GenSeedRequest)(nil),
			res:     (*walletunlocker_pb.GenSeedResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				genSeedReq, ok := m.(*walletunlocker_pb.GenSeedRequest)
				if !ok {
					return nil, er.New("Argument is not a GenSeedRequest")
				}

				//	generate a new seed
				cc, errr := c.withUnlocker()
				if cc != nil {
					var genSeedResp *walletunlocker_pb.GenSeedResponse

					genSeedResp, err := cc.GenSeed0(context.TODO(), genSeedReq)
					if err != nil {
						return nil, err
					} else {
						return genSeedResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},

		//	>>> wtclient/tower subCategory command

		//	service CreateWatchTower  -  URI /wtclient/tower/create
		{
			command: help.CommandCreateWatchTower,
			req:     (*wtclientrpc_pb.AddTowerRequest)(nil),
			res:     (*rest_pb.RestEmptyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				addTowerReq, ok := m.(*wtclientrpc_pb.AddTowerRequest)
				if !ok {
					return nil, er.New("Argument is not a AddTowerRequest")
				}

				//	invoke wallet get transactions command
				cc, errr := c.withWatchTowerClient()

				if cc != nil {
					_, err := cc.AddTower(context.TODO(), addTowerReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return &rest_pb.RestEmptyResponse{}, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service RemoveWatchTower  -  URI /wtclient/tower/remove
		{
			command: help.CommandRemoveTower,
			req:     (*wtclientrpc_pb.RemoveTowerRequest)(nil),
			res:     (*rest_pb.RestEmptyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				removeTowerReq, ok := m.(*wtclientrpc_pb.RemoveTowerRequest)
				if !ok {
					return nil, er.New("Argument is not a RemoveTowerRequest")
				}

				//	invoke wallet get transactions command
				cc, errr := c.withWatchTowerClient()

				if cc != nil {
					_, err := cc.RemoveTower(context.TODO(), removeTowerReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return &rest_pb.RestEmptyResponse{}, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service ListTowers  -  URI /wtclient/tower
		{
			command: help.CommandListTowers,
			req:     (*wtclientrpc_pb.ListTowersRequest)(nil),
			res:     (*wtclientrpc_pb.ListTowersResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				listTowersReq, ok := m.(*wtclientrpc_pb.ListTowersRequest)
				if !ok {
					return nil, er.New("Argument is not a ListTowersRequest")
				}

				//	invoke wallet get transactions command
				cc, errr := c.withWatchTowerClient()

				if cc != nil {
					var listTowersResp *wtclientrpc_pb.ListTowersResponse

					listTowersResp, err := cc.ListTowers(context.TODO(), listTowersReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return listTowersResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service GetTowerInfo  -  URI /wtclient/tower/getinfo
		{
			command: help.CommandGetTowerInfo,
			req:     (*wtclientrpc_pb.GetTowerInfoRequest)(nil),
			res:     (*wtclientrpc_pb.Tower)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				getTowerInfoReq, ok := m.(*wtclientrpc_pb.GetTowerInfoRequest)
				if !ok {
					return nil, er.New("Argument is not a GetTowerInfoRequest")
				}

				//	invoke wallet get transactions command
				cc, errr := c.withWatchTowerClient()

				if cc != nil {
					var towerResp *wtclientrpc_pb.Tower

					towerResp, err := cc.GetTowerInfo(context.TODO(), getTowerInfoReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return towerResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service GetTowerStats  -  URI /wtclient/tower/stats
		{
			command: help.CommandGetTowerStats,
			req:     nil,
			res:     (*wtclientrpc_pb.StatsResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				statsReq, ok := m.(*wtclientrpc_pb.StatsRequest)
				if !ok {
					return nil, er.New("Argument is not a StatsRequest")
				}

				//	invoke wallet get transactions command
				cc, errr := c.withWatchTowerClient()

				if cc != nil {
					var statsResp *wtclientrpc_pb.StatsResponse

					statsResp, err := cc.Stats(context.TODO(), statsReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return statsResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
		//	service GetTowerPolicy  -  URI /wtclient/tower/policy
		{
			command: help.CommandGetTowerPolicy,
			req:     (*wtclientrpc_pb.PolicyRequest)(nil),
			res:     (*wtclientrpc_pb.PolicyResponse)(nil),
			f: func(c *RpcContext, m proto.Message) (proto.Message, er.R) {

				//	get the request payload
				policyReq, ok := m.(*wtclientrpc_pb.PolicyRequest)
				if !ok {
					return nil, er.New("Argument is not a PolicyRequest")
				}

				//	invoke wallet get transactions command
				cc, errr := c.withWatchTowerClient()

				if cc != nil {
					var policyResp *wtclientrpc_pb.PolicyResponse

					policyResp, err := cc.Policy(context.TODO(), policyReq)
					if err != nil {
						return nil, er.E(err)
					} else {
						return policyResp, nil
					}
				} else {
					return nil, errr
				}
			},
		},
	*/
}

type RpcContext struct {
	MaybeCC               *chainreg.ChainControl
	MaybeNeutrino         *neutrino.ChainService
	MaybeWallet           *wallet.Wallet
	MaybeRpcServer        *LightningRPCServer
	MaybeWalletUnlocker   *walletunlocker.UnlockerService
	MaybeMetaService      *lnrpc.MetaService
	MaybeVerRPCServer     *verrpc.Server
	MaybeRouterServer     *routerrpc.Server
	MaybeWatchTowerClient *wtclientrpc.WatchtowerClient
}

func with(thing interface{}, name string) er.R {
	if thing == nil {
		return er.Errorf("Could not call function because [%s] is not yet ready", name)
	}
	return nil
}
func (c *RpcContext) withCC() (*chainreg.ChainControl, er.R) {
	return c.MaybeCC, with(c.MaybeCC, "ChainController")
}
func (c *RpcContext) withNeutrino() (*neutrino.ChainService, er.R) {
	return c.MaybeNeutrino, with(c.MaybeNeutrino, "Neutrino")
}
func (c *RpcContext) withWallet() (*wallet.Wallet, er.R) {
	return c.MaybeWallet, with(c.MaybeWallet, "Wallet")
}
func (c *RpcContext) withRpcServer() (*LightningRPCServer, er.R) {
	return c.MaybeRpcServer, with(c.MaybeRpcServer, "LightningServer")
}
func withRpc[Q, R proto.Message](
	c *RpcContext,
	f func(*LightningRPCServer, Q) (R, er.R),
) func(Q) (R, er.R) {
	return func(q Q) (R, er.R) {
		if c.MaybeRpcServer == nil {
			var none R
			return none, er.Errorf("Could not call function because LightningRPCServer is not yet ready")
		}
		return f(c.MaybeRpcServer, q)
	}
}
func withUnlocker[Q, R proto.Message](
	c *RpcContext,
	f func(*walletunlocker.UnlockerService, Q) (R, er.R),
) func(Q) (R, er.R) {
	return func(q Q) (R, er.R) {
		if c.MaybeWalletUnlocker == nil {
			var none R
			return none, er.Errorf("Could not call function because LightningRPCServer is not yet ready")
		}
		return f(c.MaybeWalletUnlocker, q)
	}
}
func withMeta[Q, R proto.Message](
	c *RpcContext,
	f func(*lnrpc.MetaService, Q) (R, er.R),
) func(Q) (R, er.R) {
	return func(q Q) (R, er.R) {
		if c.MaybeMetaService == nil {
			var none R
			return none, er.Errorf("Could not call function because LightningRPCServer is not yet ready")
		}
		return f(c.MaybeMetaService, q)
	}
}
func (c *RpcContext) withVerRPCServer() (*verrpc.Server, er.R) {
	return c.MaybeVerRPCServer, with(c.MaybeVerRPCServer, "VersionerService")
}
func (c *RpcContext) withRouterServer() (*routerrpc.Server, er.R) {
	return c.MaybeRouterServer, with(c.MaybeRouterServer, "RouterServer")
}
func withRouter[Q, R proto.Message](
	c *RpcContext,
	f func(*routerrpc.Server, Q) (R, er.R),
) func(Q) (R, er.R) {
	return func(q Q) (R, er.R) {
		if c.MaybeRouterServer == nil {
			var none R
			return none, er.Errorf("Could not call function because RouterServer is not yet ready")
		}
		return f(c.MaybeRouterServer, q)
	}
}

func (c *RpcContext) withWatchTowerClient() (*wtclientrpc.WatchtowerClient, er.R) {
	return c.MaybeWatchTowerClient, with(c.MaybeWatchTowerClient, "WatchTowerClient")
}
