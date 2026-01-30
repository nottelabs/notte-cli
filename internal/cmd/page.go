package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

// Page command flags
var (
	// click flags
	pageClickTimeout int
	pageClickEnter   bool

	// fill flags
	pageFillClear bool
	pageFillEnter bool

	// check flags
	pageCheckValue bool

	// upload flags
	pageUploadFile string

	// complete flags
	pageCompleteSuccess bool

	// form-fill flags
	pageFormFillData string

	// screenshot flags
	pageScreenshotOutput string
)

// printExecuteResponse formats execute response output.
// In JSON mode, returns the full response. In text mode, prints
// only the message and data fields, hiding the Session field.
func printExecuteResponse(resp *api.ApiExecutionResponse) error {
	// JSON mode: return full response
	if IsJSONOutput() {
		return GetFormatter().Print(resp)
	}

	if !resp.Success {
		// Build error message with available context
		if resp.Exception != nil {
			// Include exception and message if both present and different
			if resp.Message != "" && *resp.Exception != resp.Message {
				return fmt.Errorf("%s: %s", *resp.Exception, resp.Message)
			}
			return fmt.Errorf("%s", *resp.Exception)
		}
		// No exception - use message or generic fallback
		if resp.Message != "" {
			return fmt.Errorf("action failed: %s", resp.Message)
		}
		return fmt.Errorf("action failed")
	}

	// Print message
	fmt.Println(resp.Message)

	// Print data if non-nil
	if resp.Data != nil {
		return GetFormatter().Print(resp.Data)
	}
	return nil
}

// idPattern matches element IDs: single letter (I, B, L, F, O, M) followed by digits
// Examples: B1, I5, L10, F2, O3, M1
var idPattern = regexp.MustCompile(`^[IBLMFO]\d+$`)

// parseSelector returns (id, selector, error) based on pattern matching
// B3 or @B3 -> element ID (id: "B3")
// #btn or any other string -> CSS selector (selector: "#btn")
func parseSelector(arg string) (string, string, error) {
	if arg == "" {
		return "", "", fmt.Errorf("selector cannot be empty")
	}

	// Support legacy @-prefix format for backwards compatibility
	if strings.HasPrefix(arg, "@") {
		id := strings.TrimPrefix(arg, "@")
		if id == "" {
			return "", "", fmt.Errorf("element ID cannot be empty (use @id format)")
		}
		return id, "", nil
	}

	// Auto-detect ID format: single letter [IBLMFO] followed by digits
	if idPattern.MatchString(arg) {
		return arg, "", nil
	}

	// Otherwise treat as CSS selector
	return "", arg, nil
}

// executePageAction builds JSON and calls the PageExecute API
func executePageAction(cmd *cobra.Command, action map[string]any) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	actionJSON, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("failed to marshal action: %w", err)
	}

	params := &api.PageExecuteParams{}
	resp, err := client.Client().PageExecuteWithBodyWithResponse(ctx, sessionID, params, "application/json", bytes.NewReader(actionJSON))
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return printExecuteResponse(resp.JSON200)
}

var pageCmd = &cobra.Command{
	Use:   "page",
	Short: "Execute page actions (syntactic sugar for sessions execute)",
	Long: `Execute page actions with a simplified command interface.

Use:
  notte page click "#btn"
  notte page click B3         # element ID (auto-detected)
  notte page click @B3        # @-prefix also works (legacy)
  notte page fill I1 "hello"
  notte page goto "https://example.com"`,
}

// Element Actions (selector-based)

var pageClickCmd = &cobra.Command{
	Use:   "click <id|selector>",
	Short: "Click an element",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageClick,
}

func runPageClick(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "click"}

	id, selector, err := parseSelector(args[0])
	if err != nil {
		return err
	}
	if id != "" {
		action["id"] = id
	} else {
		action["selector"] = selector
	}

	if pageClickTimeout > 0 {
		action["timeout"] = pageClickTimeout
	}
	if pageClickEnter {
		action["press_enter"] = true
	}

	return executePageAction(cmd, action)
}

var pageFillCmd = &cobra.Command{
	Use:   "fill <id|selector> <value>",
	Short: "Fill an input field with a value",
	Args:  cobra.ExactArgs(2),
	RunE:  runPageFill,
}

