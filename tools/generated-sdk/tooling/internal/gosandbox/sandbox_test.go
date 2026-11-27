package gosandbox

import (
	"os"
	"runtime"
	"slices"
	"testing"
)

func TestCompileEnvironmentOverridesAmbientGoSettings(t *testing.T) {
	t.Setenv("GOARCH", "ambient-arch")
	t.Setenv("GOENV", "/tmp/ambient-goenv")
	t.Setenv("GOEXPERIMENT", "ambient-experiment")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOROOT", "/tmp/ambient-goroot")
	t.Setenv("GOOS", "ambient-os")
	t.Setenv("GOPROXY", "https://ambient.invalid")
	t.Setenv("GOTOOLCHAIN", "auto")
	got := CompileEnvironment("-mod=vendor")
	for _, want := range []string{
		"CGO_ENABLED=0",
		"GOARCH=" + runtime.GOARCH,
		"GOENV=off",
		"GOEXPERIMENT=",
		"GOFLAGS=-mod=vendor",
		"GOOS=" + runtime.GOOS,
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("environment %v does not contain %q", got, want)
		}
	}
	for _, unwanted := range []string{
		"GOARCH=ambient-arch",
		"GOENV=/tmp/ambient-goenv",
		"GOEXPERIMENT=ambient-experiment",
		"GOFLAGS=-mod=mod",
		"GOROOT=/tmp/ambient-goroot",
		"GOOS=ambient-os",
		"GOPROXY=https://ambient.invalid",
		"GOTOOLCHAIN=auto",
	} {
		if slices.Contains(got, unwanted) {
			t.Fatalf("environment retained %q: %v", unwanted, got)
		}
	}
	if os.Getenv("GOFLAGS") != "-mod=mod" {
		t.Fatal("CompileEnvironment mutated the process environment")
	}
}
