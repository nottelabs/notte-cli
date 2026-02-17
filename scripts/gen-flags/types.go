package main

import (
	"fmt"
	"strings"
)

// FieldCategory represents the classification of an OpenAPI field
type FieldCategory int

const (
	CategorySimpleFlag FieldCategory = iota
	CategoryEnumFlag
	CategoryFlattenedFlags
	CategoryRepeatedFlag
	CategoryJSONFileInput // For complex objects that should be passed via JSON file
	CategorySkipped       // Fields to skip entirely (don't generate, don't error)
	CategoryUnsupported
)

func (c FieldCategory) String() string {
	switch c {
	case CategorySimpleFlag:
		return "SimpleFlag"
	case CategoryEnumFlag:
		return "EnumFlag"
	case CategoryFlattenedFlags:
		return "FlattenedFlags"
	case CategoryRepeatedFlag:
		return "RepeatedFlag"
	case CategoryJSONFileInput:
		return "JSONFileInput"
	case CategorySkipped:
		return "Skipped"
	case CategoryUnsupported:
		return "Unsupported"
	default:
		return "Unknown"
	}
}

// SkippedFields contains field names that should be silently skipped (no error, no generation)
var SkippedFields = map[string]bool{
	"notifier_config": true,
}

// JSONFileFields contains field names that should be handled via --field-json @file
var JSONFileFields = map[string]bool{
	"response_format": true,
}

// FlattenWithoutPrefix - command-scoped fields that flatten without parent prefix
// Key: CommandName, Value: map of field names that should use short flag names
// e.g., "credentials" in VaultCredentialsAdd generates --email, --password (not --credentials-email)
var FlattenWithoutPrefix = map[string]map[string]bool{
	"VaultCredentialsAdd": {
		"credentials": true,
	},
}

// Field represents a field in an OpenAPI schema
type Field struct {
	Name        string
	JSONName    string
	Type        string
	Description string
	Required    bool
	Enum        []string
	Properties  map[string]*Field
	Items       *Field
	Ref         string
	Default     interface{}
	Nullable    bool
	IsUnionType bool // True if field is an anyOf with enum + string (not a simple enum)
}

// GoType returns the Go type for flag variables (without pointers)
func (f *Field) GoType() string {
	switch f.Type {
	case "string":
		return "string"
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	case "number":
		return "float64"
	case "array":
		if f.Items != nil {
			itemType := f.Items.GoType()
			return "[]" + strings.TrimPrefix(itemType, "*")
		}
		return "[]interface{}"
	default:
		return "interface{}"
	}
}

// FlagType returns the cobra flag type method
func (f *Field) FlagType() string {
	switch f.Type {
	case "string":
		return "StringVar"
	case "integer":
		return "IntVar"
	case "boolean":
		return "BoolVar"
	case "number":
		return "Float64Var"
	case "array":
		if f.Items != nil && f.Items.Type == "string" {
			return "StringSliceVar"
		}
		return "" // unsupported
	default:
		return ""
	}
}

// GenerationError represents an error during flag generation
type GenerationError struct {
	Endpoint   string
	Field      string
	FieldType  string
	Reason     string
	Suggestion string
	Location   string
}

func (e GenerationError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\nERROR: Unsupported field type in %s\n", e.Endpoint)
	fmt.Fprintf(&sb, "  Field: '%s'\n", e.Field)
	fmt.Fprintf(&sb, "  Type: %s\n", e.FieldType)
	if e.Location != "" {
		fmt.Fprintf(&sb, "  Location: %s\n", e.Location)
	}
	sb.WriteString("\n  ACTION REQUIRED:\n")
	fmt.Fprintf(&sb, "  %s\n", e.Reason)
	if e.Suggestion != "" {
		fmt.Fprintf(&sb, "\n  %s\n", e.Suggestion)
	}
	return sb.String()
}

// CommandConfig represents configuration for a CLI command
type CommandConfig struct {
	Name            string
	EndpointPath    string
	HTTPMethod      string
	RequestBodyType string
	Fields          []*FieldConfig
}

// FieldConfig represents a field to generate flags for
type FieldConfig struct {
	Field     *Field
	Category  FieldCategory
	FlagName  string
	VarName   string
	FlagType  string
	GoType    string
	SubFields []*FieldConfig // For flattened objects
}

// ClassifyField determines how to generate flags for a field
func ClassifyField(field *Field, schemas map[string]*Field) (FieldCategory, error) {
	// Check if field should be skipped entirely
	if SkippedFields[field.Name] || SkippedFields[field.JSONName] {
		return CategorySkipped, nil
	}

	// Check if field should use JSON file input
	if JSONFileFields[field.Name] || JSONFileFields[field.JSONName] {
		return CategoryJSONFileInput, nil
	}

	// Resolve $ref if present
	if field.Ref != "" {
		refField, ok := schemas[field.Ref]
		if !ok {
			return CategoryUnsupported, fmt.Errorf("unresolved reference: %s", field.Ref)
		}
		// Check if the referenced type is an enum
		if len(refField.Enum) > 0 {
			// Copy enum values to field for later use
			field.Enum = refField.Enum
			field.Type = "string"
			return CategoryEnumFlag, nil
		}
		field = refField
	}

	// Simple scalar types
	if isSimpleScalar(field) {
		return CategorySimpleFlag, nil
	}

	// Enum types
	if isEnum(field) {
		return CategoryEnumFlag, nil
	}

	// Primitive arrays ([]string, []int)
	if isPrimitiveArray(field) {
		return CategoryRepeatedFlag, nil
	}

	// Flattenable simple objects
	if isFlattenableObject(field, schemas) {
		return CategoryFlattenedFlags, nil
	}

	// Everything else is unsupported
	return CategoryUnsupported, nil
}

