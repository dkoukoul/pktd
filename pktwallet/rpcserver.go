// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"runtime"
	"strings"

	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/pktlog/log"
	"github.com/pkt-cash/pktd/pktwallet/rpc/legacyrpc"
	"github.com/pkt-cash/pktd/pktwallet/wallet"
)

var netListen = func(n, laddr string) (net.Listener, er.R) {
	ret, errr := net.Listen(n, laddr)
	return ret, er.E(errr)
}

func startRPCServers(walletLoader *wallet.Loader) (*legacyrpc.Server, er.R) {
	var legacyServer *legacyrpc.Server

	if cfg.Username == "" || cfg.Password == "" {
		log.Info("Legacy RPC server disabled (requires username and password)")
	} else if len(cfg.LegacyRPCListeners) != 0 {
		listeners := makeListeners(cfg.LegacyRPCListeners, netListen)
		if len(listeners) == 0 {
			err := er.New("failed to create listeners for legacy RPC server")
			return nil, err
		}
		opts := legacyrpc.Options{
			Username:            cfg.Username,
			Password:            cfg.Password,
			MaxPOSTClients:      cfg.LegacyRPCMaxClients,
			MaxWebsocketClients: cfg.LegacyRPCMaxWebsockets,
		}
		legacyServer = legacyrpc.NewServer(&opts, walletLoader, listeners)
	}

	// Error when neither the GRPC nor legacy RPC servers can be started.
	if legacyServer == nil {
		return nil, er.New("no suitable RPC services can be started")
	}

	return legacyServer, nil
}

type listenFunc func(net string, laddr string) (net.Listener, er.R)

// makeListeners splits the normalized listen addresses into IPv4 and IPv6
// addresses and creates new net.Listeners for each with the passed listen func.
// Invalid addresses are logged and skipped.
func makeListeners(normalizedListenAddrs []string, listen listenFunc) []net.Listener {
	ipv4Addrs := make([]string, 0, len(normalizedListenAddrs)*2)
	ipv6Addrs := make([]string, 0, len(normalizedListenAddrs)*2)
	for _, addr := range normalizedListenAddrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Shouldn't happen due to already being normalized.
			log.Errorf("`%s` is not a normalized "+
				"listener address", addr)
			continue
		}

		// Empty host or host of * on plan9 is both IPv4 and IPv6.
		if host == "" || (host == "*" && runtime.GOOS == "plan9") {
			ipv4Addrs = append(ipv4Addrs, addr)
			ipv6Addrs = append(ipv6Addrs, addr)
			continue
		}

		// Remove the IPv6 zone from the host, if present.  The zone
		// prevents ParseIP from correctly parsing the IP address.
		zoneIndex := strings.Index(host, "%")
		if zoneIndex != -1 {
			host = host[:zoneIndex]
		}

		ip := net.ParseIP(host)
		switch {
		case ip == nil:
			log.Warnf("`%s` is not a valid IP address", host)
		case ip.To4() == nil:
			ipv6Addrs = append(ipv6Addrs, addr)
		default:
			ipv4Addrs = append(ipv4Addrs, addr)
		}
	}
	listeners := make([]net.Listener, 0, len(ipv6Addrs)+len(ipv4Addrs))
	for _, addr := range ipv4Addrs {
		listener, err := listen("tcp4", addr)
		if err != nil {
			log.Warnf("Can't listen on %s: %v", addr, err)
			continue
		}
		listeners = append(listeners, listener)
	}
	for _, addr := range ipv6Addrs {
		listener, err := listen("tcp6", addr)
		if err != nil {
			log.Warnf("Can't listen on %s: %v", addr, err)
			continue
		}
		listeners = append(listeners, listener)
	}
	return listeners
}
