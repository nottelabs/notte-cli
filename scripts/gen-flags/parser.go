package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	multipartContentType = "multipart/form-data"
	octetStream          = "application/octet-stream"
)

// OpenAPISpec represents a simplified OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string              `json:"openapi"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
}

type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
}

type Operation struct {
	OperationID string       `json:"operationId"`
	Summary     string       `json:"summary"`
	RequestBody *RequestBody `json:"requestBody,omitempty"`
}

type RequestBody struct {
	Content map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema SchemaRef `json:"schema"`
}

type SchemaRef struct {
	Ref         string               `json:"$ref,omitempty"`
	Type        string               `json:"type,omitempty"`
	Properties  map[string]SchemaRef `json:"properties,omitempty"`
	Items       *SchemaRef           `json:"items,omitempty"`
	Enum        []interface{}        `json:"enum,omitempty"`
	Required    []string             `json:"required,omitempty"`
	Nullable    bool                 `json:"nullable,omitempty"`
	AnyOf       []SchemaRef          `json:"anyOf,omitempty"`
	Description string               `json:"description,omitempty"`
	Default     interface{}          `json:"default,omitempty"`
	// Set on multipart file parts. OpenAPI 3.1 spells it contentMediaType and
	// 3.0 spells it format: binary; the spec we read is converted from 3.1 by
	// generate.sh, which leaves contentMediaType alone, so both are accepted.
	ContentMediaType string `json:"contentMediaType,omitempty"`
	Format           string `json:"format,omitempty"`
}

type Components struct {
	Schemas map[string]SchemaRef `json:"schemas"`
}

// ParseOpenAPISpec reads and parses an OpenAPI specification file
func ParseOpenAPISpec(filename string) (*OpenAPISpec, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	return &spec, nil
}

// endpoint identifies one operation. Keyed by method as well as path because a
// path is not a command: /functions/{function_id} is `functions update` under
// POST and a metadata patch under PATCH, and only one of those is generated.
type endpoint struct {
	Method string
	Path   string
}

// endpointMap lists the operations that generate flags, and is deliberately
// opt-in: the generated Register*/Build* pair only earns its keep once a
// hand-written command calls it.
//
// Endpoints excluded from the spec by scripts/excluded-endpoints.txt can never
// appear here — generate.sh filters them out before this runs — so entries for
// /scrape, /scrape_from_html and /vaults/{vault_id}/card were removed rather
// than repaired.
var endpointMap = map[endpoint]string{
	{"POST", "/sessions/start"}:                   "SessionStart",
	{"POST", "/personas/create"}:                  "PersonaCreate",
	{"POST", "/profiles/create"}:                  "ProfileCreate",
	{"POST", "/vaults/create"}:                    "VaultCreate",
	{"PATCH", "/vaults/{vault_id}"}:               "VaultUpdate",
	{"POST", "/vaults/{vault_id}/credentials"}:    "VaultCredentialsAdd",
	{"POST", "/functions"}:                        "FunctionCreate",
	{"POST", "/functions/{function_id}"}:          "FunctionUpdate",
	{"POST", "/functions/{function_id}/schedule"}: "FunctionScheduleSet",
}

// methods is the set of operations inspected, in a fixed order so a spec that
// maps two methods on one path generates deterministically.
//
// GET is absent on purpose: this generator reads request bodies, and a GET has
// none. Flags for a GET would come from its query parameters, which are a
// different feature — see registerPaginationFlags for the hand-written ones.
var methods = []string{"POST", "PUT", "PATCH"}

