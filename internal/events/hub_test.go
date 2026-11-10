package events

import "testing"

func TestHubBroadcastsUpdates(t *testing.T) {
	hub := New()
	first, cancelFirst := hub.Subscribe()
	defer cancelFirst()
	second, cancelSecond := hub.Subscribe()
	defer cancelSecond()

	hub.Publish(Update{Reason: "video.seen", Scopes: AllScopes(), VideoIDs: []string{"video"}})
	for _, subscriber := range []<-chan Update{first, second} {
		update := <-subscriber
		if update.Revision != 1 || update.Reason != "video.seen" || update.VideoIDs[0] != "video" {
			t.Fatalf("update = %#v", update)
		}
	}
}

func TestHubSyncUsesCurrentRevision(t *testing.T) {
	hub := New()
	hub.Publish(Update{Reason: "changed"})
	if update := hub.SyncUpdate(); update.Revision != 1 || update.Reason != "sync" {
		t.Fatalf("sync update = %#v", update)
	}
}
