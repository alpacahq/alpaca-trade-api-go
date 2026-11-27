package overlay

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestApplyDistinguishesMissingAndNull(t *testing.T) {
	root := map[string]any{"present": nil}
	add := Operation{Operation: "add", Pointer: "/missing", Value: json.RawMessage(`null`)}
	if err := Apply(root, add); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["missing"]; !ok {
		t.Fatal("add did not create an explicit null property")
	}
	if err := Apply(root, add); err == nil {
		t.Fatal("add unexpectedly replaced an existing property")
	}
	replace := Operation{
		Operation: "replace", Pointer: "/present",
		Before: json.RawMessage(`null`), Value: json.RawMessage(`"value"`),
	}
	if err := Apply(root, replace); err != nil {
		t.Fatal(err)
	}
	if root["present"] != "value" {
		t.Fatalf("replace result = %#v", root["present"])
	}
	replace.Pointer = "/absent"
	if err := Apply(root, replace); err == nil {
		t.Fatal("replace unexpectedly accepted an absent property")
	}
}

func TestDecodeRequiresOperationSpecificOperands(t *testing.T) {
	base := `{
	  "schema_version": 2,
	  "purpose": "public-compatibility-overlays",
	  "status": "accepted",
	  "operations": [%s]
	}`
	tests := []struct {
		name      string
		operation string
		wantError bool
	}{
		{
			name:      "add",
			operation: `{"target":"trading","operation":"add","pointer":"/x","value":null,"reason":"reviewed"}`,
		},
		{
			name:      "replace-null",
			operation: `{"target":"trading","operation":"replace","pointer":"/x","before":null,"value":1,"reason":"reviewed"}`,
		},
		{
			name:      "remove",
			operation: `{"target":"market-data","operation":"remove","pointer":"/x","before":1,"reason":"reviewed"}`,
		},
		{
			name:      "add-before",
			operation: `{"target":"trading","operation":"add","pointer":"/x","before":null,"value":1,"reason":"reviewed"}`,
			wantError: true,
		},
		{
			name:      "replace-missing-before",
			operation: `{"target":"trading","operation":"replace","pointer":"/x","value":1,"reason":"reviewed"}`,
			wantError: true,
		},
		{
			name:      "invalid-pointer-escape",
			operation: `{"target":"trading","operation":"add","pointer":"/x~2y","value":1,"reason":"reviewed"}`,
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode([]byte(fmt.Sprintf(base, test.operation)))
			if (err != nil) != test.wantError {
				t.Fatalf("Decode() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}
