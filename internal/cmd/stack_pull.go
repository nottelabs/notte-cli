package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/project"
)

// pullConcurrency bounds the walk. A full pull is one page request per hundred
// functions plus two per function — the list carries no download URL — so
// marketplace's 2,049 functions cost about 4,120 requests. It runs that at 48
// without ever being rate limited, so this is not a throttling guess.
const pullConcurrency = 8

var (
	stackPullWrite bool
	stackPullLimit int
)

var stackPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Adopt functions that already exist in an environment",
	Long: `Record functions that exist remotely but not in the lockfile, and write
any whose source this tree does not have.

Run this after cloning a stack, or when deploy reports that a function already
exists upstream. Nothing is ever deleted: functions present remotely and absent
locally are reported, not removed.`,
	Args: cobra.NoArgs,
	RunE: runStackPull,
}

func init() {
	stackCmd.AddCommand(stackPullCmd)
	stackPullCmd.Flags().BoolVar(&stackPullWrite, "write-sources", true,
		"Write the source of functions this tree does not have")
	stackPullCmd.Flags().IntVar(&stackPullLimit, "limit", 0,
		"Adopt at most this many functions (0 = all)")
}

// decryptionKey re-derives the key the API uses to encrypt a download URL.
//
// Notte-managed functions return a Fernet token where the URL should be,
// keyed on the caller's own API key. The rule is duplicated from the backend
// and there is no way around that until the URL is returned decrypted; keeping
// it in one place here at least stops it spreading further.
func decryptionKey(apiKey, functionID string) string {
	sum := sha256.Sum256([]byte("api_key:" + apiKey + ":workflow_id:" + functionID + ":dumb"))
	return hex.EncodeToString(sum[:])[:64]
}

func runStackPull(cmd *cobra.Command, args []string) error {
	cfg, err := loadStack()
	if err != nil {
		return err
	}
	env := envName()

	lock, err := project.LoadLock(cfg.Root)
	if err != nil {
		return err
	}
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	remote, err := listAllFunctions(ctx, client)
	if err != nil {
		return err
	}

	local, err := project.Discover(cfg)
	if err != nil {
		return err
	}
	localByName := map[string]project.Function{}
	for _, f := range local {
		localByName[f.Name] = f
	}

	names := assignNames(remote)

	var adopted, alreadyKnown, written []string
	for _, fn := range remote {
		name := names[fn.FunctionId]
		if name == "" {
			continue
		}
		if stackPullLimit > 0 && len(adopted) >= stackPullLimit {
			break
		}

		// A function already deployed from this tree keeps its sources. The
		// artifact cannot be un-flattened, so overwriting a package with its
		// own concatenated output would destroy the sources to "sync" them.
		if existing, ok := localByName[name]; ok {
			if _, known := lock.State(existing.Entrypoint, env); known {
				alreadyKnown = append(alreadyKnown, name)
				continue
			}
			lock.Record(existing.Entrypoint, env, project.EnvState{
				FunctionID: fn.FunctionId, Version: fn.LatestVersion,
			})
			adopted = append(adopted, name)
			continue
		}

		// Unknown to this tree: it lands as a single-file function, because
		// that is genuinely what an artifact is.
		entrypoint := name + ".py"
		lock.Record(entrypoint, env, project.EnvState{
			FunctionID: fn.FunctionId, Version: fn.LatestVersion,
		})
		adopted = append(adopted, name)
		if stackPullWrite {
			written = append(written, entrypoint)
		}
	}

	var failures map[string]string
	if stackPullWrite && len(written) > 0 {
		failures = downloadSources(ctx, client, cfg, remote, written)
	}

	// The lock is written even when some downloads failed. Their entries are
	// still correct — the function exists and has that id — and discarding the
	// whole run because one function was unreadable is exactly the "authoritative
	// only for what it inspected" rule read backwards.
	if err := lock.Save(cfg.Root); err != nil {
		return err
	}

	// Local functions with no counterpart upstream are reported, never
	// removed: absence from a listing is not an instruction to delete.
	var notDeployed []string
	remoteNames := map[string]bool{}
	for _, fn := range remote {
		remoteNames[names[fn.FunctionId]] = true
	}
	for _, f := range local {
		if !remoteNames[f.Name] {
			notDeployed = append(notDeployed, f.Name)
		}
	}
	for name := range failures {
		for i, w := range written {
			if w == name+".py" {
				written = append(written[:i], written[i+1:]...)
				break
			}
		}
	}
	sort.Strings(adopted)
	sort.Strings(notDeployed)
	sort.Strings(written)

	for _, name := range adopted {
		PrintInfo("  adopted      " + name)
	}
	for _, name := range alreadyKnown {
		PrintInfo("  already      " + name)
	}
	for _, name := range notDeployed {
		PrintInfo("  local only   " + name + " (not deployed to " + env + " yet)")
	}
	for _, name := range sortedNames(failures) {
		PrintInfo("  unreadable   " + name + ": " + failures[name])
	}

	return PrintResult(
		fmt.Sprintf("\n%d adopted, %d already known, %d not deployed", len(adopted), len(alreadyKnown), len(notDeployed)),
		map[string]any{
			"env": env, "adopted": adopted, "already_known": alreadyKnown,
			"not_deployed": notDeployed, "written": written, "unreadable": failures,
		},
	)
}

