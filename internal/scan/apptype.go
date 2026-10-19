package scan

import "path"

// appTypeDetector infers a repository's stack from the manifest files it
// contains (plan.md §4.3). Stack feeds severity: a leaked DB URL in a deployed
// Go or Node service outranks the same string in a tutorial repo.
//
// It records the first manifest seen for each stack and reports the
// highest-priority one, so a repo with both a Dockerfile and a package.json is
// classified by the stronger signal.
type appTypeDetector struct {
	seen map[string]bool
}

func newAppTypeDetector() *appTypeDetector {
	return &appTypeDetector{seen: map[string]bool{}}
}

// manifests maps a manifest filename to the stack it indicates, in priority
// order: the first entry whose file is present wins.
var manifests = []struct{ file, stack string }{
	{"dockerfile", "container"},
	{"go.mod", "go"},
	{"package.json", "node"},
	{"requirements.txt", "python"},
	{"pyproject.toml", "python"},
	{"pom.xml", "java"},
	{"build.gradle", "java"},
	{"gemfile", "ruby"},
	{"cargo.toml", "rust"},
	{"composer.json", "php"},
}

func (d *appTypeDetector) observe(p string) {
	base := toLower(path.Base(p))
	for _, m := range manifests {
		if base == m.file {
			d.seen[m.stack] = true
			return
		}
	}
}

func (d *appTypeDetector) result() string {
	for _, m := range manifests {
		if d.seen[m.stack] {
			return m.stack
		}
	}
	return ""
}

// toLower avoids pulling strings in for one call on short manifest names.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
