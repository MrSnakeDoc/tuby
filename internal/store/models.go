package store

import "time"

type Channel struct {
	YouTubeID          string
	CategoryID         int64
	Position           int
	SourceURL          string
	Title              string
	AvatarURL          string
	FeedETag           string
	FeedLastModified   string
	LastCheckedAt      *time.Time
	LastSuccessfulSync *time.Time
	NextCheckAt        *time.Time
	FailureCount       int
	LastError          string
	IncludeLong        bool
	IncludeShorts      bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Category struct {
	ID        int64
	Name      string
	Position  int
	IsDefault bool
}

type Video struct {
	YouTubeID        string
	ChannelYouTubeID string
	Title            string
	URL              string
	ThumbnailURL     string
	PublishedAt      time.Time
	DiscoveredAt     time.Time
	LastSeenAt       time.Time
	Unseen           bool
	WatchLater       bool
	WatchedAt        *time.Time
	IsShort          bool
}

type ChannelSummary struct {
	Channel
	VideoCount    int
	UnseenCount   int
	ImageURL      string
	ThumbnailURLs []string
}
