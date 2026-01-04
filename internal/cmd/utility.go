package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var promptText string

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check API health status",
	Long:  "Check if the Notte API is healthy and responding.",
	RunE:  runHealth,
}

var promptImproveCmd = &cobra.Command{
	Use:   "prompt-improve",
	Short: "Improve a prompt using AI",
	Long:  "Use AI to improve and enhance a prompt for better results.",
	RunE:  runPromptImprove,
}

var promptNudgeCmd = &cobra.Command{
	Use:   "prompt-nudge",
	Short: "Get suggestions to nudge a prompt",
	Long:  "Get AI-generated suggestions to improve or refine your prompt.",
	RunE:  runPromptNudge,
}

func init() {
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(promptImproveCmd)
	rootCmd.AddCommand(promptNudgeCmd)

	// Add --text flag to prompt commands
	promptImproveCmd.Flags().StringVar(&promptText, "text", "", "Prompt text to improve (required)")
	_ = promptImproveCmd.MarkFlagRequired("text")

	promptNudgeCmd.Flags().StringVar(&promptText, "text", "", "Prompt text to get nudges for (required)")
	_ = promptNudgeCmd.MarkFlagRequired("text")
}

func runHealth(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	resp, err := client.Client().HealthCheckWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runPromptImprove(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.ImprovePromptJSONRequestBody{
		Prompt: promptText,
	}

	resp, err := client.Client().ImprovePromptWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runPromptNudge(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// NudgePromptRequest requires task (string) and agent_messages (array)
	// For a simple CLI command, we'll use the text as the task and empty messages
	body := api.NudgePromptJSONRequestBody{
		Task:          promptText,
		AgentMessages: []map[string]interface{}{},
	}

	resp, err := client.Client().NudgePromptWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}
