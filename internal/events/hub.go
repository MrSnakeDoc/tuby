package events

import (
	"sync"
)

const subscriberBuffer = 8

type Update struct {
	Revision    uint64   `json:"revision"`
	BuildID     string   `json:"build_id"`
	Reason      string   `json:"reason"`
	Scopes      []string `json:"scopes"`
	CategoryIDs []int64  `json:"category_ids,omitempty"`
	ChannelIDs  []string `json:"channel_ids,omitempty"`
	VideoIDs    []string `json:"video_ids,omitempty"`
}

type Hub struct {
	mu          sync.Mutex
	subscribers map[chan Update]struct{}
	revision    uint64
	buildID     string
	closed      bool
}

func New(buildID string) *Hub {
	return &Hub{subscribers: make(map[chan Update]struct{}), buildID: buildID}
}

func (h *Hub) Publish(update Update) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.revision++
	update.Revision = h.revision
	update.BuildID = h.buildID
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
	if h.closed {
		close(subscriber)
		h.mu.Unlock()
		return subscriber, func() {}
	}
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

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for subscriber := range h.subscribers {
		close(subscriber)
		delete(h.subscribers, subscriber)
	}
}

func (h *Hub) SyncUpdate() Update {
	h.mu.Lock()
	defer h.mu.Unlock()
	return Update{Revision: h.revision, BuildID: h.buildID, Reason: "sync", Scopes: AllScopes()}
}

func AllScopes() []string {
	return []string{"dock", "home", "channel", "missed", "later"}
}
