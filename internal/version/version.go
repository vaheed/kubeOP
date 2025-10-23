package version

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"golang.org/x/mod/semver"
)

var (
	rawVersion = "0.15.2"
	rawCommit  = ""
	rawDate    = ""
)

const fallbackDevVersion = "0.0.0-dev"

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
	return deriveMetadata(Build{
		Version: rawVersion,
		Commit:  rawCommit,
		Date:    rawDate,
	})
}

// FromStrings derives build metadata from raw string inputs. It mirrors the
// runtime behaviour and is primarily exposed for tests.
func FromStrings(version, commit, date string) (Info, error) {
	return deriveMetadata(Build{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
}

func deriveMetadata(raw Build) (Info, error) {
	build := Build{
		Version: strings.TrimSpace(raw.Version),
		Commit:  strings.TrimSpace(raw.Commit),
		Date:    strings.TrimSpace(raw.Date),
	}
	if build.Version == "" {
		return Info{}, fmt.Errorf("version metadata: version is required")
	}
	normalized := normalizeSemver(build.Version)
	if normalized == "" {
		fallback := fallbackDevVersion
		if normalizedFallback := normalizeSemver(fallback); normalizedFallback != "" {
			fallback = normalizedFallback
		}
		logFallback(build.Version, fallback)
		build.Version = fallback
	} else {
		build.Version = normalized
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

func logFallback(invalid, fallback string) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	logger.Printf("version metadata: falling back to %q because %q is not a valid semantic version", fallback, invalid)
}
