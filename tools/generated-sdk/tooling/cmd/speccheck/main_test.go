package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	specpolicy "github.com/alpacahq/alpaca-trade-api-go/v3/tools/generated-sdk/tooling/internal/spec"
)

func TestCheckPinnedAndLatest(t *testing.T) {
	t.Parallel()
	canonical := []byte("{\n  \"info\": {\n    \"title\": \"test\",\n    \"version\": \"1\"\n  },\n  \"openapi\": \"3.1.2\",\n  \"paths\": {}\n}\n")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.2","paths":{}}`))
	}))
	defer server.Close()
	raw := []byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.2","paths":{}}`)
	root := specFixture(t, canonical, raw)
	client := rewriteClient(t, server)

	results, err := check(context.Background(), root, "pinned", client)
	if err != nil || !allStatuses(results, "current") {
		t.Fatalf("pinned check = %#v, %v", results, err)
	}
	results, err = check(context.Background(), root, "latest", client)
	if err != nil || !allStatuses(results, "current") {
		t.Fatalf("latest check = %#v, %v", results, err)
	}
}

func TestAllowlistProposalMatchesReviewedFixture(t *testing.T) {
	t.Parallel()
	canonical := []byte("{\n  \"info\": {\n    \"title\": \"test\",\n    \"version\": \"1\"\n  },\n  \"openapi\": \"3.1.2\",\n  \"paths\": {}\n}\n")
	root := specFixture(t, canonical, canonical)
	proposal, err := proposeAllowlist(root)
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := os.ReadFile(filepath.Join(root, allowlistPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(proposal, reviewed) {
		t.Fatalf("allowlist proposal differs from reviewed fixture:\n%s", proposal)
	}
}

func TestCheckReportsLatestDriftWithoutWriting(t *testing.T) {
	t.Parallel()
	pinned := []byte("{\n  \"info\": {\n    \"title\": \"test\",\n    \"version\": \"1\"\n  },\n  \"openapi\": \"3.1.2\",\n  \"paths\": {}\n}\n")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.3","paths":{}}`))
	}))
	defer server.Close()
	raw := []byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.3","paths":{}}`)
	root := specFixture(t, pinned, raw)

	results, err := check(context.Background(), root, "latest", rewriteClient(t, server))
	if err == nil || !allStatuses(results, "drift") || results[0].Drift == nil || results[1].Drift == nil {
		t.Fatalf("latest drift check = %#v, %v", results, err)
	}
	after, readErr := os.ReadFile(filepath.Join(root, "tools/generated-sdk/specs/trading.openapi.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(pinned) {
		t.Fatal("latest drift check modified the pinned specification")
	}
}

func TestCheckReportsApprovedURLByteDrift(t *testing.T) {
	t.Parallel()
	pinned := []byte("{\n  \"info\": {\n    \"title\": \"test\",\n    \"version\": \"1\"\n  },\n  \"openapi\": \"3.1.2\",\n  \"paths\": {}\n}\n")
	raw := []byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.2","paths":{}}`)
	root := specFixture(t, pinned, raw)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := raw
		if !strings.Contains(request.URL.Path, "/us/") {
			body = append(append([]byte(nil), raw...), '\n')
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}
	results, err := check(context.Background(), root, "latest", client)
	if err == nil || !allStatuses(results, "redirect-drift") {
		t.Fatalf("redirect drift check = %#v, %v", results, err)
	}
}

