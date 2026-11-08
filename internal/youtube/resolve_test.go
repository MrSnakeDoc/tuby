package youtube

import "testing"

func TestChannelIDFromPage(t *testing.T) {
	page := `<link rel="alternate" type="application/rss+xml" title="RSS" href="https://www.youtube.com/feeds/videos.xml?channel_id=UCHnyfMqiRRG1u-2MsSQLbXA">`
	channelID, err := channelIDFromPage(page)
	if err != nil {
		t.Fatal(err)
	}
	if channelID != "UCHnyfMqiRRG1u-2MsSQLbXA" {
		t.Fatalf("channel ID = %q", channelID)
	}
}
