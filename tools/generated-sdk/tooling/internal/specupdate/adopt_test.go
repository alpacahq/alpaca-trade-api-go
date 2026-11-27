package specupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/treecompare"
)

type validatorFunc func(context.Context, string) (CandidateResult, error)

func (function validatorFunc) Validate(ctx context.Context, root string) (CandidateResult, error) {
	return function(ctx, root)
}

func TestAdoptUpdatesAllowlistedFiles(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: successfulValidator(manifest),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range trackedPaths {
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "proposed:"+path+"\n" {
			t.Fatalf("%s = %q", path, data)
		}
	}
	if _, err := os.Lstat(filepath.Join(repoRoot, filepath.FromSlash(transactionPath))); !os.IsNotExist(err) {
		t.Fatalf("transaction journal remains: %v", err)
	}
}

func TestAdoptRollsBackValidationFailure(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: validatorFunc(func(context.Context, string) (CandidateResult, error) {
			return CandidateResult{}, errors.New("validation failed")
		}),
	})
	if err == nil {
		t.Fatal("failed validation unexpectedly passed")
	}
	for _, path := range trackedPaths {
		data, readErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(data) != "original:"+path+"\n" {
			t.Fatalf("%s was not rolled back: %q", path, data)
		}
	}
}

func TestAdoptRollsBackCandidateMismatch(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: validatorFunc(func(context.Context, string) (CandidateResult, error) {
			mismatch := manifest.ProposedCandidate
			mismatch.FinalSHA256 = spec.Digest([]byte("different"))
			return mismatch, nil
		}),
	})
	if err == nil {
		t.Fatal("candidate mismatch unexpectedly passed")
	}
	for _, path := range trackedPaths {
		data, readErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(data) != "original:"+path+"\n" {
			t.Fatalf("%s was not rolled back: %q", path, data)
		}
	}
}

func TestAdditiveAdoptionRejectsBreakingPlan(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationBreaking)
	err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: successfulValidator(manifest),
	})
	if err == nil {
		t.Fatal("breaking plan unexpectedly passed additive adoption")
	}
	if err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, AllowBreaking: true, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: successfulValidator(manifest),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAdoptRejectsStaleBaseline(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	path := filepath.Join(repoRoot, filepath.FromSlash(trackedPaths[0]))
	if err := os.WriteFile(path, []byte("locally changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: successfulValidator(manifest),
	})
	if err == nil {
		t.Fatal("stale baseline unexpectedly passed")
	}
}

func TestAdditiveAdoptionRejectsGeneratedPublicChange(t *testing.T) {
	additive := Manifest{Generated: treecompare.Summary{
		PublicAdded:    []string{"model_order.go:method *Order.GetLabel"},
		PublicAdditive: []string{"model_order.go:type Order", "model_status.go:var AllowedStatusEnumValues"},
	}}
	if err := validateAdoptionClassification(additive, false); err != nil {
		t.Fatalf("additive generated declaration evolution unexpectedly failed additive adoption: %v", err)
	}
	manifest := Manifest{Generated: treecompare.Summary{PublicChanged: []string{"client.go:type Client"}}}
	if err := validateAdoptionClassification(manifest, false); err == nil {
		t.Fatal("generated public change unexpectedly passed additive adoption")
	}
	if err := validateAdoptionClassification(manifest, true); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackResumesAfterDestinationWasBackedUp(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	lockPath := filepath.Join(repoRoot, filepath.FromSlash(transactionLockPath))
	if err := acquireTransactionLock(repoRoot, lockPath); err != nil {
		t.Fatal(err)
	}
	entries, _, err := prepareTransactionEntries(repoRoot, bundleRoot, manifest)
	if err != nil {
		t.Fatal(err)
	}
	journal := transaction{
		Applied: 1, Entries: entries, Phase: "applying",
		PlanDigest: manifest.PlanDigest, SchemaVersion: 1,
	}
	journalPath := filepath.Join(repoRoot, filepath.FromSlash(transactionPath))
	if err := createJournal(journalPath, journal); err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	if err := os.Rename(
		filepath.Join(repoRoot, filepath.FromSlash(entry.Destination)),
		filepath.Join(repoRoot, filepath.FromSlash(entry.BackupPath)),
	); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(repoRoot); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(entry.Destination)))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original:"+entry.Destination+"\n" {
		t.Fatalf("restored data = %q", data)
	}
}

