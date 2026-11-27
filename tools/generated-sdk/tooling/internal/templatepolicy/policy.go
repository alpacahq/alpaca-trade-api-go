package templatepolicy

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
)

const (
	CustomTemplateDir = "tools/generated-sdk/generator/templates/go"
	DeltaManifestPath = "tools/generated-sdk/generator/template-deltas.json"
	StockManifestPath = "tools/generated-sdk/generator/stock-template-digests.json"
)

var acceptedCustomDigests = map[string]string{
	"model_anyof.mustache":  "9e3c6e71d63ac079227786a43c7e73273f6967c7fbcc2771e6145df59da81e99",
	"model_enum.mustache":   "153e3a586982664544edb2849c8c7fd517a7cea5a635825f5219f755bdb2b2d3",
	"model_oneof.mustache":  "667441f070b77e2bd07100f16942bad440a8afb949c242b207f2f1e1d2848041",
	"model_simple.mustache": "e0ea8a1b093106f9bb8a2b283779523e38bc15674a20136f3c80e7eb227de70d",
}

var acceptedStockDigests = map[string]string{
	"model_anyof.mustache":  "29694d9c68575cbba5c8fd9af3de3633437a31836a4e17588a143951c8c15741",
	"model_enum.mustache":   "c7a5069f3c2896887abf53add1d1a83717a96dec4d25f79d8154c4227aeffda7",
	"model_oneof.mustache":  "98148bb5ed88123a26e1f0ec0c1f2b18744934732407858cd2359702b2c0ec5c",
	"model_simple.mustache": "80293db777ee6940cfc7f489e73d0c540990263a4e6d75a41aad5a711d7ff861",
}

type StockManifest struct {
	Entries          []TemplateDigest `json:"entries"`
	GeneratorVersion string           `json:"generator_version"`
	JARSHA256        string           `json:"jar_sha256"`
	SchemaVersion    int              `json:"schema_version"`
}

type TemplateDigest struct {
	CustomPath   string `json:"custom_path"`
	CustomSHA256 string `json:"custom_sha256"`
	Name         string `json:"name"`
	StockPath    string `json:"stock_path"`
	StockSHA256  string `json:"stock_sha256"`
}

type DeltaManifest struct {
	Entries       []TemplateDelta `json:"entries"`
	SchemaVersion int             `json:"schema_version"`
}

type TemplateDelta struct {
	CustomSHA256 string    `json:"custom_sha256"`
	Hunks        []Hunk    `json:"hunks"`
	LineStats    LineStats `json:"line_stats"`
	Name         string    `json:"name"`
	Policy       string    `json:"policy"`
	StockSHA256  string    `json:"stock_sha256"`
}

type Hunk struct {
	CustomLines []string `json:"custom_lines"`
	CustomStart int      `json:"custom_start"`
	Semantic    string   `json:"semantic"`
	StockLines  []string `json:"stock_lines"`
	StockStart  int      `json:"stock_start"`
}

type LineStats struct {
	Deletions  int `json:"deletions"`
	Insertions int `json:"insertions"`
}

func VerifyTracked(repoRoot string) error {
	_, _, _, err := loadAndVerifyTracked(repoRoot)
	return err
}

func loadAndVerifyTracked(repoRoot string) (*StockManifest, *DeltaManifest, map[string][]byte, error) {
	stock, err := loadCanonical[StockManifest](repoRoot, StockManifestPath)
	if err != nil {
		return nil, nil, nil, err
	}
	deltas, err := loadCanonical[DeltaManifest](repoRoot, DeltaManifestPath)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validateManifests(stock, deltas); err != nil {
		return nil, nil, nil, err
	}
	customFiles, err := readCustomFiles(repoRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, entry := range stock.Entries {
		customContent, ok := customFiles[entry.Name]
		if !ok {
			return nil, nil, nil, fmt.Errorf("custom template %s is missing", entry.Name)
		}
		if digest := spec.Digest(customContent); digest != acceptedCustomDigests[entry.Name] {
			return nil, nil, nil, fmt.Errorf("custom template %s SHA-256 is %s, want %s", entry.Name, digest, acceptedCustomDigests[entry.Name])
		}
	}
	return stock, deltas, customFiles, nil
}

func Verify(repoRoot string) error {
	stock, deltas, customFiles, err := loadAndVerifyTracked(repoRoot)
	if err != nil {
		return err
	}
	stockFiles, err := readStockFiles(repoRoot, stock)
	if err != nil {
		return err
	}
	return verifyFiles(stock, deltas, stockFiles, customFiles)
}

func LoadStockTemplate(repoRoot, name string) ([]byte, error) {
	stock, err := loadCanonical[StockManifest](repoRoot, StockManifestPath)
	if err != nil {
		return nil, err
	}
	files, err := readStockFiles(repoRoot, stock)
	if err != nil {
		return nil, err
	}
	content, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("unknown stock template %q", name)
	}
	return append([]byte(nil), content...), nil
}

