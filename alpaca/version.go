package alpaca

import (
	"runtime/debug"
	"strings"
	"sync"
)

const repoName = "github.com/alpacahq/alpaca-trade-api-go"

var (
	once            = sync.Once{}
	encodedVersions string
)

// GetVersion returns running go version and alpaca-trade-api-go version
func GetVersion() string {
	once.Do(func() {
		var (
			goVersion     string
			moduleVersion string
		)

		buildInfo, found := debug.ReadBuildInfo()
		if !found {
			return
		}
		goVersion = buildInfo.GoVersion
		for _, dep := range buildInfo.Deps {
			if strings.HasPrefix(dep.Path, repoName) {
				moduleVersion = dep.Version
				break
			}
		}
		encodedVersions = "APCA-GO/" + moduleVersion + "/" + goVersion
	})
	return encodedVersions
}
