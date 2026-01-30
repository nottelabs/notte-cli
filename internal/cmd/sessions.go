package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
)

var (
	sessionsStartHeadless      bool
	sessionsStartBrowser       string
	sessionsStartIdleTimeout   int
	sessionsStartMaxDuration   int
	sessionsStartProxies       bool
	sessionsStartSolveCaptchas bool
	sessionsStartViewportW     int
	sessionsStartViewportH     int
	sessionsStartUserAgent     string
	sessionsStartCdpURL        string
	sessionsStartFileStorage   bool
)

var (
	sessionID                 string
	sessionObserveURL         string
	sessionExecuteAction      string
	sessionScrapeInstructions string
	sessionScrapeOnlyMain     bool
	sessionCookiesSetFile     string
)

// GetCurrentSessionID returns the session ID from flag, env var, or file (in priority order)
func GetCurrentSessionID() string {
	// 1. Check --id flag (already in sessionID variable if set)
	if sessionID != "" {
		return sessionID
	}

	// 2. Check NOTTE_SESSION_ID env var
	if envID := os.Getenv(config.EnvSessionID); envID != "" {
		return envID
	}

	// 3. Check current_session file
	configDir, err := config.Dir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, config.CurrentSessionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// setCurrentSession saves the session ID to the current_session file
func setCurrentSession(id string) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, config.CurrentSessionFile), []byte(id), 0o600)
}

// clearCurrentSession removes the current_session file
func clearCurrentSession() error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RequireSessionID ensures a session ID is available from flag, env, or file
func RequireSessionID() error {
	sessionID = GetCurrentSessionID()
	if sessionID == "" {
		return errors.New("session ID required: use --id flag, set NOTTE_SESSION_ID env var, or start a session first")
	}
	return nil
}

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage browser sessions",
	Long:  "List, create, and operate on browser sessions.",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active sessions",
	RunE:  runSessionsList,
}

var sessionsStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new browser session",
	RunE:  runSessionsStart,
}

var sessionsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get session status",
	Args:  cobra.NoArgs,
	RunE:  runSessionStatus,
}

var sessionsStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionStop,
}

var sessionsObserveCmd = &cobra.Command{
	Use:   "observe",
	Short: "Observe page state and available actions",
	Args:  cobra.NoArgs,
	RunE:  runSessionObserve,
}

var sessionsExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute an action on the page",
	Args:  cobra.NoArgs,
	Example: `  # Direct JSON
  notte sessions execute --id <session-id> --action '{"type": "goto", "url": "https://example.com"}'

  # From file
  notte sessions execute --id <session-id> --action @action.json

  # From stdin
  echo '{"type": "goto", "url": "https://example.com"}' | notte sessions execute --id <session-id>`,
	RunE: runSessionExecute,
}

var sessionsScrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape content from the page",
	Args:  cobra.NoArgs,
	RunE:  runSessionScrape,
}

var sessionsCookiesCmd = &cobra.Command{
	Use:   "cookies",
	Short: "Get all cookies for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionCookies,
}

var sessionsCookiesSetCmd = &cobra.Command{
	Use:   "cookies-set",
	Short: "Set cookies from a JSON file",
	Args:  cobra.NoArgs,
	RunE:  runSessionCookiesSet,
}

var sessionsDebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Get debug info for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionDebug,
}

var sessionsNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "Get network logs for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionNetwork,
}

var sessionsReplayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Get replay URL/data for the session",
	Args:  cobra.NoArgs,
	RunE:  runSessionReplay,
}

var sessionsOffsetCmd = &cobra.Command{
	Use:   "offset",
	Short: "Get session offset info",
	Args:  cobra.NoArgs,
	RunE:  runSessionOffset,
}

var sessionsWorkflowCodeCmd = &cobra.Command{
	Use:   "workflow-code",
	Short: "Export session steps as code",
	Args:  cobra.NoArgs,
	RunE:  runSessionWorkflowCode,
}

