package version

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

var (
	rawVersion             = "0.8.27"
	rawCommit              = ""
	rawDate                = ""
	rawMinClientVersion    = "0.8.16"
	rawMinAPIVersion       = "v1"
	rawMaxAPIVersion       = "v1"
	rawDeprecationNote     = ""
	rawDeprecationDeadline = ""
)

// Build describes the immutable build metadata baked into the binary.
type Build struct {
	Version string
	Commit  string
	Date    string
}

// Compatibility captures supported client and API ranges.
type Compatibility struct {
	MinClientVersion string
	MinAPIVersion    string
	MaxAPIVersion    string
}

// Deprecation advertises sunset information for the current build.
type Deprecation struct {
	Deadline string
	Note     string

	parsedDeadline *time.Time
}

// Info aggregates all version metadata exposed by the API and CLI.
type Info struct {
	Build         Build
	Compatibility Compatibility
	Deprecation   *Deprecation
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

// Deprecated reports whether the current build is past its advertised deadline.
func (i Info) Deprecated(at time.Time) bool {
	if i.Deprecation == nil || i.Deprecation.parsedDeadline == nil {
		return false
	}
	deadline := *i.Deprecation.parsedDeadline
	return !at.Before(deadline)
}

// DeadlineTime returns the parsed deprecation deadline, if present.
func (i Info) DeadlineTime() (time.Time, bool) {
	if i.Deprecation == nil || i.Deprecation.parsedDeadline == nil {
		return time.Time{}, false
	}
	return *i.Deprecation.parsedDeadline, true
}

// SupportsClient reports whether the provided SemVer string is within the
// supported client range.
func (i Info) SupportsClient(v string) bool {
	v = normalizeSemver(strings.TrimSpace(v))
	if v == "" || i.Compatibility.MinClientVersion == "" {
		return true
	}
	return semver.Compare(withPrefix(v), withPrefix(i.Compatibility.MinClientVersion)) >= 0
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

	compat := Compatibility{
		MinClientVersion: normalizeSemver(strings.TrimSpace(rawMinClientVersion)),
		MinAPIVersion:    strings.TrimSpace(rawMinAPIVersion),
		MaxAPIVersion:    strings.TrimSpace(rawMaxAPIVersion),
	}
	if compat.MinClientVersion != "" {
		if semver.Compare(withPrefix(build.Version), withPrefix(compat.MinClientVersion)) < 0 {
			return Info{}, fmt.Errorf("version metadata: build version %s is older than supported min client %s", build.Version, compat.MinClientVersion)
		}
	}

	var dep *Deprecation
	note := strings.TrimSpace(rawDeprecationNote)
	deadline := strings.TrimSpace(rawDeprecationDeadline)
	if note != "" || deadline != "" {
		dep = &Deprecation{Deadline: deadline, Note: note}
		if deadline != "" {
			t, err := time.Parse(time.RFC3339, deadline)
			if err != nil {
				return Info{}, fmt.Errorf("version metadata: invalid deprecation deadline: %w", err)
			}
			t = t.UTC()
			dep.parsedDeadline = &t
		}
	}

	return Info{
		Build:         build,
		Compatibility: compat,
		Deprecation:   dep,
	}, nil
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
		return ""
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