func loadCanonical[T any](repoRoot, path string) (*T, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
	if err != nil {
		return nil, err
	}
	var value T
	if err := spec.DecodeStrict(data, &value); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	canonical, err := spec.Canonical(value)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		index := 0
		for index < len(data) && index < len(canonical) && data[index] == canonical[index] {
			index++
		}
		return nil, fmt.Errorf("%s is not canonical JSON at byte %d", path, index)
	}
	return &value, nil
}

func validateManifests(stock *StockManifest, deltas *DeltaManifest) error {
	if stock.SchemaVersion != 1 || deltas.SchemaVersion != 1 {
		return errors.New("template manifest schema version must be 1")
	}
	if stock.GeneratorVersion != "7.27.0" ||
		stock.JARSHA256 != "db9c4b738727698748746144394cb7ee96512fe50166555baaeeca9311ee1dba" {
		return errors.New("template manifest generator identity differs from reviewed values")
	}
	if len(stock.Entries) != len(acceptedCustomDigests) || len(deltas.Entries) != len(acceptedCustomDigests) {
		return errors.New("template manifests must contain exactly four entries")
	}
	deltaByName := make(map[string]TemplateDelta, len(deltas.Entries))
	for _, delta := range deltas.Entries {
		if _, duplicate := deltaByName[delta.Name]; duplicate {
			return fmt.Errorf("duplicate delta entry %s", delta.Name)
		}
		if delta.Policy == "" || len(delta.Hunks) == 0 {
			return fmt.Errorf("template delta %s has no reviewed policy or hunks", delta.Name)
		}
		deltaByName[delta.Name] = delta
	}
	for _, entry := range stock.Entries {
		customDigest, ok := acceptedCustomDigests[entry.Name]
		if !ok {
			return fmt.Errorf("unexpected template manifest entry %s", entry.Name)
		}
		if entry.StockPath != "go/"+entry.Name ||
			entry.CustomPath != CustomTemplateDir+"/"+entry.Name ||
			entry.StockSHA256 != acceptedStockDigests[entry.Name] ||
			entry.CustomSHA256 != customDigest {
			return fmt.Errorf("template manifest entry %s differs from reviewed paths or digests", entry.Name)
		}
		delta, ok := deltaByName[entry.Name]
		if !ok || delta.StockSHA256 != entry.StockSHA256 || delta.CustomSHA256 != entry.CustomSHA256 {
			return fmt.Errorf("template delta %s does not match the digest manifest", entry.Name)
		}
	}
	return nil
}

