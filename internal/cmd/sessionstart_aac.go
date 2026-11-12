package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

// sessionsStartAacVault fills the session's logins from the vault `notte vaults connect` connected.
var sessionsStartAacVault bool

func registerSessionStartAacVaultFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&sessionsStartAacVault, "aac-vault", false,
		"Fill logins from the Bitwarden vault connected with `notte vaults connect` (experimental)")
}

// sessionStartEditors are the request editors of a start: `aac_vault` is out of the API's
// OpenAPI schema while Agent Access is in alpha, so it is added to the generated body.
func sessionStartEditors(cmd *cobra.Command) []api.RequestEditorFn {
	if !cmd.Flags().Changed("aac-vault") || !sessionsStartAacVault {
		return nil
	}
	return []api.RequestEditorFn{withAacVault}
}

func withAacVault(_ context.Context, req *http.Request) error {
	if req.Body == nil {
		return fmt.Errorf("session start request has no body")
	}
	raw, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return err
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return fmt.Errorf("session start body: %w", err)
	}
	body["aac_vault"] = json.RawMessage("true")
	out, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(bytes.NewReader(out))
	req.ContentLength = int64(len(out))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(out)), nil }
	return nil
}
