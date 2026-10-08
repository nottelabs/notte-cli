package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/tunnel"
)

// ipLookupURL answers with the caller's public IPv4 address only, so the
// session and this machine are compared like for like.
const ipLookupURL = "https://api.ipify.org"

// checkSessionMaxMinutes bounds the check session in case it is never stopped.
// The idle timeout is set too: the API rejects an idle timeout above the
// maximum duration, and its default idle timeout may exceed this cap.
const (
	checkSessionMaxMinutes  = 5
	checkSessionIdleMinutes = 2
)

// stopTimeout bounds the cleanup call that closes the check session.
const stopTimeout = 30 * time.Second

var ipv4Pattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// lookupPublicIP returns this machine's public IPv4 address. Swapped out in tests.
var lookupPublicIP = fetchPublicIP

var tunnelCheckName string

var tunnelCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify a tunnel end to end with a short session",
	Long: `Verify that sessions leave through a saved tunnel.

The check starts a short session through the tunnel, opens an IP lookup inside
it, and compares the session's public IP with this machine's. The session is
billed like any other (usually well under a minute) and is closed when the
check ends.

If the Notte workspace has no Tailscale connection, connect one in the console
(Settings > Integrations) first.`,
	Example: `  notte tunnel check
  notte tunnel check --name home-mac`,
	RunE: runTunnelCheck,
}

func init() {
	tunnelCmd.AddCommand(tunnelCheckCmd)
	tunnelCheckCmd.Flags().StringVar(&tunnelCheckName, "name", "", "Tunnel to check (default: this machine's MagicDNS host name)")
}

func runTunnelCheck(cmd *cobra.Command, args []string) error {
	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}
	// Ctrl-C cancels the check instead of killing the process, so the deferred
	// stop in tunnelSessionIP still closes the billed session.
	parent, stopSignals := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// This machine's state is read when Tailscale is available. It names the
	// default tunnel and decides whether the session IP can be compared.
	var self *tunnel.Status
	if ts, err := findTailscale(); err == nil {
		ctx, cancel := context.WithTimeout(parent, tailscaleStatusTimeout)
		self, _ = ts.Status(ctx)
		cancel()
	}

	name := tunnelCheckName
	if name == "" {
		if self == nil {
			return fmt.Errorf("pass --name: Tailscale is not running on this machine, so there is no default tunnel")
		}
		name = strings.ToLower(strings.SplitN(self.MagicDNSName(), ".", 2)[0])
	}
	t, err := tunnel.Get(name)
	if err != nil {
		return err
	}

	if t.OAuthClientID == "" {
		ctx, cancel := GetContextWithTimeout(parent)
		connected, err := workspaceTailscaleConnected(ctx)
		cancel()
		if err != nil {
			return fmt.Errorf("could not check the workspace's Tailscale connection: %w", err)
		}
		if !connected {
			return fmt.Errorf("this workspace has no Tailscale connection: connect one at %s", integrationsURL())
		}
	}

	onThisMachine := self != nil && self.MagicDNSName() == t.ExitNode
	if onThisMachine && !self.Self.ExitNodeOption {
		return fmt.Errorf(`%s is not approved as an exit node: approve it at https://login.tailscale.com/admin/machines, or run "notte tunnel up --name %s" if it is no longer offered`, t.ExitNode, t.Name)
	}

	var machineIP string
	if onThisMachine {
		ctx, cancel := GetContextWithTimeout(parent)
		machineIP, err = lookupPublicIP(ctx)
		cancel()
		if err != nil {
			return fmt.Errorf("could not read this machine's public IP: %w", err)
		}
	}

	sessionIP, sessionID, err := tunnelSessionIP(parent, t.Name)
	if err != nil {
		return err
	}

	data := map[string]any{
		"name":       t.Name,
		"exit_node":  t.ExitNode,
		"session_id": sessionID,
		"session_ip": sessionIP,
	}
	if !onThisMachine {
		data["status"] = "unverified"
		return PrintResult(fmt.Sprintf(`Tunnel %q works: a session started through %s and left from %s.
Run the check on %s to compare that with the machine's own IP.`, t.Name, t.ExitNode, sessionIP, t.ExitNode), data)
	}
	data["machine_ip"] = machineIP
	if sessionIP != machineIP {
		return fmt.Errorf(`tunnel %q is not routing: the session left from %s, but this machine is at %s.
If this machine sends its own traffic through a VPN or another exit node, its IP differs from the tunnel's even when the tunnel works`, t.Name, sessionIP, machineIP)
	}
	data["status"] = "ok"
	return PrintResult(fmt.Sprintf(`Tunnel %q works: sessions leave from %s, this machine's public IP.`, t.Name, sessionIP), data)
}

