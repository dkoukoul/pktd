package cjdns

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/lnd/lnrpc/apiv1"
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
	api           apiv1.Apiv1
}

type LndPubkeyRequest struct {
	CjdnsAddr   string
	CjdnsPubKey string
	Txid        string
}

type LndPubkeyResponse struct {
	lndPubkey string
	Txid      string
}
type InvoiceRequest struct {
	CjdnsAddr   string
	CjdnsPubKey string
	Txid        string
	Amount      uint64
}

type CjdnsNode struct {
	CjdnsAddr   string
	CjdnsPubKey string
	lndPubkey   string
}

type InvoiceResponse struct {
	RHash          []byte `json:"rHash"`
	PaymentRequest string `json:"paymentRequest"`
	AddIndex       string `json:"addIndex"`
	Txid           string `json:"txid"`
	Error          string `json:"error"`
}

type CjdnsPeer struct {
	addr   string
	lladdr string
	state  string
}

var CjdnsInvoiceResponse InvoiceResponse
var lndPubkeyResponse LndPubkeyResponse

type lndRpcServer interface {
	LndListPeers(ctx context.Context, in *rpc_pb.ListPeersRequest) (*rpc_pb.ListPeersResponse, er.R)
	LndConnectPeer(ctx context.Context, in *rpc_pb.ConnectPeerRequest) (*rpc_pb.Null, er.R)
	LndAddInvoice(ctx context.Context, in *rpc_pb.Invoice) (*rpc_pb.AddInvoiceResponse, er.R)
	LndIdentityPubkey() string
}

func (c *Cjdns) registerRpc() {
	cjdnsCategory := c.api.Category("cjdns")
	apiv1.Endpoint(
		cjdnsCategory,
		"ping",
		`
		Ping a cjdns node.
		`,
		func(req *rpc_pb.CjdnsPingRequest) (*rpc_pb.CjdnsPingResponse, er.R) {
			return c.PingCjdns(req)
		},
	)
	apiv1.Endpoint(
		cjdnsCategory,
		"requestinvoice",
		`
		Request a payment invoice using a cjdns address.
		`,
		func(req *rpc_pb.CjdnsPaymentInvoiceRequest) (*rpc_pb.CjdnsPaymentInvoiceResponse, er.R) {
			return c.CjdnsInvoiceRequest(req)
		},
	)
}

func (c *Cjdns) PingCjdns(req *rpc_pb.CjdnsPingRequest) (*rpc_pb.CjdnsPingResponse, er.R) {
	// if !isValidIPv6(req.CjdnsAddr) {
	// 	return nil, er.New("Invalid CJDNS address")
	// }
	// if r.cfg.CjdnsSocket == "" {
	// 	return nil, er.New("Cjdns socket not found")
	// }
	res, pingerr := c.Ping(req.CjdnsAddr)
	if pingerr != nil {
		return nil, er.New("Cjdns ping error")
	}

	return &rpc_pb.CjdnsPingResponse{
		Pong: res,
	}, nil
}

func (c *Cjdns) CjdnsInvoiceRequest(req *rpc_pb.CjdnsPaymentInvoiceRequest) (*rpc_pb.CjdnsPaymentInvoiceResponse, er.R) {
	// if r.cfg.CjdnsSocket == "" {
	// 	return nil, er.New("Cjdns socket not found")
	// }
	//TODO: check for cjdns initialization
	// err := cjdns.Initialize(r.cfg.CjdnsSocket)
	// if err != nil {
	// 	return nil, er.New("Cjdns socket error")
	// }
	//TODO: remove lndpubkey from request
	// should retreive it from lnd
	// idPub := cjdns.Server.identityECDH.PubKey().SerializeCompressed()
	// idPubHex := hex.EncodeToString(idPub)
	txid, err := c.SendCjdnsInvoiceRequest(req.CjdnsAddr, req.CjdnsPubkey, req.Amount)
	if err != nil {
		return nil, er.New("Cjdns invoice request error")
	}
	// Wait 10 seconds for cjdns response with same txid
	response := make(chan InvoiceResponse)
	start := time.Now()
	closed := false
	go func() {
		for {
			time.Sleep(1 * time.Second)
			if CjdnsInvoiceResponse.Txid == txid {
				log.Infof("Cjdns invoice response received")
				respCopy := CjdnsInvoiceResponse
				response <- respCopy
				CjdnsInvoiceResponse = InvoiceResponse{}
				if !closed {
					close(response)
					closed = true
				}
				return
			}
			if time.Since(start) > 30*time.Second {
				log.Infof("Cjdns invoice response timeout")
				response <- InvoiceResponse{
					Error: "Cjdns invoice response timeout",
				}
				CjdnsInvoiceResponse = InvoiceResponse{}
				if !closed {
					close(response)
					closed = true
				}
				return
			}
		}
	}()
	res := <-response
	if res.Error != "" {
		log.Infof("Cjdns invoice response error: ", res.Error)
		return nil, er.New(res.Error)
	} else {
		return &rpc_pb.CjdnsPaymentInvoiceResponse{
			RHash:          res.RHash,
			PaymentRequest: res.PaymentRequest,
		}, nil
	}
}

