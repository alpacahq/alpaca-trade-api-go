package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	specpolicy "github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
	"github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/specsource"
)

const (
	provenancePath = "tools/generated-sdk/specs/provenance.json"
	allowlistPath  = "tools/generated-sdk/specs/feature-allowlist.json"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type provenance struct {
	RetrievalDate      string               `json:"retrieval_date"`
	SchemaVersion      int                  `json:"schema_version"`
	SelectedSpecs      map[string]specEntry `json:"selected_specs"`
	SelectionRationale string               `json:"selection_rationale"`
}

type specEntry struct {
	CanonicalSHA256              string `json:"canonical_sha256"`
	ComponentSchemaCount         int    `json:"component_schema_count"`
	InfoVersion                  string `json:"info_version"`
	OpenAPI                      string `json:"openapi"`
	OperationCount               int    `json:"operation_count"`
	Path                         string `json:"path"`
	RedirectRawSHA256            string `json:"redirect_raw_sha256"`
	ReachableSchemaIdentityCount int    `json:"reachable_schema_identity_count"`
	RedirectEquivalentURL        string `json:"redirect_equivalent_url"`
	RetrievalAttestation         string `json:"retrieval_attestation"`
	RetrievalDate                string `json:"retrieval_date"`
	SourceRawSHA256              string `json:"source_raw_sha256"`
	SourceURL                    string `json:"source_url"`
}

type result struct {
	Target          string `json:"target"`
	Status          string `json:"status"`
	Path            string `json:"path"`
	SourceURL       string `json:"source_url"`
	PinnedSHA256    string `json:"pinned_sha256"`
	LatestSHA256    string `json:"latest_sha256,omitempty"`
	LatestRawSHA256 string `json:"latest_raw_sha256,omitempty"`
	RedirectSHA256  string `json:"latest_redirect_raw_sha256,omitempty"`
	Drift           *drift `json:"drift,omitempty"`
}

type drift struct {
	Operations changeSet `json:"operations"`
	Schemas    changeSet `json:"schemas"`
}

type changeSet struct {
	Added   []string `json:"added"`
	Changed []string `json:"changed"`
	Removed []string `json:"removed"`
}

func main() {
	mode := flag.String("mode", "pinned", "pinned, latest, or allowlist-proposal")
	rootFlag := flag.String("repo-root", ".", "repository root")
	timeout := flag.Duration("timeout", 30*time.Second, "latest-spec request timeout")
	flag.Parse()
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		exit(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	client := specsource.NewClient(*timeout)
	if *mode == "allowlist-proposal" {
		proposal, err := proposeAllowlist(root)
		if err != nil {
			exit(err)
		}
		if _, err := os.Stdout.Write(proposal); err != nil {
			exit(err)
		}
		return
	}
	results, err := check(ctx, root, *mode, client)
	if encodeErr := json.NewEncoder(os.Stdout).Encode(results); encodeErr != nil {
		exit(encodeErr)
	}
	if err != nil {
		exit(err)
	}
}

func proposeAllowlist(root string) ([]byte, error) {
	metadata, err := loadProvenance(root)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(metadata.SelectedSpecs))
	for target := range metadata.SelectedSpecs {
		targets = append(targets, target)
	}
	slices.Sort(targets)
	counts := make(map[string]int, len(targets))
	inventories := make([]*specpolicy.Inventory, 0, len(targets))
	for _, target := range targets {
		entry := metadata.SelectedSpecs[target]
		path, err := containedRegularFile(root, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", target, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		document, err := specpolicy.Parse(data, filepath.Ext(entry.Path))
		if err != nil {
			return nil, fmt.Errorf("%s feature inventory: %w", target, err)
		}
		inventory, err := specpolicy.BuildInventory(document, target)
		if err != nil {
			return nil, fmt.Errorf("%s feature inventory: %w", target, err)
		}
		counts[target] = inventory.OperationCount
		inventories = append(inventories, inventory)
	}
	return specpolicy.Canonical(specpolicy.ReviewedAllowlist(counts, inventories...))
}

func check(ctx context.Context, root, mode string, client *http.Client) ([]result, error) {
	if mode != "pinned" && mode != "latest" {
		return nil, fmt.Errorf("unsupported spec check mode %q", mode)
	}
	metadata, err := loadProvenance(root)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(metadata.SelectedSpecs))
	for target := range metadata.SelectedSpecs {
		targets = append(targets, target)
	}
	slices.Sort(targets)
	results := make([]result, 0, len(targets))
	inventories := make([]*specpolicy.Inventory, 0, len(targets))
	var failures []error
	for _, target := range targets {
		entry := metadata.SelectedSpecs[target]
		specPath, pathErr := containedRegularFile(root, entry.Path)
		if pathErr != nil {
			failures = append(failures, fmt.Errorf("%s: %w", target, pathErr))
			results = append(results, result{Target: target, Status: "invalid", Path: entry.Path})
			continue
		}
		pinned, readErr := os.ReadFile(specPath)
		item := result{
			Target: target, Status: "current", Path: entry.Path,
			SourceURL: entry.SourceURL, PinnedSHA256: digest(pinned),
		}
		if readErr != nil {
			item.Status = "invalid"
			failures = append(failures, fmt.Errorf("%s: %w", target, readErr))
			results = append(results, item)
			continue
		}
		if item.PinnedSHA256 != entry.CanonicalSHA256 {
			item.Status = "invalid"
			failures = append(failures, fmt.Errorf("%s pinned digest differs from provenance", target))
		}
		canonical, canonicalErr := canonicalJSON(pinned)
		if canonicalErr != nil || !bytes.Equal(canonical, pinned) {
			item.Status = "invalid"
			if canonicalErr != nil {
				failures = append(failures, fmt.Errorf("%s pinned specification: %w", target, canonicalErr))
			} else {
				failures = append(failures, fmt.Errorf("%s pinned specification is not canonical JSON", target))
			}
		}
		document, parseErr := specpolicy.Parse(pinned, filepath.Ext(entry.Path))
		if parseErr != nil {
			item.Status = "invalid"
			failures = append(failures, fmt.Errorf("%s feature inventory: %w", target, parseErr))
		} else {
			inventory, inventoryErr := specpolicy.BuildInventory(document, target)
			if inventoryErr != nil {
				item.Status = "invalid"
				failures = append(failures, fmt.Errorf("%s feature inventory: %w", target, inventoryErr))
			} else {
				componentCount := 0
				if components, ok := document.Root["components"].(map[string]any); ok {
					if schemas, ok := components["schemas"].(map[string]any); ok {
						componentCount = len(schemas)
					}
				}
				if inventory.OpenAPI != entry.OpenAPI ||
					inventory.InfoVersion != entry.InfoVersion ||
					inventory.OperationCount != entry.OperationCount ||
					inventory.ReachableSchemaCount != entry.ReachableSchemaIdentityCount ||
					componentCount != entry.ComponentSchemaCount {
					item.Status = "invalid"
					failures = append(failures, fmt.Errorf("%s specification counts or versions differ from provenance", target))
				} else {
					inventories = append(inventories, inventory)
				}
			}
		}
		if mode == "latest" && item.Status == "current" {
			raw, fetchErr := fetch(ctx, client, entry.SourceURL)
			if fetchErr != nil {
				item.Status = "unavailable"
				failures = append(failures, fmt.Errorf("%s: %w", target, fetchErr))
			} else {
				item.LatestRawSHA256 = digest(raw)
				redirectRaw, redirectErr := fetch(ctx, client, entry.RedirectEquivalentURL)
				if redirectErr != nil {
					item.Status = "unavailable"
					failures = append(failures, fmt.Errorf("%s redirect form: %w", target, redirectErr))
					results = append(results, item)
					continue
				}
				item.RedirectSHA256 = digest(redirectRaw)
				if !bytes.Equal(raw, redirectRaw) {
					item.Status = "redirect-drift"
					failures = append(failures, fmt.Errorf("%s approved URL forms returned different bytes", target))
					results = append(results, item)
					continue
				}
				latest, latestErr := canonicalJSON(raw)
				if latestErr != nil {
					item.Status = "invalid-upstream"
					failures = append(failures, fmt.Errorf("%s latest specification: %w", target, latestErr))
				} else {
					item.LatestSHA256 = digest(latest)
					if !bytes.Equal(latest, pinned) {
						item.Status = "drift"
						summary, summaryErr := semanticDrift(pinned, latest)
						if summaryErr != nil {
							failures = append(failures, fmt.Errorf("%s drift summary: %w", target, summaryErr))
						} else {
							item.Drift = &summary
						}
						failures = append(failures, fmt.Errorf("%s latest specification differs from pinned source", target))
					} else if item.LatestRawSHA256 != entry.SourceRawSHA256 ||
						item.RedirectSHA256 != entry.RedirectRawSHA256 {
						item.Status = "raw-drift"
						failures = append(failures, fmt.Errorf("%s latest raw source differs from recorded provenance", target))
					}
				}
			}
		}
		results = append(results, item)
	}
	if len(inventories) == len(targets) {
		if err := checkAllowlist(root, inventories); err != nil {
			failures = append(failures, err)
		}
	}
	return results, errors.Join(failures...)
}

func loadProvenance(root string) (provenance, error) {
	path, err := containedRegularFile(root, provenancePath)
	if err != nil {
		return provenance{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return provenance{}, err
	}
	var result provenance
	if err := specpolicy.DecodeStrict(data, &result); err != nil {
		return provenance{}, err
	}
	canonical, err := specpolicy.Canonical(result)
	if err != nil {
		return provenance{}, err
	}
	if !bytes.Equal(data, canonical) {
		return provenance{}, errors.New("specification provenance is not canonical JSON")
	}
	if result.SchemaVersion != 1 {
		return provenance{}, errors.New("invalid specification provenance")
	}
	expected := map[string]struct {
		path        string
		url         string
		redirectURL string
	}{
		"trading": {
			path:        "tools/generated-sdk/specs/trading.openapi.json",
			url:         "https://docs.alpaca.markets/us/openapi/trading-api.json",
			redirectURL: "https://docs.alpaca.markets/openapi/trading-api.json",
		},
		"market-data": {
			path:        "tools/generated-sdk/specs/market-data.openapi.json",
			url:         "https://docs.alpaca.markets/us/openapi/market-data-api.json",
			redirectURL: "https://docs.alpaca.markets/openapi/market-data-api.json",
		},
	}
	if len(result.SelectedSpecs) != len(expected) {
		return provenance{}, fmt.Errorf("specification provenance has %d targets, want %d", len(result.SelectedSpecs), len(expected))
	}
	retrievalDate, err := time.Parse("2006-01-02", result.RetrievalDate)
	if err != nil || retrievalDate.Format("2006-01-02") != result.RetrievalDate {
		return provenance{}, errors.New("top-level retrieval date must use YYYY-MM-DD")
	}
	for target, entry := range result.SelectedSpecs {
		want, ok := expected[target]
		if !ok || entry.Path != want.path || entry.SourceURL != want.url ||
			entry.RedirectEquivalentURL != want.redirectURL {
			return provenance{}, fmt.Errorf("%s has unapproved specification path or URL", target)
		}
		if entry.RetrievalAttestation != "recorded-source-and-redirect-bytes" ||
			entry.RetrievalDate != result.RetrievalDate {
			return provenance{}, fmt.Errorf("%s has incomplete retrieval provenance", target)
		}
		if _, err := time.Parse("2006-01-02", entry.RetrievalDate); err != nil {
			return provenance{}, fmt.Errorf("%s has invalid retrieval date: %w", target, err)
		}
		if !sha256Pattern.MatchString(entry.CanonicalSHA256) ||
			!sha256Pattern.MatchString(entry.SourceRawSHA256) ||
			!sha256Pattern.MatchString(entry.RedirectRawSHA256) ||
			entry.OpenAPI != specpolicy.OpenAPIVersion {
			return provenance{}, fmt.Errorf("%s has incomplete specification provenance", target)
		}
		if _, err := approvedSourceURL(entry.SourceURL); err != nil {
			return provenance{}, fmt.Errorf("%s: %w", target, err)
		}
		if _, err := approvedSourceURL(entry.RedirectEquivalentURL); err != nil {
			return provenance{}, fmt.Errorf("%s redirect form: %w", target, err)
		}
	}
	return result, nil
}

func approvedSourceURL(value string) (*url.URL, error) {
	return specsource.ApprovedURL(value)
}

func fetch(ctx context.Context, client *http.Client, value string) ([]byte, error) {
	return specsource.Fetch(ctx, client, value)
}

func containedRegularFile(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) ||
		filepath.Clean(relative) != filepath.FromSlash(relative) ||
		!filepath.IsLocal(relative) {
		return "", fmt.Errorf("unsafe repository-relative path %q", relative)
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", relative)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	inside, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s escapes the repository root", relative)
	}
	return resolvedPath, nil
}

