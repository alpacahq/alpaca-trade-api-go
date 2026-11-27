package templatepolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyRepositoryTrackedTemplatePolicy(t *testing.T) {
	if err := VerifyTracked(repoRoot(t)); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRepositoryTemplatePolicy(t *testing.T) {
	root := repoRoot(t)
	requireBootstrappedJAR(t, root)
	if err := Verify(root); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyTrackedMutationsFail(t *testing.T) {
	root := repoRoot(t)
	stock, err := loadCanonical[StockManifest](root, StockManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	deltas, err := loadCanonical[DeltaManifest](root, DeltaManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("accepted-hash-drift", func(t *testing.T) {
		mutated := *stock
		mutated.Entries = append([]TemplateDigest(nil), stock.Entries...)
		mutated.Entries[0].CustomSHA256 = "bad"
		if err := validateManifests(&mutated, deltas); err == nil {
			t.Fatal("mutated accepted custom hash unexpectedly passed")
		}
	})
	t.Run("custom-template-drift", func(t *testing.T) {
		tempRoot := t.TempDir()
		copyTrackedTemplateInputs(t, root, tempRoot)
		if err := VerifyTracked(tempRoot); err != nil {
			t.Fatalf("clean tracked template copy failed: %v", err)
		}
		target := filepath.Join(tempRoot, filepath.FromSlash(CustomTemplateDir), "model_enum.mustache")
		content, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, append(content, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := VerifyTracked(tempRoot); err == nil {
			t.Fatal("drifted custom template unexpectedly passed VerifyTracked")
		}
	})
}

func TestVerifyDeltaMutationsFail(t *testing.T) {
	stock := []byte("line1\nold\nline3\n")
	custom := []byte("line1\nnew1\nnew2\nline3\n")
	baseDelta := TemplateDelta{
		Name: "synthetic.mustache",
		LineStats: LineStats{
			Deletions:  1,
			Insertions: 2,
		},
		Hunks: []Hunk{
			{
				StockStart:  2,
				CustomStart: 2,
				StockLines:  []string{"old"},
				CustomLines: []string{"new1", "new2"},
				Semantic:    "synthetic test delta",
			},
		},
	}
	if err := verifyDelta("synthetic.mustache", stock, custom, baseDelta); err != nil {
		t.Fatalf("valid synthetic delta failed: %v", err)
	}
	t.Run("stock-drift", func(t *testing.T) {
		if err := verifyDelta("synthetic.mustache", []byte("line1\nchanged\nline3\n"), custom, baseDelta); err == nil {
			t.Fatal("stock drift unexpectedly passed")
		}
	})
	t.Run("outside-reviewed-region", func(t *testing.T) {
		if err := verifyDelta("synthetic.mustache", stock, []byte("changed\nnew1\nnew2\nline3\n"), baseDelta); err == nil {
			t.Fatal("change outside reviewed hunks unexpectedly passed")
		}
	})
	t.Run("line-stat-drift", func(t *testing.T) {
		mutated := baseDelta
		mutated.LineStats.Insertions++
		if err := verifyDelta("synthetic.mustache", stock, custom, mutated); err == nil {
			t.Fatal("mutated line stats unexpectedly passed")
		}
	})
}

func TestReadCustomFilesRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(CustomTemplateDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "template.mustache")
	if err := os.WriteFile(target, []byte("template"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "model_enum.mustache")); err != nil {
		t.Fatal(err)
	}
	if _, err := readCustomFiles(root); err == nil {
		t.Fatal("symlinked custom template unexpectedly passed")
	}
}

func TestTemplatePolicyMutationsFail(t *testing.T) {
	root := repoRoot(t)
	requireBootstrappedJAR(t, root)
	load := func(t *testing.T) (*StockManifest, *DeltaManifest, map[string][]byte, map[string][]byte) {
		t.Helper()
		stock, err := loadCanonical[StockManifest](root, StockManifestPath)
		if err != nil {
			t.Fatal(err)
		}
		deltas, err := loadCanonical[DeltaManifest](root, DeltaManifestPath)
		if err != nil {
			t.Fatal(err)
		}
		stockFiles, err := readStockFiles(root, stock)
		if err != nil {
			t.Fatal(err)
		}
		customFiles, err := readCustomFiles(root)
		if err != nil {
			t.Fatal(err)
		}
		return stock, deltas, stockFiles, customFiles
	}
	t.Run("custom-drift", func(t *testing.T) {
		stock, deltas, stockFiles, customFiles := load(t)
		customFiles["model_enum.mustache"] = append(customFiles["model_enum.mustache"], '\n')
		if err := verifyFiles(stock, deltas, stockFiles, customFiles); err == nil {
			t.Fatal("custom template drift unexpectedly passed")
		}
	})
	t.Run("stock-drift", func(t *testing.T) {
		stock, deltas, stockFiles, customFiles := load(t)
		stockFiles["model_enum.mustache"] = append(stockFiles["model_enum.mustache"], '\n')
		if err := verifyFiles(stock, deltas, stockFiles, customFiles); err == nil {
			t.Fatal("stock template drift unexpectedly passed")
		}
	})
	t.Run("extra-template", func(t *testing.T) {
		stock, deltas, stockFiles, customFiles := load(t)
		customFiles["future.mustache"] = []byte("future\n")
		if err := verifyFiles(stock, deltas, stockFiles, customFiles); err == nil {
			t.Fatal("extra template unexpectedly passed")
		}
	})
	t.Run("line-stat-drift", func(t *testing.T) {
		stock, deltas, stockFiles, customFiles := load(t)
		deltas.Entries[0].LineStats.Insertions++
		if err := verifyFiles(stock, deltas, stockFiles, customFiles); err == nil {
			t.Fatal("mutated line stats unexpectedly passed")
		}
	})
	t.Run("outside-reviewed-region", func(t *testing.T) {
		_, deltas, stockFiles, customFiles := load(t)
		var delta TemplateDelta
		for _, candidate := range deltas.Entries {
			if candidate.Name == "model_enum.mustache" {
				delta = candidate
			}
		}
		custom := append([]byte(nil), customFiles["model_enum.mustache"]...)
		custom[0] = '#'
		if err := verifyDelta("model_enum.mustache", stockFiles["model_enum.mustache"], custom, delta); err == nil {
			t.Fatal("change outside reviewed hunks unexpectedly passed")
		}
	})
	t.Run("accepted-hash-drift", func(t *testing.T) {
		stock, deltas, _, _ := load(t)
		stock.Entries[0].CustomSHA256 = "bad"
		if err := validateManifests(stock, deltas); err == nil {
			t.Fatal("mutated accepted custom hash unexpectedly passed")
		}
	})
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

func requireBootstrappedJAR(t *testing.T, root string) {
	t.Helper()
	jarPath := filepath.Join(root, "tools/generated-sdk/.cache/openapi-generator-cli-7.27.0.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		t.Skip("generator JAR cache not bootstrapped; run make generated-sdk-generator-bootstrap for stock-template diff checks")
	} else if err != nil {
		t.Fatal(err)
	}
}

func copyTrackedTemplateInputs(t *testing.T, srcRoot, dstRoot string) {
	t.Helper()
	files := []string{
		StockManifestPath,
		DeltaManifestPath,
	}
	for name := range acceptedCustomDigests {
		files = append(files, CustomTemplateDir+"/"+name)
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(srcRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
