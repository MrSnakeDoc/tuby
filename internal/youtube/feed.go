package youtube

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

type Feed struct {
	ChannelID string
	Title     string
	AvatarURL string
	Videos    []Video
}

type Video struct {
	YouTubeID string
	ChannelID string
	Title     string
	URL       string
	Thumbnail string
	Published time.Time
	IsShort   bool
}

type atomFeed struct {
	XMLName   xml.Name    `xml:"feed"`
	ChannelID string      `xml:"channelId"`
	Title     string      `xml:"title"`
	Logo      string      `xml:"logo"`
	Links     []atomLink  `xml:"link"`
	Entries   []atomEntry `xml:"entry"`
}

type atomEntry struct {
	VideoID   string     `xml:"videoId"`
	ChannelID string     `xml:"channelId"`
	Title     string     `xml:"title"`
	Published time.Time  `xml:"published"`
	Links     []atomLink `xml:"link"`
	Thumbnail mediaThumb `xml:"group>thumbnail"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type mediaThumb struct {
	URL string `xml:"url,attr"`
}

func ParseFeed(document []byte) (Feed, error) {
	var source atomFeed
	if err := xml.Unmarshal(document, &source); err != nil {
		return Feed{}, fmt.Errorf("parse YouTube RSS feed: %w", err)
	}
	channelID := feedChannelID(source)
	if !validChannelID(channelID) {
		return Feed{}, fmt.Errorf("YouTube RSS feed has an invalid channel ID")
	}
	feed := Feed{ChannelID: channelID, Title: strings.TrimSpace(source.Title), AvatarURL: strings.TrimSpace(source.Logo)}
	seen := make(map[string]struct{}, len(source.Entries))
	for _, entry := range source.Entries {
		video, ok := parseEntry(entry, feed.ChannelID)
		if ok && !containsVideo(seen, video.YouTubeID) {
			feed.Videos = append(feed.Videos, video)
		}
	}
	return feed, nil
}

func feedChannelID(source atomFeed) string {
	if validChannelID(source.ChannelID) {
		return source.ChannelID
	}
	for _, link := range source.Links {
		if link.Rel == "alternate" {
			if id := channelIDFromURL(link.Href); id != "" {
				return id
			}
		}
	}
	return ""
}

func channelIDFromURL(value string) string {
	const prefix = "https://www.youtube.com/channel/"
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	id := strings.TrimPrefix(value, prefix)
	if validChannelID(id) {
		return id
	}
	return ""
}

func containsVideo(seen map[string]struct{}, videoID string) bool {
	if _, exists := seen[videoID]; exists {
		return true
	}
	seen[videoID] = struct{}{}
	return false
}

func parseEntry(entry atomEntry, channelID string) (Video, bool) {
	if entry.ChannelID != channelID || strings.TrimSpace(entry.VideoID) == "" || strings.TrimSpace(entry.Title) == "" || entry.Published.IsZero() {
		return Video{}, false
	}
	url := alternateURL(entry.Links)
	if url == "" {
		return Video{}, false
	}
	return Video{
		YouTubeID: entry.VideoID, ChannelID: channelID, Title: strings.TrimSpace(entry.Title),
		URL: url, Thumbnail: strings.TrimSpace(entry.Thumbnail.URL), Published: entry.Published.UTC(),
		IsShort: isShortURL(url),
	}, true
}

func isShortURL(value string) bool {
	return strings.Contains(value, "/shorts/")
}

func alternateURL(links []atomLink) string {
	for _, link := range links {
		if link.Rel == "alternate" && link.Href != "" {
			return link.Href
		}
	}
	return ""
}
