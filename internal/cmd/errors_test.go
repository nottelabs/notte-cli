package cmd

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestHandleAPIResponse_Success(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	err := HandleAPIResponse(resp)
	if err != nil {
		t.Errorf("expected nil error for 200, got: %v", err)
	}
}

func TestHandleAPIResponse_Error(t *testing.T) {
	body := `{"error":{"code":"not_found","message":"Resource not found"}}`
	resp := &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
	}

	err := HandleAPIResponse(resp)
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestHandleAPIResponse_Unauthorized(t *testing.T) {
	resp := &http.Response{
		StatusCode: 401,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	err := HandleAPIResponse(resp)
	if err == nil {
		t.Error("expected error for 401, got nil")
	}
}
