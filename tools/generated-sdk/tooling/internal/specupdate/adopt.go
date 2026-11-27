package specupdate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/safetree"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
)

const (
	transactionLockPath = "tools/generated-sdk/.transactions/spec-update.lock"
	transactionPath     = "tools/generated-sdk/.transactions/spec-update.json"
)

type Validator interface {
	Validate(context.Context, string) (CandidateResult, error)
}

type AdoptOptions struct {
	AcceptPlan    string
	AllowBreaking bool
	BundleDir     string
	RepoRoot      string
	Validator     Validator
}

type transaction struct {
	Applied       int                `json:"applied"`
	Entries       []transactionEntry `json:"entries"`
	Phase         string             `json:"phase"`
	PlanDigest    string             `json:"plan_digest"`
	SchemaVersion int                `json:"schema_version"`
}

type transactionEntry struct {
	BackupPath     string      `json:"backup_path"`
	Destination    string      `json:"destination"`
	Mode           fs.FileMode `json:"mode"`
	OriginalSHA256 string      `json:"original_sha256"`
	ProposedSHA256 string      `json:"proposed_sha256"`
	StagePath      string      `json:"stage_path"`
}

func Adopt(ctx context.Context, options AdoptOptions) (resultErr error) {
	if err := requireOutsideRepository(options.RepoRoot, options.BundleDir); err != nil {
		return fmt.Errorf("proposal bundle: %w", err)
	}
	manifest, err := LoadManifest(options.BundleDir)
	if err != nil {
		return err
	}
	if manifest.Status != StatusAdoptable || manifest.PlanDigest != options.AcceptPlan {
		return errors.New("finalized proposal digest was not explicitly accepted")
	}
	if err := verifyArtifacts(options.BundleDir, manifest.Artifacts); err != nil {
		return err
	}
	if err := validateAdoptionClassification(manifest, options.AllowBreaking); err != nil {
		return err
	}
	if options.Validator == nil {
		return errors.New("adoption requires an offline validator")
	}
	if !slices.Equal(sortedKeys(manifest.Artifacts), sortedTrackedPaths()) {
		return errors.New("proposal artifact set differs from the adoption allowlist")
	}
	journalPath := filepath.Join(options.RepoRoot, filepath.FromSlash(transactionPath))
	lockPath := filepath.Join(options.RepoRoot, filepath.FromSlash(transactionLockPath))
	if err := acquireTransactionLock(options.RepoRoot, lockPath); err != nil {
		return err
	}
	releaseLock := true
	defer func() {
		if releaseLock {
			resultErr = errors.Join(resultErr, removeTransactionLock(lockPath))
		}
	}()
	entries, proposedBytes, err := prepareTransactionEntries(options.RepoRoot, options.BundleDir, manifest)
	if err != nil {
		return err
	}
	journal := transaction{
		Applied: 0, Entries: entries, Phase: "preparing",
		PlanDigest: manifest.PlanDigest, SchemaVersion: 1,
	}
	if err := createJournal(journalPath, journal); err != nil {
		if os.IsExist(err) {
			return errors.New("an incomplete generated SDK adoption transaction already exists; run rollback")
		}
		return err
	}
	journalCreated := true
	committed := false
	defer func() {
		if resultErr != nil && journalCreated && !committed {
			resultErr = errors.Join(resultErr, rollbackTransaction(options.RepoRoot, journalPath))
		}
	}()
	for index := range journal.Entries {
		entry := &journal.Entries[index]
		data := proposedBytes[entry.Destination]
		if err := writeExclusiveSynced(filepath.Join(options.RepoRoot, filepath.FromSlash(entry.StagePath)), data, entry.Mode); err != nil {
			return err
		}
	}
	journal.Phase = "staged"
	if err := updateJournal(journalPath, journal); err != nil {
		return err
	}
	for index := range journal.Entries {
		entry := journal.Entries[index]
		journal.Applied = index + 1
		journal.Phase = "applying"
		if err := updateJournal(journalPath, journal); err != nil {
			return err
		}
		destination := filepath.Join(options.RepoRoot, filepath.FromSlash(entry.Destination))
		backup := filepath.Join(options.RepoRoot, filepath.FromSlash(entry.BackupPath))
		if err := os.Rename(destination, backup); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(destination)); err != nil {
			return err
		}
		if filepath.Dir(destination) != filepath.Dir(backup) {
			if err := syncDirectory(filepath.Dir(backup)); err != nil {
				return err
			}
		}
		backupData, err := safetree.ReadRegular(backup)
		if err != nil {
			return err
		}
		if spec.Digest(backupData) != entry.OriginalSHA256 {
			return fmt.Errorf("%s changed after the proposal baseline check", entry.Destination)
		}
		stage := filepath.Join(options.RepoRoot, filepath.FromSlash(entry.StagePath))
		stageData, err := safetree.ReadRegular(stage)
		if err != nil {
			return err
		}
		if spec.Digest(stageData) != entry.ProposedSHA256 {
			return fmt.Errorf("%s staged bytes differ from the finalized proposal", entry.Destination)
		}
		if err := os.Rename(
			stage,
			destination,
		); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(destination)); err != nil {
			return err
		}
	}
	journal.Phase = "validating"
	if err := updateJournal(journalPath, journal); err != nil {
		return err
	}
	validatedCandidate, err := options.Validator.Validate(ctx, options.RepoRoot)
	if err != nil {
		return fmt.Errorf("post-adoption validation: %w", err)
	}
	if !equalCandidateResult(validatedCandidate, manifest.ProposedCandidate) {
		return errors.New("post-adoption candidate differs from the finalized proposal")
	}
	journal.Phase = "committed"
	if err := updateJournal(journalPath, journal); err != nil {
		return err
	}
	committed = true
	if err := cleanupTransaction(options.RepoRoot, journalPath, journal); err != nil {
		return err
	}
	journalCreated = false
	releaseLock = false
	return nil
}