func runPageFill(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "fill"}

	id, selector, err := parseSelector(args[0])
	if err != nil {
		return err
	}
	if id != "" {
		action["id"] = id
	} else {
		action["selector"] = selector
	}

	action["value"] = args[1]

	if pageFillClear {
		action["clear"] = true
	}
	if pageFillEnter {
		action["press_enter"] = true
	}

	return executePageAction(cmd, action)
}

var pageCheckCmd = &cobra.Command{
	Use:   "check <id|selector>",
	Short: "Check or uncheck a checkbox",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageCheck,
}

func runPageCheck(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "check"}

	id, selector, err := parseSelector(args[0])
	if err != nil {
		return err
	}
	if id != "" {
		action["id"] = id
	} else {
		action["selector"] = selector
	}

	action["value"] = pageCheckValue

	return executePageAction(cmd, action)
}

var pageSelectCmd = &cobra.Command{
	Use:   "select <id|selector> <value>",
	Short: "Select a dropdown option",
	Args:  cobra.ExactArgs(2),
	RunE:  runPageSelect,
}

func runPageSelect(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "select_dropdown_option"}

	id, selector, err := parseSelector(args[0])
	if err != nil {
		return err
	}
	if id != "" {
		action["id"] = id
	} else {
		action["selector"] = selector
	}

	action["value"] = args[1]

	return executePageAction(cmd, action)
}

var pageDownloadCmd = &cobra.Command{
	Use:   "download <id|selector>",
	Short: "Download a file by clicking an element",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageDownload,
}

func runPageDownload(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "download_file"}

	id, selector, err := parseSelector(args[0])
	if err != nil {
		return err
	}
	if id != "" {
		action["id"] = id
	} else {
		action["selector"] = selector
	}

	return executePageAction(cmd, action)
}

var pageUploadCmd = &cobra.Command{
	Use:   "upload <id|selector> --file <path>",
	Short: "Upload a file to an input element",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageUpload,
}

func runPageUpload(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "upload_file"}

	id, selector, err := parseSelector(args[0])
	if err != nil {
		return err
	}
	if id != "" {
		action["id"] = id
	} else {
		action["selector"] = selector
	}

	action["file_path"] = pageUploadFile

	return executePageAction(cmd, action)
}

// Navigation Actions

var pageGotoCmd = &cobra.Command{
	Use:   "goto <url>",
	Short: "Navigate to a URL",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageGoto,
}

func runPageGoto(cmd *cobra.Command, args []string) error {
	action := map[string]any{
		"type": "goto",
		"url":  args[0],
	}
	return executePageAction(cmd, action)
}

var pageNewTabCmd = &cobra.Command{
	Use:   "new-tab <url>",
	Short: "Open a URL in a new tab",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageNewTab,
}

func runPageNewTab(cmd *cobra.Command, args []string) error {
	action := map[string]any{
		"type": "goto_new_tab",
		"url":  args[0],
	}
	return executePageAction(cmd, action)
}

var pageBackCmd = &cobra.Command{
	Use:   "back",
	Short: "Go back in browser history",
	Args:  cobra.NoArgs,
	RunE:  runPageBack,
}

func runPageBack(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "go_back"}
	return executePageAction(cmd, action)
}

var pageForwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "Go forward in browser history",
	Args:  cobra.NoArgs,
	RunE:  runPageForward,
}

func runPageForward(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "go_forward"}
	return executePageAction(cmd, action)
}

var pageReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the current page",
	Args:  cobra.NoArgs,
	RunE:  runPageReload,
}

func runPageReload(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "reload"}
	return executePageAction(cmd, action)
}

// Scroll Actions

var pageScrollDownCmd = &cobra.Command{
	Use:   "scroll-down [amount]",
	Short: "Scroll down the page",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPageScrollDown,
}

func runPageScrollDown(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "scroll_down"}

	if len(args) > 0 {
		amount, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid scroll amount: %w", err)
		}
		action["amount"] = amount
	}

	return executePageAction(cmd, action)
}

var pageScrollUpCmd = &cobra.Command{
	Use:   "scroll-up [amount]",
	Short: "Scroll up the page",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPageScrollUp,
}

func runPageScrollUp(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "scroll_up"}

	if len(args) > 0 {
		amount, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid scroll amount: %w", err)
		}
		action["amount"] = amount
	}

	return executePageAction(cmd, action)
}

