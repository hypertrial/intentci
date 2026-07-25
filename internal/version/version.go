package version

// Version is set via ldflags at release time.
var Version = "0.2.0-dev"

// String returns the version string.
func String() string {
	if Version == "" {
		return "0.2.0-dev"
	}
	return Version
}
