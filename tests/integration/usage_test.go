//go:build integration

package integration

import (
	"encoding/json"
	"testing"
)

func TestUsage(t *testing.T) {
	result := runCLI(t, "usage")
	requireSuccess(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &resp); err != nil {
		t.Fatalf("Failed to parse usage response: %v", err)
	}

	// Validate key fields exist
	period, ok := resp["period"]
	if !ok {
		t.Fatal("Response missing 'period' field")
	}
	if period == "" {
		t.Fatal("'period' field is empty")
	}

	if _, ok := resp["plan_type"]; !ok {
		t.Fatal("Response missing 'plan_type' field")
	}

	if _, ok := resp["total_cost"]; !ok {
		t.Fatal("Response missing 'total_cost' field")
	}

	if _, ok := resp["session_count"]; !ok {
		t.Fatal("Response missing 'session_count' field")
	}

	t.Logf("Successfully retrieved usage for period: %s", period)
}

func TestUsageWithPeriod(t *testing.T) {
	result := runCLI(t, "usage", "--period", "March 2026")
	requireSuccess(t, result)

	var resp struct {
		Period string `json:"period"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &resp); err != nil {
		t.Fatalf("Failed to parse usage response: %v", err)
	}

	if resp.Period != "March 2026" {
		t.Fatalf("Expected period 'March 2026', got '%s'", resp.Period)
	}

	t.Logf("Successfully retrieved usage for specified period: %s", resp.Period)
}