// ExtractCommandConfigs extracts command configurations from the spec
func ExtractCommandConfigs(spec *OpenAPISpec) ([]*CommandConfig, error) {
	var configs []*CommandConfig
	schemas := buildSchemaMap(spec)

	// Sorted so the generator's output does not depend on map iteration order.
	paths := make([]string, 0, len(spec.Paths))
	for path := range spec.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem := spec.Paths[path]
		for _, method := range methods {
			commandName, ok := endpointMap[endpoint{method, path}]
			if !ok {
				continue
			}
			op := pathItem.operation(method)
			if op == nil || op.RequestBody == nil {
				return nil, fmt.Errorf(
					"%s %s is in endpointMap but the spec has no %s body for it "+
						"(renamed or removed upstream?)", method, path, method)
			}
			config, err := extractCommandConfig(commandName, path, method, op, schemas)
			if err != nil {
				return nil, err
			}
			if config != nil {
				configs = append(configs, config)
			}
		}
	}

	// A path that vanished from the spec used to fall out of the loop silently,
	// which is how five of the eleven original entries came to point at paths
	// the API no longer serves without anyone noticing.
	if len(configs) != len(endpointMap) {
		found := make(map[string]bool, len(configs))
		for _, c := range configs {
			found[c.Name] = true
		}
		var missing []string
		for _, name := range endpointMap {
			if !found[name] {
				missing = append(missing, name)
			}
		}
		sort.Strings(missing)
		return nil, fmt.Errorf(
			"%d endpoint(s) in endpointMap matched nothing in the spec: %s",
			len(missing), strings.Join(missing, ", "))
	}

	return configs, nil
}

// operation returns the operation for an HTTP method, or nil.
func (p PathItem) operation(method string) *Operation {
	switch method {
	case "POST":
		return p.Post
	case "PUT":
		return p.Put
	case "PATCH":
		return p.Patch
	}
	return nil
}

func extractCommandConfig(name, path, method string, op *Operation, schemas map[string]*Field) (*CommandConfig, error) {
	// JSON first: an endpoint offering both is a JSON endpoint that also accepts
	// a form, and the typed request struct is the better target.
	content, ok := op.RequestBody.Content["application/json"]
	multipart := false
	if !ok {
		content, ok = op.RequestBody.Content[multipartContentType]
		multipart = ok
	}
	if !ok {
		return nil, fmt.Errorf(
			"%s %s has no application/json or %s request body", method, path, multipartContentType)
	}

	if content.Schema.Ref == "" {
		return nil, fmt.Errorf(
			"%s %s request body is an inline schema; only $ref bodies are supported", method, path)
	}

	// Extract schema name from $ref
	schemaName := extractSchemaName(content.Schema.Ref)
	schema, ok := schemas[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %s not found", schemaName)
	}

	config := &CommandConfig{
		Name:            name,
		EndpointPath:    path,
		HTTPMethod:      method,
		RequestBodyType: schemaName,
		IsMultipart:     multipart,
	}

	// Process fields (sorted for deterministic output)
	fieldNames := make([]string, 0, len(schema.Properties))
	for fieldName := range schema.Properties {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		field := schema.Properties[fieldName]
		fieldConfig, err := processField(name, fieldName, field, schemas)
		if err != nil {
			return nil, err
		}
		config.Fields = append(config.Fields, fieldConfig)
	}

	return config, nil
}

func processField(commandName, fieldName string, field *Field, schemas map[string]*Field) (*FieldConfig, error) {
	ApplyDescriptionOverride(commandName, fieldName, field)

	category, err := ClassifyField(commandName, field, schemas)
	if err != nil {
		return nil, err
	}

	// Check if this field should be force-flattened (e.g., credentials in VaultCredentialsAdd)
	skipPrefix := ShouldFlattenWithoutPrefix(commandName, fieldName)
	if category == CategoryUnsupported && IsForceFlattenable(commandName, fieldName, field, schemas) {
		category = CategoryFlattenedFlags
	}

	flagName := toKebabCase(fieldName)
	varName := commandName + toCamelCase(fieldName)

	fc := &FieldConfig{
		Field:    field,
		Category: category,
		FlagName: flagName,
		VarName:  varName,
		FlagType: field.FlagType(),
		GoType:   field.GoType(),
	}

	// For flattened objects, process sub-fields (sorted for deterministic output)
	if category == CategoryFlattenedFlags {
		resolvedField := field
		if field.Ref != "" {
			if refField, ok := schemas[field.Ref]; ok {
				resolvedField = refField
			}
		}

		subFieldNames := make([]string, 0, len(resolvedField.Properties))
		for subFieldName := range resolvedField.Properties {
			subFieldNames = append(subFieldNames, subFieldName)
		}
		sort.Strings(subFieldNames)

		for _, subFieldName := range subFieldNames {
			subField := resolvedField.Properties[subFieldName]
			var subFlagName string
			if skipPrefix {
				// Use short flag names (e.g., --email instead of --credentials-email)
				subFlagName = toKebabCase(subFieldName)
			} else {
				subFlagName = flagName + "-" + toKebabCase(subFieldName)
			}
			subVarName := varName + toCamelCase(subFieldName)

			subFC := &FieldConfig{
				Field:    subField,
				Category: CategorySimpleFlag,
				FlagName: subFlagName,
				VarName:  subVarName,
				FlagType: subField.FlagType(),
				GoType:   subField.GoType(),
			}
			fc.SubFields = append(fc.SubFields, subFC)
		}
	}

	return fc, nil
}