// Keyboard Actions

var pagePressCmd = &cobra.Command{
	Use:   "press <key>",
	Short: "Press a key (e.g., Enter, Escape, Tab)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPagePress,
}

func runPagePress(cmd *cobra.Command, args []string) error {
	action := map[string]any{
		"type": "press_key",
		"key":  args[0],
	}
	return executePageAction(cmd, action)
}

// Tab Management

var pageSwitchTabCmd = &cobra.Command{
	Use:   "switch-tab <index>",
	Short: "Switch to a tab by index (0-based)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageSwitchTab,
}

func runPageSwitchTab(cmd *cobra.Command, args []string) error {
	tabIndex, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid tab index: %w", err)
	}

	action := map[string]any{
		"type":      "switch_tab",
		"tab_index": tabIndex,
	}
	return executePageAction(cmd, action)
}

var pageCloseTabCmd = &cobra.Command{
	Use:   "close-tab",
	Short: "Close the current tab",
	Args:  cobra.NoArgs,
	RunE:  runPageCloseTab,
}

func runPageCloseTab(cmd *cobra.Command, args []string) error {
	action := map[string]any{"type": "close_tab"}
	return executePageAction(cmd, action)
}

// Wait/Utility

var pageWaitCmd = &cobra.Command{
	Use:   "wait <milliseconds>",
	Short: "Wait for a specified duration",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageWait,
}

func runPageWait(cmd *cobra.Command, args []string) error {
	timeMs, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid time value: %w", err)
	}

	action := map[string]any{
		"type":    "wait",
		"time_ms": timeMs,
	}
	return executePageAction(cmd, action)
}

// Page State

var pageObserveCmd = &cobra.Command{
	Use:   "observe",
	Short: "Observe the current page state",
	Args:  cobra.NoArgs,
	RunE:  runSessionObserve,
}

// Data Extraction

var pageScrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape content from the page",
	Args:  cobra.NoArgs,
	RunE:  runSessionScrape,
}

// Other Actions

var pageCaptchaSolveCmd = &cobra.Command{
	Use:   "captcha-solve <type>",
	Short: "Solve a CAPTCHA (e.g., recaptcha_v2, hcaptcha)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageCaptchaSolve,
}

func runPageCaptchaSolve(cmd *cobra.Command, args []string) error {
	action := map[string]any{
		"type":         "captcha_solve",
		"captcha_type": args[0],
	}
	return executePageAction(cmd, action)
}

var pageCompleteCmd = &cobra.Command{
	Use:   "complete <answer>",
	Short: "Mark task as complete with an answer",
	Args:  cobra.ExactArgs(1),
	RunE:  runPageComplete,
}

func runPageComplete(cmd *cobra.Command, args []string) error {
	action := map[string]any{
		"type":    "completion",
		"answer":  args[0],
		"success": pageCompleteSuccess,
	}
	return executePageAction(cmd, action)
}

var pageFormFillCmd = &cobra.Command{
	Use:   "form-fill --data <json>",
	Short: "Fill a form with JSON data",
	Args:  cobra.NoArgs,
	RunE:  runPageFormFill,
}

func runPageFormFill(cmd *cobra.Command, args []string) error {
	var formData map[string]any
	if err := json.Unmarshal([]byte(pageFormFillData), &formData); err != nil {
		return fmt.Errorf("invalid JSON data: %w", err)
	}

	action := map[string]any{
		"type":  "form_fill",
		"value": formData,
	}
	return executePageAction(cmd, action)
}

