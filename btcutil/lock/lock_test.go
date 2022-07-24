package lock_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/lock"
	"github.com/stretchr/testify/assert"
)

type testObj struct {
	n int
}

func test(write bool, to ...*testObj) er.R {
	nums := make([]int, len(to))
	for i := 0; i < len(to); i++ {
		if write {
			to[i].n = rand.Int()
		}
		nums[i] = to[i].n
	}
	time.Sleep(time.Millisecond)
	for i := 0; i < len(to); i++ {
		if to[i].n != nums[i] {
			return er.New("n has changed")
		}
	}
	return nil
}

func testWrite(to ...*testObj) er.R {
	return test(true, to...)
}
func testRead(to ...*testObj) er.R {
	return test(false, to...)
}

func doInThreads(f func() er.R) func() er.R {
	w := sync.WaitGroup{}
	errl := lock.NewGenMutex[er.R](nil, "error lock")
	for i := 0; i < 10; i++ {
		w.Add(1)
		go func() {
			for j := 0; j < 10; j++ {
				if err := f(); err != nil {
					lock.With1[er.R](&errl, func(e *er.R) er.R {
						*e = err
						w.Done()
						return nil
					})
					return
				}
			}
			w.Done()
		}()
	}
	return func() er.R {
		w.Wait()
		return errl.In(func(e *er.R) er.R {
			return *e
		})
	}
}

func TestNoLock(t *testing.T) {
	to := testObj{}
	for i := 0; i < 100; i++ {
		if doInThreads(func() er.R {
			return testWrite(&to)
		})() != nil {
			return
		}
	}
	t.Error("Expected an error testing with no lock")
}

func TestLock(t *testing.T) {
	l := lock.NewGenMutex(testObj{}, "mylock")
	assert.NoError(t, er.Native(doInThreads(func() er.R {
		return lock.With1[testObj](&l, func(to *testObj) er.R { return testWrite(to) })
	})()))
}

func TestRwLock(t *testing.T) {
	l := lock.NewGenRwLock(testObj{}, "myrwlock")
	assert.NoError(t, er.Native(doInThreads(func() er.R {
		switch rand.Int() % 2 {
		case 0:
			return l.R().In(func(to *testObj) er.R { return testRead(to) })
		case 1:
			return l.W().In(func(to *testObj) er.R { return testWrite(to) })
		default:
			panic("")
		}
	})()))
}

func batch2(a, b *lock.GenMutex[testObj]) er.R {
	return lock.With2[testObj, testObj](a, b, func(a, b *testObj) er.R {
		return testWrite(a, b)
	})
}

func batch3(a, b, c *lock.GenMutex[testObj]) er.R {
	return lock.With3[testObj, testObj, testObj](a, b, c, func(a, b, c *testObj) er.R {
		return testWrite(a, b, c)
	})
}

func TestBatchLock(t *testing.T) {
	l1 := lock.NewGenMutex(testObj{}, "l1")
	l2 := lock.NewGenMutex(testObj{}, "l2")
	l3 := lock.NewGenMutex(testObj{}, "l3")
	assert.NoError(t, er.Native(doInThreads(func() er.R {
		switch rand.Int() % 8 {
		case 0:
			return batch3(&l1, &l2, &l3)
		case 1:
			return batch3(&l3, &l2, &l1)
		case 2:
			return batch2(&l1, &l2)
		case 3:
			return batch2(&l1, &l3)
		case 4:
			return batch2(&l2, &l3)
		case 5:
			return batch2(&l3, &l1)
		case 6:
			return batch2(&l3, &l2)
		case 7:
			return batch2(&l2, &l1)
		default:
			panic("")
		}
	})()))
}
