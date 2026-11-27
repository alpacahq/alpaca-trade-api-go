package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const OpenAPIVersion = "3.1.2"

var (
	httpMethods = map[string]bool{
		"delete": true, "get": true, "head": true, "options": true,
		"patch": true, "post": true, "put": true, "trace": true,
	}
	pathItemKeys = map[string]bool{
		"$ref": true, "summary": true, "description": true, "servers": true, "parameters": true,
	}
	numberRE = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
)

type Document struct {
	Root map[string]any
}

type Inventory struct {
	SchemaVersion        int                       `json:"schema_version"`
	Source               string                    `json:"source"`
	OpenAPI              string                    `json:"openapi"`
	InfoVersion          string                    `json:"info_version"`
	Servers              []Server                  `json:"servers"`
	OperationCount       int                       `json:"operation_count"`
	ReachableSchemaCount int                       `json:"reachable_schema_count"`
	Operations           []Operation               `json:"operations"`
	SecuritySchemes      map[string]SecurityScheme `json:"security_schemes"`
	ReachableSchemas     []SchemaIdentity          `json:"reachable_schemas"`
	Features             []Feature                 `json:"features"`
}

type Operation struct {
	Method           string       `json:"method"`
	Path             string       `json:"path"`
	OperationID      string       `json:"operation_id"`
	Tags             []string     `json:"tags"`
	Deprecated       bool         `json:"deprecated"`
	ServerSource     string       `json:"server_source"`
	Servers          []Server     `json:"servers"`
	Parameters       []Parameter  `json:"parameters"`
	RequestBody      *RequestBody `json:"request_body,omitempty"`
	Responses        []Response   `json:"responses"`
	Security         any          `json:"security"`
	ReachableSchemas []string     `json:"reachable_schemas"`
}

type Parameter struct {
	Name            string        `json:"name"`
	In              string        `json:"in"`
	Style           string        `json:"style"`
	Explode         bool          `json:"explode"`
	Required        bool          `json:"required"`
	Deprecated      bool          `json:"deprecated"`
	AllowEmptyValue bool          `json:"allow_empty_value"`
	AllowReserved   bool          `json:"allow_reserved"`
	Schema          SchemaSummary `json:"schema"`
}

type RequestBody struct {
	Required bool          `json:"required"`
	Content  []MediaSchema `json:"content"`
}

type MediaSchema struct {
	MediaType string        `json:"media_type"`
	Schema    SchemaSummary `json:"schema"`
}

type Response struct {
	Status  string        `json:"status"`
	Content []MediaSchema `json:"content"`
	Headers []Header      `json:"headers"`
}

type Header struct {
	Name       string        `json:"name"`
	Required   bool          `json:"required"`
	Deprecated bool          `json:"deprecated"`
	Style      string        `json:"style"`
	Explode    bool          `json:"explode"`
	Schema     SchemaSummary `json:"schema"`
}

type Server struct {
	URL       string         `json:"url"`
	Variables map[string]any `json:"variables"`
}

