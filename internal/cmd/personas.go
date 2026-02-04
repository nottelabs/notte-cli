package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var personaID string

var personasCmd = &cobra.Command{
	Use:   "personas",
	Short: "Manage personas",
	Long:  "List, create, and operate on personas.",
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

var personasShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show persona details",
	Args:  cobra.NoArgs,
	RunE:  runPersonaShow,
}

var personasDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaDelete,
}

var personasEmailsCmd = &cobra.Command{
	Use:   "emails",
	Short: "List emails for the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaEmails,
}

var personasSmsCmd = &cobra.Command{
	Use:   "sms",
	Short: "List SMS messages for the persona",
	Args:  cobra.NoArgs,
	RunE:  runPersonaSms,
}

func init() {
	rootCmd.AddCommand(personasCmd)
	personasCmd.AddCommand(personasListCmd)
	personasCmd.AddCommand(personasCreateCmd)
	personasCmd.AddCommand(personasShowCmd)
	personasCmd.AddCommand(personasDeleteCmd)
	personasCmd.AddCommand(personasEmailsCmd)
	personasCmd.AddCommand(personasSmsCmd)

	// Create command flags (auto-generated)
	RegisterPersonaCreateFlags(personasCreateCmd)

	// Show command flags
	personasShowCmd.Flags().StringVar(&personaID, "persona-id", "", "Persona ID (required)")
	_ = personasShowCmd.MarkFlagRequired("persona-id")

	// Delete command flags
	personasDeleteCmd.Flags().StringVar(&personaID, "persona-id", "", "Persona ID (required)")
	_ = personasDeleteCmd.MarkFlagRequired("persona-id")

	// Emails command flags
	personasEmailsCmd.Flags().StringVar(&personaID, "persona-id", "", "Persona ID (required)")
	_ = personasEmailsCmd.MarkFlagRequired("persona-id")

	// SMS command flags
	personasSmsCmd.Flags().StringVar(&personaID, "persona-id", "", "Persona ID (required)")
	_ = personasSmsCmd.MarkFlagRequired("persona-id")
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()

	var items []api.PersonaResponse
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No personas found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return formatter.Print(items)
}

func runPersonasCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Build request body from generated flags
	body, err := BuildPersonaCreateRequest(cmd)
	if err != nil {
		return err
	}

	params := &api.PersonaCreateParams{}
	resp, err := client.Client().PersonaCreateWithResponse(ctx, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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
		return PrintResult("Cancelled.", map[string]any{"cancelled": true})
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Persona %s deleted.", personaID), map[string]any{
		"id":     personaID,
		"status": "deleted",
	})
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}
