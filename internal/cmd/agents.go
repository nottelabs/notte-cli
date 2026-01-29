package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var (
	agentsStartTask           string
	agentsStartSession        string
	agentsStartVault          string
	agentsStartPersona        string
	agentsStartMaxSteps       int
	agentsStartReasoningModel string
)

var agentID string

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage AI agents",
	Long:  "List, start, and operate on AI agents.",
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List running agents",
	RunE:  runAgentsList,
}

var agentsStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new agent task",
	RunE:  runAgentsStart,
}

var agentsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get agent status",
	RunE:  runAgentStatus,
}

var agentsStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the agent",
	RunE:  runAgentStop,
}

var agentsWorkflowCodeCmd = &cobra.Command{
	Use:   "workflow-code",
	Short: "Export agent steps as code",
	RunE:  runAgentWorkflowCode,
}

var agentsReplayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Get replay data for the agent",
	RunE:  runAgentReplay,
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsStartCmd)
	agentsCmd.AddCommand(agentsStatusCmd)
	agentsCmd.AddCommand(agentsStopCmd)
	agentsCmd.AddCommand(agentsWorkflowCodeCmd)
	agentsCmd.AddCommand(agentsReplayCmd)

	// Start command flags
	agentsStartCmd.Flags().StringVar(&agentsStartTask, "task", "", "Task for the agent (required)")
	agentsStartCmd.Flags().StringVar(&agentsStartSession, "session", "", "Session ID to use")
	agentsStartCmd.Flags().StringVar(&agentsStartVault, "vault", "", "Vault ID for credentials")
	agentsStartCmd.Flags().StringVar(&agentsStartPersona, "persona", "", "Persona ID to use")
	agentsStartCmd.Flags().IntVar(&agentsStartMaxSteps, "max-steps", 30, "Maximum steps")
	agentsStartCmd.Flags().StringVar(&agentsStartReasoningModel, "reasoning-model", "", "Reasoning model to use")
	_ = agentsStartCmd.MarkFlagRequired("task")

	// Status command flags
	agentsStatusCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (required)")
	_ = agentsStatusCmd.MarkFlagRequired("id")

	// Stop command flags
	agentsStopCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (required)")
	_ = agentsStopCmd.MarkFlagRequired("id")

	// Workflow-code command flags
	agentsWorkflowCodeCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (required)")
	_ = agentsWorkflowCodeCmd.MarkFlagRequired("id")

	// Replay command flags
	agentsReplayCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (required)")
	_ = agentsReplayCmd.MarkFlagRequired("id")
}

func runAgentsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListAgentsParams{}
	resp, err := client.Client().ListAgentsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	var items []api.AgentResponse
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No running agents."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return GetFormatter().Print(items)
}

func runAgentsStart(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.AgentStartJSONRequestBody{
		Task:      agentsStartTask,
		SessionId: agentsStartSession,
		MaxSteps:  &agentsStartMaxSteps,
	}

	if agentsStartVault != "" {
		body.VaultId = &agentsStartVault
	}
	if agentsStartPersona != "" {
		body.PersonaId = &agentsStartPersona
	}
	if agentsStartReasoningModel != "" {
		reasoningModel := &api.ApiAgentStartRequest_ReasoningModel{}
		if err := reasoningModel.FromApiAgentStartRequestReasoningModel1(agentsStartReasoningModel); err != nil {
			return fmt.Errorf("failed to set reasoning model: %w", err)
		}
		body.ReasoningModel = reasoningModel
	}

	params := &api.AgentStartParams{}
	resp, err := client.Client().AgentStartWithResponse(ctx, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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
		return PrintResult("Cancelled.", map[string]any{"cancelled": true})
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Agent %s stopped.", agentID), map[string]any{
		"id":     agentID,
		"status": "stopped",
	})
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
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

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// Wrap raw body for formatter compatibility
	result := map[string]interface{}{
		"agent_id":    agentID,
		"replay_data": string(resp.Body),
	}
	return GetFormatter().Print(result)
}