func Rollback(repoRoot string) error {
	if err := prepareTransactionDirectory(repoRoot, false); err != nil {
		return err
	}
	journalPath := filepath.Join(repoRoot, filepath.FromSlash(transactionPath))
	if _, err := os.Lstat(journalPath); err != nil {
		if os.IsNotExist(err) {
			lockPath := filepath.Join(repoRoot, filepath.FromSlash(transactionLockPath))
			if _, lockErr := os.Lstat(lockPath); lockErr == nil {
				for _, suffix := range []string{".initial", ".next"} {
					_ = os.Remove(journalPath + suffix)
				}
				return removeTransactionLock(lockPath)
			}
			return errors.New("no generated SDK adoption transaction exists")
		}
		return err
	}
	return rollbackTransaction(repoRoot, journalPath)
}

func validateAdoptionClassification(manifest Manifest, allowBreaking bool) error {
	for _, comparison := range manifest.Classifications {
		switch comparison.Classification {
		case spec.ClassificationBreaking, spec.ClassificationManualReview:
			if !allowBreaking {
				return fmt.Errorf("%s contains %s changes; use breaking adoption with the reviewed plan digest", comparison.Target, comparison.Classification)
			}
		}
	}
	if !allowBreaking &&
		(len(manifest.Generated.RemovedFiles) != 0 ||
			len(manifest.Generated.PublicChanged) != 0 ||
			len(manifest.Generated.PublicRemoved) != 0) {
		return errors.New("generated Go removals or public declaration changes require breaking adoption")
	}
	return nil
}

func equalCandidateResult(left, right CandidateResult) bool {
	return left.FileCount == right.FileCount &&
		left.FinalSHA256 == right.FinalSHA256 &&
		left.Platform == right.Platform &&
		left.RawSHA256 == right.RawSHA256 &&
		slices.Equal(left.Toolchains, right.Toolchains)
}

