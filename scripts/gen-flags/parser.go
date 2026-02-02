package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
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

// ExtractCommandConfigs extracts command configurations from the spec
func ExtractCommandConfigs(spec *OpenAPISpec) ([]*CommandConfig, error) {
	var configs []*CommandConfig
	schemas := buildSchemaMap(spec)

	// Map of endpoints to command names
	endpointMap := map[string]string{
		"/sessions/start":                                 "SessionStart",
		"/agents/start":                                   "AgentStart",
		"/personas/create":                                "PersonaCreate",
		"/profiles/create":                                "ProfileCreate",
		"/vaults/create":                                  "VaultCreate",
		"/vaults/{vault_id}":                              "VaultUpdate",
		"/vaults/{vault_id}/credentials":                  "VaultCredentialsAdd",
		"/vaults/{vault_id}/credit-card":                  "VaultCreditCardSet",
		"/functions/schedule":                             "FunctionScheduleSet",
		"/functions/{function_id}/runs/{run_id}/metadata": "FunctionRunUpdateMetadata",
		"/scrape":                                         "ScrapeWebpage",
		"/scrape-html":                                    "ScrapeFromHtml",
	}

	for path, pathItem := range spec.Paths {
		commandName, ok := endpointMap[path]
		if !ok {
			continue
		}

		// Check POST operations
		if pathItem.Post != nil && pathItem.Post.RequestBody != nil {
			config, err := extractCommandConfig(commandName, path, "POST", pathItem.Post, schemas)
			if err != nil {
				return nil, err
			}
			if config != nil {
				configs = append(configs, config)
			}
		}

		// Check PUT operations
		if pathItem.Put != nil && pathItem.Put.RequestBody != nil {
			config, err := extractCommandConfig(commandName, path, "PUT", pathItem.Put, schemas)
			if err != nil {
				return nil, err
			}
			if config != nil {
				configs = append(configs, config)
			}
		}
	}

	return configs, nil
}

func extractCommandConfig(name, path, method string, op *Operation, schemas map[string]*Field) (*CommandConfig, error) {
	content, ok := op.RequestBody.Content["application/json"]
	if !ok {
		return nil, nil
	}

	if content.Schema.Ref == "" {
		return nil, nil
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
	category, err := ClassifyField(field, schemas)
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
