package version

// Default build metadata; overridden via -ldflags when the binary is built.
var (
    Version = "0.12.5"
	Commit  = ""
	Date    = ""
)
