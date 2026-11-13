// Package buildinfo holds what packaging/build.sh stamps into every binary
// with -ldflags "-X github.com/Bank5y/mokoup2p/buildinfo.Version=…".
package buildinfo

// Version is the release version without a leading "v" (e.g. "0.1.0"), or
// DevVersion for a build that did not go through packaging/build.sh.
var Version = DevVersion

// Repo is the GitHub repository ("owner/name") whose releases are checked
// for updates; forks override it.
var Repo = "Bank5y/mokoup2p"

const DevVersion = "dev"

// ReleasesURL is GitHub's "list releases" endpoint for Repo.
func ReleasesURL() string { return "https://api.github.com/repos/" + Repo + "/releases" }
