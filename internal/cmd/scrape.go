package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var (
	scrapeInstructions string
	scrapeOnlyMain     bool

	scrapeHtmlFile         string
	scrapeHtmlInstructions string
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape <url>",
	Short: "Scrape a webpage",
	Long:  "Quick scrape a webpage without creating a session.",
	Args:  cobra.ExactArgs(1),
	RunE:  runScrape,
}

var scrapeHtmlCmd = &cobra.Command{
	Use:   "scrape-html",
	Short: "Scrape content from an HTML file",
	Long:  "Quick scrape content from a local HTML file without creating a session.",
	RunE:  runScrapeHtml,
}

func init() {
	rootCmd.AddCommand(scrapeCmd)
	rootCmd.AddCommand(scrapeHtmlCmd)

	scrapeCmd.Flags().StringVar(&scrapeInstructions, "instructions", "", "Extraction instructions")
	scrapeCmd.Flags().BoolVar(&scrapeOnlyMain, "only-main-content", false, "Only main content")

	scrapeHtmlCmd.Flags().StringVar(&scrapeHtmlFile, "file", "", "Path to HTML file (required)")
	_ = scrapeHtmlCmd.MarkFlagRequired("file")
	scrapeHtmlCmd.Flags().StringVar(&scrapeHtmlInstructions, "instructions", "", "Extraction instructions")
}

func runScrape(cmd *cobra.Command, args []string) error {
	url := args[0]

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.ScrapeWebpageJSONRequestBody{
		Url: url,
	}

	hasInstructions := scrapeInstructions != ""
	if hasInstructions {
		body.Instructions = &scrapeInstructions
	}
	if scrapeOnlyMain {
		body.OnlyMainContent = &scrapeOnlyMain
	}

	resp, err := client.Client().ScrapeWebpageWithResponse(ctx, nil, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintScrapeResponse(resp.JSON200, hasInstructions)
}

func runScrapeHtml(cmd *cobra.Command, args []string) error {
	// Read the HTML file
	htmlContent, err := os.ReadFile(scrapeHtmlFile)
	if err != nil {
		return fmt.Errorf("failed to read HTML file: %w", err)
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// Create the frame data with the HTML content
	frameUrl := "file://" + scrapeHtmlFile
	frameData := api.FrameData{
		FrameData: string(htmlContent),
		FrameUrl:  frameUrl,
	}
	frames := []api.FrameData{frameData}

	// Build the request body
	body := api.ScrapeFromHtmlJSONRequestBody{
		Frames: &frames,
	}

	if scrapeHtmlInstructions != "" {
		body.Instructions = &scrapeHtmlInstructions
	}

	resp, err := client.Client().ScrapeFromHtmlWithResponse(ctx, nil, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// ScrapeFromHtml returns ScrapeSchemaResponse which has a different structure
	// Just print the Scrape field which contains the extracted data
	if IsJSONOutput() {
		return GetFormatter().Print(resp.JSON200)
	}
	return GetFormatter().Print(resp.JSON200.Scrape)
}
