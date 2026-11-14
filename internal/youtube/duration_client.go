package youtube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

const maxVideoPageBytes = 12 << 20

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

type DurationClient struct {
	client *http.Client
}

func NewDurationClient(client *http.Client) *DurationClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &DurationClient{client: client}
}

func (c *DurationClient) FetchDuration(ctx context.Context, videoID string) (int64, error) {
	if !videoIDPattern.MatchString(videoID) {
		return 0, fmt.Errorf("invalid YouTube video ID")
	}
	pageURL := "https://www.youtube.com/watch?" + url.Values{"v": []string{videoID}, "hl": []string{"en"}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return 0, fmt.Errorf("create YouTube video request: %w", err)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("User-Agent", "Tuby/1.0")
	response, err := c.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("fetch YouTube video page: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("fetch YouTube video page: unexpected status %d", response.StatusCode)
	}
	page, err := io.ReadAll(io.LimitReader(response.Body, maxVideoPageBytes+1))
	if err != nil {
		return 0, fmt.Errorf("read YouTube video page: %w", err)
	}
	if len(page) > maxVideoPageBytes {
		return 0, fmt.Errorf("YouTube video page exceeds %d bytes", maxVideoPageBytes)
	}
	seconds, err := ParseVideoDuration(page)
	if err != nil {
		return 0, fmt.Errorf("parse YouTube video duration: %w", err)
	}
	return seconds, nil
}
