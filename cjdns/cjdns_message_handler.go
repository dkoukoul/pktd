package cjdns

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/pktlog/log"
	"github.com/zeebo/bencode"
)

const (
	ContentType_IP6_IP       = 0
	ContentType_IP6_ICMP     = 1
	ContentType_IP6_IGMP     = 2
	ContentType_IP6_IPIP     = 4
	ContentType_IP6_TCP      = 6
	ContentType_IP6_EGP      = 8
	ContentType_IP6_PUP      = 12
	ContentType_IP6_UDP      = 17
	ContentType_IP6_IDP      = 22
	ContentType_IP6_TP       = 29
	ContentType_IP6_DCCP     = 33
	ContentType_IP6_IPV6     = 41
	ContentType_IP6_RSVP     = 46
	ContentType_IP6_GRE      = 47
	ContentType_IP6_ESP      = 50
	ContentType_IP6_AH       = 51
	ContentType_IP6_MTP      = 92
	ContentType_IP6_BEETPH   = 94
	ContentType_IP6_ENCAP    = 98
	ContentType_IP6_PIM      = 103
	ContentType_IP6_COMP     = 108
	ContentType_IP6_SCTP     = 132
	ContentType_IP6_UDPLITE  = 136
	ContentType_IP6_RAW      = 255
	ContentType_CJDHT        = 256
	ContentType_IPTUN        = 257
	ContentType_RESERVED     = 258
	ContentType_RESERVED_MAX = 0x7fff
	ContentType_AVAILABLE    = 0x8000
	ContentType_CTRL         = 0xffff + 1
	ContentType_MAX          = 0xffff + 2
)

type Cjdns struct {
	SocketPath    string
	Socket        net.Conn
	Device        string
	IPv6          string
	ListeningPort int
	ListenConn    *net.UDPConn
}

type InvoiceRequest struct {
	CjdnsAddr   string
	CjdnsPubKey string
	LndPubkey   string
	Txid        string
	Amount      uint64
}

type InvoiceResponse struct {
	RHash          []byte `json:"rHash"`
	PaymentRequest string `json:"paymentRequest"`
	AddIndex       string `json:"addIndex"`
	Txid           string `json:"txid"`
	Error          string `json:"error"`
}

var cjdns Cjdns
var CjdnsInvoiceResponse InvoiceResponse

func listHandlers() ([]int, error) {
	message := map[string]interface{}{
		"q":    "UpperDistributor_listHandlers",
		"page": 0,
	}

	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		return nil, err
	}
	_, err = cjdns.Socket.Write(bytes)
	if err != nil {
		return nil, err
	}
	cjdns.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1024)
	resultChan := make(chan []int, 1)
	var response map[string]interface{}
	go func(resultChan chan<- []int) {
		n, err := cjdns.Socket.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			err := bencode.DecodeBytes(buf, &response)
			if err != nil {
				log.Error("Error decoding response: %s", err)
				return
			}
			if handlers, ok := response["handlers"].(interface{}); ok {
				udpPorts := []int{}
				for _, handler := range handlers.([]interface{}) {
					udpPort := handler.(map[string]interface{})["udpPort"].(int64)
					udpPorts = append(udpPorts, int(udpPort))
				}
				resultChan <- udpPorts
			}
		}
	}(resultChan)

	return <-resultChan, nil
}

// Connect to CJDNS socket
func Initialize(socket string) error {
	//TODO: check if cjdns is running and if tun0 is up

	cjdns.Device = "tun0"
	ipv6, err := getDeviceAddr(cjdns.Device)
	if err != nil {
		log.Errorf("Error getting CJDNS address:", err)
		return err
	} else {
		cjdns.IPv6 = ipv6
	}
	cjdns.SocketPath = socket
	conn, err := net.Dial("unix", cjdns.SocketPath)
	if err != nil {
		log.Errorf("Error connecting to CJDNS socket:", err)
		return err
	}
	cjdns.Socket = conn
	udpPorts, _ := listHandlers()
	for _, port := range udpPorts {
		unregisterHandler(port)
		time.Sleep(1 * time.Second)
	}
	log.Infof("CJDNS socket initialized successfully")
	return nil
}

