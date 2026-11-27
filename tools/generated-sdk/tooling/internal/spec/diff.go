package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

type Classification string

const (
	ClassificationNoChange          Classification = "no-change"
	ClassificationProvenanceOnly    Classification = "provenance-only"
	ClassificationDocumentationOnly Classification = "documentation-only"
	ClassificationAdditive          Classification = "additive"
	ClassificationManualReview      Classification = "manual-review"
	ClassificationBreaking          Classification = "breaking"
)

type Change struct {
	Classification Classification `json:"classification"`
	Kind           string         `json:"kind"`
	Subject        string         `json:"subject"`
	Detail         string         `json:"detail"`
}

type Comparison struct {
	Target         string         `json:"target"`
	Classification Classification `json:"classification"`
	Changes        []Change       `json:"changes"`
}

func Compare(target string, baselineData, proposedData []byte) (Comparison, error) {
	baseline, err := Parse(baselineData, ".json")
	if err != nil {
		return Comparison{}, fmt.Errorf("baseline: %w", err)
	}
	proposed, err := Parse(proposedData, ".json")
	if err != nil {
		return Comparison{}, fmt.Errorf("proposed: %w", err)
	}
	baselineInventory, err := BuildInventory(baseline, target)
	if err != nil {
		return Comparison{}, fmt.Errorf("baseline inventory: %w", err)
	}
	proposedInventory, err := BuildInventory(proposed, target)
	if err != nil {
		return Comparison{}, fmt.Errorf("proposed inventory: %w", err)
	}
	comparison := Comparison{Target: target, Classification: ClassificationNoChange, Changes: []Change{}}
	if bytes.Equal(baselineData, proposedData) {
		return comparison, nil
	}
	compareOperations(&comparison, baselineInventory.Operations, proposedInventory.Operations)
	compareComponentSchemas(&comparison, baseline.Root, proposed.Root)
	compareDocumentPolicy(&comparison, baselineInventory, proposedInventory)
	compareExtensionPolicy(&comparison, baselineInventory.Features, proposedInventory.Features)
	if documentationOnlyDifference(baseline.Root, proposed.Root) {
		addChange(&comparison, Change{
			Classification: ClassificationDocumentationOnly,
			Kind:           "documentation",
			Subject:        target,
			Detail:         "only documentation or specification metadata changed",
		})
	} else if len(comparison.Changes) == 0 {
		addChange(&comparison, Change{
			Classification: ClassificationManualReview,
			Kind:           "unclassified",
			Subject:        target,
			Detail:         "the document changed outside the feature-policy-bounded classifier",
		})
	}
	sort.Slice(comparison.Changes, func(i, j int) bool {
		left, right := comparison.Changes[i], comparison.Changes[j]
		return left.Kind+"\x00"+left.Subject+"\x00"+left.Detail <
			right.Kind+"\x00"+right.Subject+"\x00"+right.Detail
	})
	return comparison, nil
}

func compareOperations(comparison *Comparison, baseline, proposed []Operation) {
	oldOperations := operationMap(baseline)
	newOperations := operationMap(proposed)
	for id, current := range newOperations {
		previous, ok := oldOperations[id]
		if !ok {
			addChange(comparison, Change{Classification: ClassificationAdditive, Kind: "operation-added", Subject: id, Detail: current.Method + " " + current.Path})
			continue
		}
		if previous.Method != current.Method || previous.Path != current.Path {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "operation-moved", Subject: id, Detail: previous.Method + " " + previous.Path + " -> " + current.Method + " " + current.Path})
		}
		if !slices.Equal(previous.Tags, current.Tags) {
			addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "operation-grouping", Subject: id, Detail: "operation tags changed"})
		}
		if previous.Deprecated != current.Deprecated {
			addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "operation-deprecation", Subject: id, Detail: "operation deprecation changed"})
		}
		if previous.ServerSource != current.ServerSource {
			addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "operation-server-source", Subject: id, Detail: "operation server inheritance changed"})
		}
		compareParameters(comparison, id, previous.Parameters, current.Parameters)
		compareRequestBody(comparison, id, previous.RequestBody, current.RequestBody)
		compareResponses(comparison, id, previous.Responses, current.Responses)
		if canonicalValue(previous.Security) != canonicalValue(current.Security) {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "operation-security", Subject: id, Detail: "effective security requirements changed"})
		}
		if canonicalValue(previous.Servers) != canonicalValue(current.Servers) {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "operation-server", Subject: id, Detail: "effective server selection changed"})
		}
	}
	for id, previous := range oldOperations {
		if _, ok := newOperations[id]; !ok {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "operation-removed", Subject: id, Detail: previous.Method + " " + previous.Path})
		}
	}
}

