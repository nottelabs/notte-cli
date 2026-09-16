package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/browser"
)

var (
	syncBrowser       string
	syncChromeProfile string
	syncDomains       []string
	syncMode          string
	syncProfileName   string
)

var profilesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Upload cookies from your local browser into a Notte profile",
	Long: "Read the cookies from a local Firefox, Chrome, Brave, Edge or Chromium profile\n" +
		"and upload them into a Notte cloud profile, so remote sessions and agents\n" +
		"start already logged in.\n\n" +
		"Cookies are read from a copy of the browser's own files on this machine and\n" +
		"decrypted locally. Nothing leaves your computer until you confirm the upload,\n" +
		"and with --domain you choose exactly which sites are included.\n\n" +
		"With --profile-id the cookies go into an existing profile; without it a new\n" +
		"profile is created and its id is printed for you to reuse next time.",
	Args: cobra.NoArgs,
	RunE: runProfilesSync,
}

func init() {
	profilesCmd.AddCommand(profilesSyncCmd)
	profilesSyncCmd.Flags().StringVar(&profileID, "profile-id", "", "Existing Notte profile to sync into (creates a new one if omitted)")
	profilesSyncCmd.Flags().StringVar(&syncBrowser, "browser", "", "Browser to read from (firefox, chrome, brave, edge, chromium)")
	profilesSyncCmd.Flags().StringVar(&syncChromeProfile, "local-profile", "", "Local browser profile, by name or directory (prompts if there is more than one)")
	profilesSyncCmd.Flags().StringSliceVar(&syncDomains, "domain", nil, "Only sync cookies for these domains and their subdomains (repeatable)")
	profilesSyncCmd.Flags().StringVar(&syncMode, "mode", "replace", "How to apply the cookies: replace the profile's cookies, or append to them")
	profilesSyncCmd.Flags().StringVar(&syncProfileName, "name", "", "Name for the new profile when --profile-id is omitted")
}

