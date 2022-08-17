package apiv1

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/generated/proto/restrpc_pb/rest_pb"
	"github.com/pkt-cash/pktd/pktlog/log"
	jsonpb "google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type websocketConn websocket.Conn

type WebSocketJSonRequest struct {
	Endpoint  string          `json:"endpoint,omitempty"`
	RequestId string          `json:"request_id,omitempty"`
	HasMore   bool            `json:"has_more,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type WebSocketJSonResponse struct {
	RequestId string                 `json:"request_id,omitempty"`
	HasMore   bool                   `json:"has_more,omitempty"`
	Payload   json.RawMessage        `json:"payload,omitempty"`
	Error     rest_pb.WebSocketError `json:"error,omitempty"`
}

var upgrader = websocket.Upgrader{}

func webSocketHandler(ctx *Apiv1, httpResponse http.ResponseWriter, httpRequest *http.Request) {
	//	upgrade raw HTTP connection to a websocket
	conn, err := upgrader.Upgrade(httpResponse, httpRequest, nil)
	if err != nil {
		httpResponse.Header().Set("Content-Type", "text/plain")
		http.Error(httpResponse, "503 - Service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer conn.Close()

	//	webSocket communication loop
	var wsConn websocketConn = websocketConn(*conn)

	for {
		msgType, message, err := conn.ReadMessage()
		if err != nil {
			log.Errorf("Fail during message reading:", err)
			return
		}

		//	handle the messages according to it's type
		switch msgType {

		case websocket.TextMessage:
			wsConn.handleJsonMessage(ctx, message)

		case websocket.BinaryMessage:
			wsConn.handleProtobufMessage(ctx, message)

		case websocket.CloseMessage:
			log.Info("WebSocket closed by the client")
			return

		default:
			wsConn.errorClose(
				er.Errorf("Expecting a text/json or binary/protobuf request message, got [%T]", msgType),
			)
		}
	}
}

func wsError(err er.R) rest_pb.WebSocketError {
	return rest_pb.WebSocketError{
		Message: err.Message(),
		Stack:   err.Stack(),
	}
}

func pWsError(err er.R) *rest_pb.WebSocketProtobufResponse_Error {
	e := wsError(err)
	return &rest_pb.WebSocketProtobufResponse_Error{
		Error: &e,
	}
}

func (conn *websocketConn) errorClose(err er.R) {
	resp := WebSocketJSonResponse{
		RequestId: "FATAL ERROR",
		HasMore:   false,
		Error:     wsError(err),
	}
	if respPayload, err := jsoniter.Marshal(&resp); err != nil {
		log.Errorf("Unable to marshal error message: [%s]", err)
	} else if err := (*websocket.Conn)(conn).WriteMessage(websocket.TextMessage, respPayload); err != nil {
		log.Errorf("Unable to send error message: [%s]", err)
	}
	if err := (*websocket.Conn)(conn).Close(); err != nil {
		log.Errorf("Unable to close websocket: [%s]", err)
	}
}

func (conn *websocketConn) handleJsonMessage(ctx *Apiv1, req []byte) {

	//	unmarshal the request message
	var webSocketReq WebSocketJSonRequest

	err := jsoniter.Unmarshal(req, &webSocketReq)
	if err != nil {
		conn.errorClose(er.Errorf("Cannot parse websocket json message: [%s]", err))
		return
	}

	resp := WebSocketJSonResponse{
		RequestId: webSocketReq.RequestId,
		HasMore:   false,
		Payload:   nil,
	}
	var endpt *endpoint
	ctx.internal.funcs.R().In(func(funcs *map[string]*endpoint) er.R {
		if ep, ok := (*funcs)[webSocketReq.Endpoint]; ok {
			endpt = ep
		}
		return nil
	})
	if endpt == nil {
		resp.Error = wsError(er.Errorf("No such endpoint: [%s]", webSocketReq.Endpoint))
	} else {
		req := endpt.mkReq()
		if err := er.E(jsonpb.Unmarshal(webSocketReq.Payload, req)); err != nil {
			resp.Error = wsError(err)
		} else if res, err := endpt.f(req); err != nil {
			resp.Error = wsError(err)
		} else if resBytes, err := er.E1(jsoniter.Marshal(res)); err != nil {
			resp.Error = wsError(err)
		} else {
			resp.Payload = resBytes
		}
	}

	respPayload, err := jsoniter.Marshal(&resp)
	if err != nil {
		log.Errorf("Unable to marshal response to req: [%s]: [%s]", webSocketReq.RequestId, err)
		return
	}
	//	write the result message to the webSocket client
	err = (*websocket.Conn)(conn).WriteMessage(websocket.TextMessage, respPayload)
	if err != nil {
		log.Errorf("Cannot write error message to webSocket client: [%s]", err)
	}
}

func (conn *websocketConn) handleProtobufMessage(ctx *Apiv1, req []byte) {
	//	unmarshal the request message
	var webSocketReq rest_pb.WebSocketProtobufRequest

	if err := proto.Unmarshal(req, &webSocketReq); err != nil {
		conn.errorClose(er.Errorf("Cannot parse websocket proto message: [%s]", err))
		return
	}

	resp := rest_pb.WebSocketProtobufResponse{
		RequestId: webSocketReq.RequestId,
		HasMore:   false,
		Payload:   nil,
	}
	var endpt *endpoint
	ctx.internal.funcs.R().In(func(funcs *map[string]*endpoint) er.R {
		if ep, ok := (*funcs)[webSocketReq.Endpoint]; ok {
			endpt = ep
		}
		return nil
	})
	if endpt == nil {
		resp.Payload = pWsError(er.Errorf("No such endpoint: [%s]", webSocketReq.Endpoint))
	} else {
		req := endpt.mkReq()
		if err := er.E(webSocketReq.Payload.UnmarshalTo(req)); err != nil {
			resp.Payload = pWsError(err)
		} else if res, err := endpt.f(req); err != nil {
			resp.Payload = pWsError(err)
		} else if resBytes, err := er.E1(jsoniter.Marshal(res)); err != nil {
			resp.Payload = pWsError(err)
		} else {
			resp.Payload = &rest_pb.WebSocketProtobufResponse_Ok{
				Ok: &anypb.Any{
					TypeUrl: "github.com/pkt-cash/pktd/lnd/" + reflect.TypeOf(res).String()[1:],
					Value:   resBytes,
				},
			}
		}
	}

	if respPayload, err := proto.Marshal(&resp); err != nil {
		log.Errorf("Unable to marshal response to req: [%s]: [%s]", webSocketReq.RequestId, err)
	} else if err := (*websocket.Conn)(conn).WriteMessage(websocket.TextMessage, respPayload); err != nil {
		log.Errorf("Cannot write error message to webSocket client: [%s]", err)
	}
}
