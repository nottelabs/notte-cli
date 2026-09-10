#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="$PROJECT_ROOT/internal/api"

# Use NOTTE_API_URL env var, defaulting to staging for latest API features
NOTTE_API_URL="${NOTTE_API_URL:-https://us-staging.notte.cc}"
OPENAPI_URL="${NOTTE_API_URL}/openapi.json"

echo "Fetching OpenAPI spec from ${NOTTE_API_URL}..."
if ! curl -f -s "${OPENAPI_URL}" -o /tmp/notte-openapi.json; then
  echo "Error: Failed to fetch OpenAPI spec from ${NOTTE_API_URL}" >&2
  exit 1
fi

# Read excluded endpoints and build regex pattern
EXCLUDED_ENDPOINTS_FILE="$SCRIPT_DIR/excluded-endpoints.txt"
if [[ -f "$EXCLUDED_ENDPOINTS_FILE" ]]; then
  EXCLUDED_PATHS=$(grep -v '^#' "$EXCLUDED_ENDPOINTS_FILE" | grep -v '^$' | tr '\n' '|' | sed 's/|$//')
  echo "Excluding endpoints: $EXCLUDED_PATHS"
else
  EXCLUDED_PATHS=""
fi

echo "Converting OpenAPI 3.1 to 3.0 format..."
# Convert OpenAPI 3.1 to 3.0:
# 1. Filter out excluded endpoints
# 2. exclusiveMinimum from number to boolean
# 3. anyOf with null to nullable type
# 4. Remove null from type arrays
# 5. Add missing path parameters
jq --arg excluded "$EXCLUDED_PATHS" '
  # Filter out excluded paths
  (if $excluded != "" then .paths |= with_entries(select(.key | test($excluded) | not)) else . end) |
  # First pass: fix schema types
  walk(
    if type == "object" then
      # Handle exclusiveMinimum
      if has("exclusiveMinimum") and (.exclusiveMinimum | type == "number") then
        .minimum = .exclusiveMinimum | .exclusiveMinimum = true
      else . end |
      # Handle anyOf with null type - convert to nullable
      if has("anyOf") and (.anyOf | map(select(.type == "null")) | length > 0) then
        # Remove null types and set nullable
        .anyOf = (.anyOf | map(select(.type != "null"))) | .nullable = true |
        # If only one non-null type remains, flatten it
        if (.anyOf | length == 1) then
          if .anyOf[0].type then .type = .anyOf[0].type else . end |
          if .anyOf[0]["$ref"] then .["$ref"] = .anyOf[0]["$ref"] else . end |
          if .anyOf[0].items then .items = .anyOf[0].items else . end |
          del(.anyOf)
        else . end
      else . end |
      # Handle type arrays with null
      if has("type") and (.type | type == "array") and (.type | contains(["null"])) then
        .type = (.type | map(select(. != "null"))[0]) | .nullable = true
      else . end
    else . end
  ) |
  # Second pass: add missing path parameters
  .paths |= with_entries(
    .key as $path |
    # Extract all path parameter names from the path
    ([$path | scan("\\{([^}]+)\\}") | .[0]]) as $path_params |
    .value |= with_entries(
      if .key | IN("get", "post", "put", "delete", "patch") then
        .value.parameters //= [] |
        # Get existing parameter names
        (.value.parameters | map(.name)) as $existing_params |
        # Add any missing path parameters
        .value.parameters += (
          $path_params | map(
            . as $param |
            if ($existing_params | index($param)) == null then
              {
                "name": $param,
                "in": "path",
                "required": true,
                "schema": {"type": "string"}
              }
            else empty end
          )
        )
      else . end
    )
  ) | .openapi = "3.0.3"
' /tmp/notte-openapi.json > /tmp/notte-openapi-3.0.json

echo "Generating Go client..."
mkdir -p "$OUTPUT_DIR"

# Create oapi-codegen config file
cat > /tmp/notte-codegen-config.yaml <<EOF
package: api
output: $OUTPUT_DIR/client.gen.go
generate:
  models: true
  client: true
output-options:
  skip-prune: true
  skip-fmt: false
  response-type-suffix: Result
EOF

oapi-codegen \
  -config /tmp/notte-codegen-config.yaml \
  /tmp/notte-openapi-3.0.json

echo "Fixing generated code..."
# Fix string literal assignments to *string fields
# Convert: v.Type = "value" to: tmp := "value"; v.Type = &tmp
perl -i -pe 's/(\s+)(\w+)\.Type = "([^"]+)"/\1tmp := "\3"; \2.Type = &tmp/' "$OUTPUT_DIR/client.gen.go"

