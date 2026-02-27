package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
)

// Manual flags for proxies (union type not auto-generated)
var sessionsStartProxy bool
var sessionsStartProxyCountry string

var (
	sessionID                 string
	sessionExecuteAction      string
	sessionScrapeInstructions string
	sessionScrapeOnlyMain     bool
	sessionCookiesSetFile     string
	sessionNetworkURLsOnly    bool
	sessionNetworkPath        string
)

// GetCurrentSessionID returns the session ID from flag, env var, or file (in priority order)
func GetCurrentSessionID() string {
	// 1. Check --session-id flag (already in sessionID variable if set)
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

// setCurrentViewerURL saves the viewer URL to the current_viewer_url file
func setCurrentViewerURL(url string) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, config.CurrentViewerURLFile), []byte(url), 0o600)
}

// getCurrentViewerURL reads the viewer URL from the current_viewer_url file
func getCurrentViewerURL() string {
	configDir, err := config.Dir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, config.CurrentViewerURLFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// clearCurrentViewerURL removes the current_viewer_url file
func clearCurrentViewerURL() error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, config.CurrentViewerURLFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// setCurrentSessionExpiry saves the session expiry timestamp to the current_session_expiry file
func setCurrentSessionExpiry(t time.Time) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, config.CurrentSessionExpiryFile), []byte(t.Format(time.RFC3339)), 0o600)
}

// getCurrentSessionExpiry reads the session expiry timestamp from the current_session_expiry file
func getCurrentSessionExpiry() (time.Time, error) {
	configDir, err := config.Dir()
	if err != nil {
		return time.Time{}, err
	}
	data, err := os.ReadFile(filepath.Join(configDir, config.CurrentSessionExpiryFile))
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
}

// clearCurrentSessionExpiry removes the current_session_expiry file
func clearCurrentSessionExpiry() error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, config.CurrentSessionExpiryFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RequireSessionID ensures a session ID is available from flag, env, or file
func RequireSessionID() error {
	sessionID = GetCurrentSessionID()
	if sessionID == "" {
		return errors.New("session ID required: use --session-id flag, set NOTTE_SESSION_ID env var, or start a session first")
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
	Use:    "observe",
	Short:  "Observe page state and available actions",
	Args:   cobra.NoArgs,
	RunE:   runSessionObserve,
	Hidden: true, // Use "notte page observe" instead
}

var sessionsExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute an action on the page",
	Args:  cobra.NoArgs,
	Example: `  # Direct JSON
  notte sessions execute --session-id <session-id> --action '{"type": "goto", "url": "https://example.com"}'

  # From file
  notte sessions execute --session-id <session-id> --action @action.json

  # From stdin
  echo '{"type": "goto", "url": "https://example.com"}' | notte sessions execute --session-id <session-id>

  # Using heredoc
  notte sessions execute --session-id "27ac8eea-1afc-4cad-aa23-bf122ed2390f" << 'EOF'
  {"type": "fill", "id": "I1", "value": "my text"}
  EOF`,
	RunE:   runSessionExecute,
	Hidden: true, // Use "notte page <action>" instead
}

