package graphql

import (
	"sync"
)

type PubSub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan *DriveItemEvent]struct{}
}

func NewPubSub() *PubSub {
	return &PubSub{
		subscribers: make(map[string]map[chan *DriveItemEvent]struct{}),
	}
}

// Publish sends an event to all subscribers of a specific tenant.
func (p *PubSub) Publish(tenantID string, event *DriveItemEvent) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if subs, ok := p.subscribers[tenantID]; ok {
		for ch := range subs {
			// Non-blocking send
			select {
			case ch <- event:
			default:
				// If channel is full, drop the event.
				// In a real system, we'd want a larger buffer or disconnect slow clients.
			}
		}
	}
}

// Subscribe returns a channel that receives events for the given tenant.
func (p *PubSub) Subscribe(tenantID string) (<-chan *DriveItemEvent, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch := make(chan *DriveItemEvent, 100)
	if _, ok := p.subscribers[tenantID]; !ok {
		p.subscribers[tenantID] = make(map[chan *DriveItemEvent]struct{})
	}
	p.subscribers[tenantID][ch] = struct{}{}

	unsub := func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if subs, ok := p.subscribers[tenantID]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(p.subscribers, tenantID)
			}
		}
		close(ch)
	}

	return ch, unsub
}
