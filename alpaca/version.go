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
	if isSDKModulePath(buildInfo.Main.Path) && isValidVersion(buildInfo.Main.Version) {
		return strings.TrimPrefix(buildInfo.Main.Version, "v")
	}
	for _, dep := range buildInfo.Deps {
		if isSDKModulePath(dep.Path) && isValidVersion(dep.Version) {
			return strings.TrimPrefix(dep.Version, "v")
		}
	}
	return "unknown"
}

// isSDKModulePath reports whether path is this SDK's module path, either the
// canonical form (repoName) or a subsequent major-version form such as
// "repoName/v3". It requires an exact match (or exact match up to a numeric
// "/vN" suffix) so that unrelated modules sharing repoName as a prefix, e.g.
// "github.com/alpacahq/alpaca-trade-api-go-extra", aren't misclassified.
func isSDKModulePath(path string) bool {
	if path == repoName {
		return true
	}
	suffix, ok := strings.CutPrefix(path, repoName+"/v")
	if !ok || suffix == "" {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
