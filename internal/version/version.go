package version

const DefaultVersion = "2.0.3"

var Version = DefaultVersion

func String() string {
	if Version == "" {
		return DefaultVersion
	}
	return Version
}
