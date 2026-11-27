package spec

import (
	"fmt"
	"sort"
	"strings"
)

type featureAdder func(category, value string, operations ...string)

func validateDocumentStructure(doc *Document, inv *Inventory, add featureAdder) error {
	allOperations := make([]string, 0, len(inv.Operations))
	operationsByPath := map[string][]string{}
	for _, operation := range inv.Operations {
		allOperations = append(allOperations, operation.OperationID)
		operationsByPath[operation.Path] = append(operationsByPath[operation.Path], operation.OperationID)
		add("server-selection", serverSelectionValue(operation.ServerSource, operation.Servers), operation.OperationID)
	}
	sort.Strings(allOperations)

	if _, exists := doc.Root["webhooks"]; exists {
		return fmt.Errorf("root uses unsupported webhooks")
	}
	if _, exists := doc.Root["jsonSchemaDialect"]; exists {
		return fmt.Errorf("root uses unsupported jsonSchemaDialect")
	}
	if err := validateFields(doc.Root, "root", fieldSet(
		"openapi", "info", "servers", "paths", "components", "security", "tags", "externalDocs",
	), allOperations, add); err != nil {
		return err
	}
	if err := requireStringField(doc.Root, "openapi", "root openapi"); err != nil {
		return err
	}
	info, ok := doc.Root["info"].(map[string]any)
	if !ok {
		return errorsNewShape("root info", "object")
	}
	if err := validateInfo(info, allOperations, add); err != nil {
		return err
	}
	if err := validateServers(doc.Root["servers"], "root", allOperations, add); err != nil {
		return err
	}
	if err := validateSecurityRequirements(doc.Root["security"], "root security"); err != nil {
		return err
	}
	if err := validateTags(doc.Root["tags"], allOperations, add); err != nil {
		return err
	}
	if err := validateExternalDocs(doc.Root["externalDocs"], "root externalDocs", allOperations, add); err != nil {
		return err
	}

	components, err := optionalObject(doc.Root, "components", "root components")
	if err != nil {
		return err
	}
	if err := validateComponents(doc, components, allOperations, add); err != nil {
		return err
	}

	paths, ok := doc.Root["paths"].(map[string]any)
	if !ok {
		return errorsNewShape("root paths", "object")
	}
	for _, path := range sortedKeys(paths) {
		if strings.HasPrefix(path, "x-") {
			add("extension", extensionFeatureValue("paths", path, paths[path]), allOperations...)
			continue
		}
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("paths uses unknown field %q", path)
		}
		pathItem, ok := paths[path].(map[string]any)
		if !ok {
			return fmt.Errorf("path %s is not an object", path)
		}
		pathOperations := operationsByPath[path]
		if err := validatePathItem(doc, pathItem, path, pathOperations, add); err != nil {
			return err
		}
	}
	return nil
}

