package youtube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxFeedBytes = 4 << 20

type (
	FeedClient   struct{ client *http.Client }
	FetchOptions struct{ ETag, LastModified string }
	FetchResult  struct {
		Feed               Feed
		ETag, LastModified string
		NotModified        bool
	}
)

type FetchError struct {
	StatusCode int
	Temporary  bool
	err        error
}

func (e *FetchError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("YouTube RSS status %d", e.StatusCode)
	}
	return fmt.Sprintf("fetch YouTube RSS feed: %v", e.err)
}

func (e *FetchError) Unwrap() error { return e.err }

func IsTemporary(errorValue error) bool {
	var fetchError *FetchError
	return errors.As(errorValue, &fetchError) && fetchError.Temporary
}

func NewFeedClient(client *http.Client) *FeedClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &FeedClient{client: client}
}

func (c *FeedClient) Fetch(ctx context.Context, channelID string, options FetchOptions) (FetchResult, error) {
	if !validChannelID(channelID) {
		return FetchResult{}, fmt.Errorf("invalid YouTube channel ID")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.youtube.com/feeds/videos.xml?channel_id="+channelID, http.NoBody)
	if err != nil {
		return FetchResult{}, fmt.Errorf("create YouTube RSS request: %w", err)
	}
	request.Header.Set("Accept", "application/atom+xml, application/xml")
	request.Header.Set("User-Agent", "Tuby/1.0")
	if options.ETag != "" {
		request.Header.Set("If-None-Match", options.ETag)
	}
	if options.LastModified != "" {
		request.Header.Set("If-Modified-Since", options.LastModified)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return FetchResult{}, &FetchError{Temporary: true, err: err}
	}
	defer func() { _ = response.Body.Close() }()
	result := FetchResult{ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified")}
	if response.StatusCode == http.StatusNotModified {
		result.NotModified = true
		return result, nil
	}
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
		return FetchResult{}, &FetchError{StatusCode: response.StatusCode, Temporary: true}
	}
	if response.StatusCode != http.StatusOK {
		return FetchResult{}, &FetchError{StatusCode: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxFeedBytes))
	if err != nil {
		return FetchResult{}, fmt.Errorf("read YouTube RSS feed: %w", err)
	}
	feed, err := ParseFeed(body)
	if err != nil {
		return FetchResult{}, err
	}
	if feed.ChannelID != channelID {
		return FetchResult{}, fmt.Errorf("YouTube RSS feed returned another channel")
	}
	return FetchResult{Feed: feed, ETag: result.ETag, LastModified: result.LastModified}, nil
}
