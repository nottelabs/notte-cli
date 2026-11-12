package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/output"
)

// stopAllSessions stops every active session, in parallel, after one
// confirmation. Its default is no, unlike stopping a single session, since
// pressing enter by mistake would stop everything.
func stopAllSessions(parent context.Context, in io.Reader, prompt io.Writer) error {
	client, err := GetClient()
	if err != nil {
		return err
	}
	listCtx, cancelList := GetContextWithTimeout(parent)
	ids, err := activeSessionIDs(listCtx, client)
	cancelList()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		if IsJSONOutput() {
			return GetFormatter().Print(map[string]any{"stopped": []string{}})
		}
		_, err := fmt.Println("No active sessions " + output.FaceEmpty)
		return err
	}

	if !skipConfirmation {
		noun := "sessions"
		if len(ids) == 1 {
			noun = "session"
		}
		if in == os.Stdin && !canPrompt() {
			return &confirmationError{action: fmt.Sprintf("stop %d active %s", len(ids), noun)}
		}
		if _, err := fmt.Fprintf(prompt, "Stop %d active %s? [y/N]: ", len(ids), noun); err != nil {
			return err
		}
		answer, _ := bufio.NewReader(in).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			return PrintResult("Cancelled.", map[string]any{"cancelled": true})
		}
	}

	// The timeout starts here, not before the prompt: time spent answering
	// must not leave the stop requests with an expired context.
	ctx, cancel := GetContextWithTimeout(parent)
	defer cancel()

	failures := make([]error, len(ids))
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			resp, err := client.Client().SessionStopWithResponse(ctx, id, &api.SessionStopParams{})
			if err == nil {
				err = HandleAPIResponse(resp.HTTPResponse, resp.Body)
			}
			if err != nil {
				failures[i] = fmt.Errorf("%s: %w", id, err)
			}
		}()
	}
	wg.Wait()

	var stopped []string
	var failed []error
	for i, id := range ids {
		if failures[i] != nil {
			failed = append(failed, failures[i])
		} else {
			stopped = append(stopped, id)
		}
	}
	// The pinned session is gone if it was one of them.
	pinned := GetCurrentSessionID()
	for _, id := range stopped {
		if id == pinned {
			_ = clearCurrentSession()
			_ = clearCurrentViewerURL()
			_ = clearCurrentSessionExpiry()
		}
	}

	message := fmt.Sprintf("Stopped %d sessions", len(stopped))
	if len(stopped) == 1 {
		message = "Stopped 1 session"
	}
	if err := printDone(message, map[string]any{"stopped": stopped}, "notte sessions list --all"); err != nil {
		return err
	}
	if len(failed) > 0 {
		for _, err := range failed {
			PrintInfo(fmt.Sprintf("Warning: could not stop session %v", err))
		}
		return &partialError{message: fmt.Sprintf("stopped %d of %d sessions", len(stopped), len(ids)), causes: failed}
	}
	return nil
}

// maxSessionPages stops a server that keeps reporting another page from
// looping forever. Reaching it is an error, never a silent truncation.
const maxSessionPages = 1000

// activeSessions lists every active session, following pages.
func activeSessions(ctx context.Context, client *api.NotteClient) ([]api.SessionResponse, error) {
	var sessions []api.SessionResponse
	onlyActive, pageSize := true, 100
	for page := 1; ; page++ {
		if page > maxSessionPages {
			return nil, fmt.Errorf("stopped listing sessions after %d pages; more remain", maxSessionPages)
		}
		resp, err := client.Client().ListSessionsWithResponse(ctx, &api.ListSessionsParams{OnlyActive: &onlyActive, Page: &page, PageSize: &pageSize})
		if err != nil {
			return nil, fmt.Errorf("API request failed: %w", err)
		}
		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return sessions, nil
		}
		sessions = append(sessions, resp.JSON200.Items...)
		if !resp.JSON200.HasNext || len(resp.JSON200.Items) == 0 {
			return sessions, nil
		}
	}
}

// activeSessionIDs is activeSessions reduced to the IDs.
func activeSessionIDs(ctx context.Context, client *api.NotteClient) ([]string, error) {
	sessions, err := activeSessions(ctx, client)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(sessions))
	for i, session := range sessions {
		ids[i] = session.SessionId
	}
	return ids, nil
}
