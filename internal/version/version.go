package version

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/mod/semver"
)

var (
	rawVersion = "0.14.1"
	rawCommit  = ""
	rawDate    = ""
)

// Build describes the immutable build metadata baked into the binary.
type Build struct {
	Version string
	Commit  string
	Date    string
}

// Info aggregates build metadata exposed by the API and CLI.
type Info struct {
	Build Build
}

var (
	metadata Info
	once     sync.Once
)

func init() {
	once.Do(func() {
		info, err := buildMetadata()
		if err != nil {
			panic(err)
		}
		metadata = info
	})
}

// Metadata returns a copy of the build metadata for the running binary.
func Metadata() Info {
	return metadata
}

func buildMetadata() (Info, error) {
	build := Build{
		Version: strings.TrimSpace(rawVersion),
		Commit:  strings.TrimSpace(rawCommit),
		Date:    strings.TrimSpace(rawDate),
	}
	if build.Version == "" {
		return Info{}, fmt.Errorf("version metadata: version is required")
	}
	build.Version = normalizeSemver(build.Version)
	if build.Version == "" {
		return Info{}, fmt.Errorf("version metadata: invalid semantic version")
	}

	return Info{Build: build}, nil
}

func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "v") {
		v = v[1:]
	}
	if !semver.IsValid(withPrefix(v)) {
		return ""
	}
	return v
}

func withPrefix(v string) string {
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
