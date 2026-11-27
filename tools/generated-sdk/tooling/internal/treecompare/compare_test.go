package treecompare

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCompareReportsFilesAndPublicDeclarations(t *testing.T) {
	baseline := t.TempDir()
	proposed := t.TempDir()
	writeTestFile(t, baseline, "client.go", "package client\n\ntype Widget struct { ID string }\nfunc NewWidget() Widget { return Widget{} }\n")
	writeTestFile(t, baseline, "removed.go", "package client\n")
	writeTestFile(t, proposed, "client.go", "package client\n\ntype Widget struct { ID int64 }\nfunc NewWidget() Widget { return Widget{} }\nfunc Added() {}\n")
	writeTestFile(t, proposed, "added.go", "package client\n")
	result, err := Compare(baseline, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Summary.AddedFiles, []string{"added.go"}) ||
		!slices.Equal(result.Summary.RemovedFiles, []string{"removed.go"}) ||
		!slices.Equal(result.Summary.ChangedFiles, []string{"client.go"}) {
		t.Fatalf("unexpected file summary: %#v", result.Summary)
	}
	if !slices.Contains(result.Summary.PublicAdded, "client.go:func Added") ||
		!slices.Contains(result.Summary.PublicChanged, "client.go:type Widget") {
		t.Fatalf("unexpected declaration summary: %#v", result.Summary)
	}
	if !strings.Contains(string(result.Diff), "--- pinned/client.go") {
		t.Fatalf("diff lacks client.go: %s", result.Diff)
	}
}

func writeTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
