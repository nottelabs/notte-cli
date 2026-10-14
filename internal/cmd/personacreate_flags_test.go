package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestPersonaCreateEmailProviderResendMapping(t *testing.T) {
	cmd := &cobra.Command{Use: "create", RunE: func(*cobra.Command, []string) error { return nil }}
	RegisterPersonaCreateFlags(cmd)
	cmd.SetArgs([]string{"--email-provider", "resend"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body, err := BuildPersonaCreateRequest(cmd)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(payload), `{"email_provider":"resend"}`; got != want {
		t.Fatalf("request = %s, want %s", got, want)
	}
}
