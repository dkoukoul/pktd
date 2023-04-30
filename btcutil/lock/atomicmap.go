package lock

import (
	"fmt"
	"sync"
)

type AtomicMapItem[K comparable, V any] struct {
	K K
	V V
}

type AtomicMap[K comparable, V any] struct {
	internal sync.Map
}

func (am *AtomicMap[K, V]) Get(k K) (V, bool) {
	val, ok := am.internal.Load(k)
	if !ok {
		var v V
		return v, false
	}
	return as[V](val), true
}

func (am *AtomicMap[K, V]) Put(k K, v V) {
	am.internal.Store(k, v)
}

func (am *AtomicMap[K, V]) Delete(k K) {
	am.internal.Delete(k)
}

func (am *AtomicMap[K, V]) Update(k K, f func(val *V, exists bool) bool) {
	val, ok := am.internal.Load(k)
	if !ok {
		var v V
		if f(&v, false) {
			am.Put(k, v)
		}
		return
	}
	ret := as[V](val)
	if !f(&ret, true) {
		am.Delete(k)
	} else {
		am.Put(k, ret)
	}
}

func as[T any](t any) T {
	if tt, isOk := t.(T); isOk {
		return tt
	}
	panic(fmt.Sprintf("Wrong type [%T]", t))
}

func (am *AtomicMap[K, V]) Items() []AtomicMapItem[K, V] {
	var keys []AtomicMapItem[K, V]
	am.internal.Range(func(k any, v any) bool {
		keys = append(keys, AtomicMapItem[K, V]{K: as[K](k), V: as[V](v)})
		return true
	})
	return keys
}

func (am *AtomicMap[K, V]) Retain(f func(k K, v *V) bool) {
	var deleteKeys []K
	am.internal.Range(func(key0 any, val0 any) bool {
		key := as[K](key0)
		val := as[V](val0)
		if !f(key, &val) {
			deleteKeys = append(deleteKeys, key)
		}
		return true
	})
	for _, k := range deleteKeys {
		am.internal.Delete(k)
	}
}
