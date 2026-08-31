package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// responseFormatFlag is shared by `functions create` and `functions update`:
// the API accepts the field on both multipart bodies, and a schema attached at
// create time has to survive the next code push.
const responseFormatFlag = "response-format"

const responseFormatFlagUsage = "JSON Schema of run()'s return value, as inline JSON, @file, or - for stdin"

// readResponseFormat resolves --response-format into the value the API expects.
//
// The API documents the field as "JSON Schema of run()'s return value,
// computed by the caller" — the server never derives it, so a client that does
// not send it leaves the function with no schema at all. That is what the
// console reads to draw an endpoint's response card, so omitting it is the
// difference between a documented API and an undocumented one.
//
// Returns ("", nil) when the flag was not passed, which keeps the field out of
// the multipart body entirely. That distinction matters on update: sending an
// empty value would replace a schema the function already has with nothing.
func readResponseFormat(cmd *cobra.Command, value string) (string, error) {
	if !cmd.Flags().Changed(responseFormatFlag) {
		return "", nil
	}

	data, err := readJSONInput(cmd, value, responseFormatFlag)
	if err != nil {
		return "", err
	}

	// Parsed here rather than left to the server, which stores the field
	// verbatim. A caller who drops the leading @ otherwise uploads the string
	// "schema.json" as their schema, gets a 200, and only finds out when the
	// function renders as undocumented weeks later.
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return "", fmt.Errorf(
			"--%s must be a JSON object describing run()'s return value (use --%s @file.json to read one from disk): %w",
			responseFormatFlag, responseFormatFlag, err,
		)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return "", fmt.Errorf("failed to compact --%s: %w", responseFormatFlag, err)
	}
	return compact.String(), nil
}