// Close CJDNS socket
func Close(ls net.Conn) error {
	err := ls.Close()
	if err != nil {
		return err
	}
	return nil
}

func cjdnsSocketListener() (string, error) {
	log.Infof("cjdnsSocketListener...")
	cjdns.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1024)

	resultChan := make(chan string, 1)

	var response map[string]interface{}
	go func(resultChan chan<- string) {
		n, err := cjdns.Socket.Read(buf)

		if n > 0 {
			err := bencode.DecodeBytes(buf, &response)
			if err != nil {
				log.Errorf("Error decoding response: %s", err)
				resultChan <- err.Error()
			}
			// check if response has "addr" and "ms" fields
			if addr, ok := response["addr"].(string); ok {
				res := addr + " ms:" + fmt.Sprintf("%d", response["ms"].(int64))
				resultChan <- res
				return
			} else if q, ok := response["q"].(string); ok && q == "pong" {
				resultChan <- q
				return
			}
			// check if response has "q" field
			if q, ok := response["q"].(string); ok && q == "UpperDistributor_listHandlers" {
				if response["handlers"] != nil {
					handlers := response["handlers"].([]interface{})
					for _, handler := range handlers {
						if handler.(map[string]interface{})["udpPort"].(int64) == 1 {
							resultChan <- "Handler found"
							return
						}
					}
				}
			}
		}
		if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
			resultChan <- "CJDNS connection timeout"
		} else if err != nil {
			resultChan <- err.Error()
		}
	}(resultChan)

	result := <-resultChan
	fmt.Println("Result:", result)
	return "", nil
}

func registerHandler(contentType int64, udpPort int64) error {
	message := map[string]interface{}{
		"q":    "UpperDistributor_registerHandler",
		"args": map[string]int64{"contentType": contentType, "udpPort": udpPort},
	}
	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		log.Errorf("Error encoding CJDNS message:", err)
		return err
	}
	_, err = cjdns.Socket.Write(bytes)
	if err != nil {
		log.Errorf("Error writing to CJDNS socket:", err)
		return err
	}
	return nil
}

func unregisterHandler(udpPort int) error {
	message := map[string]interface{}{
		"q":    "UpperDistributor_unregisterHandler",
		"args": map[string]int{"udpPort": udpPort},
	}
	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		log.Errorf("Error encoding CJDNS message:", err)
		return err
	}
	_, err = cjdns.Socket.Write(bytes)
	if err != nil {
		log.Errorf("Error writing to CJDNS socket:", err)
		return err
	}
	return nil
}

func getDeviceAddr(device string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Errorf("Error getting interfaces for CJDNS connection:", err)
		return "", err
	}
	deviceAddr := ""
	for _, iface := range ifaces {
		if iface.Name == device {
			addrs, err := iface.Addrs()
			if err != nil {
				log.Errorf("Error getting CJDNS addresses at tun0:", err)
				return "", err
			}

			for _, addr := range addrs {
				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil {
					log.Errorf("Error parsing CIDR:", err)
					return "", err
				}
				deviceAddr = ip.String()
				break
			}
		}
		if deviceAddr != "" {
			return deviceAddr, nil
		}
	}
	return "", errors.New("device not found")
}

func getSendConn() (*net.UDPConn, error) {
	// use this to send a packet to cjdns throught tun0
	rAddr, err := net.ResolveUDPAddr("udp", "[fc00::1]:1")
	if err != nil {
		log.Errorf("Error resolving UDP address:", err)
		return nil, err
	}
	if err != nil {
		log.Errorf("Error getting device address:", err)
		return nil, err
	}
	//bind to local address (tun0) and a port, then register that port to cjdns
	sAddr := &net.UDPAddr{IP: net.ParseIP(cjdns.IPv6), Port: 1}
	conn, err := net.DialUDP("udp", sAddr, rAddr)

	registerHandler(ContentType_RESERVED, int64(sAddr.Port))
	if err != nil {
		log.Errorf("Error dialing UDP address:", err)
		return nil, err
	}

	return conn, nil
}

