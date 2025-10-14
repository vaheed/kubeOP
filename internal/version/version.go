package version

// Default build metadata; overridden via -ldflags when the binary is built.
var (
	Version = "0.8.2"
	Commit  = ""
	Date    = ""
)