func prepareTransactionEntries(
	repoRoot, bundleRoot string,
	manifest Manifest,
) ([]transactionEntry, map[string][]byte, error) {
	paths := sortedTrackedPaths()
	entries := make([]transactionEntry, 0, len(paths))
	proposedBytes := make(map[string][]byte, len(paths))
	for index, path := range paths {
		destination := filepath.Join(repoRoot, filepath.FromSlash(path))
		info, err := os.Lstat(destination)
		if err != nil {
			return nil, nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("%s is not a regular file", path)
		}
		original, err := safetree.ReadRegular(destination)
		if err != nil {
			return nil, nil, err
		}
		if spec.Digest(original) != manifest.BaselineArtifacts[path] {
			return nil, nil, fmt.Errorf("%s no longer matches the proposal baseline", path)
		}
		proposed, err := safetree.ReadRegular(filepath.Join(bundleRoot, filepath.FromSlash(path)))
		if err != nil {
			return nil, nil, err
		}
		if spec.Digest(proposed) != manifest.Artifacts[path] {
			return nil, nil, fmt.Errorf("%s no longer matches the finalized proposal", path)
		}
		proposedBytes[path] = append([]byte(nil), proposed...)
		suffix := fmt.Sprintf(".generated-sdk-%02d-%s", index, manifest.PlanDigest[:12])
		entries = append(entries, transactionEntry{
			BackupPath:     filepath.ToSlash(filepath.Join("tools/generated-sdk/.transactions", fmt.Sprintf("backup-%02d", index))),
			Destination:    path,
			Mode:           info.Mode().Perm(),
			OriginalSHA256: manifest.BaselineArtifacts[path],
			ProposedSHA256: manifest.Artifacts[path],
			StagePath:      path + suffix,
		})
	}
	return entries, proposedBytes, nil
}

func rollbackTransaction(repoRoot, journalPath string) error {
	journal, err := loadTransaction(journalPath)
	if err != nil {
		return err
	}
	if journal.Phase == "committed" {
		return cleanupTransaction(repoRoot, journalPath, journal)
	}
	journal.Phase = "rolling-back"
	if err := updateJournal(journalPath, journal); err != nil {
		return err
	}
	for index := journal.Applied - 1; index >= 0; index-- {
		entry := journal.Entries[index]
		backup := filepath.Join(repoRoot, filepath.FromSlash(entry.BackupPath))
		destination := filepath.Join(repoRoot, filepath.FromSlash(entry.Destination))
		data, readErr := safetree.ReadRegular(backup)
		if readErr == nil {
			if spec.Digest(data) != entry.OriginalSHA256 {
				return fmt.Errorf("%s backup digest is invalid", entry.Destination)
			}
			if err := os.Rename(backup, destination); err != nil {
				return err
			}
			if err := syncDirectory(filepath.Dir(destination)); err != nil {
				return err
			}
			if filepath.Dir(destination) != filepath.Dir(backup) {
				if err := syncDirectory(filepath.Dir(backup)); err != nil {
					return err
				}
			}
		} else if os.IsNotExist(readErr) {
			current, currentErr := safetree.ReadRegular(destination)
			if currentErr != nil || spec.Digest(current) != entry.OriginalSHA256 {
				return fmt.Errorf("%s cannot be proven restored", entry.Destination)
			}
		} else {
			return readErr
		}
		journal.Applied = index
		if err := updateJournal(journalPath, journal); err != nil {
			return err
		}
	}
	return cleanupTransaction(repoRoot, journalPath, journal)
}

