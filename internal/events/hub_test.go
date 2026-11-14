package events

import (
	"sync"
	"testing"
)

func TestHubBroadcastsUpdates(t *testing.T) {
	hub := New("v2-test")
	first, cancelFirst := hub.Subscribe()
	defer cancelFirst()
	second, cancelSecond := hub.Subscribe()
	defer cancelSecond()

	hub.Publish(Update{Reason: "video.seen", Scopes: AllScopes(), VideoIDs: []string{"video"}})
	for _, subscriber := range []<-chan Update{first, second} {
		update := <-subscriber
		if update.Revision != 1 || update.BuildID != "v2-test" || update.Reason != "video.seen" ||
			update.VideoIDs[0] != "video" {
			t.Fatalf("update = %#v", update)
		}
	}
}

func TestHubSyncUsesCurrentRevision(t *testing.T) {
	hub := New("v2-restarted")
	hub.Publish(Update{Reason: "changed"})
	if update := hub.SyncUpdate(); update.Revision != 1 || update.BuildID != "v2-restarted" || update.Reason != "sync" {
		t.Fatalf("sync update = %#v", update)
	}
}

func TestHubSyncChangesBuildAfterServerRestart(t *testing.T) {
	before := New("v1-old").SyncUpdate()
	after := New("v2-new").SyncUpdate()
	if before.Revision != 0 || after.Revision != 0 || before.BuildID == after.BuildID {
		t.Fatalf("before = %#v, after = %#v", before, after)
	}
}

func TestHubCloseDisconnectsExistingAndFutureSubscribers(t *testing.T) {
	hub := New("v2-test")
	existing, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	hub.Close()
	hub.Close()
	assertClosedSubscriber(t, existing)

	future, unsubscribeFuture := hub.Subscribe()
	defer unsubscribeFuture()
	assertClosedSubscriber(t, future)
	hub.Publish(Update{Reason: "ignored"})
}

func TestHubConcurrentPublishPreservesRevisionOrder(t *testing.T) {
	const publishers = subscriberBuffer
	hub := New("v2-test")
	updates, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	start := make(chan struct{})
	var group sync.WaitGroup
	group.Add(publishers)
	for index := 0; index < publishers; index++ {
		go func() {
			defer group.Done()
			<-start
			hub.Publish(Update{Reason: "concurrent"})
		}()
	}
	close(start)
	group.Wait()
	for want := uint64(1); want <= publishers; want++ {
		if update := <-updates; update.Revision != want {
			t.Fatalf("revision = %d, want %d", update.Revision, want)
		}
	}
}

func assertClosedSubscriber(t *testing.T, subscriber <-chan Update) {
	t.Helper()
	if _, open := <-subscriber; open {
		t.Fatal("subscriber remained open after hub shutdown")
	}
}