var sessionsScrapeCmd = &cobra.Command{
	Use:    "scrape",
	Short:  "Scrape content from the page",
	Args:   cobra.NoArgs,
	RunE:   runSessionScrape,
	Hidden: true, // Use "notte page scrape" instead
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
	Use:    "debug",
	Short:  "Get debug info for the session",
	Args:   cobra.NoArgs,
	RunE:   runSessionDebug,
	Hidden: true,
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

var sessionsViewerCmd = &cobra.Command{
	Use:   "viewer",
	Short: "Open session viewer in browser",
	Args:  cobra.NoArgs,
	RunE:  runSessionViewer,
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	registerPaginationFlags(sessionsListCmd)
	sessionsListCmd.Flags().Bool("only-active", false, "Only return active sessions")

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
	sessionsCmd.AddCommand(sessionsViewerCmd)

	// Start command flags (auto-generated + manual proxy)
	RegisterSessionStartFlags(sessionsStartCmd)
	// Manual flags for proxies (union type: bool | array of proxy objects)
	sessionsStartCmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "Use default proxies")
	sessionsStartCmd.Flags().StringVar(&sessionsStartProxyCountry, "proxy-country", "", "Proxy country code (e.g. us, gb, fr). Implies --proxy")

	// Status command flags
	sessionsStatusCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Stop command flags
	sessionsStopCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Observe command flags
	sessionsObserveCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	// Execute command flags
	sessionsExecuteCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsExecuteCmd.Flags().StringVar(&sessionExecuteAction, "action", "", "Action JSON, @file, or '-' for stdin")

	// Scrape command flags
	sessionsScrapeCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsScrapeCmd.Flags().StringVar(&sessionScrapeInstructions, "instructions", "", "Extraction instructions")
	sessionsScrapeCmd.Flags().BoolVar(&sessionScrapeOnlyMain, "only-main-content", false, "Only scrape main content")

	// Cookies command flags
	sessionsCookiesCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Cookies-set command flags
	sessionsCookiesSetCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsCookiesSetCmd.Flags().StringVar(&sessionCookiesSetFile, "file", "", "JSON file containing cookies array (required)")
	_ = sessionsCookiesSetCmd.MarkFlagRequired("file")

	// Debug command flags
	sessionsDebugCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Network command flags
	sessionsNetworkCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsNetworkCmd.Flags().BoolVar(&sessionNetworkURLsOnly, "urls-only", false, "Only show download URLs without downloading")
	sessionsNetworkCmd.Flags().StringVar(&sessionNetworkPath, "path", "", "Output directory for downloaded files (defaults to temp directory)")

	// Replay command flags
	sessionsReplayCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Offset command flags
	sessionsOffsetCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Workflow-code command flags
	sessionsWorkflowCodeCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Code command flags
	sessionsCodeCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Viewer command flags
	sessionsViewerCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
}

