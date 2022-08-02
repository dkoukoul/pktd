package localaddrs

import (
	"net"
	"strings"

	"github.com/pkt-cash/pktd/addrmgr"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/lock"
	"github.com/pkt-cash/pktd/pktlog/log"
	"github.com/pkt-cash/pktd/wire"
)

type LocalAddrs struct {
	a        lock.GenMutex[map[string]*wire.NetAddress]
	wasTried lock.AtomicBool
}

func New() LocalAddrs {
	return LocalAddrs{
		a: lock.NewGenMutex(make(map[string]*wire.NetAddress), "LocalAddrs lock"),
	}
}

func (la *LocalAddrs) Referesh() {
	ifaces, errr := net.Interfaces()
	if errr != nil {
		log.Warnf("LocalAddrs.Referesh() failed: [%v]", errr.Error())
		la.wasTried.Store(true)
		return
	}
	out := make(map[string]struct{})
	for _, i := range ifaces {
		addrs, errr := i.Addrs()
		if errr != nil {
			log.Warnf("LocalAddrs.Referesh(): [%s]", errr.Error())
			continue
		}
		for _, a := range addrs {
			out[a.String()] = struct{}{}
		}
	}
	la.a.In(func(m *map[string]*wire.NetAddress) er.R {
		for s := range *m {
			if _, ok := out[s]; !ok {
				log.Infof("Local address gone [%s]", log.IpAddr(s))
				delete(*m, s)
			}
		}
		for s := range out {
			if _, ok := (*m)[s]; !ok {
				// drop the port
				spl := strings.Split(s, "/")
				ip := net.ParseIP(spl[0])
				if ip == nil {
					log.Warnf("LocalAddrs.Referesh(): unable to parse addr [%s]", s)
				} else {
					wip := wire.NewNetAddressIPPort(ip, 0, 0)
					if (addrmgr.IsIPv4(wip) && !addrmgr.IsLocal(wip)) || addrmgr.IsRoutable(wip) {
						log.Infof("Local address detected [%s]", log.IpAddr(s))
						(*m)[s] = wip
					} else {
						log.Debugf("Non-routable local address detected [%s]", s)
						(*m)[s] = nil
					}
				}
			}
		}
		return nil
	})
}

func (la *LocalAddrs) IsWorking() bool {
	if !la.wasTried.Load() {
		// We don't yet know...
		return true
	}
	ok := false
	la.a.In(func(m *map[string]*wire.NetAddress) er.R {
		ok = len(*m) > 0
		return nil
	})
	return ok
}

func (la *LocalAddrs) Reachable(na *wire.NetAddress) bool {
	out := false
	la.a.In(func(m *map[string]*wire.NetAddress) er.R {
		for _, localNa := range *m {
			if localNa != nil && addrmgr.Reachable(localNa, na) {
				log.Infof("[%s] reachable via [%s]", na.IP.String(), localNa.IP.String())
				out = true
				break
			}
		}
		return nil
	})
	return out
}
