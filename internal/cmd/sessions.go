package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/output"
)

// Manual flags for proxies and extra headers (union types not auto-generated)
var (
	sessionsStartProxy                 bool
	sessionsStartProxyCountry          string
	sessionsStartProxyExtServer        string
	sessionsStartProxyExtUsername      string
	sessionsStartProxyExtPassword      string
	sessionsStartProxyTailClientID     string
	sessionsStartProxyTailClientSecret string
	sessionsStartTunnel                string
	sessionsStartExtraHttpHeaders      string
	sessionsStartCount                 int
	sessionStopAll                     bool
)

var (
	sessionStatusAuth         bool
	sessionID                 string
	sessionExecuteAction      string
	sessionScrapeInstructions string
	sessionScrapeOnlyMain     bool
	sessionCookiesSetFile     string
	sessionSaveProfileID      string
	sessionSaveProfileName    string
	sessionNetworkURLsOnly    bool
	sessionNetworkPath        string
	sessionReplayOutput       string
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

// sessionIDFromArgs accepts the session ID as a positional argument, the
// form `notte sessions stop <id>` people and agents reach for, as well as
// through --session-id. The two must not disagree.
func sessionIDFromArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if cmd.Flags().Changed("session-id") && sessionID != args[0] {
		return errUsage("pass the session ID once, either as an argument or with --session-id")
	}
	// An empty argument is an unset variable in a script, not a request for
	// the current session: falling back would stop a session nobody named.
	if args[0] == "" {
		return errUsage("session ID must not be empty")
	}
	sessionID = args[0]
	return nil
}

// RequireSessionID ensures a session ID is available from flag, env, or file
func RequireSessionID() error {
	sessionID = GetCurrentSessionID()
	if sessionID == "" {
		return errUsage("session ID required: use --session-id flag, set NOTTE_SESSION_ID env var, or start a session first")
	}
	return nil
}

var sessionsCmd = &cobra.Command{
	Use:     "sessions",
	Aliases: []string{"browser", "browsers"},
	Short:   "Manage browser sessions",
	Long:    "List, create, and operate on browser sessions.",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active sessions",
	RunE:  runSessionsList,
}

var sessionsStartCmd = &cobra.Command{
	Use: "start",
	// Flag conflicts must be caught before RunE, which stops the current
	// session before it builds the request.
	PreRunE: validateSessionStartFlags,
	Short:   "Start a new browser session",
	RunE:    runSessionsStart,
}

var sessionsStatusCmd = &cobra.Command{
	Use:   "status [session-id]",
	Short: "Get session status",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionStatus,
}

