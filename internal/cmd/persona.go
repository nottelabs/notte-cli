package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var personaID string

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Operate on a specific persona",
	Long:  "Subcommands for interacting with a specific persona. Use --id flag to specify the persona ID.",
}

var personaShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show persona details",
	Args:  cobra.NoArgs,
	RunE:  runPersonaShow,
}

var personaDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaDelete,
}

var personaEmailsCmd = &cobra.Command{
	Use:   "emails",
	Short: "List emails for the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaEmails,
}

var personaSmsCmd = &cobra.Command{
	Use:   "sms",
	Short: "List SMS messages for the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaSms,
}

var personaPhoneCreateCmd = &cobra.Command{
	Use:   "phone-create",
	Short: "Create a new phone number for the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaPhoneCreate,
}

var personaPhoneDeleteCmd = &cobra.Command{
	Use:   "phone-delete",
	Short: "Delete the phone number for the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaPhoneDelete,
}

func init() {
	rootCmd.AddCommand(personaCmd)

	personaCmd.AddCommand(personaShowCmd)
	personaCmd.AddCommand(personaDeleteCmd)
	personaCmd.AddCommand(personaEmailsCmd)
	personaCmd.AddCommand(personaSmsCmd)
	personaCmd.AddCommand(personaPhoneCreateCmd)
	personaCmd.AddCommand(personaPhoneDeleteCmd)

	// Persistent flag for persona ID (required for all subcommands)
	personaCmd.PersistentFlags().StringVar(&personaID, "id", "", "Persona ID (required)")
	_ = personaCmd.MarkPersistentFlagRequired("id")
}

func runPersonaShow(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.PersonaGetParams{}
	resp, err := client.Client().PersonaGetWithResponse(ctx, personaID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runPersonaDelete(cmd *cobra.Command, args []string) error {
	confirmed, err := ConfirmAction("persona", personaID)
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

	params := &api.PersonaDeleteParams{}
	resp, err := client.Client().PersonaDeleteWithResponse(ctx, personaID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Persona %s deleted.\n", personaID)
	return nil
}

func runPersonaEmails(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.PersonaEmailsListParams{}
	resp, err := client.Client().PersonaEmailsListWithResponse(ctx, personaID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runPersonaSms(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.PersonaSmsListParams{}
	resp, err := client.Client().PersonaSmsListWithResponse(ctx, personaID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runPersonaPhoneCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.PersonaCreateNumberParams{}
	resp, err := client.Client().PersonaCreateNumberWithResponse(ctx, personaID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runPersonaPhoneDelete(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.PersonaDeleteNumberParams{}
	resp, err := client.Client().PersonaDeleteNumberWithResponse(ctx, personaID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}
