package autopilotrpc

import (
	"encoding/hex"

	"github.com/pkt-cash/pktd/btcec"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/util"
	"github.com/pkt-cash/pktd/generated/proto/autopilotrpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/lnd/autopilot"
	"github.com/pkt-cash/pktd/lnd/lnrpc/apiv1"
)

// Server is a sub-server of the main RPC server: the autopilot RPC. This sub
// RPC server allows external callers to access the status of the autopilot
// currently active within lnd, as well as configuring it at runtime.
type Server struct {
	manager *autopilot.Manager
}

// QueryScores queries all available autopilot heuristics, in addition to any
// active combination of these heruristics, for the scores they would give to
// the given nodes.
//
// NOTE: Part of the AutopilotServer interface.
func (s *Server) queryScores(in *autopilotrpc_pb.QueryScoresRequest) (
	*autopilotrpc_pb.QueryScoresResponse, er.R) {

	var nodes []autopilot.NodeID
	for _, pubStr := range in.Pubkeys {
		pubHex, err := util.DecodeHex(pubStr)
		if err != nil {
			return nil, err
		}
		pubKey, err := btcec.ParsePubKey(pubHex, btcec.S256())
		if err != nil {
			return nil, err
		}
		nID := autopilot.NewNodeID(pubKey)
		nodes = append(nodes, nID)
	}

	// Query the heuristics.
	heuristicScores, err := s.manager.QueryHeuristics(
		nodes, !in.IgnoreLocalState,
	)
	if err != nil {
		return nil, err
	}

	resp := &autopilotrpc_pb.QueryScoresResponse{}
	for heuristic, scores := range heuristicScores {
		result := &autopilotrpc_pb.QueryScoresResponse_HeuristicResult{
			Heuristic: heuristic,
			Scores:    make(map[string]float64),
		}

		for pub, score := range scores {
			pubkeyHex := hex.EncodeToString(pub[:])
			result.Scores[pubkeyHex] = score
		}

		// Since a node not being part of the internally returned
		// scores imply a zero score, we add these before we return the
		// RPC results.
		for _, node := range nodes {
			if _, ok := scores[node]; ok {
				continue
			}
			pubkeyHex := hex.EncodeToString(node[:])
			result.Scores[pubkeyHex] = 0.0
		}

		resp.Results = append(resp.Results, result)
	}

	return resp, nil
}

// SetScores sets the scores of the external score heuristic, if active.
//
// NOTE: Part of the AutopilotServer interface.
func (s *Server) setScores(in *autopilotrpc_pb.SetScoresRequest) (*rpc_pb.Null, er.R) {

	scores := make(map[autopilot.NodeID]float64)
	for pubStr, score := range in.Scores {
		pubHex, err := util.DecodeHex(pubStr)
		if err != nil {
			return nil, err
		}
		pubKey, err := btcec.ParsePubKey(pubHex, btcec.S256())
		if err != nil {
			return nil, err
		}
		nID := autopilot.NewNodeID(pubKey)
		scores[nID] = score
	}

	if err := s.manager.SetNodeScores(in.Heuristic, scores); err != nil {
		return nil, err
	}

	return nil, nil
}

func Register(mgr *autopilot.Manager, lightning *apiv1.Apiv1) er.R {
	s := &Server{
		manager: mgr,
	}

	a := apiv1.DefineCategory(lightning, "autopilot", "Used to automatically set up and maintain channels")
	apiv1.Endpoint(
		a,
		"",
		`
		Return whether the daemon's autopilot agent is active
		`,
		func(*rpc_pb.Null) (*autopilotrpc_pb.StatusResponse, er.R) {
			return &autopilotrpc_pb.StatusResponse{
				Active: s.manager.IsActive(),
			}, nil
		},
	)
	apiv1.Endpoint(
		a,
		"start",
		`
		Start up the autopilot agent
		`,
		func(*rpc_pb.Null) (*rpc_pb.Null, er.R) {
			return nil, s.manager.StartAgent()
		},
	)
	apiv1.Endpoint(
		a,
		"stop",
		`
		Shutdown the autopilot agent
		`,
		func(*rpc_pb.Null) (*rpc_pb.Null, er.R) {
			return nil, s.manager.StopAgent()
		},
	)
	apiv1.Endpoint(
		a,
		"scores",
		`
		Queries all available autopilot heuristics
		
		In addition to any active combination of these heruristics,
		for the scores they would give to the given nodes.
		`,
		s.queryScores,
	)
	apiv1.Endpoint(
		a,
		"setscores",
		`
		Attempts to set the scores used by the running autopilot agent

    	Only works if the external scoring heuristic is enabled.
		`,
		s.setScores,
	)

	return nil
}
