package cmd

import (
	"fmt"
	"os"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/project"
)

// stackTarget is the endpoint a stack command writes to, and the label its
// results are recorded under. The two are resolved together and can never
// disagree.
type stackTarget struct {
	Env    string
	APIURL string
	client *api.NotteClient
}

// resolveStackTarget couples --env to the endpoint it names.
//
// The previous version resolved these separately: --env chose the lockfile key
// while the client came from the ambient NOTTE_API_URL and credential. So
// `deploy --env staging` with a prod default wrote functions to prod and filed
// their ids under staging — the destination and the record disagreeing is the
// one failure a per-environment lockfile exists to prevent, and it is silent.
//
// Now naming an environment either resolves to that environment or fails.
func resolveStackTarget(cfg *project.Config) (*stackTarget, error) {
	env := stackEnv
	_, hasBlock := cfg.Envs[env]
	if env == "" {
		// No --env: the label follows whatever endpoint is already configured,
		// so a single-environment project needs no configuration at all.
		url := auth.GetCurrentAPIURL()
		client, err := GetClient()
		if err != nil {
			return nil, err
		}
		return &stackTarget{Env: auth.ResolveEnvLabel(url), APIURL: url, client: client}, nil
	}

	// An environment named but not declared is only safe when the ambient
	// endpoint already is that environment. Checked before ResolveEnv so the
	// error can name the endpoint the caller would otherwise have hit.
	ambient := auth.GetCurrentAPIURL()
	ambientLabel := auth.ResolveEnvLabel(ambient)

	var resolved project.EnvConfig
	if hasBlock {
		var err error
		if resolved, err = cfg.ResolveEnv(env); err != nil {
			return nil, err
		}
	} else if ambientLabel != env {
		return nil, fmt.Errorf(
			"--env %s is not declared in %s, and the configured endpoint is %s (%s).\n"+
				"  add an [env.%s] block with its api_url, or drop --env to target %s",
			env, project.ConfigName, ambient, ambientLabel, env, ambientLabel)
	}

	if resolved.APIURL == "" {
		if ambientLabel != env {
			return nil, fmt.Errorf(
				"[env.%s] in %s sets no api_url, and the configured endpoint is %s (%s).\n"+
					"  add api_url to [env.%s], or drop --env to target %s",
				env, project.ConfigName, ambient, ambientLabel, env, ambientLabel)
		}
		resolved.APIURL = ambient
	}

	apiKey := resolved.APIKey
	if apiKey == "" {
		// Fall back to the keyring entry for *this* environment, which
		// internal/auth already namespaces by the same label. The global
		// NOTTE_API_KEY is deliberately not consulted: it is not tied to an
		// endpoint, so using it here is how a prod key reaches staging.
		// Chosen by endpoint, never by the section name. SetKeyringAPIKey
		// files entries under ResolveEnvLabel(url), so "api_key:staging" means
		// the credential for the staging *endpoint* — and a project is free to
		// call any endpoint whatever it likes. An earlier version preferred the
		// declared name, which meant `[env.staging] api_url = api.notte.cc`
		// sent a staging credential to production. To bind a specific
		// credential to a section, set api_key in the block.
		endpointLabel := auth.ResolveEnvLabel(resolved.APIURL)
		key, err := auth.GetKeyringAPIKeyForEnv(endpointLabel)
		if err != nil {
			return nil, fmt.Errorf(
				"no credential for the endpoint %s (%s) that [env.%s] names: %w\n"+
					"  set api_key in [env.%s], or run `notte auth login` against that endpoint",
				resolved.APIURL, endpointLabel, env, err, env)
		}
		apiKey = key
	}

	var opts []api.NotteClientOption
	if origin := os.Getenv(config.EnvRequestOrigin); origin != "" {
		opts = append(opts, api.WithRequestOrigin(origin))
	}
	client, err := api.NewClientWithURL(apiKey, resolved.APIURL, Version, opts...)
	if err != nil {
		return nil, err
	}
	return &stackTarget{Env: env, APIURL: resolved.APIURL, client: client}, nil
}
