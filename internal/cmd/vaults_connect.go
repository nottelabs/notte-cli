package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/aac"
	"github.com/nottelabs/notte-cli/internal/api"
	clierrors "github.com/nottelabs/notte-cli/internal/errors"
)

// aacTokenEnv holds the token of a listener the user runs themselves (aac reads the same variable).
const aacTokenEnv = "AAC_TOKEN"

// providerBitwarden is the only password manager a session can connect to.
const providerBitwarden = "bitwarden"

// listenerIdentityTimeout bounds the wait for a starting `aac listen` to create its identity.
const listenerIdentityTimeout = 15 * time.Second

// listenerStopTimeout bounds the wait for `aac listen` to quit after an interrupt. Swapped out in tests.
var listenerStopTimeout = 5 * time.Second

var (
	vaultConnectProvider    string
	vaultConnectToken       string
	vaultConnectAACProvider string
)

// findAAC and notifyStop are swapped out in tests.
var (
	findAAC    = aac.Find
	notifyStop = func() (<-chan os.Signal, func()) {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		return signals, func() { signal.Stop(signals) }
	}
)

var vaultsConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect your Bitwarden vault to a session",
	Long: `Connect your Bitwarden Password Manager to a running cloud session, through
Agent Access (aac, https://github.com/bitwarden/agent-access).

The session's fills of login placeholders then ask your vault for the page's
login first (then the session's Notte vault, if it has one). You approve each
request in aac, and the login goes from your vault to the session's browser.
Nothing is imported into a Notte vault.

This command runs "aac listen --reusable-psk" in this terminal, connects it to the
session and keeps it running: unlock your vault with /unlock and approve the
requests there. Quit aac (Esc or Ctrl+C) to disconnect the session.

To run the listener yourself instead (in another terminal or on another
machine), start "aac listen --reusable-psk" and pass its token in ` + aacTokenEnv + `
(or --token): the command then connects the session and returns. Disconnect it
with "notte vaults disconnect". Prefer ` + aacTokenEnv + `: a --token value is in argv,
where other processes on this machine can read it while the command runs.

Requires aac ` + aac.Version + ` and the Bitwarden CLI (bw), logged in.`,
	Example: `  notte vaults connect --provider bitwarden --session-id <session-id>
  AAC_TOKEN=<token> notte vaults connect --provider bitwarden
  notte page fill 'input[name=password]' --vault-field password`,
	Args: cobra.NoArgs,
	RunE: runVaultsConnect,
}

var vaultsDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect the password manager from a session",
	Long:  `Disconnect the password manager connected with "notte vaults connect": the session's fills no longer ask it. Values it already typed stay masked in the session.`,
	Args:  cobra.NoArgs,
	RunE:  runVaultsDisconnect,
}

func init() {
	vaultsCmd.AddCommand(vaultsConnectCmd)
	vaultsCmd.AddCommand(vaultsDisconnectCmd)

	vaultsConnectCmd.Flags().StringVar(&vaultConnectProvider, "provider", providerBitwarden, "Password manager to connect (bitwarden)")
	vaultsConnectCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	vaultsConnectCmd.Flags().StringVar(&vaultConnectToken, "token", "", "Token of an \"aac listen --reusable-psk\" you run yourself (prefer "+aacTokenEnv+": flag values are visible in ps)")
	// aac's own provider for the listener: "example" serves aac's demo logins, for trying the flow without a vault.
	vaultsConnectCmd.Flags().StringVar(&vaultConnectAACProvider, "aac-provider", providerBitwarden, "aac credential provider the listener reads")
	_ = vaultsConnectCmd.Flags().MarkHidden("aac-provider")

	vaultsDisconnectCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
}

func runVaultsConnect(cmd *cobra.Command, args []string) error {
	if vaultConnectProvider != providerBitwarden {
		return fmt.Errorf("--provider must be %s", providerBitwarden)
	}
	if err := RequireSessionID(); err != nil {
		return err
	}
	token := vaultConnectToken
	if !cmd.Flags().Changed("token") {
		token = os.Getenv(aacTokenEnv)
	}
	client, err := GetClient()
	if err != nil {
		return err
	}

	if token != "" {
		if !aac.ValidToken(token) {
			return fmt.Errorf("the token must be the one \"aac listen --reusable-psk\" prints (<64 hex>_<64 hex>): a pairing code works only once")
		}
		if err := connectVault(cmd.Context(), client, sessionID, token); err != nil {
			return err
		}
		return PrintResult(fmt.Sprintf(`Bitwarden is connected to session %s.
Keep "aac listen" running and approve each login request there.
Run "notte vaults disconnect --session-id %s" to disconnect it.`, sessionID, sessionID), map[string]any{
			"session_id": sessionID,
			"provider":   providerBitwarden,
			"connected":  true,
		})
	}

	binary, err := findAAC()
	if err != nil {
		return err
	}
	psk, err := aac.ReusablePSK()
	if err != nil {
		return err
	}
	// Printed before the listener takes over the terminal.
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), `Connecting Bitwarden to session %s through aac.
In aac: unlock your vault with /unlock, then approve each login request.
Quit aac (Esc or Ctrl+C) to disconnect the session.
`, sessionID)

	stops, stopNotify := notifyStop()
	defer stopNotify()
	// Whether the session may hold the token: a connect whose answer was lost or
	// cut short may have been applied, so only an answer refusing it rules that out.
	mayBeConnected := false
	err = startListener(binary, vaultConnectAACProvider, stops, func(ctx context.Context) error {
		fingerprint, err := aac.WaitForFingerprint(ctx, binary, 200*time.Millisecond)
		if err != nil {
			return err
		}
		err = connectVault(ctx, client, sessionID, aac.Token(psk, fingerprint))
		mayBeConnected = err == nil || !refusedByAPI(err)
		return err
	})
	if !mayBeConnected {
		return err
	}
	// The listener quit: later fills must not wait for approvals nobody can give.
	disconnectErr := disconnectVault(cmd.Context(), client, sessionID)
	if disconnectErr != nil {
		disconnectErr = fmt.Errorf("the session may still be connected to the stopped listener: %w; run \"notte vaults disconnect --session-id %s\"", disconnectErr, sessionID)
	}
	if err != nil || disconnectErr != nil {
		return errors.Join(err, disconnectErr)
	}
	return PrintResult(fmt.Sprintf("aac quit: Bitwarden is disconnected from session %s.", sessionID), map[string]any{
		"session_id": sessionID,
		"provider":   providerBitwarden,
		"connected":  false,
	})
}

