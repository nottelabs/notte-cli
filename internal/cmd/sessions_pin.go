package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/output"
)

var sessionsPinCmd = &cobra.Command{
	Use:   "pin [session-id]",
	Short: "Pin a session as the one commands use by default",
	Long: `Pin an active session as the current one: the session commands use when
--session-id is omitted, marked "*" in "notte sessions list".

Pass a session ID, or enough of its start to be unique. Without one, pick
from a numbered list of active sessions.`,
	Example: `  notte sessions pin 3e1103a1
  notte browser pin`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSessionsPin,
}

func init() {
	sessionsCmd.AddCommand(sessionsPinCmd)
}

func runSessionsPin(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	onlyActive, pageSize := true, 100
	resp, err := client.Client().ListSessionsWithResponse(ctx, &api.ListSessionsParams{OnlyActive: &onlyActive, PageSize: &pageSize})
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}
	var sessions []api.SessionResponse
	if resp.JSON200 != nil {
		sessions = resp.JSON200.Items
	}

	var chosen *api.SessionResponse
	switch {
	case len(args) == 1:
		chosen, err = matchSession(sessions, args[0])
	case len(sessions) == 0:
		err = fmt.Errorf("no active sessions to pin. Start one with: notte sessions start")
	case len(sessions) == 1:
		chosen = &sessions[0]
	case canUseArrowMenu():
		chosen, err = pickSessionWithArrows(sessions, GetCurrentSessionID(), time.Now())
	default:
		chosen, err = pickSession(os.Stdin, os.Stderr, sessions, GetCurrentSessionID(), time.Now())
	}
	if err != nil {
		return err
	}

	// Forget what was stored for the previous session before remembering
	// this one, so its viewer link or expiry never carries over.
	_ = clearCurrentViewerURL()
	_ = clearCurrentSessionExpiry()
	rememberCurrentSession(chosen)

	if env := os.Getenv(config.EnvSessionID); env != "" && env != chosen.SessionId {
		PrintInfo(fmt.Sprintf("Note: %s=%s is set and takes precedence over the pinned session.", config.EnvSessionID, env))
	}
	return printDone("Pinned session "+chosen.SessionId, map[string]any{
		"session_id": chosen.SessionId,
		"pinned":     true,
	}, "notte sessions status")
}

// matchSession finds the active session whose ID is, or starts with, query.
func matchSession(sessions []api.SessionResponse, query string) (*api.SessionResponse, error) {
	query = strings.TrimSpace(query)
	var matches []*api.SessionResponse
	for i := range sessions {
		if sessions[i].SessionId == query {
			return &sessions[i], nil
		}
		if strings.HasPrefix(sessions[i].SessionId, query) {
			matches = append(matches, &sessions[i])
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("no active session matches %q. List them with: notte sessions list", query)
	}
	ids := make([]string, len(matches))
	for i, m := range matches {
		ids[i] = m.SessionId
	}
	return nil, fmt.Errorf("%q matches %d sessions (%s); type more of the ID", query, len(matches), strings.Join(ids, ", "))
}

// pickSession shows a numbered list of sessions on out and reads a choice
// from in. Plain numbered input works in every terminal and when piped.
func pickSession(in io.Reader, out io.Writer, sessions []api.SessionResponse, current string, now time.Time) (*api.SessionResponse, error) {
	formatter, _ := GetFormatter().(*output.TextFormatter)
	text := &output.TextFormatter{Writer: out, NoColor: formatter == nil || formatter.NoColor}

	rows := make([][]string, 0, len(sessions))
	for i, session := range sessions {
		pinned := ""
		if session.SessionId == current {
			pinned = "pinned"
		}
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			session.SessionId,
			relativeTime(session.CreatedAt.Time, now),
			text.Faint(pinned),
		})
	}
	if err := text.Table([]string{"#", "BROWSER SESSION", "STARTED", ""}, rows); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprintf(out, "Pin which session? [1-%d]: ", len(sessions)); err != nil {
			return nil, err
		}
		line, err := reader.ReadString('\n')
		answer := strings.TrimSpace(line)
		if n, convErr := strconv.Atoi(answer); convErr == nil && n >= 1 && n <= len(sessions) {
			return &sessions[n-1], nil
		}
		if answer != "" {
			if session, matchErr := matchSession(sessions, answer); matchErr == nil {
				return session, nil
			}
		}
		if err != nil {
			// No interactive input: say how to pin without the menu.
			return nil, fmt.Errorf("no session picked. Pass one: notte sessions pin <session-id>")
		}
		if _, err := fmt.Fprintf(out, "Type a number from 1 to %d.\n", len(sessions)); err != nil {
			return nil, err
		}
	}
}

