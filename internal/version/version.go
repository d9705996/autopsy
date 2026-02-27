// Package version holds build-time variables injected by goreleaser ldflags.
package version

// These vars are overwritten at link time:
//   -X github.com/d9705996/autopsy/internal/version.Version=v1.2.3
//   -X github.com/d9705996/autopsy/internal/version.Commit=abc1234
//   -X github.com/d9705996/autopsy/internal/version.Date=2026-02-26T00:00:00Z
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