var pageScreenshotCmd = &cobra.Command{
	Use:   "screenshot [output]",
	Short: "Take a screenshot of the current page",
	Long: `Take a screenshot of the current page and save it as a JPEG file.

By default, saves to a temporary directory. Optionally provide a path to save to a specific location.

Examples:
  notte page screenshot                    # saves to tmp directory
  notte page screenshot screenshot.jpg     # saves to specified path
  notte page screenshot --output out.jpg   # saves to specified path (alt syntax)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPageScreenshot,
}

func runPageScreenshot(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Construct the URL manually since this endpoint isn't in the generated client yet
	url := fmt.Sprintf("%s/sessions/%s/page/screenshot", client.BaseURL(), sessionID)

	// Create the POST request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute the request through the client's HTTP client (which has auth and retry)
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if err := HandleAPIResponse(resp, body); err != nil {
			return err
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Determine output path
	outputPath := pageScreenshotOutput
	if len(args) > 0 {
		outputPath = args[0]
	}

	if outputPath == "" {
		// Default to temp directory
		tmpDir := os.TempDir()
		outputPath = filepath.Join(tmpDir, fmt.Sprintf("notte-screenshot-%s.jpg", sessionID))
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

	// Write the file
	err = os.WriteFile(outputPath, imageData, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write screenshot: %w", err)
	}

	return PrintResult(fmt.Sprintf("Screenshot saved: %s", outputPath), map[string]any{
		"path":    outputPath,
		"session": sessionID,
		"success": true,
	})
}

func init() {
	rootCmd.AddCommand(pageCmd)

	// Add all subcommands
	pageCmd.AddCommand(pageClickCmd)
	pageCmd.AddCommand(pageFillCmd)
	pageCmd.AddCommand(pageCheckCmd)
	pageCmd.AddCommand(pageSelectCmd)
	pageCmd.AddCommand(pageDownloadCmd)
	pageCmd.AddCommand(pageUploadCmd)
	pageCmd.AddCommand(pageGotoCmd)
	pageCmd.AddCommand(pageNewTabCmd)
	pageCmd.AddCommand(pageBackCmd)
	pageCmd.AddCommand(pageForwardCmd)
	pageCmd.AddCommand(pageReloadCmd)
	pageCmd.AddCommand(pageScrollDownCmd)
	pageCmd.AddCommand(pageScrollUpCmd)
	pageCmd.AddCommand(pagePressCmd)
	pageCmd.AddCommand(pageSwitchTabCmd)
	pageCmd.AddCommand(pageCloseTabCmd)
	pageCmd.AddCommand(pageWaitCmd)
	pageCmd.AddCommand(pageObserveCmd)
	pageCmd.AddCommand(pageScrapeCmd)
	pageCmd.AddCommand(pageCaptchaSolveCmd)
	pageCmd.AddCommand(pageCompleteCmd)
	pageCmd.AddCommand(pageFormFillCmd)
	pageCmd.AddCommand(pageScreenshotCmd)

	// Add --id flag to parent command (inherited by all subcommands)
	pageCmd.PersistentFlags().StringVar(&sessionID, "id", "", "Session ID (uses current session if not specified)")

	// click flags
	pageClickCmd.Flags().IntVar(&pageClickTimeout, "timeout", 0, "Timeout in milliseconds")
	pageClickCmd.Flags().BoolVar(&pageClickEnter, "enter", false, "Press Enter after clicking")

	// fill flags
	pageFillCmd.Flags().BoolVar(&pageFillClear, "clear", false, "Clear the field before filling")
	pageFillCmd.Flags().BoolVar(&pageFillEnter, "enter", false, "Press Enter after filling")

	// check flags
	pageCheckCmd.Flags().BoolVar(&pageCheckValue, "value", true, "Check (true) or uncheck (false)")

	// upload flags
	pageUploadCmd.Flags().StringVar(&pageUploadFile, "file", "", "Path to the file to upload (required)")
	_ = pageUploadCmd.MarkFlagRequired("file")

	// observe flags
	pageObserveCmd.Flags().StringVar(&sessionObserveURL, "url", "", "Navigate to URL before observing")

	// scrape flags
	pageScrapeCmd.Flags().StringVar(&sessionScrapeInstructions, "instructions", "", "Extraction instructions")
	pageScrapeCmd.Flags().BoolVar(&sessionScrapeOnlyMain, "only-main-content", false, "Only scrape main content")

	// complete flags
	pageCompleteCmd.Flags().BoolVar(&pageCompleteSuccess, "success", true, "Whether the completion was successful")

	// form-fill flags
	pageFormFillCmd.Flags().StringVar(&pageFormFillData, "data", "", "JSON object with form field values (required)")
	_ = pageFormFillCmd.MarkFlagRequired("data")

	// screenshot flags
	pageScreenshotCmd.Flags().StringVarP(&pageScreenshotOutput, "output", "o", "", "Output path for the screenshot (defaults to temp directory)")
}
