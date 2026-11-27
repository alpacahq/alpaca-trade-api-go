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

func TestComparePreservesPropertiesNamedLikeDocumentationFields(t *testing.T) {
	for _, field := range []string{"description", "summary", "example", "examples", "externalDocs"} {
		t.Run(field, func(t *testing.T) {
			withField := strings.Replace(
				baseDocument,
				`"id": {"type": "string", "format": "uuid"},`,
				`"id": {"type": "string", "format": "uuid"}, "`+field+`": {"type": "string", "description": "initial doc"},`,
				1,
			)
			added, err := Compare("test", []byte(baseDocument), []byte(withField))
			if err != nil {
				t.Fatal(err)
			}
			if added.Classification != ClassificationAdditive {
				t.Fatalf("adding property %q = %q, want additive: %#v", field, added.Classification, added.Changes)
			}
			if !hasChange(added.Changes, "property-added", "Widget."+field) {
				t.Fatalf("missing property-added for %q: %#v", field, added.Changes)
			}

			typeChanged := strings.Replace(
				withField,
				`"`+field+`": {"type": "string", "description": "initial doc"},`,
				`"`+field+`": {"type": "integer", "description": "initial doc"},`,
				1,
			)
			changed, err := Compare("test", []byte(withField), []byte(typeChanged))
			if err != nil {
				t.Fatal(err)
			}
			if changed.Classification != ClassificationBreaking {
				t.Fatalf("changing property %q = %q, want breaking: %#v", field, changed.Classification, changed.Changes)
			}
			if !hasChange(changed.Changes, "property-changed", "Widget."+field) {
				t.Fatalf("missing property-changed for %q: %#v", field, changed.Changes)
			}

			removed, err := Compare("test", []byte(withField), []byte(baseDocument))
			if err != nil {
				t.Fatal(err)
			}
			if removed.Classification != ClassificationBreaking {
				t.Fatalf("removing property %q = %q, want breaking: %#v", field, removed.Classification, removed.Changes)
			}
			if !hasChange(removed.Changes, "property-removed", "Widget."+field) {
				t.Fatalf("missing property-removed for %q: %#v", field, removed.Changes)
			}

			docChanged := strings.Replace(
				withField,
				`"description": "initial doc"`,
				`"description": "updated doc"`,
				1,
			)
			docOnly, err := Compare("test", []byte(withField), []byte(docChanged))
			if err != nil {
				t.Fatal(err)
			}
			if docOnly.Classification != ClassificationDocumentationOnly {
				t.Fatalf("editing doc string inside property %q = %q, want documentation-only: %#v", field, docOnly.Classification, docOnly.Changes)
			}
		})
	}
}

func TestComparePreservesLiteralValuesAndInlineSchemaDocs(t *testing.T) {
	withDefaultObject := strings.Replace(
		baseDocument,
		`"id": {"type": "string", "format": "uuid"},`,
		`"id": {"type": "string", "format": "uuid"}, "meta": {"type": "object", "default": {"description": "v1"}},`,
		1,
	)
	changedDefaultObject := strings.Replace(withDefaultObject, `"default": {"description": "v1"}`, `"default": {"description": "v2"}`, 1)
	comparison, err := Compare("test", []byte(withDefaultObject), []byte(changedDefaultObject))
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Classification != ClassificationBreaking {
		t.Fatalf("changing literal default object key 'description' = %q, want breaking: %#v", comparison.Classification, comparison.Changes)
	}

	inlineParamDoc := strings.Replace(
		baseDocument,
		`"schema": {"type": "string", "format": "uuid"}`,
		`"schema": {"type": "string", "format": "uuid", "description": "Widget identifier."}`,
		1,
	)
	docComparison, err := Compare("test", []byte(baseDocument), []byte(inlineParamDoc))
	if err != nil {
		t.Fatal(err)
	}
	if docComparison.Classification != ClassificationDocumentationOnly {
		t.Fatalf("inline parameter schema description change = %q, want documentation-only: %#v", docComparison.Classification, docComparison.Changes)
	}
}

func TestCompareClassifiesTopLevelComponentEnumEvolution(t *testing.T) {
	withEnum := strings.Replace(
		baseDocument,
		`"Widget": {`,
		`"WidgetState": {"type": "string", "enum": ["open", "closed"]}, "Widget": {`,
		1,
	)
	added := strings.Replace(withEnum, `["open", "closed"]`, `["open", "closed", "pending"]`, 1)
	comparison, err := Compare("test", []byte(withEnum), []byte(added))
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Classification != ClassificationAdditive {
		t.Fatalf("top-level enum addition = %q, want additive: %#v", comparison.Classification, comparison.Changes)
	}
	if !hasChange(comparison.Changes, "enum-value-added", "WidgetState") {
		t.Fatalf("missing top-level enum-value-added: %#v", comparison.Changes)
	}

	removed := strings.Replace(withEnum, `["open", "closed"]`, `["open"]`, 1)
	comparison, err = Compare("test", []byte(withEnum), []byte(removed))
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Classification != ClassificationBreaking {
		t.Fatalf("top-level enum removal = %q, want breaking: %#v", comparison.Classification, comparison.Changes)
	}
	if !hasChange(comparison.Changes, "enum-value-removed", "WidgetState") {
		t.Fatalf("missing top-level enum-value-removed: %#v", comparison.Changes)
	}
}

func hasChange(changes []Change, kind, subject string) bool {
	for _, change := range changes {
		if change.Kind == kind && change.Subject == subject {
			return true
		}
	}
	return false
}
