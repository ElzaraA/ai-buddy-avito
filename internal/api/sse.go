package api

import (
"sync"
)

type SSEEvent struct {
Event string
Data  string
}

type Broker struct {
mu   sync.RWMutex
subs map[string][]chan SSEEvent
}

func NewBroker() *Broker {
return &Broker{subs: map[string][]chan SSEEvent{}}
}

func (b *Broker) Subscribe(sessionID string) chan SSEEvent {
ch := make(chan SSEEvent, 16)
b.mu.Lock()
b.subs[sessionID] = append(b.subs[sessionID], ch)
b.mu.Unlock()
return ch
}

func (b *Broker) Unsubscribe(sessionID string, ch chan SSEEvent) {
b.mu.Lock()
defer b.mu.Unlock()
list := b.subs[sessionID]
for i, c := range list {
if c == ch {
b.subs[sessionID] = append(list[:i], list[i+1:]...)
close(ch)
return
}
}
}

func (b *Broker) Publish(sessionID string, ev SSEEvent) {
b.mu.RLock()
defer b.mu.RUnlock()
for _, ch := range b.subs[sessionID] {
select {
case ch <- ev:
default:
}
}
}