var sessionsStopCmd = &cobra.Command{
	Use:   "stop [session-id]",
	Short: "Stop the session",
	Args:  cobra.MaximumNArgs(1),
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

var sessionsSaveProfileCmd = &cobra.Command{
	Use:   "save-profile",
	Short: "Save the running session's browser state into a profile",
	Long: `Save the running session's browser state (cookies, localStorage, sessionStorage) into a browser profile.

The session keeps running. Pass --profile-id to overwrite an existing profile, or --name to create a new one.
With neither, the state is saved into the profile the session was started with.`,
	Args: cobra.NoArgs,
	RunE: runSessionSaveProfile,
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
	Short: "Download session replay video",
	Long: `Download the replay video (MP4) for a session.

Examples:
  notte sessions replay                       # saves to temp directory
  notte sessions replay --path replay.mp4    # saves to specified path`,
	Args: cobra.NoArgs,
	RunE: runSessionReplay,
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
	registerFilterFlag(sessionsListCmd, flagAll, "a", "Include stopped sessions")

	sessionsCmd.AddCommand(sessionsStartCmd)
	sessionsCmd.AddCommand(sessionsStatusCmd)
	sessionsCmd.AddCommand(sessionsStopCmd)
	sessionsCmd.AddCommand(sessionsObserveCmd)
	sessionsCmd.AddCommand(sessionsExecuteCmd)
	sessionsCmd.AddCommand(sessionsScrapeCmd)
	sessionsCmd.AddCommand(sessionsCookiesCmd)
	sessionsCmd.AddCommand(sessionsCookiesSetCmd)
	sessionsCmd.AddCommand(sessionsSaveProfileCmd)
	sessionsCmd.AddCommand(sessionsDebugCmd)
	sessionsCmd.AddCommand(sessionsNetworkCmd)
	sessionsCmd.AddCommand(sessionsReplayCmd)
	sessionsCmd.AddCommand(sessionsOffsetCmd)
	sessionsCmd.AddCommand(sessionsWorkflowCodeCmd)
	sessionsCmd.AddCommand(sessionsCodeCmd)
	sessionsCmd.AddCommand(sessionsViewerCmd)

	// Start command flags (auto-generated + manual proxy)
	RegisterSessionStartFlags(sessionsStartCmd)
	// Positive opt-outs for options that default to true server-side
	registerSessionStartOptOutFlags(sessionsStartCmd)
	// Manual flags for proxies (union type: bool | array of proxy objects)
	sessionsStartCmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "Use default proxies")
	sessionsStartCmd.Flags().StringVar(&sessionsStartProxyCountry, "proxy-country", "", "Proxy country code (e.g. us, gb, fr). Selects a proxy on its own; do not combine with --proxy")
	sessionsStartCmd.Flags().StringVar(&sessionsStartProxyExtServer, "proxy-external-server", "", "External proxy server URL (e.g. http://proxy:8080). Enables external proxy")
	sessionsStartCmd.Flags().StringVar(&sessionsStartProxyExtUsername, "proxy-external-username", "", "External proxy username")
	sessionsStartCmd.Flags().StringVar(&sessionsStartProxyExtPassword, "proxy-external-password", "", "External proxy password")
	sessionsStartCmd.Flags().StringVar(&sessionsStartProxyTailClientID, "proxy-tailnet-client-id", "", "Tailnet OAuth client ID. Enables Tailscale proxy")
	sessionsStartCmd.Flags().StringVar(&sessionsStartProxyTailClientSecret, "proxy-tailnet-client-secret", "", "Tailnet OAuth client secret")
	sessionsStartCmd.Flags().StringVar(&sessionsStartTunnel, "tunnel", "", "Route public traffic through a saved tunnel (see `notte tunnel up`)")
	// Manual flag for extra HTTP headers (map type not auto-generated)
	sessionsStartCmd.Flags().StringVar(&sessionsStartExtraHttpHeaders, "extra-http-headers", "", `Extra HTTP headers as JSON (e.g. '{"Authorization": "Bearer xxx"}')`)
	sessionsStartCmd.Flags().IntVarP(&sessionsStartCount, "count", "n", 1, fmt.Sprintf("Number of sessions to start in parallel, with the same settings (at most %d)", maxSessionsStartCount))

	// Status command flags
	sessionsStatusCmd.Flags().BoolVar(&sessionStatusAuth, "auth", false, "Show managed authentication readiness and operations")
	sessionsStatusCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Stop command flags
	sessionsStopCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsStopCmd.Flags().BoolVarP(&sessionStopAll, "all", "a", false, "Stop every active session")

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
	sessionsCookiesSetCmd.Flags().StringVar(&sessionCookiesSetFile, "file", "", `JSON file with the cookies as {"cookies": [...]} (required)`)
	_ = sessionsCookiesSetCmd.MarkFlagRequired("file")

	// Save-profile command flags
	sessionsSaveProfileCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsSaveProfileCmd.Flags().StringVar(&sessionSaveProfileID, "profile-id", "", "Existing profile to overwrite")
	sessionsSaveProfileCmd.Flags().StringVar(&sessionSaveProfileName, "name", "", "Create a new profile with this name and save into it")
	sessionsSaveProfileCmd.MarkFlagsMutuallyExclusive("profile-id", "name")

	// Debug command flags
	sessionsDebugCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")

	// Network command flags
	sessionsNetworkCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsNetworkCmd.Flags().BoolVar(&sessionNetworkURLsOnly, "urls-only", false, "Only show download URLs without downloading")
	sessionsNetworkCmd.Flags().StringVar(&sessionNetworkPath, "path", "", "Output directory for downloaded files (defaults to temp directory)")

	// Replay command flags
	sessionsReplayCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	sessionsReplayCmd.Flags().StringVar(&sessionReplayOutput, "path", "", "Output path for the replay video (defaults to temp directory)")

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
	params.OnlyActive = resolveOnlyActive(cmd, flagAll, true)
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
	activeOnly := params.OnlyActive != nil && *params.OnlyActive
	empty, next := "No sessions", "notte sessions start"
	if activeOnly {
		// Closed sessions are one flag away.
		empty, next = "No active sessions", "notte sessions list --all"
	}
	if printed, err := PrintListOrEmpty(items, empty, next); err != nil {
		return err
	} else if printed {
		return nil
	}

	text, ok := formatter.(*output.TextFormatter)
	if !ok {
		if err := render(items); err != nil {
			return err
		}
		noteNextPage(cmd, resp.JSON200 != nil && resp.JSON200.HasNext)
		return nil
	}
	fillViewerURLs(ctx, client, items)
	more := resp.JSON200 != nil && resp.JSON200.HasNext
	if err := printSessionList(text, items, more, GetCurrentSessionID(), time.Now()); err != nil {
		return err
	}
	noteNextPage(cmd, more)
	return nil
}

