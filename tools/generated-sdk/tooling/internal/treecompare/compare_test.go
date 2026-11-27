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

func TestCompareClassifiesAdditiveStructAndEnumEvolutions(t *testing.T) {
	baseline := t.TempDir()
	proposed := t.TempDir()
	writeTestFile(t, baseline, "models.go", `package generated

type Order struct {
	ID string `+"`json:\"id\"`"+`
}

type ApiGetOrdersRequest struct {
	ctx   any
	limit *int32
}

type OrderStatus string

const (
	ORDERSTATUS_NEW OrderStatus = "new"
)

var AllowedOrderStatusEnumValues = []OrderStatus{
	"new",
}
`)
	writeTestFile(t, proposed, "models.go", `package generated

type Order struct {
	ID    string  `+"`json:\"id\"`"+`
	Label *string `+"`json:\"label,omitempty\"`"+`
}

func (o *Order) GetLabel() string { return "" }

type ApiGetOrdersRequest struct {
	ctx    any
	limit  *int32
	symbol *string
}

func (r ApiGetOrdersRequest) Symbol(v string) ApiGetOrdersRequest { return r }

type OrderStatus string

const (
	ORDERSTATUS_NEW     OrderStatus = "new"
	ORDERSTATUS_PENDING OrderStatus = "pending"
)

var AllowedOrderStatusEnumValues = []OrderStatus{
	"new",
	"pending",
}
`)
	result, err := Compare(baseline, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Summary.PublicChanged) != 0 || len(result.Summary.PublicRemoved) != 0 {
		t.Fatalf("additive evolution reported breaking changes: %#v", result.Summary)
	}
	for _, want := range []string{
		"models.go:type Order",
		"models.go:type ApiGetOrdersRequest",
		"models.go:var AllowedOrderStatusEnumValues",
	} {
		if !slices.Contains(result.Summary.PublicAdditive, want) {
			t.Fatalf("missing additive declaration %q in %#v", want, result.Summary.PublicAdditive)
		}
	}
	for _, want := range []string{
		"models.go:method *Order.GetLabel",
		"models.go:method ApiGetOrdersRequest.Symbol",
		"models.go:const ORDERSTATUS_PENDING",
	} {
		if !slices.Contains(result.Summary.PublicAdded, want) {
			t.Fatalf("missing added declaration %q in %#v", want, result.Summary.PublicAdded)
		}
	}
}

func TestCompareKeepsIncompatibleStructAndEnumChangesInPublicChanged(t *testing.T) {
	tests := []struct {
		name     string
		baseline string
		proposed string
		want     string
	}{
		{
			name:     "field tag changed",
			baseline: "package p\ntype M struct { ID string `json:\"id\"` }\n",
			proposed: "package p\ntype M struct { ID string `json:\"id,omitempty\"`; Extra *string }\n",
			want:     "m.go:type M",
		},
		{
			name:     "new unexported field added to all-exported struct",
			baseline: "package p\ntype M struct { ID string }\n",
			proposed: "package p\ntype M struct { ID string; hidden string }\n",
			want:     "m.go:type M",
		},
		{
			name:     "embedded field added",
			baseline: "package p\ntype Base struct{}\ntype M struct { ID string }\n",
			proposed: "package p\ntype Base struct{}\ntype M struct { ID string; Base }\n",
			want:     "m.go:type M",
		},
		{
			name:     "enum value removed",
			baseline: "package p\ntype E string\nvar AllowedEEnumValues = []E{\"a\", \"b\"}\n",
			proposed: "package p\ntype E string\nvar AllowedEEnumValues = []E{\"a\", \"c\"}\n",
			want:     "m.go:var AllowedEEnumValues",
		},
		{
			name:     "non-enum slice var modified",
			baseline: "package p\nvar OtherValues = []string{\"a\"}\n",
			proposed: "package p\nvar OtherValues = []string{\"a\", \"b\"}\n",
			want:     "m.go:var OtherValues",
		},
		{
			name:     "comparable struct loses comparability via slice field",
			baseline: "package p\ntype CaEventReorganizationStockMovement struct { Cusip string; Isin *string; Symbol string }\n",
			proposed: "package p\ntype CaEventReorganizationStockMovement struct { Cusip string; Isin *string; Symbol string; Notes []string }\n",
			want:     "m.go:type CaEventReorganizationStockMovement",
		},
		{
			name:     "comparable struct loses comparability via map field",
			baseline: "package p\ntype M struct { ID string }\n",
			proposed: "package p\ntype M struct { ID string; Meta map[string]string }\n",
			want:     "m.go:type M",
		},
		{
			name:     "comparable struct loses comparability via json.RawMessage field",
			baseline: "package p\nimport \"encoding/json\"\ntype M struct { ID string; _ *json.RawMessage }\n",
			proposed: "package p\nimport \"encoding/json\"\ntype M struct { ID string; _ *json.RawMessage; Raw json.RawMessage }\n",
			want:     "m.go:type M",
		},
		{
			name:     "strictly comparable struct degrades to interface comparability",
			baseline: "package p\ntype M struct { ID string }\n",
			proposed: "package p\ntype M struct { ID string; Payload any }\n",
			want:     "m.go:type M",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := t.TempDir()
			proposed := t.TempDir()
			writeTestFile(t, baseline, "m.go", test.baseline)
			writeTestFile(t, proposed, "m.go", test.proposed)
			result, err := Compare(baseline, proposed)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(result.Summary.PublicChanged, test.want) {
				t.Fatalf("expected %q in PublicChanged, got summary=%#v", test.want, result.Summary)
			}
			if slices.Contains(result.Summary.PublicAdditive, test.want) {
				t.Fatalf("unexpectedly classified %q as PublicAdditive: %#v", test.want, result.Summary)
			}
		})
	}
}