func NewCjdnsHandler(socket string, api *apiv1.Apiv1) (*Cjdns, er.R) {
	log.Infof("Starting CJDNS message handler...")
	//TODO: check if cjdns is running and if tun0 is up
	device := "tun0"
	ipv6, err := getDeviceAddr(device)
	if err != nil {
		return nil, er.New("Error getting CJDNS address at device tun0")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, er.New("Error connecting to CJDNS socket")
	}

	log.Infof("CJDNS socket initialized successfully")
	cjdns := Cjdns{
		Device:     device,
		IPv6:       ipv6,
		SocketPath: socket,
		Socket:     conn,
		api:        *api,
	}

	return &cjdns, nil
}

func (c *Cjdns) Start(lnd lndRpcServer) er.R {
	// Unregister all handlers
	udpPorts, _ := c.ListHandlers()
	for _, port := range udpPorts {
		c.UnregisterHandler(port)
		time.Sleep(1 * time.Second)
	}
	c.registerRpc()
	listPeersResponse, err := lnd.LndListPeers(context.TODO(), &rpc_pb.ListPeersRequest{})
	//TODO: Start listening for cjdns messages
	if err != nil {
		log.Errorf("Error listing lnd peers: %v", err)
	}
	//TESTING: ONLY from developer's host, will try to connect to hardcoded lnd peer
	hostname, _ := os.Hostname()
	if hostname == "x1" {
		if len(listPeersResponse.Peers) == 0 {
			log.Infof("No current lnd peer connection, getting nodes...")
			//TODO: read cjdns addres from cjdns peers
			// lndhost := "[fce3:86e9:b183:1a06:ad9a:c37f:14fe:36c2]:9735"
			lndhost := "192.168.1.12:9735"
			//TODO: get lnd identity pubkey
			lndPubkey := "037a44193419b4e58eb43607a521dc1da3bb262019c5ab565e7f4224714f1b5695"
			lndAddress := rpc_pb.LightningAddress{
				Pubkey: lndPubkey,
				Host:   lndhost,
			}
			connectRequest := &rpc_pb.ConnectPeerRequest{
				Addr: &lndAddress,
			}
			log.Infof("Connecting to LND peer: %s", connectRequest.Addr.Host)
			for {
				_, err := lnd.LndConnectPeer(context.TODO(), connectRequest)
				if err != nil {
					log.Warnf("Error connecting to LND peer: %s", err.Message())
				} else {
					break
				}
				time.Sleep(time.Second * 5)
			}
			// nodes := c.GetNodes()
			// for _,node := range nodes {
			// 	fmt.Println("Connect LND to CJDNS node: ", node)
			// 	lndAddress := rpc_pb.LightningAddress{
			// 		Pubkey: node.CjdnsPubKey,
			// 		Host:   node.CjdnsAddr,
			// 	}
			// 	connectRequest := &rpc_pb.ConnectPeerRequest{
			// 		Addr: &lndAddress,
			// 	}
			// 	log.Infof("Connecting to LND peer: ", connectRequest.Addr.Host)
			// 	for {
			// 		_, err := lnd.LndConnectPeer(context.TODO(), connectRequest)
			// 		if err != nil {
			// 			log.Warnf("Error connecting to LND peer: ", err)
			// 		} else {
			// 			break
			// 		}
			// 		time.Sleep(time.Second * 5)
			// 	}
			// }
		}
	}

	for {
		// invoiceRequestchan, lndPubkeychan, err := c.ListenCjdnsMessages()
		invoiceRequestchan, err := c.ListenCjdnsMessages()
		request := <-invoiceRequestchan
		// lndPubkeyRequest := <-lndPubkeychan
		if err != nil {
			log.Warnf("Error listening for CJDNS invoice request: %v", err)
		}
		if request.CjdnsAddr != "" {
			// log.Infof("CJDNS invoice request: %v", request)
			errorInvoiceResponse := &rpc_pb.RestError{
				Message: "Error creating invoice",
			}
			// log.Infof("Listing lnd peers...")
			listPeersResponse, err := lnd.LndListPeers(context.TODO(), &rpc_pb.ListPeersRequest{})
			if err != nil {
				log.Errorf("Error listing lnd peers: %v", err)
			}
			if len(listPeersResponse.Peers) == 0 {
				errorInvoiceResponse = &rpc_pb.RestError{
					Message: "No lnd peer connection",
				}
				c.SendCjdnsInvoiceResponse(request, &rpc_pb.AddInvoiceResponse{}, errorInvoiceResponse)
			} else {
				invoice := &rpc_pb.Invoice{
					Value: int64(request.Amount),
				}
				// log.Infof("Creating lnd invoice with: %v", invoice)
				invoiceResponse, errr := lnd.LndAddInvoice(context.TODO(), invoice)
				if errr != nil {
					log.Errorf("Error adding invoice: %v", errr)
				}
				// log.Infof("Lnd Invoice response: %v", invoiceResponse)
				errrr := c.SendCjdnsInvoiceResponse(request, invoiceResponse, nil)
				if errrr != nil {
					log.Errorf("Error sending CJDNS invoice response: %v", err)
				}
			}
		}
		// } else if lndPubkeyRequest.CjdnsAddr != "" {
		// 	log.Infof("CJDNS lnd pubkey request: %s", lndPubkeyRequest.CjdnsAddr)
		// 	//TODO: get lnd identity pubkey
		// 	idPubHex := lnd.LndIdentityPubkey()
		// 	c.SendLndPubkeyResponse(idPubHex, lndPubkeyRequest)
		// }
	}

	return nil
}

