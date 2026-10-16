package api

import (
	"encoding/json"
	"io"
	"testing"
)

func TestBrandRequestPreservesUnionBody(t *testing.T) {
	const input = `{"type":"by_domain","domain":"example.com"}`
	var body BrandRetrieveJSONRequestBody
	if err := json.Unmarshal([]byte(input), &body); err != nil {
		t.Fatal(err)
	}
	request, err := NewBrandRetrieveRequest("https://example.com", &BrandRetrieveParams{}, body)
	if err != nil {
		t.Fatal(err)
	}
	defer request.Body.Close()
	data, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["type"] != "by_domain" || got["domain"] != "example.com" {
		t.Fatalf("lost brand request fields: %s", data)
	}
}
