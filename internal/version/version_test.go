package version

import "testing"

func TestBuildIDUsesVersionAndCommit(t *testing.T) {
	previousVersion, previousCommit := Version, Commit
	t.Cleanup(func() {
		Version = previousVersion
		Commit = previousCommit
	})
	Version = " v2.0.0 "
	Commit = " abc123 "
	if buildID := BuildID(); buildID != "v2.0.0-abc123" {
		t.Fatalf("BuildID() = %q", buildID)
	}
}