var (
	slugUnsafe     = regexp.MustCompile(`[^a-z0-9_-]+`)
	slugSeparators = regexp.MustCompile(`[-_]{2,}`)
)

// assignNames gives every remote function a unique local name.
//
// Names are not unique upstream — the API has no constraint on them, and a
// real workspace has several functions called "test". Slugging without
// resolving that silently collapses them onto one path, and the lockfile keeps
// whichever id happened to be recorded last while the rest become
// unreachable. Suffixes are assigned in function-id order so the mapping is
// stable across runs rather than depending on listing order.
func assignNames(remote []api.FunctionListItemResponse) map[string]string {
	bySlug := map[string][]api.FunctionListItemResponse{}
	for _, fn := range remote {
		if slug := functionSlug(fn); slug != "" {
			bySlug[slug] = append(bySlug[slug], fn)
		}
	}

	out := map[string]string{}
	for slug, group := range bySlug {
		sort.Slice(group, func(i, j int) bool { return group[i].FunctionId < group[j].FunctionId })
		for i, fn := range group {
			if i == 0 {
				out[fn.FunctionId] = slug
				continue
			}
			out[fn.FunctionId] = fmt.Sprintf("%s_%d", slug, i+1)
		}
	}
	return out
}

// functionSlug turns an upstream name into a tree-safe identifier.
func functionSlug(fn api.FunctionListItemResponse) string {
	if fn.Name == nil {
		return ""
	}
	slug := slugUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(*fn.Name)), "_")
	// Runs of separators collapse, so "managed auth - bluesky" becomes
	// managed_auth_bluesky rather than managed_auth_-_bluesky. Hyphens
	// otherwise survive: real functions are named hn-top-posts, and
	// rewriting that would make the tree disagree with the console.
	slug = slugSeparators.ReplaceAllString(slug, "_")
	return strings.Trim(slug, "-_")
}

// listAllFunctions walks every page.
//
// A partial walk is never returned. A short listing that looks complete would
// let a caller read absence as deletion, which is the one inference this
// command must never make.
func listAllFunctions(ctx context.Context, client *api.NotteClient) ([]api.FunctionListItemResponse, error) {
	var out []api.FunctionListItemResponse
	page, size := 1, 100
	for {
		resp, err := client.Client().ListFunctionsWithResponse(ctx, &api.ListFunctionsParams{
			Page: &page, PageSize: &size,
		})
		if err != nil {
			return nil, fmt.Errorf("list functions: %w", err)
		}
		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return out, nil
		}
		out = append(out, resp.JSON200.Items...)
		if !resp.JSON200.HasNext {
			return out, nil
		}
		page++
		if page > 400 {
			return nil, fmt.Errorf("function listing did not terminate after %d pages", page)
		}
	}
}

// downloadSources fetches code for the functions being adopted.
// downloadSources fetches what it can and reports what it could not, keyed by
// function name. A published function owned by another workspace answers 403,
// and one of those must not cost the caller the other two thousand.
func downloadSources(ctx context.Context, client *api.NotteClient, cfg *project.Config,
	remote []api.FunctionListItemResponse, entrypoints []string,
) map[string]string {
	wanted := map[string]bool{}
	for _, e := range entrypoints {
		wanted[strings.TrimSuffix(e, ".py")] = true
	}

	sem := make(chan struct{}, pullConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	failures := map[string]string{}

	names := assignNames(remote)
	for _, fn := range remote {
		name := names[fn.FunctionId]
		if !wanted[name] {
			continue
		}
		wg.Add(1)
		go func(fn api.FunctionListItemResponse, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			code, err := downloadOne(ctx, client, fn.FunctionId)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures[name] = err.Error()
				return
			}
			path := filepath.Join(cfg.FunctionsPath(), name+".py")
			if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
				failures[name] = err.Error()
			}
		}(fn, name)
	}
	wg.Wait()
	return failures
}

// downloadOne resolves the signed URL and fetches the code.
func downloadOne(ctx context.Context, client *api.NotteClient, functionID string) (string, error) {
	key := decryptionKey(client.APIKey(), functionID)
	resp, err := client.Client().FunctionDownloadUrlWithResponse(ctx, functionID,
		&api.FunctionDownloadUrlParams{DecryptionKey: &key})
	if err != nil {
		return "", err
	}
	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return "", err
	}
	if resp.JSON200 == nil || resp.JSON200.Url == "" {
		return "", fmt.Errorf("the API returned no download URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.JSON200.Url, nil)
	if err != nil {
		return "", err
	}
	fetched, err := client.HTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = fetched.Body.Close() }()
	if fetched.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading source: HTTP %d", fetched.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(fetched.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
