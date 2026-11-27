package safetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsSymlinksAndSpecialFiles(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular.go")
	if err := os.WriteFile(regular, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Validate(root); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.go")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "symlinked generated entry") {
		t.Fatalf("Validate symlink error = %v", err)
	}
	if _, err := ReadRegular(link); err == nil {
		t.Fatal("ReadRegular followed a symlink")
	}
}

func TestWriteNewRefusesExistingFilesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture_test.go")
	if err := WriteNew(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteNew(path, []byte("second"), 0o600); err == nil {
		t.Fatal("WriteNew replaced an existing file")
	}
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked_test.go")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteNew(link, []byte("changed"), 0o600); err == nil {
		t.Fatal("WriteNew followed a symlink")
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "kept" {
		t.Fatalf("symlink target changed to %q", content)
	}
}
