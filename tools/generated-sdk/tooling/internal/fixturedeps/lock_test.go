package fixturedeps

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedFixtureDependencyLock(t *testing.T) {
	root := repoRoot(t)
	if err := VerifyLocks(root); err != nil {
		t.Fatal(err)
	}
	generated := writeGeneratedLocks(t, root)
	if err := VerifyGenerated(root, generated); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedFixtureDependencyDriftFails(t *testing.T) {
	root := repoRoot(t)
	tests := []struct {
		name   string
		file   string
		mutate string
	}{
		{
			name:   "direct-requirement",
			file:   "go.mod",
			mutate: "\nrequire example.invalid/drift v1.0.0\n",
		},
		{
			name:   "checksum",
			file:   "go.sum",
			mutate: "example.invalid/drift v1.0.0 h1:drift\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generated := writeGeneratedLocks(t, root)
			path := filepath.Join(generated, test.file)
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString(test.mutate); err != nil {
				t.Fatal(errors.Join(err, file.Close()))
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if err := VerifyGenerated(root, generated); err == nil {
				t.Fatal("mutated generated dependency lock unexpectedly passed")
			}
		})
	}
}

func TestActiveFixtureFilesFail(t *testing.T) {
	for _, name := range []string{"go.mod", "go.sum", "fixture_test.go"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, name), []byte("active\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := verifyInactiveFixtureDir(dir); err == nil {
				t.Fatalf("active fixture file %s unexpectedly passed", name)
			}
		})
	}
}

func writeGeneratedLocks(t *testing.T, repoRoot string) string {
	t.Helper()
	dir := t.TempDir()
	for source, target := range map[string]string{
		ModLockPath: "go.mod",
		SumLockPath: "go.sum",
	} {
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(source)))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, target), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func repoRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(current, "..", "..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("locate repository root: %v", err)
	}
	return root
}
