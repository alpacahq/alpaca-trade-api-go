package alpaca

import (
	"runtime/debug"
	"strings"
	"sync"
)

const repoName = "github.com/alpacahq/alpaca-trade-api-go"

var (
	goVersion     string
	moduleVersion string
	once          = sync.Once{}
)

// GetVersion returns running go version and alpaca-trade-api-go version
func GetVersion() (string, string) {
	once.Do(func() {
		buildInfo, found := debug.ReadBuildInfo()
		if !found {
			return
		}
		goVersion = buildInfo.GoVersion

		for _, dep := range buildInfo.Deps {
			if strings.HasPrefix(dep.Path, repoName) {
				moduleVersion = dep.Version
				return
			}
		}
	})
	return goVersion, moduleVersion
}
