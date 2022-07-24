package event_test

import (
	"sync"
	"testing"

	"github.com/pkt-cash/pktd/btcutil/event"
	"github.com/stretchr/testify/assert"
)

type secretNumber struct {
	n int
}

func TestEventSimple(t *testing.T) {
	sn := secretNumber{3}
	ee := event.NewEmitter[secretNumber]("my emitter")
	ok := false
	var active sync.WaitGroup
	var done sync.WaitGroup

	for i := 0; i < 10; i++ {
		active.Add(1)
		event.GoWg(&done, func(loop *event.Loop) {
			ee.On(loop, func(sn secretNumber) {
				assert.Equal(t, sn.n, 3)
				sn.n = 5
				loop.CurrentHandler().Cancel()
			})
			active.Done()
		})
	}

	active.Wait()
	ee.TryEmit(sn)
	done.Wait()
	assert.True(t, ok)
}

func TestMulti(t *testing.T) {
	sn3 := secretNumber{3}
	sn7 := secretNumber{7}
	ee1 := event.NewEmitter[secretNumber]("my emitter")
	ee2 := event.NewEmitter[secretNumber]("my emitter2")
	var active sync.WaitGroup
	var done sync.WaitGroup

	for i := 0; i < 10; i++ {
		active.Add(1)
		event.GoWg(&done, func(loop *event.Loop) {
			ecount1 := 0
			ecount2 := 0
			ee1.On(loop, func(sn secretNumber) {
				assert.Equal(t, sn.n, 3)
				sn.n = 5
				ecount1 += 1
				if ecount1 == 5 {
					loop.CurrentHandler().Cancel()
				}
			})
			ee2.On(loop, func(sn secretNumber) {
				assert.Equal(t, sn.n, 7)
				sn.n = 5
				ecount2 += 1
				if ecount2 == 5 {
					loop.CurrentHandler().Cancel()
				}
			})
			active.Done()
		})
	}

	active.Wait()
	for i := 0; i < 5; i++ {
		ee1.TryEmit(sn3)
		ee2.TryEmit(sn7)
	}
	done.Wait()
}
