package cmd

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var (
	vaultID                   string
	vaultUpdateName           string
	vaultCredentialsGetURL    string
	vaultCredentialsDeleteURL string
)

var vaultsCmd = &cobra.Command{
	Use:   "vaults",
	Short: "Manage vaults",
	Long:  "List, create, and operate on vaults for storing credentials.",
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

var vaultsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update vault details",
	Args:  cobra.NoArgs,
	RunE:  runVaultUpdate,
}

var vaultsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultDelete,
}

var vaultsCredentialsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Manage vault credentials",
	Long:  "List, add, get, and delete credentials stored in the vault.",
}

var vaultsCredentialsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all credentials in the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsList,
}

var vaultsCredentialsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add credentials to the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsAdd,
}

var vaultsCredentialsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get credentials for a specific URL",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsGet,
}

var vaultsCredentialsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete credentials for a specific URL",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsDelete,
}

func init() {
	rootCmd.AddCommand(vaultsCmd)
	vaultsCmd.AddCommand(vaultsListCmd)
	vaultsCmd.AddCommand(vaultsCreateCmd)
	vaultsCmd.AddCommand(vaultsUpdateCmd)
	vaultsCmd.AddCommand(vaultsDeleteCmd)
	vaultsCmd.AddCommand(vaultsCredentialsCmd)

	vaultsCredentialsCmd.AddCommand(vaultsCredentialsListCmd)
	vaultsCredentialsCmd.AddCommand(vaultsCredentialsAddCmd)
	vaultsCredentialsCmd.AddCommand(vaultsCredentialsGetCmd)
	vaultsCredentialsCmd.AddCommand(vaultsCredentialsDeleteCmd)

	// Create command flags (auto-generated)
	RegisterVaultCreateFlags(vaultsCreateCmd)

	// Credentials subcommand group - use PersistentFlags for --id
	vaultsCredentialsCmd.PersistentFlags().StringVar(&vaultID, "id", "", "Vault ID (required)")
	_ = vaultsCredentialsCmd.MarkPersistentFlagRequired("id")

	// Update command flags
	vaultsUpdateCmd.Flags().StringVar(&vaultID, "id", "", "Vault ID (required)")
	_ = vaultsUpdateCmd.MarkFlagRequired("id")
	vaultsUpdateCmd.Flags().StringVar(&vaultUpdateName, "name", "", "New name for the vault (required)")
	_ = vaultsUpdateCmd.MarkFlagRequired("name")

	// Delete command flags
	vaultsDeleteCmd.Flags().StringVar(&vaultID, "id", "", "Vault ID (required)")
	_ = vaultsDeleteCmd.MarkFlagRequired("id")

	// Credentials add command flags (auto-generated)
	RegisterVaultCredentialsAddFlags(vaultsCredentialsAddCmd)
	_ = vaultsCredentialsAddCmd.MarkFlagRequired("url")
	_ = vaultsCredentialsAddCmd.MarkFlagRequired("password")

	// Credentials get command flags
	vaultsCredentialsGetCmd.Flags().StringVar(&vaultCredentialsGetURL, "url", "", "URL to get credentials for (required)")
	_ = vaultsCredentialsGetCmd.MarkFlagRequired("url")

	// Credentials delete command flags
	vaultsCredentialsDeleteCmd.Flags().StringVar(&vaultCredentialsDeleteURL, "url", "", "URL to delete credentials for (required)")
	_ = vaultsCredentialsDeleteCmd.MarkFlagRequired("url")
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()

	var items []api.Vault
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No vaults found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return formatter.Print(items)
}

func runVaultsCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Build request body from generated flags
	body, err := BuildVaultCreateRequest(cmd)
	if err != nil {
		return err
	}

	params := &api.VaultCreateParams{}
	resp, err := client.Client().VaultCreateWithResponse(ctx, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}

func runVaultUpdate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.VaultUpdateJSONRequestBody{
		Name: vaultUpdateName,
	}

	params := &api.VaultUpdateParams{}
	resp, err := client.Client().VaultUpdateWithResponse(ctx, vaultID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runVaultDelete(cmd *cobra.Command, args []string) error {
	// Confirm before deletion
	confirmed, err := ConfirmAction("vault", vaultID)
	if err != nil {
		return err
	}
	if !confirmed {
		return PrintResult("Cancelled.", map[string]any{"cancelled": true})
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.VaultDeleteParams{}
	resp, err := client.Client().VaultDeleteWithResponse(ctx, vaultID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Vault %s deleted.", vaultID), map[string]any{
		"id":     vaultID,
		"status": "deleted",
	})
}

func runVaultCredentialsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.VaultCredentialsListParams{}
	resp, err := client.Client().VaultCredentialsListWithResponse(ctx, vaultID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()

	var creds []api.Credential
	if resp.JSON200 != nil {
		creds = resp.JSON200.Credentials
	}
	if printed, err := PrintListOrEmpty(creds, "No credentials found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return formatter.Print(creds)
}

func runVaultCredentialsAdd(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Validate URL format
	if _, err := url.Parse(VaultCredentialsAddUrl); err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate password not empty
	if strings.TrimSpace(VaultCredentialsAddCredentialsPassword) == "" {
		return fmt.Errorf("password cannot be empty or whitespace")
	}

	// Validate email format if provided
	if VaultCredentialsAddCredentialsEmail != "" {
		if _, err := mail.ParseAddress(VaultCredentialsAddCredentialsEmail); err != nil {
			return fmt.Errorf("invalid email format: %w", err)
		}
	}

	// Build request body from generated flags
	body, err := BuildVaultCredentialsAddRequest(cmd)
	if err != nil {
		return err
	}

	params := &api.VaultCredentialsAddParams{}
	resp, err := client.Client().VaultCredentialsAddWithResponse(ctx, vaultID, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runVaultCredentialsGet(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.VaultCredentialsGetParams{
		Url: vaultCredentialsGetURL,
	}

	resp, err := client.Client().VaultCredentialsGetWithResponse(ctx, vaultID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runVaultCredentialsDelete(cmd *cobra.Command, args []string) error {
	confirmed, err := ConfirmAction("credentials for", vaultCredentialsDeleteURL)
	if err != nil {
		return err
	}
	if !confirmed {
		return PrintResult("Cancelled.", map[string]any{"cancelled": true})
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.VaultCredentialsDeleteParams{
		Url: vaultCredentialsDeleteURL,
	}

	resp, err := client.Client().VaultCredentialsDeleteWithResponse(ctx, vaultID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Credentials for URL %s deleted from vault %s.", vaultCredentialsDeleteURL, vaultID), map[string]any{
		"id":  vaultID,
		"url": vaultCredentialsDeleteURL,
	})
}
