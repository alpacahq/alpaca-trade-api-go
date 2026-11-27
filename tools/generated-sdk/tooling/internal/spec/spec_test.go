package spec

import (
	"bytes"
	"strings"
	"testing"
)

const baseDocument = `{
  "openapi": "3.1.2",
  "info": {"title": "test", "version": "1"},
  "servers": [{"url": "https://api.example.test/v1", "variables": {}}],
  "paths": {
    "/widgets/{id}": {
      "get": {
        "operationId": "getWidget",
        "tags": ["widgets"],
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "string", "format": "uuid"}},
          {"name": "fields", "in": "query", "style": "form", "explode": false, "schema": {"type": "array", "items": {"type": "string"}}}
        ],
        "responses": {
          "200": {
            "headers": {"X-Next": {"schema": {"type": "string"}}},
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}
          }
        },
        "security": [{"ApiKey": []}]
      }
    }
  },
  "components": {
    "securitySchemes": {"ApiKey": {"type": "apiKey", "in": "header", "name": "APCA-API-KEY-ID"}},
    "schemas": {
      "Widget": {
        "type": "object",
        "required": ["id"],
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "state": {"type": ["string", "null"], "enum": ["open", "closed", null], "default": "open"},
          "detail": {"oneOf": [{"type": "string"}, {"type": "integer", "format": "int64"}]}
        },
        "additionalProperties": false
      }
    }
  }
}`

func TestCanonicalJSONDeterminism(t *testing.T) {
	jsonDoc, err := Parse([]byte(`{"z":[3,2,1],"n":1.2300,"a":{"b":true,"a":"x"}}`), ".json")
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": {\n    \"a\": \"x\",\n    \"b\": true\n  },\n  \"n\": 1.2300,\n  \"z\": [\n    3,\n    2,\n    1\n  ]\n}\n"
	first, err := Canonical(jsonDoc.Root)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != want {
		t.Fatalf("canonical JSON mismatch:\n%s", first)
	}
	secondDoc, err := Parse(first, ".json")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Canonical(secondDoc.Root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonicalization is not idempotent")
	}
	if _, err := Parse(first, ".yaml"); err == nil {
		t.Fatal("unsupported YAML input unexpectedly passed")
	}
	if first[len(first)-1] != '\n' {
		t.Fatal("canonical output has no trailing newline")
	}
}

func TestInventoryCapturesGateDimensions(t *testing.T) {
	_, inventory := buildInventory(t, baseDocument)
	if inventory.OperationCount != 1 {
		t.Fatalf("operation count = %d", inventory.OperationCount)
	}
	op := inventory.Operations[0]
	if op.Method != "GET" || op.Path != "/widgets/{id}" || op.OperationID != "getWidget" {
		t.Fatalf("operation identity = %#v", op)
	}
	if len(op.Parameters) != 2 || len(op.Responses) != 1 || len(op.Responses[0].Headers) != 1 {
		t.Fatalf("operation dimensions missing: %#v", op)
	}
	if len(inventory.SecuritySchemes) != 1 || len(op.ReachableSchemas) == 0 {
		t.Fatalf("security or reachable schemas missing: %#v", inventory)
	}
	if op.ServerSource != "root" || len(op.Servers) != 1 || op.Servers[0].URL != "https://api.example.test/v1" {
		t.Fatalf("server selection missing: %#v", op)
	}
}