func checkAllowlist(root string, inventories []*specpolicy.Inventory) error {
	path, err := containedRegularFile(root, allowlistPath)
	if err != nil {
		return fmt.Errorf("feature allowlist: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	allowlist, err := specpolicy.DecodeAllowlist(data)
	if err != nil {
		return fmt.Errorf("decode feature allowlist: %w", err)
	}
	if err := specpolicy.CheckAllowlist(allowlist, inventories...); err != nil {
		return fmt.Errorf("pinned specification feature policy: %w", err)
	}
	return nil
}

func canonicalJSON(data []byte) ([]byte, error) {
	value, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(canonical, '\n'), nil
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("specification contains trailing JSON")
	}
	return value, nil
}

func semanticDrift(pinned, latest []byte) (drift, error) {
	pinnedDocument, err := decodeDocument(pinned)
	if err != nil {
		return drift{}, err
	}
	latestDocument, err := decodeDocument(latest)
	if err != nil {
		return drift{}, err
	}
	return drift{
		Operations: compareItems(operationItems(pinnedDocument), operationItems(latestDocument)),
		Schemas:    compareItems(schemaItems(pinnedDocument), schemaItems(latestDocument)),
	}, nil
}

func decodeDocument(data []byte) (map[string]any, error) {
	value, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	document, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("specification root is not an object")
	}
	return document, nil
}