func readCustomFiles(repoRoot string) (map[string][]byte, error) {
	dir := filepath.Join(repoRoot, filepath.FromSlash(CustomTemplateDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlinked template file is forbidden: %s", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("non-regular template file is forbidden: %s", entry.Name())
		}
		if _, ok := acceptedCustomDigests[entry.Name()]; !ok {
			return nil, fmt.Errorf("extra template file is forbidden: %s", entry.Name())
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		result[entry.Name()] = content
	}
	if len(result) != len(acceptedCustomDigests) {
		return nil, fmt.Errorf("custom template count is %d, want %d", len(result), len(acceptedCustomDigests))
	}
	return result, nil
}

func readStockFiles(repoRoot string, manifest *StockManifest) (map[string][]byte, error) {
	jarPath := filepath.Join(repoRoot, "tools/generated-sdk/.cache/openapi-generator-cli-7.27.0.jar")
	data, err := os.ReadFile(jarPath)
	if err != nil {
		return nil, err
	}
	if digest := spec.Digest(data); digest != manifest.JARSHA256 {
		return nil, fmt.Errorf("generator JAR SHA-256 is %s, want %s", digest, manifest.JARSHA256)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	entries := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		entries[file.Name] = file
	}
	result := make(map[string][]byte, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		file := entries[entry.StockPath]
		if file == nil {
			return nil, fmt.Errorf("stock template %s is missing from generator JAR", entry.StockPath)
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		result[entry.Name] = content
	}
	return result, nil
}

func verifyFiles(stock *StockManifest, deltas *DeltaManifest, stockFiles, customFiles map[string][]byte) error {
	if len(stockFiles) != len(acceptedStockDigests) || len(customFiles) != len(acceptedCustomDigests) {
		return errors.New("stock and custom template sets must contain exactly the reviewed files")
	}
	for name := range customFiles {
		if _, ok := acceptedCustomDigests[name]; !ok {
			return fmt.Errorf("extra template file is forbidden: %s", name)
		}
	}
	deltaByName := make(map[string]TemplateDelta, len(deltas.Entries))
	for _, delta := range deltas.Entries {
		deltaByName[delta.Name] = delta
	}
	names := make([]string, 0, len(stock.Entries))
	for _, entry := range stock.Entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	for _, name := range names {
		stockContent, stockOK := stockFiles[name]
		customContent, customOK := customFiles[name]
		if !stockOK || !customOK {
			return fmt.Errorf("template %s is missing stock or custom bytes", name)
		}
		if digest := spec.Digest(stockContent); digest != acceptedStockDigests[name] {
			return fmt.Errorf("stock template %s SHA-256 is %s, want %s", name, digest, acceptedStockDigests[name])
		}
		if digest := spec.Digest(customContent); digest != acceptedCustomDigests[name] {
			return fmt.Errorf("custom template %s SHA-256 is %s, want %s", name, digest, acceptedCustomDigests[name])
		}
		if err := verifyDelta(name, stockContent, customContent, deltaByName[name]); err != nil {
			return err
		}
	}
	return nil
}

func verifyDelta(name string, stockContent, customContent []byte, delta TemplateDelta) error {
	stockLines, err := logicalLines(stockContent)
	if err != nil {
		return fmt.Errorf("%s stock: %w", name, err)
	}
	customLines, err := logicalLines(customContent)
	if err != nil {
		return fmt.Errorf("%s custom: %w", name, err)
	}
	var reconstructed []string
	cursor := 0
	stats := LineStats{}
	for index, hunk := range delta.Hunks {
		start := hunk.StockStart - 1
		if start < cursor || start > len(stockLines) {
			return fmt.Errorf("%s hunk %d has invalid or overlapping stock start", name, index+1)
		}
		if hunk.CustomStart != len(reconstructed)+(start-cursor)+1 {
			return fmt.Errorf("%s hunk %d custom start is inconsistent", name, index+1)
		}
		end := start + len(hunk.StockLines)
		if end > len(stockLines) || !reflect.DeepEqual(stockLines[start:end], hunk.StockLines) {
			return fmt.Errorf("%s hunk %d stock lines drifted", name, index+1)
		}
		if hunk.Semantic == "" {
			return fmt.Errorf("%s hunk %d has no semantic classification", name, index+1)
		}
		reconstructed = append(reconstructed, stockLines[cursor:start]...)
		reconstructed = append(reconstructed, hunk.CustomLines...)
		cursor = end
		stats.Deletions += len(hunk.StockLines)
		stats.Insertions += len(hunk.CustomLines)
	}
	reconstructed = append(reconstructed, stockLines[cursor:]...)
	if !reflect.DeepEqual(reconstructed, customLines) {
		return fmt.Errorf("%s contains semantic changes outside reviewed regions", name)
	}
	if stats != delta.LineStats {
		return fmt.Errorf("%s line stats are %+v, manifest records %+v", name, stats, delta.LineStats)
	}
	return nil
}

func logicalLines(content []byte) ([]string, error) {
	if len(content) == 0 || content[len(content)-1] != '\n' {
		return nil, errors.New("template must end with a newline")
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n"), nil
}
