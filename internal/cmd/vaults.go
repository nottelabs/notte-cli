package cmd

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var vaultsCreateName string

var (
	vaultID                     string
	vaultUpdateName             string
	vaultCredentialsAddURL      string
	vaultCredentialsAddEmail    string
	vaultCredentialsAddUsername string
	vaultCredentialsAddPassword string
	vaultCredentialsAddMFA      string
	vaultCredentialsGetURL      string
	vaultCredentialsDeleteURL   string
	vaultCardSetNumber          string
	vaultCardSetExpiry          string
	vaultCardSetCVV             string
	vaultCardSetName            string
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

var vaultsCardCmd = &cobra.Command{
	Use:   "card",
	Short: "Get the credit card for the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCard,
}

var vaultsCardSetCmd = &cobra.Command{
	Use:   "card-set",
	Short: "Set credit card for the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCardSet,
}

var vaultsCardDeleteCmd = &cobra.Command{
	Use:   "card-delete",
	Short: "Delete the credit card from the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCardDelete,
}

func init() {
	rootCmd.AddCommand(vaultsCmd)
	vaultsCmd.AddCommand(vaultsListCmd)
	vaultsCmd.AddCommand(vaultsCreateCmd)
	vaultsCmd.AddCommand(vaultsUpdateCmd)
	vaultsCmd.AddCommand(vaultsDeleteCmd)
	vaultsCmd.AddCommand(vaultsCredentialsCmd)
	vaultsCmd.AddCommand(vaultsCardCmd)
	vaultsCmd.AddCommand(vaultsCardSetCmd)
	vaultsCmd.AddCommand(vaultsCardDeleteCmd)

	vaultsCredentialsCmd.AddCommand(vaultsCredentialsListCmd)
	vaultsCredentialsCmd.AddCommand(vaultsCredentialsAddCmd)
	vaultsCredentialsCmd.AddCommand(vaultsCredentialsGetCmd)
	vaultsCredentialsCmd.AddCommand(vaultsCredentialsDeleteCmd)

	// Create command flags
	vaultsCreateCmd.Flags().StringVar(&vaultsCreateName, "name", "", "Name of the vault")

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

	// Credentials add command flags
	vaultsCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddURL, "url", "", "URL for the credentials (required)")
	vaultsCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddEmail, "email", "", "Email for the credentials")
	vaultsCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddUsername, "username", "", "Username for the credentials")
	vaultsCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddPassword, "password", "", "Password for the credentials (required)")
	vaultsCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddMFA, "mfa-secret", "", "MFA secret for the credentials")
	_ = vaultsCredentialsAddCmd.MarkFlagRequired("url")
	_ = vaultsCredentialsAddCmd.MarkFlagRequired("password")

	// Credentials get command flags
	vaultsCredentialsGetCmd.Flags().StringVar(&vaultCredentialsGetURL, "url", "", "URL to get credentials for (required)")
	_ = vaultsCredentialsGetCmd.MarkFlagRequired("url")

	// Credentials delete command flags
	vaultsCredentialsDeleteCmd.Flags().StringVar(&vaultCredentialsDeleteURL, "url", "", "URL to delete credentials for (required)")
	_ = vaultsCredentialsDeleteCmd.MarkFlagRequired("url")

	// Card command flags
	vaultsCardCmd.Flags().StringVar(&vaultID, "id", "", "Vault ID (required)")
	_ = vaultsCardCmd.MarkFlagRequired("id")

	// Card-set command flags
	vaultsCardSetCmd.Flags().StringVar(&vaultID, "id", "", "Vault ID (required)")
	_ = vaultsCardSetCmd.MarkFlagRequired("id")
	vaultsCardSetCmd.Flags().StringVar(&vaultCardSetNumber, "number", "", "Credit card number (required)")
	vaultsCardSetCmd.Flags().StringVar(&vaultCardSetExpiry, "expiry", "", "Card expiration date (e.g., 12/25) (required)")
	vaultsCardSetCmd.Flags().StringVar(&vaultCardSetCVV, "cvv", "", "Card CVV (required)")
	vaultsCardSetCmd.Flags().StringVar(&vaultCardSetName, "name", "", "Cardholder name (required)")
	_ = vaultsCardSetCmd.MarkFlagRequired("number")
	_ = vaultsCardSetCmd.MarkFlagRequired("expiry")
	_ = vaultsCardSetCmd.MarkFlagRequired("cvv")
	_ = vaultsCardSetCmd.MarkFlagRequired("name")

	// Card-delete command flags
	vaultsCardDeleteCmd.Flags().StringVar(&vaultID, "id", "", "Vault ID (required)")
	_ = vaultsCardDeleteCmd.MarkFlagRequired("id")
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
	if _, err := url.Parse(vaultCredentialsAddURL); err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate password not empty
	if strings.TrimSpace(vaultCredentialsAddPassword) == "" {
		return fmt.Errorf("password cannot be empty or whitespace")
	}

	// Validate email format if provided
	if vaultCredentialsAddEmail != "" {
		if _, err := mail.ParseAddress(vaultCredentialsAddEmail); err != nil {
			return fmt.Errorf("invalid email format: %w", err)
		}
	}

	// Build credentials object
	credentials := api.CredentialsDictInput{
		Password: vaultCredentialsAddPassword,
	}

	if vaultCredentialsAddEmail != "" {
		credentials.Email = &vaultCredentialsAddEmail
	}

	if vaultCredentialsAddUsername != "" {
		credentials.Username = &vaultCredentialsAddUsername
	}

	if vaultCredentialsAddMFA != "" {
		credentials.MfaSecret = &vaultCredentialsAddMFA
	}

	body := api.VaultCredentialsAddJSONRequestBody{
		Url:         vaultCredentialsAddURL,
		Credentials: credentials,
	}

	params := &api.VaultCredentialsAddParams{}
	resp, err := client.Client().VaultCredentialsAddWithResponse(ctx, vaultID, params, body)
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

func runVaultCard(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.VaultCreditCardGetParams{}
	resp, err := client.Client().VaultCreditCardGetWithResponse(ctx, vaultID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runVaultCardSet(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Validate required fields are not empty
	if strings.TrimSpace(vaultCardSetNumber) == "" {
		return fmt.Errorf("card number cannot be empty")
	}
	if strings.TrimSpace(vaultCardSetExpiry) == "" {
		return fmt.Errorf("card expiry cannot be empty")
	}
	if strings.TrimSpace(vaultCardSetCVV) == "" {
		return fmt.Errorf("card CVV cannot be empty")
	}
	if strings.TrimSpace(vaultCardSetName) == "" {
		return fmt.Errorf("cardholder name cannot be empty")
	}

	body := api.VaultCreditCardSetJSONRequestBody{
		CreditCard: api.CreditCardDictInput{
			CardNumber:         vaultCardSetNumber,
			CardFullExpiration: vaultCardSetExpiry,
			CardCvv:            vaultCardSetCVV,
			CardHolderName:     vaultCardSetName,
		},
	}

	params := &api.VaultCreditCardSetParams{}
	resp, err := client.Client().VaultCreditCardSetWithResponse(ctx, vaultID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runVaultCardDelete(cmd *cobra.Command, args []string) error {
	confirmed, err := ConfirmAction("credit card from vault", vaultID)
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

	params := &api.VaultCreditCardDeleteParams{}
	resp, err := client.Client().VaultCreditCardDeleteWithResponse(ctx, vaultID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Credit card deleted from vault %s.", vaultID), map[string]any{
		"id":     vaultID,
		"status": "deleted",
	})
}
