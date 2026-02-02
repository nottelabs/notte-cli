package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
)

var agentID string

// GetCurrentAgentID returns the agent ID from flag, env var, or file (in priority order)
func GetCurrentAgentID() string {
	if agentID != "" {
		return agentID
	}
	if envID := os.Getenv(config.EnvAgentID); envID != "" {
		return envID
	}
	configDir, err := config.Dir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, config.CurrentAgentFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func setCurrentAgent(id string) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, config.CurrentAgentFile), []byte(id), 0o600)
}

func clearCurrentAgent() error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RequireAgentID ensures an agent ID is available from flag, env, or file
func RequireAgentID() error {
	agentID = GetCurrentAgentID()
	if agentID == "" {
		return errors.New("agent ID required: use --id flag, set NOTTE_AGENT_ID env var, or start an agent first")
	}
	return nil
}

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

	// Start command flags (auto-generated)
	RegisterAgentStartFlags(agentsStartCmd)
	_ = agentsStartCmd.MarkFlagRequired("task")

	// Status command flags
	agentsStatusCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (uses current agent if not specified)")

	// Stop command flags
	agentsStopCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (uses current agent if not specified)")

	// Workflow-code command flags
	agentsWorkflowCodeCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (uses current agent if not specified)")

	// Replay command flags
	agentsReplayCmd.Flags().StringVar(&agentID, "id", "", "Agent ID (uses current agent if not specified)")
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

	// Build request body from generated flags
	body, err := BuildAgentStartRequest(cmd)
	if err != nil {
		return err
	}

	// Auto-use current session ID if --session-id not provided
	if body.SessionId == "" {
		if currentSessionID := GetCurrentSessionID(); currentSessionID != "" {
			body.SessionId = currentSessionID
		}
	}

	params := &api.AgentStartParams{}
	resp, err := client.Client().AgentStartWithResponse(ctx, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// Save agent ID as current agent
	if resp.JSON200 != nil {
		if err := setCurrentAgent(resp.JSON200.AgentId); err != nil {
			PrintInfo(fmt.Sprintf("Warning: could not save current agent: %v", err))
		}
	}

	return GetFormatter().Print(resp.JSON200)
}

func runAgentStatus(cmd *cobra.Command, args []string) error {
	if err := RequireAgentID(); err != nil {
		return err
	}

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
	if err := RequireAgentID(); err != nil {
		return err
	}

	confirmed, err := ConfirmStop("agent", agentID)
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

	// Use current session ID for the stop request
	params := &api.AgentStopParams{
		SessionId: GetCurrentSessionID(),
	}
	resp, err := client.Client().AgentStopWithResponse(ctx, agentID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// Clear current agent only if it matches the stopped agent
	configDir, _ := config.Dir()
	if configDir != "" {
		data, _ := os.ReadFile(filepath.Join(configDir, config.CurrentAgentFile))
		if strings.TrimSpace(string(data)) == agentID {
			_ = clearCurrentAgent()
		}
	}

	return PrintResult(fmt.Sprintf("Agent %s stopped.", agentID), map[string]any{
		"id":     agentID,
		"status": "stopped",
	})
}

func runAgentWorkflowCode(cmd *cobra.Command, args []string) error {
	if err := RequireAgentID(); err != nil {
		return err
	}

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
	if err := RequireAgentID(); err != nil {
		return err
	}

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
