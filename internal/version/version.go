package version

var Version = "2.0.2"

func String() string {
	if Version == "" {
		return "2.0.2"
	}
	return Version
}
