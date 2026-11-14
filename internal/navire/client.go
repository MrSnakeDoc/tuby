package navire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	templateKey     = "tuby_video"
	maxResponseBody = 1 << 20
)

type Event struct {
	Variant     string
	Priority    string
	ChannelName string
	VideoTitle  string
	Message     string
}

type Client struct {
	url    string
	client *http.Client
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("Navire returned HTTP status %d", e.StatusCode)
	}
	return fmt.Sprintf("Navire returned HTTP status %d: %s", e.StatusCode, e.Body)
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		url:    strings.TrimRight(baseURL, "/") + "/api/v1/events",
		client: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Publish(ctx context.Context, event Event) error {
	body, err := json.Marshal(eventRequest{
		Key: templateKey, Variant: event.Variant, Priority: event.Priority,
		Data: eventData{ChannelName: event.ChannelName, VideoTitle: event.VideoTitle, Message: event.Message},
	})
	if err != nil {
		return fmt.Errorf("encode Navire event: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Navire request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Tuby/1.0")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("publish Navire event: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("read Navire response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return &HTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	return nil
}

type eventRequest struct {
	Key      string    `json:"key"`
	Variant  string    `json:"variant"`
	Priority string    `json:"priority"`
	Data     eventData `json:"data"`
}

type eventData struct {
	ChannelName string `json:"channel_name"`
	VideoTitle  string `json:"video_title"`
	Message     string `json:"message,omitempty"`
}
