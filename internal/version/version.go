package version

// Default build metadata; overridden via -ldflags when the binary is built.
var (
    Version = "0.8.6"
	Commit  = ""
	Date    = ""
)