func SendCjdnsInvoiceResponse(request InvoiceRequest, response *rpc_pb.AddInvoiceResponse, restError *rpc_pb.RestError) error {
	var data []byte
	if restError != nil {
		data = createErrorResponse(request.CjdnsAddr, request.CjdnsPubKey, restError.Message, request.Txid)
		//TODO: handle error messages in ListeningsCjdnsMessages
	} else {
		invoice := InvoiceResponse{
			RHash:          response.RHash,
			PaymentRequest: response.PaymentRequest,
			AddIndex:       strconv.Itoa(int(response.AddIndex)),
		}
		data = createInvoiceResponse(request.CjdnsAddr, request.CjdnsPubKey, invoice, request.Txid)
	}
	conn, err := getSendConn()
	if err != nil {
		log.Errorf("Error getting CJDNS UDP connection:", err)
		return err
	}

	// Send data
	_, err = conn.Write(data)
	if err != nil {
		log.Errorf("Error sending CJDNS UDP packet:", err)
		return err
	}

	log.Infof("CJDNS Invoice Response sent successfully to: ", request.CjdnsAddr)
	defer conn.Close()
	return nil
}

func SendCjdnsInvoiceRequest(cjdns_addr string, cjdns_pubkey string, lnd_pubkey string, amount uint64) (string, error) {
	log.Infof("SendCjdnsInvoiceRequest: %s, %d", cjdns_addr, amount)
	conn, err := getSendConn()
	if err != nil {
		log.Errorf("Error getting UDP connection:", err)
		return "", err
	}
	// Data to send
	if cjdns_addr == "" {
		//TODO: retrieve pubkey from ping
	}
	data, txid := createInvoiceRequest(cjdns_addr, cjdns_pubkey, lnd_pubkey, amount)
	log.Infof("createInvoiceRequest: %s", data)
	// Send data
	_, err = conn.Write(data)
	if err != nil {
		log.Errorf("Error sending UDP packet:", err)
		return "", err
	}

	log.Infof("CJDNS Invoice Request sent successfully to: ", cjdns_addr)
	defer conn.Close()
	return txid, nil
}

