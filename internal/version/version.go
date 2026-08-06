// Package version holds build-time version information injected via ldflags.
package version

// These variables are set at build time by the Makefile / GoReleaser via
// -ldflags "-X github.com/chrisophus/clipd/internal/version.Version=..."
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
