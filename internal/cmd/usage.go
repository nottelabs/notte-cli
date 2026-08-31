package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var (
	usageShowPeriod      string
	usageLogsEndpoint    string
	usageLogsCurrentOnly bool
	usageLogsSystem      bool
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show API usage statistics",
	Long:  "Display usage statistics including credits, costs, and quotas.",
	RunE:  runUsageShow,
}

var usageLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "List API request logs",
	Long:  "List the API requests made with this workspace's credentials, newest first.",
	Args:  cobra.NoArgs,
	RunE:  runUsageLogs,
}

func init() {
	rootCmd.AddCommand(usageCmd)

	// Flags for usage show command
	usageCmd.Flags().StringVar(&usageShowPeriod, "period", "", "Monthly period to get usage for (e.g., 'May 2025')")

	// Flags for usage logs command
	usageCmd.AddCommand(usageLogsCmd)
	registerPaginationFlags(usageLogsCmd)
	usageLogsCmd.Flags().StringVar(&usageLogsEndpoint, "endpoint", "", "Only show requests to this endpoint, e.g. /sessions/start")
	usageLogsCmd.Flags().BoolVar(&usageLogsCurrentOnly, "only-current-token", false, "Only show requests made with the API key in use now")
	usageLogsCmd.Flags().BoolVar(&usageLogsSystem, "include-system", false, "Include Notte's own internal requests")
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

	page, err := getPageFlag(cmd)
	if err != nil {
		return err
	}
	pageSize, err := getPageSizeFlag(cmd)
	if err != nil {
		return err
	}

	params := &api.GetUsageLogsParams{
		Page:     page,
		PageSize: pageSize,
	}
	if usageLogsEndpoint != "" {
		params.Endpoint = &usageLogsEndpoint
	}
	// Sent only when asked, like every other optional filter: the API owns the
	// defaults, and transmitting false would freeze today's values into the
	// client. `only_active` is deliberately not exposed - it is the shared
	// listing filter, and a request log is never active or inactive.
	if cmd.Flags().Changed("only-current-token") {
		params.OnlyCurrentToken = &usageLogsCurrentOnly
	}
	if cmd.Flags().Changed("include-system") {
		params.IncludeSystem = &usageLogsSystem
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	resp, err := client.Client().GetUsageLogsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}
	var items []api.UsageLog
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No usage logs found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return GetFormatter().Print(items)
}