var sessionsCodeCmd = &cobra.Command{
	Use:   "code",
	Short: "Get Python script for session steps",
	Args:  cobra.NoArgs,
	RunE:  runSessionCode,
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsStartCmd)
	sessionsCmd.AddCommand(sessionsStatusCmd)
	sessionsCmd.AddCommand(sessionsStopCmd)
	sessionsCmd.AddCommand(sessionsObserveCmd)
	sessionsCmd.AddCommand(sessionsExecuteCmd)
	sessionsCmd.AddCommand(sessionsScrapeCmd)
	sessionsCmd.AddCommand(sessionsCookiesCmd)
	sessionsCmd.AddCommand(sessionsCookiesSetCmd)
	sessionsCmd.AddCommand(sessionsDebugCmd)
	sessionsCmd.AddCommand(sessionsNetworkCmd)
	sessionsCmd.AddCommand(sessionsReplayCmd)
	sessionsCmd.AddCommand(sessionsOffsetCmd)
	sessionsCmd.AddCommand(sessionsWorkflowCodeCmd)
	sessionsCmd.AddCommand(sessionsCodeCmd)

	// Start command flags
	sessionsStartCmd.Flags().BoolVar(&sessionsStartHeadless, "headless", true, "Run session in headless mode")
	sessionsStartCmd.Flags().StringVar(&sessionsStartBrowser, "browser", "chromium", "Browser type (chromium, chrome, firefox)")
	sessionsStartCmd.Flags().IntVar(&sessionsStartIdleTimeout, "idle-timeout", 0, "Idle timeout in minutes (session closes after this period of inactivity)")
	sessionsStartCmd.Flags().IntVar(&sessionsStartMaxDuration, "max-duration", 0, "Maximum session lifetime in minutes (absolute maximum, not affected by activity)")
	sessionsStartCmd.Flags().BoolVar(&sessionsStartProxies, "proxies", false, "Use default proxies")
	sessionsStartCmd.Flags().BoolVar(&sessionsStartSolveCaptchas, "solve-captchas", false, "Automatically solve captchas")
	sessionsStartCmd.Flags().IntVar(&sessionsStartViewportW, "viewport-width", 0, "Viewport width in pixels")
	sessionsStartCmd.Flags().IntVar(&sessionsStartViewportH, "viewport-height", 0, "Viewport height in pixels")
	sessionsStartCmd.Flags().StringVar(&sessionsStartUserAgent, "user-agent", "", "Custom user agent string")
	sessionsStartCmd.Flags().StringVar(&sessionsStartCdpURL, "cdp-url", "", "CDP URL of remote session provider")
	sessionsStartCmd.Flags().BoolVar(&sessionsStartFileStorage, "file-storage", false, "Enable file storage for the session")

	// Status command flags
	sessionsStatusCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Stop command flags
	sessionsStopCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Observe command flags
	sessionsObserveCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")
	sessionsObserveCmd.Flags().StringVar(&sessionObserveURL, "url", "", "Navigate to URL before observing")

	// Execute command flags
	sessionsExecuteCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")
	sessionsExecuteCmd.Flags().StringVar(&sessionExecuteAction, "action", "", "Action JSON, @file, or '-' for stdin")

	// Scrape command flags
	sessionsScrapeCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")
	sessionsScrapeCmd.Flags().StringVar(&sessionScrapeInstructions, "instructions", "", "Extraction instructions")
	sessionsScrapeCmd.Flags().BoolVar(&sessionScrapeOnlyMain, "only-main-content", false, "Only scrape main content")

	// Cookies command flags
	sessionsCookiesCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Cookies-set command flags
	sessionsCookiesSetCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")
	sessionsCookiesSetCmd.Flags().StringVar(&sessionCookiesSetFile, "file", "", "JSON file containing cookies array (required)")
	_ = sessionsCookiesSetCmd.MarkFlagRequired("file")

	// Debug command flags
	sessionsDebugCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Network command flags
	sessionsNetworkCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Replay command flags
	sessionsReplayCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Offset command flags
	sessionsOffsetCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Workflow-code command flags
	sessionsWorkflowCodeCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// Code command flags
	sessionsCodeCmd.Flags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListSessionsParams{}
	resp, err := client.Client().ListSessionsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()

	var items []api.SessionResponse
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No active sessions."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return formatter.Print(items)
}

