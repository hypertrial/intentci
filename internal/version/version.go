package version

var Version = "2.0.1"

func String() string {
	if Version == "" {
		return "2.0.1"
	}
	return Version
}
