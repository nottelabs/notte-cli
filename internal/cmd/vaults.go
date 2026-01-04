package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var vaultsCreateName string

var vaultsCmd = &cobra.Command{
	Use:   "vaults",
	Short: "Manage vaults",
	Long:  "List and create vaults for storing credentials.",
}

var vaultsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all vaults",
	RunE:  runVaultsList,
}

var vaultsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new vault",
	RunE:  runVaultsCreate,
}

func init() {
	rootCmd.AddCommand(vaultsCmd)
	vaultsCmd.AddCommand(vaultsListCmd)
	vaultsCmd.AddCommand(vaultsCreateCmd)

	vaultsCreateCmd.Flags().StringVar(&vaultsCreateName, "name", "", "Name of the vault")
}

func runVaultsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListVaultsParams{}
	resp, err := client.Client().ListVaultsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()

	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		fmt.Println("No vaults found.")
		return nil
	}

	return formatter.Print(resp.JSON200.Items)
}

func runVaultsCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Build request body from flags
	body := api.VaultCreateJSONRequestBody{}

	// Set name if provided
	if vaultsCreateName != "" {
		body.Name = &vaultsCreateName
	}

	params := &api.VaultCreateParams{}
	resp, err := client.Client().VaultCreateWithResponse(ctx, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}
