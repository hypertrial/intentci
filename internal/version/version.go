package version

import "runtime/debug"

// Version is set via ldflags at release time.
var Version = "0.1.0-dev"

// String returns the version string.
func String() string {
	if Version != "" && Version != "0.1.0-dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return Version
}
