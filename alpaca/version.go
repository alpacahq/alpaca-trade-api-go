package alpaca

import (
	"runtime"
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
		buildInfo, found := debug.ReadBuildInfo()
		if !found {
			return
		}
		for _, dep := range buildInfo.Deps {
			if strings.HasPrefix(dep.Path, repoName) {
				encodedVersions += "APCA-GO/" + dep.Version + " "
				break
			}
		}
		encodedVersions += "GoRuntime/" + runtime.Version()
	})
	return encodedVersions
}