func TestProvenanceRequiresMatchingDates(t *testing.T) {
	t.Parallel()
	pinned := []byte("{\n  \"info\": {\n    \"title\": \"test\",\n    \"version\": \"1\"\n  },\n  \"openapi\": \"3.1.2\",\n  \"paths\": {}\n}\n")
	raw := []byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.2","paths":{}}`)
	root := specFixture(t, pinned, raw)
	path := filepath.Join(root, provenancePath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var metadata provenance
	if err := specpolicy.DecodeStrict(data, &metadata); err != nil {
		t.Fatal(err)
	}
	entry := metadata.SelectedSpecs["trading"]
	entry.RetrievalDate = "2026-11-19"
	metadata.SelectedSpecs["trading"] = entry
	data, err = specpolicy.Canonical(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProvenance(root); err == nil {
		t.Fatal("mismatched retrieval dates unexpectedly passed")
	}
}

func TestSemanticDriftReportsOperationsAndSchemas(t *testing.T) {
	t.Parallel()
	pinned := []byte(`{"paths":{"/old":{"get":{"operationId":"oldOperation","responses":{}}},"/changed":{"get":{"operationId":"changedOperation","summary":"old"}}},"components":{"schemas":{"Old":{"type":"string"},"Changed":{"type":"string"}}}}`)
	latest := []byte(`{"paths":{"/new":{"get":{"operationId":"newOperation","responses":{}}},"/changed":{"get":{"operationId":"changedOperation","summary":"new"}}},"components":{"schemas":{"New":{"type":"string"},"Changed":{"type":"integer"}}}}`)
	got, err := semanticDrift(pinned, latest)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got.Operations.Added) != "[newOperation]" ||
		fmt.Sprint(got.Operations.Changed) != "[changedOperation]" ||
		fmt.Sprint(got.Operations.Removed) != "[oldOperation]" {
		t.Fatalf("operation drift = %#v", got.Operations)
	}
	if fmt.Sprint(got.Schemas.Added) != "[New]" ||
		fmt.Sprint(got.Schemas.Changed) != "[Changed]" ||
		fmt.Sprint(got.Schemas.Removed) != "[Old]" {
		t.Fatalf("schema drift = %#v", got.Schemas)
	}
}

func TestCheckRejectsNonCanonicalPinnedSpec(t *testing.T) {
	t.Parallel()
	root := specFixture(
		t,
		[]byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.2","paths":{}}`),
		[]byte(`{"info":{"title":"test","version":"1"},"openapi":"3.1.2","paths":{}}`),
	)
	results, err := check(context.Background(), root, "pinned", http.DefaultClient)
	if err == nil || !allStatuses(results, "invalid") {
		t.Fatalf("non-canonical check = %#v, %v", results, err)
	}
}

func TestFetchRejectsOversizedResponse(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return oversizedResponse(), nil
	})}
	if _, err := fetch(
		context.Background(),
		client,
		"https://docs.alpaca.markets/us/openapi/trading-api.json",
	); err == nil {
		t.Fatal("oversized specification response unexpectedly passed")
	}
}

func TestContainedRegularFileRejectsSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "spec.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := containedRegularFile(root, "spec.json"); err == nil {
		t.Fatal("symlinked specification unexpectedly passed")
	}
}

func specFixture(t *testing.T, pinned, raw []byte) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, "tools/generated-sdk/specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	document, err := specpolicy.Parse(pinned, ".json")
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]specEntry{}
	inventories := make([]*specpolicy.Inventory, 0, 2)
	counts := map[string]int{}
	for _, target := range []string{"market-data", "trading"} {
		filename := target + ".openapi.json"
		if err := os.WriteFile(filepath.Join(specDir, filename), pinned, 0o644); err != nil {
			t.Fatal(err)
		}
		inventory, buildErr := specpolicy.BuildInventory(document, target)
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		inventories = append(inventories, inventory)
		counts[target] = inventory.OperationCount
		var apiName string
		if target == "market-data" {
			apiName = "market-data-api"
		} else {
			apiName = "trading-api"
		}
		selected[target] = specEntry{
			CanonicalSHA256:              specpolicy.Digest(pinned),
			ComponentSchemaCount:         0,
			InfoVersion:                  "1",
			OpenAPI:                      "3.1.2",
			OperationCount:               0,
			Path:                         "tools/generated-sdk/specs/" + filename,
			ReachableSchemaIdentityCount: 0,
			RedirectEquivalentURL:        "https://docs.alpaca.markets/openapi/" + apiName + ".json",
			RedirectRawSHA256:            specpolicy.Digest(raw),
			RetrievalAttestation:         "recorded-source-and-redirect-bytes",
			RetrievalDate:                "2026-11-20",
			SourceRawSHA256:              specpolicy.Digest(raw),
			SourceURL:                    "https://docs.alpaca.markets/us/openapi/" + apiName + ".json",
		}
	}
	provenanceData, err := specpolicy.Canonical(provenance{
		RetrievalDate:      "2026-11-20",
		SchemaVersion:      1,
		SelectedSpecs:      selected,
		SelectionRationale: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, provenancePath), provenanceData, 0o644); err != nil {
		t.Fatal(err)
	}
	allowlist := specpolicy.ReviewedAllowlist(counts, inventories...)
	allowlistData, err := specpolicy.Canonical(allowlist)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, allowlistPath), allowlistData, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func allStatuses(results []result, want string) bool {
	if len(results) != 2 {
		return false
	}
	for _, item := range results {
		if item.Status != want {
			return false
		}
	}
	return true
}

func rewriteClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	base := server.Client().Transport
	return &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			clone := request.Clone(request.Context())
			clone.URL.Scheme = target.Scheme
			clone.URL.Host = target.Host
			return base.RoundTrip(clone)
		}),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func oversizedResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", (64<<20)+1))),
	}
}
