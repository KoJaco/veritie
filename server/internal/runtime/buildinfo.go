package runtime

// Build-time values, typically set via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// BuildInfo captures build metadata.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// CurrentBuildInfo returns current build metadata.
func CurrentBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
	}
}