// startListener runs `aac listen --reusable-psk` in this terminal. Once it runs,
// connect gets the listener's identity and connects the session; then it waits
// for the user to quit aac. When connect fails the listener is stopped.
//
// A signal on stops (Ctrl+C outside aac's screen, or `kill` of this process
// only) is forwarded to aac, which is killed if it has not quit within
// listenerStopTimeout. A quit the user or such a signal asked for is not an
// error; any other non-zero exit of aac is.
func startListener(binary, provider string, stops <-chan os.Signal, connect func(context.Context) error) error {
	listener := exec.Command(binary, "listen", "--reusable-psk", "--provider", provider)
	listener.Stdin, listener.Stdout, listener.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := listener.Start(); err != nil {
		return fmt.Errorf("could not start aac listen: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- listener.Wait() }()

	ctx, cancel := context.WithTimeout(context.Background(), listenerIdentityTimeout)
	defer cancel()
	connected := make(chan error, 1)
	go func() { connected <- connect(ctx) }()

	stopping := false
	var connectErr error
	var deadline <-chan time.Time
	stop := func(sig os.Signal) {
		stopping = true
		_ = listener.Process.Signal(sig)
		if deadline == nil {
			deadline = time.After(listenerStopTimeout)
		}
	}
	for {
		select {
		case err := <-connected:
			connected = nil
			if err != nil {
				// Interrupted rather than killed, so aac restores the terminal; killed after the deadline.
				connectErr = err
				stop(os.Interrupt)
			}
		case sig := <-stops:
			stop(sig)
		case <-deadline:
			_ = listener.Process.Kill()
		case err := <-exited:
			if connectErr != nil {
				return connectErr
			}
			err = listenerExit(err, stopping)
			if connected != nil {
				cancel()
				<-connected
				if err == nil {
					return errors.New("aac listen quit before the session was connected")
				}
				return fmt.Errorf("aac listen quit before the session was connected: %w", err)
			}
			return err
		}
	}
}

// listenerExit is the error of aac's exit: none when it quit normally (Esc,
// Ctrl+C or /exit in its screen) or after a stop this command sent it.
func listenerExit(err error, stopping bool) error {
	var exit *exec.ExitError
	if err == nil || (stopping && errors.As(err, &exit)) {
		return nil
	}
	return fmt.Errorf("aac listen failed: %w", err)
}

// refusedByAPI reports whether err is the API's answer to a request, which it did not apply.
func refusedByAPI(err error) bool {
	var apiErr *clierrors.APIError
	var authErr *clierrors.AuthError
	var validationErr *clierrors.ValidationError
	var rateLimitErr *clierrors.RateLimitError
	return errors.As(err, &apiErr) || errors.As(err, &authErr) || errors.As(err, &validationErr) || errors.As(err, &rateLimitErr)
}

func connectVault(ctx context.Context, client *api.NotteClient, sessionID, token string) error {
	ctx, cancel := GetContextWithTimeout(ctx)
	defer cancel()
	_, resp, body, err := client.ConnectVault(ctx, sessionID, providerBitwarden, token)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	return HandleAPIResponse(resp, body)
}

func disconnectVault(ctx context.Context, client *api.NotteClient, sessionID string) error {
	ctx, cancel := GetContextWithTimeout(ctx)
	defer cancel()
	_, resp, body, err := client.DisconnectVault(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	return HandleAPIResponse(resp, body)
}

func runVaultsDisconnect(cmd *cobra.Command, args []string) error {
	if err := RequireSessionID(); err != nil {
		return err
	}
	client, err := GetClient()
	if err != nil {
		return err
	}
	if err := disconnectVault(cmd.Context(), client, sessionID); err != nil {
		return err
	}
	return PrintResult(fmt.Sprintf("Disconnected the password manager from session %s.", sessionID), map[string]any{
		"session_id": sessionID,
		"connected":  false,
	})
}
