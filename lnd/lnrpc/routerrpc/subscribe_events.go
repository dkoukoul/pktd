package routerrpc

import (
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/generated/proto/routerrpc_pb"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/lnd/htlcswitch"
	"github.com/pkt-cash/pktd/lnd/invoices"
)

// rpcHtlcEvent returns a rpc htlc event from a htlcswitch event.
func rpcHtlcEvent(htlcEvent interface{}) (*routerrpc_pb.HtlcEvent, er.R) {
	rpcEvent := &routerrpc_pb.HtlcEvent{}
	var (
		key       htlcswitch.HtlcKey
		eventType htlcswitch.HtlcEventType
	)

	switch e := htlcEvent.(type) {
	case *htlcswitch.ForwardingEvent:
		rpcEvent.Event = &routerrpc_pb.HtlcEvent_ForwardEvent{
			ForwardEvent: &routerrpc_pb.ForwardEvent{
				Info: rpcInfo(e.HtlcInfo),
			},
		}

		key = e.HtlcKey
		eventType = e.HtlcEventType
		rpcEvent.TimestampNs = uint64(e.Timestamp.UnixNano())

	case *htlcswitch.ForwardingFailEvent:
		rpcEvent.Event = &routerrpc_pb.HtlcEvent_ForwardFailEvent{
			ForwardFailEvent: &routerrpc_pb.ForwardFailEvent{},
		}

		key = e.HtlcKey
		eventType = e.HtlcEventType
		rpcEvent.TimestampNs = uint64(e.Timestamp.UnixNano())

	case *htlcswitch.LinkFailEvent:
		failureCode, failReason, err := rpcFailReason(
			e.LinkError,
		)
		if err != nil {
			return nil, err
		}

		rpcEvent.Event = &routerrpc_pb.HtlcEvent_LinkFailEvent{
			LinkFailEvent: &routerrpc_pb.LinkFailEvent{
				Info:          rpcInfo(e.HtlcInfo),
				WireFailure:   failureCode,
				FailureDetail: failReason,
				FailureString: e.LinkError.Error(),
			},
		}

		key = e.HtlcKey
		eventType = e.HtlcEventType
		rpcEvent.TimestampNs = uint64(e.Timestamp.UnixNano())

	case *htlcswitch.SettleEvent:
		rpcEvent.Event = &routerrpc_pb.HtlcEvent_SettleEvent{
			SettleEvent: &routerrpc_pb.SettleEvent{},
		}

		key = e.HtlcKey
		eventType = e.HtlcEventType
		rpcEvent.TimestampNs = uint64(e.Timestamp.UnixNano())

	default:
		return nil, er.Errorf("unknown event type: %T", e)
	}

	rpcEvent.IncomingChannelId = key.IncomingCircuit.ChanID.ToUint64()
	rpcEvent.OutgoingChannelId = key.OutgoingCircuit.ChanID.ToUint64()
	rpcEvent.IncomingHtlcId = key.IncomingCircuit.HtlcID
	rpcEvent.OutgoingHtlcId = key.OutgoingCircuit.HtlcID

	// Convert the htlc event type to a rpc event.
	switch eventType {
	case htlcswitch.HtlcEventTypeSend:
		rpcEvent.EventType = routerrpc_pb.HtlcEvent_SEND

	case htlcswitch.HtlcEventTypeReceive:
		rpcEvent.EventType = routerrpc_pb.HtlcEvent_RECEIVE

	case htlcswitch.HtlcEventTypeForward:
		rpcEvent.EventType = routerrpc_pb.HtlcEvent_FORWARD

	default:
		return nil, er.Errorf("unknown event type: %v", eventType)
	}

	return rpcEvent, nil
}

// rpcInfo returns a rpc struct containing the htlc information from the
// switch's htlc info struct.
func rpcInfo(info htlcswitch.HtlcInfo) *routerrpc_pb.HtlcInfo {
	return &routerrpc_pb.HtlcInfo{
		IncomingTimelock: info.IncomingTimeLock,
		OutgoingTimelock: info.OutgoingTimeLock,
		IncomingAmtMsat:  uint64(info.IncomingAmt),
		OutgoingAmtMsat:  uint64(info.OutgoingAmt),
	}
}

// rpcFailReason maps a lnwire failure message and failure detail to a rpc
// failure code and detail.
func rpcFailReason(linkErr *htlcswitch.LinkError) (rpc_pb.Failure_FailureCode,
	routerrpc_pb.FailureDetail, er.R) {

	wireErr, err := marshallError(er.E(linkErr))
	if err != nil {
		return 0, 0, err
	}
	wireCode := wireErr.GetCode()

	// If the link has no failure detail, return with failure detail none.
	if linkErr.FailureDetail == nil {
		return wireCode, routerrpc_pb.FailureDetail_NO_DETAIL, nil
	}

	switch failureDetail := linkErr.FailureDetail.(type) {
	case invoices.FailResolutionResult:
		fd, err := rpcFailureResolution(failureDetail)
		return wireCode, fd, err

	case htlcswitch.OutgoingFailure:
		fd, err := rpcOutgoingFailure(failureDetail)
		return wireCode, fd, err

	default:
		return 0, 0, er.Errorf("unknown failure "+
			"detail type: %T", linkErr.FailureDetail)

	}

}