func validateComponents(doc *Document, components map[string]any, operations []string, add featureAdder) error {
	if components == nil {
		return nil
	}
	if err := validateFields(components, "components", fieldSet(
		"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes",
	), operations, add); err != nil {
		return err
	}
	for _, collection := range []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes"} {
		if _, err := optionalObject(components, collection, "components "+collection); err != nil {
			return err
		}
	}
	if schemas, ok := components["schemas"].(map[string]any); ok {
		for _, name := range sortedKeys(schemas) {
			schema, ok := schemas[name].(map[string]any)
			if !ok {
				return errorsNewShape("component schema "+name, "object")
			}
			if err := validateSchemaShape(schema, "component schema "+name, map[string]bool{}); err != nil {
				return err
			}
		}
	}
	for _, unsupported := range []string{"callbacks", "links", "pathItems"} {
		if _, exists := components[unsupported]; exists {
			return fmt.Errorf("components uses unsupported %s", unsupported)
		}
	}
	if values, ok := components["parameters"].(map[string]any); ok {
		for _, name := range sortedKeys(values) {
			if err := validateParameterReference(doc, values[name], "components parameter "+name, operations, add); err != nil {
				return err
			}
		}
	}
	if values, ok := components["requestBodies"].(map[string]any); ok {
		for _, name := range sortedKeys(values) {
			if err := validateRequestBodyReference(doc, values[name], "components requestBody "+name, operations, add); err != nil {
				return err
			}
		}
	}
	if values, ok := components["responses"].(map[string]any); ok {
		for _, name := range sortedKeys(values) {
			if err := validateResponseReference(doc, values[name], "components response "+name, operations, add); err != nil {
				return err
			}
		}
	}
	if values, ok := components["headers"].(map[string]any); ok {
		for _, name := range sortedKeys(values) {
			if err := validateHeaderReference(doc, values[name], "components header "+name, operations, add); err != nil {
				return err
			}
		}
	}
	if schemes, ok := components["securitySchemes"].(map[string]any); ok {
		for _, name := range sortedKeys(schemes) {
			raw, ok := schemes[name].(map[string]any)
			if !ok {
				return fmt.Errorf("security scheme %s is not an object", name)
			}
			if err := validateReferenceObject(raw, "security scheme "+name, operations, add); err != nil {
				return err
			}
			resolved, _, err := doc.resolveObject(raw)
			if err != nil {
				return err
			}
			affected := securitySchemeOperations(name, operations, doc)
			if err := validateSecurityScheme(resolved, "security scheme "+name, affected, add); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePathItem(doc *Document, pathItem map[string]any, path string, operations []string, add featureAdder) error {
	allowed := fieldSet("$ref", "summary", "description", "servers", "parameters")
	for method := range httpMethods {
		allowed[method] = true
	}
	if err := validateFields(pathItem, "path item "+path, allowed, operations, add); err != nil {
		return err
	}
	if ref, exists := pathItem["$ref"]; exists {
		return fmt.Errorf("path item %s uses unsupported reference %q", path, ref)
	}
	if err := validateServers(pathItem["servers"], "path item "+path, operations, add); err != nil {
		return err
	}
	parameters, err := optionalArray(pathItem, "parameters", "path item "+path+" parameters")
	if err != nil {
		return err
	}
	for index, parameter := range parameters {
		if err := validateParameterReference(doc, parameter, fmt.Sprintf("path item %s parameter %d", path, index), operations, add); err != nil {
			return err
		}
	}
	for _, method := range sortedKeys(pathItem) {
		if !httpMethods[strings.ToLower(method)] {
			continue
		}
		operation, ok := pathItem[method].(map[string]any)
		if !ok {
			return fmt.Errorf("%s %s operation is not an object", method, path)
		}
		if err := validateOperation(doc, operation, strings.ToUpper(method), path, add); err != nil {
			return err
		}
	}
	return nil
}

func validateOperation(doc *Document, operation map[string]any, method, path string, add featureAdder) error {
	operationID := stringValue(operation["operationId"])
	operations := []string{operationID}
	scope := method + " " + path
	if err := validateFields(operation, "operation "+scope, fieldSet(
		"tags", "summary", "description", "externalDocs", "operationId", "parameters", "requestBody",
		"responses", "deprecated", "security", "servers",
	), operations, add); err != nil {
		return err
	}
	if err := requireStringField(operation, "operationId", "operation "+scope+" operationId"); err != nil {
		return err
	}
	if err := requireBoolField(operation, "deprecated", "operation "+scope+" deprecated"); err != nil {
		return err
	}
	if _, exists := operation["callbacks"]; exists {
		return fmt.Errorf("%s uses unsupported callbacks", operationID)
	}
	if err := validateExternalDocs(operation["externalDocs"], "operation "+scope+" externalDocs", operations, add); err != nil {
		return err
	}
	if _, err := optionalStringArray(operation, "tags", "operation "+scope+" tags"); err != nil {
		return err
	}
	if err := validateServers(operation["servers"], "operation "+scope, operations, add); err != nil {
		return err
	}
	if err := validateSecurityRequirements(operation["security"], "operation "+scope+" security"); err != nil {
		return err
	}
	parameters, err := optionalArray(operation, "parameters", "operation "+scope+" parameters")
	if err != nil {
		return err
	}
	for index, parameter := range parameters {
		if err := validateParameterReference(doc, parameter, fmt.Sprintf("operation %s parameter %d", scope, index), operations, add); err != nil {
			return err
		}
	}
	if requestBody, exists := operation["requestBody"]; exists {
		if err := validateRequestBodyReference(doc, requestBody, "operation "+scope+" requestBody", operations, add); err != nil {
			return err
		}
	}
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		return fmt.Errorf("operation %s responses is not an object", scope)
	}
	for _, status := range sortedKeys(responses) {
		if err := validateResponseReference(doc, responses[status], "operation "+scope+" response "+status, operations, add); err != nil {
			return err
		}
	}
	return nil
}

func validateParameterReference(doc *Document, raw any, scope string, operations []string, add featureAdder) error {
	object, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s is not an object", scope)
	}
	if err := validateReferenceObject(object, scope, operations, add); err != nil {
		return err
	}
	parameter, _, err := doc.resolveObject(object)
	if err != nil {
		return err
	}
	if err := validateFields(parameter, scope, fieldSet(
		"name", "in", "description", "required", "deprecated", "allowEmptyValue", "style", "explode",
		"allowReserved", "schema", "example", "examples",
	), operations, add); err != nil {
		return err
	}
	for _, field := range []string{"name", "in", "description", "style"} {
		if err := requireStringField(parameter, field, scope+" "+field); err != nil {
			return err
		}
	}
	for _, field := range []string{"required", "deprecated", "allowEmptyValue", "explode", "allowReserved"} {
		if err := requireBoolField(parameter, field, scope+" "+field); err != nil {
			return err
		}
	}
	if _, exists := parameter["content"]; exists {
		return fmt.Errorf("%s uses unsupported parameter content", scope)
	}
	if schema, exists := parameter["schema"]; exists {
		schemaObject, ok := schema.(map[string]any)
		if !ok {
			return errorsNewShape(scope+" schema", "object")
		}
		if err := validateSchemaShape(schemaObject, scope+" schema", map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func validateRequestBodyReference(doc *Document, raw any, scope string, operations []string, add featureAdder) error {
	object, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s is not an object", scope)
	}
	if err := validateReferenceObject(object, scope, operations, add); err != nil {
		return err
	}
	body, _, err := doc.resolveObject(object)
	if err != nil {
		return err
	}
	if err := validateFields(body, scope, fieldSet("description", "content", "required"), operations, add); err != nil {
		return err
	}
	if err := requireBoolField(body, "required", scope+" required"); err != nil {
		return err
	}
	return validateContent(body["content"], scope+" content", operations, add)
}

func validateResponseReference(doc *Document, raw any, scope string, operations []string, add featureAdder) error {
	object, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s is not an object", scope)
	}
	if err := validateReferenceObject(object, scope, operations, add); err != nil {
		return err
	}
	response, _, err := doc.resolveObject(object)
	if err != nil {
		return err
	}
	if err := validateFields(response, scope, fieldSet("description", "headers", "content"), operations, add); err != nil {
		return err
	}
	if _, exists := response["links"]; exists {
		return fmt.Errorf("%s uses unsupported links", scope)
	}
	headers, err := optionalObject(response, "headers", scope+" headers")
	if err != nil {
		return err
	}
	if headers != nil {
		for _, name := range sortedKeys(headers) {
			if err := validateHeaderReference(doc, headers[name], scope+" header "+name, operations, add); err != nil {
				return err
			}
		}
	}
	return validateContent(response["content"], scope+" content", operations, add)
}

func validateHeaderReference(doc *Document, raw any, scope string, operations []string, add featureAdder) error {
	object, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s is not an object", scope)
	}
	if err := validateReferenceObject(object, scope, operations, add); err != nil {
		return err
	}
	header, _, err := doc.resolveObject(object)
	if err != nil {
		return err
	}
	if err := validateFields(header, scope, fieldSet(
		"description", "required", "deprecated", "allowEmptyValue", "style", "explode", "allowReserved",
		"schema", "example", "examples",
	), operations, add); err != nil {
		return err
	}
	for _, field := range []string{"description", "style"} {
		if err := requireStringField(header, field, scope+" "+field); err != nil {
			return err
		}
	}
	for _, field := range []string{"required", "deprecated", "allowEmptyValue", "explode", "allowReserved"} {
		if err := requireBoolField(header, field, scope+" "+field); err != nil {
			return err
		}
	}
	if _, exists := header["content"]; exists {
		return fmt.Errorf("%s uses unsupported header content", scope)
	}
	if schema, exists := header["schema"]; exists {
		schemaObject, ok := schema.(map[string]any)
		if !ok {
			return errorsNewShape(scope+" schema", "object")
		}
		if err := validateSchemaShape(schemaObject, scope+" schema", map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func validateContent(raw any, scope string, operations []string, add featureAdder) error {
	if raw == nil {
		return nil
	}
	content, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s is not an object", scope)
	}
	for _, mediaType := range sortedKeys(content) {
		media, ok := content[mediaType].(map[string]any)
		if !ok {
			return fmt.Errorf("%s media %s is not an object", scope, mediaType)
		}
		if err := validateFields(media, scope+" media "+mediaType, fieldSet("schema", "example", "examples"), operations, add); err != nil {
			return err
		}
		if _, exists := media["encoding"]; exists {
			return fmt.Errorf("%s media %s uses unsupported encoding", scope, mediaType)
		}
		if schema, exists := media["schema"]; exists {
			schemaObject, ok := schema.(map[string]any)
			if !ok {
				return errorsNewShape(scope+" media "+mediaType+" schema", "object")
			}
			if err := validateSchemaShape(schemaObject, scope+" media "+mediaType+" schema", map[string]bool{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSecurityScheme(scheme map[string]any, scope string, operations []string, add featureAdder) error {
	if err := validateFields(scheme, scope, fieldSet(
		"type", "description", "name", "in", "scheme", "bearerFormat", "flows", "openIdConnectUrl",
	), operations, add); err != nil {
		return err
	}
	for _, field := range []string{"type", "description", "name", "in", "scheme", "bearerFormat", "openIdConnectUrl"} {
		if err := requireStringField(scheme, field, scope+" "+field); err != nil {
			return err
		}
	}
	flows, ok := scheme["flows"].(map[string]any)
	if !ok && scheme["flows"] != nil {
		return fmt.Errorf("%s flows is not an object", scope)
	}
	if flows == nil {
		return nil
	}
	if err := validateFields(flows, scope+" flows", fieldSet("implicit", "password", "clientCredentials", "authorizationCode"), operations, add); err != nil {
		return err
	}
	for _, name := range sortedKeys(flows) {
		if strings.HasPrefix(name, "x-") {
			continue
		}
		flow, ok := flows[name].(map[string]any)
		if !ok {
			return fmt.Errorf("%s flow %s is not an object", scope, name)
		}
		if err := validateFields(flow, scope+" flow "+name, fieldSet("authorizationUrl", "tokenUrl", "refreshUrl", "scopes"), operations, add); err != nil {
			return err
		}
		for _, field := range []string{"authorizationUrl", "tokenUrl", "refreshUrl"} {
			if err := requireStringField(flow, field, scope+" flow "+name+" "+field); err != nil {
				return err
			}
		}
		scopes, err := optionalObject(flow, "scopes", scope+" flow "+name+" scopes")
		if err != nil {
			return err
		}
		for scopeName, description := range scopes {
			if _, ok := description.(string); !ok {
				return errorsNewShape(scope+" flow "+name+" scope "+scopeName, "string")
			}
		}
	}
	return nil
}

func validateServers(raw any, scope string, operations []string, add featureAdder) error {
	if raw == nil {
		return nil
	}
	servers, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("%s servers is not an array", scope)
	}
	for index, serverRaw := range servers {
		server, ok := serverRaw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s server %d is not an object", scope, index)
		}
		serverScope := fmt.Sprintf("%s server %d", scope, index)
		if err := validateFields(server, serverScope, fieldSet("url", "description", "variables"), operations, add); err != nil {
			return err
		}
		if err := requireStringField(server, "url", serverScope+" url"); err != nil {
			return err
		}
		if err := requireStringField(server, "description", serverScope+" description"); err != nil {
			return err
		}
		variables, err := optionalObject(server, "variables", serverScope+" variables")
		if err != nil {
			return err
		}
		for _, name := range sortedKeys(variables) {
			variable, ok := variables[name].(map[string]any)
			if !ok {
				return fmt.Errorf("%s variable %s is not an object", serverScope, name)
			}
			if err := validateFields(variable, serverScope+" variable "+name, fieldSet("default", "description", "enum"), operations, add); err != nil {
				return err
			}
			if _, exists := variable["default"]; !exists {
				return fmt.Errorf("%s variable %s has no default", serverScope, name)
			} else if _, ok := variable["default"].(string); !ok {
				return errorsNewShape(serverScope+" variable "+name+" default", "string")
			}
			if err := requireStringField(variable, "description", serverScope+" variable "+name+" description"); err != nil {
				return err
			}
			if _, err := optionalStringArray(variable, "enum", serverScope+" variable "+name+" enum"); err != nil {
				return err
			}
		}
		parsed, _ := serversFromRaw([]any{server})
		add("server-definition", serverScope+":"+serverSelectionValue("defined", parsed), operations...)
	}
	return nil
}

func validateInfo(info map[string]any, operations []string, add featureAdder) error {
	if err := validateFields(info, "info", fieldSet(
		"title", "summary", "description", "termsOfService", "contact", "license", "version",
	), operations, add); err != nil {
		return err
	}
	for _, field := range []string{"title", "summary", "description", "termsOfService", "version"} {
		if err := requireStringField(info, field, "info "+field); err != nil {
			return err
		}
	}
	contact, err := optionalObject(info, "contact", "info contact")
	if err != nil {
		return err
	}
	if contact != nil {
		if err := validateFields(contact, "info contact", fieldSet("name", "url", "email"), operations, add); err != nil {
			return err
		}
	}
	license, err := optionalObject(info, "license", "info license")
	if err != nil {
		return err
	}
	if license != nil {
		if err := validateFields(license, "info license", fieldSet("name", "identifier", "url"), operations, add); err != nil {
			return err
		}
	}
	return nil
}

func validateTags(raw any, operations []string, add featureAdder) error {
	if raw == nil {
		return nil
	}
	tags, ok := raw.([]any)
	if !ok {
		return errorsNewShape("root tags", "array")
	}
	for index, tagRaw := range tags {
		tag, ok := tagRaw.(map[string]any)
		if !ok {
			return fmt.Errorf("tag %d is not an object", index)
		}
		if err := validateFields(tag, fmt.Sprintf("tag %d", index), fieldSet("name", "description", "externalDocs"), operations, add); err != nil {
			return err
		}
		if err := validateExternalDocs(tag["externalDocs"], fmt.Sprintf("tag %d externalDocs", index), operations, add); err != nil {
			return err
		}
	}
	return nil
}

func validateExternalDocs(raw any, scope string, operations []string, add featureAdder) error {
	if raw == nil {
		return nil
	}
	docs, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s is not an object", scope)
	}
	return validateFields(docs, scope, fieldSet("description", "url"), operations, add)
}

func validateReferenceObject(object map[string]any, scope string, operations []string, add featureAdder) error {
	ref, hasRef := object["$ref"]
	if !hasRef {
		return nil
	}
	if err := validateFields(object, scope+" reference", fieldSet("$ref", "summary", "description"), operations, add); err != nil {
		return err
	}
	refValue, ok := ref.(string)
	if !ok || !strings.HasPrefix(refValue, "#/") {
		return fmt.Errorf("%s uses unsupported external reference %q", scope, refValue)
	}
	return nil
}

func validateSecurityRequirements(raw any, scope string) error {
	if raw == nil {
		return nil
	}
	requirements, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("%s is not an array", scope)
	}
	for index, requirementRaw := range requirements {
		requirement, ok := requirementRaw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s requirement %d is not an object", scope, index)
		}
		for name, scopesRaw := range requirement {
			scopes, ok := scopesRaw.([]any)
			if !ok {
				return fmt.Errorf("%s requirement %s scopes is not an array", scope, name)
			}
			for _, item := range scopes {
				if _, ok := item.(string); !ok {
					return fmt.Errorf("%s requirement %s has non-string scope", scope, name)
				}
			}
		}
	}
	return nil
}

func validateFields(object map[string]any, scope string, allowed map[string]bool, operations []string, add featureAdder) error {
	for _, field := range sortedKeys(object) {
		if allowed[field] {
			continue
		}
		if strings.HasPrefix(field, "x-") {
			add("extension", extensionFeatureValue(scope, field, object[field]), operations...)
			continue
		}
		return fmt.Errorf("%s uses unknown field %q", scope, field)
	}
	return nil
}

func fieldSet(fields ...string) map[string]bool {
	result := make(map[string]bool, len(fields))
	for _, field := range fields {
		result[field] = true
	}
	return result
}

func extensionFeatureValue(scope, field string, value any) string {
	digest, _ := DigestValue(value)
	return scope + ":" + field + ":" + digest
}

func serverSelectionValue(source string, servers []Server) string {
	digest, _ := DigestValue(servers)
	return source + ":" + digest
}

func serversFromRaw(raw any) ([]Server, bool) {
	values, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	result := make([]Server, 0, len(values))
	for _, value := range values {
		server, ok := value.(map[string]any)
		if !ok {
			continue
		}
		variables := map[string]any{}
		if rawVariables, ok := server["variables"].(map[string]any); ok {
			variables = clone(rawVariables).(map[string]any)
		}
		result = append(result, Server{URL: stringValue(server["url"]), Variables: variables})
	}
	return result, true
}

func (doc *Document) effectiveServers(pathItem, operation map[string]any) (string, []Server) {
	if servers, exists := operation["servers"]; exists {
		parsed, _ := serversFromRaw(servers)
		return "operation", parsed
	}
	if servers, exists := pathItem["servers"]; exists {
		parsed, _ := serversFromRaw(servers)
		return "path", parsed
	}
	if servers, exists := doc.Root["servers"]; exists {
		parsed, _ := serversFromRaw(servers)
		return "root", parsed
	}
	return "implicit", []Server{}
}

func securitySchemeOperations(name string, allOperations []string, doc *Document) []string {
	used := map[string]bool{}
	paths, _ := doc.Root["paths"].(map[string]any)
	for _, pathRaw := range paths {
		pathItem, _ := pathRaw.(map[string]any)
		for method, operationRaw := range pathItem {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			operation, _ := operationRaw.(map[string]any)
			security := operation["security"]
			if security == nil {
				security = doc.Root["security"]
			}
			if containsString(securityRequirementNames(security), name) {
				used[stringValue(operation["operationId"])] = true
			}
		}
	}
	if len(used) == 0 {
		return allOperations
	}
	return sortedBoolKeys(used)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func optionalObject(object map[string]any, field, scope string) (map[string]any, error) {
	value, exists := object[field]
	if !exists {
		return nil, nil
	}
	result, ok := value.(map[string]any)
	if !ok {
		return nil, errorsNewShape(scope, "object")
	}
	return result, nil
}

func optionalArray(object map[string]any, field, scope string) ([]any, error) {
	value, exists := object[field]
	if !exists {
		return []any{}, nil
	}
	result, ok := value.([]any)
	if !ok {
		return nil, errorsNewShape(scope, "array")
	}
	return result, nil
}

func optionalStringArray(object map[string]any, field, scope string) ([]string, error) {
	values, err := optionalArray(object, field, scope)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, errorsNewShape(scope+" item", "string")
		}
		result = append(result, text)
	}
	return result, nil
}

func errorsNewShape(scope, want string) error {
	return fmt.Errorf("%s must be a %s", scope, want)
}

func requireStringField(object map[string]any, field, scope string) error {
	value, exists := object[field]
	if !exists {
		return nil
	}
	if _, ok := value.(string); !ok {
		return errorsNewShape(scope, "string")
	}
	return nil
}

func requireBoolField(object map[string]any, field, scope string) error {
	value, exists := object[field]
	if !exists {
		return nil
	}
	if _, ok := value.(bool); !ok {
		return errorsNewShape(scope, "boolean")
	}
	return nil
}

func validateSchemaShape(schema map[string]any, scope string, stack map[string]bool) error {
	for field := range schema {
		if !isSchemaKeyword(field) {
			if field == "itemSchema" {
				return fmt.Errorf("%s uses unsupported OAS 3.2 itemSchema", scope)
			}
			return fmt.Errorf("%s uses unknown schema keyword %q", scope, field)
		}
	}
	if ref, exists := schema["$ref"]; exists {
		text, ok := ref.(string)
		if !ok {
			return errorsNewShape(scope+" $ref", "string")
		}
		if !strings.HasPrefix(text, "#/") {
			return fmt.Errorf("%s uses unsupported external reference %q", scope, text)
		}
	}
	if value, exists := schema["type"]; exists {
		switch typed := value.(type) {
		case string:
		case []any:
			for _, item := range typed {
				if _, ok := item.(string); !ok {
					return errorsNewShape(scope+" type array item", "string")
				}
			}
		default:
			return fmt.Errorf("%s type must be a string or string array", scope)
		}
	}
	if value, exists := schema["format"]; exists {
		if _, ok := value.(string); !ok {
			return errorsNewShape(scope+" format", "string")
		}
	}
	for _, field := range []string{"required", "enum", "examples"} {
		if _, exists := schema[field]; exists {
			values, ok := schema[field].([]any)
			if !ok {
				return errorsNewShape(scope+" "+field, "array")
			}
			if field == "required" {
				for _, value := range values {
					if _, ok := value.(string); !ok {
						return errorsNewShape(scope+" required item", "string")
					}
				}
			}
		}
	}
	for _, field := range []string{"items", "contains", "not", "propertyNames", "unevaluatedItems"} {
		if value, exists := schema[field]; exists {
			child, ok := value.(map[string]any)
			if !ok {
				return errorsNewShape(scope+" "+field, "object")
			}
			if err := validateSchemaShape(child, scope+" "+field, stack); err != nil {
				return err
			}
		}
	}
	for _, field := range []string{"additionalProperties", "unevaluatedProperties"} {
		if value, exists := schema[field]; exists {
			switch typed := value.(type) {
			case bool:
			case map[string]any:
				if err := validateSchemaShape(typed, scope+" "+field, stack); err != nil {
					return err
				}
			default:
				return fmt.Errorf("%s %s must be a boolean or object", scope, field)
			}
		}
	}
	for _, field := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		values, err := optionalArray(schema, field, scope+" "+field)
		if err != nil {
			return err
		}
		for index, value := range values {
			child, ok := value.(map[string]any)
			if !ok {
				return errorsNewShape(fmt.Sprintf("%s %s item %d", scope, field, index), "object")
			}
			if err := validateSchemaShape(child, fmt.Sprintf("%s %s item %d", scope, field, index), stack); err != nil {
				return err
			}
		}
	}
	for _, field := range []string{"properties", "patternProperties", "$defs", "dependentSchemas"} {
		children, err := optionalObject(schema, field, scope+" "+field)
		if err != nil {
			return err
		}
		for _, name := range sortedKeys(children) {
			child, ok := children[name].(map[string]any)
			if !ok {
				return errorsNewShape(scope+" "+field+" "+name, "object")
			}
			if err := validateSchemaShape(child, scope+" "+field+" "+name, stack); err != nil {
				return err
			}
		}
	}
	if discriminator, err := optionalObject(schema, "discriminator", scope+" discriminator"); err != nil {
		return err
	} else if discriminator != nil {
		for field := range discriminator {
			if field != "propertyName" && field != "mapping" {
				return fmt.Errorf("%s discriminator uses unknown field %q", scope, field)
			}
		}
		if property, exists := discriminator["propertyName"]; exists {
			if _, ok := property.(string); !ok {
				return errorsNewShape(scope+" discriminator propertyName", "string")
			}
		}
		if mapping, err := optionalObject(discriminator, "mapping", scope+" discriminator mapping"); err != nil {
			return err
		} else {
			for name, value := range mapping {
				if _, ok := value.(string); !ok {
					return errorsNewShape(scope+" discriminator mapping "+name, "string")
				}
			}
		}
	}
	if dependent, err := optionalObject(schema, "dependentRequired", scope+" dependentRequired"); err != nil {
		return err
	} else {
		for name := range dependent {
			values, ok := dependent[name].([]any)
			if !ok {
				return errorsNewShape(scope+" dependentRequired "+name, "array")
			}
			for _, value := range values {
				if _, ok := value.(string); !ok {
					return errorsNewShape(scope+" dependentRequired "+name+" item", "string")
				}
			}
		}
	}
	return nil
}
