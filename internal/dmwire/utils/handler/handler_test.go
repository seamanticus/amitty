package handler

import (
	"context"
	"slices"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
)

// record returns a subscriber that appends name to got for each event.
func record(got *[]string, name string) func(ws.Event) {
	return func(ws.Event) { *got = append(*got, name) }
}

func TestCallOrder(t *testing.T) {
	h := New()
	var got []string
	h.Subscribe(record(&got, "a"))
	h.Subscribe(record(&got, "b"))
	h.Subscribe(record(&got, "c"))

	h.Call(&gateway.MessageCreateEvent{})
	if want := []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestZeroHandler(t *testing.T) {
	var h Handler
	h.Call(&gateway.MessageCreateEvent{})
}

func TestUnsubscribe(t *testing.T) {
	h := New()
	var got []string
	unsubscribe := h.Subscribe(record(&got, "a"))
	h.Subscribe(record(&got, "b"))

	unsubscribe()
	unsubscribe()
	h.Call(&gateway.MessageCreateEvent{})
	if want := []string{"b"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReentrant(t *testing.T) {
	h := New()
	var got []string
	var unsubscribe func()
	unsubscribe = h.Subscribe(func(ev ws.Event) {
		if _, ok := ev.(*gateway.MessageCreateEvent); !ok {
			return
		}
		got = append(got, "first")
		unsubscribe()
		// Subscribing and calling from within a call must not deadlock.
		h.Subscribe(record(&got, "added"))
		h.Call(&gateway.TypingStartEvent{})
	})

	h.Call(&gateway.MessageCreateEvent{})
	h.Call(&gateway.MessageCreateEvent{})
	// The added subscriber sees the nested call and the second message, and is not called by the call it was added in.
	if want := []string{"first", "added", "added"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSend(t *testing.T) {
	ch := make(chan ws.Event, 1)
	want := &gateway.MessageCreateEvent{}
	Send(t.Context(), ch)(want)
	if got := <-ch; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// Unbuffered and never received: only the done context returns.
	Send(ctx, make(chan ws.Event))(want)
}

func TestQueue(t *testing.T) {
	q := NewQueue()
	first, second := &gateway.MessageCreateEvent{}, &gateway.TypingStartEvent{}
	q.Push(first)
	q.Push(second)

	for _, want := range []ws.Event{first, second} {
		if got, ok := q.Pop(t.Context()); !ok || got != want {
			t.Fatalf("Pop = %v, %v, want %v in push order", got, ok, want)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, ok := q.Pop(ctx); ok {
		t.Fatal("Pop on an empty queue returned an event after ctx ended")
	}
}

func BenchmarkCall(b *testing.B) {
	h := New()
	for range 30 {
		h.Subscribe(func(ev ws.Event) {
			switch ev.(type) {
			case *gateway.TypingStartEvent:
			}
		})
	}
	ev := &gateway.MessageCreateEvent{}
	b.ReportAllocs()
	for b.Loop() {
		h.Call(ev)
	}
}
