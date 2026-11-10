package youtube

import (
	"strings"
	"testing"
	"time"
)

func TestParseFeedPreservesVideosAndDeduplicatesIDs(t *testing.T) {
	feed, err := ParseFeed([]byte(testFeedXML))
	if err != nil {
		t.Fatal(err)
	}
	if feed.ChannelID != "UCHnyfMqiRRG1u-2MsSQLbXA" || feed.Title != "Veritasium" || feed.AvatarURL != "https://example.com/avatar.jpg" {
		t.Fatalf("feed metadata = %#v", feed)
	}
	if len(feed.Videos) != 1 {
		t.Fatalf("videos = %d, want 1", len(feed.Videos))
	}
	video := feed.Videos[0]
	if video.YouTubeID != "abcdefghijk" || video.Title != "A real video" || video.Thumbnail != "https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg" {
		t.Fatalf("video = %#v", video)
	}
	if !video.Published.Equal(time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("published = %s", video.Published)
	}
}

func TestParseFeedRejectsInvalidChannelID(t *testing.T) {
	document := strings.Replace(testFeedXML, "UCHnyfMqiRRG1u-2MsSQLbXA", "invalid", 1)
	if _, err := ParseFeed([]byte(document)); err == nil {
		t.Fatal("ParseFeed succeeded")
	}
}

func TestParseFeedClassifiesShorts(t *testing.T) {
	document := strings.ReplaceAll(testFeedXML, "https://www.youtube.com/watch?v=abcdefghijk", "https://www.youtube.com/shorts/abcdefghijk")
	feed, err := ParseFeed([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Videos) != 1 || !feed.Videos[0].IsShort {
		t.Fatalf("videos = %#v, want one Short", feed.Videos)
	}
}

const testFeedXML = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns:media="http://search.yahoo.com/mrss/">
  <title>Veritasium</title>
  <logo>https://example.com/avatar.jpg</logo>
  <yt:channelId>UCHnyfMqiRRG1u-2MsSQLbXA</yt:channelId>
  <entry>
    <yt:videoId>abcdefghijk</yt:videoId>
    <yt:channelId>UCHnyfMqiRRG1u-2MsSQLbXA</yt:channelId>
    <title>A real video</title>
    <published>2026-11-08T12:00:00+00:00</published>
    <link rel="alternate" href="https://www.youtube.com/watch?v=abcdefghijk"/>
    <media:group><media:thumbnail url="https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg"/></media:group>
  </entry>
  <entry>
    <yt:videoId>abcdefghijk</yt:videoId>
    <yt:channelId>UCHnyfMqiRRG1u-2MsSQLbXA</yt:channelId>
    <title>Duplicate</title>
    <published>2026-11-08T12:00:00+00:00</published>
    <link rel="alternate" href="https://www.youtube.com/watch?v=abcdefghijk"/>
  </entry>
</feed>`