# Rename NewClient to newGeneratedClient to avoid collision with wrapper
# Only rename the base NewClient, not NewClientWithResponses
sed -i.bak 's/^func NewClient(/func newGeneratedClient(/g' "$OUTPUT_DIR/client.gen.go"
# Also update the call to NewClient within NewClientWithResponses
sed -i.bak 's/NewClient(server, opts\.\.\.)/newGeneratedClient(server, opts...)/g' "$OUTPUT_DIR/client.gen.go"

# Replace time.Time with FlexibleTime in struct fields for flexible timestamp parsing
# This handles API responses that don't include timezone info.
#
# The gap between the field name and its type is one space in a struct whose
# names happen to be the same length and gofmt's alignment padding otherwise, so
# both are matched: `  *` rather than a single space, and rather than GNU sed's
# `\+`, which BSD sed on macOS reads as a literal plus. Matching only one space
# is how FunctionResponse.created_at silently lost FlexibleTime when the spec
# grew a longer field beside it. gofmt below restores the alignment.
sed -i.bak 's/\(	[A-Za-z]*\)  *time\.Time /\1 FlexibleTime /g' "$OUTPUT_DIR/client.gen.go"
sed -i.bak 's/\(	[A-Za-z]*\)  *\*time\.Time /\1 *FlexibleTime /g' "$OUTPUT_DIR/client.gen.go"

# A timestamp the substitution above missed parses only RFC3339, so a response
# without timezone info fails to decode and the command dies on a field it was
# only going to print. That failure reaches a user, not this script, so fail
# here instead: a new spelling of a time field should stop the regeneration.
if REMAINING=$(grep -nE '^	[A-Za-z]+ +\*?time\.Time' "$OUTPUT_DIR/client.gen.go"); then
  echo "Error: struct fields still typed time.Time after the FlexibleTime substitution:" >&2
  echo "$REMAINING" >&2
  echo "Widen the sed patterns above to cover them." >&2
  exit 1
fi

# With every timestamp now a FlexibleTime, nothing in the file refers to the
# time package and Go refuses to compile an unused import. Conditional because a
# spec that reintroduces one - a time.Duration in a params struct, say - has to
# keep it. Comment lines are dropped first so prose mentioning the package does
# not read as a use.
#
# Deliberately not `grep -q` here: it exits on the first match, and under
# pipefail the SIGPIPE that kills the upstream grep becomes the pipeline's
# status. A file that does use the package would then take the branch that
# deletes its import, intermittently, depending on whether the first grep had
# already finished writing. Reading all the input costs nothing on one file.
if ! grep -vE '^[[:space:]]*//' "$OUTPUT_DIR/client.gen.go" | grep -E '(^|[^A-Za-z_])time\.[A-Za-z]' >/dev/null; then
  sed -i.bak '/^	"time"$/d' "$OUTPUT_DIR/client.gen.go"
fi

# Add omitempty to pointer fields that don't have it (optional fields shouldn't serialize as null)
sed -i.bak -E 's/(\*[^`]+`json:"[^",]+)"`/\1,omitempty"`/g' "$OUTPUT_DIR/client.gen.go"

# Remove backup files
rm -f "$OUTPUT_DIR/client.gen.go.bak"

# Format the fixed code
gofmt -w "$OUTPUT_DIR/client.gen.go"

echo "Done! Generated $OUTPUT_DIR/client.gen.go"

# Generate propertyNames enums from the spec
echo ""
echo "Generating propertyNames enums..."
python3 "$SCRIPT_DIR/gen-property-names.py" \
  /tmp/notte-openapi-3.0.json \
  "$OUTPUT_DIR/property_names.gen.go"
gofmt -w "$OUTPUT_DIR/property_names.gen.go"
echo "Done! Generated $OUTPUT_DIR/property_names.gen.go"

# Generate CLI flags
echo ""
echo "Generating CLI flags..."
CMD_OUTPUT_DIR="$PROJECT_ROOT/internal/cmd"

# Run the package, not a glob of its files: `*.go` sweeps in _test.go files,
# which `go run` refuses outright.
if ! (cd "$PROJECT_ROOT" && go run ./scripts/gen-flags \
  -spec /tmp/notte-openapi-3.0.json \
  -output "$CMD_OUTPUT_DIR"); then
  echo "ERROR: Flag generation failed. See errors above." >&2
  exit 1
fi

echo "Formatting generated flag files..."
gofmt -w "$CMD_OUTPUT_DIR"/*_flags.gen.go 2>/dev/null || true

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "✓ Code generation complete!"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Generated files:"
echo "  - $OUTPUT_DIR/client.gen.go"
echo "  - $OUTPUT_DIR/property_names.gen.go"
echo "  - $CMD_OUTPUT_DIR/*_flags.gen.go"
echo ""
