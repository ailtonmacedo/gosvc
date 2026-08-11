package buildinfo

// These variables are intentionally mutable so release builds can inject
// values with -ldflags -X.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)