func runSessionsList(cmd *cobra.Command, args []string) error {
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
	params := &api.ListSessionsParams{
		Page:     page,
		PageSize: pageSize,
	}
	if cmd.Flags().Changed("only-active") {
		v, _ := cmd.Flags().GetBool("only-active")
		params.OnlyActive = &v
	}
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
	// Check if there's already a current session
	existingSessionID := GetCurrentSessionID()
	if existingSessionID != "" {
		// Check if the session has expired based on stored max expiry
		if expiry, err := getCurrentSessionExpiry(); err == nil && !expiry.IsZero() && time.Now().UTC().After(expiry) {
			// Session has expired — silently clear stale state
			_ = clearCurrentSession()
			_ = clearCurrentViewerURL()
			_ = clearCurrentAgent()
			_ = clearCurrentSessionExpiry()
			existingSessionID = "" // skip the confirmation prompt
		}
	}
	if existingSessionID != "" {
		confirmed, err := confirmReplaceSession(existingSessionID)
		if err != nil {
			return err
		}
		if confirmed {
			// Stop the existing session
			stopClient, err := GetClient()
			if err != nil {
				return err
			}
			ctx, cancel := GetContextWithTimeout(cmd.Context())
			params := &api.SessionStopParams{}
			_, stopErr := stopClient.Client().SessionStopWithResponse(ctx, existingSessionID, params)
			cancel()
			if stopErr != nil {
				PrintInfo(fmt.Sprintf("Warning: could not stop session %s: %v", existingSessionID, stopErr))
			}
			_ = clearCurrentSession()
			_ = clearCurrentViewerURL()
			_ = clearCurrentAgent()
			_ = clearCurrentSessionExpiry()
		}
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Build request body from generated flags
	body, err := BuildSessionStartRequest(cmd)
	if err != nil {
		return err
	}

	// Handle proxies manually (union type: bool | array of proxy objects)
	// --proxy-country takes precedence over --proxy
	if cmd.Flags().Changed("proxy-country") {
		country := api.ProxyGeolocationCountry(sessionsStartProxyCountry)
		notteProxy := api.NotteProxy{Country: &country}
		var item api.ApiSessionStartRequest_Proxies_0_Item
		if err := item.FromNotteProxy(notteProxy); err != nil {
			return fmt.Errorf("failed to create proxy: %w", err)
		}
		proxyList := api.ApiSessionStartRequestProxies0{item}
		var proxies api.ApiSessionStartRequest_Proxies
		if err := proxies.FromApiSessionStartRequestProxies0(proxyList); err != nil {
			return fmt.Errorf("failed to set proxies: %w", err)
		}
		body.Proxies = &proxies
	} else if cmd.Flags().Changed("proxy") {
		var proxies api.ApiSessionStartRequest_Proxies
		if err := proxies.FromApiSessionStartRequestProxies1(sessionsStartProxy); err != nil {
			return fmt.Errorf("failed to set proxies: %w", err)
		}
		body.Proxies = &proxies
	}

	params := &api.SessionStartParams{}
	resp, err := client.Client().SessionStartWithResponse(ctx, params, *body)
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
		// Store session expiry if max duration is set
		if resp.JSON200.MaxDurationMinutes != nil && !resp.JSON200.CreatedAt.IsZero() {
			expiry := resp.JSON200.CreatedAt.Add(time.Duration(*resp.JSON200.MaxDurationMinutes) * time.Minute)
			if err := setCurrentSessionExpiry(expiry); err != nil {
				PrintInfo(fmt.Sprintf("Warning: could not save session expiry: %v", err))
			}
		}
		// Store viewer URL if available
		if resp.JSON200.ViewerUrl != nil && *resp.JSON200.ViewerUrl != "" {
			if err := setCurrentViewerURL(*resp.JSON200.ViewerUrl); err != nil {
				PrintInfo(fmt.Sprintf("Warning: could not save viewer URL: %v", err))
			}
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

	return printSessionStatus(resp.JSON200)
}

func runSessionStop(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	confirmed, err := ConfirmStop("session", sessionID)
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
			_ = clearCurrentViewerURL()
			_ = clearCurrentAgent()
			_ = clearCurrentSessionExpiry()
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

	params := &api.PageObserveParams{}
	resp, err := client.Client().PageObserveWithResponse(ctx, sessionID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// JSON mode: return filtered response (exclude screenshot and space.actions)
	if IsJSONOutput() {
		filtered := map[string]any{
			"ended_at":   resp.JSON200.EndedAt,
			"metadata":   resp.JSON200.Metadata,
			"started_at": resp.JSON200.StartedAt,
			"space": map[string]any{
				"description": resp.JSON200.Space.Description,
			},
		}
		return GetFormatter().Print(filtered)
	}

	// Text mode: return only the page description
	fmt.Println(resp.JSON200.Space.Description)
	return nil
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

	return printExecuteResponse(resp.JSON200)
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

	// Enable download URLs in API response
	download := true
	params := &api.SessionNetworkLogsParams{
		Download: &download,
	}
	resp, err := client.Client().SessionNetworkLogsWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// If urls-only flag is set, just print the URLs
	if sessionNetworkURLsOnly {
		// JSON mode: return full response
		if IsJSONOutput() {
			return GetFormatter().Print(resp.JSON200)
		}
		// Text mode: print nicely formatted list of URLs
		return printNetworkURLs(resp.JSON200)
	}

	// Default: download files to folder
	if resp.JSON200 != nil {
		return downloadNetworkLogs(resp.JSON200, sessionNetworkPath)
	}

	return GetFormatter().Print(resp.JSON200)
}

// downloadNetworkLogs downloads all network log files in parallel to a folder
func downloadNetworkLogs(logs *api.NetworkLogsResponse, outputPath string) error {
	var outDir string
	var err error

	if outputPath != "" {
		// Use specified path
		outDir = outputPath
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	} else {
		// Create temp directory
		outDir, err = os.MkdirTemp("", fmt.Sprintf("notte-network-%s-*", logs.SessionId))
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
	}

	// Create subdirectories for requests and responses
	requestsDir := filepath.Join(outDir, "requests")
	responsesDir := filepath.Join(outDir, "responses")
	if err := os.MkdirAll(requestsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create requests directory: %w", err)
	}
	if err := os.MkdirAll(responsesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create responses directory: %w", err)
	}

	// Collect all download tasks
	type downloadTask struct {
		url      string
		filename string
		dir      string
	}
	var tasks []downloadTask

	for _, req := range logs.Requests {
		if req.DownloadUrl != nil && *req.DownloadUrl != "" {
			tasks = append(tasks, downloadTask{
				url:      *req.DownloadUrl,
				filename: sanitizeFilename(req.Filename),
				dir:      requestsDir,
			})
		}
	}

	for _, resp := range logs.Responses {
		if resp.DownloadUrl != nil && *resp.DownloadUrl != "" {
			tasks = append(tasks, downloadTask{
				url:      *resp.DownloadUrl,
				filename: sanitizeFilename(resp.Filename),
				dir:      responsesDir,
			})
		}
	}

	if len(tasks) == 0 {
		return PrintResult(fmt.Sprintf("No network logs to download for session %s", logs.SessionId), map[string]any{
			"session_id": logs.SessionId,
			"path":       outDir,
			"count":      0,
		})
	}

	// Download all files in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))
	successCount := 0
	var successMu sync.Mutex

	for _, task := range tasks {
		wg.Add(1)
		go func(t downloadTask) {
			defer wg.Done()
			if err := downloadFile(t.url, filepath.Join(t.dir, t.filename)); err != nil {
				errChan <- fmt.Errorf("failed to download %s: %w", t.filename, err)
				return
			}
			successMu.Lock()
			successCount++
			successMu.Unlock()
		}(task)
	}

	wg.Wait()
	close(errChan)

	// Collect errors (only report first few)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		// Print warning but don't fail if some downloads succeeded
		if successCount > 0 {
			PrintInfo(fmt.Sprintf("Warning: %d download(s) failed", len(errs)))
		} else {
			return fmt.Errorf("all downloads failed: %v", errs[0])
		}
	}

	return PrintResult(fmt.Sprintf("Downloaded %d network logs to %s", successCount, outDir), map[string]any{
		"session_id": logs.SessionId,
		"path":       outDir,
		"count":      successCount,
		"requests":   len(logs.Requests),
		"responses":  len(logs.Responses),
	})
}

// httpClient is a shared HTTP client with timeout for downloading files
var httpClient = &http.Client{Timeout: 60 * time.Second}

// sanitizeFilename removes path traversal components from a filename
func sanitizeFilename(filename string) string {
	// Get only the base name to prevent directory traversal
	return filepath.Base(filename)
}

// downloadFile downloads a file from the given URL to the given path
func downloadFile(url, destPath string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	return err
}

// printNetworkURLs prints a nicely formatted list of network log URLs
func printNetworkURLs(logs *api.NetworkLogsResponse) error {
	if logs == nil {
		return nil
	}

	fmt.Printf("Session: %s\n", logs.SessionId)
	fmt.Printf("Total:   %d files\n\n", logs.TotalCount)

	if len(logs.Requests) > 0 {
		fmt.Printf("Requests (%d):\n", len(logs.Requests))
		for _, req := range logs.Requests {
			if req.DownloadUrl != nil && *req.DownloadUrl != "" {
				fmt.Printf("  %s\n", *req.DownloadUrl)
			}
		}
		fmt.Println()
	}

	if len(logs.Responses) > 0 {
		fmt.Printf("Responses (%d):\n", len(logs.Responses))
		for _, resp := range logs.Responses {
			if resp.DownloadUrl != nil && *resp.DownloadUrl != "" {
				fmt.Printf("  %s\n", *resp.DownloadUrl)
			}
		}
	}

	return nil
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
		AsWorkflow:          true,
		InferResponseFormat: boolPtr(true),
	}
	resp, err := client.Client().GetSessionScriptWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// JSON mode: return full response
	if IsJSONOutput() {
		return GetFormatter().Print(resp.JSON200)
	}

	// Text mode: just print the Python script
	if resp.JSON200 != nil {
		fmt.Println(resp.JSON200.PythonScript)
	}

	return nil
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

	params := &api.GetSessionScriptParams{
		InferResponseFormat: boolPtr(true),
	}
	resp, err := client.Client().GetSessionScriptWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// JSON mode: return full response
	if IsJSONOutput() {
		return GetFormatter().Print(resp.JSON200)
	}

	// Text mode: just print the Python script
	if resp.JSON200 != nil {
		fmt.Println(resp.JSON200.PythonScript)
	}

	return nil
}

func runSessionViewer(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	viewerURL := getCurrentViewerURL()

	// Fallback: fetch viewer URL from session status if not stored locally
	if viewerURL == "" {
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

		if resp.JSON200 != nil && resp.JSON200.ViewerUrl != nil {
			viewerURL = *resp.JSON200.ViewerUrl
		}
	}

	if viewerURL == "" {
		return fmt.Errorf("no viewer URL available for this session")
	}

	if !IsJSONOutput() {
		PrintInfo(fmt.Sprintf("Opening viewer in browser: %s", viewerURL))
	}
	if err := openBrowser(viewerURL); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return PrintResult("Opened viewer in browser.", map[string]any{
		"session_id": sessionID,
		"viewer_url": viewerURL,
		"success":    true,
	})
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait in background to avoid zombie processes
	go func() { _ = cmd.Wait() }()

	return nil
}
