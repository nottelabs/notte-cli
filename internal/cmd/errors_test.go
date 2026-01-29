package cmd

import (
	"net/http"
	"testing"
)

func TestHandleAPIResponse_Success(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", 200},
		{"201 Created", 201},
		{"204 No Content", 204},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
			}
			err := HandleAPIResponse(resp, nil)
			if err != nil {
				t.Errorf("expected nil error for %d, got %v", tt.statusCode, err)
			}
		})
	}
}

func TestHandleAPIResponse_Error(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", 400},
		{"401 Unauthorized", 401},
		{"404 Not Found", 404},
		{"500 Internal Server Error", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"error": "test error"}`)
			resp := &http.Response{
				StatusCode: tt.statusCode,
			}

			err := HandleAPIResponse(resp, body)
			if err == nil {
				t.Errorf("expected error for %d, got nil", tt.statusCode)
			}
		})
	}
}
