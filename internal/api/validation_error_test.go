package api

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestHTTPValidationErrorAcceptsStringDetail(t *testing.T) {
	rsp := &http.Response{
		StatusCode: 422,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"detail":"idle timeout cannot exceed max duration"}`)),
	}
	parsed, err := ParseSessionStartResult(rsp)
	if err != nil {
		t.Fatalf("string detail must decode, got %v", err)
	}
	if parsed.JSON422 == nil || parsed.JSON422.Detail == nil || (*parsed.JSON422.Detail)[0].Msg != "idle timeout cannot exceed max duration" {
		t.Fatalf("unexpected JSON422: %+v", parsed.JSON422)
	}
	if !bytes.Contains(parsed.Body, []byte("idle timeout")) {
		t.Fatalf("body not preserved: %s", parsed.Body)
	}
}

func TestHTTPValidationErrorAcceptsListDetail(t *testing.T) {
	var e HTTPValidationError
	if err := e.UnmarshalJSON([]byte(`{"detail":[{"loc":["body","x"],"msg":"bad","type":"value_error"}]}`)); err != nil {
		t.Fatal(err)
	}
	if e.Detail == nil || len(*e.Detail) != 1 || (*e.Detail)[0].Msg != "bad" {
		t.Fatalf("unexpected: %+v", e.Detail)
	}
}