func runSessionsStart(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Build request body from flags
	body := api.SessionStartJSONRequestBody{}

	// Set headless if flag was provided
	if cmd.Flags().Changed("headless") {
		body.Headless = &sessionsStartHeadless
	}

	// Set browser type if provided
	if sessionsStartBrowser != "" {
		browserType := api.ApiSessionStartRequestBrowserType(sessionsStartBrowser)
		body.BrowserType = &browserType
	}

	// Set idle timeout if provided
	if sessionsStartIdleTimeout > 0 {
		body.IdleTimeoutMinutes = &sessionsStartIdleTimeout
	}

	// Set max duration if provided
	if sessionsStartMaxDuration > 0 {
		body.MaxDurationMinutes = &sessionsStartMaxDuration
	}

	// Set proxies if flag was provided
	if cmd.Flags().Changed("proxies") {
		var proxies api.ApiSessionStartRequest_Proxies
		if err := proxies.FromApiSessionStartRequestProxies1(sessionsStartProxies); err != nil {
			return fmt.Errorf("failed to set proxies: %w", err)
		}
		body.Proxies = &proxies
	}

	// Set solve captchas if flag was provided
	if cmd.Flags().Changed("solve-captchas") {
		body.SolveCaptchas = &sessionsStartSolveCaptchas
	}

	// Set viewport dimensions if provided
	if sessionsStartViewportW > 0 {
		body.ViewportWidth = &sessionsStartViewportW
	}
	if sessionsStartViewportH > 0 {
		body.ViewportHeight = &sessionsStartViewportH
	}

	// Set user agent if provided
	if sessionsStartUserAgent != "" {
		body.UserAgent = &sessionsStartUserAgent
	}

	// Set CDP URL if provided
	if sessionsStartCdpURL != "" {
		body.CdpUrl = &sessionsStartCdpURL
	}

	// Set file storage if flag was provided
	if cmd.Flags().Changed("file-storage") {
		body.UseFileStorage = &sessionsStartFileStorage
	}

	params := &api.SessionStartParams{}
	resp, err := client.Client().SessionStartWithResponse(ctx, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// Save session ID as current session
	if resp.JSON200 != nil {
		if err := setCurrentSession(resp.JSON200.SessionId); err != nil {
			PrintInfo(fmt.Sprintf("Warning: could not save current session: %v", err))
		}
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}

func runSessionStatus(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionStop(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	confirmed, err := ConfirmAction("session", sessionID)
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

	params := &api.SessionStopParams{}
	resp, err := client.Client().SessionStopWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// Clear current session only if it matches the stopped session
	configDir, _ := config.Dir()
	if configDir != "" {
		data, _ := os.ReadFile(filepath.Join(configDir, config.CurrentSessionFile))
		if strings.TrimSpace(string(data)) == sessionID {
			_ = clearCurrentSession()
		}
	}

	return PrintResult(fmt.Sprintf("Session %s stopped.", sessionID), map[string]any{
		"id":     sessionID,
		"status": "stopped",
	})
}

func runSessionObserve(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionExecute(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	actionPayload, err := readJSONInput(cmd, sessionExecuteAction, "action")
	if err != nil {
		return err
	}

	// Validate action JSON
	var actionData json.RawMessage
	if err := json.Unmarshal(actionPayload, &actionData); err != nil {
		return fmt.Errorf("invalid action JSON: %w", err)
	}

	params := &api.PageExecuteParams{}
	resp, err := client.Client().PageExecuteWithBodyWithResponse(ctx, sessionID, params, "application/json", bytes.NewReader(actionData))
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionScrape(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.PageScrapeJSONRequestBody{}
	hasInstructions := sessionScrapeInstructions != ""
	if hasInstructions {
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintScrapeResponse(resp.JSON200, hasInstructions)
}

func runSessionCookies(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionCookiesSet(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionDebug(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionNetwork(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionReplay(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionWorkflowCode(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runSessionCode(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.GetSessionScriptParams{}
	resp, err := client.Client().GetSessionScriptWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	if resp.JSON200 != nil {
		fmt.Println(resp.JSON200.PythonScript)
	}

	return nil
}
