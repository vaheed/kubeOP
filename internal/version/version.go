package version

// Version is the semantic version for this build; default overridden at build time.
var Version = "dev"

// Build is the VCS revision or build metadata; may be empty.
var Build = ""

func Full() string {
	if Build == "" {
		return Version
	}
	return Version + "+" + Build
}
