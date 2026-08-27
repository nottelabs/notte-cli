package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// newSessionStartFlagsCmd registers the real generated flag set, so these tests
// exercise RegisterSessionStartFlags/BuildSessionStartRequest as a pair rather
// than a hand-rolled duplicate that could drift from the generator.
func newSessionStartFlagsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "start", RunE: func(*cobra.Command, []string) error { return nil }}
	// BoolVar and friends write the default into the package-level flag var, so
	// registering per test is also what resets shared state between cases.
	RegisterSessionStartFlags(cmd)
	return cmd
}

// TestAdvancedStealthMapping covers the flag added by the OpenAPI regeneration
// that introduced advanced_stealth. The nil case is the important one: omitting
// the flag has to send nothing so the server default applies, and an explicit
// value would freeze today's default into the client.
func TestAdvancedStealthMapping(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want *bool
	}{
		{name: "omitted sends nothing", args: nil, want: nil},
		{name: "--advanced-stealth sends true", args: []string{"--advanced-stealth"}, want: boolPtr(true)},
		{name: "--advanced-stealth=false sends false", args: []string{"--advanced-stealth=false"}, want: boolPtr(false)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newSessionStartFlagsCmd()
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute(%v) error = %v", tc.args, err)
			}

			body, err := BuildSessionStartRequest(cmd)
			if err != nil {
				t.Fatalf("BuildSessionStartRequest() error = %v", err)
			}

			switch {
			case tc.want == nil && body.AdvancedStealth != nil:
				t.Errorf("AdvancedStealth = %v, want nil (unset)", *body.AdvancedStealth)
			case tc.want != nil && body.AdvancedStealth == nil:
				t.Errorf("AdvancedStealth = nil, want %v", *tc.want)
			case tc.want != nil && *body.AdvancedStealth != *tc.want:
				t.Errorf("AdvancedStealth = %v, want %v", *body.AdvancedStealth, *tc.want)
			}
		})
	}
}
