package fixturedeps

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ModLockPath = "tools/generated-sdk/generator/fixtures/template-fixture.mod"
	SumLockPath = "tools/generated-sdk/generator/fixtures/template-fixture.sum"
)

func VerifyLocks(repoRoot string) error {
	fixtureDir := filepath.Join(repoRoot, "tools", "generated-sdk", "generator", "fixtures")
	if err := verifyInactiveFixtureDir(fixtureDir); err != nil {
		return err
	}
	modLock, sumLock, err := readLocks(repoRoot)
	if err != nil {
		return err
	}
	if len(modLock) == 0 || len(sumLock) == 0 {
		return errors.New("fixture dependency locks must not be empty")
	}
	return nil
}

func verifyInactiveFixtureDir(fixtureDir string) error {
	for _, activeName := range []string{"go.mod", "go.sum"} {
		if _, err := os.Stat(filepath.Join(fixtureDir, activeName)); err == nil {
			return fmt.Errorf("active nested fixture module is forbidden: %s", activeName)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return fmt.Errorf("active fixture Go source is forbidden: %s", entry.Name())
		}
	}
	return nil
}

func VerifyGenerated(repoRoot, generatedDir string) error {
	if err := VerifyLocks(repoRoot); err != nil {
		return err
	}
	modLock, sumLock, err := readLocks(repoRoot)
	if err != nil {
		return err
	}
	generatedMod, err := os.ReadFile(filepath.Join(generatedDir, "go.mod"))
	if err != nil {
		return fmt.Errorf("read generated fixture go.mod: %w", err)
	}
	if !bytes.Equal(generatedMod, modLock) {
		return errors.New("generated fixture go.mod dependency graph differs from reviewed lock")
	}
	generatedSum, err := os.ReadFile(filepath.Join(generatedDir, "go.sum"))
	if err != nil {
		return fmt.Errorf("read generated fixture go.sum: %w", err)
	}
	if !bytes.Equal(generatedSum, sumLock) {
		return errors.New("generated fixture go.sum differs from reviewed lock")
	}
	return nil
}

func readLocks(repoRoot string) ([]byte, []byte, error) {
	modLock, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(ModLockPath)))
	if err != nil {
		return nil, nil, err
	}
	sumLock, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(SumLockPath)))
	if err != nil {
		return nil, nil, err
	}
	return modLock, sumLock, nil
}
