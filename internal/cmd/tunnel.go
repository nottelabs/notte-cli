package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/tunnel"
)

// tailnetSecretEnv lets `tunnel up` read the OAuth client secret without
// putting it on the command line.
const tailnetSecretEnv = "NOTTE_TAILNET_OAUTH_CLIENT_SECRET"

// tailscaleStatusTimeout bounds the optional status read in `tunnel list`.
const tailscaleStatusTimeout = 5 * time.Second

// findTailscale is swapped out in tests.
var findTailscale = tunnel.Find

// workspaceTailscaleConnected reports whether the Notte workspace has a
// Tailscale OAuth client connected in the console. Swapped out in tests.
var workspaceTailscaleConnected = checkWorkspaceTailscale

// consoleIntegrationsHint points to where a workspace connects Tailscale.
const consoleIntegrationsHint = "connect Tailscale in the Notte console (Settings > Integrations), or pass --oauth-client-id and --oauth-client-secret"

var (
	tunnelName              string
	tunnelOAuthClientID     string
	tunnelOAuthClientSecret string
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Route sessions through this machine's internet connection",
	Long: `Route session traffic through this machine's internet connection using
Tailscale. "notte tunnel up" offers this machine as a Tailscale exit node and
saves it as a named tunnel; "notte sessions start --tunnel <name>" then sends
the session's public traffic out through it.

Requires Tailscale running and signed in on this machine, and a Tailscale OAuth
client (auth_keys write scope, tagged tag:notte only). Connect the client once
in the Notte console (Settings > Integrations), or pass it to "tunnel up" to
keep it on this machine. Your tailnet policy must let tag:notte reach
autogroup:internet, and this machine must be approved as an exit node.`,
}

var tunnelUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Offer this machine as an exit node and save it as a tunnel",
	Long: `Offer this machine as a Tailscale exit node and save it as a named tunnel.

Without credentials, the tunnel uses the Tailscale OAuth client connected to
your Notte workspace in the console (Settings > Integrations), and nothing
secret is stored on this machine.

To keep the client on this machine instead, pass --oauth-client-id and
--oauth-client-secret (or ` + tailnetSecretEnv + `). The secret is stored in
the keyring, never in the config file. Re-running "up" for a saved tunnel
reuses its stored credentials.`,
	Example: `  notte tunnel up --name home-mac
  notte tunnel up --name home-mac --oauth-client-id <id> --oauth-client-secret <secret>
  notte sessions start --tunnel home-mac`,
	RunE: runTunnelUp,
}

var tunnelDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop offering this machine as an exit node",
	Long:  `Stop offering this machine as a Tailscale exit node. Saved tunnels are kept, so "notte tunnel up" can bring it back without credentials.`,
	RunE:  runTunnelDown,
}

var tunnelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved tunnels",
	RunE:  runTunnelList,
}

var tunnelRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a saved tunnel and its stored credentials",
	Args:  cobra.ExactArgs(1),
	RunE:  runTunnelRemove,
}

func init() {
	rootCmd.AddCommand(tunnelCmd)
	tunnelCmd.AddCommand(tunnelUpCmd, tunnelDownCmd, tunnelListCmd, tunnelRemoveCmd)

	tunnelUpCmd.Flags().StringVar(&tunnelName, "name", "", "Tunnel name (default: this machine's MagicDNS host name)")
	tunnelUpCmd.Flags().StringVar(&tunnelOAuthClientID, "oauth-client-id", "", "Tailscale OAuth client ID (default: the workspace's console connection)")
	tunnelUpCmd.Flags().StringVar(&tunnelOAuthClientSecret, "oauth-client-secret", "", "Tailscale OAuth client secret (or set "+tailnetSecretEnv+")")
}

