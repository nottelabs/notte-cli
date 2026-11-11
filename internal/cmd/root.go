package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	apierrors "github.com/nottelabs/notte-cli/internal/errors"
	"github.com/nottelabs/notte-cli/internal/output"
	"github.com/nottelabs/notte-cli/internal/update"
)

var (
	// Global flags
	outputFormat   string
	noColor        bool
	verbose        bool
	requestTimeout int
	yesFlag        bool // Skip confirmation prompts

	// Version set at build time
	Version = "dev"
)

// rootCmd is the base command
var rootCmd = &cobra.Command{
	Use:   "notte",
	Short: "CLI for notte.cc browser automation platform",
	Long: `notte-cli provides command-line access to the notte.cc platform
for browser automation and web scraping.

Get started:
  notte login             # Log in (approve a code in the console)
  notte sessions start    # Start a browser session
  notte search <query>    # Search the web
  notte scrape <url>      # Quick scrape a webpage

For scripts and agents:
  -o json (or NOTTE_OUTPUT=json) prints JSON on stdout; errors are JSON on
  stderr with a stable "code". Pass --yes to commands that delete.

Exit codes:
  0 success    1 failure           2 wrong flags or arguments
  3 not signed in or key rejected  4 not found
  5 needs --yes                    6 rate limited
  7 the function run failed`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI
func Execute() {
	// Start background update check (nil-safe; returns nil for dev builds)
	checker := update.NewChecker(Version)
	if checker != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		go checker.Run(ctx)
	}

	err := rootCmd.Execute()

	// Show update notification after command output
	if checker != nil && !upgradeRan {
		if result := checker.GetResult(); result != nil {
			update.PrintUpdateNotification(result, os.Stderr, os.Stdin, yesFlag, IsJSONOutput(), noColor)
		}
	}

	if err != nil {
		code, exit := classifyError(err)
		switch formatter := GetFormatter().(type) {
		case *output.TextFormatter:
			if isUnauthorized(err) {
				formatter.PrintErrorWithFace(err, output.FaceDenied)
			} else {
				formatter.PrintError(err)
			}
		case *output.JSONFormatter:
			formatter.ErrorCode = code
			formatter.PrintError(err)
		}
		os.Exit(exit)
	}
}

// isUnauthorized reports errors about signing in: no saved key, or a key the
// API rejected.
func isUnauthorized(err error) bool {
	if errors.Is(err, auth.ErrNoAPIKey) {
		return true
	}
	var authErr *apierrors.AuthError
	if errors.As(err, &authErr) {
		return true
	}
	var apiErr *apierrors.APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == 401 || apiErr.StatusCode == 403)
}

func init() {
	// Hide completion command from help output (still accessible via `notte completion`)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", defaultOutputFormat(), "Output format (text, json); NOTTE_OUTPUT sets the default")
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &usageError{err: err} })
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().IntVar(&requestTimeout, "timeout", 60, "API request timeout in seconds")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "Skip confirmation prompts (needed to delete without a terminal)")

	// Set up confirmation state before each command
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		SetSkipConfirmation(yesFlag)
	}

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return PrintResult(fmt.Sprintf("notte version %s", Version), map[string]any{
				"version": Version,
			})
		},
	})
}

// defaultOutputFormat lets a harness ask for JSON once, through NOTTE_OUTPUT,
// instead of on every command. --output still wins.
func defaultOutputFormat() string {
	if os.Getenv(config.EnvOutput) == string(output.FormatJSON) {
		return string(output.FormatJSON)
	}
	return string(output.FormatText)
}

// GetFormatter returns the appropriate formatter based on flags
func GetFormatter() output.Formatter {
	format := output.Format(outputFormat)
	f := output.NewFormatter(format, os.Stdout)
	if tf, ok := f.(*output.TextFormatter); ok {
		tf.NoColor = noColor
	}
	return f
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	return verbose
}

// GetClient creates an authenticated API client
func GetClient() (*api.NotteClient, error) {
	apiKey, source, err := auth.GetAPIKey("")
	if err != nil {
		return nil, err
	}

	baseURL := os.Getenv(config.EnvAPIURL)
	if baseURL == "" {
		cfg, err := config.Load()
		if err != nil {
			return nil, err
		}
		baseURL = cfg.APIURL
	}

	if baseURL == "" {
		baseURL = api.DefaultBaseURL
	}
	if source != auth.SourceEnv {
		if err := auth.CheckSavedKeyURL(baseURL); err != nil {
			return nil, err
		}
	}

	var opts []api.NotteClientOption
	if origin := os.Getenv(config.EnvRequestOrigin); origin != "" {
		opts = append(opts, api.WithRequestOrigin(origin))
	}

	client, err := api.NewClientWithURL(apiKey, baseURL, Version, opts...)
	if err != nil {
		return nil, err
	}
	// Commands enforce --timeout through their request contexts. The client's
	// shorter default must not silently override a caller's explicit timeout.
	client.HTTPClient().Timeout = time.Duration(requestTimeout) * time.Second
	return client, nil
}

// GetContextWithTimeout wraps the provided context with a timeout
func GetContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(requestTimeout)*time.Second)
}

// boolPtr returns a pointer to the given bool value
func boolPtr(b bool) *bool {
	return &b
}
