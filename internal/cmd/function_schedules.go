package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var (
	functionScheduleVariables     []string // Variables as key=value pairs
	functionScheduleVariablesJSON string   // Variables as JSON string
)

var functionSchedulesCmd = &cobra.Command{
	Use:   "schedules",
	Short: "Manage the function's schedules, each with its own variables",
	Long: `A function can run on several schedules, each passing its own variables to
its runs. Pausing applies to all of them: "resume" restarts a paused function's
schedules without changing them.

"notte functions schedule" replaces every schedule with one, and
"notte functions unschedule" removes them all.`,
}

var functionSchedulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the function's schedules",
	Args:  cobra.NoArgs,
	RunE:  runFunctionSchedulesList,
}

var functionSchedulesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a schedule to the function",
	Args:  cobra.NoArgs,
	Example: `  # Every weekday at 9:00 UTC, for Paris
  notte functions schedules add --cron "0 9 ? * MON-FRI *" --var city=paris

  # Every day at 18:00 UTC, with JSON variables
  notte functions schedules add --cron "0 18 ? * * *" --vars '{"city": "tokyo", "limit": 5}'`,
	RunE: runFunctionSchedulesAdd,
}

var functionSchedulesUpdateCmd = &cobra.Command{
	Use:   "update <schedule-id>",
	Short: "Change a schedule's cron expression or variables",
	Long: `Change a schedule's cron expression or variables. What is not given keeps its
current value: --cron alone keeps the variables, and --var or --vars alone keeps
the cron expression. Given variables replace all of the schedule's variables.`,
	Args: cobra.ExactArgs(1),
	RunE: runFunctionSchedulesUpdate,
}

var functionSchedulesRemoveCmd = &cobra.Command{
	Use:     "remove <schedule-id>",
	Aliases: []string{"rm"},
	Short:   "Remove one schedule from the function",
	Args:    cobra.ExactArgs(1),
	RunE:    runFunctionSchedulesRemove,
}

var functionSchedulesResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the function's paused schedules without changing them",
	Args:  cobra.NoArgs,
	RunE:  runFunctionSchedulesResume,
}

func init() {
	functionsCmd.AddCommand(functionSchedulesCmd)
	for _, c := range []*cobra.Command{
		functionSchedulesListCmd, functionSchedulesAddCmd, functionSchedulesUpdateCmd,
		functionSchedulesRemoveCmd, functionSchedulesResumeCmd,
	} {
		functionSchedulesCmd.AddCommand(c)
		c.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	}

	RegisterFunctionSchedulesAddFlags(functionSchedulesAddCmd)
	_ = functionSchedulesAddCmd.MarkFlagRequired("cron")
	RegisterFunctionSchedulesUpdateFlags(functionSchedulesUpdateCmd)
	for _, c := range []*cobra.Command{functionSchedulesAddCmd, functionSchedulesUpdateCmd} {
		c.Flags().StringArrayVar(&functionScheduleVariables, "var", []string{}, "Variable as key=value pair (can be used multiple times)")
		c.Flags().StringVar(&functionScheduleVariablesJSON, "vars", "", "Variables as JSON object string")
	}
}

// parseVariables reads --vars (a JSON object), then --var key=value pairs,
// which override it.
func parseVariables(variablesJSON string, pairs []string) (map[string]interface{}, error) {
	variables := make(map[string]interface{})
	if variablesJSON != "" {
		if err := json.Unmarshal([]byte(variablesJSON), &variables); err != nil {
			return nil, fmt.Errorf("failed to parse --vars JSON: %w", err)
		}
	}
	if variables == nil {
		return nil, errors.New("--vars must be a JSON object")
	}
	for _, kv := range pairs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid variable format %q: expected key=value", kv)
		}
		variables[parts[0]] = parts[1]
	}
	return variables, nil
}

func scheduleVariablesGiven(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("var") || cmd.Flags().Changed("vars")
}

func listFunctionSchedules(ctx context.Context, client *api.NotteClient) (*api.FunctionScheduleListResponse, error) {
	resp, err := client.Client().FunctionSchedulesListWithResponse(ctx, functionID, &api.FunctionSchedulesListParams{})
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return &api.FunctionScheduleListResponse{}, nil
	}
	return resp.JSON200, nil
}

func runFunctionSchedulesList(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	list, err := listFunctionSchedules(ctx, client)
	if err != nil {
		return err
	}
	if printed, err := PrintListOrEmpty(list.Schedules, "No schedules found."); err != nil {
		return err
	} else if printed {
		return nil
	}
	return GetFormatter().Print(list)
}

func runFunctionSchedulesAdd(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}
	body, err := BuildFunctionSchedulesAddRequest(cmd)
	if err != nil {
		return err
	}
	// `variables` is required by the API; no variables is an empty object.
	variables, err := parseVariables(functionScheduleVariablesJSON, functionScheduleVariables)
	if err != nil {
		return err
	}
	body.Variables = &variables

	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	resp, err := client.Client().FunctionSchedulesAddWithResponse(ctx, functionID, &api.FunctionSchedulesAddParams{}, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}
	return GetFormatter().Print(resp.JSON200)
}

func runFunctionSchedulesUpdate(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}
	scheduleID := args[0]
	if !cmd.Flags().Changed("cron") && !scheduleVariablesGiven(cmd) {
		return errors.New("nothing to update: pass --cron, --var or --vars")
	}
	body, err := BuildFunctionSchedulesUpdateRequest(cmd)
	if err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// The API replaces both the cron expression and the variables: fill in
	// the current value of whichever was not given.
	if !cmd.Flags().Changed("cron") || !scheduleVariablesGiven(cmd) {
		list, err := listFunctionSchedules(ctx, client)
		if err != nil {
			return err
		}
		var current *api.FunctionSchedule
		for i := range list.Schedules {
			if list.Schedules[i].ScheduleId == scheduleID {
				current = &list.Schedules[i]
			}
		}
		if current == nil {
			return fmt.Errorf("schedule %s not found on function %s", scheduleID, functionID)
		}
		if !cmd.Flags().Changed("cron") {
			body.Cron = current.Cron
		}
		if !scheduleVariablesGiven(cmd) {
			body.Variables = current.Variables
		}
	}
	if scheduleVariablesGiven(cmd) {
		variables, err := parseVariables(functionScheduleVariablesJSON, functionScheduleVariables)
		if err != nil {
			return err
		}
		body.Variables = &variables
	}
	if body.Variables == nil {
		empty := make(map[string]interface{})
		body.Variables = &empty
	}

	resp, err := client.Client().FunctionSchedulesUpdateWithResponse(ctx, functionID, scheduleID, &api.FunctionSchedulesUpdateParams{}, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}
	return GetFormatter().Print(resp.JSON200)
}

func runFunctionSchedulesRemove(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}
	scheduleID := args[0]
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	resp, err := client.Client().FunctionSchedulesRemoveWithResponse(ctx, functionID, scheduleID, &api.FunctionSchedulesRemoveParams{})
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}
	return PrintResult(fmt.Sprintf("Schedule %s removed from function %s.", scheduleID, functionID), map[string]any{
		"id":          functionID,
		"schedule_id": scheduleID,
		"status":      "removed",
	})
}

func runFunctionSchedulesResume(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	resp, err := client.Client().FunctionSchedulesResumeWithResponse(ctx, functionID, &api.FunctionSchedulesResumeParams{})
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}
	return GetFormatter().Print(resp.JSON200)
}