func isSimpleScalar(field *Field) bool {
	switch field.Type {
	case "string", "integer", "boolean", "number":
		return len(field.Enum) == 0
	default:
		return false
	}
}

func isEnum(field *Field) bool {
	return field.Type == "string" && len(field.Enum) > 0
}

func isPrimitiveArray(field *Field) bool {
	if field.Type != "array" || field.Items == nil {
		return false
	}
	return field.Items.Type == "string" || field.Items.Type == "integer"
}

func isFlattenableObject(field *Field, schemas map[string]*Field) bool {
	return isFlattenableObjectWithLimit(field, schemas, 3)
}

func isFlattenableObjectWithLimit(field *Field, schemas map[string]*Field, maxFields int) bool {
	// Resolve reference if needed
	if field.Ref != "" {
		refField, ok := schemas[field.Ref]
		if !ok {
			return false
		}
		field = refField
	}

	if field.Type != "object" {
		return false
	}

	// Object must have <= maxFields properties (0 means no limit)
	if len(field.Properties) == 0 || (maxFields > 0 && len(field.Properties) > maxFields) {
		return false
	}

	// All properties must be simple types
	for _, prop := range field.Properties {
		if !isSimpleScalar(prop) && !isEnum(prop) {
			return false
		}
	}

	return true
}

// ShouldFlattenWithoutPrefix checks if a command+field combo should use short flag names
func ShouldFlattenWithoutPrefix(commandName, fieldName string) bool {
	if cmdFields, ok := FlattenWithoutPrefix[commandName]; ok {
		return cmdFields[fieldName]
	}
	return false
}

// IsForceFlattenable checks if a field should be force-flattened (regardless of field count)
func IsForceFlattenable(commandName, fieldName string, field *Field, schemas map[string]*Field) bool {
	if !ShouldFlattenWithoutPrefix(commandName, fieldName) {
		return false
	}
	// Check if it's flattenable with no field limit
	return isFlattenableObjectWithLimit(field, schemas, 0)
}

// BuildGenerationError creates a detailed error for unsupported fields
func BuildGenerationError(endpoint, fieldName string, field *Field, schemas map[string]*Field) GenerationError {
	// Resolve ref for better error messages
	originalField := field
	if field.Ref != "" {
		if refField, ok := schemas[field.Ref]; ok {
			field = refField
		}
	}

	var fieldType, reason, suggestion string

	switch {
	case field.Type == "object" && len(field.Properties) > 3:
		fieldType = fmt.Sprintf("object with %d fields", len(field.Properties))
		reason = "Complex objects with >3 fields cannot be auto-generated."
		suggestion = `Options:
  1. Keep existing manual implementation
  2. Add --` + toKebabCase(fieldName) + `-json flag for JSON input
  3. Extend generator in scripts/gen-flags/types.go`

	case field.Type == "object" && hasNestedObjects(field, schemas):
		fieldType = "nested object"
		reason = "Objects containing nested objects cannot be auto-generated."
		suggestion = `Options:
  1. Provide JSON input via --` + toKebabCase(fieldName) + `-json @file.json
  2. Flatten nested structure in API design
  3. Add custom parser in command file`

	case field.Type == "array" && field.Items != nil && field.Items.Type == "object":
		fieldType = "array of objects"
		reason = "Arrays of complex objects cannot be auto-generated."
		suggestion = `Recommended approach:
  - Add --` + toKebabCase(fieldName) + `-json @file.json for complex input
  - Or: Implement repeated --` + toKebabCase(fieldName) + ` flag parser manually`

	case originalField.Ref != "" && strings.Contains(originalField.Ref, "union"):
		fieldType = "union type (anyOf/oneOf)"
		reason = "Union types require custom handling."
		suggestion = `Options:
  1. Keep existing manual implementation
  2. Add type-specific flags (e.g., --` + toKebabCase(fieldName) + `-type, --` + toKebabCase(fieldName) + `-value)
  3. Use JSON input for complex cases`

	default:
		fieldType = field.Type
		if field.Ref != "" {
			fieldType += " (ref: " + field.Ref + ")"
		}
		reason = "This field type is not supported by the auto-generator."
		suggestion = `Please add manual handling in the command file or extend the generator.`
	}

	return GenerationError{
		Endpoint:   endpoint,
		Field:      fieldName,
		FieldType:  fieldType,
		Reason:     reason,
		Suggestion: suggestion,
		Location:   "", // Will be set by caller
	}
}

func hasNestedObjects(field *Field, schemas map[string]*Field) bool {
	for _, prop := range field.Properties {
		// Resolve reference
		checkField := prop
		if prop.Ref != "" {
			if refField, ok := schemas[prop.Ref]; ok {
				checkField = refField
			}
		}
		if checkField.Type == "object" {
			return true
		}
	}
	return false
}

func toKebabCase(s string) string {
	// First convert underscores to hyphens
	s = strings.ReplaceAll(s, "_", "-")

	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func toSnakeCase(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}
