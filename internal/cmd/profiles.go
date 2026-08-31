package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var (
	profileID            string
	profileCookiesFile   string
	profileCookiesFormat string
	profileCookiesMode   string
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

var profilesCookiesCmd = &cobra.Command{
	Use:   "cookies",
	Short: "Get the cookies stored in a profile",
	Args:  cobra.NoArgs,
	RunE:  runProfileCookies,
}

var profilesCookiesSetCmd = &cobra.Command{
	Use:   "cookies-set",
	Short: "Import cookies into a profile",
	Long: "Import cookies into a profile from a JSON file.\n\n" +
		"The file may be a bare array of cookies, which is what Playwright and the\n" +
		"Chrome extensions export, or an object with a `cookies` key.",
	Args: cobra.NoArgs,
	RunE: runProfileCookiesSet,
}

func init() {
	rootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(profilesListCmd)
	registerPaginationFlags(profilesListCmd)
	registerFilterFlag(profilesListCmd, flagIncludeDeleted, "", "Include deleted profiles")
	profilesListCmd.Flags().String("name", "", "Filter profiles by name")

	profilesCmd.AddCommand(profilesCreateCmd)
	profilesCmd.AddCommand(profilesShowCmd)
	profilesCmd.AddCommand(profilesDeleteCmd)

	// Create command flags (auto-generated)
	RegisterProfileCreateFlags(profilesCreateCmd)

	// Show command flags
	profilesShowCmd.Flags().StringVar(&profileID, "profile-id", "", "Profile ID (required)")
	_ = profilesShowCmd.MarkFlagRequired("profile-id")

	// Delete command flags
	profilesDeleteCmd.Flags().StringVar(&profileID, "profile-id", "", "Profile ID (required)")
	_ = profilesDeleteCmd.MarkFlagRequired("profile-id")

	// Cookies command flags
	profilesCmd.AddCommand(profilesCookiesCmd)
	profilesCookiesCmd.Flags().StringVar(&profileID, "profile-id", "", "Profile ID (required)")
	_ = profilesCookiesCmd.MarkFlagRequired("profile-id")

	profilesCmd.AddCommand(profilesCookiesSetCmd)
	profilesCookiesSetCmd.Flags().StringVar(&profileID, "profile-id", "", "Profile ID (required)")
	_ = profilesCookiesSetCmd.MarkFlagRequired("profile-id")
	profilesCookiesSetCmd.Flags().StringVar(&profileCookiesFile, "file", "", "Path to a cookies JSON file (required)")
	_ = profilesCookiesSetCmd.MarkFlagRequired("file")
	profilesCookiesSetCmd.Flags().StringVar(&profileCookiesFormat, "source-format", "", "Format the cookies were exported in (playwright, chrome)")
	profilesCookiesSetCmd.Flags().StringVar(&profileCookiesMode, "mode", "", "replace the profile's cookies, or append to them")
}

func runProfilesList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	page, err := getPageFlag(cmd)
	if err != nil {
		return err
	}
	pageSize, err := getPageSizeFlag(cmd)
	if err != nil {
		return err
	}
	params := &api.ProfileListParams{
		Page:     page,
		PageSize: pageSize,
	}
	params.OnlyActive = resolveOnlyActive(cmd, flagIncludeDeleted, true)
	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		params.Name = &v
	}
	resp, err := client.Client().ProfileListWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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

	// Build request body from generated flags
	body, err := BuildProfileCreateRequest(cmd)
	if err != nil {
		return err
	}

	params := &api.ProfileCreateParams{}
	resp, err := client.Client().ProfileCreateWithResponse(ctx, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Profile %s deleted.", profileID), map[string]any{
		"id":     profileID,
		"status": "deleted",
	})
}

func runProfileCookies(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ProfileCookiesGetParams{}
	resp, err := client.Client().ProfileCookiesGetWithResponse(ctx, profileID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runProfileCookiesSet(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	fileData, err := os.ReadFile(profileCookiesFile)
	if err != nil {
		return fmt.Errorf("failed to read cookies file: %w", err)
	}

	body, err := parseProfileCookies(fileData)
	if err != nil {
		return err
	}
	if profileCookiesFormat != "" {
		format := api.ProfileCookiesImportRequestSourceFormat(profileCookiesFormat)
		body.SourceFormat = &format
	}
	if profileCookiesMode != "" {
		mode := api.ProfileCookiesImportRequestMode(profileCookiesMode)
		body.Mode = &mode
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ProfileCookiesSetParams{}
	resp, err := client.Client().ProfileCookiesSetWithResponse(ctx, profileID, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

// parseProfileCookies accepts either shape a cookies file comes in.
//
// Playwright's storageState and the browser extensions people use both write a
// bare array, while the API wants it under a `cookies` key. Requiring the
// caller to reshape their own export first would be a papercut for no reason,
// so both are read here.
func parseProfileCookies(data []byte) (*api.ProfileCookiesImportRequest, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("cookies file %s is empty", profileCookiesFile)
	}

	if trimmed[0] == '[' {
		var cookies []api.Cookie
		if err := json.Unmarshal(trimmed, &cookies); err != nil {
			return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
		}
		return &api.ProfileCookiesImportRequest{Cookies: cookies}, nil
	}

	var body api.ProfileCookiesImportRequest
	if err := json.Unmarshal(trimmed, &body); err != nil {
		return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}
	if len(body.Cookies) == 0 {
		return nil, fmt.Errorf("cookies file %s has no cookies: expected an array, or an object with a \"cookies\" key", profileCookiesFile)
	}
	return &body, nil
}