func buildSchemaMap(spec *OpenAPISpec) map[string]*Field {
	schemas := make(map[string]*Field)
	for name, schemaRef := range spec.Components.Schemas {
		schemas[name] = convertSchemaRefToField(name, schemaRef, spec.Components.Schemas)
	}
	return schemas
}

func convertSchemaRefToField(name string, schemaRef SchemaRef, allSchemas map[string]SchemaRef) *Field {
	field := &Field{
		Name:        name,
		Type:        schemaRef.Type,
		Nullable:    schemaRef.Nullable,
		Description: schemaRef.Description,
		Default:     schemaRef.Default,
		IsFile:      schemaRef.ContentMediaType == octetStream || schemaRef.Format == "binary",
		Properties:  make(map[string]*Field),
	}

	if schemaRef.Ref != "" {
		field.Ref = extractSchemaName(schemaRef.Ref)
	}

	// Handle anyOf pattern (e.g., enum | string union)
	if len(schemaRef.AnyOf) > 0 {
		field = handleAnyOf(name, schemaRef, allSchemas)
		field.Description = schemaRef.Description
		return field
	}

	// Convert enum
	if len(schemaRef.Enum) > 0 {
		field.Enum = make([]string, len(schemaRef.Enum))
		for i, e := range schemaRef.Enum {
			field.Enum[i] = fmt.Sprintf("%v", e)
		}
	}

	// Convert properties
	for propName, propRef := range schemaRef.Properties {
		field.Properties[propName] = convertSchemaRefToField(propName, propRef, allSchemas)
		field.Properties[propName].JSONName = propName

		// Check if required
		for _, req := range schemaRef.Required {
			if req == propName {
				field.Properties[propName].Required = true
				break
			}
		}
	}

	// Convert items (for arrays)
	if schemaRef.Items != nil {
		field.Items = convertSchemaRefToField("", *schemaRef.Items, allSchemas)
	}

	return field
}

// handleAnyOf processes anyOf schemas and determines the appropriate field type
func handleAnyOf(name string, schemaRef SchemaRef, allSchemas map[string]SchemaRef) *Field {
	field := &Field{
		Name:       name,
		Properties: make(map[string]*Field),
	}

	// Check for enum | string pattern (e.g., reasoning_model)
	var enumValues []string
	hasString := false
	hasEnumRef := false

	for _, variant := range schemaRef.AnyOf {
		if variant.Type == "string" && len(variant.Enum) == 0 {
			hasString = true
		}
		if variant.Ref != "" {
			refName := extractSchemaName(variant.Ref)
			if refSchema, ok := allSchemas[refName]; ok {
				if len(refSchema.Enum) > 0 {
					hasEnumRef = true
					for _, e := range refSchema.Enum {
						enumValues = append(enumValues, fmt.Sprintf("%v", e))
					}
				}
			}
		}
	}

	// If we have enum ref + string, treat as string with enum suggestions (union type)
	if hasEnumRef && hasString {
		field.Type = "string"
		field.Enum = enumValues // Store for description purposes
		field.IsUnionType = true
		return field
	}

	// If we only have enum ref (without string), treat as simple enum
	if hasEnumRef && !hasString {
		field.Type = "string"
		field.Enum = enumValues
		field.IsUnionType = false
		return field
	}

	// Otherwise, mark as unsupported (complex union)
	field.Type = "" // Will be classified as unsupported
	return field
}

func extractSchemaName(ref string) string {
	// Extract name from "#/components/schemas/SchemaName"
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ref
}
