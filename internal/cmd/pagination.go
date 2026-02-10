package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func registerPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().Int("page", 0, "Page number (1-indexed)")
	cmd.Flags().Int("page-size", 0, "Number of items per page")
}

func getPageFlag(cmd *cobra.Command) (*int, error) {
	if cmd.Flags().Changed("page") {
		v, _ := cmd.Flags().GetInt("page")
		if v < 1 {
			return nil, fmt.Errorf("--page must be >= 1 (got %d)", v)
		}
		return &v, nil
	}
	return nil, nil
}

func getPageSizeFlag(cmd *cobra.Command) (*int, error) {
	if cmd.Flags().Changed("page-size") {
		v, _ := cmd.Flags().GetInt("page-size")
		if v < 1 {
			return nil, fmt.Errorf("--page-size must be >= 1 (got %d)", v)
		}
		return &v, nil
	}
	return nil, nil
}
