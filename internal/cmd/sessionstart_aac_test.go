package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
)

func TestTheAacVaultFlagAddsTheFieldToTheStartBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://api.test/sessions/start", bytes.NewReader([]byte(`{"headless":true,"proxies":false}`)))
	if err != nil {
		t.Fatal(err)
	}
	if err := withAacVault(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(req.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["aac_vault"] != true || body["headless"] != true || body["proxies"] != false {
		t.Fatalf("unexpected body %s", raw)
	}
	if req.ContentLength != int64(len(raw)) {
		t.Fatalf("content length %d, body %d bytes", req.ContentLength, len(raw))
	}
	again, err := req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	if replay, _ := io.ReadAll(again); !bytes.Equal(replay, raw) {
		t.Fatal("a retried request must send the same body")
	}
}

func TestOnlyTheAacVaultFlagAddsAnEditor(t *testing.T) {
	cmd := &cobra.Command{}
	registerSessionStartAacVaultFlag(cmd)
	t.Cleanup(func() { sessionsStartAacVault = false })
	if editors := sessionStartEditors(cmd); len(editors) != 0 {
		t.Fatal("no flag, no editor")
	}
	if err := cmd.Flags().Parse([]string{"--aac-vault"}); err != nil {
		t.Fatal(err)
	}
	if editors := sessionStartEditors(cmd); len(editors) != 1 {
		t.Fatal("--aac-vault adds the field")
	}
	if err := cmd.Flags().Parse([]string{"--aac-vault=false"}); err != nil {
		t.Fatal(err)
	}
	if editors := sessionStartEditors(cmd); len(editors) != 0 {
		t.Fatal("--aac-vault=false sends nothing")
	}
}
