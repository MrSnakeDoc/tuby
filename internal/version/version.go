package version

import "strings"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
	GoVersion = "unknown"
)

func BuildID() string {
	return strings.TrimSpace(Version) + "-" + strings.TrimSpace(Commit)
}
