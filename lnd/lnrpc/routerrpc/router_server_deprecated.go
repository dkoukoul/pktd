package routerrpc

import (
	"context"

	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/util"
	"github.com/pkt-cash/pktd/generated/proto/routerrpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
)

// legacyTrackPaymentServer is a wrapper struct that transforms a stream of main
// rpc payment structs into the legacy PaymentStatus format.
type legacyTrackPaymentServer struct {
	routerrpc_pb.Router_TrackPaymentServer
}

// Send converts a Payment object and sends it as a PaymentStatus object on the
// embedded stream.
func (i *legacyTrackPaymentServer) Send(p *rpc_pb.Payment) error {
	var state routerrpc_pb.PaymentState
	switch p.Status {
	case rpc_pb.Payment_IN_FLIGHT:
		state = routerrpc_pb.PaymentState_IN_FLIGHT
	case rpc_pb.Payment_SUCCEEDED:
		state = routerrpc_pb.PaymentState_SUCCEEDED
	case rpc_pb.Payment_FAILED:
		switch p.FailureReason {
		case rpc_pb.PaymentFailureReason_FAILURE_REASON_NONE:
			return er.Native(er.Errorf("expected fail reason"))

		case rpc_pb.PaymentFailureReason_FAILURE_REASON_TIMEOUT:
			state = routerrpc_pb.PaymentState_FAILED_TIMEOUT

		case rpc_pb.PaymentFailureReason_FAILURE_REASON_NO_ROUTE:
			state = routerrpc_pb.PaymentState_FAILED_NO_ROUTE

		case rpc_pb.PaymentFailureReason_FAILURE_REASON_ERROR:
			state = routerrpc_pb.PaymentState_FAILED_ERROR

		case rpc_pb.PaymentFailureReason_FAILURE_REASON_INCORRECT_PAYMENT_DETAILS:
			state = routerrpc_pb.PaymentState_FAILED_INCORRECT_PAYMENT_DETAILS

		case rpc_pb.PaymentFailureReason_FAILURE_REASON_INSUFFICIENT_BALANCE:
			state = routerrpc_pb.PaymentState_FAILED_INSUFFICIENT_BALANCE

		default:
			return er.Native(er.Errorf("unknown failure reason %v",
				p.FailureReason))
		}
	default:
		return er.Native(er.Errorf("unknown state %v", p.Status))
	}

	preimage, err := util.DecodeHex(string(p.PaymentPreimage))
	if err != nil {
		return er.Native(err)
	}

	legacyState := routerrpc_pb.PaymentStatus{
		State:    state,
		Preimage: preimage,
		Htlcs:    p.Htlcs,
	}

	return i.Router_TrackPaymentServer.Send(&legacyState)
}

// TrackPayment returns a stream of payment state updates. The stream is
// closed when the payment completes.
func (s *Server) TrackPayment(request *routerrpc_pb.TrackPaymentRequest,
	stream routerrpc_pb.Router_TrackPaymentServer) error {

	legacyStream := legacyTrackPaymentServer{
		Router_TrackPaymentServer: stream,
	}
	return s.TrackPaymentV2(request, &legacyStream)
}

// SendPayment attempts to route a payment described by the passed
// PaymentRequest to the final destination. If we are unable to route the
// payment, or cannot find a route that satisfies the constraints in the
// PaymentRequest, then an error will be returned. Otherwise, the payment
// pre-image, along with the final route will be returned.
func (s *Server) SendPayment(request *routerrpc_pb.SendPaymentRequest,
	stream routerrpc_pb.Router_SendPaymentServer) error {

	if request.MaxParts > 1 {
		return er.Native(er.New("for multi-part payments, use SendPaymentV2"))
	}

	legacyStream := legacyTrackPaymentServer{
		Router_TrackPaymentServer: stream,
	}
	return s.SendPaymentV2(request, &legacyStream)
}

// SendToRoute sends a payment through a predefined route. The response of this
// call contains structured error information.
func (s *Server) SendToRoute(ctx context.Context,
	req *routerrpc_pb.SendToRouteRequest) (*routerrpc_pb.SendToRouteResponse, error) {

	resp, err := s.SendToRouteV2(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, nil
	}

	// Need to convert to legacy response message because proto identifiers
	// don't line up.
	legacyResp := &routerrpc_pb.SendToRouteResponse{
		Preimage: resp.Preimage,
		Failure:  resp.Failure,
	}

	return legacyResp, err
}
