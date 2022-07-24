package event

import (
	"reflect"
	"sync"

	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/lock"
)

type emitterMut[T any] struct {
	handlerChans []chan *T
}

// Emitter is an event emitter, it can be used to inform different event handlers
// that something is going on, so they can react.
type Emitter[T any] struct {
	m    lock.GenMutex[emitterMut[T]]
	name string
}

// NewEmitter creates a new event emitter, the type of the event data cannot be inferred
// so it must be explicitly specified, e.g.  event.NewEmitter[MyObj]("My Emitter")
// A name is requested which will appear in any errors.
func NewEmitter[T any](name string) Emitter[T] {
	return Emitter[T]{
		name: name,
		m:    lock.NewGenMutex(emitterMut[T]{}, name),
	}
}

// Handler is an event handler interface, useful for cancelling an event
type Handler struct {
	id uintptr
	l  *Loop
}

type caseData struct {
	id    uintptr
	cb    func(v reflect.Value)
	close func() er.R
}

// This stuct must NOT be copied between goroutines
// Each goroutine needs it's own loop
type Loop struct {
	// Keep cases separate from data so that it can be fed directly to Select
	cases  []reflect.SelectCase
	data   []caseData
	nextId uintptr
	curId  uintptr
	Wg     *sync.WaitGroup
}

func (l *Loop) dropCase(chosen int) er.R {
	l.cases[chosen] = l.cases[len(l.cases)-1]
	l.cases = l.cases[:len(l.cases)-1]
	err := l.data[chosen].close()
	l.data[chosen] = l.data[len(l.data)-1]
	l.data = l.data[:len(l.data)-1]
	return err
}

// GoWg creates a new goroutine with an event loop
// and allows a WaitGroup to wait until it is done
func GoWg(wg *sync.WaitGroup, f func(loop *Loop)) {
	wg.Add(1)
	go func() {
		l := Loop{
			Wg:     wg,
			nextId: 1, // curId = 0 -> invalid
		}
		f(&l)
		for len(l.cases) > 0 {
			chosen, recv, recvOK := reflect.Select(l.cases)
			if !recvOK {
				// closed channel, remove
				l.dropCase(chosen)
				continue
			}
			l.curId = l.data[chosen].id
			l.data[chosen].cb(recv)
			l.curId = 0
		}
		wg.Done()
	}()
}

// Go creates a new goroutine with an event loop
func Go(f func(loop *Loop)) {
	var wg sync.WaitGroup
	GoWg(&wg, f)
}

const chanDepth = 256

// Listeners gets number of listeners for an event emitter
func (ee *Emitter[T]) Listeners() (out int) {
	ee.m.In(func(em *emitterMut[T]) er.R {
		out = len(em.handlerChans)
		return nil
	})
	return
}

// Clear cancels all listeners to an event
// Note that more listeners may register after.
func (ee *Emitter[T]) Clear() er.R {
	return ee.m.In(func(em *emitterMut[T]) er.R {
		for _, hc := range em.handlerChans {
			close(hc)
		}
		em.handlerChans = nil
		return nil
	})
}

// TryEmit attempts to emit an event, an error is returned if
// one or more of the handler channels is full.
func (ee *Emitter[T]) TryEmit(t T) er.R {
	return ee.m.In(func(em *emitterMut[T]) er.R {
		ok := 0
		for _, c := range em.handlerChans {
			if len(c) == cap(c) {
				continue
			}
			tt := t
			c <- &tt
			ok += 1
		}
		if ok < len(em.handlerChans) {
			return er.Errorf("Emitter: [%s] out of space, event emitted to [%d] of [%d] handlers",
				ee.name, ok, len(em.handlerChans))
		}
		return nil
	})
}

// On registers a handler to be called when an event fires
func (ee *Emitter[T]) On(l *Loop, f func(t T)) *Handler {
	ch := make(chan *T, chanDepth)
	ee.m.In(func(em *emitterMut[T]) er.R {
		em.handlerChans = append(em.handlerChans, ch)
		return nil
	})
	l.cases = append(l.cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ch),
	})
	id := l.nextId
	l.data = append(l.data, caseData{
		id: id,
		cb: func(v reflect.Value) { f(*v.Interface().(*T)) },
		close: func() er.R {
			return ee.m.In(func(em *emitterMut[T]) er.R {
				for i, c := range em.handlerChans {
					if c == ch {
						em.handlerChans[i] = em.handlerChans[len(em.handlerChans)-1]
						em.handlerChans = em.handlerChans[:len(em.handlerChans)-1]
						return nil
					}
				}
				return er.Errorf("Emitter: [%s] unable to remove channel, already cancelled?",
					ee.name)
			})
		},
	})
	l.nextId += 1
	return &Handler{
		id: id,
		l:  l,
	}
}

// CurrentHandler gets the CURRENTLY executing event handler, if any.
// If it is called outside of an event handler, it returns nil.
// To end the currently executing event: loop.CurrentHandler().Cancel()
func (l *Loop) CurrentHandler() *Handler {
	if l.curId == 0 {
		return nil
	}
	return &Handler{
		id: l.curId,
		l:  l,
	}
}

// Cancel removes a registered handler
func (h *Handler) Cancel() er.R {
	for i, d := range h.l.data {
		if d.id == h.id {
			h.l.dropCase(i)
			return nil
		}
	}
	return er.New("No such handler is registered, was it cancelled already?")
}

// Quit cancels all awaiting listeners so that the loop will exit
func (l *Loop) Quit() er.R {
	for i := len(l.cases) - 1; i >= 0; i-- {
		if err := l.dropCase(i); err != nil {
			return err
		}
	}
	return nil
}