func ListeningForInvoiceRequest() (chan InvoiceRequest, error) {
	// use this to read a packet to cjdns throught tun0
	invoiceRequestChan := make(chan InvoiceRequest)
	rAddr, err := net.ResolveUDPAddr("udp", "["+cjdns.IPv6+"]:0")
	if err != nil {
		log.Errorf("Error resolving UDP address:", err)
		return invoiceRequestChan, err
	}

	//bind to local address (tun0) and a port, then register that port to cjdns
	listenPort := 0

	if cjdns.ListeningPort != 0 {
		listenPort = cjdns.ListeningPort
	}
	if cjdns.ListenConn == nil {
		cjdns.ListenConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(cjdns.IPv6), Port: listenPort})
		if err != nil {
			log.Errorf("Error listening UDP address:", err)
			return invoiceRequestChan, err
		}
		localAddr := cjdns.ListenConn.LocalAddr().(*net.UDPAddr)
		log.Infof("Listening for CJDNS messages at address:", cjdns.IPv6, ":", localAddr.Port)
		cjdns.ListeningPort = localAddr.Port
		registerHandler(ContentType_RESERVED, int64(localAddr.Port))
	}
	buf := make([]byte, 1024)

	go func() {
		for {
			n, _, err := cjdns.ListenConn.ReadFromUDP(buf)
			if err != nil {
				log.Errorf("Error reading from UDP port %d: %v\n", rAddr.Port, err)
				continue
			}

			message, err := decode(buf[:n])
			if err != nil {
				log.Errorf("Error decoding message: %v", err)
				continue
			}
			if message.DataHeader.ContentType == uint16(ContentType_RESERVED) {
				// fmt.Println("Received RESERVED message")
				if message.ContentBenc != nil {
					// Invoice Request
					if message.ContentBenc.(map[string]interface{})["q"] != nil && message.ContentBenc.(map[string]interface{})["q"].(string) == "invoice_req" {
						// fmt.Println("Received invoice request")

						invoiceRequest := InvoiceRequest{
							CjdnsAddr:   message.RouteHeader.IP.String(),
							CjdnsPubKey: message.RouteHeader.PublicKey,
							LndPubkey:   message.ContentBenc.(map[string]interface{})["lndpubkey"].(string),
							Txid:        message.ContentBenc.(map[string]interface{})["txid"].(string),
							Amount:      uint64(message.ContentBenc.(map[string]interface{})["amt"].(int64)),
						}
						invoiceRequestChan <- invoiceRequest
						// Invoice Response Error
					} else if message.ContentBenc.(map[string]interface{})["error"] != nil {
						// fmt.Println("Received error message")
						// fmt.Println("Error:", message.ContentBenc.(map[string]interface{})["error"].(map[string]interface{})["message"].(string))
						CjdnsInvoiceResponse.Error = message.ContentBenc.(map[string]interface{})["error"].(map[string]interface{})["message"].(string)
						CjdnsInvoiceResponse.Txid = message.ContentBenc.(map[string]interface{})["txid"].(string)
						// Invoice Response
					} else if message.ContentBenc.(map[string]interface{})["invoice"] != nil {
						printError := false
						txid, ok := message.ContentBenc.(map[string]interface{})["txid"].(string)
						if ok {
							CjdnsInvoiceResponse.Txid = txid
						} else {
							log.Errorf("Error getting txid from CJDNS invoice response.")
							printError = true
						}
						paymentRequest, ok := message.ContentBenc.(map[string]interface{})["invoice"].(map[string]interface{})["paymentRequest"].(string)
						if ok {
							CjdnsInvoiceResponse.PaymentRequest = paymentRequest
						} else {
							log.Errorf("Error getting paymentRequest from CJDNS invoice response.")
							printError = true
						}
						index, ok := message.ContentBenc.(map[string]interface{})["invoice"].(map[string]interface{})["addIndex"].(string)
						if ok {
							CjdnsInvoiceResponse.AddIndex = index
						} else {
							log.Errorf("Error getting addIndex from CJDNS invoice response.")
							printError = true
						}
						rHash, ok := message.ContentBenc.(map[string]interface{})["invoice"].(map[string]interface{})["rHash"]
						if ok {
							CjdnsInvoiceResponse.RHash = []byte(rHash.(string)) 
						} else {
							log.Errorf("Error getting rHash from CJDNS invoice response.")
							printError = true
						}
						if printError {
							log.Errorf("CJDNS invoice response: %v", message.ContentBenc)
						}
					}
				} else {
					//TODO: handle unknown types
					log.Warn("Received CJDNS invoice response with unknown format")
					log.Warnf("Message.ContentBenc:", message.ContentBenc)
					CjdnsInvoiceResponse.Txid = message.ContentBenc.(map[string]interface{})["txid"].(string)
				}
			}
		}
	}()

	return invoiceRequestChan, nil
}

func generateRandomNumber() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(9000000000) + 1000000000
}

func createErrorResponse(receiverIP string, receiverPubkey string, errorMsg string, txid string) []byte {
	// coinType := []byte{0x80, 0x00, 0x01, 0x86}
	msg := map[string]interface{}{
		"error": map[string]string{
			"message": errorMsg,
		},
		"txid": txid,
	}
	payload := encodeMsg(msg, receiverIP, receiverPubkey)
	return payload
}