// rpcFailureResolution maps an invoice failure resolution to a rpc failure
// detail. Invoice failures have no zero resolution results (every failure
// is accompanied with a result), so we error if we fail to match the result
// type.
func rpcFailureResolution(invoiceFailure invoices.FailResolutionResult) (
	routerrpc_pb.FailureDetail, er.R) {

	switch invoiceFailure {
	case invoices.ResultReplayToCanceled:
		return routerrpc_pb.FailureDetail_INVOICE_CANCELED, nil

	case invoices.ResultInvoiceAlreadyCanceled:
		return routerrpc_pb.FailureDetail_INVOICE_CANCELED, nil

	case invoices.ResultAmountTooLow:
		return routerrpc_pb.FailureDetail_INVOICE_UNDERPAID, nil

	case invoices.ResultExpiryTooSoon:
		return routerrpc_pb.FailureDetail_INVOICE_EXPIRY_TOO_SOON, nil

	case invoices.ResultCanceled:
		return routerrpc_pb.FailureDetail_INVOICE_CANCELED, nil

	case invoices.ResultInvoiceNotOpen:
		return routerrpc_pb.FailureDetail_INVOICE_NOT_OPEN, nil

	case invoices.ResultMppTimeout:
		return routerrpc_pb.FailureDetail_MPP_INVOICE_TIMEOUT, nil

	case invoices.ResultAddressMismatch:
		return routerrpc_pb.FailureDetail_ADDRESS_MISMATCH, nil

	case invoices.ResultHtlcSetTotalMismatch:
		return routerrpc_pb.FailureDetail_SET_TOTAL_MISMATCH, nil

	case invoices.ResultHtlcSetTotalTooLow:
		return routerrpc_pb.FailureDetail_SET_TOTAL_TOO_LOW, nil

	case invoices.ResultHtlcSetOverpayment:
		return routerrpc_pb.FailureDetail_SET_OVERPAID, nil

	case invoices.ResultInvoiceNotFound:
		return routerrpc_pb.FailureDetail_UNKNOWN_INVOICE, nil

	case invoices.ResultKeySendError:
		return routerrpc_pb.FailureDetail_INVALID_KEYSEND, nil

	case invoices.ResultMppInProgress:
		return routerrpc_pb.FailureDetail_MPP_IN_PROGRESS, nil

	default:
		return 0, er.Errorf("unknown fail resolution: %v",
			invoiceFailure.FailureString())
	}
}

// rpcOutgoingFailure maps an outgoing failure to a rpc FailureDetail. If the
// failure detail is FailureDetailNone, which indicates that the failure was
// a wire message which required no further failure detail, we return a no
// detail failure detail to indicate that there was no additional information.
func rpcOutgoingFailure(failureDetail htlcswitch.OutgoingFailure) (
	routerrpc_pb.FailureDetail, er.R) {

	switch failureDetail {
	case htlcswitch.OutgoingFailureNone:
		return routerrpc_pb.FailureDetail_NO_DETAIL, nil

	case htlcswitch.OutgoingFailureDecodeError:
		return routerrpc_pb.FailureDetail_ONION_DECODE, nil

	case htlcswitch.OutgoingFailureLinkNotEligible:
		return routerrpc_pb.FailureDetail_LINK_NOT_ELIGIBLE, nil

	case htlcswitch.OutgoingFailureOnChainTimeout:
		return routerrpc_pb.FailureDetail_ON_CHAIN_TIMEOUT, nil

	case htlcswitch.OutgoingFailureHTLCExceedsMax:
		return routerrpc_pb.FailureDetail_HTLC_EXCEEDS_MAX, nil

	case htlcswitch.OutgoingFailureInsufficientBalance:
		return routerrpc_pb.FailureDetail_INSUFFICIENT_BALANCE, nil

	case htlcswitch.OutgoingFailureCircularRoute:
		return routerrpc_pb.FailureDetail_CIRCULAR_ROUTE, nil

	case htlcswitch.OutgoingFailureIncompleteForward:
		return routerrpc_pb.FailureDetail_INCOMPLETE_FORWARD, nil

	case htlcswitch.OutgoingFailureDownstreamHtlcAdd:
		return routerrpc_pb.FailureDetail_HTLC_ADD_FAILED, nil

	case htlcswitch.OutgoingFailureForwardsDisabled:
		return routerrpc_pb.FailureDetail_FORWARDS_DISABLED, nil

	default:
		return 0, er.Errorf("unknown outgoing failure "+
			"detail: %v", failureDetail.FailureString())
	}
}
