package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var (
	profilesCreateName string
	profileID          string
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage browser profiles",
	Long:  "List, create, and operate on browser profiles.",
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	RunE:  runProfilesList,
}

var profilesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new profile",
	RunE:  runProfilesCreate,
}

var profilesShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show profile details",
	Args:  cobra.NoArgs,
	RunE:  runProfileShow,
}

var profilesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete profile",
	Args:  cobra.NoArgs,
	RunE:  runProfileDelete,
}

func init() {
	rootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesCreateCmd)
	profilesCmd.AddCommand(profilesShowCmd)
	profilesCmd.AddCommand(profilesDeleteCmd)

	// Create command flags
	profilesCreateCmd.Flags().StringVar(&profilesCreateName, "name", "", "Profile name")

	// Show command flags
	profilesShowCmd.Flags().StringVar(&profileID, "id", "", "Profile ID (required)")
	_ = profilesShowCmd.MarkFlagRequired("id")

	// Delete command flags
	profilesDeleteCmd.Flags().StringVar(&profileID, "id", "", "Profile ID (required)")
	_ = profilesDeleteCmd.MarkFlagRequired("id")
}

func runProfilesList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ProfileListParams{}
	resp, err := client.Client().ProfileListWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()

	var items []api.ProfileResponse
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No profiles found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return formatter.Print(items)
}

func runProfilesCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.ProfileCreateJSONRequestBody{}
	if profilesCreateName != "" {
		body.Name = &profilesCreateName
	}

	params := &api.ProfileCreateParams{}
	resp, err := client.Client().ProfileCreateWithResponse(ctx, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ProfileGetParams{}
	resp, err := client.Client().ProfileGetWithResponse(ctx, profileID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	confirmed, err := ConfirmAction("profile", profileID)
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

	params := &api.ProfileDeleteParams{}
	resp, err := client.Client().ProfileDeleteWithResponse(ctx, profileID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Profile %s deleted.", profileID), map[string]any{
		"id":     profileID,
		"status": "deleted",
	})
}
