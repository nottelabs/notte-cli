package api

import (
	"encoding/json"
	"testing"
)

// Every timestamp on a generated response has to survive a payload without
// timezone info, which is what FlexibleTime is for. Asserting it here rather
// than only in generate.sh keeps the guarantee where a reader of the decoding
// code will find it: the substitution that applies FlexibleTime matched a
// single space between a field and its type, so a spec change that widened
// gofmt's alignment padding beside `created_at` silently dropped these three
// back to time.Time, and only a decode against a real response would have
// caught it.
func TestGeneratedTimestampsAcceptMissingTimezone(t *testing.T) {
	t.Parallel()

	const naive = `"2026-09-09T15:04:05"`

	tests := []struct {
		name    string
		payload string
		decode  func([]byte) (interface{ IsZero() bool }, error)
	}{
		{
			name:    "FunctionResponse.created_at",
			payload: `{"created_at":` + naive + `}`,
			decode: func(b []byte) (interface{ IsZero() bool }, error) {
				var v FunctionResponse
				err := json.Unmarshal(b, &v)
				return v.CreatedAt, err
			},
		},
		{
			name:    "FunctionListItemResponse.created_at",
			payload: `{"created_at":` + naive + `}`,
			decode: func(b []byte) (interface{ IsZero() bool }, error) {
				var v FunctionListItemResponse
				err := json.Unmarshal(b, &v)
				return v.CreatedAt, err
			},
		},
		{
			name:    "FunctionWithLinkResponse.created_at",
			payload: `{"created_at":` + naive + `}`,
			decode: func(b []byte) (interface{ IsZero() bool }, error) {
				var v FunctionWithLinkResponse
				err := json.Unmarshal(b, &v)
				return v.CreatedAt, err
			},
		},
		{
			name:    "FunctionRunListItemResponse.updated_at",
			payload: `{"updated_at":` + naive + `}`,
			decode: func(b []byte) (interface{ IsZero() bool }, error) {
				var v FunctionRunListItemResponse
				err := json.Unmarshal(b, &v)
				return v.UpdatedAt, err
			},
		},
		{
			name:    "Vault.created_at",
			payload: `{"created_at":` + naive + `}`,
			decode: func(b []byte) (interface{ IsZero() bool }, error) {
				var v Vault
				err := json.Unmarshal(b, &v)
				return v.CreatedAt, err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.decode([]byte(tc.payload))
			if err != nil {
				t.Fatalf("decoding %s: %v", tc.payload, err)
			}
			if got.IsZero() {
				t.Errorf("%s decoded to the zero time", tc.name)
			}
		})
	}
}
