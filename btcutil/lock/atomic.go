package lock

import (
	"sync/atomic"
)

type AtomicInt32 struct {
	b int32
}

func (ab *AtomicInt32) Load() int32 {
	return atomic.LoadInt32(&ab.b)
}
func (ab *AtomicInt32) Store(n int32) {
	atomic.StoreInt32(&ab.b, n)
}
func (ab *AtomicInt32) Swap(b int32) int32 {
	return atomic.SwapInt32(&ab.b, b)
}

type AtomicBool AtomicInt32

func (ab *AtomicBool) Load() bool {
	return atomic.LoadInt32(&ab.b) > 0
}
func (ab *AtomicBool) Store(b bool) {
	n := int32(0)
	if b {
		n = 1
	}
	atomic.StoreInt32(&ab.b, n)
}
func (ab *AtomicBool) Swap(b bool) bool {
	n := int32(0)
	if b {
		n = 1
	}
	return atomic.SwapInt32(&ab.b, n) > 0
}
