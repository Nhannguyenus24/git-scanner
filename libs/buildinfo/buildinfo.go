// Package buildinfo reports which build of a service is running.
//
// During a rollout several versions of a workload run at once. Logging and
// serving the version is how an operator tells which pod produced a log line or
// a finding, without guessing from image tags.
package buildinfo

import "runtime/debug"

// Set at link time by the Makefile and the Dockerfile:
//
//	-ldflags "-X github.com/mservice/gitscanner/libs/buildinfo.version=v1.2.3
//	          -X github.com/mservice/gitscanner/libs/buildinfo.commit=abc1234"
var (
	version = "dev"
	commit  = ""
)

// Info is the build identity of the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

// Get returns the build identity. When the commit was not injected at link
// time it falls back to the VCS stamp the Go toolchain records for builds made
// inside a git checkout, so `go run` still reports something useful.
func Get() Info {
	info := Info{Version: version, Commit: commit}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	info.GoVersion = bi.GoVersion

	if info.Commit == "" {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				info.Commit = s.Value
			}
		}
	}
	return info
}
