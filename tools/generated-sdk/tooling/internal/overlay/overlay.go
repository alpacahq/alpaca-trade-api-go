package overlay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	Purpose       = "public-compatibility-overlays"
	Status        = "accepted"
	SchemaVersion = 2
)

type Operation struct {
	Target    string          `json:"target"`
	Operation string          `json:"operation"`
	Pointer   string          `json:"pointer"`
	Before    json.RawMessage `json:"before,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
	Reason    string          `json:"reason"`
}

type Manifest struct {
	SchemaVersion int         `json:"schema_version"`
	Purpose       string      `json:"purpose"`
	Status        string      `json:"status"`
	Operations    []Operation `json:"operations"`
}

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return Decode(data)
}

func Decode(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode overlay manifest: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != SchemaVersion ||
		manifest.Purpose != Purpose ||
		manifest.Status != Status ||
		len(manifest.Operations) == 0 {
		return Manifest{}, errors.New("invalid generated SDK overlay manifest")
	}
	seen := map[string]bool{}
	for index, operation := range manifest.Operations {
		if operation.Target != "trading" && operation.Target != "market-data" {
			return Manifest{}, fmt.Errorf("overlay %d has unsupported target %q", index+1, operation.Target)
		}
		if operation.Operation != "add" && operation.Operation != "replace" && operation.Operation != "remove" {
			return Manifest{}, fmt.Errorf("overlay %d has unsupported operation %q", index+1, operation.Operation)
		}
		if _, err := pointerSegments(operation.Pointer); err != nil {
			return Manifest{}, fmt.Errorf("overlay %d has invalid JSON pointer %q", index+1, operation.Pointer)
		}
		if strings.TrimSpace(operation.Reason) == "" {
			return Manifest{}, fmt.Errorf("overlay %d has no review reason", index+1)
		}
		if err := validateOperands(index, operation); err != nil {
			return Manifest{}, err
		}
		key := operation.Target + "\x00" + operation.Pointer
		if seen[key] {
			return Manifest{}, fmt.Errorf("duplicate overlay for %s %s", operation.Target, operation.Pointer)
		}
		seen[key] = true
	}
	return manifest, nil
}

func validateOperands(index int, operation Operation) error {
	hasBefore := len(operation.Before) != 0
	hasValue := len(operation.Value) != 0
	switch operation.Operation {
	case "add":
		if hasBefore || !hasValue {
			return fmt.Errorf("overlay %d add requires value and forbids before", index+1)
		}
	case "replace":
		if !hasBefore || !hasValue {
			return fmt.Errorf("overlay %d replace requires before and value", index+1)
		}
	case "remove":
		if !hasBefore || hasValue {
			return fmt.Errorf("overlay %d remove requires before and forbids value", index+1)
		}
	}
	for name, raw := range map[string]json.RawMessage{"before": operation.Before, "value": operation.Value} {
		if len(raw) == 0 {
			continue
		}
		if _, err := decodeValue(raw); err != nil {
			return fmt.Errorf("overlay %d %s: %w", index+1, name, err)
		}
	}
	return nil
}

func Apply(root map[string]any, operation Operation) error {
	segments, err := pointerSegments(operation.Pointer)
	if err != nil {
		return err
	}
	current := root
	for _, segment := range segments[:len(segments)-1] {
		value, ok := current[segment].(map[string]any)
		if !ok {
			return fmt.Errorf("%s lacks object %s", operation.Pointer, segment)
		}
		current = value
	}
	key := segments[len(segments)-1]
	actual, exists := current[key]
	switch operation.Operation {
	case "add":
		if exists {
			return fmt.Errorf("%s already exists", operation.Pointer)
		}
		value, err := decodeValue(operation.Value)
		if err != nil {
			return err
		}
		current[key] = value
	case "replace", "remove":
		if !exists {
			return fmt.Errorf("%s does not exist", operation.Pointer)
		}
		before, err := decodeValue(operation.Before)
		if err != nil {
			return err
		}
		if !equalJSON(actual, before) {
			return fmt.Errorf("%s before value is %v, want %v", operation.Pointer, actual, before)
		}
		if operation.Operation == "remove" {
			delete(current, key)
			return nil
		}
		value, err := decodeValue(operation.Value)
		if err != nil {
			return err
		}
		current[key] = value
	default:
		return fmt.Errorf("unsupported overlay operation %s", operation.Operation)
	}
	return nil
}

func pointerSegments(pointer string) ([]string, error) {
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("overlay pointer %q is invalid", pointer)
	}
	raw := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	if len(raw) == 0 || len(raw) == 1 && raw[0] == "" {
		return nil, fmt.Errorf("overlay pointer %q is empty", pointer)
	}
	for index, segment := range raw {
		for offset := 0; offset < len(segment); offset++ {
			if segment[offset] != '~' {
				continue
			}
			if offset+1 >= len(segment) || segment[offset+1] != '0' && segment[offset+1] != '1' {
				return nil, fmt.Errorf("overlay pointer %q has an invalid escape", pointer)
			}
			offset++
		}
		raw[index] = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
	}
	return raw, nil
}

func decodeValue(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := requireEOF(decoder); err != nil {
		return nil, err
	}
	return value, nil
}

func requireEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("JSON contains trailing values")
		}
		return err
	}
	return nil
}

func equalJSON(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return bytes.Equal(leftJSON, rightJSON)
}