func (c *Cjdns) Stop() er.R {
	//TODO: unregister handlers
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

// func cjdnsSocketListener() (string, error) {
// 	log.Infof("cjdnsSocketListener...")
// 	cjdns.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
// 	buf := make([]byte, 1024)

// 	resultChan := make(chan string, 1)

// 	var response map[string]interface{}
// 	go func(resultChan chan<- string) {
// 		n, err := cjdns.Socket.Read(buf)

// 		if n > 0 {
// 			err := bencode.DecodeBytes(buf, &response)
// 			if err != nil {
// 				log.Errorf("Error decoding response: %s", err)
// 				resultChan <- err.Error()
// 			}
// 			// check if response has "addr" and "ms" fields
// 			if addr, ok := response["addr"].(string); ok {
// 				res := addr + " ms:" + fmt.Sprintf("%d", response["ms"].(int64))
// 				resultChan <- res
// 				return
// 			} else if q, ok := response["q"].(string); ok && q == "pong" {
// 				resultChan <- q
// 				return
// 			}
// 			// check if response has "q" field
// 			if q, ok := response["q"].(string); ok && q == "UpperDistributor_listHandlers" {
// 				if response["handlers"] != nil {
// 					handlers := response["handlers"].([]interface{})
// 					for _, handler := range handlers {
// 						if handler.(map[string]interface{})["udpPort"].(int64) == 1 {
// 							resultChan <- "Handler found"
// 							return
// 						}
// 					}
// 				}
// 			}
// 		}
// 		if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
// 			resultChan <- "CJDNS connection timeout"
// 		} else if err != nil {
// 			resultChan <- err.Error()
// 		}
// 	}(resultChan)

// 	result := <-resultChan
// 	fmt.Println("Result:", result)
// 	return "", nil
// }

func getDeviceAddr(device string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Errorf("Error getting interfaces for CJDNS connection: %v", err)
		return "", err
	}
	deviceAddr := ""
	for _, iface := range ifaces {
		if iface.Name == device {
			addrs, err := iface.Addrs()
			if err != nil {
				log.Errorf("Error getting CJDNS addresses at tun0: %v", err)
				return "", err
			}

			for _, addr := range addrs {
				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil {
					log.Errorf("Error parsing CIDR: %v", err)
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

func (c *Cjdns) getSendConn() (*net.UDPConn, error) {
	// use this to send a packet to cjdns throught tun0
	rAddr, err := net.ResolveUDPAddr("udp", "[fc00::1]:1")
	if err != nil {
		log.Errorf("Error resolving UDP address: %v", err)
		return nil, err
	}
	if err != nil {
		log.Errorf("Error getting device address: %v", err)
		return nil, err
	}
	//bind to local address (tun0) and a port, then register that port to cjdns
	sAddr := &net.UDPAddr{IP: net.ParseIP(c.IPv6), Port: 1}
	conn, err := net.DialUDP("udp", sAddr, rAddr)

	c.RegisterHandler(ContentType_RESERVED, int64(sAddr.Port))
	if err != nil {
		log.Errorf("Error dialing UDP address: %v", err)
		return nil, err
	}

	return conn, nil
}

func (c *Cjdns) GetNodes() []CjdnsNode {
	log.Infof("Get CJDNS Nodes...")
	//Get cjdns nodes
	peers, err := c.PeerStats()
	if err != nil {
		log.Errorf("Error getting CJDNS peer stats: %v", err)
	}
	log.Infof("Cjdns peer stats: %v", peers)
	nodes := []CjdnsNode{}
	// Get cjdns nodes routes from dumpTables, in order to get cjdns IPv6 address
	nodeRoutes, err := c.NodeStore_dumpTable()
	if err != nil {
		log.Errorf("Error getting CJDNS node routes: %v", err)
	}
	//Query them one by one for lnd pubkey
	for _, peer := range peers {
		parts := strings.Split(peer.addr, ".")
		pubkey := strings.Join(parts[len(parts)-2:], ".")

		cjdnsIpv6 := ""
		for _, nodeRoute := range nodeRoutes {
			if nodeRoute.CjdnsPubKey == peer.addr {
				cjdnsIpv6 = nodeRoute.CjdnsAddr
			}
		}

		node := CjdnsNode{
			CjdnsAddr:   cjdnsIpv6,
			CjdnsPubKey: pubkey,
			lndPubkey:   "",
		}
		log.Infof("Cjdns node: %v", node)
		nodes = append(nodes, node)
	}
	for _, node := range nodes {
		log.Infof("Sending Cjdns lnd pubkey query to: %v %v", node.CjdnsAddr, node.CjdnsPubKey)
		lndpubkey, err := c.sendLndPubkeyQuery(node.CjdnsAddr, node.CjdnsPubKey)

		if err != nil {
			log.Errorf("Error sending CJDNS lnd pubkey query: %v", err)
		}
		log.Infof("Lnd pubkey: %s", lndpubkey)
		node.lndPubkey = lndpubkey
	}
	return nodes
}

func (c *Cjdns) sendLndPubkeyQuery(cjdnsNodeIp string, cjdnsNodePubkey string) (string, error) {
	conn, err := c.getSendConn()
	if err != nil {
		log.Errorf("Error getting CJDNS UDP connection:", err)
		return "", err
	}
	data, txid := createLndQueryRequest(cjdnsNodeIp, cjdnsNodePubkey)
	// Send data
	_, err = conn.Write(data)
	if err != nil {
		log.Errorf("Error sending CJDNS UDP packet:", err)
		return "", err
	}
	// Wait 10 seconds for cjdns response with same txid
	response := make(chan string)
	start := time.Now()
	closed := false
	go func() {
		for {
			time.Sleep(1 * time.Second)
			if lndPubkeyResponse.Txid == txid {
				log.Infof("Cjdns lnd pubkey response received")
				respCopy := lndPubkeyResponse
				response <- respCopy.lndPubkey
				lndPubkeyResponse = LndPubkeyResponse{}
				if !closed {
					close(response)
					closed = true
				}
			}
			// if CjdnsInvoiceResponse.Txid == txid {
			// 	fmt.Println("Cjdns invoice response received")
			// 	respCopy := CjdnsInvoiceResponse
			// 	response <- respCopy
			// 	CjdnsInvoiceResponse = InvoiceResponse{}
			// 	if !closed {
			// 		close(response)
			// 		closed = true
			// 	}
			// 	return
			// }
			if time.Since(start) > 10*time.Second {
				if !closed {
					close(response)
					closed = true
				}
				return
			}
		}
	}()
	res := <-response

	defer conn.Close()
	return res, nil
}

func (c *Cjdns) SendLndPubkeyResponse(lndPubkey string, req LndPubkeyRequest) error {
	conn, err := c.getSendConn()
	if err != nil {
		log.Errorf("Error getting CJDNS UDP connection: %v", err)
		return err
	}
	data := createLndQueryResponse(req.CjdnsAddr, req.CjdnsPubKey, lndPubkey, req.Txid)
	// Send data
	_, err = conn.Write(data)
	if err != nil {
		log.Errorf("Error sending CJDNS UDP packet: %v", err)
		return err
	}

	log.Infof("CJDNS LND Pubkey query sent successfully to: %s", req.CjdnsAddr)
	defer conn.Close()
	return nil
}

func (c *Cjdns) SendCjdnsInvoiceResponse(request InvoiceRequest, response *rpc_pb.AddInvoiceResponse, restError *rpc_pb.RestError) error {
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
	conn, err := c.getSendConn()
	if err != nil {
		log.Errorf("Error getting CJDNS UDP connection: %v", err)
		return err
	}

	// Send data
	_, err = conn.Write(data)
	if err != nil {
		log.Errorf("Error sending CJDNS UDP packet: %v", err)
		return err
	}

	log.Infof("CJDNS Invoice Response sent successfully to: %s", request.CjdnsAddr)
	defer conn.Close()
	return nil
}

func (c *Cjdns) SendCjdnsInvoiceRequest(cjdns_addr string, cjdns_pubkey string, amount uint64) (string, error) {
	log.Infof("SendCjdnsInvoiceRequest: %s, %d", cjdns_addr, amount)
	conn, err := c.getSendConn()
	if err != nil {
		log.Errorf("Error getting UDP connection: %v", err)
		return "", err
	}
	// Data to send
	if cjdns_addr == "" {
		//TODO: retrieve pubkey from ping
	}
	data, txid := createInvoiceRequest(cjdns_addr, cjdns_pubkey, amount)

	// Send data
	_, err = conn.Write(data)
	if err != nil {
		log.Errorf("Error sending UDP packet: %v", err)
		return "", err
	}

	log.Infof("CJDNS Invoice Request sent successfully to: %s", cjdns_addr)
	defer conn.Close()
	return txid, nil
}

// func (c *Cjdns) ListenCjdnsMessages() (chan InvoiceRequest, chan LndPubkeyRequest, error) {
func (c *Cjdns) ListenCjdnsMessages() (chan InvoiceRequest, error) {
	// use this to read a packet to cjdns throught tun0
	invoiceRequestChan := make(chan InvoiceRequest)
	// lndPubkeyRequestChan := make(chan LndPubkeyRequest)
	rAddr, err := net.ResolveUDPAddr("udp", "["+c.IPv6+"]:0")
	if err != nil {
		log.Errorf("Error resolving UDP address: %v", err)
		// return invoiceRequestChan, lndPubkeyRequestChan, err
		return invoiceRequestChan, err
	}

	//bind to local address (tun0) and a port, then register that port to cjdns
	listenPort := 0

	if c.ListeningPort != 0 {
		listenPort = c.ListeningPort
	}
	if c.ListenConn == nil {
		c.ListenConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(c.IPv6), Port: listenPort})
		if err != nil {
			log.Errorf("Error listening UDP address: %s", err)
			// return invoiceRequestChan, lndPubkeyRequestChan, err
			return invoiceRequestChan, err
		}
		localAddr := c.ListenConn.LocalAddr().(*net.UDPAddr)
		log.Infof("Listening for CJDNS messages at address: %s:%d", c.IPv6, localAddr.Port)
		c.ListeningPort = localAddr.Port
		c.RegisterHandler(ContentType_RESERVED, int64(localAddr.Port))
	}
	buf := make([]byte, 4096)

	go func() {
		for {
			n, _, err := c.ListenConn.ReadFromUDP(buf)
			if err != nil {
				log.Errorf("Error reading from UDP port %d: %v", rAddr.Port, err)
				continue
			}

			message, err := decode(buf[:n])
			if err != nil {
				log.Errorf("Error decoding message: %v", err)
				continue
			}
			if message.DataHeader.ContentType == uint16(ContentType_RESERVED) {
				// log.Infof("Received CJDNS RESERVED message")
				if message.ContentBenc != nil {
					// Invoice Request
					if message.ContentBenc.(map[string]interface{})["q"] != nil && message.ContentBenc.(map[string]interface{})["q"].(string) == "invoice_req" {
						log.Infof("Received CJDNS invoice request")

						invoiceRequest := InvoiceRequest{
							CjdnsAddr:   message.RouteHeader.IP.String(),
							CjdnsPubKey: message.RouteHeader.PublicKey,
							Txid:        message.ContentBenc.(map[string]interface{})["txid"].(string),
							Amount:      uint64(message.ContentBenc.(map[string]interface{})["amt"].(int64)),
						}
						invoiceRequestChan <- invoiceRequest
						// Invoice Response Error
					} else if message.ContentBenc.(map[string]interface{})["error"] != nil {
						log.Infof("Received CJDNS invoice response error")
						CjdnsInvoiceResponse.Error = message.ContentBenc.(map[string]interface{})["error"].(map[string]interface{})["message"].(string)
						CjdnsInvoiceResponse.Txid = message.ContentBenc.(map[string]interface{})["txid"].(string)
						// Invoice Response
					} else if message.ContentBenc.(map[string]interface{})["invoice"] != nil {
						log.Infof("Received CJDNS invoice response")
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
				} else if message.ContentBenc.(map[string]interface{})["q"] != nil && message.ContentBenc.(map[string]interface{})["q"].(string) == "lnd_pubkey" {
					log.Infof("Received CJDNS lnd pubkey request")
					response := LndPubkeyRequest{
						CjdnsAddr:   message.RouteHeader.IP.String(),
						CjdnsPubKey: message.RouteHeader.PublicKey,
						Txid:        message.ContentBenc.(map[string]interface{})["txid"].(string),
					}
					log.Infof("Sending lnd pubkey response to: %s", response.CjdnsAddr)
					// lndPubkeyRequestChan <- response
				} else if message.ContentBenc.(map[string]interface{})["lnd_pubkey"] != nil {
					log.Infof("Received lnd pubkey response")
					lndPubkey := message.ContentBenc.(map[string]interface{})["lnd_pubkey"].(string)
					log.Infof("Lnd pubkey: %v", lndPubkey)
				} else {
					//TODO: handle unknown types
					log.Warn("Received CJDNS invoice response with unknown format")
					log.Warnf("Message.ContentBenc: %v", message.ContentBenc)
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

func createLndQueryRequest(receiverIP string, receiverPubkey string) ([]byte, string) {
	if receiverIP == "" {
		log.Errorf("Can not create CJDNS lnd pubkey request, IP is empty")
		return nil, ""
	}
	random := generateRandomNumber()
	requests := 1
	txid := strconv.Itoa(random) + "/" + strconv.Itoa(requests)
	msg := map[string]interface{}{
		"q":    "lnd_pubkey",
		"txid": txid,
	}
	payload := encodeMsg(msg, receiverIP, receiverPubkey)
	return payload, txid
}

func createLndQueryResponse(receiverIP string, receiverPubkey string, lndPubkey string, txid string) []byte {
	msg := map[string]interface{}{
		"lnd_pubkey": lndPubkey,
		"txid":       txid,
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
		log.Errorf("Error bencoding message: %s", err)
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
		log.Errorf("Error encoding CJDNS message: %s", err)
	}
	return payload
}

func createInvoiceRequest(receiverIP string, receiverPubkey string, amount uint64) ([]byte, string) {
	if receiverIP == "" {
		log.Errorf("Can not create CJDNS invoice request, IP is empty")
		return nil, ""
	}
	random := generateRandomNumber()
	requests := 1
	txid := strconv.Itoa(random) + "/" + strconv.Itoa(requests)
	msg := map[string]interface{}{
		"q":    "invoice_req",
		"amt":  amount,
		"txid": txid,
	}
	payload := encodeMsg(msg, receiverIP, receiverPubkey)
	return payload, txid
}

func (c *Cjdns) Ping(node string) (string, error) {
	log.Infof("Ping Cjdns node: %v", node)
	err := error(nil)
	if c.Socket == nil {
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

	_, err = c.Socket.Write(bytes)
	if err != nil {
		return "", err
	}

	// Listen for response
	c.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1024)
	n, err := c.Socket.Read(buf)

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