func runProfilesSync(cmd *cobra.Command, args []string) error {
	if syncMode != "replace" && syncMode != "append" {
		return fmt.Errorf("invalid --mode %q: expected \"replace\" or \"append\"", syncMode)
	}

	profile, err := resolveLocalProfile()
	if err != nil {
		return err
	}

	PrintInfo(fmt.Sprintf("Reading cookies from %s profile %q ...", profile.Browser.DisplayName, profile.Name))
	result, err := profile.ReadCookies(syncDomains)
	if err != nil {
		return err
	}
	if len(result.Cookies) == 0 {
		if len(syncDomains) > 0 {
			return fmt.Errorf("no cookies found for %s in that profile", strings.Join(syncDomains, ", "))
		}
		return fmt.Errorf("no cookies found in that profile")
	}

	cookies := toAPICookies(result.Cookies)
	printCookieSummary(result, cookies)

	// Decide where the cookies will go and confirm before touching the network,
	// so a decline (or a dry run) never creates anything.
	newName := ""
	target := profileID
	if target == "" {
		newName = syncProfileName
		if newName == "" {
			newName = fmt.Sprintf("%s - %s", profile.Browser.DisplayName, profile.Name)
		}
		target = fmt.Sprintf("a new profile named %q", newName)
	}
	if !confirmSync(len(cookies), target) {
		return PrintResult("Cancelled.", map[string]any{"cancelled": true})
	}

	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	targetID := profileID
	created := false
	if targetID == "" {
		id, err := createProfile(ctx, client, newName)
		if err != nil {
			return err
		}
		targetID = id
		created = true
	}

	mode := api.ProfileCookiesImportRequestMode(syncMode)
	// The cookies are already normalized to the Playwright shape (string
	// sameSite, Unix-seconds expiry) in toAPICookies, so we declare that shape
	// rather than "chrome": the server cannot convert Chrome's raw integer
	// sameSite or WebKit timestamps, and must be told what it is actually given.
	format := api.ProfileCookiesImportRequestSourceFormatPlaywright
	body := api.ProfileCookiesImportRequest{
		Cookies:      cookies,
		Mode:         &mode,
		SourceFormat: &format,
	}

	resp, err := client.Client().ProfileCookiesSetWithResponse(ctx, targetID, &api.ProfileCookiesSetParams{}, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	if IsJSONOutput() {
		out := map[string]any{
			"profile_id": targetID,
			"created":    created,
			"synced":     len(cookies),
		}
		if resp.JSON200 != nil {
			out["cookies_count"] = resp.JSON200.CookiesCount
			out["mode"] = resp.JSON200.Mode
		}
		return GetFormatter().Print(out)
	}

	printSyncSuccess(targetID, len(cookies), resp.JSON200)
	return nil
}

// resolveLocalProfile picks which local browser profile to read, from flags or
// by prompting when the choice is ambiguous.
func resolveLocalProfile() (browser.Profile, error) {
	var candidates []browser.Profile
	if syncBrowser != "" {
		b, ok := browser.BrowserByID(syncBrowser)
		if !ok {
			return browser.Profile{}, fmt.Errorf("unknown browser %q: supported browsers are firefox, chrome, brave, edge, chromium", syncBrowser)
		}
		if !b.Installed() {
			return browser.Profile{}, fmt.Errorf("%s does not appear to be installed", b.DisplayName)
		}
		profiles, err := b.Profiles()
		if err != nil {
			return browser.Profile{}, err
		}
		candidates = profiles
	} else {
		candidates = browser.InstalledProfiles()
	}

	if len(candidates) == 0 {
		return browser.Profile{}, fmt.Errorf("no supported browser profiles found on this machine")
	}

	if syncChromeProfile != "" {
		matched := filterByProfileName(candidates, syncChromeProfile)
		switch len(matched) {
		case 0:
			return browser.Profile{}, fmt.Errorf("no local profile matches %q", syncChromeProfile)
		case 1:
			return matched[0], nil
		default:
			return browser.Profile{}, fmt.Errorf("%q matches more than one profile; also pass --browser to disambiguate", syncChromeProfile)
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return pickProfile(candidates)
}

func filterByProfileName(profiles []browser.Profile, query string) []browser.Profile {
	q := strings.ToLower(query)
	var matched []browser.Profile
	for _, p := range profiles {
		if strings.ToLower(p.Dir) == q || strings.ToLower(p.Name) == q {
			matched = append(matched, p)
		}
	}
	return matched
}

// pickProfile shows a numbered list and reads a choice. It requires an
// interactive terminal; in a non-interactive context it tells the caller how to
// choose explicitly instead of guessing.
func pickProfile(profiles []browser.Profile) (browser.Profile, error) {
	if skipConfirmation || !term.IsTerminal(int(os.Stdin.Fd())) {
		return browser.Profile{}, fmt.Errorf("more than one browser profile found; pass --browser and --local-profile to choose one")
	}
	return pickProfileWithIO(os.Stdin, os.Stderr, profiles)
}

func pickProfileWithIO(in io.Reader, out io.Writer, profiles []browser.Profile) (browser.Profile, error) {
	_, _ = fmt.Fprintln(out, "Which browser profile do you want to sync?")
	for i, p := range profiles {
		label := fmt.Sprintf("%s - %s", p.Browser.DisplayName, p.Name)
		if p.Email != "" {
			label += fmt.Sprintf(" (%s)", p.Email)
		}
		_, _ = fmt.Fprintf(out, "  %d) %s\n", i+1, label)
	}
	_, _ = fmt.Fprintf(out, "Enter a number [1-%d]: ", len(profiles))

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return browser.Profile{}, fmt.Errorf("failed to read choice: %w", err)
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(profiles) {
		return browser.Profile{}, fmt.Errorf("invalid choice")
	}
	return profiles[choice-1], nil
}

func toAPICookies(cookies []browser.Cookie) []api.Cookie {
	out := make([]api.Cookie, 0, len(cookies))
	for _, c := range cookies {
		secure := c.Secure
		item := api.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HttpOnly: c.HTTPOnly,
			Secure:   &secure,
		}
		if c.SameSite != "" {
			ss := c.SameSite
			item.SameSite = &ss
		}
		if c.Session {
			session := true
			item.Session = &session
		} else if c.Expires > 0 {
			exp := float32(c.Expires)
			item.Expires = &exp
			item.ExpirationDate = &exp
		}
		out = append(out, item)
	}
	return out
}

// domainCounts returns the per-domain cookie counts, sorted by domain, without
// ever exposing a cookie value.
func domainCounts(cookies []api.Cookie) []struct {
	Domain string
	Count  int
} {
	counts := map[string]int{}
	for _, c := range cookies {
		d := strings.TrimPrefix(strings.ToLower(c.Domain), ".")
		counts[d]++
	}
	domains := make([]string, 0, len(counts))
	for d := range counts {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	out := make([]struct {
		Domain string
		Count  int
	}, 0, len(domains))
	for _, d := range domains {
		out = append(out, struct {
			Domain string
			Count  int
		}{d, counts[d]})
	}
	return out
}

func printCookieSummary(result browser.ReadResult, cookies []api.Cookie) {
	if IsJSONOutput() {
		return
	}
	counts := domainCounts(cookies)
	PrintInfo(fmt.Sprintf("Found %d cookies across %d domains:", len(cookies), len(counts)))
	shown := counts
	const maxShown = 12
	if len(shown) > maxShown {
		shown = shown[:maxShown]
	}
	for _, c := range shown {
		PrintInfo(fmt.Sprintf("  %-32s %d", c.Domain, c.Count))
	}
	if len(counts) > len(shown) {
		PrintInfo(fmt.Sprintf("  ... and %d more domains", len(counts)-len(shown)))
	}
	if result.DecryptFailures > 0 {
		PrintInfo(fmt.Sprintf("(%d cookies could not be decrypted and were skipped)", result.DecryptFailures))
	}
}

// createProfile creates a new empty Notte profile and returns its id.
func createProfile(ctx context.Context, client *api.NotteClient, name string) (string, error) {
	resp, err := client.Client().ProfileCreateWithResponse(ctx, &api.ProfileCreateParams{}, api.ProfileCreateRequest{Name: &name})
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return "", err
	}
	if resp.JSON200 == nil {
		return "", fmt.Errorf("profile creation returned no profile")
	}
	return resp.JSON200.ProfileId, nil
}

func confirmSync(cookieCount int, target string) bool {
	if skipConfirmation {
		return true
	}
	return confirmSyncWithIO(os.Stdin, os.Stderr, cookieCount, target)
}

func confirmSyncWithIO(in io.Reader, out io.Writer, cookieCount int, target string) bool {
	_, _ = fmt.Fprintf(out, "Upload %d cookies into %s in mode %q? [y/N]: ", cookieCount, target, syncMode)

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func printSyncSuccess(profileID string, synced int, resp *api.ProfileCookiesImportResponse) {
	total := synced
	if resp != nil {
		total = resp.CookiesCount
	}
	fmt.Printf("Synced %d cookies into profile %s (%d cookies total).\n", synced, profileID, total)
	fmt.Println()
	fmt.Println("Use it in a session:")
	fmt.Printf("  notte sessions start --profile-id %s\n", profileID)
	fmt.Println()
	fmt.Println("Re-run this any time your local login changes:")
	fmt.Printf("  notte profiles sync --profile-id %s\n", profileID)
}
