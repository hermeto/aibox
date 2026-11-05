// Package version holds build information injected at link time by GoReleaser:
//
//	-X github.com/hermeto/aibox/internal/version.Version=...
package version

// Build information. Defaults identify a development build.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the structured form of the build information.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Get returns the current build information.
func Get() Info { return Info{Version: Version, Commit: Commit, Date: Date} }
