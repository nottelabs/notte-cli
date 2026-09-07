package api

import (
	"encoding/json"
	"testing"
)

func TestGotoActionUnionRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		roundTrip func(GotoAction) ([]byte, GotoAction, error)
	}{
		{
			name: "action space actions",
			roundTrip: func(input GotoAction) ([]byte, GotoAction, error) {
				var union ActionSpace_Actions_Item
				if err := union.FromGotoAction(input); err != nil {
					return nil, GotoAction{}, err
				}
				encoded, err := json.Marshal(union)
				if err != nil {
					return nil, GotoAction{}, err
				}
				var decoded ActionSpace_Actions_Item
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					return nil, GotoAction{}, err
				}
				output, err := decoded.AsGotoAction()
				return encoded, output, err
			},
		},
		{
			name: "action space browser actions",
			roundTrip: func(input GotoAction) ([]byte, GotoAction, error) {
				var union ActionSpace_BrowserActions_Item
				if err := union.FromGotoAction(input); err != nil {
					return nil, GotoAction{}, err
				}
				encoded, err := json.Marshal(union)
				if err != nil {
					return nil, GotoAction{}, err
				}
				var decoded ActionSpace_BrowserActions_Item
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					return nil, GotoAction{}, err
				}
				output, err := decoded.AsGotoAction()
				return encoded, output, err
			},
		},
		{
			name: "API execution response action",
			roundTrip: func(input GotoAction) ([]byte, GotoAction, error) {
				var union ApiExecutionResponse_Action
				if err := union.FromGotoAction(input); err != nil {
					return nil, GotoAction{}, err
				}
				encoded, err := json.Marshal(union)
				if err != nil {
					return nil, GotoAction{}, err
				}
				var decoded ApiExecutionResponse_Action
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					return nil, GotoAction{}, err
				}
				output, err := decoded.AsGotoAction()
				return encoded, output, err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			waitUntil := "load"
			encoded, output, err := tt.roundTrip(GotoAction{
				Url:       "https://example.com",
				WaitUntil: &waitUntil,
			})
			if err != nil {
				t.Fatalf("round trip goto action: %v", err)
			}

			var object map[string]interface{}
			if err := json.Unmarshal(encoded, &object); err != nil {
				t.Fatalf("unmarshal encoded action: %v", err)
			}
			if got := object["type"]; got != "goto" {
				t.Fatalf("type = %v, want goto", got)
			}
			if output.Type == nil || *output.Type != "goto" {
				t.Fatalf("round-tripped type = %v, want goto", output.Type)
			}
			if got := output.Url; got != "https://example.com" {
				t.Fatalf("round-tripped url = %v, want https://example.com", got)
			}
			if output.WaitUntil == nil || *output.WaitUntil != "load" {
				t.Fatalf("round-tripped wait_until = %v, want load", output.WaitUntil)
			}
		})
	}
}
