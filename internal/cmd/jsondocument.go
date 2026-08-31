package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// readJSONDocumentFlag resolves a flag holding a JSON document into the string
// the API expects. Generated multipart builders call it for every field listed
// in JSONDocumentFields (see scripts/gen-flags/types.go).
//
// Such a field is typed `string` in the spec because the API stores it
// verbatim — `response_format`, the JSON Schema of a function's return value,
// is the field this exists for. That means nothing on the server rejects a
// document that is not JSON at all, so it is parsed here instead.
//
// Returns ("", nil) when the flag was not passed, which keeps the field out of
// the request entirely. The distinction matters on update: sending an empty
// value would replace a document the resource already has with nothing.
func readJSONDocumentFlag(cmd *cobra.Command, flagName, value string) (string, error) {
	if !cmd.Flags().Changed(flagName) {
		return "", nil
	}

	data, err := readJSONInput(cmd, value, flagName)
	if err != nil {
		return "", err
	}

	// Dropping the @ is the easy mistake, and without this check
	// `--response-format schema.json` uploads the string "schema.json" as the
	// document and gets a 200 back.
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return "", fmt.Errorf(
			"--%s must be a JSON object (use --%s @file.json to read one from disk): %w",
			flagName, flagName, err,
		)
	}
	// `null` is valid JSON and unmarshals into a nil map without error, so it
	// would otherwise pass the check above and be sent verbatim — which on
	// update replaces a working schema with nothing, the exact outcome the
	// "unset means send nothing" rule exists to prevent.
	if document == nil {
		return "", fmt.Errorf("--%s must be a JSON object, not null", flagName)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return "", fmt.Errorf("failed to compact --%s: %w", flagName, err)
	}
	return compact.String(), nil
}
