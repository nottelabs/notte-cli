package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

func TestRunUsageShow(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/usage", 200, `{"additional_credits":0,"browser_usage_cost":0,"is_usage_limit_exceeded":false,"llm_usage_cost":0,"monthly_credits_limit":0,"monthly_credits_usage":0,"monthly_session_count":0,"monthly_session_usage_minutes":0,"period":"May 2025","plan_type":"free","proxy_usage_cost":0,"proxy_usage_gb":0,"total_cost":0}`)

	origPeriod := usageShowPeriod
	usageShowPeriod = "May 2025"
	t.Cleanup(func() { usageShowPeriod = origPeriod })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runUsageShow(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunUsageLogs(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/usage/logs", 200, `{"has_next":false,"items":[{"created_at":"2020-01-01T00:00:00Z","duration_ms":10,"endpoint":"/v1/test"}],"page":1,"page_size":20}`)

	origEndpoint := usageLogsEndpoint
	origPage := usageLogsPage
	origPageSize := usageLogsPageSize
	origOnlyActive := usageLogsOnlyActive
	t.Cleanup(func() {
		usageLogsEndpoint = origEndpoint
		usageLogsPage = origPage
		usageLogsPageSize = origPageSize
		usageLogsOnlyActive = origOnlyActive
	})

	usageLogsEndpoint = "/v1/test"
	usageLogsPage = 1
	usageLogsPageSize = 20
	usageLogsOnlyActive = true

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&usageLogsOnlyActive, "only-active", false, "")
	_ = cmd.Flags().Set("only-active", "true")
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runUsageLogs(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunUsageLogs_Empty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/usage/logs", 200, `{"has_next":false,"items":[],"page":1,"page_size":20}`)

	origEndpoint := usageLogsEndpoint
	origPage := usageLogsPage
	origPageSize := usageLogsPageSize
	t.Cleanup(func() {
		usageLogsEndpoint = origEndpoint
		usageLogsPage = origPage
		usageLogsPageSize = origPageSize
	})

	usageLogsEndpoint = ""
	usageLogsPage = 1
	usageLogsPageSize = 20

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runUsageLogs(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No usage logs found.") {
		t.Errorf("expected empty message, got %q", stdout)
	}
}
