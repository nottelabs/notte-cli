package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var (
	sessionID                 string
	sessionObserveURL         string
	sessionExecuteAction      string
	sessionScrapeInstructions string
	sessionScrapeOnlyMain     bool
	sessionCookiesSetFile     string
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Operate on a specific session",
	Long:  "Subcommands for interacting with a specific browser session. Use --id flag to specify the session ID.",
}

var sessionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get session status",
	Args:  cobra.NoArgs,
	RunE:  runSessionStatus,
}

var sessionStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionStop,
}

var sessionObserveCmd = &cobra.Command{
	Use:   "observe",
	Short: "Observe page state and available actions",
	Args:  cobra.NoArgs,
	RunE:  runSessionObserve,
}

var sessionExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute an action on the page",
	Args:  cobra.NoArgs,
	RunE:  runSessionExecute,
}

var sessionScrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape content from the page",
	Args:  cobra.NoArgs,
	RunE:  runSessionScrape,
}

var sessionCookiesCmd = &cobra.Command{
	Use:   "cookies",
	Short: "Get all cookies for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionCookies,
}

var sessionCookiesSetCmd = &cobra.Command{
	Use:   "cookies-set",
	Short: "Set cookies from a JSON file",
	Args:  cobra.NoArgs,
	RunE:  runSessionCookiesSet,
}

var sessionDebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Get debug info for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionDebug,
}

var sessionNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "Get network logs for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionNetwork,
}

var sessionReplayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Get replay URL/data for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionReplay,
}

var sessionOffsetCmd = &cobra.Command{
	Use:   "offset",
	Short: "Get session offset info",
	Args:  cobra.NoArgs,
	RunE:  runSessionOffset,
}

var sessionWorkflowCodeCmd = &cobra.Command{
	Use:   "workflow-code",
	Short: "Export session steps as code",
	Args:  cobra.NoArgs,
	RunE:  runSessionWorkflowCode,
}

func init() {
	rootCmd.AddCommand(sessionCmd)

	sessionCmd.AddCommand(sessionStatusCmd)
	sessionCmd.AddCommand(sessionStopCmd)
	sessionCmd.AddCommand(sessionObserveCmd)
	sessionCmd.AddCommand(sessionExecuteCmd)
	sessionCmd.AddCommand(sessionScrapeCmd)
	sessionCmd.AddCommand(sessionCookiesCmd)
	sessionCmd.AddCommand(sessionCookiesSetCmd)
	sessionCmd.AddCommand(sessionDebugCmd)
	sessionCmd.AddCommand(sessionNetworkCmd)
	sessionCmd.AddCommand(sessionReplayCmd)
	sessionCmd.AddCommand(sessionOffsetCmd)
	sessionCmd.AddCommand(sessionWorkflowCodeCmd)

	// Persistent flag for session ID (required for all subcommands)
	sessionCmd.PersistentFlags().StringVar(&sessionID, "id", "", "Session ID (required)")
	_ = sessionCmd.MarkPersistentFlagRequired("id")

	sessionObserveCmd.Flags().StringVar(&sessionObserveURL, "url", "", "Navigate to URL before observing")

	sessionExecuteCmd.Flags().StringVar(&sessionExecuteAction, "action", "", "Action JSON (required)")
	_ = sessionExecuteCmd.MarkFlagRequired("action")

	sessionScrapeCmd.Flags().StringVar(&sessionScrapeInstructions, "instructions", "", "Extraction instructions")
	sessionScrapeCmd.Flags().BoolVar(&sessionScrapeOnlyMain, "only-main-content", false, "Only scrape main content")

	sessionCookiesSetCmd.Flags().StringVar(&sessionCookiesSetFile, "file", "", "JSON file containing cookies array (required)")
	_ = sessionCookiesSetCmd.MarkFlagRequired("file")
}

func runSessionStatus(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.SessionStatusParams{}
	resp, err := client.Client().SessionStatusWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionStop(cmd *cobra.Command, args []string) error {
	confirmed, err := ConfirmAction("session", sessionID)
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

	params := &api.SessionStopParams{}
	resp, err := client.Client().SessionStopWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Session %s stopped.\n", sessionID)
	return nil
}

func runSessionObserve(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.PageObserveJSONRequestBody{}
	if sessionObserveURL != "" {
		body.Url = &sessionObserveURL
	}

	params := &api.PageObserveParams{}
	resp, err := client.Client().PageObserveWithResponse(ctx, sessionID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionExecute(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Validate action JSON
	var actionData json.RawMessage
	if err := json.Unmarshal([]byte(sessionExecuteAction), &actionData); err != nil {
		return fmt.Errorf("invalid action JSON: %w", err)
	}

	params := &api.PageExecuteParams{}
	resp, err := client.Client().PageExecuteWithBodyWithResponse(ctx, sessionID, params, "application/json", bytes.NewReader(actionData))
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionScrape(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.PageScrapeJSONRequestBody{}
	if sessionScrapeInstructions != "" {
		body.Instructions = &sessionScrapeInstructions
	}
	if sessionScrapeOnlyMain {
		body.OnlyMainContent = &sessionScrapeOnlyMain
	}

	params := &api.PageScrapeParams{}
	resp, err := client.Client().PageScrapeWithResponse(ctx, sessionID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionCookies(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.SessionCookiesGetParams{}
	resp, err := client.Client().SessionCookiesGetWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionCookiesSet(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Read cookies from JSON file
	fileData, err := os.ReadFile(sessionCookiesSetFile)
	if err != nil {
		return fmt.Errorf("failed to read cookies file: %w", err)
	}

	// Parse the cookies JSON
	var body api.SessionCookiesSetJSONRequestBody
	if err := json.Unmarshal(fileData, &body); err != nil {
		return fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	params := &api.SessionCookiesSetParams{}
	resp, err := client.Client().SessionCookiesSetWithResponse(ctx, sessionID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionDebug(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.SessionDebugInfoParams{}
	resp, err := client.Client().SessionDebugInfoWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionNetwork(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.SessionNetworkLogsParams{}
	resp, err := client.Client().SessionNetworkLogsWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionReplay(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.SessionReplayParams{}
	resp, err := client.Client().SessionReplayWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	// Wrap raw body for formatter compatibility
	result := map[string]interface{}{
		"session_id":  sessionID,
		"replay_data": string(resp.Body),
	}
	return GetFormatter().Print(result)
}

func runSessionOffset(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.SessionOffsetParams{}
	resp, err := client.Client().SessionOffsetWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionWorkflowCode(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.GetSessionScriptParams{
		AsWorkflow: true,
	}
	resp, err := client.Client().GetSessionScriptWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}
