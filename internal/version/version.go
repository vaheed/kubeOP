package version

// Default build metadata; overridden via -ldflags when the binary is built.
var (
	// Version is the semantic version baked into the binary by default.
	Version = "0.14.5"
	// Commit is the git SHA injected by the build pipeline.
	Commit = ""
	// Date is the build timestamp injected by the build pipeline.
	Date = ""
)
