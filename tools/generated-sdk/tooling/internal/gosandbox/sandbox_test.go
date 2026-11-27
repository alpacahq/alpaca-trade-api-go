package gosandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func TestPrepareReadOnlyMountMakesModuleReadableForContainerUser(t *testing.T) {
	parent, err := os.MkdirTemp("", "generated-sdk-mount-parent-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(parent)
	})
	moduleDir := filepath.Join(parent, "module")
	nestedDir := filepath.Join(moduleDir, "pkg")
	if err := os.MkdirAll(nestedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(moduleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(nestedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(nestedDir, "policy_test.go")
	if err := os.WriteFile(testFile, []byte("package pkg\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareReadOnlyMount(moduleDir); err != nil {
		t.Fatal(err)
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if parentInfo.Mode().Perm() != 0o700 {
		t.Fatalf("parent scratch dir perm = %04o, want 0700", parentInfo.Mode().Perm())
	}
	for _, dir := range []string{moduleDir, nestedDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o005 != 0o005 {
			t.Fatalf("mounted dir %s perm = %04o, want world read+execute", dir, info.Mode().Perm())
		}
	}
	fileInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm()&0o004 != 0o004 {
		t.Fatalf("mounted file %s perm = %04o, want world readable", testFile, fileInfo.Mode().Perm())
	}
}

func TestPrepareReadOnlyMountRejectsSymlink(t *testing.T) {
	moduleDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte("package secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(moduleDir, "link.go")); err != nil {
		t.Fatal(err)
	}
	if err := prepareReadOnlyMount(moduleDir); err == nil {
		t.Fatal("prepareReadOnlyMount accepted symlinked file")
	}
}

func TestVendorDoesNotLeaveBuildCacheInsideModuleDir(t *testing.T) {
	t.Setenv("GOMODCACHE", t.TempDir())
	moduleDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(moduleDir, "go.mod"),
		[]byte("module example.com/vendor-clean\n\ngo 1.23.0\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "go.sum"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "doc.go"), []byte("package clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	if err := Vendor(context.Background(), goCommand, moduleDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(moduleDir, ".vendor-cache")); !os.IsNotExist(err) {
		t.Fatalf("unexpected .vendor-cache inside moduleDir: %v", err)
	}
}
