package lock

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/pkt-cash/pktd/btcutil/er"
)

/// internal iface

type lockable[T any] interface {
	lock() *T
	unlock()
	id() uintptr
	String() string
	In(func(t *T) er.R) er.R
}

/// id

var idgen uintptr

func mkId() uintptr {
	return atomic.AddUintptr(&idgen, 1)
}

type withId struct {
	n string
	i uintptr
}

func (l withId) id() uintptr {
	return l.i
}

func (l withId) String() string {
	return l.n
}

/// lock

type GenMutex[T any] struct {
	withId
	m sync.Mutex
	t T
}

var _ lockable[any] = (*GenMutex[any])(nil)

func NewGenMutex[T any](t T, name string) GenMutex[T] {
	return GenMutex[T]{t: t, withId: withId{n: name, i: mkId()}}
}

func (gm *GenMutex[T]) lock() *T {
	gm.m.Lock()
	return &gm.t
}
func (gm *GenMutex[T]) unlock() {
	gm.m.Unlock()
}

/// rwlock

type GenRwLock[T any] struct {
	withId
	m sync.RWMutex
	t T
}

func NewGenRwLock[T any](t T, name string) GenRwLock[T] {
	return GenRwLock[T]{t: t, withId: withId{n: name, i: mkId()}}
}

/// read

type GenRwLockR[T any] struct {
	*GenRwLock[T]
}

var _ lockable[any] = GenRwLockR[any]{nil}

func (gm GenRwLockR[T]) lock() *T {
	gm.m.RLock()
	return &gm.t
}
func (gm GenRwLockR[T]) unlock() {
	gm.m.RUnlock()
}

/// write

type GenRwLockW[T any] struct {
	*GenRwLock[T]
}

var _ lockable[any] = GenRwLockW[any]{nil}

func (gm GenRwLockW[T]) lock() *T {
	gm.m.Lock()
	return &gm.t
}
func (gm GenRwLockW[T]) unlock() {
	gm.m.Unlock()
}

/// anyLock uses a function closure to erase generic info

type anyLock struct {
	id     uintptr
	name   string
	lock   func()
	unlock func()
}

func mkAny[T any](l lockable[T], t **T) anyLock {
	return anyLock{
		id:     l.id(),
		name:   l.String(),
		lock:   func() { *t = l.lock() },
		unlock: func() { l.unlock() },
	}
}

type anyLocks []anyLock

func (al anyLocks) process(f func() er.R) er.R {
	sort.Slice(al, func(a int, b int) bool { return al[a].id < al[b].id })
	for i, l := range al {
		if i == 0 {
			continue
		}
		if l.id == al[i-1].id {
			return er.Errorf("Failed to lock [%s] and [%s] which have the same id (%x)",
				l.name, al[i-1].name, l.id)
		}
	}
	for _, l := range al {
		l.lock()
		defer l.unlock()
	}
	return f()
}

// Public

/// R gets the reader "view" of an RWLock
func (gm *GenRwLock[T]) R() GenRwLockR[T] {
	return GenRwLockR[T]{gm}
}

/// W gets the writer "view" of an RWLock
func (gm *GenRwLock[T]) W() GenRwLockW[T] {
	return GenRwLockW[T]{gm}
}

/// With1 performs an operation with one lock held.
/// When using this function, you MUST specify the type.
/// e.g.  lock.With1[MyObject](myLock, func(mo *MyObject) er.R { ... })
func With1[T any](l lockable[T], f func(t *T) er.R) er.R {
	ret := l.lock()
	defer l.unlock()
	return f(ret)
}
func (gm *GenMutex[T]) In(f func(t *T) er.R) er.R {
	return With1[T](gm, f)
}
func (gm GenRwLockR[T]) In(f func(t *T) er.R) er.R {
	return With1[T](gm, f)
}
func (gm GenRwLockW[T]) In(f func(t *T) er.R) er.R {
	return With1[T](gm, f)
}

/// With2 performs an operation with two locks held.
/// Locks are taken in reliable order to avoid possible deadlock.
/// When using this function, you MUST specify the type of each.
/// e.g.  lock.With2[Obj1, Obj2](l1, l2, func(o1 *Obj1, o2 *Obj2) er.R { ... })
func With2[T1, T2 any](
	l1 lockable[T1],
	l2 lockable[T2],
	f func(
		t1 *T1,
		t2 *T2,
	) er.R,
) er.R {
	var r1 *T1
	var r2 *T2
	return anyLocks{
		mkAny(l1, &r1),
		mkAny(l2, &r2),
	}.process(func() er.R {
		return f(r1, r2)
	})
}

/// With3 performs an operation with two locks held.
/// Locks are taken in reliable order to avoid possible deadlock.
/// When using this function, you MUST specify the type of each.
/// e.g.  lock.With3[Obj1, Obj2, Obj3](l1, l2, l3 ...
func With3[T1, T2, T3 any](
	l1 lockable[T1],
	l2 lockable[T2],
	l3 lockable[T3],
	f func(
		t1 *T1,
		t2 *T2,
		t3 *T3,
	) er.R,
) er.R {
	var r1 *T1
	var r2 *T2
	var r3 *T3
	return anyLocks{
		mkAny(l1, &r1),
		mkAny(l2, &r2),
		mkAny(l3, &r3),
	}.process(func() er.R {
		return f(r1, r2, r3)
	})
}

/// With4 performs an operation with four locks held.
/// Locks are taken in reliable order to avoid possible deadlock.
/// When using this function, you MUST specify the type of each.
/// e.g.  lock.With4[Obj1, Obj2, Obj3, Obj4](l1, l2, l3, l4, func(...
func With4[T1, T2, T3, T4 any](
	l1 lockable[T1],
	l2 lockable[T2],
	l3 lockable[T3],
	l4 lockable[T4],
	f func(
		t1 *T1,
		t2 *T2,
		t3 *T3,
		t4 *T4,
	) er.R,
) er.R {
	var r1 *T1
	var r2 *T2
	var r3 *T3
	var r4 *T4
	return anyLocks{
		mkAny(l1, &r1),
		mkAny(l2, &r2),
		mkAny(l3, &r3),
		mkAny(l4, &r4),
	}.process(func() er.R {
		return f(r1, r2, r3, r4)
	})
}

/// With5 performs an operation with five locks held.
/// Locks are taken in reliable order to avoid possible deadlock.
/// When using this function, you MUST specify the type of each.
/// e.g.  lock.With5[Obj1, Obj2, Obj3, Obj4, Obj5](l1, l2, l3, l4, l5, ...
func With5[T1, T2, T3, T4, T5 any](
	l1 lockable[T1],
	l2 lockable[T2],
	l3 lockable[T3],
	l4 lockable[T4],
	l5 lockable[T5],
	f func(
		t1 *T1,
		t2 *T2,
		t3 *T3,
		t4 *T4,
		t5 *T5,
	) er.R,
) er.R {
	var r1 *T1
	var r2 *T2
	var r3 *T3
	var r4 *T4
	var r5 *T5
	return anyLocks{
		mkAny(l1, &r1),
		mkAny(l2, &r2),
		mkAny(l3, &r3),
		mkAny(l4, &r4),
		mkAny(l5, &r5),
	}.process(func() er.R {
		return f(r1, r2, r3, r4, r5)
	})
}
