package version

// Version is set via ldflags at release time.
var Version = "1.0.0-dev"

// String returns the version string.
func String() string {
	if Version == "" {
		return "1.0.0-dev"
	}
	return Version
}
