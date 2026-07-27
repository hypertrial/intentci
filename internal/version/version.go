package version

var Version = "2.0.0"

func String() string {
	if Version == "" {
		return "2.0.0"
	}
	return Version
}