// canUseArrowMenu reports whether the menu can take single key presses: both
// input and the screen are a terminal. Otherwise the numbered menu is used.
// NOTTE_FORCE_INTERACTIVE=1 is for terminals that relay each key press over a
// pipe instead of a TTY,.
func canUseArrowMenu() bool {
	if IsJSONOutput() {
		return false
	}
	return forcedInteractive() || (term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())))
}

func forcedInteractive() bool { return os.Getenv("NOTTE_FORCE_INTERACTIVE") == "1" }

// errPinCancelled is returned when the menu is left without a choice.
var errPinCancelled = fmt.Errorf("no session pinned")

// pickSessionWithArrows lets the user move through the sessions with the
// arrow keys (or j/k) and pin one with enter. Esc, q or Ctrl+C cancel; a
// digit jumps to that row. The menu is drawn on stderr and erased after.
func pickSessionWithArrows(sessions []api.SessionResponse, current string, now time.Time) (*api.SessionResponse, error) {
	// A TTY must be switched to raw mode to get key presses one at a time; a
	// forced-interactive pipe already delivers them that way.
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		state, err := term.MakeRaw(fd)
		if err != nil {
			return pickSession(os.Stdin, os.Stderr, sessions, current, now)
		}
		defer func() { _ = term.Restore(fd, state) }()
	} else if !forcedInteractive() {
		return pickSession(os.Stdin, os.Stderr, sessions, current, now)
	}

	cursor := 0
	for i, session := range sessions {
		if session.SessionId == current {
			cursor = i
		}
	}
	formatter, _ := GetFormatter().(*output.TextFormatter)
	noColor := formatter == nil || formatter.NoColor

	out := os.Stderr
	_, _ = io.WriteString(out, "\x1b[?25l") // hide the cursor while the menu is up
	defer func() { _, _ = io.WriteString(out, "\x1b[?25h") }()
	lines := 0
	draw := func() {
		if lines > 0 {
			_, _ = fmt.Fprintf(out, "\x1b[%dA", lines) // back to the menu's first line
		}
		menu := renderPinMenu(sessions, cursor, current, now, noColor)
		_, _ = io.WriteString(out, menu)
		lines = strings.Count(menu, "\n")
	}
	erase := func() {
		_, _ = fmt.Fprintf(out, "\x1b[%dA\x1b[J", lines)
	}

	draw()
	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			erase()
			return nil, errPinCancelled
		}
		next, done, cancel := pinMenuKey(cursor, len(sessions), buf[:n])
		switch {
		case cancel:
			erase()
			return nil, errPinCancelled
		case done:
			erase()
			return &sessions[next], nil
		}
		cursor = next
		draw()
	}
}

// pinMenuKey applies one key press to the menu.
func pinMenuKey(cursor, count int, key []byte) (next int, done, cancel bool) {
	switch k := string(key); {
	case k == "\x1b[A" || k == "\x1bOA" || k == "k":
		return (cursor - 1 + count) % count, false, false
	case k == "\x1b[B" || k == "\x1bOB" || k == "j":
		return (cursor + 1) % count, false, false
	case k == "\r" || k == "\n":
		return cursor, true, false
	case k == "\x1b" || k == "q" || k == "\x03" || k == "\x04":
		return cursor, false, true
	case len(k) == 1 && k[0] >= '1' && k[0] <= '9' && int(k[0]-'0') <= count:
		return int(k[0] - '1'), false, false
	}
	return cursor, false, false
}

// renderPinMenu draws the menu. In raw mode a line feed does not return the
// carriage, so every line ends with "\r\n".
func renderPinMenu(sessions []api.SessionResponse, cursor int, current string, now time.Time, noColor bool) string {
	var b strings.Builder
	text := &output.TextFormatter{Writer: &b, NoColor: noColor}
	b.WriteString("\x1b[2K" + text.Faint("Pin a session  ↑/↓ move · enter pin · esc cancel") + "\r\n")
	for i, session := range sessions {
		line := fmt.Sprintf("%s   %s", session.SessionId, relativeTime(session.CreatedAt.Time, now))
		if session.SessionId == current {
			line += "   " + text.Faint("pinned")
		}
		if i == cursor {
			b.WriteString("\x1b[2K" + text.Colorize("❯ "+line, termenv.ANSICyan) + "\r\n")
		} else {
			b.WriteString("\x1b[2K  " + line + "\r\n")
		}
	}
	return b.String()
}