type SchemaSummary struct {
	Identity string `json:"identity"`
	Digest   string `json:"digest"`
	Schema   any    `json:"schema"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	In           string `json:"in,omitempty"`
	Name         string `json:"name,omitempty"`
	BearerFormat string `json:"bearer_format,omitempty"`
	OpenIDURL    string `json:"open_id_connect_url,omitempty"`
	Flows        any    `json:"flows,omitempty"`
}

type SchemaIdentity struct {
	Identity             string   `json:"identity"`
	Digest               string   `json:"digest"`
	Operations           []string `json:"operations"`
	Types                []string `json:"types"`
	Formats              []string `json:"formats"`
	Composition          []string `json:"composition"`
	Nullable             bool     `json:"nullable"`
	HasDefault           bool     `json:"has_default"`
	HasEnum              bool     `json:"has_enum"`
	HasDiscriminator     bool     `json:"has_discriminator"`
	AdditionalProperties string   `json:"additional_properties"`
	Schema               any      `json:"schema"`
}

type Feature struct {
	Category   string   `json:"category"`
	Value      string   `json:"value"`
	Operations []string `json:"operations"`
	Policy     string   `json:"policy"`
}

type Allowlist struct {
	SchemaVersion   int            `json:"schema_version"`
	OpenAPI         string         `json:"openapi"`
	OperationCounts map[string]int `json:"operation_counts"`
	Features        []Feature      `json:"features"`
}

func Parse(data []byte, extension string) (*Document, error) {
	var value any
	if normalized := strings.ToLower(extension); normalized != ".json" && normalized != "json" {
		return nil, fmt.Errorf("unsupported OpenAPI input extension %q", extension)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if err := requireEOF(dec); err != nil {
		return nil, err
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("OpenAPI root must be an object")
	}
	return &Document{Root: root}, nil
}

func requireEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return fmt.Errorf("parse trailing JSON: %w", err)
	}
	return nil
}

func Canonical(value any) ([]byte, error) {
	var out bytes.Buffer
	if err := writeCanonical(&out, value, 0); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func writeCanonical(out *bytes.Buffer, value any, depth int) error {
	switch v := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case string:
		encoded, _ := json.Marshal(v)
		out.Write(encoded)
	case json.Number:
		if !numberRE.MatchString(v.String()) {
			return fmt.Errorf("invalid JSON number %q", v)
		}
		out.WriteString(v.String())
	case float64:
		out.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
	case int:
		out.WriteString(strconv.Itoa(v))
	case []string:
		items := make([]any, len(v))
		for i := range v {
			items[i] = v[i]
		}
		return writeCanonical(out, items, depth)
	case []any:
		if len(v) == 0 {
			out.WriteString("[]")
			return nil
		}
		out.WriteString("[\n")
		for i, item := range v {
			indent(out, depth+1)
			if err := writeCanonical(out, item, depth+1); err != nil {
				return err
			}
			if i != len(v)-1 {
				out.WriteByte(',')
			}
			out.WriteByte('\n')
		}
		indent(out, depth)
		out.WriteByte(']')
	case map[string]any:
		if len(v) == 0 {
			out.WriteString("{}")
			return nil
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteString("{\n")
		for i, key := range keys {
			indent(out, depth+1)
			encoded, _ := json.Marshal(key)
			out.Write(encoded)
			out.WriteString(": ")
			if err := writeCanonical(out, v[key], depth+1); err != nil {
				return err
			}
			if i != len(keys)-1 {
				out.WriteByte(',')
			}
			out.WriteByte('\n')
		}
		indent(out, depth)
		out.WriteByte('}')
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal %T: %w", value, err)
		}
		var normalized any
		dec := json.NewDecoder(bytes.NewReader(encoded))
		dec.UseNumber()
		if err := dec.Decode(&normalized); err != nil {
			return err
		}
		return writeCanonical(out, normalized, depth)
	}
	return nil
}

func indent(out *bytes.Buffer, depth int) {
	out.WriteString(strings.Repeat("  ", depth))
}

func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func DigestValue(value any) (string, error) {
	data, err := Canonical(value)
	if err != nil {
		return "", err
	}
	return Digest(data), nil
}

func BuildInventory(doc *Document, source string) (*Inventory, error) {
	openapi, _ := doc.Root["openapi"].(string)
	infoVersion := stringAt(doc.Root, "info", "version")
	inv := &Inventory{
		SchemaVersion:   1,
		Source:          source,
		OpenAPI:         openapi,
		InfoVersion:     infoVersion,
		SecuritySchemes: map[string]SecurityScheme{},
	}
	inv.Servers, _ = serversFromRaw(doc.Root["servers"])
	if schemes, ok := objectAt(doc.Root, "components", "securitySchemes"); ok {
		for name, raw := range schemes {
			resolved, _, err := doc.resolveObject(raw)
			if err != nil {
				return nil, fmt.Errorf("security scheme %s: %w", name, err)
			}
			inv.SecuritySchemes[name] = SecurityScheme{
				Type:         stringValue(resolved["type"]),
				Scheme:       stringValue(resolved["scheme"]),
				In:           stringValue(resolved["in"]),
				Name:         stringValue(resolved["name"]),
				BearerFormat: stringValue(resolved["bearerFormat"]),
				OpenIDURL:    stringValue(resolved["openIdConnectUrl"]),
				Flows:        clone(resolved["flows"]),
			}
		}
	}
	paths, _ := doc.Root["paths"].(map[string]any)
	reachable := map[string]*schemaUse{}
	for _, path := range sortedKeys(paths) {
		pathRaw := paths[path]
		pathItem, ok := pathRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %s is not an object", path)
		}
		for _, method := range sortedKeys(pathItem) {
			operationRaw := pathItem[method]
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			operationMap, ok := operationRaw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s %s operation is not an object", method, path)
			}
			op, err := doc.buildOperation(strings.ToUpper(method), path, pathItem, operationMap, reachable)
			if err != nil {
				return nil, err
			}
			inv.Operations = append(inv.Operations, op)
		}
	}
	sort.Slice(inv.Operations, func(i, j int) bool {
		if inv.Operations[i].Path != inv.Operations[j].Path {
			return inv.Operations[i].Path < inv.Operations[j].Path
		}
		return inv.Operations[i].Method < inv.Operations[j].Method
	})
	operationIDs := map[string]bool{}
	for _, operation := range inv.Operations {
		if operation.OperationID == "" {
			return nil, fmt.Errorf("%s %s has no operationId", operation.Method, operation.Path)
		}
		if operationIDs[operation.OperationID] {
			return nil, fmt.Errorf("duplicate operationId %q", operation.OperationID)
		}
		operationIDs[operation.OperationID] = true
	}
	inv.OperationCount = len(inv.Operations)
	for identity, use := range reachable {
		summary, err := summarizeSchema(identity, use.schema, use.operations)
		if err != nil {
			return nil, err
		}
		inv.ReachableSchemas = append(inv.ReachableSchemas, summary)
	}
	sort.Slice(inv.ReachableSchemas, func(i, j int) bool {
		return inv.ReachableSchemas[i].Identity < inv.ReachableSchemas[j].Identity
	})
	inv.ReachableSchemaCount = len(inv.ReachableSchemas)
	features, err := ScanFeatures(doc, inv)
	if err != nil {
		return nil, err
	}
	inv.Features = features
	return inv, nil
}

type schemaUse struct {
	schema     map[string]any
	operations map[string]bool
}

func (doc *Document) buildOperation(method, path string, pathItem, raw map[string]any, reachable map[string]*schemaUse) (Operation, error) {
	op := Operation{
		Method:      method,
		Path:        path,
		OperationID: stringValue(raw["operationId"]),
		Tags:        stringSlice(raw["tags"]),
		Deprecated:  boolValue(raw["deprecated"], false),
		Security:    clone(raw["security"]),
	}
	op.ServerSource, op.Servers = doc.effectiveServers(pathItem, raw)
	if op.Security == nil {
		op.Security = clone(doc.Root["security"])
	}
	if op.Security == nil {
		op.Security = []any{}
	}
	sort.Strings(op.Tags)
	operationKey := op.OperationID
	if operationKey == "" {
		operationKey = method + " " + path
	}
	allParameters := append(anySlice(pathItem["parameters"]), anySlice(raw["parameters"])...)
	for _, parameterRaw := range allParameters {
		parameter, _, err := doc.resolveObject(parameterRaw)
		if err != nil {
			return op, fmt.Errorf("%s parameter: %w", operationKey, err)
		}
		style := stringValue(parameter["style"])
		location := stringValue(parameter["in"])
		if style == "" {
			switch location {
			case "query", "cookie":
				style = "form"
			case "path", "header":
				style = "simple"
			}
		}
		explode := boolValue(parameter["explode"], style == "form")
		summary, err := doc.schemaSummary(parameter["schema"], operationKey, reachable)
		if err != nil {
			return op, fmt.Errorf("%s parameter %s: %w", operationKey, stringValue(parameter["name"]), err)
		}
		op.Parameters = append(op.Parameters, Parameter{
			Name:            stringValue(parameter["name"]),
			In:              location,
			Style:           style,
			Explode:         explode,
			Required:        boolValue(parameter["required"], false),
			Deprecated:      boolValue(parameter["deprecated"], false),
			AllowEmptyValue: boolValue(parameter["allowEmptyValue"], false),
			AllowReserved:   boolValue(parameter["allowReserved"], false),
			Schema:          summary,
		})
	}
	sort.Slice(op.Parameters, func(i, j int) bool {
		if op.Parameters[i].In != op.Parameters[j].In {
			return op.Parameters[i].In < op.Parameters[j].In
		}
		return op.Parameters[i].Name < op.Parameters[j].Name
	})
	if bodyRaw, ok := raw["requestBody"]; ok {
		body, _, err := doc.resolveObject(bodyRaw)
		if err != nil {
			return op, fmt.Errorf("%s request body: %w", operationKey, err)
		}
		requestBody := &RequestBody{Required: boolValue(body["required"], false)}
		requestBody.Content, err = doc.mediaSchemas(body["content"], operationKey, reachable)
		if err != nil {
			return op, err
		}
		op.RequestBody = requestBody
	}
	responses, _ := raw["responses"].(map[string]any)
	for status, responseRaw := range responses {
		responseMap, _, err := doc.resolveObject(responseRaw)
		if err != nil {
			return op, fmt.Errorf("%s response %s: %w", operationKey, status, err)
		}
		if links, exists := responseMap["links"]; exists && !isEmpty(links) {
			return op, fmt.Errorf("%s response %s uses unsupported links", operationKey, status)
		}
		response := Response{Status: status}
		response.Content, err = doc.mediaSchemas(responseMap["content"], operationKey, reachable)
		if err != nil {
			return op, err
		}
		if headers, ok := responseMap["headers"].(map[string]any); ok {
			for name, headerRaw := range headers {
				headerMap, _, err := doc.resolveObject(headerRaw)
				if err != nil {
					return op, fmt.Errorf("%s response %s header %s: %w", operationKey, status, name, err)
				}
				summary, err := doc.schemaSummary(headerMap["schema"], operationKey, reachable)
				if err != nil {
					return op, err
				}
				style := stringValue(headerMap["style"])
				if style == "" {
					style = "simple"
				}
				response.Headers = append(response.Headers, Header{
					Name:       name,
					Required:   boolValue(headerMap["required"], false),
					Deprecated: boolValue(headerMap["deprecated"], false),
					Style:      style,
					Explode:    boolValue(headerMap["explode"], false),
					Schema:     summary,
				})
			}
			sort.Slice(response.Headers, func(i, j int) bool { return response.Headers[i].Name < response.Headers[j].Name })
		}
		op.Responses = append(op.Responses, response)
	}
	sort.Slice(op.Responses, func(i, j int) bool { return op.Responses[i].Status < op.Responses[j].Status })
	for identity, use := range reachable {
		if use.operations[operationKey] {
			op.ReachableSchemas = append(op.ReachableSchemas, identity)
		}
	}
	sort.Strings(op.ReachableSchemas)
	return op, nil
}

func (doc *Document) mediaSchemas(raw any, operation string, reachable map[string]*schemaUse) ([]MediaSchema, error) {
	content, _ := raw.(map[string]any)
	result := make([]MediaSchema, 0, len(content))
	for media, mediaRaw := range content {
		mediaMap, ok := mediaRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s media %s is not an object", operation, media)
		}
		summary, err := doc.schemaSummary(mediaMap["schema"], operation, reachable)
		if err != nil {
			return nil, err
		}
		result = append(result, MediaSchema{MediaType: media, Schema: summary})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].MediaType < result[j].MediaType })
	return result, nil
}

func (doc *Document) schemaSummary(raw any, operation string, reachable map[string]*schemaUse) (SchemaSummary, error) {
	if raw == nil {
		return SchemaSummary{Identity: "none", Digest: Digest([]byte("null\n")), Schema: nil}, nil
	}
	schema, ok := raw.(map[string]any)
	if !ok {
		return SchemaSummary{}, fmt.Errorf("%s schema is not an object", operation)
	}
	identity := "inline:" + operation + ":" + shortDigest(schema)
	if ref, ok := schema["$ref"].(string); ok {
		identity = ref
	}
	if err := doc.walkSchema(schema, identity, operation, reachable, map[string]bool{}); err != nil {
		return SchemaSummary{}, err
	}
	digest, err := DigestValue(schema)
	if err != nil {
		return SchemaSummary{}, err
	}
	return SchemaSummary{Identity: identity, Digest: digest, Schema: clone(schema)}, nil
}

func (doc *Document) walkSchema(schema map[string]any, identity, operation string, reachable map[string]*schemaUse, stack map[string]bool) error {
	if ref, ok := schema["$ref"].(string); ok {
		if !strings.HasPrefix(ref, "#/") {
			return fmt.Errorf("%s uses unsupported external schema reference %q", operation, ref)
		}
		resolved, err := doc.resolvePointer(ref)
		if err != nil {
			return err
		}
		resolvedSchema, ok := resolved.(map[string]any)
		if !ok {
			return fmt.Errorf("schema reference %q does not resolve to an object", ref)
		}
		use := reachable[ref]
		if use == nil {
			use = &schemaUse{schema: resolvedSchema, operations: map[string]bool{}}
			reachable[ref] = use
		}
		use.operations[operation] = true
		if stack[ref] {
			return nil
		}
		nextStack := cloneBoolMap(stack)
		nextStack[ref] = true
		return doc.walkSchemaChildren(resolvedSchema, ref, operation, reachable, nextStack)
	}
	inlineIdentity := identity
	use := reachable[inlineIdentity]
	if use == nil {
		use = &schemaUse{schema: schema, operations: map[string]bool{}}
		reachable[inlineIdentity] = use
	}
	use.operations[operation] = true
	return doc.walkSchemaChildren(schema, inlineIdentity, operation, reachable, stack)
}

func (doc *Document) walkSchemaChildren(schema map[string]any, identity, operation string, reachable map[string]*schemaUse, stack map[string]bool) error {
	for _, keyword := range []string{"items", "contains", "not", "propertyNames", "unevaluatedItems", "unevaluatedProperties"} {
		if child, ok := schema[keyword].(map[string]any); ok {
			if err := doc.walkSchema(child, identity+"/"+keyword, operation, reachable, stack); err != nil {
				return err
			}
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		for i, childRaw := range anySlice(schema[keyword]) {
			child, ok := childRaw.(map[string]any)
			if !ok {
				return fmt.Errorf("%s %s item %d is not a schema", operation, keyword, i)
			}
			if err := doc.walkSchema(child, fmt.Sprintf("%s/%s/%d", identity, keyword, i), operation, reachable, stack); err != nil {
				return err
			}
		}
	}
	for _, keyword := range []string{"properties", "patternProperties", "$defs", "dependentSchemas"} {
		children, _ := schema[keyword].(map[string]any)
		names := sortedKeys(children)
		for _, name := range names {
			child, ok := children[name].(map[string]any)
			if !ok {
				return fmt.Errorf("%s %s.%s is not a schema", operation, keyword, name)
			}
			if err := doc.walkSchema(child, identity+"/"+keyword+"/"+name, operation, reachable, stack); err != nil {
				return err
			}
		}
	}
	if child, ok := schema["additionalProperties"].(map[string]any); ok {
		if err := doc.walkSchema(child, identity+"/additionalProperties", operation, reachable, stack); err != nil {
			return err
		}
	}
	return nil
}

func summarizeSchema(identity string, schema map[string]any, operations map[string]bool) (SchemaIdentity, error) {
	digest, err := DigestValue(schema)
	if err != nil {
		return SchemaIdentity{}, err
	}
	summary := SchemaIdentity{
		Identity:             identity,
		Digest:               digest,
		Operations:           sortedBoolKeys(operations),
		Types:                schemaTypes(schema["type"]),
		Formats:              valueStrings(schema["format"]),
		Nullable:             isNullable(schema),
		HasDefault:           hasKey(schema, "default"),
		HasEnum:              hasKey(schema, "enum"),
		HasDiscriminator:     hasKey(schema, "discriminator"),
		AdditionalProperties: additionalPropertiesKind(schema["additionalProperties"]),
		Schema:               clone(schema),
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		if _, ok := schema[keyword]; ok {
			summary.Composition = append(summary.Composition, keyword)
		}
	}
	return summary, nil
}

func ScanFeatures(doc *Document, inv *Inventory) ([]Feature, error) {
	if inv.OpenAPI != OpenAPIVersion {
		return nil, fmt.Errorf("unsupported OpenAPI version %q, want %s", inv.OpenAPI, OpenAPIVersion)
	}
	if webhooks, ok := doc.Root["webhooks"].(map[string]any); ok && len(webhooks) != 0 {
		return nil, errors.New("webhooks are unsupported")
	}
	acc := map[string]map[string]bool{}
	add := func(category, value string, operations ...string) {
		key := category + "\x00" + value
		if acc[key] == nil {
			acc[key] = map[string]bool{}
		}
		for _, operation := range operations {
			if operation != "" {
				acc[key][operation] = true
			}
		}
	}
	if err := validateDocumentStructure(doc, inv, add); err != nil {
		return nil, err
	}
	paths, _ := doc.Root["paths"].(map[string]any)
	sourceOperations := 0
	for path, pathRaw := range paths {
		pathItem, ok := pathRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %s is not an object", path)
		}
		for key, raw := range pathItem {
			lower := strings.ToLower(key)
			if httpMethods[lower] {
				sourceOperations++
				op, ok := raw.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("%s %s operation is not an object", key, path)
				}
				operation := stringValue(op["operationId"])
				if operation == "" {
					return nil, fmt.Errorf("%s %s has no operationId", strings.ToUpper(key), path)
				}
				add("http-method", strings.ToUpper(key), operation)
				for _, forbidden := range []string{"callbacks", "additionalOperations"} {
					if value, exists := op[forbidden]; exists && !isEmpty(value) {
						return nil, fmt.Errorf("%s uses unsupported operation feature %s", operation, forbidden)
					}
				}
			} else if !pathItemKeys[key] {
				return nil, fmt.Errorf("unknown HTTP method or path-item feature %q at %s", key, path)
			}
		}
	}
	if sourceOperations != inv.OperationCount {
		return nil, fmt.Errorf("silent operation omission: source has %d, inventory has %d", sourceOperations, inv.OperationCount)
	}
	for _, op := range inv.Operations {
		for _, parameter := range op.Parameters {
			if parameter.In == "querystring" {
				return nil, fmt.Errorf("%s uses unsupported querystring parameter %s", op.OperationID, parameter.Name)
			}
			add("parameter-location", parameter.In, op.OperationID)
			add("parameter-style", parameter.In+":"+parameter.Style+":"+strconv.FormatBool(parameter.Explode), op.OperationID)
		}
		if op.RequestBody != nil {
			for _, media := range op.RequestBody.Content {
				add("media-type", media.MediaType, op.OperationID)
			}
		}
		for _, response := range op.Responses {
			for _, media := range response.Content {
				add("media-type", media.MediaType, op.OperationID)
			}
		}
		for _, requirement := range securityRequirementNames(op.Security) {
			add("security-requirement", requirement, op.OperationID)
		}
		for _, combination := range securityRequirementCombinations(op.Security) {
			add("security-requirement-combination", combination, op.OperationID)
		}
	}
	for name, scheme := range inv.SecuritySchemes {
		operations := []string{}
		for _, op := range inv.Operations {
			if slices.Contains(securityRequirementNames(op.Security), name) {
				operations = append(operations, op.OperationID)
			}
		}
		encodedScheme, err := json.Marshal(scheme)
		if err != nil {
			return nil, fmt.Errorf("encode security scheme %s: %w", name, err)
		}
		add("security-scheme", name+":"+string(encodedScheme), operations...)
	}
	for _, schema := range inv.ReachableSchemas {
		for _, typ := range schema.Types {
			add("schema-type", typ, schema.Operations...)
		}
		for _, format := range schema.Formats {
			add("schema-format", format, schema.Operations...)
		}
		schemaObject, _ := schema.Schema.(map[string]any)
		for keyword := range schemaObject {
			if isSchemaKeyword(keyword) {
				if strings.HasPrefix(keyword, "x-") {
					add("extension", extensionFeatureValue("schema", keyword, schemaObject[keyword]), schema.Operations...)
				} else {
					add("schema-keyword", keyword, schema.Operations...)
				}
				continue
			}
			if keyword == "itemSchema" {
				return nil, fmt.Errorf("%s uses unsupported OAS 3.2 itemSchema", schema.Identity)
			}
			return nil, fmt.Errorf("%s uses unknown schema keyword %q", schema.Identity, keyword)
		}
	}
	features := make([]Feature, 0, len(acc))
	for key, operations := range acc {
		parts := strings.SplitN(key, "\x00", 2)
		features = append(features, Feature{
			Category:   parts[0],
			Value:      parts[1],
			Operations: sortedBoolKeys(operations),
			Policy:     featurePolicy(parts[0], parts[1]),
		})
	}
	sort.Slice(features, func(i, j int) bool {
		if features[i].Category != features[j].Category {
			return features[i].Category < features[j].Category
		}
		return features[i].Value < features[j].Value
	})
	return features, nil
}

func CheckAllowlist(allowlist *Allowlist, inventories ...*Inventory) error {
	if allowlist.SchemaVersion != 1 {
		return fmt.Errorf("unsupported allowlist schema version %d", allowlist.SchemaVersion)
	}
	if allowlist.OpenAPI != OpenAPIVersion {
		return fmt.Errorf("allowlist OpenAPI version is %q, want %s", allowlist.OpenAPI, OpenAPIVersion)
	}
	actualCounts := map[string]int{}
	for _, inventory := range inventories {
		actualCounts[inventory.Source] = inventory.OperationCount
	}
	if !valuesEqual(actualCounts, allowlist.OperationCounts) {
		return fmt.Errorf("allowlist operation count drift: got %v, want %v", actualCounts, allowlist.OperationCounts)
	}
	expected := map[string]Feature{}
	for _, feature := range allowlist.Features {
		key := feature.Category + "\x00" + feature.Value
		if _, exists := expected[key]; exists {
			return fmt.Errorf("duplicate allowlist feature %s/%s", feature.Category, feature.Value)
		}
		expected[key] = feature
	}
	var errs []string
	actualFeatures := mergeFeatures(inventories...)
	for _, actual := range actualFeatures {
		key := actual.Category + "\x00" + actual.Value
		allowed, ok := expected[key]
		if !ok {
			errs = append(errs, fmt.Sprintf("unreviewed feature %s/%s", actual.Category, actual.Value))
			continue
		}
		if !slices.Equal(actual.Operations, allowed.Operations) {
			errs = append(errs, fmt.Sprintf("feature operation drift %s/%s", actual.Category, actual.Value))
		}
		if actual.Policy != allowed.Policy {
			errs = append(errs, fmt.Sprintf("feature policy drift %s/%s", actual.Category, actual.Value))
		}
		delete(expected, key)
	}
	for _, feature := range expected {
		errs = append(errs, fmt.Sprintf("allowlisted feature absent %s/%s", feature.Category, feature.Value))
	}
	sort.Strings(errs)
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func ReviewedAllowlist(operationCounts map[string]int, inventories ...*Inventory) Allowlist {
	return Allowlist{
		SchemaVersion:   1,
		OpenAPI:         OpenAPIVersion,
		OperationCounts: operationCounts,
		Features:        mergeFeatures(inventories...),
	}
}

func mergeFeatures(inventories ...*Inventory) []Feature {
	type featureUse struct {
		policy     string
		operations map[string]bool
	}
	merged := map[string]*featureUse{}
	for _, inventory := range inventories {
		for _, feature := range inventory.Features {
			key := feature.Category + "\x00" + feature.Value
			use := merged[key]
			if use == nil {
				use = &featureUse{policy: feature.Policy, operations: map[string]bool{}}
				merged[key] = use
			}
			for _, operation := range feature.Operations {
				use.operations[operation] = true
			}
		}
	}
	result := make([]Feature, 0, len(merged))
	for key, use := range merged {
		parts := strings.SplitN(key, "\x00", 2)
		result = append(result, Feature{
			Category: parts[0], Value: parts[1],
			Operations: sortedBoolKeys(use.operations), Policy: use.policy,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].Value < result[j].Value
	})
	return result
}

func DecodeAllowlist(data []byte) (*Allowlist, error) {
	var result Allowlist
	if err := DecodeStrict(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func DecodeStrict(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	return requireEOF(dec)
}

func (doc *Document) resolveObject(raw any) (map[string]any, string, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, "", errors.New("value is not an object")
	}
	ref, _ := object["$ref"].(string)
	if ref == "" {
		return object, "", nil
	}
	resolved, err := doc.resolvePointer(ref)
	if err != nil {
		return nil, "", err
	}
	result, ok := resolved.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("%q does not resolve to an object", ref)
	}
	return result, ref, nil
}

func (doc *Document) resolvePointer(pointer string) (any, error) {
	if !strings.HasPrefix(pointer, "#/") {
		return nil, fmt.Errorf("external reference %q is unsupported", pointer)
	}
	var current any = doc.Root
	for _, rawToken := range strings.Split(strings.TrimPrefix(pointer, "#/"), "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(rawToken, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("reference %q traverses a non-object", pointer)
		}
		current, ok = object[token]
		if !ok {
			return nil, fmt.Errorf("reference %q is unresolved", pointer)
		}
	}
	return current, nil
}

func featurePolicy(category, value string) string {
	switch category {
	case "http-method":
		return "generated-rest-operation"
	case "media-type":
		if strings.Contains(value, "event-stream") {
			return "retained-streaming-implementation"
		}
		return "generated-json-or-binary-media"
	case "security-scheme", "security-requirement":
		return "generated-auth-input-with-handwritten-runtime"
	case "security-requirement-combination":
		return "reviewed-security-alternative"
	case "parameter-location", "parameter-style":
		return "generated-parameter-serialization"
	case "schema-type", "schema-format", "schema-keyword":
		return "openapi-generator-7.27.0-with-reviewed-template-policy"
	case "server-definition":
		return "generated-server-selection-and-base-path"
	case "extension":
		return "reviewed-openapi-extension"
	default:
		return "reviewed"
	}
}

func isSchemaKeyword(keyword string) bool {
	if strings.HasPrefix(keyword, "x-") {
		return true
	}
	allowed := map[string]bool{
		"$anchor": true, "$comment": true, "$defs": true, "$dynamicAnchor": true, "$dynamicRef": true,
		"$id": true, "$ref": true, "$schema": true, "$vocabulary": true,
		"additionalProperties": true, "allOf": true, "anyOf": true, "const": true, "contains": true,
		"contentEncoding": true, "contentMediaType": true, "default": true, "dependentRequired": true,
		"dependentSchemas": true, "deprecated": true, "description": true, "discriminator": true,
		"else": true, "enum": true, "example": true, "examples": true, "exclusiveMaximum": true, "exclusiveMinimum": true,
		"externalDocs": true, "format": true, "if": true, "items": true, "maxContains": true,
		"maxItems": true, "maxLength": true, "maxProperties": true, "maximum": true, "minContains": true,
		"minItems": true, "minLength": true, "minProperties": true, "minimum": true, "multipleOf": true,
		"not": true, "nullable": true, "oneOf": true, "pattern": true, "patternProperties": true,
		"prefixItems": true, "properties": true, "propertyNames": true, "readOnly": true, "required": true,
		"then": true, "title": true, "type": true, "unevaluatedItems": true, "unevaluatedProperties": true,
		"uniqueItems": true, "writeOnly": true, "xml": true,
	}
	return allowed[keyword]
}

func schemaTypes(value any) []string {
	result := valueStrings(value)
	sort.Strings(result)
	return result
}

func valueStrings(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return []string{}
	}
}

func isNullable(schema map[string]any) bool {
	return boolValue(schema["nullable"], false) || slices.Contains(schemaTypes(schema["type"]), "null")
}

func additionalPropertiesKind(value any) string {
	switch typed := value.(type) {
	case nil:
		return "unspecified"
	case bool:
		return strconv.FormatBool(typed)
	case map[string]any:
		if _, ok := typed["$ref"]; ok {
			return "schema-ref"
		}
		return "schema"
	default:
		return "invalid"
	}
}

func securityRequirementNames(value any) []string {
	var result []string
	for _, requirementRaw := range anySlice(value) {
		requirement, _ := requirementRaw.(map[string]any)
		for name := range requirement {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return slices.Compact(result)
}

func securityRequirementCombinations(value any) []string {
	var result []string
	for _, requirementRaw := range anySlice(value) {
		requirement, _ := requirementRaw.(map[string]any)
		names := sortedKeys(requirement)
		if len(names) == 0 {
			result = append(result, "anonymous")
		} else {
			result = append(result, strings.Join(names, "+"))
		}
	}
	sort.Strings(result)
	return slices.Compact(result)
}

func valuesEqual(left, right any) bool {
	leftData, leftErr := Canonical(left)
	rightData, rightErr := Canonical(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}

func clone(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = clone(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = clone(item)
		}
		return result
	default:
		return typed
	}
}

func cloneBoolMap(input map[string]bool) map[string]bool {
	result := make(map[string]bool, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func shortDigest(value any) string {
	digest, _ := DigestValue(value)
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func sortedKeys(input map[string]any) []string {
	result := make([]string, 0, len(input))
	for key := range input {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func sortedBoolKeys(input map[string]bool) []string {
	result := make([]string, 0, len(input))
	for key := range input {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func objectAt(root map[string]any, path ...string) (map[string]any, bool) {
	var current any = root
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[key]
		if !ok {
			return nil, false
		}
	}
	result, ok := current.(map[string]any)
	return result, ok
}

func stringAt(root map[string]any, path ...string) string {
	var current any = root
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	return stringValue(current)
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func stringSlice(value any) []string {
	return valueStrings(value)
}

func anySlice(value any) []any {
	result, _ := value.([]any)
	return result
}

func boolValue(value any, fallback bool) bool {
	result, ok := value.(bool)
	if !ok {
		return fallback
	}
	return result
}

func hasKey(input map[string]any, key string) bool {
	_, ok := input[key]
	return ok
}

func isEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}
