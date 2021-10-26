package connmgr

import (
	"sync"
	"time"
)

type dbs struct {
	bs          *DynamicBanScore
	lastUsedSec int64
}

type BanMgr struct {
	m  sync.Mutex
	bs map[string]dbs
}

func now() int64 {
	return time.Now().Unix()
}

func (b *BanMgr) GetScore(host string) *DynamicBanScore {
	b.m.Lock()
	if _, ok := b.bs[host]; !ok {
		b.bs[host] = dbs{
			bs:          &DynamicBanScore{},
			lastUsedSec: now(),
		}
	}
	bs := b.bs[host].bs
	b.m.Unlock()
	return bs
}
