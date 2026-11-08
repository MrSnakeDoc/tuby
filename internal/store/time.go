package store

import "time"

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
