package cmd

import (
	"fmt"

	"github.com/salmonumbrella/notte-cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	sessionsStartHeadless      bool
	sessionsStartBrowser       string
	sessionsStartTimeout       int
	sessionsStartProxies       bool
	sessionsStartSolveCaptchas bool
	sessionsStartViewportW     int
	sessionsStartViewportH     int
	sessionsStartUserAgent     string
	sessionsStartCdpURL        string
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage browser sessions",
	Long:  "List and create browser sessions.",
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

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsStartCmd)

	sessionsStartCmd.Flags().BoolVar(&sessionsStartHeadless, "headless", true, "Run session in headless mode")
	sessionsStartCmd.Flags().StringVar(&sessionsStartBrowser, "browser", "chromium", "Browser type (chromium, chrome, firefox)")
	sessionsStartCmd.Flags().IntVar(&sessionsStartTimeout, "timeout", 3, "Session timeout in minutes (1-15)")
	sessionsStartCmd.Flags().BoolVar(&sessionsStartProxies, "proxies", false, "Use default proxies")
	sessionsStartCmd.Flags().BoolVar(&sessionsStartSolveCaptchas, "solve-captchas", false, "Automatically solve captchas")
	sessionsStartCmd.Flags().IntVar(&sessionsStartViewportW, "viewport-width", 0, "Viewport width in pixels")
	sessionsStartCmd.Flags().IntVar(&sessionsStartViewportH, "viewport-height", 0, "Viewport height in pixels")
	sessionsStartCmd.Flags().StringVar(&sessionsStartUserAgent, "user-agent", "", "Custom user agent string")
	sessionsStartCmd.Flags().StringVar(&sessionsStartCdpURL, "cdp-url", "", "CDP URL of remote session provider")
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()

	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		fmt.Println("No active sessions.")
		return nil
	}

	return formatter.Print(resp.JSON200.Items)
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

	// Set timeout if provided
	if sessionsStartTimeout > 0 {
		body.TimeoutMinutes = &sessionsStartTimeout
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

	params := &api.SessionStartParams{}
	resp, err := client.Client().SessionStartWithResponse(ctx, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}
