package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var (
	usageShowPeriod     string
	usageLogsEndpoint   string
	usageLogsPage       int
	usageLogsPageSize   int
	usageLogsOnlyActive bool
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show API usage statistics",
	Long:  "Display usage statistics including credits, costs, and quotas.",
	RunE:  runUsageShow,
}

var usageLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show usage logs",
	Long:  "Display paginated usage logs with optional filtering by endpoint.",
	RunE:  runUsageLogs,
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.AddCommand(usageLogsCmd)

	// Flags for usage show command
	usageCmd.Flags().StringVar(&usageShowPeriod, "period", "", "Monthly period to get usage for (e.g., 'May 2025')")

	// Flags for usage logs command
	usageLogsCmd.Flags().StringVar(&usageLogsEndpoint, "endpoint", "", "Filter logs by endpoint")
	usageLogsCmd.Flags().IntVar(&usageLogsPage, "page", 1, "Page number")
	usageLogsCmd.Flags().IntVar(&usageLogsPageSize, "page-size", 20, "Number of items per page")
	usageLogsCmd.Flags().BoolVar(&usageLogsOnlyActive, "only-active", false, "Only return active sessions")
}

func runUsageShow(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.GetUsageParams{}
	if usageShowPeriod != "" {
		params.Period = &usageShowPeriod
	}

	resp, err := client.Client().GetUsageWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}

func runUsageLogs(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.GetUsageLogsParams{}

	if usageLogsEndpoint != "" {
		params.Endpoint = &usageLogsEndpoint
	}

	if usageLogsPage > 0 {
		params.Page = &usageLogsPage
	}

	if usageLogsPageSize > 0 {
		params.PageSize = &usageLogsPageSize
	}

	if cmd.Flags().Changed("only-active") {
		params.OnlyActive = &usageLogsOnlyActive
	}

	resp, err := client.Client().GetUsageLogsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()
	var items []api.UsageLog
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No usage logs found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return formatter.Print(items)
}