// tunnelSessionIP starts a short session through the tunnel and returns the
// public IP it browses from. The session is always closed before returning.
func tunnelSessionIP(parent context.Context, name string) (ip, sessionID string, err error) {
	proxy, err := tunnelProxyItem(name)
	if err != nil {
		return "", "", err
	}
	var proxies api.ApiSessionStartRequest_Proxies
	if err := proxies.FromApiSessionStartRequestProxies0([]api.ApiSessionStartRequest_Proxies_0_Item{proxy}); err != nil {
		return "", "", fmt.Errorf("failed to set proxies: %w", err)
	}
	maxMinutes, idleMinutes := checkSessionMaxMinutes, checkSessionIdleMinutes
	body := api.ApiSessionStartRequest{Proxies: &proxies, MaxDurationMinutes: &maxMinutes, IdleTimeoutMinutes: &idleMinutes}

	client, err := GetClient()
	if err != nil {
		return "", "", err
	}

	ctx, cancel := GetContextWithTimeout(parent)
	resp, err := client.Client().SessionStartWithResponse(ctx, &api.SessionStartParams{}, body)
	cancel()
	if err != nil {
		return "", "", fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return "", "", explainTunnelStartError(err)
	}
	if resp.JSON200 == nil {
		return "", "", fmt.Errorf("session start returned no session")
	}
	sessionID = resp.JSON200.SessionId

	// Close the session even when the check fails or is interrupted, with a
	// context of its own so a cancelled check still cleans up.
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), stopTimeout)
		defer stopCancel()
		if stopResp, stopErr := client.Client().SessionStopWithResponse(stopCtx, sessionID, &api.SessionStopParams{}); stopErr != nil || HandleAPIResponse(stopResp.HTTPResponse, stopResp.Body) != nil {
			PrintInfo(fmt.Sprintf("Warning: could not stop check session %s; it closes on its own after %d idle minutes", sessionID, checkSessionIdleMinutes))
		}
	}()

	action, err := json.Marshal(map[string]string{"type": "goto", "url": ipLookupURL})
	if err != nil {
		return "", sessionID, fmt.Errorf("failed to marshal action: %w", err)
	}
	ctx, cancel = GetContextWithTimeout(parent)
	execResp, err := client.Client().PageExecuteWithBodyWithResponse(ctx, sessionID, &api.PageExecuteParams{}, "application/json", bytes.NewReader(action))
	cancel()
	if err != nil {
		return "", sessionID, fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(execResp.HTTPResponse, execResp.Body); err != nil {
		return "", sessionID, fmt.Errorf("the session started but could not open %s: %w", ipLookupURL, err)
	}
	if r := execResp.JSON200; r != nil && !r.Success {
		return "", sessionID, fmt.Errorf("the session started but could not open %s: %w", ipLookupURL, executionFailureError(r.ExceptionDetail, r.Exception, r.Message))
	}

	ctx, cancel = GetContextWithTimeout(parent)
	scrapeResp, err := client.Client().PageScrapeWithResponse(ctx, sessionID, &api.PageScrapeParams{}, api.PageScrapeJSONRequestBody{})
	cancel()
	if err != nil {
		return "", sessionID, fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(scrapeResp.HTTPResponse, scrapeResp.Body); err != nil {
		return "", sessionID, err
	}
	if scrapeResp.JSON200 == nil {
		return "", sessionID, fmt.Errorf("could not read the IP lookup page")
	}
	ip = ipv4Pattern.FindString(scrapeResp.JSON200.Markdown)
	if ip == "" {
		return "", sessionID, fmt.Errorf("the IP lookup page did not show an IP address: %q", scrapeResp.JSON200.Markdown)
	}
	return ip, sessionID, nil
}

// explainTunnelStartError adds the likely fix to a session start failure.
func explainTunnelStartError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "No Tailscale OAuth client"):
		return fmt.Errorf("%w\nConnect Tailscale at %s", err, integrationsURL())
	case strings.Contains(msg, "rejected these credentials"), strings.Contains(msg, "OAuth token exchange failed"), strings.Contains(msg, "Auth key minting failed"):
		return fmt.Errorf("%w\nThe OAuth client may have been deleted, lost its auth_keys write scope, or carry a tag other than tag:notte", err)
	case strings.Contains(msg, "exit node"):
		return fmt.Errorf("%w\nApprove the device as an exit node, and let tag:notte reach autogroup:internet in your tailnet policy", err)
	}
	return err
}

// integrationsURL is the console page where a workspace connects Tailscale.
func integrationsURL() string {
	return strings.TrimSuffix(auth.ConsoleURL(), "/") + "/settings/integrations"
}

// directHTTPClient ignores HTTP(S)_PROXY settings. The exit node sends traffic
// over this machine's direct connection, so the lookup must not go through a
// proxy that would report a different IP.
func directHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport}
}

// fetchPublicIP asks the IP lookup service for this machine's public IPv4.
func fetchPublicIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ipLookupURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := directHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned %s", ipLookupURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	ip := ipv4Pattern.FindString(string(body))
	if ip == "" {
		return "", fmt.Errorf("%s did not return an IPv4 address", ipLookupURL)
	}
	return ip, nil
}
