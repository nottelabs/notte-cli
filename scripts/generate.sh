#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="$PROJECT_ROOT/internal/api"

echo "Fetching OpenAPI spec from api.notte.cc..."
if ! curl -f -s https://api.notte.cc/openapi.json -o /tmp/notte-openapi.json; then
  echo "Error: Failed to fetch OpenAPI spec from api.notte.cc" >&2
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
          if .anyOf[0].type then .type = .anyOf[0].type else . end | del(.anyOf)
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
sed -i '' 's/^func NewClient(/func newGeneratedClient(/g' "$OUTPUT_DIR/client.gen.go"
# Also update the call to NewClient within NewClientWithResponses
sed -i '' 's/NewClient(server, opts\.\.\.)/newGeneratedClient(server, opts...)/g' "$OUTPUT_DIR/client.gen.go"

# Format the fixed code
gofmt -w "$OUTPUT_DIR/client.gen.go"

echo "Done! Generated $OUTPUT_DIR/client.gen.go"