// fillViewerURLs adds the live viewer link to active sessions. The list
// endpoint does not return it, the status endpoint does, so each active
// session's status is fetched, a few at a time. A failed fetch leaves the
// link out rather than failing the list.
func fillViewerURLs(ctx context.Context, client *api.NotteClient, sessions []api.SessionResponse) {
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i := range sessions {
		session := &sessions[i]
		if session.Status != api.SessionResponseStatusActive || (session.ViewerUrl != nil && *session.ViewerUrl != "") {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			resp, err := client.Client().SessionStatusWithResponse(ctx, session.SessionId, &api.SessionStatusParams{})
			if err == nil && resp.JSON200 != nil && resp.JSON200.ViewerUrl != nil {
				session.ViewerUrl = resp.JSON200.ViewerUrl
			}
		}()
	}
	wg.Wait()
}

// printSessionList shows one session per row. The current session, which
// commands use when --session-id is omitted, is marked with "*".
func printSessionList(text *output.TextFormatter, sessions []api.SessionResponse, more bool, current string, now time.Time) error {
	headers := []string{" ", sessionsHeader(len(sessions), more), "STATUS", "LAST ACTIVE", "CREATED", "VIEWER"}
	rows := make([][]string, 0, len(sessions))
	for _, session := range sessions {
		marker := " "
		if session.SessionId == current {
			marker = text.Colorize("*", termenv.ANSIGreen)
		}
		viewer := sessionViewer(session)
		rows = append(rows, []string{
			marker,
			session.SessionId,
			sessionStatus(text, string(session.Status)),
			relativeTime(session.LastAccessedAt.Time, now),
			relativeTime(session.CreatedAt.Time, now),
			viewer,
		})
	}
	if err := text.Table(headers, rows); err != nil {
		return err
	}
	printHint(text, "notte sessions status --session-id <id>")
	return nil
}

// printStartedSessions lists sessions that were just started, with how long
// each took to start. The first one is the current session.
// sessionViewer links a running session to its live viewer, and a stopped
// one, or one without a viewer, to its page in the console (replay, logs).
func sessionViewer(session api.SessionResponse) string {
	if session.Status == api.SessionResponseStatusActive && session.ViewerUrl != nil && *session.ViewerUrl != "" {
		return output.Link(*session.ViewerUrl, "open live viewer ↗")
	}
	console := strings.TrimSuffix(auth.ConsoleURL(), "/")
	return output.Link(console+"/sessions/"+url.PathEscape(session.SessionId), "open in console ↗")
}

func printStartedSessions(text *output.TextFormatter, sessions []api.SessionResponse, spinUps []time.Duration) error {
	headers := []string{" ", sessionsHeader(len(sessions), false), "STATUS", "SPIN-UP", "VIEWER"}
	rows := make([][]string, 0, len(sessions))
	for i, session := range sessions {
		marker := " "
		if i == 0 {
			marker = text.Colorize("*", termenv.ANSIGreen)
		}
		rows = append(rows, []string{
			marker,
			session.SessionId,
			sessionStatus(text, string(session.Status)),
			formatSpinUp(spinUps[i]),
			sessionViewer(session),
		})
	}
	return text.Table(headers, rows)
}

