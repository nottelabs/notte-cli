package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/auth"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Login, logout, and check authentication status.",
}

const loginLong = `Log in to Notte from any machine.

The CLI prints a link and a code. Open the link in a browser on any device,
check that the code matches, choose a workspace and approve. The CLI then
receives a new API key named after this machine ("CLI · <host> · <date>"),
which you can revoke from the console's API keys page or with notte logout.

Nothing listens on this machine, so this also works over SSH, in containers
and in sandboxes.

To store a key you already have instead, pipe it in:
  echo "$NOTTE_API_KEY" | notte login --with-token

The API key is stored in your system credential store: the macOS Keychain,
the Windows Credential Manager, or a Secret Service, KWallet or pass on Linux.
Without one (headless hosts, containers), it goes to an encrypted file in the
config directory whose password is fixed, so it is protected no better than a
file only your user can read.`

var (
	loginWithToken bool
	loginNoBrowser bool
)

// stdinReader is replaced in tests.
var stdinReader io.Reader = os.Stdin

// newDeviceFlow is replaced in tests to point at a fake API.
var newDeviceFlow = func(apiURL string) *auth.DeviceFlow {
	return auth.NewDeviceFlow(apiURL, Version)
}

func newLoginCommand(use string) *cobra.Command {
	c := &cobra.Command{
		Use:   use,
		Short: "Log in to Notte (opens a link you approve in the console)",
		Long:  loginLong,
		Args:  cobra.NoArgs,
		RunE:  runAuthLogin,
	}
	c.Flags().BoolVar(&loginWithToken, "with-token", false, "Read an existing API key from standard input instead of logging in through the console")
	c.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "Print the link without trying to open a browser")
	return c
}

func newLogoutCommand(use string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: "Log out: revoke this machine's key and remove it",
		Long: `Remove the stored API key from this machine.

A key created by notte login is also revoked, so it stops working everywhere.
A key you stored with --with-token is only removed from this machine.`,
		Args: cobra.NoArgs,
		RunE: runAuthLogout,
	}
}

func newStatusCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE:  runAuthStatus,
	}
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(newLoginCommand("login"))
	authCmd.AddCommand(newLogoutCommand("logout"))
	authCmd.AddCommand(newStatusCommand("status", "Show current authentication status"))

	// The same commands at the top level, as other CLIs name them.
	rootCmd.AddCommand(newLoginCommand("login"))
	rootCmd.AddCommand(newLogoutCommand("logout"))
	rootCmd.AddCommand(newStatusCommand("whoami", "Show which API key and environment this CLI uses"))
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	// The key is checked against this API and then saved for it, so it would
	// be sent in clear now and on every later command.
	apiURL := auth.GetCurrentAPIURL()
	if !auth.IsSecureAPIURL(apiURL) {
		return fmt.Errorf("refusing to log in to %s: %w", apiURL, auth.ErrInsecureAPIURL)
	}

	if loginWithToken {
		return loginWithStdinToken(cmd.Context())
	}

	ctx := cmd.Context()
	flow := newDeviceFlow(apiURL)
	code, err := flow.RequestCode(ctx, deviceClientInfo())
	if err != nil {
		return fmt.Errorf("could not start the login: %w", err)
	}

	link := auth.VerificationURL(code.VerificationURIComplete)
	PrintInfo(fmt.Sprintf("\n  Visit %s\n  and check that it shows the code %s\n", link, code.UserCode))
	if !loginNoBrowser {
		_ = openBrowser(link) // Over SSH or in a container there is no browser to open; the link is printed.
	}
	PrintInfo("Waiting for approval in the console...")

	token, err := flow.Poll(ctx, code)
	if err != nil {
		return err
	}

	// The key was just created for this API, but check it works where it is
	// about to be saved, as for a pasted key.
	if err := auth.ValidateAPIKey(ctx, token.AccessToken); err != nil {
		return fmt.Errorf("the new API key could not be used: %w", err)
	}

	previous, _ := auth.GetKeyringAPIKey()
	if err := auth.SetKeyringAPIKey(token.AccessToken); err != nil {
		return fmt.Errorf("failed to save the API key: %w", err)
	}
	// Logging in again replaces this machine's key: revoke the old one if a
	// CLI login created it, so keys do not pile up on the API keys page.
	if previous != "" && previous != token.AccessToken {
		revokeQuietly(ctx, flow, previous)
	}

	workspace := token.WorkspaceName
	if workspace == "" {
		workspace = "personal workspace"
	}
	who := token.Email
	if who == "" {
		who = "your account"
	}
	return PrintResult(
		fmt.Sprintf("Logged in as %s (%s). API key %q stored.", who, workspace, token.KeyName),
		map[string]any{
			"authenticated": true,
			"source":        "keychain",
			"email":         token.Email,
			"workspace":     token.WorkspaceName,
			"key_name":      token.KeyName,
		},
	)
}

func deviceClientInfo() auth.DeviceClientInfo {
	host, _ := os.Hostname()
	return auth.DeviceClientInfo{
		Hostname: host,
		OS:       fmt.Sprintf("%s (%s)", runtime.GOOS, runtime.GOARCH),
		Version:  Version,
	}
}

func loginWithStdinToken(ctx context.Context) error {
	line, err := bufio.NewReader(stdinReader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to read the API key from standard input: %w", err)
	}
	key := strings.TrimSpace(line)
	if key == "" {
		return errors.New("no API key on standard input; pipe one in, for example: echo \"$NOTTE_API_KEY\" | notte login --with-token")
	}
	if err := auth.ValidateAPIKey(ctx, key); err != nil {
		return fmt.Errorf("API key not accepted: %w", err)
	}
	if err := auth.SetKeyringAPIKey(key); err != nil {
		return fmt.Errorf("failed to save the API key: %w", err)
	}
	return PrintResult("API key stored successfully.", map[string]any{
		"authenticated": true,
		"source":        "keychain",
	})
}

// revokeQuietly revokes a key a CLI login created, reporting failures without
// failing the command: the local key is what the user asked to change.
func revokeQuietly(ctx context.Context, flow *auth.DeviceFlow, key string) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	revoked, err := flow.Revoke(ctx, key)
	if err != nil {
		PrintInfo(fmt.Sprintf("Could not revoke the previous key (%v). You can revoke it from the console's API keys page.", err))
		return false
	}
	return revoked
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	key, _ := auth.GetKeyringAPIKey()
	revoked := false
	if key != "" {
		if apiURL := auth.GetCurrentAPIURL(); auth.IsSecureAPIURL(apiURL) {
			revoked = revokeQuietly(cmd.Context(), newDeviceFlow(apiURL), key)
		}
	}

	if err := auth.DeleteKeyringAPIKey(); err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	message := "API key removed."
	if revoked {
		message = "API key revoked and removed."
	}
	return PrintResult(message, map[string]any{
		"authenticated": false,
		"revoked":       revoked,
	})
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	key, source, err := auth.GetAPIKey("")
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	// Mask key for display (handle short keys safely)
	var masked string
	if len(key) < 12 {
		masked = "****"
	} else {
		masked = key[:8] + "..." + key[len(key)-4:]
	}

	envLabel := auth.ResolveEnvLabel(auth.GetCurrentAPIURL())

	formatter := GetFormatter()
	data := map[string]any{
		"Authenticated": "yes",
		"Source":        string(source),
		"API Key":       masked,
		"Environment":   envLabel,
	}

	return formatter.Print(data)
}
