package cmd

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var (
	searchDepth      string
	searchOutputType string

	validSearchDepths      = map[string]bool{"standard": true, "fast": true, "deep": true}
	validSearchOutputTypes = map[string]bool{"searchResults": true, "sourcedAnswer": true, "structured": true}

	// termenvOutput is shared so we don't allocate a fresh termenv.Output for
	// every colorize call (the color profile / TTY detection doesn't change
	// during a single command run).
	termenvOutput = termenv.NewOutput(os.Stdout)
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the web",
	Long: `Search the web via the Notte search API.

Returns either a list of search results, or an LLM-generated answer with sources,
depending on --output-type.

Examples:
  notte search "what is anthropic"
  notte search what is anthropic
  notte search "latest llm releases" --depth deep
  notte search "what is anthropic" --output-type sourcedAnswer`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVar(&searchDepth, "depth", "", "Search depth: standard, fast, or deep")
	searchCmd.Flags().StringVar(&searchOutputType, "output-type", "", "Output type: searchResults, sourcedAnswer, or structured")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}
	if searchDepth != "" && !validSearchDepths[searchDepth] {
		return fmt.Errorf("invalid --depth %q: must be standard, fast, or deep", searchDepth)
	}
	if searchOutputType != "" && !validSearchOutputTypes[searchOutputType] {
		return fmt.Errorf("invalid --output-type %q: must be searchResults, sourcedAnswer, or structured", searchOutputType)
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.SearchRequest{Q: query}
	if searchDepth != "" {
		body.Depth = &searchDepth
	}
	if searchOutputType != "" {
		body.OutputType = &searchOutputType
	}

	resp, err := client.Client().SearchWebWithResponse(ctx, &api.SearchWebParams{}, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return printSearchResponse(resp.Body, query)
}

// printSearchResponse renders the raw search response body. The /search endpoint
// returns different shapes depending on outputType, so we decode loosely and
// render whichever fields are present.
func printSearchResponse(body []byte, query string) error {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Fall back to printing the raw body if it isn't a JSON object
		if IsJSONOutput() {
			_, _ = os.Stdout.Write(body)
			if len(body) == 0 || body[len(body)-1] != '\n' {
				fmt.Println()
			}
			return nil
		}
		fmt.Println(string(body))
		return nil
	}

	if IsJSONOutput() {
		return GetFormatter().Print(parsed)
	}

	// sourcedAnswer shape: { "answer": "...", "sources": [...] }
	if answer, ok := parsed["answer"].(string); ok {
		printAnswer(query, answer)
		if sources, ok := parsed["sources"].([]any); ok && len(sources) > 0 {
			fmt.Println()
			printSources(sources)
		}
		return nil
	}

	// searchResults shape: { "results": [...] }
	if results, ok := parsed["results"].([]any); ok {
		if len(results) == 0 {
			fmt.Printf("No results for %q.\n", query)
			return nil
		}
		printResults(query, results)
		return nil
	}

	// Unknown shape - fall back to formatter
	return GetFormatter().Print(parsed)
}

func printAnswer(query, answer string) {
	header := colorizeText(fmt.Sprintf("Answer for %q:", query), termenv.ANSICyan)
	fmt.Println(header)
	fmt.Println()
	fmt.Println(html.UnescapeString(answer))
}

func printSources(sources []any) {
	header := colorizeText("Sources:", termenv.ANSICyan)
	fmt.Println(header)
	for i, raw := range sources {
		src, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		title := stringField(src, "name", "title")
		url := stringField(src, "url")
		snippet := stringField(src, "snippet", "content")

		num := fmt.Sprintf("%d.", i+1)
		fmt.Printf("%s %s\n", colorizeText(num, termenv.ANSIYellow), html.UnescapeString(title))
		if url != "" {
			fmt.Printf("   %s\n", colorizeText(url, termenv.ANSIBlue))
		}
		if snippet != "" {
			fmt.Printf("   %s\n", truncate(snippet, 240))
		}
	}
}

func printResults(query string, results []any) {
	header := colorizeText(fmt.Sprintf("Search results for %q (%d):", query, len(results)), termenv.ANSICyan)
	fmt.Println(header)
	fmt.Println()
	for i, raw := range results {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		title := stringField(item, "name", "title")
		url := stringField(item, "url")
		content := stringField(item, "content", "snippet", "description")

		num := fmt.Sprintf("%d.", i+1)
		fmt.Printf("%s %s\n", colorizeText(num, termenv.ANSIYellow), html.UnescapeString(title))
		if url != "" {
			fmt.Printf("   %s\n", colorizeText(url, termenv.ANSIBlue))
		}
		if content != "" {
			fmt.Printf("   %s\n", truncate(content, 240))
		}
		if i < len(results)-1 {
			fmt.Println()
		}
	}
}

// stringField returns the first non-empty string value among the given keys.
func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, maxRunes int) string {
	// Decode HTML entities (the API returns &#x27;, &amp;, etc.) so the text
	// reads naturally in the terminal. JSON output stays raw.
	s = html.UnescapeString(s)
	s = strings.TrimSpace(s)
	// Collapse internal whitespace/newlines so multi-line snippets render on one line
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}

// colorizeText applies a color via the shared termenv output, respecting --no-color.
func colorizeText(s string, color termenv.ANSIColor) string {
	if noColor {
		return s
	}
	return termenvOutput.String(s).Foreground(color).String()
}
