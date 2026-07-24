package alpaca

import (
	"runtime"
	"runtime/debug"
	"strings"
)

const repoName = "github.com/alpacahq/alpaca-trade-api-go"

var version = initVersion()

// initVersion builds the User-Agent value in the format:
// "APCA-GO/<sdk-version> GoRuntime/<runtime-version>", e.g. "APCA-GO/3.11.0 GoRuntime/1.21".
func initVersion() string {
	return "APCA-GO/" + sdkVersion() + " GoRuntime/" + runtimeVersion()
}

// sdkVersion returns the version of this SDK module, stripped of its leading "v"
// (e.g. "3.11.0"). It falls back to "unknown" when the version can't be determined,
// such as when the code isn't built as a module (e.g. via `go run`).
func sdkVersion() string {
	buildInfo, found := debug.ReadBuildInfo()
	if !found {
		return "unknown"
	}
	if strings.HasPrefix(buildInfo.Main.Path, repoName) && isValidVersion(buildInfo.Main.Version) {
		return strings.TrimPrefix(buildInfo.Main.Version, "v")
	}
	for _, dep := range buildInfo.Deps {
		if strings.HasPrefix(dep.Path, repoName) {
			return strings.TrimPrefix(dep.Version, "v")
		}
	}
	return "unknown"
}

func isValidVersion(v string) bool {
	return v != "" && v != "(devel)"
}

// runtimeVersion returns the Go runtime version without its leading "go"
// (e.g. "1.21" from "go1.21").
func runtimeVersion() string {
	return strings.TrimPrefix(runtime.Version(), "go")
}

// Version returns the User-Agent string sent with every request, containing the
// alpaca-trade-api-go SDK version and the Go runtime version.
func Version() string {
	return version
}