// sessionsHeader names the session column with how many rows follow; "+"
// means more are on the next page.
func sessionsHeader(count int, more bool) string {
	if more {
		return fmt.Sprintf("BROWSER SESSION (%d+)", count)
	}
	return fmt.Sprintf("BROWSER SESSION (%d)", count)
}

// formatSpinUp shows a start time as "850ms" or "1.8s".
func formatSpinUp(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func sessionStatus(text *output.TextFormatter, status string) string {
	switch status {
	case "active":
		return text.Colorize(status, termenv.ANSIGreen)
	case "error":
		return text.Colorize(status, termenv.ANSIRed)
	case "timed_out":
		return text.Colorize("timed out", termenv.ANSIYellow)
	case "":
		return text.Faint("-")
	}
	return text.Faint(status)
}

// relativeTime describes a recent moment as "2 min ago" and an older one by
// its date.
func relativeTime(t, now time.Time) string { return output.RelativeTime(t, now) }

// maxSessionsStartCount bounds --count, so a typo cannot start hundreds of
// billed sessions.
const maxSessionsStartCount = 20

func runSessionsStart(cmd *cobra.Command, args []string) error {
	count := sessionsStartCount
	if !cmd.Flags().Changed("count") {
		count = 1
	}
	if count < 1 || count > maxSessionsStartCount {
		return errUsage("--count must be between 1 and %d", maxSessionsStartCount)
	}

	// Resolve --tunnel first, so an unknown name or missing secret fails
	// before anything is sent.
	var tunnelItem *api.ApiSessionStartRequest_Proxies_0_Item
	if cmd.Flags().Changed("tunnel") {
		item, err := tunnelProxyItem(sessionsStartTunnel)
		if err != nil {
			return err
		}
		tunnelItem = &item
	}

	// Forget a pinned session whose maximum lifetime has passed.
	if GetCurrentSessionID() != "" {
		if expiry, err := getCurrentSessionExpiry(); err == nil && !expiry.IsZero() && time.Now().UTC().After(expiry) {
			_ = clearCurrentSession()
			_ = clearCurrentViewerURL()
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

	if err := applySessionStartOptOuts(cmd, body); err != nil {
		return err
	}

	// Handle proxies manually (union type: bool | array of proxy objects).
	// At most one proxy kind may be selected per call.
	// Already checked in PreRunE; repeated so the invariant does not depend on
	// the command wiring.
	if err := validateSessionStartProxyFlags(cmd); err != nil {
		return err
	}

	var proxyItems api.ApiSessionStartRequestProxies0

	if cmd.Flags().Changed("proxy-country") {
		country := api.ProxyGeolocationCountry(sessionsStartProxyCountry)
		notteProxy := api.NotteProxy{Country: &country}
		var item api.ApiSessionStartRequest_Proxies_0_Item
		if err := item.FromNotteProxy(notteProxy); err != nil {
			return fmt.Errorf("failed to create notte proxy: %w", err)
		}
		proxyItems = append(proxyItems, item)
	}

	if cmd.Flags().Changed("proxy-external-server") {
		ext := api.ExternalProxy{Server: sessionsStartProxyExtServer}
		if cmd.Flags().Changed("proxy-external-username") {
			ext.Username = &sessionsStartProxyExtUsername
		}
		if cmd.Flags().Changed("proxy-external-password") {
			ext.Password = &sessionsStartProxyExtPassword
		}
		var item api.ApiSessionStartRequest_Proxies_0_Item
		if err := item.FromExternalProxy(ext); err != nil {
			return fmt.Errorf("failed to create external proxy: %w", err)
		}
		proxyItems = append(proxyItems, item)
	}

	if cmd.Flags().Changed("proxy-tailnet-client-id") {
		tail := api.TailnetProxy{OauthClientId: &sessionsStartProxyTailClientID}
		if cmd.Flags().Changed("proxy-tailnet-client-secret") {
			tail.OauthClientSecret = &sessionsStartProxyTailClientSecret
		}
		var item api.ApiSessionStartRequest_Proxies_0_Item
		if err := item.FromTailnetProxy(tail); err != nil {
			return fmt.Errorf("failed to create tailnet proxy: %w", err)
		}
		proxyItems = append(proxyItems, item)
	}

	if tunnelItem != nil {
		proxyItems = append(proxyItems, *tunnelItem)
	}

	if len(proxyItems) > 0 {
		var proxies api.ApiSessionStartRequest_Proxies
		if err := proxies.FromApiSessionStartRequestProxies0(proxyItems); err != nil {
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

	// Handle extra HTTP headers (map type not auto-generated)
	if cmd.Flags().Changed("extra-http-headers") {
		var headers map[string]interface{}
		if err := json.Unmarshal([]byte(sessionsStartExtraHttpHeaders), &headers); err != nil {
			return fmt.Errorf("invalid JSON for --extra-http-headers: %w", err)
		}
		body.ExtraHttpHeaders = &headers
	}

	if count > 1 {
		return startSessions(ctx, client, body, count)
	}

	params := &api.SessionStartParams{}
	requested := time.Now()
	resp, err := client.Client().SessionStartWithResponse(ctx, params, *body)
	spinUp := time.Since(requested)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	rememberCurrentSession(resp.JSON200)
	return printSessionStarted(resp.JSON200, spinUp)
}

// startSessions starts count sessions with the same settings in parallel. The
// ones that started are shown even when others fail; the first becomes the
// current session.
func startSessions(ctx context.Context, client *api.NotteClient, body *api.SessionStartJSONRequestBody, count int) error {
	started := make([]*api.SessionResponse, count)
	spinUps := make([]time.Duration, count)
	failures := make([]error, count)
	requested := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Client().SessionStartWithResponse(ctx, &api.SessionStartParams{}, *body)
			spinUps[i] = time.Since(requested)
			if err != nil {
				failures[i] = fmt.Errorf("API request failed: %w", err)
				return
			}
			if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
				failures[i] = err
				return
			}
			started[i] = resp.JSON200
		}()
	}
	wg.Wait()
	total := time.Since(requested)

	var sessions []api.SessionResponse
	var times []time.Duration
	var firstErr error
	failed := 0
	for i := range started {
		if failures[i] != nil {
			failed++
			if firstErr == nil {
				firstErr = failures[i]
			}
			continue
		}
		if started[i] != nil {
			sessions = append(sessions, *started[i])
			times = append(times, spinUps[i])
		}
	}
	if len(sessions) == 0 {
		return firstErr
	}
	rememberCurrentSession(&sessions[0])

	formatter := GetFormatter()
	if text, ok := formatter.(*output.TextFormatter); ok {
		_, _ = fmt.Fprintf(text.Writer, "Started %d sessions in %s\n", len(sessions), formatSpinUp(total))
		if err := printStartedSessions(text, sessions, times); err != nil {
			return err
		}
	} else if err := render(sessions); err != nil {
		return err
	}
	if failed > 0 {
		for _, err := range failures {
			if err != nil {
				PrintInfo(fmt.Sprintf("Warning: a session failed to start: %v", err))
			}
		}
		var causes []error
		for _, err := range failures {
			if err != nil {
				causes = append(causes, err)
			}
		}
		return &partialError{message: fmt.Sprintf("started %d of %d sessions", len(sessions), count), causes: causes}
	}
	return nil
}

// rememberCurrentSession makes session the one commands use when
// --session-id is omitted.
func rememberCurrentSession(session *api.SessionResponse) {
	if session != nil {
		if err := setCurrentSession(session.SessionId); err != nil {
			PrintInfo(fmt.Sprintf("Warning: could not save current session: %v", err))
		}
		// The previous session's viewer link and expiry belong to it, not to
		// this one, whatever the new response leaves out.
		_ = clearCurrentViewerURL()
		_ = clearCurrentSessionExpiry()
		// Store session expiry if max duration is set
		if session.MaxDurationMinutes != nil && !session.CreatedAt.IsZero() {
			expiry := session.CreatedAt.Add(time.Duration(*session.MaxDurationMinutes) * time.Minute)
			if err := setCurrentSessionExpiry(expiry); err != nil {
				PrintInfo(fmt.Sprintf("Warning: could not save session expiry: %v", err))
			}
		}
		// Store viewer URL if available
		if session.ViewerUrl != nil && *session.ViewerUrl != "" {
			if err := setCurrentViewerURL(*session.ViewerUrl); err != nil {
				PrintInfo(fmt.Sprintf("Warning: could not save viewer URL: %v", err))
			}
		}
	}
}

// printSessionStarted shows what is needed right after a start: the ID, the
// browser settings, when the session stops, and the live viewer. JSON output
// keeps the full API response, including the CDP URL.
func printSessionStarted(session *api.SessionResponse, spinUp time.Duration) error {
	formatter := GetFormatter()
	text, ok := formatter.(*output.TextFormatter)
	if !ok || session == nil {
		return render(session)
	}

	var browser []string
	first := "chromium"
	if session.BrowserType != nil && *session.BrowserType != "" {
		first = string(*session.BrowserType)
	}
	if headless, ok := session.AdditionalProperties["headless"].(bool); ok && headless {
		first += ", headless"
	}
	browser = append(browser, first)
	if session.Proxies != nil && *session.Proxies {
		browser = append(browser, "proxies")
	}
	if session.SolveCaptchas != nil && *session.SolveCaptchas {
		browser = append(browser, "captcha solving")
	}

	stops := fmt.Sprintf("after %d min idle", session.IdleTimeoutMinutes)
	if session.MaxDurationMinutes != nil {
		stops += fmt.Sprintf(", %d min max", *session.MaxDurationMinutes)
	}

	lines := [][2]string{
		{"Browser", strings.Join(browser, " · ")},
		{"Stops", stops},
	}
	if session.ViewerUrl != nil && *session.ViewerUrl != "" {
		lines = append(lines, [2]string{"Viewer", output.Link(*session.ViewerUrl, "open live viewer ↗")})
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Started session %s in %s\n", session.SessionId, formatSpinUp(spinUp))
	for _, line := range lines {
		fmt.Fprintf(&b, "  %s%s%s\n", text.Label(line[0]), strings.Repeat(" ", 10-len(line[0])), line[1])
	}
	_, err := io.WriteString(text.Writer, b.String())
	return err
}

func runSessionStatus(cmd *cobra.Command, args []string) error {
	if err := sessionIDFromArgs(cmd, args); err != nil {
		return err
	}
	if err := RequireSessionID(); err != nil {
		return err
	}
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	if sessionStatusAuth {
		resp, err := client.Client().SessionAuthReadinessWithResponse(ctx, sessionID, &api.SessionAuthReadinessParams{})
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return err
		}
		return render(resp.JSON200)
	}

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
	if sessionStopAll {
		if cmd.Flags().Changed("session-id") || len(args) > 0 {
			return errUsage("use either --all or a session ID, not both")
		}
		return stopAllSessions(cmd.Context(), os.Stdin, os.Stderr)
	}
	if err := sessionIDFromArgs(cmd, args); err != nil {
		return err
	}
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
			_ = clearCurrentSessionExpiry()
		}
	}

	return printDone("Stopped session "+sessionID, map[string]any{
		"id":     sessionID,
		"status": "stopped",
	}, "notte sessions status --session-id "+sessionID)
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
		if resp.JSON200.ElementStates != nil {
			filtered["element_states"] = resp.JSON200.ElementStates
		}
		return render(filtered)
	}

	// Text mode: the page, and its elements with their live state
	fmt.Println(renderObservation(resp.JSON200))
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

	return render(resp.JSON200)
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

	return renderDone("Set cookies in session "+sessionID, resp.JSON200)
}

