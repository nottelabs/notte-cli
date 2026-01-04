package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var (
	personasCreatePhoneNumber bool
	personasCreateVault       bool
)

var personasCmd = &cobra.Command{
	Use:   "personas",
	Short: "Manage personas",
	Long:  "List and create personas.",
}

var personasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all personas",
	RunE:  runPersonasList,
}

var personasCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new persona",
	RunE:  runPersonasCreate,
}

func init() {
	rootCmd.AddCommand(personasCmd)
	personasCmd.AddCommand(personasListCmd)
	personasCmd.AddCommand(personasCreateCmd)

	personasCreateCmd.Flags().BoolVar(&personasCreatePhoneNumber, "create-phone-number", false, "Create a phone number for the persona")
	personasCreateCmd.Flags().BoolVar(&personasCreateVault, "create-vault", false, "Create a vault for the persona")
}

func runPersonasList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListPersonasParams{}
	resp, err := client.Client().ListPersonasWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()

	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		fmt.Println("No personas found.")
		return nil
	}

	return formatter.Print(resp.JSON200.Items)
}

func runPersonasCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Build request body from flags
	body := api.PersonaCreateJSONRequestBody{}

	// Set create phone number if flag was provided
	if cmd.Flags().Changed("create-phone-number") {
		body.CreatePhoneNumber = &personasCreatePhoneNumber
	}

	// Set create vault if flag was provided
	if cmd.Flags().Changed("create-vault") {
		body.CreateVault = &personasCreateVault
	}

	params := &api.PersonaCreateParams{}
	resp, err := client.Client().PersonaCreateWithResponse(ctx, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}
