package cjdns

import (
	"time"

	"github.com/pkt-cash/pktd/pktlog/log"
	"github.com/zeebo/bencode"
)

func (c *Cjdns) ListHandlers() ([]int, error) {
	message := map[string]interface{}{
		"q":    "UpperDistributor_listHandlers",
		"page": 0,
	}

	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		return nil, err
	}
	_, err = c.Socket.Write(bytes)
	if err != nil {
		return nil, err
	}
	c.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1024)
	resultChan := make(chan []int, 1)
	var response map[string]interface{}
	go func(resultChan chan<- []int) {
		n, err := c.Socket.Read(buf)
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


func (c *Cjdns) PeerStats() ([]CjdnsPeer, error) {
	message := map[string]interface{}{
		"q":    "InterfaceController_peerStats",
		"page": 0,
	}

	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		return nil, err
	}

	_, err = c.Socket.Write(bytes)
	if err != nil {
		return nil, err
	}

	c.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 4096)
	resultChan := make(chan []CjdnsPeer)
	var response map[string]interface{}
	go func(resultChan chan<- []CjdnsPeer) {
		n, err := c.Socket.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			err := bencode.DecodeBytes(buf, &response)
			if err != nil {
				log.Errorf("Error decoding response: %v", err)
			}
			if peers, ok := response["peers"].(interface{}); ok {
				cjdnsPeers := []CjdnsPeer{}
				for _, peer := range peers.([]interface{}) {
					cjdnsPeer := CjdnsPeer{
						addr:   peer.(map[string]interface{})["addr"].(string),
						lladdr: peer.(map[string]interface{})["lladdr"].(string),
						state:  peer.(map[string]interface{})["state"].(string),
					}
					cjdnsPeers = append(cjdnsPeers, cjdnsPeer)
				}
				resultChan <- cjdnsPeers
			}
		}
	}(resultChan)
	peers := <-resultChan
	return peers, nil
}

func (c *Cjdns) NodeStore_dumpTable() ([]CjdnsNode, error) {
	message := map[string]interface{}{
		"q":    "NodeStore_dumpTable",
		"args": map[string]int{"page": 0},
	}

	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		return nil, err
	}

	_, err = c.Socket.Write(bytes)
	if err != nil {
		return nil, err
	}

	c.Socket.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 4096)
	resultChan := make(chan []CjdnsNode)
	var response map[string]interface{}
	go func(resultChan chan<- []CjdnsNode) {
		n, err := c.Socket.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			err := bencode.DecodeBytes(buf, &response)
			if err != nil {
				log.Errorf("Error decoding response: %v", err)
			}
			log.Infof("NodeStore_dumpTable response: %v", response)
			if nodes, ok := response["routingTable"].(interface{}); ok {
				cjdnsNodes := []CjdnsNode{}
				for _, node := range nodes.([]interface{}) {
					cjdnsNode := CjdnsNode{
						CjdnsPubKey:  node.(map[string]interface{})["addr"].(string),
						CjdnsAddr: node.(map[string]interface{})["ip"].(string),
					}
					cjdnsNodes = append(cjdnsNodes, cjdnsNode)
				}
				resultChan <- cjdnsNodes
			}
		}
	}(resultChan)
	peers := <-resultChan
	return peers, nil
}

func (c *Cjdns) RegisterHandler(contentType int64, udpPort int64) error {
	message := map[string]interface{}{
		"q":    "UpperDistributor_registerHandler",
		"args": map[string]int64{"contentType": contentType, "udpPort": udpPort},
	}
	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		log.Errorf("Error encoding CJDNS message:", err)
		return err
	}
	_, err = c.Socket.Write(bytes)
	if err != nil {
		log.Errorf("Error writing to CJDNS socket:", err)
		return err
	}
	return nil
}

func (c *Cjdns) UnregisterHandler(udpPort int) error {
	message := map[string]interface{}{
		"q":    "UpperDistributor_unregisterHandler",
		"args": map[string]int{"udpPort": udpPort},
	}
	bytes, err := bencode.EncodeBytes(message)
	if err != nil {
		log.Errorf("Error encoding CJDNS message:", err)
		return err
	}
	_, err = c.Socket.Write(bytes)
	if err != nil {
		log.Errorf("Error writing to CJDNS socket:", err)
		return err
	}
	return nil
}