func TestCompareTracksPackageComparabilityAcrossFilesAndIncomparableBaselines(t *testing.T) {
	baseline := t.TempDir()
	proposed := t.TempDir()
	writeTestFile(t, baseline, "pkg/utils.go", "package pkg\ntype NullableString struct { value *string; isSet bool }\n")
	writeTestFile(t, baseline, "pkg/movement.go", "package pkg\ntype Movement struct { Symbol string }\n")
	writeTestFile(t, baseline, "pkg/wrapper.go", "package pkg\ntype Wrapper struct { Item Movement }\n")
	writeTestFile(t, baseline, "pkg/extensible.go", "package pkg\nimport \"encoding/json\"\ntype ExtensibleOrder struct { ID string; AdditionalProperties map[string]json.RawMessage }\n")
	writeTestFile(t, baseline, "pkg/comparable_keep.go", "package pkg\nimport \"time\"\ntype ComparableEvent struct { ID string }\nvar _ time.Time\n")

	writeTestFile(t, proposed, "pkg/utils.go", "package pkg\ntype NullableString struct { value *string; isSet bool }\n")
	writeTestFile(t, proposed, "pkg/movement.go", "package pkg\ntype Movement struct { Symbol string; Notes []string }\n")
	writeTestFile(t, proposed, "pkg/wrapper.go", "package pkg\ntype Wrapper struct { Item Movement }\n")
	writeTestFile(t, proposed, "pkg/extensible.go", "package pkg\nimport \"encoding/json\"\ntype ExtensibleOrder struct { ID string; Tags []string; AdditionalProperties map[string]json.RawMessage }\n")
	writeTestFile(t, proposed, "pkg/comparable_keep.go", "package pkg\nimport \"time\"\ntype ComparableEvent struct { ID string; At time.Time; Note NullableString; Items *[]string }\n")

	result, err := Compare(baseline, proposed)
	if err != nil {
		t.Fatal(err)
	}
	for _, wantChanged := range []string{
		"pkg/movement.go:type Movement",
		"pkg/wrapper.go:type Wrapper",
	} {
		if !slices.Contains(result.Summary.PublicChanged, wantChanged) {
			t.Fatalf("missing comparability-breaking declaration %q in PublicChanged: %#v", wantChanged, result.Summary)
		}
	}
	for _, wantAdditive := range []string{
		"pkg/extensible.go:type ExtensibleOrder",
		"pkg/comparable_keep.go:type ComparableEvent",
	} {
		if !slices.Contains(result.Summary.PublicAdditive, wantAdditive) {
			t.Fatalf("missing additive declaration %q in PublicAdditive: %#v", wantAdditive, result.Summary)
		}
		if slices.Contains(result.Summary.PublicChanged, wantAdditive) {
			t.Fatalf("unexpectedly marked %q as PublicChanged: %#v", wantAdditive, result.Summary)
		}
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
