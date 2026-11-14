package home

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrSnakeDoc/tuby/internal/api/deps"
	"github.com/MrSnakeDoc/tuby/internal/store"
	"github.com/MrSnakeDoc/tuby/internal/web"
)

func TestVideoCardKeepsTimerAndRendersKnownDurationInFooter(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "tuby.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	channel := store.Channel{YouTubeID: "UC123", SourceURL: "https://youtube.com/@example", Title: "Example"}
	if err := database.SaveChannel(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	video := store.Video{
		YouTubeID: "dQw4w9WgXcQ", ChannelYouTubeID: channel.YouTubeID, Title: "Video",
		URL: "https://youtube.com/watch?v=dQw4w9WgXcQ", PublishedAt: time.Now().UTC(),
	}
	if err := database.SaveVideo(context.Background(), video); err != nil {
		t.Fatal(err)
	}
	if err := database.SetVideoDuration(context.Background(), video.YouTubeID, 754); err != nil {
		t.Fatal(err)
	}
	renderer, err := web.New()
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(deps.Deps{Templates: renderer, Store: database, BuildID: "test-build"})
	request := httptest.NewRequest(http.MethodGet, "/channels/UC123", http.NoBody)
	response := httptest.NewRecorder()
	handler.execute(response, request)
	body := response.Body.String()
	for _, expected := range []string{
		`class="duration-button"`, `/api/videos/dQw4w9WgXcQ/duration`,
		`timer.svg?v=test-build`, `<time class="video-duration">12:34</time>`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("video card does not contain %q", expected)
		}
	}
}
