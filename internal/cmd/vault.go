package cmd

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

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

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Operate on a specific vault",
	Long:  "Subcommands for interacting with a specific vault. Use --id flag to specify the vault ID.",
}

var vaultUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update vault details",
	Args:  cobra.NoArgs,
	RunE:  runVaultUpdate,
}

var vaultDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultDelete,
}

var vaultCredentialsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Manage vault credentials",
	Long:  "List, add, get, and delete credentials stored in the vault.",
}

var vaultCredentialsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all credentials in the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsList,
}

var vaultCredentialsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add credentials to the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsAdd,
}

var vaultCredentialsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get credentials for a specific URL",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsGet,
}

var vaultCredentialsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete credentials for a specific URL",
	Args:  cobra.NoArgs,
	RunE:  runVaultCredentialsDelete,
}

var vaultCardCmd = &cobra.Command{
	Use:   "card",
	Short: "Get the credit card for the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCard,
}

var vaultCardSetCmd = &cobra.Command{
	Use:   "card-set",
	Short: "Set credit card for the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCardSet,
}

var vaultCardDeleteCmd = &cobra.Command{
	Use:   "card-delete",
	Short: "Delete the credit card from the vault",
	Args:  cobra.NoArgs,
	RunE:  runVaultCardDelete,
}

func init() {
	rootCmd.AddCommand(vaultCmd)

	vaultCmd.AddCommand(vaultUpdateCmd)
	vaultCmd.AddCommand(vaultDeleteCmd)
	vaultCmd.AddCommand(vaultCredentialsCmd)
	vaultCmd.AddCommand(vaultCardCmd)
	vaultCmd.AddCommand(vaultCardSetCmd)
	vaultCmd.AddCommand(vaultCardDeleteCmd)

	vaultCredentialsCmd.AddCommand(vaultCredentialsListCmd)
	vaultCredentialsCmd.AddCommand(vaultCredentialsAddCmd)
	vaultCredentialsCmd.AddCommand(vaultCredentialsGetCmd)
	vaultCredentialsCmd.AddCommand(vaultCredentialsDeleteCmd)

	// Persistent flag for vault ID (required for all subcommands)
	vaultCmd.PersistentFlags().StringVar(&vaultID, "id", "", "Vault ID (required)")
	_ = vaultCmd.MarkPersistentFlagRequired("id")

	// Update command flags
	vaultUpdateCmd.Flags().StringVar(&vaultUpdateName, "name", "", "New name for the vault (required)")
	_ = vaultUpdateCmd.MarkFlagRequired("name")

	// Credentials add command flags
	vaultCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddURL, "url", "", "URL for the credentials (required)")
	vaultCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddEmail, "email", "", "Email for the credentials")
	vaultCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddUsername, "username", "", "Username for the credentials")
	vaultCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddPassword, "password", "", "Password for the credentials (required)")
	vaultCredentialsAddCmd.Flags().StringVar(&vaultCredentialsAddMFA, "mfa-secret", "", "MFA secret for the credentials")
	_ = vaultCredentialsAddCmd.MarkFlagRequired("url")
	_ = vaultCredentialsAddCmd.MarkFlagRequired("password")

	// Credentials get command flags
	vaultCredentialsGetCmd.Flags().StringVar(&vaultCredentialsGetURL, "url", "", "URL to get credentials for (required)")
	_ = vaultCredentialsGetCmd.MarkFlagRequired("url")

	// Credentials delete command flags
	vaultCredentialsDeleteCmd.Flags().StringVar(&vaultCredentialsDeleteURL, "url", "", "URL to delete credentials for (required)")
	_ = vaultCredentialsDeleteCmd.MarkFlagRequired("url")

	// Card set command flags
	vaultCardSetCmd.Flags().StringVar(&vaultCardSetNumber, "number", "", "Credit card number (required)")
	vaultCardSetCmd.Flags().StringVar(&vaultCardSetExpiry, "expiry", "", "Card expiration date (e.g., 12/25) (required)")
	vaultCardSetCmd.Flags().StringVar(&vaultCardSetCVV, "cvv", "", "Card CVV (required)")
	vaultCardSetCmd.Flags().StringVar(&vaultCardSetName, "name", "", "Cardholder name (required)")
	_ = vaultCardSetCmd.MarkFlagRequired("number")
	_ = vaultCardSetCmd.MarkFlagRequired("expiry")
	_ = vaultCardSetCmd.MarkFlagRequired("cvv")
	_ = vaultCardSetCmd.MarkFlagRequired("name")
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
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
		fmt.Println("Cancelled.")
		return nil
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Vault %s deleted.\n", vaultID)
	return nil
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()

	if resp.JSON200 == nil || len(resp.JSON200.Credentials) == 0 {
		fmt.Println("No credentials found.")
		return nil
	}

	return formatter.Print(resp.JSON200.Credentials)
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
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
		fmt.Println("Cancelled.")
		return nil
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Credentials for URL %s deleted from vault %s.\n", vaultCredentialsDeleteURL, vaultID)
	return nil
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
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
		fmt.Println("Cancelled.")
		return nil
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Credit card deleted from vault %s.\n", vaultID)
	return nil
}
