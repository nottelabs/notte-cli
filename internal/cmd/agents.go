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

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage AI agents",
	Long:  "List agents and start new agent tasks.",
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

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsStartCmd)

	agentsStartCmd.Flags().StringVar(&agentsStartTask, "task", "", "Task for the agent (required)")
	agentsStartCmd.Flags().StringVar(&agentsStartSession, "session", "", "Session ID to use")
	agentsStartCmd.Flags().StringVar(&agentsStartVault, "vault", "", "Vault ID for credentials")
	agentsStartCmd.Flags().StringVar(&agentsStartPersona, "persona", "", "Persona ID to use")
	agentsStartCmd.Flags().IntVar(&agentsStartMaxSteps, "max-steps", 30, "Maximum steps")
	agentsStartCmd.Flags().StringVar(&agentsStartReasoningModel, "reasoning-model", "", "Reasoning model to use")
	_ = agentsStartCmd.MarkFlagRequired("task")
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		fmt.Println("No running agents.")
		return nil
	}

	return GetFormatter().Print(resp.JSON200.Items)
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}
