package lock

import (
	"sync/atomic"
)

type AtomicInt32 struct {
	I int32
}

func (ab *AtomicInt32) Load() int32 {
	return atomic.LoadInt32(&ab.I)
}
func (ab *AtomicInt32) Store(n int32) {
	atomic.StoreInt32(&ab.I, n)
}
func (ab *AtomicInt32) Add(n int32) {
	atomic.AddInt32(&ab.I, n)
}
func (ab *AtomicInt32) Swap(b int32) int32 {
	return atomic.SwapInt32(&ab.I, b)
}

type AtomicBool AtomicInt32

func (ab *AtomicBool) Load() bool {
	return atomic.LoadInt32(&ab.I) > 0
}
func (ab *AtomicBool) Store(b bool) {
	n := int32(0)
	if b {
		n = 1
	}
	atomic.StoreInt32(&ab.I, n)
}
func (ab *AtomicBool) Swap(b bool) bool {
	n := int32(0)
	if b {
		n = 1
	}
	return atomic.SwapInt32(&ab.I, n) > 0
}