func TestFailClosedMutations(t *testing.T) {
	_, baseline := buildInventory(t, baseDocument)
	allowlist := ReviewedAllowlist(map[string]int{"test": 1}, baseline)
	tests := []struct {
		name       string
		mutate     func(string) string
		buildFails bool
	}{
		{"version", func(value string) string { return strings.Replace(value, `"3.1.2"`, `"3.2.0"`, 1) }, true},
		{"unknown-root-field", func(value string) string {
			return strings.Replace(value, `"paths": {`, `"futureRoot": true, "paths": {`, 1)
		}, true},
		{"webhooks", func(value string) string {
			return strings.Replace(value, `"paths": {`, `"webhooks": {}, "paths": {`, 1)
		}, true},
		{"unknown-components-field", func(value string) string {
			return strings.Replace(value, `"securitySchemes": {`, `"futureComponents": {}, "securitySchemes": {`, 1)
		}, true},
		{"query-method", func(value string) string { return strings.Replace(value, `"get": {`, `"QUERY": {`, 1) }, true},
		{"custom-method", func(value string) string { return strings.Replace(value, `"get": {`, `"WATCH": {`, 1) }, true},
		{"unknown-path-item-field", func(value string) string {
			return strings.Replace(value, `"get": {`, `"futurePathItem": true, "get": {`, 1)
		}, true},
		{"unknown-operation-field", func(value string) string {
			return strings.Replace(value, `"operationId": "getWidget"`, `"operationId": "getWidget", "futureOperation": true`, 1)
		}, true},
		{"additional-operations", func(value string) string {
			return strings.Replace(value, `"get": {`, `"additionalOperations": {}, "get": {`, 1)
		}, true},
		{"querystring", func(value string) string { return strings.Replace(value, `"in": "query"`, `"in": "querystring"`, 1) }, true},
		{"unknown-parameter-field", func(value string) string {
			return strings.Replace(value, `"name": "id"`, `"name": "id", "futureParameter": true`, 1)
		}, true},
		{"parameter-content", func(value string) string {
			return strings.Replace(value, `"schema": {"type": "string", "format": "uuid"}`, `"content": {"application/json": {"schema": {"type": "string"}}}`, 1)
		}, true},
		{"unknown-request-body-field", func(value string) string {
			return strings.Replace(value, `"responses": {`, `"requestBody": {"futureBody": true, "content": {}}, "responses": {`, 1)
		}, true},
		{"unknown-response-field", func(value string) string {
			return strings.Replace(value, `"headers": {`, `"futureResponse": true, "headers": {`, 1)
		}, true},
		{"unknown-header-field", func(value string) string {
			return strings.Replace(value, `"X-Next": {"schema":`, `"X-Next": {"futureHeader": true, "schema":`, 1)
		}, true},
		{"unknown-media-field", func(value string) string {
			return strings.Replace(value, `"application/json": {"schema":`, `"application/json": {"futureMedia": true, "schema":`, 1)
		}, true},
		{"media-encoding", func(value string) string {
			return strings.Replace(value, `"application/json": {"schema":`, `"application/json": {"encoding": {}, "schema":`, 1)
		}, true},
		{"unknown-security-field", func(value string) string {
			return strings.Replace(value, `"type": "apiKey"`, `"type": "apiKey", "futureSecurity": true`, 1)
		}, true},
		{"unknown-server-field", func(value string) string {
			return strings.Replace(value, `"url": "https://api.example.test/v1"`, `"url": "https://api.example.test/v1", "futureServer": true`, 1)
		}, true},
		{"external-reference", func(value string) string {
			return strings.Replace(value, `"#/components/schemas/Widget"`, `"https://example.test/schema.json"`, 1)
		}, true},
		{"item-schema", func(value string) string {
			return strings.Replace(value, `"items": {"type": "string"}`, `"items": {"type": "string"}, "itemSchema": {"type": "string"}`, 1)
		}, true},
		{"media-item-schema", func(value string) string {
			return strings.Replace(value, `"application/json": {"schema":`, `"application/json": {"itemSchema": {"type": "string"}, "schema":`, 1)
		}, true},
		{"unknown-keyword", func(value string) string {
			return strings.Replace(value, `"type": "object"`, `"type": "object", "xorOf": []`, 1)
		}, true},
		{"callbacks", func(value string) string {
			return strings.Replace(value, `"responses": {`, `"callbacks": {"later": {}}, "responses": {`, 1)
		}, true},
		{"links", func(value string) string {
			return strings.Replace(value, `"headers": {`, `"links": {"next": {}}, "headers": {`, 1)
		}, true},
		{"unknown-type", func(value string) string { return strings.Replace(value, `"type": "object"`, `"type": "future"`, 1) }, false},
		{"unknown-format", func(value string) string { return strings.Replace(value, `"format": "uuid"`, `"format": "future"`, 1) }, false},
		{"unknown-media", func(value string) string {
			return strings.Replace(value, `"application/json"`, `"application/future"`, 1)
		}, false},
		{"unknown-extension", func(value string) string {
			return strings.Replace(value, `"type": "object"`, `"type": "object", "x-future": true`, 1)
		}, false},
		{"unknown-root-extension", func(value string) string {
			return strings.Replace(value, `"paths": {`, `"x-future-root": {"enabled": true}, "paths": {`, 1)
		}, false},
		{"unknown-composition", func(value string) string { return strings.Replace(value, `"oneOf"`, `"xorOf"`, 1) }, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, err := Parse([]byte(test.mutate(baseDocument)), ".json")
			if err != nil {
				t.Fatal(err)
			}
			inventory, buildErr := BuildInventory(document, "mutant")
			if test.buildFails {
				if buildErr == nil {
					t.Fatal("mutant unexpectedly passed feature scan")
				}
				return
			}
			if buildErr != nil {
				t.Fatal(buildErr)
			}
			if err := CheckAllowlist(&allowlist, inventory); err == nil {
				t.Fatal("mutant unexpectedly passed reviewed allowlist")
			}
		})
	}
}