func runSessionSaveProfile(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}
	if sessionSaveProfileID != "" && sessionSaveProfileName != "" {
		return errUsage("--profile-id and --name are mutually exclusive")
	}
	// An explicitly passed but empty target (e.g. an unset shell variable) must
	// not silently fall back to overwriting the session's start profile.
	if cmd.Flags().Changed("profile-id") && strings.TrimSpace(sessionSaveProfileID) == "" {
		return errUsage("--profile-id must not be empty")
	}
	if cmd.Flags().Changed("name") && strings.TrimSpace(sessionSaveProfileName) == "" {
		return errUsage("--name must not be empty")
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.SessionProfileSaveJSONRequestBody{}
	if sessionSaveProfileID != "" {
		body.ProfileId = &sessionSaveProfileID
	}
	if sessionSaveProfileName != "" {
		body.Name = &sessionSaveProfileName
	}

	params := &api.SessionProfileSaveParams{}
	resp, err := client.Client().SessionProfileSaveWithResponse(ctx, sessionID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	if resp.JSON200 == nil {
		return render(resp.JSON200)
	}
	return renderDone(fmt.Sprintf("Saved session %s to profile %s", sessionID, resp.JSON200.ProfileId), resp.JSON200)
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

	return render(resp.JSON200)
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
			return render(resp.JSON200)
		}
		// Text mode: print nicely formatted list of URLs
		return printNetworkURLs(resp.JSON200)
	}

	// Default: download files to folder
	if resp.JSON200 != nil {
		return downloadNetworkLogs(resp.JSON200, sessionNetworkPath)
	}

	return render(resp.JSON200)
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

	// Collect all download tasks
	type downloadTask struct {
		url      string
		filename string
		dir      string
	}
	var tasks []downloadTask

	for _, batch := range logs.Batches {
		if batch.DownloadUrl != nil && *batch.DownloadUrl != "" {
			tasks = append(tasks, downloadTask{
				url:      *batch.DownloadUrl,
				filename: sanitizeFilename(batch.Key),
				dir:      outDir,
			})
		}
	}

	if len(tasks) == 0 {
		return PrintResult(withEmptyFace("No network logs for session "+logs.SessionId), map[string]any{
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

	return printDone(fmt.Sprintf("Downloaded %d network logs of session %s to %s", successCount, logs.SessionId, outDir), map[string]any{
		"session_id": logs.SessionId,
		"path":       outDir,
		"count":      successCount,
		"batches":    len(logs.Batches),
	}, "notte sessions status --session-id "+logs.SessionId)
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

	text := textFormatter()
	if text == nil {
		return nil
	}
	var urls []string
	for _, batch := range logs.Batches {
		if batch.DownloadUrl != nil && *batch.DownloadUrl != "" {
			urls = append(urls, *batch.DownloadUrl)
		}
	}
	if err := printDetails(text, "Network logs", [][2]string{
		{"Session", logs.SessionId},
		{"Files", strconv.Itoa(logs.TotalBatchCount)},
		{"Format", deref(logs.Format)},
		{"Links", strings.Join(urls, "\n")},
		{"Console", consoleLink("sessions", logs.SessionId)},
	}); err != nil {
		return err
	}
	printHint(text, "notte sessions network --session-id "+logs.SessionId+" --path <folder>")
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

	if resp.JSON200 == nil {
		return fmt.Errorf("unexpected empty response from replay API")
	}

	replay := resp.JSON200

	// If no mp4_url, return the raw response data
	if replay.Mp4Url == nil || *replay.Mp4Url == "" {
		return PrintResult(withEmptyFace("No replay video for session "+sessionID), map[string]any{
			"session_id": sessionID,
			"success":    false,
		})
	}

	// Determine output path
	outputPath := sessionReplayOutput
	if outputPath == "" {
		// Default to temp directory
		tmpDir := os.TempDir()
		outputPath = filepath.Join(tmpDir, fmt.Sprintf("notte-replay-%s.mp4", sessionID))
	}

	// Clean the path to resolve any ".." components
	outputPath = filepath.Clean(outputPath)

	// Create directory if it doesn't exist
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Download the replay video from the presigned URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *replay.Mp4Url, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	httpResp, err := http.DefaultClient.Do(req) //nolint:gosec // URL is from trusted API response
	if err != nil {
		return fmt.Errorf("failed to download replay video: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download replay video: HTTP %d", httpResp.StatusCode)
	}

	videoData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read replay video: %w", err)
	}

	// Write the replay video file
	err = os.WriteFile(outputPath, videoData, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write replay video: %w", err)
	}

	return printDone(fmt.Sprintf("Saved the replay of session %s to %s", sessionID, outputPath), map[string]any{
		"path":       outputPath,
		"session_id": sessionID,
		"success":    true,
	}, "notte sessions status --session-id "+sessionID)
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

	return render(resp.JSON200)
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
		return render(resp.JSON200)
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
		return render(resp.JSON200)
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

	if err := openBrowser(viewerURL); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return printDone(fmt.Sprintf("Opened the viewer of session %s: %s", sessionID, viewerURL), map[string]any{
		"session_id": sessionID,
		"viewer_url": viewerURL,
		"success":    true,
	}, "notte sessions status --session-id "+sessionID)
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
