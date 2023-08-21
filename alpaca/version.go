package alpaca

import (
	"runtime"
	"runtime/debug"
	"strings"
)

const repoName = "github.com/alpacahq/alpaca-trade-api-go"

var version = initVersion()

func initVersion() string {
	var moduleVersion string
	buildInfo, found := debug.ReadBuildInfo()
	if !found {
		return "GoRuntime/" + runtime.Version()
	}
	for _, dep := range buildInfo.Deps {
		if strings.HasPrefix(dep.Path, repoName) {
			moduleVersion += "APCA-GO/" + dep.Version + " "
			break
		}
	}
	return moduleVersion + "GoRuntime/" + runtime.Version()
}

// Version returns a string contains alpaca-trade-api-go dep version and go runtime version
func Version() string {
	return version
}
