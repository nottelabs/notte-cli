package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var stackSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Compare the secrets your functions need against what is configured",
}

var stackSecretsDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Report required secrets that are not configured",
	Long: `List every secret the deployed functions declare and show which are
missing from this environment.

What a function requires is computed server-side at upload, by scanning for
os.environ reads plus any NOTTE_REQUIRED_SECRETS constant — so this reflects
what the API will preflight, not a local guess.`,
	Args: cobra.NoArgs,
	RunE: runStackSecretsDiff,
}

var stackSecretsPushCmd = &cobra.Command{
	Use:   "push [file]",
	Short: "Set missing secrets from a .env file",
	Long: `Read KEY=VALUE lines and set the ones this environment is missing.

Defaults to .env.<env>, which the scaffolded .gitignore already excludes.
Existing secrets are never touched: the API has no update, so changing one
means delete-then-create, and doing that implicitly would leave a window where
a live function has no secret at all.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStackSecretsPush,
}

func init() {
	stackCmd.AddCommand(stackSecretsCmd)
	stackSecretsCmd.AddCommand(stackSecretsDiffCmd)
	stackSecretsCmd.AddCommand(stackSecretsPushCmd)
}

// requiredSecrets is the union of what every deployed function declares.
func requiredSecrets(ctx context.Context, client *api.NotteClient) (map[string][]string, error) {
	functions, err := listAllFunctions(ctx, client)
	if err != nil {
		return nil, err
	}
	byName := map[string][]string{}
	for _, fn := range functions {
		if fn.RequiredSecrets == nil {
			continue
		}
		for _, secret := range *fn.RequiredSecrets {
			name := "?"
			if fn.Name != nil {
				name = *fn.Name
			}
			byName[secret] = append(byName[secret], name)
		}
	}
	for secret := range byName {
		sort.Strings(byName[secret])
	}
	return byName, nil
}

func runStackSecretsDiff(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	required, err := requiredSecrets(ctx, client)
	if err != nil {
		return err
	}
	configured, err := configuredSecretNames(ctx, client)
	if err != nil {
		return err
	}

	var missing, satisfied []string
	for secret := range required {
		if configured[secret] {
			satisfied = append(satisfied, secret)
		} else {
			missing = append(missing, secret)
		}
	}
	// Configured but required by nothing. Reported, never deleted: a secret
	// may exist for a function that has not been deployed yet.
	var extra []string
	for name := range configured {
		if _, needed := required[name]; !needed {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(satisfied)
	sort.Strings(extra)

	if IsJSONOutput() {
		return GetFormatter().Print(map[string]any{
			"env": envName(), "missing": missing, "satisfied": satisfied, "unused": extra,
		})
	}
	for _, s := range missing {
		PrintInfo(fmt.Sprintf("  ✗ %-28s required by %s", s, strings.Join(required[s], ", ")))
	}
	for _, s := range satisfied {
		PrintInfo(fmt.Sprintf("  ✓ %-28s required by %s", s, strings.Join(required[s], ", ")))
	}
	for _, s := range extra {
		PrintInfo(fmt.Sprintf("  · %-28s set but required by no deployed function", s))
	}
	if len(missing) == 0 && len(satisfied) == 0 && len(extra) == 0 {
		PrintInfo("no function declares a secret, and none are configured")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%d required secret(s) are not configured", len(missing))
	}
	return nil
}

var envLine = regexp.MustCompile(`^\s*(?:export\s+)?([A-Z_][A-Z0-9_]*)\s*=\s*(.*)$`)

func runStackSecretsPush(cmd *cobra.Command, args []string) error {
	cfg, err := loadStack()
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.Root, ".env."+envName())
	if len(args) == 1 {
		path = args[0]
	}

	values, err := readEnvFile(path)
	if err != nil {
		return err
	}
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	configured, err := configuredSecretNames(ctx, client)
	if err != nil {
		return err
	}

	var created, skipped []string
	for _, name := range sortedNames(values) {
		if configured[name] {
			skipped = append(skipped, name)
			continue
		}
		body := api.StoreSecretJSONRequestBody{
			Name: name, Namespace: api.FunctionEnv, Value: values[name],
		}
		resp, err := client.Client().StoreSecretWithResponse(ctx, &api.StoreSecretParams{}, body)
		if err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
		created = append(created, name)
	}

	for _, name := range created {
		PrintInfo("  set      " + name)
	}
	for _, name := range skipped {
		PrintInfo("  exists   " + name + " (delete it first to change the value)")
	}
	return PrintResult(
		fmt.Sprintf("\n%d set, %d already configured", len(created), len(skipped)),
		map[string]any{"created": created, "skipped": skipped, "source": path},
	)
}

// readEnvFile parses KEY=VALUE lines. Values are never echoed anywhere.
func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w\n  create it, or pass a path", filepath.Base(path), err)
	}
	defer func() { _ = file.Close() }()

	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := envLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[2])
		value = strings.Trim(value, `"'`)
		out[m[1]] = value
	}
	return out, scanner.Err()
}

func sortedNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
