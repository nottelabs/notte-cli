package cmd

import (
	"fmt"

	"github.com/salmonumbrella/notte-cli/internal/api"
	"github.com/spf13/cobra"
)

var agentID string

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Operate on a specific agent",
	Long:  "Subcommands for interacting with a specific agent. Use --id flag to specify the agent ID.",
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get agent status",
	RunE:  runAgentStatus,
}

var agentStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the agent",
	RunE:  runAgentStop,
}

var agentWorkflowCodeCmd = &cobra.Command{
	Use:   "workflow-code",
	Short: "Export agent steps as code",
	RunE:  runAgentWorkflowCode,
}

var agentReplayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Get replay data for the agent",
	RunE:  runAgentReplay,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentStatusCmd)
	agentCmd.AddCommand(agentStopCmd)
	agentCmd.AddCommand(agentWorkflowCodeCmd)
	agentCmd.AddCommand(agentReplayCmd)

	// Persistent flag for agent ID (required for all subcommands)
	agentCmd.PersistentFlags().StringVar(&agentID, "id", "", "Agent ID (required)")
	_ = agentCmd.MarkPersistentFlagRequired("id")
}

func runAgentStatus(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.AgentStatusParams{}
	resp, err := client.Client().AgentStatusWithResponse(ctx, agentID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runAgentStop(cmd *cobra.Command, args []string) error {
	confirmed, err := ConfirmAction("agent", agentID)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled.")
		return nil
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.AgentStopParams{
		SessionId: "", // Session ID is required but can be empty
	}
	resp, err := client.Client().AgentStopWithResponse(ctx, agentID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Agent %s stopped.\n", agentID)
	return nil
}

func runAgentWorkflowCode(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.GetScriptParams{
		AsWorkflow: true, // Return as standalone workflow
	}
	resp, err := client.Client().GetScriptWithResponse(ctx, agentID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runAgentReplay(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.AgentReplayParams{}
	resp, err := client.Client().AgentReplayWithResponse(ctx, agentID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	// Wrap raw body for formatter compatibility
	result := map[string]interface{}{
		"agent_id":    agentID,
		"replay_data": string(resp.Body),
	}
	return GetFormatter().Print(result)
}
