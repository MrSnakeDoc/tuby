package events

import (
	"sync"
	"sync/atomic"
)

const subscriberBuffer = 8

type Update struct {
	Revision   uint64   `json:"revision"`
	Reason     string   `json:"reason"`
	Scopes     []string `json:"scopes"`
	ChannelIDs []string `json:"channel_ids,omitempty"`
	VideoIDs   []string `json:"video_ids,omitempty"`
}

type Hub struct {
	mu          sync.Mutex
	subscribers map[chan Update]struct{}
	revision    atomic.Uint64
}

func New() *Hub {
	return &Hub{subscribers: make(map[chan Update]struct{})}
}

func (h *Hub) Publish(update Update) {
	update.Revision = h.revision.Add(1)
	h.mu.Lock()
	defer h.mu.Unlock()
	for subscriber := range h.subscribers {
		select {
		case subscriber <- update:
		default:
			close(subscriber)
			delete(h.subscribers, subscriber)
		}
	}
}

func (h *Hub) Subscribe() (<-chan Update, func()) {
	subscriber := make(chan Update, subscriberBuffer)
	h.mu.Lock()
	h.subscribers[subscriber] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return subscriber, func() {
		once.Do(func() {
			h.mu.Lock()
			if _, exists := h.subscribers[subscriber]; exists {
				delete(h.subscribers, subscriber)
				close(subscriber)
			}
			h.mu.Unlock()
		})
	}
}

func (h *Hub) SyncUpdate() Update {
	return Update{Revision: h.revision.Load(), Reason: "sync", Scopes: AllScopes()}
}

func AllScopes() []string {
	return []string{"dock", "home", "channel", "missed", "later"}
}
