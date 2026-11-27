package specupdate

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
)

type testRoundTrip func(*http.Request) (*http.Response, error)

func (function testRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, bundleRoot, outputDir string) (CandidateResult, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return CandidateResult{}, err
	}
	field := "string"
	if bundleRoot != "" {
		field = "int64"
	}
	source := "package generated\n\ntype Widget struct { ID " + field + " }\n"
	if err := os.WriteFile(filepath.Join(outputDir, "widget.go"), []byte(source), 0o444); err != nil {
		return CandidateResult{}, err
	}
	digest := spec.Digest([]byte(field))
	return CandidateResult{FileCount: 1, FinalSHA256: digest, Platform: "linux/amd64", RawSHA256: digest, Toolchains: []string{"test"}}, nil
}

func TestAcquireAndFinalizeAreExternalAndDigestBound(t *testing.T) {
	repoRoot := testRepoRoot(t)
	specifications := map[string][]byte{}
	for target, definition := range targetDefinitions {
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(definition.Path)))
		if err != nil {
			t.Fatal(err)
		}
		specifications[target] = data
	}
	client := &http.Client{Transport: testRoundTrip(func(request *http.Request) (*http.Response, error) {
		target := "trading"
		if strings.Contains(request.URL.Path, "market-data") {
			target = "market-data"
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(string(specifications[target]))),
			Header:     make(http.Header),
		}, nil
	})}
	parent := t.TempDir()
	acquisitionDir := filepath.Join(parent, "acquisition")
	acquisition, err := Acquire(context.Background(), AcquireOptions{
		Client: client, OutputDir: acquisitionDir, RepoRoot: repoRoot,
		RetrievalDate: time.Date(2026, time.November, 26, 22, 30, 0, 0, time.FixedZone("test", 3600)),
		Runner:        fakeRunner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if acquisition.Status != StatusReviewRequired || acquisition.RetrievalDate != "2026-11-26" {
		t.Fatalf("unexpected acquisition: %#v", acquisition)
	}
	if len(acquisition.Generated.PublicChanged) != 1 {
		t.Fatalf("generated comparison = %#v", acquisition.Generated)
	}
	loaded, err := LoadManifest(acquisitionDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PlanDigest != acquisition.PlanDigest {
		t.Fatalf("loaded digest = %q, want %q", loaded.PlanDigest, acquisition.PlanDigest)
	}
	reviewedDir := filepath.Join(parent, "reviewed")
	if err := os.Mkdir(reviewedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join(acquisitionDir, "tools/generated-sdk/specs/feature-allowlist.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reviewedDir, "feature-allowlist.json"), policy, 0o444); err != nil {
		t.Fatal(err)
	}
	finalDir := filepath.Join(parent, "final")
	finalized, err := Finalize(context.Background(), FinalizeOptions{
		AcquisitionDir: acquisitionDir,
		AcceptPlan:     acquisition.PlanDigest,
		OutputDir:      finalDir,
		RepoRoot:       repoRoot,
		ReviewedDir:    reviewedDir,
		Runner:         fakeRunner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if finalized.Status != StatusAdoptable || finalized.PlanDigest == acquisition.PlanDigest {
		t.Fatalf("unexpected finalized proposal: %#v", finalized)
	}
	changedRepo := copyTrackedInputs(t, repoRoot)
	changedPath := filepath.Join(
		changedRepo,
		filepath.FromSlash("tools/generated-sdk/specs/trading.openapi.json"),
	)
	if err := os.Chmod(changedPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changedPath, []byte("{}\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := Finalize(context.Background(), FinalizeOptions{
		AcquisitionDir: acquisitionDir,
		AcceptPlan:     acquisition.PlanDigest,
		OutputDir:      filepath.Join(parent, "changed-baseline"),
		RepoRoot:       changedRepo,
		ReviewedDir:    reviewedDir,
		Runner:         fakeRunner{},
	}); err == nil {
		t.Fatal("finalization against a changed acquisition baseline unexpectedly passed")
	}
	if _, err := Finalize(context.Background(), FinalizeOptions{
		AcquisitionDir: acquisitionDir,
		AcceptPlan:     strings.Repeat("0", 64),
		OutputDir:      filepath.Join(parent, "rejected"),
		RepoRoot:       repoRoot,
		ReviewedDir:    reviewedDir,
		Runner:         fakeRunner{},
	}); err == nil {
		t.Fatal("finalization with the wrong plan digest unexpectedly passed")
	}
}

func TestManifestTamperingFails(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{Artifacts: map[string]string{}, BaselineArtifacts: map[string]string{}, Classifications: []spec.Comparison{}, SchemaVersion: 1, Status: StatusReviewRequired}
	if err := sealManifest(root, &manifest); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ManifestPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte(" "), data...)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(root); err == nil {
		t.Fatal("tampered manifest unexpectedly passed")
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func copyTrackedInputs(t *testing.T, sourceRoot string) string {
	t.Helper()
	destinationRoot := t.TempDir()
	for _, path := range trackedPaths {
		data, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(destinationRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	return destinationRoot
}