func createInvoiceResponse(receiverIP string, receiverPubkey string, invoice InvoiceResponse, txid string) []byte {
	msg := map[string]interface{}{
		"invoice": map[string]interface{}{
			"rHash":          invoice.RHash,
			"paymentRequest": invoice.PaymentRequest,
			"addIndex":       invoice.AddIndex,
		},
		"txid": txid,
	}
	payload := encodeMsg(msg, receiverIP, receiverPubkey)
	return payload
}

func encodeMsg(msg map[string]interface{}, ip string, pubkey string) []byte {
	coinType := []byte{0x80, 0x00, 0x01, 0x86}
	var bytesMessage []byte = nil
	bytesMessage = append(bytesMessage, coinType...)
	encodedMsg, err := bencode.EncodeBytes(msg)
	if err != nil {
		log.Errorf("Error bencoding message:", err)
	}
	bytesMessage = append(bytesMessage, encodedMsg...)
	var cjdnsip net.IP = net.ParseIP(ip)
	var message Message = Message{
		RouteHeader: RouteHeader{
			PublicKey: pubkey,
			Version:   22,
			IP:        cjdnsip,
			SwitchHeader: SwitchHeader{
				Label:   "0000",
				Version: 1,
			},
			IsIncoming: false,
			IsCtrl:     false,
		},
		DataHeader: DataHeader{
			ContentType: ContentType_RESERVED,
			Version:     1,
		},
		ContentBytes: bytesMessage,
		RawBytes:     nil,
		ContentBenc:  msg,
		Content:      nil,
	}

	payload, err := message.encode()
	if err != nil {
		log.Errorf("Error encoding CJDNS message:", err)
	}
	return payload
}

func createInvoiceRequest(receiverIP string, receiverPubkey string, lnd_pubkey string, amount uint64) ([]byte, string) {
	if receiverIP == "" {
		log.Errorf("Can not create CJDNS invoice request, IP is empty")
		return nil, ""
	}
	random := generateRandomNumber()
	requests := 1
	txid := strconv.Itoa(random) + "/" + strconv.Itoa(requests)
	msg := map[string]interface{}{
		"q":         "invoice_req",
		"amt":       amount,
		"lndpubkey": lnd_pubkey,
		"txid":      txid,
	}
	payload := encodeMsg(msg, receiverIP, receiverPubkey)
	return payload, txid
}

func Ping(node string) (string, error) {
	log.Infof("Ping Cjdns node:", node)
	err := error(nil)
	if cjdns.Socket == nil {
		return "", errors.New("CJDNS connection is nil")
	}

	ping := map[string]string{
		"q": "ping",
	}
	pingNode := map[string]interface{}{
		"q":    "RouterModule_pingNode",
		"args": map[string]string{"path": "fc93:1145:f24c:ee59:4a09:288e:ada8:0901"},
	}
	var bytes []byte
	if node != "" {
		bytes, err = bencode.EncodeBytes(pingNode)
		if err != nil {
			return "", err
		}
	} else {
		bytes, err = bencode.EncodeBytes(ping)
		if err != nil {
			return "", err
		}
	}

	_, err = cjdns.Socket.Write(bytes)
	if err != nil {
		return "", err
	}
	data, err := pingListener()
	if err != nil {
		return "", err
	}

	return data, nil
}

func pingListener() (string, error) {
	cjdns.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1024)
	n, err := cjdns.Socket.Read(buf)

	var response map[string]interface{}
	if n > 0 {
		err := bencode.DecodeBytes(buf, &response)
		if err != nil {
			return "", err
		}
		// check if response has "addr" and "ms" fields
		if addr, ok := response["addr"].(string); ok {
			res := addr + " ms:" + fmt.Sprintf("%d", response["ms"].(int64))
			return res, nil
		} else if q, ok := response["q"].(string); ok && q == "pong" {
			return q, nil
		}
		return "", nil
	}
	if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
		return "", errors.New("CJDNS connection timeout")
	} else if err != nil {
		return "", err
	}
	return "", nil
}