func compareExtensionPolicy(comparison *Comparison, baseline, proposed []Feature) {
	if !slices.Equal(extensionValues(baseline), extensionValues(proposed)) {
		addChange(comparison, Change{
			Classification: ClassificationManualReview,
			Kind:           "extension-policy",
			Subject:        comparison.Target,
			Detail:         "OpenAPI extensions changed",
		})
	}
}

func extensionValues(features []Feature) []string {
	values := []string{}
	for _, feature := range features {
		if feature.Category == "extension" {
			values = append(values, feature.Value)
		}
	}
	slices.Sort(values)
	return slices.Compact(values)
}

func compareDocumentPolicy(comparison *Comparison, baseline, proposed *Inventory) {
	if baseline.OpenAPI != proposed.OpenAPI {
		addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "openapi-version", Subject: comparison.Target, Detail: baseline.OpenAPI + " -> " + proposed.OpenAPI})
	}
	if baseline.InfoVersion != proposed.InfoVersion {
		addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "api-version", Subject: comparison.Target, Detail: baseline.InfoVersion + " -> " + proposed.InfoVersion})
	}
	if canonicalValue(baseline.Servers) != canonicalValue(proposed.Servers) {
		addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "global-servers", Subject: comparison.Target, Detail: "global server policy changed"})
	}
	if canonicalValue(baseline.SecuritySchemes) != canonicalValue(proposed.SecuritySchemes) {
		addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "security-schemes", Subject: comparison.Target, Detail: "security scheme type, location, bearer metadata, OpenID metadata, or OAuth flows changed"})
	}
}

func compareParameters(comparison *Comparison, operationID string, baseline, proposed []Parameter) {
	oldParameters := parameterMap(baseline)
	newParameters := parameterMap(proposed)
	for key, current := range newParameters {
		previous, ok := oldParameters[key]
		if !ok {
			classification := ClassificationAdditive
			if current.Required {
				classification = ClassificationBreaking
			}
			addChange(comparison, Change{Classification: classification, Kind: "parameter-added", Subject: operationID + ":" + key, Detail: requiredDetail(current.Required)})
			continue
		}
		if canonicalValue(previous) != canonicalValue(current) {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "parameter-changed", Subject: operationID + ":" + key, Detail: "serialization, requiredness, or schema changed"})
		}
	}
	for key := range oldParameters {
		if _, ok := newParameters[key]; !ok {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "parameter-removed", Subject: operationID + ":" + key, Detail: "parameter was removed"})
		}
	}
}

func compareRequestBody(comparison *Comparison, operationID string, baseline, proposed *RequestBody) {
	switch {
	case baseline == nil && proposed == nil:
		return
	case baseline == nil:
		classification := ClassificationAdditive
		if proposed.Required {
			classification = ClassificationBreaking
		}
		addChange(comparison, Change{Classification: classification, Kind: "request-body-added", Subject: operationID, Detail: requiredDetail(proposed.Required)})
	case proposed == nil:
		addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "request-body-removed", Subject: operationID, Detail: "request body was removed"})
	case canonicalValue(baseline) != canonicalValue(proposed):
		addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "request-body-changed", Subject: operationID, Detail: "requiredness, media type, or schema changed"})
	}
}