func cleanupTransaction(repoRoot, journalPath string, journal transaction) error {
	directories := map[string]bool{filepath.Dir(journalPath): true}
	for _, entry := range journal.Entries {
		for _, relative := range []string{entry.StagePath, entry.BackupPath} {
			path := filepath.Join(repoRoot, filepath.FromSlash(relative))
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			directories[filepath.Dir(path)] = true
		}
	}
	if err := os.Remove(journalPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(journalPath + ".next"); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(journalPath + ".initial"); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(filepath.Join(repoRoot, filepath.FromSlash(transactionLockPath))); err != nil && !os.IsNotExist(err) {
		return err
	}
	for directory := range directories {
		if err := syncDirectory(directory); err != nil {
			return err
		}
	}
	return nil
}

func createJournal(path string, journal transaction) error {
	data, err := spec.Canonical(journal)
	if err != nil {
		return err
	}
	initial := path + ".initial"
	if err := writeExclusiveSynced(initial, data, 0o600); err != nil {
		return err
	}
	if err := os.Link(initial, path); err != nil {
		_ = os.Remove(initial)
		return err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.Remove(initial); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func updateJournal(path string, journal transaction) error {
	data, err := spec.Canonical(journal)
	if err != nil {
		return err
	}
	temporary := path + ".next"
	if err := writeExclusiveSynced(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func loadTransaction(path string) (transaction, error) {
	data, err := safetree.ReadRegular(path)
	if err != nil {
		return transaction{}, err
	}
	var journal transaction
	if err := spec.DecodeStrict(data, &journal); err != nil {
		return transaction{}, err
	}
	canonical, err := spec.Canonical(journal)
	if err != nil {
		return transaction{}, err
	}
	if string(data) != string(canonical) {
		return transaction{}, errors.New("generated SDK adoption journal is not canonical JSON")
	}
	if journal.SchemaVersion != 1 || journal.Applied < 0 || journal.Applied > len(journal.Entries) {
		return transaction{}, errors.New("invalid generated SDK adoption journal")
	}
	if err := validateTransaction(journal); err != nil {
		return transaction{}, err
	}
	return journal, nil
}

func validateTransaction(journal transaction) error {
	switch journal.Phase {
	case "preparing", "staged", "applying", "validating", "committed", "rolling-back":
	default:
		return errors.New("generated SDK adoption journal has an invalid phase")
	}
	if !sha256Pattern.MatchString(journal.PlanDigest) || len(journal.Entries) != len(trackedPaths) {
		return errors.New("generated SDK adoption journal has an invalid identity")
	}
	destinations := make([]string, 0, len(journal.Entries))
	for index, entry := range journal.Entries {
		expectedSuffix := fmt.Sprintf(".generated-sdk-%02d-%s", index, journal.PlanDigest[:12])
		expectedBackup := filepath.ToSlash(filepath.Join("tools/generated-sdk/.transactions", fmt.Sprintf("backup-%02d", index)))
		if entry.Destination != sortedTrackedPaths()[index] ||
			entry.StagePath != entry.Destination+expectedSuffix ||
			entry.BackupPath != expectedBackup ||
			!sha256Pattern.MatchString(entry.OriginalSHA256) ||
			!sha256Pattern.MatchString(entry.ProposedSHA256) ||
			entry.Mode.Perm() != entry.Mode {
			return errors.New("generated SDK adoption journal entry differs from policy")
		}
		destinations = append(destinations, entry.Destination)
	}
	if !slices.Equal(destinations, sortedTrackedPaths()) {
		return errors.New("generated SDK adoption journal paths differ from policy")
	}
	return nil
}

func acquireTransactionLock(repoRoot, path string) error {
	transactionDirectory, err := transactionStateDirectory(repoRoot)
	if err != nil {
		return err
	}
	lockDirectory, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return err
	}
	if filepath.Clean(lockDirectory) != transactionDirectory {
		return errors.New("generated SDK transaction lock path differs from policy")
	}
	if err := prepareTransactionDirectory(repoRoot, true); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(transactionDirectory)); err != nil {
		return err
	}
	if err := writeExclusiveSynced(path, []byte("generated-sdk-spec-update\n"), 0o600); err != nil {
		if os.IsExist(err) {
			return errors.New("a generated SDK adoption transaction is already active; run rollback if it was interrupted")
		}
		return err
	}
	return nil
}

func prepareTransactionDirectory(repoRoot string, create bool) error {
	transactionDirectory, err := transactionStateDirectory(repoRoot)
	if err != nil {
		return err
	}
	current := filepath.Clean(repoRoot)
	for _, component := range []string{"tools", "generated-sdk"} {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return statErr
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is not a contained directory", current)
		}
	}
	info, statErr := os.Lstat(transactionDirectory)
	switch {
	case statErr == nil:
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("generated SDK transaction state is not a contained directory")
		}
		return nil
	case !os.IsNotExist(statErr):
		return statErr
	case !create:
		return nil
	}
	if err := os.Mkdir(transactionDirectory, 0o700); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(transactionDirectory))
}

func transactionStateDirectory(repoRoot string) (string, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "tools", "generated-sdk", ".transactions"), nil
}

func removeTransactionLock(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func writeExclusiveSynced(path string, data []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	remove = false
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = directory.Close()
	}()
	return directory.Sync()
}

func sortedTrackedPaths() []string {
	result := append([]string(nil), trackedPaths...)
	slices.Sort(result)
	return result
}

func sortedKeys(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}