func runTunnelUp(cmd *cobra.Command, args []string) error {
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	ts, err := findTailscale()
	if err != nil {
		return err
	}
	st, err := ts.Status(ctx)
	if err != nil {
		return err
	}

	name := tunnelName
	if name == "" {
		name = strings.ToLower(strings.SplitN(st.MagicDNSName(), ".", 2)[0])
	}
	if err := tunnel.ValidateName(name); err != nil {
		return err
	}

	clientID, secret, err := tunnelCredentials(ctx, cmd, name)
	if err != nil {
		return err
	}

	if err := ts.SetAdvertiseExitNode(ctx, true); err != nil {
		return err
	}
	// Approval is decided by the tailnet, so read it back rather than assume.
	if st, err = ts.Status(ctx); err != nil {
		return err
	}

	t := tunnel.Tunnel{Name: name, ExitNode: st.MagicDNSName(), Tailnet: st.Tailnet(), OAuthClientID: clientID}
	if err := tunnel.Save(t, secret); err != nil {
		return err
	}

	data := map[string]any{
		"name":        t.Name,
		"exit_node":   t.ExitNode,
		"tailnet":     t.Tailnet,
		"approved":    st.Self.ExitNodeOption,
		"credentials": tunnelCredentialSource(t),
	}
	credentials := "OAuth credentials stored on this machine"
	if t.OAuthClientID == "" {
		credentials = "the workspace's Tailscale connection"
	}
	if !st.Self.ExitNodeOption {
		return PrintResult(fmt.Sprintf(`Saved tunnel %q (%s, using %s), but this machine is not approved as an exit node yet.
Approve it at https://login.tailscale.com/admin/machines (Edit route settings > Use as exit node),
or add it to autoApprovers.exitNode in your tailnet policy. The policy must also let tag:notte
reach autogroup:internet. Then run "notte tunnel up --name %s" again.`, t.Name, t.ExitNode, credentials, t.Name), data)
	}
	return PrintResult(fmt.Sprintf(`Tunnel %q is up: %s is offered as an exit node, using %s.
Start a session through it with "notte sessions start --tunnel %s".
Keep this machine awake and online while sessions use it.`, t.Name, t.ExitNode, credentials, t.Name), data)
}

// tunnelCredentials resolves the OAuth client for a tunnel: flags and the
// environment first, then what was saved for the same name, then the
// workspace's console connection. An empty client ID means the workspace
// connection; an empty secret means "keep the stored one".
func tunnelCredentials(ctx context.Context, cmd *cobra.Command, name string) (clientID, secret string, err error) {
	clientID = tunnelOAuthClientID
	secret = tunnelOAuthClientSecret
	if !cmd.Flags().Changed("oauth-client-secret") {
		secret = os.Getenv(tailnetSecretEnv)
	}

	saved, err := tunnel.Get(name)
	if err != nil && !errors.Is(err, tunnel.ErrNotFound) {
		return "", "", err
	}
	if clientID == "" && saved != nil {
		clientID = saved.OAuthClientID
	}
	if clientID == "" && secret == "" {
		connected, err := workspaceTailscaleConnected(ctx)
		if err != nil {
			return "", "", fmt.Errorf("could not check the workspace's Tailscale connection: %w; %s", err, consoleIntegrationsHint)
		}
		if !connected {
			return "", "", fmt.Errorf("this workspace has no Tailscale connection: %s", consoleIntegrationsHint)
		}
		return "", "", nil
	}
	if clientID == "" {
		return "", "", fmt.Errorf("--oauth-client-id is required with --oauth-client-secret to save tunnel %q", name)
	}
	if secret != "" {
		return clientID, secret, nil
	}
	if saved == nil || saved.OAuthClientID != clientID {
		return "", "", fmt.Errorf("--oauth-client-secret (or %s) is required to save tunnel %q", tailnetSecretEnv, name)
	}
	if _, err := tunnel.Secret(name); err != nil {
		return "", "", err
	}
	return clientID, "", nil
}

