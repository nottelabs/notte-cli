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
)

// aacTokenEnv holds the token of a listener the user runs themselves (aac reads the same variable).
const aacTokenEnv = "AAC_TOKEN"

// providerBitwarden is the only password manager Notte connects to.
const providerBitwarden = "bitwarden"

// listenerIdentityTimeout bounds the wait for a starting `aac listen` to create its identity.
const listenerIdentityTimeout = 15 * time.Second

// listenerStopTimeout bounds the wait for `aac listen` to quit after an interrupt. Swapped out in tests.
var listenerStopTimeout = 5 * time.Second

var (
	vaultConnectProvider    string
	vaultConnectToken       string
	vaultConnectAACProvider string
	vaultDisconnectProvider string
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
	Short: "Connect your Bitwarden vault to Notte (experimental)",
	Long: `Connect your Bitwarden Password Manager to your Notte account, through
Agent Access (aac, https://github.com/bitwarden/agent-access). Experimental:
Agent Access is in alpha.

Sessions you start with "notte sessions start --aac-vault" then fill login
placeholders from your vault first (then from the session's Notte vault, if it
has one). aac asks you to approve each request; "[a]" approves that site for 10
minutes. The login goes from your vault to the session's browser and is masked
there. Notte never stores a password.

This command runs "aac listen --reusable-psk" in this terminal and connects it:
unlock your vault with /unlock and approve requests there. You connect once.
Keep "aac listen --reusable-psk" running whenever sessions fill logins; quitting
aac (Esc or Ctrl+C) keeps the connection. The first fill pairs Notte with your
listener; later fills reconnect without pairing.

To run the listener yourself (in another terminal or on another machine), start
"aac listen --reusable-psk" and pass its token in ` + aacTokenEnv + ` (or --token): the
command connects it and returns. Prefer ` + aacTokenEnv + `: a --token value is in argv,
where other processes on this machine can read it while the command runs.

"notte vaults disconnect --provider bitwarden" deletes the connection.

Requires aac ` + aac.Version + ` and the Bitwarden CLI (bw), logged in.`,
	Example: `  notte vaults connect --provider bitwarden
  AAC_TOKEN=<token> notte vaults connect --provider bitwarden
  notte sessions start --aac-vault
  notte page fill 'input[name=password]' --vault-field password`,
	Args: cobra.NoArgs,
	RunE: runVaultsConnect,
}

var vaultsDisconnectCmd = &cobra.Command{
	Use:     "disconnect",
	Short:   "Disconnect your Bitwarden vault from Notte",
	Long:    `Delete the connection "notte vaults connect" made: later fills of --aac-vault sessions no longer ask your vault. Values it already typed stay masked in their sessions.`,
	Example: `  notte vaults disconnect --provider bitwarden`,
	Args:    cobra.NoArgs,
	RunE:    runVaultsDisconnect,
}

func init() {
	vaultsCmd.AddCommand(vaultsConnectCmd)
	vaultsCmd.AddCommand(vaultsDisconnectCmd)

	vaultsConnectCmd.Flags().StringVar(&vaultConnectProvider, "provider", providerBitwarden, "Password manager to connect (bitwarden)")
	vaultsConnectCmd.Flags().StringVar(&vaultConnectToken, "token", "", "Token of an \"aac listen --reusable-psk\" you run yourself (prefer "+aacTokenEnv+": flag values are visible in ps)")
	// aac's own provider for the listener: "example" serves aac's demo logins, for trying the flow without a vault.
	vaultsConnectCmd.Flags().StringVar(&vaultConnectAACProvider, "aac-provider", providerBitwarden, "aac credential provider the listener reads")
	_ = vaultsConnectCmd.Flags().MarkHidden("aac-provider")

	vaultsDisconnectCmd.Flags().StringVar(&vaultDisconnectProvider, "provider", providerBitwarden, "Password manager to disconnect (bitwarden)")
}

func runVaultsConnect(cmd *cobra.Command, args []string) error {
	if vaultConnectProvider != providerBitwarden {
		return fmt.Errorf("--provider must be %s", providerBitwarden)
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
		if err := connectAac(cmd.Context(), client, token); err != nil {
			return err
		}
		return PrintResult(`Bitwarden is connected to Notte.
Start sessions with "notte sessions start --aac-vault", keep "aac listen --reusable-psk" running and approve login requests there.`, map[string]any{
			"provider":  providerBitwarden,
			"connected": true,
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
	_, _ = fmt.Fprint(cmd.ErrOrStderr(), `Connecting Bitwarden to Notte through aac.
In aac: unlock your vault with /unlock, then approve login requests from --aac-vault sessions.
Quitting aac keeps the connection: run "aac listen --reusable-psk" again before sessions fill logins.
`)

	stops, stopNotify := notifyStop()
	defer stopNotify()
	if err := startListener(binary, vaultConnectAACProvider, stops, func(ctx context.Context) error {
		fingerprint, err := aac.WaitForFingerprint(ctx, binary, 200*time.Millisecond)
		if err != nil {
			return err
		}
		return connectAac(ctx, client, aac.Token(psk, fingerprint))
	}); err != nil {
		return err
	}
	return PrintResult(`aac quit. Bitwarden stays connected to Notte: run "aac listen --reusable-psk" before --aac-vault sessions fill logins.
Run "notte vaults disconnect --provider bitwarden" to delete the connection.`, map[string]any{
		"provider":  providerBitwarden,
		"connected": true,
	})
}

// startListener runs `aac listen --reusable-psk` in this terminal. Once it runs,
// connect gets the listener's identity and connects it; then it waits for the
// user to quit aac. When connect fails the listener is stopped.
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
					return errors.New("aac listen quit before it was connected")
				}
				return fmt.Errorf("aac listen quit before it was connected: %w", err)
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

func connectAac(ctx context.Context, client *api.NotteClient, token string) error {
	ctx, cancel := GetContextWithTimeout(ctx)
	defer cancel()
	_, resp, body, err := client.ConnectAac(ctx, providerBitwarden, token)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	return HandleAPIResponse(resp, body)
}

func runVaultsDisconnect(cmd *cobra.Command, args []string) error {
	if vaultDisconnectProvider != providerBitwarden {
		return fmt.Errorf("--provider must be %s", providerBitwarden)
	}
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()
	_, resp, body, err := client.DisconnectAac(ctx)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp, body); err != nil {
		return err
	}
	return PrintResult("Bitwarden is disconnected from Notte.", map[string]any{
		"provider":  providerBitwarden,
		"connected": false,
	})
}
