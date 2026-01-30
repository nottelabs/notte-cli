package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var usageShowPeriod string

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show API usage statistics",
	Long:  "Display usage statistics including credits, costs, and quotas.",
	RunE:  runUsageShow,
}

func init() {
	rootCmd.AddCommand(usageCmd)

	// Flags for usage show command
	usageCmd.Flags().StringVar(&usageShowPeriod, "period", "", "Monthly period to get usage for (e.g., 'May 2025')")
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
