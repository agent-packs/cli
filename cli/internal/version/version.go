package version

// Set at link time via -ldflags; defaults for dev builds.
var (
	Version = "dev"
	Commit  = "none"
)

func String() string {
	if Commit != "" && Commit != "none" {
		return Version + " (" + Commit + ")"
	}
	return Version
}
