package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTailnetProxyUnionRoundTrip(t *testing.T) {
	t.Parallel()

	type tailnetProxyUnion interface {
		json.Marshaler
		json.Unmarshaler
		FromTailnetProxy(TailnetProxy) error
		AsTailnetProxy() (TailnetProxy, error)
	}
	unions := []struct {
		name string
		new  func() tailnetProxyUnion
	}{
		{"session start", func() tailnetProxyUnion { return &ApiSessionStartRequest_Proxies_0_Item{} }},
		{"scrape", func() tailnetProxyUnion { return &GlobalScrapeRequest_Proxies_0_Item{} }},
	}
	exitNode := "100.64.0.10"
	clientSecret := "test-client-secret"
	cases := []struct {
		name     string
		exitNode *string
	}{
		{"supplied exit node", &exitNode},
		{"omitted exit node", nil},
	}

	for _, union := range unions {
		for _, tc := range cases {
			t.Run(union.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				want := TailnetProxy{
					OauthClientId:     "test-client-id",
					OauthClientSecret: &clientSecret,
					ExitNode:          tc.exitNode,
				}
				input := union.new()
				if err := input.FromTailnetProxy(want); err != nil {
					t.Fatalf("create tailnet proxy union: %v", err)
				}
				encoded, err := json.Marshal(input)
				if err != nil {
					t.Fatalf("marshal tailnet proxy union: %v", err)
				}

				var object map[string]any
				if err := json.Unmarshal(encoded, &object); err != nil {
					t.Fatalf("unmarshal encoded proxy: %v", err)
				}
				if got := object["type"]; got != "tailnet" {
					t.Errorf("type = %v, want tailnet", got)
				}
				if got, present := object["exit_node"]; tc.exitNode == nil {
					if present {
						t.Errorf("exit_node = %v, want field omitted", got)
					}
				} else if got != *tc.exitNode {
					t.Errorf("exit_node = %v, want %q", got, *tc.exitNode)
				}

				decoded := union.new()
				if err := json.Unmarshal(encoded, decoded); err != nil {
					t.Fatalf("unmarshal tailnet proxy union: %v", err)
				}
				output, err := decoded.AsTailnetProxy()
				if err != nil {
					t.Fatalf("decode tailnet proxy: %v", err)
				}
				proxyType := "tailnet"
				want.Type = &proxyType
				if !reflect.DeepEqual(output, want) {
					t.Errorf("round-tripped proxy = %+v, want %+v", output, want)
				}
			})
		}
	}
}

func TestScrollActionUnionRoundTrip(t *testing.T) {
	t.Parallel()

	type scrollActionUnion interface {
		json.Marshaler
		json.Unmarshaler
		FromScrollDownAction(ScrollDownAction) error
		FromScrollUpAction(ScrollUpAction) error
		AsScrollDownAction() (ScrollDownAction, error)
		AsScrollUpAction() (ScrollUpAction, error)
	}

	unions := []struct {
		name string
		new  func() scrollActionUnion
	}{
		{"action space actions", func() scrollActionUnion { return &ActionSpace_Actions_Item{} }},
		{"action space browser actions", func() scrollActionUnion { return &ActionSpace_BrowserActions_Item{} }},
		{"API execution response action", func() scrollActionUnion { return &ApiExecutionResponse_Action{} }},
	}
	selector := "#scroll-container"
	iframeSelector := "iframe#embedded >>> #scroll-container"
	selectors := []struct {
		name  string
		value *string
	}{
		{"selector", &selector},
		{"iframe selector", &iframeSelector},
		{"nil selector", nil},
	}

	for _, union := range unions {
		for _, actionType := range []string{"scroll_down", "scroll_up"} {
			for _, selector := range selectors {
				t.Run(union.name+"/"+actionType+"/"+selector.name, func(t *testing.T) {
					t.Parallel()

					input := union.new()
					var err error
					if actionType == "scroll_down" {
						err = input.FromScrollDownAction(ScrollDownAction{Selector: selector.value})
					} else {
						err = input.FromScrollUpAction(ScrollUpAction{Selector: selector.value})
					}
					if err != nil {
						t.Fatalf("create scroll action union: %v", err)
					}
					encoded, err := json.Marshal(input)
					if err != nil {
						t.Fatalf("marshal scroll action union: %v", err)
					}

					var object map[string]any
					if err := json.Unmarshal(encoded, &object); err != nil {
						t.Fatalf("unmarshal encoded action: %v", err)
					}
					if got := object["type"]; got != actionType {
						t.Errorf("type = %v, want %s", got, actionType)
					}
					if got, present := object["selector"]; selector.value == nil {
						if present {
							t.Errorf("selector = %v, want field omitted", got)
						}
					} else if got != *selector.value {
						t.Errorf("selector = %v, want %q", got, *selector.value)
					}

					decoded := union.new()
					if err := json.Unmarshal(encoded, decoded); err != nil {
						t.Fatalf("unmarshal scroll action union: %v", err)
					}
					var gotType, gotSelector *string
					if actionType == "scroll_down" {
						var output ScrollDownAction
						output, err = decoded.AsScrollDownAction()
						gotType, gotSelector = output.Type, output.Selector
					} else {
						var output ScrollUpAction
						output, err = decoded.AsScrollUpAction()
						gotType, gotSelector = output.Type, output.Selector
					}
					if err != nil {
						t.Fatalf("decode scroll action: %v", err)
					}
					if gotType == nil || *gotType != actionType {
						t.Errorf("round-tripped type = %v, want %s", gotType, actionType)
					}
					if selector.value == nil {
						if gotSelector != nil {
							t.Errorf("round-tripped selector = %q, want nil", *gotSelector)
						}
					} else if gotSelector == nil {
						t.Errorf("round-tripped selector = nil, want %q", *selector.value)
					} else if *gotSelector != *selector.value {
						t.Errorf("round-tripped selector = %q, want %q", *gotSelector, *selector.value)
					}
				})
			}
		}
	}
}

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
