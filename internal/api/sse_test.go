package api

import "testing"

func TestBroker(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe("s1")
	b.Publish("s1", SSEEvent{Event: "status", Data: "x"})
	select {
	case ev := <-ch:
		if ev.Event != "status" {
			t.Fatal("wrong event")
		}
	default:
		t.Fatal("no event")
	}
	b.Unsubscribe("s1", ch)
	if _, ok := <-ch; ok {
		t.Fatal("channel must be closed")
	}
}

func TestBrokerNoSubs(t *testing.T) {
	b := NewBroker()
	b.Publish("nope", SSEEvent{Event: "x"})
}