// checkWorkspaceTailscale asks the API whether the workspace has a Tailscale
// OAuth client connected.
func checkWorkspaceTailscale(ctx context.Context) (bool, error) {
	client, err := GetClient()
	if err != nil {
		return false, err
	}
	namespace := api.Tailscale
	resp, err := client.Client().ListSecretsWithResponse(ctx, &api.ListSecretsParams{Namespace: &namespace})
	if err != nil {
		return false, fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return false, err
	}
	return resp.JSON200 != nil && len(resp.JSON200.Items) > 0, nil
}

// tunnelCredentialSource says where a tunnel's OAuth client comes from.
func tunnelCredentialSource(t tunnel.Tunnel) string {
	if t.OAuthClientID == "" {
		return "workspace"
	}
	return "local"
}

func runTunnelDown(cmd *cobra.Command, args []string) error {
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	ts, err := findTailscale()
	if err != nil {
		return err
	}
	st, err := ts.Status(ctx)
	if err != nil {
		return err
	}
	if err := ts.SetAdvertiseExitNode(ctx, false); err != nil {
		return err
	}
	return PrintResult(fmt.Sprintf("%s is no longer offered as an exit node. Sessions started with its tunnel will fail until you run \"notte tunnel up\" again.", st.MagicDNSName()), map[string]any{
		"exit_node":   st.MagicDNSName(),
		"advertising": false,
	})
}

func runTunnelList(cmd *cobra.Command, args []string) error {
	tunnels, err := tunnel.List()
	if err != nil {
		return err
	}
	if handled, err := PrintListOrEmpty(tunnels, "No saved tunnels. Create one with \"notte tunnel up\"."); handled || err != nil {
		return err
	}

	// Approval can only be read for this machine, and only if Tailscale is up.
	var self *tunnel.Status
	if ts, err := findTailscale(); err == nil {
		parent := cmd.Context()
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, tailscaleStatusTimeout)
		self, _ = ts.Status(ctx)
		cancel()
	}

	rows := make([]tunnelRow, 0, len(tunnels))
	for _, t := range tunnels {
		row := tunnelRow{Name: t.Name, ExitNode: t.ExitNode, Tailnet: t.Tailnet, Credentials: tunnelCredentialSource(t), Approved: "-"}
		if self != nil && self.MagicDNSName() == t.ExitNode {
			row.Approved = fmt.Sprintf("%t", self.Self.ExitNodeOption)
		}
		rows = append(rows, row)
	}
	return GetFormatter().Print(rows)
}

// tunnelRow is one line of `tunnel list`. Approved is "-" for tunnels on other
// machines, whose approval can only be read from that machine.
type tunnelRow struct {
	Name        string `json:"name"`
	ExitNode    string `json:"exit_node"`
	Tailnet     string `json:"tailnet"`
	Credentials string `json:"credentials"`
	Approved    string `json:"approved"`
}

func runTunnelRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := tunnel.Remove(name); err != nil {
		return err
	}
	return PrintResult(fmt.Sprintf("Removed tunnel %q and its stored credentials. This does not change Tailscale; run \"notte tunnel down\" on that machine to stop offering it as an exit node.", name), map[string]any{
		"name":    name,
		"removed": true,
	})
}

// tunnelProxyItem expands a saved tunnel into a tailnet proxy that exits
// through the tunnel's machine. A tunnel on the workspace connection sends no
// credentials: the API reads them from the workspace.
func tunnelProxyItem(name string) (api.ApiSessionStartRequest_Proxies_0_Item, error) {
	var item api.ApiSessionStartRequest_Proxies_0_Item
	t, err := tunnel.Get(name)
	if err != nil {
		return item, err
	}
	proxy := api.TailnetProxy{ExitNode: &t.ExitNode}
	if t.OAuthClientID != "" {
		secret, err := tunnel.Secret(name)
		if err != nil {
			return item, err
		}
		proxy.OauthClientId = &t.OAuthClientID
		proxy.OauthClientSecret = &secret
	}
	if err := item.FromTailnetProxy(proxy); err != nil {
		return item, fmt.Errorf("failed to create tunnel proxy: %w", err)
	}
	return item, nil
}
