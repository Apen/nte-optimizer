package buildinfo

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

func Current() Info {
	return Info{Version: Version, Commit: Commit, BuildDate: BuildDate}
}

func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildDate)
}

func WindowTitle(applicationName string) string {
	if Version == "" || Version == "dev" {
		return applicationName
	}
	return applicationName + " " + Version
}
