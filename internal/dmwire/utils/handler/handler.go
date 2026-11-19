// Package handler passes gateway events to subscribers.
//
// A subscriber is a func(ws.Event) that picks the events it needs with a type switch:
//
//	h.Subscribe(func(ev ws.Event) {
//		switch ev := ev.(type) {
//		case *gateway.MessageCreateEvent:
//			...
//		}
//	})
package handler

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
)

// Handler calls its subscribers with every event, one after another in the order they subscribed. The zero value has none.
//
// Subscribers may subscribe or unsubscribe at any time, including from within a call.
type Handler struct {
	// mutex serializes writers; Call reads subscribers without it.
	mutex sync.Mutex
	// subscribers is replaced on every change, never changed in place.
	subscribers atomic.Pointer[[]*subscriber]
}

type subscriber struct {
	fn      func(ws.Event)
	removed atomic.Bool
}

func New() *Handler {
	return &Handler{}
}

// Call calls every subscriber with ev and returns once they all have.
func (h *Handler) Call(ev ws.Event) {
	list := h.subscribers.Load()
	if list == nil {
		return
	}
	for _, s := range *list {
		if !s.removed.Load() {
			s.fn(ev)
		}
	}
}

// Subscribe adds fn and returns the function that removes it. fn blocks Call, so it must return quickly.
func (h *Handler) Subscribe(fn func(ws.Event)) (unsubscribe func()) {
	s := &subscriber{fn: fn}
	h.update(func(list []*subscriber) []*subscriber { return append(list, s) })

	return sync.OnceFunc(func() {
		s.removed.Store(true)
		h.update(func(list []*subscriber) []*subscriber {
			return slices.DeleteFunc(list, func(other *subscriber) bool { return other == s })
		})
	})
}

// update replaces the subscribers with what change makes of a copy of them.
func (h *Handler) update(change func([]*subscriber) []*subscriber) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	var list []*subscriber
	if old := h.subscribers.Load(); old != nil {
		list = slices.Clone(*old)
	}
	list = change(list)
	h.subscribers.Store(&list)
}

// Send returns a subscriber that sends each event on ch, in order, blocking until it is received or ctx is done.
func Send(ctx context.Context, ch chan<- ws.Event) func(ws.Event) {
	return func(ev ws.Event) {
		select {
		case ch <- ev:
		case <-ctx.Done():
		}
	}
}

// Queue holds events in order for a reader slower than the gateway: Push never blocks, so the gateway keeps its heartbeat while the reader is busy. It grows without bound while the reader is away.
type Queue struct {
	mutex  sync.Mutex
	events []ws.Event
	// ready holds a token while events may be waiting.
	ready chan struct{}
}

func NewQueue() *Queue {
	return &Queue{ready: make(chan struct{}, 1)}
}

// Push appends ev; it is the subscriber that fills the queue.
func (q *Queue) Push(ev ws.Event) {
	q.mutex.Lock()
	q.events = append(q.events, ev)
	q.mutex.Unlock()

	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// Pop returns the oldest event, waiting for one, or false once ctx is done.
func (q *Queue) Pop(ctx context.Context) (ws.Event, bool) {
	for {
		q.mutex.Lock()
		if len(q.events) > 0 {
			ev := q.events[0]
			q.events[0] = nil
			q.events = q.events[1:]
			q.mutex.Unlock()
			return ev, true
		}
		q.mutex.Unlock()

		select {
		case <-q.ready:
		case <-ctx.Done():
			return nil, false
		}
	}
}