func TestRollbackRecoversAfterInterruptedJournalUpdate(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	lockPath := filepath.Join(repoRoot, filepath.FromSlash(transactionLockPath))
	if err := acquireTransactionLock(repoRoot, lockPath); err != nil {
		t.Fatal(err)
	}
	entries, _, err := prepareTransactionEntries(repoRoot, bundleRoot, manifest)
	if err != nil {
		t.Fatal(err)
	}
	journal := transaction{
		Applied: 1, Entries: entries, Phase: "applying",
		PlanDigest: manifest.PlanDigest, SchemaVersion: 1,
	}
	journalPath := filepath.Join(repoRoot, filepath.FromSlash(transactionPath))
	if err := createJournal(journalPath, journal); err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	if err := os.Rename(
		filepath.Join(repoRoot, filepath.FromSlash(entry.Destination)),
		filepath.Join(repoRoot, filepath.FromSlash(entry.BackupPath)),
	); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash after writing spec-update.json.next (and a lingering .initial) before os.Rename.
	if err := os.WriteFile(journalPath+".next", []byte("partial-journal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath+".initial", []byte("initial-journal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Rollback(repoRoot); err != nil {
		t.Fatalf("rollback failed with leftover .next file: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(entry.Destination)))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original:"+entry.Destination+"\n" {
		t.Fatalf("restored data = %q", data)
	}
	for _, path := range []string{journalPath, journalPath + ".next", journalPath + ".initial", lockPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("transaction artifact %s remained after rollback: %v", path, err)
		}
	}
}

func TestAdoptRetainsLockWhenRollbackFails(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: validatorFunc(func(_ context.Context, root string) (CandidateResult, error) {
			backupPath := filepath.Join(root, "tools", "generated-sdk", ".transactions", "backup-00")
			if writeErr := os.WriteFile(backupPath, []byte("corrupted backup\n"), 0o644); writeErr != nil {
				return CandidateResult{}, writeErr
			}
			return CandidateResult{}, errors.New("validation failed")
		}),
	})
	if err == nil {
		t.Fatal("failed validation and rollback unexpectedly passed")
	}
	lockPath := filepath.Join(repoRoot, filepath.FromSlash(transactionLockPath))
	if _, statErr := os.Lstat(lockPath); statErr != nil {
		t.Fatalf("transaction lock was released despite incomplete rollback: %v", statErr)
	}
	journalPath := filepath.Join(repoRoot, filepath.FromSlash(transactionPath))
	if _, statErr := os.Lstat(journalPath); statErr != nil {
		t.Fatalf("transaction journal disappeared despite failed rollback: %v", statErr)
	}
}

func TestAdoptAndRollbackRejectSymlinkedTransactionState(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	external := t.TempDir()
	transactionDirectory := filepath.Join(repoRoot, "tools", "generated-sdk", ".transactions")
	if err := os.Symlink(external, transactionDirectory); err != nil {
		t.Fatal(err)
	}
	err := Adopt(context.Background(), AdoptOptions{
		AcceptPlan: manifest.PlanDigest, BundleDir: bundleRoot, RepoRoot: repoRoot,
		Validator: validatorFunc(func(context.Context, string) (CandidateResult, error) {
			return manifest.ProposedCandidate, nil
		}),
	})
	if err == nil {
		t.Fatal("adoption with symlinked transaction state unexpectedly passed")
	}
	if err := Rollback(repoRoot); err == nil {
		t.Fatal("rollback with symlinked transaction state unexpectedly passed")
	}
	entries, readErr := os.ReadDir(external)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("transaction state escaped the repository: %#v", entries)
	}
	for _, path := range trackedPaths {
		data, readErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(data) != "original:"+path+"\n" {
			t.Fatalf("%s changed despite rejected transaction directory: %q", path, data)
		}
	}
}