func compareResponses(comparison *Comparison, operationID string, baseline, proposed []Response) {
	oldResponses := responseMap(baseline)
	newResponses := responseMap(proposed)
	for status, current := range newResponses {
		previous, ok := oldResponses[status]
		if !ok {
			addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "response-added", Subject: operationID + ":" + status, Detail: "new response handling can alter generated result types"})
			continue
		}
		if canonicalValue(previous) != canonicalValue(current) {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "response-changed", Subject: operationID + ":" + status, Detail: "response media type, headers, or schema changed"})
		}
	}
	for status := range oldResponses {
		if _, ok := newResponses[status]; !ok {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "response-removed", Subject: operationID + ":" + status, Detail: "response was removed"})
		}
	}
}

func compareComponentSchemas(comparison *Comparison, baseline, proposed map[string]any) {
	oldSchemas := componentSchemas(baseline)
	newSchemas := componentSchemas(proposed)
	for name, current := range newSchemas {
		previous, ok := oldSchemas[name]
		if !ok {
			addChange(comparison, Change{Classification: ClassificationAdditive, Kind: "schema-added", Subject: name, Detail: "component schema was added"})
			continue
		}
		compareSchema(comparison, name, previous, current)
	}
	for name := range oldSchemas {
		if _, ok := newSchemas[name]; !ok {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "schema-removed", Subject: name, Detail: "component schema was removed"})
		}
	}
}

func compareSchema(comparison *Comparison, name string, baseline, proposed any) {
	if canonicalValue(stripDocumentation(baseline)) == canonicalValue(stripDocumentation(proposed)) {
		if canonicalValue(baseline) != canonicalValue(proposed) {
			addChange(comparison, Change{Classification: ClassificationDocumentationOnly, Kind: "schema-documentation", Subject: name, Detail: "schema documentation changed"})
		}
		return
	}
	oldSchema, oldOK := baseline.(map[string]any)
	newSchema, newOK := proposed.(map[string]any)
	if !oldOK || !newOK {
		addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "schema-changed", Subject: name, Detail: "schema shape requires manual review"})
		return
	}
	oldRequired := stringSet(oldSchema["required"])
	newRequired := stringSet(newSchema["required"])
	for property := range newRequired {
		if !oldRequired[property] {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "required-property-added", Subject: name + "." + property, Detail: "property became required"})
		}
	}
	for property := range oldRequired {
		if !newRequired[property] {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "required-property-removed", Subject: name + "." + property, Detail: "requiredness changed generated pointer/value behavior"})
		}
	}
	oldProperties, _ := oldSchema["properties"].(map[string]any)
	newProperties, _ := newSchema["properties"].(map[string]any)
	for property, current := range newProperties {
		previous, ok := oldProperties[property]
		if !ok {
			classification := ClassificationAdditive
			if newRequired[property] {
				classification = ClassificationBreaking
			}
			addChange(comparison, Change{Classification: classification, Kind: "property-added", Subject: name + "." + property, Detail: requiredDetail(newRequired[property])})
			continue
		}
		if canonicalValue(stripDocumentation(previous)) == canonicalValue(stripDocumentation(current)) {
			continue
		}
		if classifyEnumChange(comparison, name+"."+property, previous, current) {
			continue
		}
		addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "property-changed", Subject: name + "." + property, Detail: "property type, format, constraints, composition, or reference changed"})
	}
	for property := range oldProperties {
		if _, ok := newProperties[property]; !ok {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "property-removed", Subject: name + "." + property, Detail: "property was removed"})
		}
	}
	oldRest := cloneWithout(oldSchema, "description", "summary", "examples", "example", "required", "properties")
	newRest := cloneWithout(newSchema, "description", "summary", "examples", "example", "required", "properties")
	if canonicalValue(oldRest) != canonicalValue(newRest) {
		addChange(comparison, Change{Classification: ClassificationManualReview, Kind: "schema-keyword-changed", Subject: name, Detail: "a non-property schema keyword changed"})
	}
}

