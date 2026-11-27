package spec

import (
	"strings"
	"testing"
)

func TestCompareClassifiesSupportedChanges(t *testing.T) {
	tests := []struct {
		name     string
		proposed string
		want     Classification
		kind     string
	}{
		{
			name: "documentation",
			proposed: strings.Replace(
				baseDocument,
				`"operationId": "getWidget",`,
				`"operationId": "getWidget", "description": "Gets a widget.",`,
				1,
			),
			want: ClassificationDocumentationOnly,
			kind: "documentation",
		},
		{
			name: "optional property",
			proposed: strings.Replace(
				baseDocument,
				`"id": {"type": "string", "format": "uuid"},`,
				`"id": {"type": "string", "format": "uuid"}, "label": {"type": "string"},`,
				1,
			),
			want: ClassificationAdditive,
			kind: "property-added",
		},
		{
			name: "required property",
			proposed: strings.NewReplacer(
				`"required": ["id"]`, `"required": ["id", "label"]`,
				`"id": {"type": "string", "format": "uuid"},`,
				`"id": {"type": "string", "format": "uuid"}, "label": {"type": "string"},`,
			).Replace(baseDocument),
			want: ClassificationBreaking,
			kind: "required-property-added",
		},
		{
			name: "unknown top-level shape",
			proposed: strings.Replace(
				baseDocument,
				`"openapi": "3.1.2",`,
				`"openapi": "3.1.2", "x-future-policy": true,`,
				1,
			),
			want: ClassificationManualReview,
			kind: "extension-policy",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Compare("test", []byte(baseDocument), []byte(test.proposed))
			if err != nil {
				t.Fatal(err)
			}
			if got.Classification != test.want {
				t.Fatalf("classification = %q, want %q; changes=%#v", got.Classification, test.want, got.Changes)
			}
			found := false
			for _, change := range got.Changes {
				found = found || change.Kind == test.kind
			}
			if !found {
				t.Fatalf("missing change kind %q: %#v", test.kind, got.Changes)
			}
		})
	}
}

func TestCompareClassifiesEnumEvolution(t *testing.T) {
	added := strings.Replace(baseDocument, `["open", "closed", null]`, `["open", "closed", "pending", null]`, 1)
	comparison, err := Compare("test", []byte(baseDocument), []byte(added))
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Classification != ClassificationAdditive {
		t.Fatalf("enum addition = %q, want additive: %#v", comparison.Classification, comparison.Changes)
	}
	removed := strings.Replace(baseDocument, `["open", "closed", null]`, `["open", null]`, 1)
	comparison, err = Compare("test", []byte(baseDocument), []byte(removed))
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Classification != ClassificationBreaking {
		t.Fatalf("enum removal = %q, want breaking: %#v", comparison.Classification, comparison.Changes)
	}
}

func TestCompareMixedChangesFailClosed(t *testing.T) {
	additive := strings.Replace(
		baseDocument,
		`"id": {"type": "string", "format": "uuid"},`,
		`"id": {"type": "string", "format": "uuid"}, "label": {"type": "string"},`,
		1,
	)
	deprecated := strings.Replace(
		additive,
		`"operationId": "getWidget",`,
		`"operationId": "getWidget", "deprecated": true,`,
		1,
	)
	comparison, err := Compare("test", []byte(baseDocument), []byte(deprecated))
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Classification != ClassificationManualReview {
		t.Fatalf("mixed deprecation change = %q, want manual-review: %#v", comparison.Classification, comparison.Changes)
	}

	security := strings.Replace(
		additive,
		`"ApiKey": {"type": "apiKey", "in": "header", "name": "APCA-API-KEY-ID"}`,
		`"ApiKey": {"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}`,
		1,
	)
	comparison, err = Compare("test", []byte(baseDocument), []byte(security))
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Classification != ClassificationBreaking {
		t.Fatalf("mixed security change = %q, want breaking: %#v", comparison.Classification, comparison.Changes)
	}

	for name, recognized := range map[string]string{
		"additive": additive,
		"documentation": strings.Replace(
			baseDocument,
			`"operationId": "getWidget",`,
			`"operationId": "getWidget", "description": "Gets a widget.",`,
			1,
		),
	} {
		t.Run("unknown extension with "+name, func(t *testing.T) {
			proposed := strings.Replace(
				recognized,
				`"openapi": "3.1.2",`,
				`"openapi": "3.1.2", "x-future-policy": true,`,
				1,
			)
			comparison, err := Compare("test", []byte(baseDocument), []byte(proposed))
			if err != nil {
				t.Fatal(err)
			}
			if comparison.Classification != ClassificationManualReview {
				t.Fatalf("mixed unknown change = %q, want manual-review: %#v", comparison.Classification, comparison.Changes)
			}
			found := false
			for _, change := range comparison.Changes {
				found = found || change.Kind == "extension-policy"
			}
			if !found {
				t.Fatalf("mixed unknown change is missing extension-policy: %#v", comparison.Changes)
			}
		})
	}
}
