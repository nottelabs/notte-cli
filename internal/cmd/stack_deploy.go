package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/bundle"
	"github.com/nottelabs/notte-cli/internal/project"
)

var (
	stackDeployYes         bool
	stackDeployForceCreate bool
	stackDeployAllowSecret bool
)

var stackDeployCmd = &cobra.Command{
	Use:   "deploy [target]",
	Short: "Build, validate and upload every function",
	Long: `Bundle each function, validate it against the runtime, upload what
changed, and apply any schedules.

A function is uploaded only when its sources have changed since the last
deploy to this environment, tracked per environment in notte.lock.json.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStackDeploy,
}

func init() {
	stackCmd.AddCommand(stackDeployCmd)
	stackDeployCmd.Flags().BoolVar(&stackDeployYes, "yes", false, "Do not ask before writing")
	stackDeployCmd.Flags().BoolVar(&stackDeployForceCreate, "force-create", false,
		"Create even when a function of the same name already exists remotely")
	stackDeployCmd.Flags().BoolVar(&stackDeployAllowSecret, "allow-missing-secrets", false,
		"Schedule even when required secrets are not configured")
}

// plannedWrite is one function about to be created or updated.
type plannedWrite struct {
	fn       project.Function
	artifact *bundle.Result
	// existingID is empty for a create.
	existingID string
}

func runStackDeploy(cmd *cobra.Command, args []string) error {
	target := ""
	if len(args) == 1 {
		target = args[0]
	}
	// Validation runs first and in full. Uploading code that check would have
	// rejected just moves the failure somewhere more expensive.
	prep, err := prepareStack(cmd, target)
	if err != nil {
		return err
	}
	if prep.failed > 0 {
		reportChecked(prep.results, prep.failed)
		return fmt.Errorf("%d function(s) failed validation; nothing was uploaded", prep.failed)
	}

	lock, err := project.LoadLock(prep.cfg.Root)
	if err != nil {
		return err
	}

	// The same client the runtime report came from, so the endpoint that was
	// validated against is the endpoint written to.
	env := prep.target.Env
	client := prep.target.client
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	remote, err := remoteFunctionsByName(ctx, client)
	if err != nil {
		return err
	}

	var writes []plannedWrite
	var unchanged []string
	for _, fn := range prep.selected {
		art := prep.artifacts[fn.Name]
		state, known := lock.State(fn.Entrypoint, env)
		if known && state.SourceSHA256 == art.SourceSHA256 {
			unchanged = append(unchanged, fn.Name)
			continue
		}

		w := plannedWrite{fn: fn, artifact: art, existingID: state.FunctionID}
		if w.existingID == "" {
			// Nothing in the lock, but a function of this name upstream. The
			// API has no unique constraint on name, so creating would silently
			// make a second one while callers keep the id of the first.
			if id, clash := remote[deployName(prep.cfg, fn.Name)]; clash && !stackDeployForceCreate {
				return fmt.Errorf(
					"%q already exists in %s but is not in %s, so deploying would create a duplicate.\n"+
						"  adopt the existing one:  notte stack pull --env %s\n"+
						"  or create a second:      --force-create   (existing id %s)",
					fn.Name, env, project.LockName, env, id)
			}
		}
		writes = append(writes, w)
	}

	for _, name := range unchanged {
		PrintInfo(fmt.Sprintf("  unchanged  %s", name))
	}
	if len(writes) == 0 {
		return PrintResult("nothing to deploy", map[string]any{"unchanged": unchanged, "deployed": []any{}})
	}

	if err := confirmWrites(writes, env); err != nil {
		return err
	}

	configured, err := configuredSecretNames(ctx, client)
	if err != nil {
		return err
	}

	deployed := make([]map[string]any, 0, len(writes))
	var refused int
	for _, w := range writes {
		result, err := uploadFunction(ctx, client, prep.cfg, w)
		if err != nil {
			// Record nothing: the write did not land.
			return fmt.Errorf("%s: %w", w.fn.Name, err)
		}

		// The hash advances to what was pushed even though the version may not
		// have been read back. Tying it to the read-back means a transient
		// error on the confirmation mints a duplicate version on the next run.
		lock.Record(w.fn.Entrypoint, env, project.EnvState{
			FunctionID:     result.id,
			Version:        result.version,
			SourceSHA256:   w.artifact.SourceSHA256,
			ArtifactSHA256: w.artifact.ArtifactSHA256,
		})
		if err := lock.Save(prep.cfg.Root); err != nil {
			return err
		}

		verb := "updated"
		if w.existingID == "" {
			verb = "created"
		}
		PrintInfo(fmt.Sprintf("  %s  %-24s %s", verb, w.fn.Name, result.version))

		missing := missingSecrets(result.requiredSecrets, configured)
		entry := map[string]any{
			"name": w.fn.Name, "function_id": result.id, "version": result.version,
			"action": verb, "missing_secrets": missing,
		}
		// Metadata is applied on every deploy, not only at create. Sending it
		// with the multipart upload sets it once and then never again, so an
		// edit to name or description in notte.toml would silently never
		// reach the deployed function — the gap marketplace still has, where
		// copy is only editable upstream.
		// A metadata failure never fails the deploy. The upload has landed, and
		// refusing the whole command over a catalog field would report a
		// success as a failure — the same mistake the secrets path avoids.
		// self_healing in particular is refused outright for anything the CLI
		// created: it resumes the thread that built the function, and a
		// CLI-deployed one has none.
		changed, err := applyMetadata(ctx, client, prep.cfg, w.fn.Name, result.id)
		if err != nil {
			PrintInfo("       metadata not applied: " + err.Error())
			entry["metadata_error"] = err.Error()
		} else if len(changed) > 0 {
			PrintInfo("       configured " + strings.Join(changed, ", "))
			entry["configured"] = changed
		}

		if len(missing) > 0 {
			PrintInfo(fmt.Sprintf("       missing secrets: %s — it will fail when invoked", strings.Join(missing, ", ")))
			for _, name := range missing {
				// The name is positional; `--name` is not a flag this command
				// takes. A suggestion that fails when pasted is worse than none.
				PrintInfo(fmt.Sprintf("         notte functions secrets set %s <value>", name))
			}
		}

		// Only a function that asked for a schedule can have one refused.
		cron := prep.cfg.Functions[w.fn.Name].Cron
		switch {
		case cron == "":
			// nothing to schedule
		case len(missing) > 0 && !stackDeployAllowSecret:
			refused++
			entry["scheduled"] = false
			PrintInfo("       NOT scheduled — a scheduled run would fail preflight (--allow-missing-secrets to override)")
		default:
			if err := applySchedule(ctx, client, result.id, cron, prep.cfg.Functions[w.fn.Name].CronVariables); err != nil {
				return fmt.Errorf("%s: schedule: %w", w.fn.Name, err)
			}
			entry["scheduled"] = true
			PrintInfo(fmt.Sprintf("       scheduled  %s", cron))
		}
		deployed = append(deployed, entry)
	}

	if err := PrintResult("", map[string]any{
		"env": env, "deployed": deployed, "unchanged": unchanged, "schedules_refused": refused,
	}); err != nil {
		return err
	}
	if refused > 0 {
		// The uploads landed; only the schedules did not. Exit non-zero because
		// something asked for was not done — but never pretend nothing happened.
		return fmt.Errorf("%d schedule(s) not applied because required secrets are missing", refused)
	}
	return nil
}

// confirmWrites shows what is about to be written and asks.
//
// Non-interactive callers must pass --yes rather than being assumed to have
// agreed: marketplace's push refuses without a terminal for the same reason,
// and names the ways out instead of failing silently.
func confirmWrites(writes []plannedWrite, env string) error {
	PrintInfo(fmt.Sprintf("\nAbout to write %d function(s) to %s:", len(writes), env))
	for _, w := range writes {
		verb := "update"
		if w.existingID == "" {
			verb = "create"
		}
		PrintInfo(fmt.Sprintf("  %-7s %-24s %s", verb, w.fn.Name, short(w.artifact.ArtifactSHA256)))
	}
	if stackDeployYes || skipConfirmation {
		return nil
	}
	if !stdinIsTerminal() {
		return fmt.Errorf("refusing to write %d change(s) without confirmation, and there is no terminal to ask.\n"+
			"  pass --yes to proceed, or run `notte stack check` to review first", len(writes))
	}
	ok, err := confirmDeploy(os.Stdin, os.Stderr, env)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cancelled")
	}
	return nil
}

// stdinIsTerminal reports whether there is someone to ask.
func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// confirmDeploy asks, defaulting to no on a bare Enter or EOF.
func confirmDeploy(in io.Reader, out io.Writer, env string) (bool, error) {
	if _, err := fmt.Fprintf(out, "\nDeploy to %s? [y/N]: ", env); err != nil {
		return false, err
	}
	response, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// deployName is the name a function carries upstream.
func deployName(cfg *project.Config, name string) string {
	if configured := cfg.Functions[name].Name; configured != "" {
		return configured
	}
	return name
}

func missingSecrets(required []string, configured map[string]bool) []string {
	var missing []string
	for _, name := range required {
		if !configured[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// configuredSecretNames lists function_env secret names.
//
// Names only. Reading a value writes an audit row and bumps last_used_at, so a
// deploy that reads every secret every run is a bad neighbour to whoever has
// to read that log.
func configuredSecretNames(ctx context.Context, client *api.NotteClient) (map[string]bool, error) {
	namespace := api.FunctionEnv
	resp, err := client.Client().ListSecretsWithResponse(ctx, &api.ListSecretsParams{Namespace: &namespace})
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	if resp.JSON200 != nil {
		for _, s := range resp.JSON200.Items {
			out[s.Name] = true
		}
	}
	return out, nil
}

// remoteFunctionsByName maps upstream names to ids, for the duplicate guard.
func remoteFunctionsByName(ctx context.Context, client *api.NotteClient) (map[string]string, error) {
	functions, err := listAllFunctions(ctx, client)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, f := range functions {
		if f.Name != nil {
			out[*f.Name] = f.FunctionId
		}
	}
	return out, nil
}

type uploadResult struct {
	id              string
	version         string
	requiredSecrets []string
}

func uploadFunction(ctx context.Context, client *api.NotteClient, cfg *project.Config, w plannedWrite) (*uploadResult, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", w.fn.Name+".py")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write([]byte(w.artifact.Code)); err != nil {
		return nil, err
	}

	if w.existingID == "" {
		// Only what create needs to exist at all. Everything else is applied
		// afterwards through the metadata endpoint, so the same code path runs
		// whether this is a create or an update.
		if err := writer.WriteField("name", deployName(cfg, w.fn.Name)); err != nil {
			return nil, err
		}
		if cfg.Functions[w.fn.Name].Shared {
			if err := writer.WriteField("shared", "true"); err != nil {
				return nil, err
			}
		}
	}
	_ = writer.Close()

	var fn *api.FunctionResponse
	if w.existingID == "" {
		resp, err := client.Client().FunctionCreateWithBodyWithResponse(ctx,
			&api.FunctionCreateParams{}, writer.FormDataContentType(), &buf)
		if err != nil {
			return nil, err
		}
		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return nil, err
		}
		fn = resp.JSON200
	} else {
		resp, err := client.Client().FunctionUpdateWithBodyWithResponse(ctx, w.existingID,
			&api.FunctionUpdateParams{}, writer.FormDataContentType(), &buf)
		if err != nil {
			return nil, err
		}
		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return nil, err
		}
		fn = resp.JSON200
	}
	if fn == nil {
		return nil, fmt.Errorf("the API accepted the upload but returned no function")
	}

	res := &uploadResult{id: fn.FunctionId, version: fn.LatestVersion}
	if fn.RequiredSecrets != nil {
		res.requiredSecrets = *fn.RequiredSecrets
	}
	return res, nil
}

// applyMetadata pushes the catalog fields notte.toml owns, and reports which
// ones it sent.
//
// It is a no-op when the config sets none, so a project that names nothing
// makes no extra call. self_healing is a pointer in the config precisely so
// that "unset" and "explicitly false" differ: turning a feature off because a
// file did not mention it would be a surprising deploy.
func applyMetadata(ctx context.Context, client *api.NotteClient, cfg *project.Config,
	name, functionID string,
) ([]string, error) {
	fc := cfg.Functions[name]
	body := api.FunctionMetadataUpdateJSONRequestBody{}
	var changed []string

	if configured := deployName(cfg, name); fc.Name != "" {
		body.Name = &configured
		changed = append(changed, "name")
	}
	if fc.Description != "" {
		body.Description = &fc.Description
		changed = append(changed, "description")
	}
	if fc.Domain != "" {
		body.Domain = &fc.Domain
		changed = append(changed, "domain")
	}
	if fc.Instructions != "" {
		body.Instructions = &fc.Instructions
		changed = append(changed, "instructions")
	}
	if fc.SelfHealing != nil {
		body.SelfHealing = fc.SelfHealing
		changed = append(changed, "self_healing")
	}
	if len(changed) == 0 {
		return nil, nil
	}

	resp, err := client.Client().FunctionMetadataUpdateWithResponse(ctx, functionID,
		&api.FunctionMetadataUpdateParams{}, body)
	if err != nil {
		return nil, fmt.Errorf("configure: %w", err)
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, fmt.Errorf("configure: %w", err)
	}
	return changed, nil
}

func applySchedule(ctx context.Context, client *api.NotteClient, functionID, cron string, variables map[string]any) error {
	vars := map[string]interface{}{}
	for k, v := range variables {
		vars[k] = v
	}
	resp, err := client.Client().FunctionScheduleSetWithResponse(ctx, functionID,
		&api.FunctionScheduleSetParams{},
		api.FunctionScheduleSetJSONRequestBody{Cron: cron, Variables: &vars})
	if err != nil {
		return err
	}
	return HandleAPIResponse(resp.HTTPResponse, resp.Body)
}