func TestLoadTransactionRejectsPathEscape(t *testing.T) {
	repoRoot, bundleRoot, manifest := adoptionFixture(t, spec.ClassificationAdditive)
	entries, _, err := prepareTransactionEntries(repoRoot, bundleRoot, manifest)
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Destination = "../outside"
	journal := transaction{
		Applied: 0, Entries: entries, Phase: "preparing",
		PlanDigest: manifest.PlanDigest, SchemaVersion: 1,
	}
	data, err := spec.Canonical(journal)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "journal.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTransaction(path); err == nil {
		t.Fatal("path-escaping journal unexpectedly passed")
	}
}

func successfulValidator(manifest Manifest) Validator {
	return validatorFunc(func(context.Context, string) (CandidateResult, error) {
		return manifest.ProposedCandidate, nil
	})
}

func adoptionFixture(t *testing.T, classification spec.Classification) (string, string, Manifest) {
	t.Helper()
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "repo")
	bundleRoot := filepath.Join(parent, "bundle")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bundleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	baseline := map[string]string{}
	proposed := map[string]string{}
	for _, path := range trackedPaths {
		original := []byte("original:" + path + "\n")
		candidate := []byte("proposed:" + path + "\n")
		writeFixtureFile(t, repoRoot, path, original)
		writeFixtureFile(t, bundleRoot, path, candidate)
		baseline[path] = spec.Digest(original)
		proposed[path] = spec.Digest(candidate)
	}
	manifest := Manifest{
		Artifacts: proposed, BaselineArtifacts: baseline,
		Classifications: []spec.Comparison{
			{
				Target: "market-data", Classification: classification,
				Changes: []spec.Change{{Classification: classification, Kind: "fixture", Subject: "market-data", Detail: "fixture"}},
			},
			{
				Target: "trading", Classification: classification,
				Changes: []spec.Change{{Classification: classification, Kind: "fixture", Subject: "trading", Detail: "fixture"}},
			},
		},
		Generated: treecompare.Summary{
			AddedFiles: []string{}, ChangedFiles: []string{}, PublicAdded: []string{},
			PublicAdditive: []string{}, PublicChanged: []string{}, PublicRemoved: []string{}, RemovedFiles: []string{},
			BaselineTreeSHA256: spec.Digest([]byte("baseline")),
			ProposedTreeSHA256: spec.Digest([]byte("proposed")),
		},
		PinnedCandidate: CandidateResult{
			FileCount: 1, FinalSHA256: spec.Digest([]byte("pinned-final")),
			Platform: "linux/amd64", RawSHA256: spec.Digest([]byte("pinned-raw")), Toolchains: []string{"test"},
		},
		ProposedCandidate: CandidateResult{
			FileCount: 1, FinalSHA256: spec.Digest([]byte("proposed-final")),
			Platform: "linux/amd64", RawSHA256: spec.Digest([]byte("proposed-raw")), Toolchains: []string{"test"},
		},
		ReviewArtifacts: map[string]string{GeneratedDiffPath: spec.Digest(nil)},
		RetrievalDate:   "2026-11-26",
		SchemaVersion:   1,
		Status:          StatusAdoptable,
	}
	if err := os.WriteFile(filepath.Join(bundleRoot, GeneratedDiffPath), nil, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := sealManifest(bundleRoot, &manifest); err != nil {
		t.Fatal(err)
	}
	return repoRoot, bundleRoot, manifest
}

func writeFixtureFile(t *testing.T, root, relative string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