func operationItems(document map[string]any) map[string]string {
	result := map[string]string{}
	paths, _ := document["paths"].(map[string]any)
	methods := map[string]bool{
		"delete": true, "get": true, "head": true, "options": true,
		"patch": true, "post": true, "put": true, "trace": true,
	}
	for _, pathValue := range paths {
		pathObject, _ := pathValue.(map[string]any)
		for method, operationValue := range pathObject {
			if !methods[method] {
				continue
			}
			operation, _ := operationValue.(map[string]any)
			operationID, _ := operation["operationId"].(string)
			if operationID != "" {
				result[operationID] = canonicalDigest(operation)
			}
		}
	}
	return result
}

func schemaItems(document map[string]any) map[string]string {
	result := map[string]string{}
	components, _ := document["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for name, schema := range schemas {
		result[name] = canonicalDigest(schema)
	}
	return result
}

func canonicalDigest(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return digest(data)
}

func compareItems(pinned, latest map[string]string) changeSet {
	result := changeSet{Added: []string{}, Changed: []string{}, Removed: []string{}}
	for name, latestDigest := range latest {
		pinnedDigest, exists := pinned[name]
		switch {
		case !exists:
			result.Added = append(result.Added, name)
		case pinnedDigest != latestDigest:
			result.Changed = append(result.Changed, name)
		}
	}
	for name := range pinned {
		if _, exists := latest[name]; !exists {
			result.Removed = append(result.Removed, name)
		}
	}
	slices.Sort(result.Added)
	slices.Sort(result.Changed)
	slices.Sort(result.Removed)
	return result
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
