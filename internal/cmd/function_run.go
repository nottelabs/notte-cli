package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

const functionPollInterval = 2 * time.Second

func functionWaitContext(cmd *cobra.Command) (context.Context, context.CancelFunc, error) {
	var timeout time.Duration
	if cmd.Flags().Lookup("wait-timeout") != nil {
		timeout, _ = cmd.Flags().GetDuration("wait-timeout")
	}
	if timeout < 0 {
		return nil, nil, errors.New("--wait-timeout must not be negative")
	}
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		return ctx, cancel, nil
	}
	ctx, cancel := context.WithCancel(cmd.Context())
	return ctx, cancel, nil
}

func functionRunHint(id, runID string) string {
	return fmt.Sprintf("notte functions run-metadata --wait --function-id %s --run-id %s", id, runID)
}

func functionWaitError(id, runID string, err error) error {
	return fmt.Errorf("stopped waiting for function run %s: %w; the CLI did not stop the function. Resume with `%s`", runID, err, functionRunHint(id, runID))
}

func startAndWaitFunction(cmd *cobra.Command, client *api.NotteClient, variables map[string]interface{}) error {
	ctx, cancel, err := functionWaitContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()

	runID, result, err := startFunction(ctx, client, functionID, functionRunRuntime, variables)
	if err != nil {
		if runID != "" {
			return fmt.Errorf("could not confirm startup of function run %s: %w; it may still be executing. Check it with `%s` before starting another run", runID, err, functionRunHint(functionID, runID))
		}
		return fmt.Errorf("function start request failed: %w; a run may have been created. Check `notte functions runs --function-id %s` before retrying", err, functionID)
	}
	// Older servers can return a complete JSON response directly.
	if result != nil {
		return GetFormatter().Print(result)
	}
	if functionRunNoWait {
		return GetFormatter().Print(map[string]any{
			"function_id":     functionID,
			"function_run_id": runID,
			"status":          "active",
		})
	}
	return waitFunction(ctx, client, functionID, runID, functionRunNoStream, functionPollInterval)
}

// The start endpoint redirects to the runner with a pre-created run ID. The
// runner executes independently, but stream:false holds the response until it
// finishes. Request streaming even with --no-stream, wait for HTTP acceptance,
// then close the connection and follow the durable run through metadata GETs.
func startFunction(ctx context.Context, client *api.NotteClient, id, runtime string, variables map[string]interface{}) (runID string, result any, err error) {
	ctx, cancel := GetContextWithTimeout(ctx)
	defer cancel()
	requestBody := map[string]any{"workflow_id": id, "variables": variables, "stream": true}
	// Omit the override to inherit the function's saved default runtime.
	if runtime != "" {
		requestBody["runtime"] = runtime
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.BaseURL(), "/")+"/functions/"+url.PathEscape(id)+"/runs/start", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-notte-api-key", client.APIKey())

	httpClient := *client.HTTPClient()
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if value := req.URL.Query().Get("function_run_id"); value != "" && runID == "" {
			runID = value
			// Always stderr: JSON stdout remains a single final response.
			_, _ = fmt.Fprintf(os.Stderr, "Function run %s created. Track it with `%s`.\n", runID, functionRunHint(id, runID))
		}
		return nil
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return runID, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return runID, nil, err
		}
		if err := HandleAPIResponse(resp, body); err != nil {
			return runID, nil, err
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return runID, nil, fmt.Errorf("invalid start response: %w", err)
		}
		if result == nil {
			return runID, nil, errors.New("empty start response")
		}
		return runID, result, nil
	}
	if runID == "" {
		return "", nil, errors.New("server did not provide a function_run_id in its startup redirect")
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") && !strings.HasPrefix(contentType, "text/event-stream") {
		return runID, nil, fmt.Errorf("unexpected runner content type %q", contentType)
	}
	// Do returns after response headers. Do not wait for a log event: a quiet
	// function may not emit one until it finishes, recreating the same timeout.
	// Execution errors and results are retrieved from durable run metadata.
	return runID, nil, nil
}

func runFunctionRunWait(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}
	if functionRunID == "" {
		return errors.New("--run-id is required")
	}
	ctx, cancel, err := functionWaitContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()
	client, err := GetClient()
	if err != nil {
		return err
	}
	noStream, _ := cmd.Flags().GetBool("no-stream")
	return waitFunction(ctx, client, functionID, functionRunID, noStream, functionPollInterval)
}

func waitFunction(ctx context.Context, client *api.NotteClient, id, runID string, noStream bool, interval time.Duration) error {
	logsPrinted := 0
	for {
		requestCtx, cancel := GetContextWithTimeout(ctx)
		resp, err := client.Client().FunctionRunGetMetadataWithResponse(requestCtx, id, runID, &api.FunctionRunGetMetadataParams{})
		cancel()
		if err != nil {
			return functionWaitError(id, runID, err)
		}
		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return functionWaitError(id, runID, err)
		}
		run := resp.JSON200
		if run == nil || run.FunctionRunId != runID || run.FunctionId != id {
			return functionWaitError(id, runID, errors.New("missing or mismatched run metadata"))
		}
		if !noStream && run.Logs != nil {
			for ; logsPrinted < len(*run.Logs); logsPrinted++ {
				_, _ = fmt.Fprintln(os.Stderr, (*run.Logs)[logsPrinted])
			}
		}
		switch run.Status {
		case api.GetFunctionRunResponseStatusClosed, api.GetFunctionRunResponseStatusFailed:
			return GetFormatter().Print(run)
		case api.GetFunctionRunResponseStatusActive:
		default:
			return functionWaitError(id, runID, fmt.Errorf("unrecognized run status %q", run.Status))
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return functionWaitError(id, runID, ctx.Err())
		case <-timer.C:
		}
	}
}