func TestSilentOperationOmissionFails(t *testing.T) {
	document, inventory := buildInventory(t, baseDocument)
	inventory.OperationCount = 0
	if _, err := ScanFeatures(document, inventory); err == nil || !strings.Contains(err.Error(), "silent operation omission") {
		t.Fatalf("omission error = %v", err)
	}
}

func TestFeatureOutputStable(t *testing.T) {
	_, inventory := buildInventory(t, baseDocument)
	allowlist := ReviewedAllowlist(map[string]int{"test": 1}, inventory)
	first, err := Canonical(allowlist)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Canonical(allowlist)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("allowlist output is nondeterministic")
	}
}

func TestStrictReviewedManifestDecoding(t *testing.T) {
	valid := []byte(`{"schema_version":1,"openapi":"3.1.2","operation_counts":{"test":1},"features":[]}`)
	if _, err := DecodeAllowlist(valid); err != nil {
		t.Fatal(err)
	}
	unknown := []byte(`{"schema_version":1,"openapi":"3.1.2","operation_counts":{"test":1},"features":[],"future":true}`)
	if _, err := DecodeAllowlist(unknown); err == nil {
		t.Fatal("unknown reviewed manifest field unexpectedly passed")
	}
	if _, err := DecodeAllowlist(append(valid, []byte(` {}`)...)); err == nil {
		t.Fatal("trailing reviewed manifest value unexpectedly passed")
	}
}

func TestMalformedKnownShapesFail(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"paths-collection", func(root map[string]any) { root["paths"] = []any{} }},
		{"components-collection", func(root map[string]any) { root["components"] = []any{} }},
		{"parameters-array", func(root map[string]any) { testOperation(root)["parameters"] = map[string]any{} }},
		{"responses-object", func(root map[string]any) { testOperation(root)["responses"] = []any{} }},
		{"headers-object", func(root map[string]any) { testResponse(root)["headers"] = []any{} }},
		{"content-object", func(root map[string]any) { testResponse(root)["content"] = []any{} }},
		{"media-object", func(root map[string]any) {
			testResponse(root)["content"].(map[string]any)["application/json"] = "bad"
		}},
		{"security-array", func(root map[string]any) { testOperation(root)["security"] = map[string]any{} }},
		{"security-map", func(root map[string]any) { testOperation(root)["security"] = []any{"bad"} }},
		{"security-scopes", func(root map[string]any) {
			testOperation(root)["security"] = []any{map[string]any{"ApiKey": "bad"}}
		}},
		{"servers-array", func(root map[string]any) { root["servers"] = map[string]any{} }},
		{"server-variables", func(root map[string]any) {
			root["servers"].([]any)[0].(map[string]any)["variables"] = []any{}
		}},
		{"schema-properties", func(root map[string]any) { testWidget(root)["properties"] = []any{} }},
		{"schema-required", func(root map[string]any) { testWidget(root)["required"] = "id" }},
		{"schema-type-array", func(root map[string]any) { testWidget(root)["type"] = []any{"object", true} }},
		{"schema-format", func(root map[string]any) {
			testWidget(root)["properties"].(map[string]any)["id"].(map[string]any)["format"] = true
		}},
		{"schema-composition-array", func(root map[string]any) {
			testWidget(root)["properties"].(map[string]any)["detail"].(map[string]any)["oneOf"] = map[string]any{}
		}},
		{"schema-child-object", func(root map[string]any) {
			testOperation(root)["parameters"].([]any)[1].(map[string]any)["schema"].(map[string]any)["items"] = "bad"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, err := Parse([]byte(baseDocument), ".json")
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(document.Root)
			if _, err := BuildInventory(document, "mutant"); err == nil {
				t.Fatal("malformed known field unexpectedly passed")
			}
		})
	}
}

func buildInventory(t *testing.T, raw string) (*Document, *Inventory) {
	t.Helper()
	document, err := Parse([]byte(raw), ".json")
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := BuildInventory(document, "test")
	if err != nil {
		t.Fatal(err)
	}
	return document, inventory
}

func testOperation(root map[string]any) map[string]any {
	return root["paths"].(map[string]any)["/widgets/{id}"].(map[string]any)["get"].(map[string]any)
}

func testResponse(root map[string]any) map[string]any {
	return testOperation(root)["responses"].(map[string]any)["200"].(map[string]any)
}

func testWidget(root map[string]any) map[string]any {
	return root["components"].(map[string]any)["schemas"].(map[string]any)["Widget"].(map[string]any)
}
