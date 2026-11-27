package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLoadObjectPreservesJSONNumberIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := filepath.Join(root, "input.json")
	output := filepath.Join(root, "output.json")
	const content = `{"integer":9007199254740993,"precise":0.12345678901234567890123456789}`
	if err := os.WriteFile(input, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := loadObject(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(output, value); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"9007199254740993", "0.12345678901234567890123456789"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("rewritten JSON lost %s: %s", want, got)
		}
	}
}

func TestApplyGeneratedSourcePatches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := `package fixture

func fixture() {
	value = strconv.FormatFloat(v.Float(), 'g', -1, 32)
	if c.cfg.Debug {
		dump, err := httputil.DumpRequestOut(request, true)
		if err != nil {
			return nil, err
		}
		log.Printf("\n%s\n", string(dump))
	}
	if c.cfg.Debug {
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return resp, err
		}
		log.Printf("\n%s\n", string(dump))
	}
}
`
	for _, packageName := range []string{"trading", "marketdata"} {
		dir := filepath.Join(root, packageName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "client.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	activityPath := filepath.Join(root, "trading", "model_get_account_activities_by_activity_type_200_response_inner.go")
	if err := os.WriteFile(
		activityPath,
		[]byte("package trading\n\nfunc fixture() error {\n\treturn fmt.Errorf(\"data failed to match schemas in oneOf(GetAccountActivitiesByActivityType200ResponseInner)\")\n}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := applyGeneratedSourcePatches(root); err != nil {
		t.Fatal(err)
	}
	for _, packageName := range []string{"trading", "marketdata"} {
		data, err := os.ReadFile(filepath.Join(root, packageName, "client.go"))
		if err != nil {
			t.Fatal(err)
		}
		got := string(data)
		for _, want := range []string{
			"v.Type().Bits()",
			`[]string{"APCA-API-KEY-ID", "APCA-API-SECRET-KEY", "Authorization"}`,
			`sanitizedRequest.Header.Set(header, "[REDACTED]")`,
			"httputil.DumpRequestOut(sanitizedRequest, false)",
			"httputil.DumpResponse(resp, false)",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s does not contain %q", packageName, want)
			}
		}
	}
	activity, err := os.ReadFile(activityPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(activity), "unknown non-FILL activity as NonTradeActivities") {
		t.Fatalf("activity fallback patch is missing:\n%s", activity)
	}
}

func TestApplyGeneratedSourcePatchesFailsClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, packageName := range []string{"trading", "marketdata"} {
		dir := filepath.Join(root, packageName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "client.go"), []byte("package fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := applyGeneratedSourcePatches(root)
	if err == nil || !strings.Contains(err.Error(), "source patch matched 0 times, want 1") {
		t.Fatalf("applyGeneratedSourcePatches error = %v", err)
	}
}

func TestDecodeStrictRejectsUnknownAndTrailingJSON(t *testing.T) {
	t.Parallel()
	var value struct {
		Name string `json:"name"`
	}
	if err := decodeStrict([]byte(`{"name":"ok"}`), &value); err != nil {
		t.Fatal(err)
	}
	if err := decodeStrict([]byte(`{"name":"ok","unknown":true}`), &value); err == nil {
		t.Fatal("unknown JSON field unexpectedly passed")
	}
	if err := decodeStrict([]byte(`{"name":"ok"} {}`), &value); err == nil {
		t.Fatal("trailing JSON unexpectedly passed")
	}
}

func TestGenerationPreflightsCoverEveryTrustBoundary(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "repo")
	commands := generationPreflightCommands(root, "linux/arm64/v8")
	if len(commands) != 3 {
		t.Fatalf("generation preflight count = %d, want 3", len(commands))
	}
	want := []string{"generator trust", "pinned specification policy", "template policy"}
	for index, command := range commands {
		if command.name != want[index] {
			t.Fatalf("generation preflight %d = %q, want %q", index, command.name, want[index])
		}
		if !slices.Contains(command.args, "-repo-root") || !slices.Contains(command.args, root) {
			t.Fatalf("generation preflight %q is not repository-bound: %v", command.name, command.args)
		}
	}
	template := commands[2].args
	if !slices.Contains(template, "-platform") || !slices.Contains(template, "linux/arm64/v8") {
		t.Fatalf("template preflight is not platform-bound: %v", template)
	}
}

func TestCandidateStopsAtFailedPreflight(t *testing.T) {
	t.Parallel()
	injected := errors.New("injected preflight failure")
	called := 0
	preflight := func(context.Context, string, string) error {
		called++
		return injected
	}
	_, err := runCandidateWithPreflight(context.Background(), t.TempDir(), "linux/arm64/v8", preflight)
	if !errors.Is(err, injected) {
		t.Fatalf("runCandidateWithPreflight error = %v, want %v", err, injected)
	}
	if called != 1 {
		t.Fatalf("preflight call count = %d, want 1", called)
	}
}

func TestTemporaryWorkspaceCleanupAcrossPipelineOutcomes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "failed generation", err: errors.New("injected generation failure")},
		{name: "failed compilation", err: errors.New("injected compilation failure")},
	} {
		t.Run(test.name, func(t *testing.T) {
			var workspace string
			_, err := withTemporaryWorkspace("generated-sdk-cleanup-test-", func(path string) (candidateResult, error) {
				workspace = path
				if writeErr := os.WriteFile(filepath.Join(path, "temporary"), []byte("temporary"), 0o644); writeErr != nil {
					t.Fatal(writeErr)
				}
				return candidateResult{}, test.err
			})
			if !errors.Is(err, test.err) {
				t.Fatalf("withTemporaryWorkspace error = %v, want %v", err, test.err)
			}
			if _, statErr := os.Stat(workspace); !os.IsNotExist(statErr) {
				t.Fatalf("temporary workspace remains after %s: %v", test.name, statErr)
			}
		})
	}
}

func TestTreeDigestIncludesFinalBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "trading", "client.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package trading\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := treeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package trading\n\nvar Generated = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := treeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("tree digest ignored changed final bytes")
	}
}

func TestGeneratedTreeOperationsRejectSymlinks(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "linked.go")); err != nil {
		t.Fatal(err)
	}
	if err := copyGoFiles(source, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("copyGoFiles followed generated symlink")
	}
	if _, err := fileDigests(source); err == nil {
		t.Fatal("fileDigests followed generated symlink")
	}
}

func TestCompileCandidateUsesIsolatedFixtureModule(t *testing.T) {
	t.Setenv("GOMODCACHE", t.TempDir())
	workspace := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(workspace, "go.mod"),
		[]byte("module example.com/generated-fixture\n\ngo 1.23.0\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "go.sum"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, packageName := range []string{"trading", "marketdata"} {
		dir := filepath.Join(workspace, "generated", packageName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(dir, "client.go"),
			[]byte("package "+packageName+"\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
	}
	version := strings.TrimPrefix(runtime.Version(), "go")
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	if err := vendorCandidateDependencies(context.Background(), goCommand, workspace); err != nil {
		t.Fatal(err)
	}
	if err := compileCandidatePackages(context.Background(), workspace, []string{version}); err != nil {
		t.Fatal(err)
	}
}

func TestGoBinaryForToolchainAcceptsVersionedPathToolchain(t *testing.T) {
	actualGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	localGo := filepath.Join(binDir, "go")
	localVersion := "go version go0.0.0 " + runtime.GOOS + "/" + runtime.GOARCH
	if err := os.WriteFile(localGo, []byte("#!/bin/sh\necho '"+localVersion+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	version := strings.TrimPrefix(runtime.Version(), "go")
	versionedGo := filepath.Join(binDir, "go"+version)
	if err := os.Symlink(actualGo, versionedGo); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOMODCACHE", t.TempDir())

	got, err := goBinaryForToolchain(context.Background(), version)
	if err != nil {
		t.Fatal(err)
	}
	if got != versionedGo {
		t.Fatalf("resolved Go binary = %q, want versioned PATH binary %q", got, versionedGo)
	}
}

func TestCandidateToolHasNoInstalledTreePaths(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"internal/generated",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("candidate-only generator contains forbidden installed-tree reference %q", forbidden)
		}
	}
}

func TestReplaceEnvironment(t *testing.T) {
	got := replaceEnvironment(
		[]string{"HOME=/tmp/home", "GOFLAGS=-mod=mod", "GOWORK=/tmp/work"},
		map[string]string{"GOFLAGS": "-mod=readonly", "GOWORK": "off"},
	)
	for _, want := range []string{"HOME=/tmp/home", "GOFLAGS=-mod=readonly", "GOWORK=off"} {
		if !slices.Contains(got, want) {
			t.Fatalf("environment %v does not contain %q", got, want)
		}
	}
	if slices.Contains(got, "GOFLAGS=-mod=mod") || slices.Contains(got, "GOWORK=/tmp/work") {
		t.Fatalf("environment retained replaced values: %v", got)
	}
}

func TestWithoutEnvironment(t *testing.T) {
	got := withoutEnvironment(
		[]string{"GOROOT=/wrong", "HOME=/tmp/home", "GOCACHE=/tmp/cache"},
		"GOROOT",
	)
	if slices.Contains(got, "GOROOT=/wrong") {
		t.Fatalf("environment retained removed value: %v", got)
	}
	for _, want := range []string{"HOME=/tmp/home", "GOCACHE=/tmp/cache"} {
		if !slices.Contains(got, want) {
			t.Fatalf("environment %v does not contain %q", got, want)
		}
	}
}