func classifyEnumChange(comparison *Comparison, subject string, baseline, proposed any) bool {
	oldSchema, oldOK := baseline.(map[string]any)
	newSchema, newOK := proposed.(map[string]any)
	if !oldOK || !newOK {
		return false
	}
	oldRest := cloneWithout(oldSchema, "description", "summary", "examples", "example", "enum")
	newRest := cloneWithout(newSchema, "description", "summary", "examples", "example", "enum")
	if canonicalValue(oldRest) != canonicalValue(newRest) {
		return false
	}
	oldEnum, oldOK := oldSchema["enum"].([]any)
	newEnum, newOK := newSchema["enum"].([]any)
	if !oldOK || !newOK {
		return false
	}
	oldValues := valueSet(oldEnum)
	newValues := valueSet(newEnum)
	for value := range oldValues {
		if !newValues[value] {
			addChange(comparison, Change{Classification: ClassificationBreaking, Kind: "enum-value-removed", Subject: subject, Detail: value})
		}
	}
	for value := range newValues {
		if !oldValues[value] {
			addChange(comparison, Change{Classification: ClassificationAdditive, Kind: "enum-value-added", Subject: subject, Detail: value})
		}
	}
	return true
}

func documentationOnlyDifference(baseline, proposed map[string]any) bool {
	return canonicalValue(stripDocumentation(baseline)) == canonicalValue(stripDocumentation(proposed))
}

func stripDocumentation(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			switch key {
			case "description", "summary", "example", "examples", "externalDocs":
				continue
			default:
				result[key] = stripDocumentation(child)
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = stripDocumentation(child)
		}
		return result
	default:
		return value
	}
}

func addChange(comparison *Comparison, change Change) {
	comparison.Changes = append(comparison.Changes, change)
	if classificationRank(change.Classification) > classificationRank(comparison.Classification) {
		comparison.Classification = change.Classification
	}
}

func classificationRank(classification Classification) int {
	switch classification {
	case ClassificationNoChange:
		return 0
	case ClassificationProvenanceOnly:
		return 1
	case ClassificationDocumentationOnly:
		return 2
	case ClassificationAdditive:
		return 3
	case ClassificationManualReview:
		return 4
	case ClassificationBreaking:
		return 5
	default:
		return 6
	}
}

func operationMap(operations []Operation) map[string]Operation {
	result := make(map[string]Operation, len(operations))
	for _, operation := range operations {
		result[operation.OperationID] = operation
	}
	return result
}

func parameterMap(parameters []Parameter) map[string]Parameter {
	result := make(map[string]Parameter, len(parameters))
	for _, parameter := range parameters {
		result[parameter.In+":"+parameter.Name] = parameter
	}
	return result
}

func responseMap(responses []Response) map[string]Response {
	result := make(map[string]Response, len(responses))
	for _, response := range responses {
		result[response.Status] = response
	}
	return result
}

func componentSchemas(root map[string]any) map[string]any {
	components, _ := root["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if schemas == nil {
		return map[string]any{}
	}
	return schemas
}

func stringSet(value any) map[string]bool {
	result := map[string]bool{}
	values, _ := value.([]any)
	for _, item := range values {
		if text, ok := item.(string); ok {
			result[text] = true
		}
	}
	return result
}

func valueSet(values []any) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[canonicalValue(value)] = true
	}
	return result
}

func cloneWithout(source map[string]any, keys ...string) map[string]any {
	excluded := make(map[string]bool, len(keys))
	for _, key := range keys {
		excluded[key] = true
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		if !excluded[key] {
			result[key] = value
		}
	}
	return result
}

func canonicalValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%#v", value)
	}
	return string(data)
}

func requiredDetail(required bool) string {
	if required {
		return "required"
	}
	return "optional"
}

func Classifications(comparisons ...Comparison) []Classification {
	result := make([]Classification, 0, len(comparisons))
	for _, comparison := range comparisons {
		if !slices.Contains(result, comparison.Classification) {
			result = append(result, comparison.Classification)
		}
	}
	slices.Sort(result)
	return result
}

func Summary(comparison Comparison) string {
	var parts []string
	for _, change := range comparison.Changes {
		parts = append(parts, string(change.Classification)+":"+change.Kind+":"+change.Subject)
	}
	return strings.Join(parts, "\n")
}
