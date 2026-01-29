package cmd

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
	"github.com/salmonumbrella/notte-cli/internal/auth"
	"github.com/salmonumbrella/notte-cli/internal/config"
	"github.com/salmonumbrella/notte-cli/internal/output"
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
	Short: "CLI for notte.cc browser agent platform",
	Long: `notte-cli provides command-line access to the notte.cc platform
for browser automation, AI agents, and web scraping.

Get started:
  notte auth login        # Authenticate with your API key
  notte sessions start    # Start a browser session
  notte scrape <url>      # Quick scrape a webpage`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		formatter := GetFormatter()
		formatter.PrintError(err)
		os.Exit(1)
	}
}

func init() {
	// Hide completion command from help output (still accessible via `notte completion`)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, json)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().IntVar(&requestTimeout, "timeout", 30, "API request timeout in seconds")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "Skip confirmation prompts")

	// Set up confirmation state before each command
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		SetSkipConfirmation(yesFlag)
	}

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("notte version %s\n", Version)
		},
	})
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
	apiKey, _, err := auth.GetAPIKey("")
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

	return api.NewClientWithURL(apiKey, baseURL)
}

// GetContextWithTimeout wraps the provided context with a timeout
func GetContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(requestTimeout)*time.Second)
}
