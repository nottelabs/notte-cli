# RFC 0001 — A `notte` project: scaffolding, bundling, and declarative deploys

| | |
|---|---|
| **Status** | Draft — for discussion, nothing implemented |
| **Author** | Lucas Giordano |
| **Date** | 2026-08-27 |
| **v1 scope** | functions, secrets, schedules. Managed-auth connections later. |

---

## Context

We have built the same framework twice, by hand:

| | `anything-api/marketplace` | `monorepo/apps/back/managed-auth` |
|---|---|---|
| What | 2,049 `.py` functions across 670 domains | 9 connectors = 18 functions (login + verifier) |
| Config | one 2.2 MB `marketplace/manifest.json` | one `connectors/<slug>.json` per connector |
| Deploy | `scripts/marketplace-catalog.ts` (2,404 lines TS) | `scripts/deploy.py` (996 lines, stdlib-only) |
| Driver | `make marketplace push prod` | `make deploy google.com staging` |
| Env selection | `$(filter …,$(MAKECMDGOALS))` + failing no-op rules | `%::` catch-all so URLs with `:` survive |
| Create vs update | manifest lockfile: `envs[env].code_sha256 != local` | server decides from `revision` + per-file sha256 |
| Shared code | **none — 0 relative imports in 2,049 files**; `has_value`/`clean_*` reimplemented hundreds of times | `contract.py` spliced in by one regex; `_verification_code()` hand-copied across 4 connectors |
| CI | designed for, never wired up | full 6-job promotion pipeline |

Both converged on the same shape. Both spent their worst code on the same two problems: **faking subcommands in `make`**, and **not having a bundler**. Meanwhile the CLI already owns auth, config, output formatting, and every `functions` API call these scripts shell out to.

Goal: fold the framework into `notte`, so a functions repo is `notte init` + `notte deploy`, and so `from ._shared.http import get` actually works.

---

## Scope

Three kinds of thing, and only the first two belong in git:

| | Owner | In v1 |
|---|---|---|
| **Code** — function sources, shared modules | git, bundled by the CLI | ✅ |
| **Config** — function name/description/shared, cron, *which* secrets are required | `notte.toml` | ✅ |
| **Secret values** | gitignored `.env.<env>`, pushed explicitly | ✅ (`notte secrets push/diff`) |
| **Runtime/user data** — managed-auth connections, sessions, personas, vaults, runs | the API; never reconciled from a file | ❌ (operational commands only) |

Managed-auth **templates** are already declarative and could join later (see below). Managed-auth **connections** should probably never be — creating one runs a real browser login, spends money, and provisions a vault + a profile as side effects.

---

## The hard constraint nobody can design around

There are **two** gates, with different scopes, and conflating them is easy — an earlier draft of this document did exactly that. Getting the distinction right matters, because it changes which arguments are load-bearing:

| | Upload | Runtime |
|---|---|---|
| Where | `POST /functions`, `?restricted=` **defaults to `True`** (`notte_api/functions/endpoints.py:353,411`) | `workflows-lambda/worker.py:891` calls `run_script(..., restricted=False)` |
| RestrictedPython AST policy | **enforced** — `FORBIDDEN_NODES`, `FORBIDDEN_CALLS`, the lot | **not applied at all** — `worker.py:608` takes the `restricted=False` branch and uses plain `compile()` |
| Import allowlist | enforced statically over the source | **still enforced**, dynamically: `__import__` is `runner.safe_import`, which calls `check_valid_import(name)` on every import (`worker.py:572-578`) |
| Which list | `notte_api.ast.ALLOWED_IMPORTS` | `_LAMBDA_ALLOWED_IMPORTS` + `{httpx, httpcloak}`, minus `tempfile` (`worker.py:520`) |

So the AST policy is an upload-time quality gate, while **the import allowlist is a genuine runtime guard** — enforced by name, on every import, by the last guard standing once `restricted=False`.

**1. Local imports are structurally impossible server-side.**
`notte-api/src/notte_api/ast.py`, `visit_ImportFrom`:
```python
if node.module is None:
    raise SyntaxError("Relative imports are not allowed")
```
`from . import util` dies there. `from .util import x` survives that check (module is `"util"`) but then hits `check_valid_import("util")` → not in `ALLOWED_IMPORTS` → rejected. Plain `import util` likewise. **There is no server-side path to multi-file. Bundling must be client-side.**

**2. Off-the-shelf bundlers don't help — and `os`/`sys` isn't the reason.**

An earlier draft said stickytape and pinliner were impossible because RestrictedPython forbids `sys`, `exec`, `compile`, `__import__` and `os`. **That was the wrong mechanism** — the AST policy isn't applied at execution time, so pointing at `FORBIDDEN_CALLS` proves nothing about what a deployed function can do. The real blockers are further down, and they bite in this order:

1. **`import tempfile` fails first.** `worker.py:520` does `ALLOWED_IMPORTS.discard("tempfile")`, so stickytape's `mkdtemp()` dies before a single module is written.
2. **`import util` — the entire point — is rejected by name at run time.** stickytape exists to make a written-to-disk module importable; that import goes through `safe_import("util")` → `check_valid_import("util")` → not in the allowlist. The one thing it does is the one thing that's gated.
3. `os`, `sys` and `shutil` are on the **runtime** denylist too (`_LAMBDA_DISALLOWED_STDLIB_IMPORTS`), so none of this is an upload-time gate a query parameter can switch off.

**On the obvious counter-proposal — "just allow `os` and `sys`":** it is neither necessary nor sufficient. It doesn't touch (1) or (2). Making stickytape work means disabling `safe_import`, i.e. permitting arbitrary imports by name at run time. That is a materially bigger decision than allowing two modules, and unlike the original framing it carries a real security dimension, because `safe_import` is the only import guard left once `restricted=False`. It deserves to be decided on its own merits, not adopted as a side effect of a bundling convenience.

**Three reasons to flatten that don't depend on any allowlist.** These are the actual argument, and the earlier draft buried them beneath a claim that turned out to be wrong:

- **The artifact stops being readable.** stickytape emits a prelude plus every module as an escaped bytes literal passed to `__stickytape_write_module`. The deployed file is what the console renders, what `functions show` downloads, and what marketplace's `push`/`check` diff against. A blob kills the diff model, and makes tracebacks worse rather than better.
- **stickytape disclaims itself.** Its README: *"bodged together… for a specific use case"*, no `from __future__` imports, `__file__` unreliable, dynamic imports need manual flags. That is an unmaintained third-party dependency sitting in the deploy path.
- **It's Python.** Adopting it reintroduces the "`notte deploy` fails on a machine where `notte` works" problem that is the entire reason for the Go-native recommendation below.

So: the bundler must be a **static flattener** emitting plain, ordinary Python — one module namespace, no runtime machinery. **That recommendation stands even in a fully permissive runtime**, which is how it should have been argued in the first place.

**3. Dependencies are a fixed allowlist, so there is no dependency resolution to build.**
`ALLOWED_IMPORTS = set(sys.stdlib_module_names) - DISALLOWED_STDLIB_IMPORTS | {notte, notte_sdk, notte_core, notte_browser, notte_agent, notte_llm, pydantic, loguru, requests, httpx, httpcloak, playwright, gspread, google, litellm, bs4, pipedream, tqdm, typing_extensions}`.

No `pip install`, no `requirements.txt`, no PEP-723 block. The CLI's only job is to **check imports against the allowlist at build time** so you get the error in 20 ms locally instead of after a multipart upload.

Two more upload-time contracts:
- `extract_env_requirements(source)` (`notte_api/functions/requirements.py`) AST-scans for literal `os.environ[...]`/`os.getenv(...)`/`.get`/`.setdefault`/`.pop` plus aliases, unions with an optional module-level `NOTTE_REQUIRED_SECRETS = [...]` list, subtracts reserved names, and persists to `functions.required_secrets`. Env vars are read via `from notte_sdk.types import os` (bare `import os` is blocked). **This is a ready-made input for the secrets planner.**
- `response_format` (a JSON Schema) is accepted only if `check_run_returns_pydantic_model` finds `def run(...) -> Model` with `class Model(BaseModel)` in the same file.

---

## What the CLI has today

- Cobra, all commands in `internal/cmd` as package globals (`internal/cmd/root.go`).
- `internal/config/config.go`: one global `~/.notte/cli/config.json` = `{api_key, api_url}`, plus bare-string state files `current_session`, `current_function`. **No per-directory config anywhere in the repo** — `notte init` would introduce the first.
- `internal/auth/env.go` **already has an environment notion**: `KeyringKeyForEnv(label)` namespaces keyring entries, and `hostToEnvLabel` maps `api.notte.cc`/`us-prod`→prod, `us-staging`→staging, `us-dev*`→dev. That's the hook `--env` hangs off.
- `internal/cmd/functions.go` covers list/create/show/update/delete/fork/run/runs/schedule/secrets. Create/update send a single multipart `file` part. No zip/tar/directory support anywhere in the repo.
- **Unexposed wins already in the generated client:** `response_format` and `restricted` (create/update), `version` (update — server-assigned `v%Y%m%d_%H%M%S`), `decryption_key` (download). The marketplace script had to hand-roll `fetch` + re-derive `sha256("api_key:{k}:workflow_id:{id}:dumb")[:64]` because `--decryption-key` doesn't exist.
- No `//go:embed` in the repo. Scaffolding would be the first.

Two things to verify rather than assume:
- `marketplace-catalog.ts` carries a `detectCliError()` workaround because *"the CLI reports API errors as a JSON body on stdout with exit 0"*. `internal/output/json.go` looks like it writes errors to stderr and `Execute()` exits 1, so this may be path-specific — reproduce before designing around it.
- The README documents `notte functions schedule --cron "0 9 * * *"` — five fields. `validate_and_format_cron` wants the six-field AWS EventBridge form (`cron(m h dom mon dow year)`). Either the README is wrong or the server is lenient; worth checking, and worth validating client-side either way.

---

## Prior art, and what each one gets right

| | Project marker | Config | Per-unit dir | Shared code | Envs |
|---|---|---|---|---|---|
| **Supabase** | `supabase/` + `config.toml` | TOML, `[functions.<name>]` | `functions/<name>/index.ts` | `_shared/` (underscore = not a function) | `supabase link --project-ref`, `[remotes]` |
| **Vercel** | `.vercel/project.json` (gitignored) | `vercel.json` | `api/<name>.py` — file *or* dir | bundler resolves it | preview vs production; `vercel promote <url>` |
| **Cloudflare** | `wrangler.toml` | TOML | one worker per config | esbuild | `[env.staging]` + `--env staging` |
| **dbt** | `dbt_project.yml` | YAML | `models/**.sql` | `macros/` | `profiles.yml` targets + `--target prod` |
| **Modal** | none | Python | `-m src.app` module mode | `Image.add_local_python_source` (explicit since 1.0) | — |
| **Val Town** | `.vt/` | — | one val | — | `vt clone` / `push` / `watch` |

Worth stealing:

1. **Underscore = not a unit** (Supabase `_shared`). Zero config, instantly legible.
2. **File *or* directory** (Vercel). `functions/quick.py` for a one-liner; promote to `functions/quick/` when it grows helpers.
3. **`--env` + a config block per env** (wrangler, dbt). Deletes both Makefile hacks outright.
4. **`promote` moves the *artifact*, not the source** (Vercel). Byte-identical staging→prod.
5. **Explicit local sources** (Modal 1.0). Modal *removed* automounting because it was unpredictable. Ours is explicit by construction — only what's reachable from `main.py` via relative import gets bundled.
6. **Secret values via a gitignored `.env`, never in the config** (Supabase). `config.toml` is safe to commit; `supabase secrets set` pushes from `.env`.

And four ideas from your own frameworks that beat anything in that table:

7. **The manifest is a lockfile with per-environment state; the path is the identity.**
   ```jsonc
   {"envs":{"prod":{"function_id":"a0b8…","functions_version":"v20260821_162138","code_sha256":"ea2e…"},
             "dev":{"function_id":"7cc4…","functions_version":"v20260826_081715","code_sha256":"ea2e…"}},
    "path":"1001tracklists.com/fetch_batch_tracklists.py", …}
   ```
   Per-env `code_sha256` is what lets **one tree serve all three environments** — a tree-wide hash would mean pushing to prod marks dev as up to date. No env-scoped id ever appears in a filename.
8. **The write and the bookkeeping are separate failure domains.** `entryAfterPush` advances `code_sha256` to the pushed bytes even when the confirmation read-back fails; only version strings go stale. Tying the hash to the read-back meant *"a transient error on the second request minted a duplicate upstream version on the next run."*
9. **A run is authoritative only for what it inspected.** `--limit 20` must never orphan the other 2,029. A *complete* walk that no longer lists something is the only thing that may remove it. And **never delete remote things absent from the tree** — report them (`reportExtraRemote`).
10. **Preview → guard → apply** (managed-auth): dry-run returns `target_state_sha256`; the apply sends it back as `expected_target_state_sha256`. Optimistic concurrency for free.

---

## Proposed layout

```
my-functions/
├── notte.toml                     # committed. intent: envs, defaults, per-function config
├── notte.lock.json                # committed, machine-written. per-env ids + hashes
├── .env.dev / .env.prod           # GITIGNORED. secret values only
├── .notte/                        # gitignored. build output, caches
│   └── build/prod/amazon_search.py
├── pyrightconfig.json             # written by `notte init`
├── AGENTS.md                      # written by `notte init` — the authoring contract
└── functions/                     # ← real python package, importable from repo root
    ├── __init__.py
    ├── _shared/                   # `_` prefix = library, never deployed
    │   ├── __init__.py
    │   ├── http.py
    │   └── contract.py
    ├── amazon_search/             # a function
    │   ├── __init__.py
    │   ├── main.py                # entrypoint: must define exactly one `run()`
    │   ├── parse.py               # local helper, bundled in
    │   └── test_main.py           # colocated test, never bundled
    └── quick_check.py             # single-file function — no directory needed
```

`functions/amazon_search/main.py`:
```python
from pydantic import BaseModel
from .parse import parse_rows              # sibling
from .._shared.http import fetch_json      # shared

class Response(BaseModel):
    items: list[dict]

def run(query: str = "laptop") -> Response:
    return Response(items=parse_rows(fetch_json(query)))
```

**Why `functions/` at the repo root and not `notte/functions/`.** `notte` is a real PyPI package and is in `ALLOWED_IMPORTS`. A top-level `notte/` directory in a repo whose root lands on `sys.path` — which happens the moment you run pytest — shadows it, and pyright resolves your empty directory instead of the SDK. `functions/` has no such collision. Make it configurable (`[project] functions_dir = "functions"`) so you *can* have `notte/functions/`, and document the caveat there rather than defaulting into it.

Everything is a real package (`__init__.py` generated by `notte new`), so relative imports resolve identically for pyright, pytest, and the bundler. Discovery rule, one sentence: **anything directly under `functions_dir` whose name doesn't start with `_` is a function — a `<name>/main.py` or a `<name>.py`.**

---

## Config file format

Two files, two owners, two formats. That split matters more than which format wins.

### Which format for the hand-written config

| | Comments | Editor schema/autocomplete | Go parsing | Deep nesting | Ecosystem |
|---|---|---|---|---|---|
| **JSON** | ✗ — fatal for a config you want to annotate | ✓✓ best in class: `$schema`, works in every editor with zero setup | stdlib | fine | JS/TS (`vercel.json`, `tsconfig.json`) |
| **JSONC / JSON5** | ✓ | partial | third-party | fine | VS Code only; no cross-editor story |
| **YAML** | ✓ | ✓ via `# yaml-language-server: $schema=` | third-party | ✓✓ | k8s, CI |
| **TOML** | ✓ | ✓ via taplo's `#:schema` directive + Even Better TOML | third-party | ✗ awkward past 2 levels | Python & Rust (`pyproject.toml`, `Cargo.toml`, `wrangler.toml`, `fly.toml`, `netlify.toml`) |

**Recommendation: TOML, as `notte.toml`.** Three reasons, in order:

1. **The users are Python developers.** They read `pyproject.toml` every day. A functions CLI whose config looks like `pyproject.toml` needs no explanation.
2. **Comments are the point.** Both existing frameworks are ~40% prose comments explaining *why* — `deploy.py`'s `NOTTE_API_URLS` carries a war story about `NOTTE_API_URL` silently pointing `pull prod` at dev. JSON cannot hold that, and it's exactly what belongs next to `[env.prod] api_url = …`.
3. **The nesting here is shallow by construction** — `[env.prod]`, `[functions.amazon_search]`. That's TOML's sweet spot, and it's the one place TOML is weak, so the weakness never bites.

YAML is rejected on safety: whitespace significance, and the Norway problem (`no` → `false`) in a config whose values include country codes — `proxy_country = "no"` is a real value in the managed-auth template schema.

Bare `.notte` is rejected outright: no extension means no highlighting, no schema association, no declared format until you open it. But `.notte/` as a **directory** for gitignored local state is right, and mirrors `.vercel/`, `.vt/`, `.terraform/`.

Also rejected: `[tool.notte]` inside `pyproject.toml` (most Python-native, but the repo isn't a distributable package and it couples project config to a file uv/pip also own); `.notte.toml` (hidden files are for user-level config — you want this discoverable in `ls`); `notte.config.toml` (the JS `*.config.*` convention disambiguates; there's nothing to disambiguate from).

### Ship a JSON Schema regardless

TOML's schema story is real but needs a directive on line 1:
```toml
#:schema https://notte.cc/schema/notte-v1.json
```
`notte init` writes it; `notte schema` prints it for vendoring. Validate against the same schema in `notte check`, so a typo'd key errors instead of doing nothing — TOML's failure mode for an unknown key is silence.

### The rule that follows from Go's TOML libraries

Every Go TOML library either drops comments on write or exposes them read-only; `pelletier/go-toml` v2 explicitly **dropped document editing from its requirements**. Comment-preserving edits need a niche lossless-AST library (`creachadair/tomledit`, `smm-h/go-toml-edit`).

Make it a rule instead of a dependency: **`notte.toml` is never machine-rewritten.** `notte init` and `notte new` *render templates* (text, not a marshal round-trip); after that only humans edit it. Everything the CLI writes back goes to `notte.lock.json` — JSON, stdlib-parsed, machine-owned. That's precisely the line `marketplace/manifest.json` failed to draw: it mixed generated state (`pulled_at`, `run_count`) with copy (`name`, `description`, `categories`) that a human wants to own, and the result is a 2.2 MB file every sync dirties.

### One interpolation syntax

Config must reference things it may not contain. Supabase solved this with `env("MY_KEY")`. Use a uniform `${namespace:key}` so there's one rule:

```toml
[env.prod]
api_url = "https://api.notte.cc"
api_key = "${env:NOTTE_API_KEY_PROD}"      # resolved at use, never stored

[env.preview]
extends = "dev"
headers = { "x-db-preview" = "${git:branch}" }
```
Namespaces: `env:` (process environment, optionally from a gitignored `.env.<env>`), `git:` (`branch`, `sha`, `short_sha`), `keyring:` (the existing `KeyringKeyForEnv` entries). **Fail loudly on an unresolved reference** — `deploy.py`'s `--db-preview` guard exists because a header silently ignored by staging meant *"a silent wrong write"*, and unresolved-to-empty-string is the same bug class.

---

## Naming conventions

| Thing | Choice | Why not the alternative |
|---|---|---|
| Project config | `notte.toml` | See above. |
| Lockfile | `notte.lock.json` | One entry per line — marketplace proved this keeps diffs reviewable at 2,049 entries. Separate from `notte.toml` because it's machine-written. |
| Local state | `.notte/` (gitignored) | Mirrors `.vercel/`. Build output lives here, never next to sources. |
| Secret values | `.env.<env>` (gitignored) | Supabase's split. Never in `notte.toml`. |
| Package root | `functions/`, configurable | See layout section. |
| Entrypoint | `main.py` | Not `function.py` (redundant inside `functions/<name>/`), not `index.py` (a JS import), not `route.py` — a Notte function has exactly one `run()`; there is no route/handler split to encode. |
| Shared code | `_`-prefixed anything; `_shared/` by convention | Supabase's rule, and it doubles as the "not a function" marker. |
| Function name | directory or file stem, `[a-z0-9_-]+` | Never contains the function id — ids are per-env. |
| Build output | `.notte/build/<env>/<name>.py` | Per-env because per-env config can change the bytes. |
| Envs | `dev`, `staging`, `prod`, `preview`, `local` | Both frameworks already use exactly these. |
| Deploy verb | `deploy` | Not `push`. `push` implies a round trip; bundling makes the tree source-of-truth and the flow one-way. Reserve `pull` for adoption/import. |

---

## Command surface

```
notte init [dir]                       # scaffold notte.toml, functions/, pyrightconfig, .gitignore, AGENTS.md
notte init --from-session <ses_id>     # bootstrap from `sessions workflow-code` — record, then scaffold
notte new <name>                       # one function directory from a template

notte pull   [--env E]                 # adopt existing remote functions into the tree + lock
notte check  [<target>] [--env E]      # build + validate + diff vs remote. writes NOTHING. the CI gate.
notte deploy [<target>] [--env E]      # build → diff → confirm → create/update → schedule → write lock
notte status [--env E]                 # what's drifted, and what a `_shared` edit would touch
```

Six commands, not eighteen. `<target>` is a name, a glob, `all`, or a path — so `notte deploy functions/amazon_search` tab-completes.

### Why `pull` is v1 and not a migration nicety

It is tempting to file `pull` under "only needed to adopt an existing tree." That is wrong twice over.

**An org with existing functions is the normal case, not the migration case.** Functions already arrive from `sessions workflow-code`, from the Anything API build agent, and from the console. The expected Notte flow is *author in the browser, then decide you want it in git* — which is `pull`, and which is also why `notte init --from-session` is really just a single-function `pull` wearing a different name. The machinery is required either way.

**Without it, `deploy` is unsafe in a non-empty org.** Create-vs-update is decided from the lock: no `function_id` for this env → create. A fresh `notte init` against an org that already has `amazon_search` produces a lock that believes nothing exists, so the first `deploy` **creates a second `amazon_search`** rather than updating the first. `functions.name` has no unique constraint (`text NOT NULL DEFAULT 'default'`), so the API accepts it silently, and now two functions share a name while callers hold the id of the one that stopped being updated. That is the worst failure mode in this document, and `pull` is what prevents it.

Which implies a companion rule: **`deploy` must refuse to create a function whose name already exists remotely but is absent from the lock**, and say `run 'notte pull --env prod' first`. Fail closed, same as the credential rule below. `--force-create` exists for the genuine case where you do want a second one.

**`pull` is not the inverse of `deploy`, and must not pretend to be.** What comes back over the wire is the *artifact*, and a bundled artifact cannot be un-flattened into the package that produced it. So:

- A function with no entry in the lock is written as a **single-file function** — `functions/<name>.py` — because that is genuinely what it is. It can be promoted to a directory with helpers later, by hand, at which point it starts bundling.
- A function already in the lock and already deployed *from this tree* is **left alone**. Overwriting `functions/amazon_search/{main,parse}.py` with one flattened file would destroy the sources to "sync" them, which is the opposite of the intent. If its `artifact_sha256` doesn't match what's deployed, that's drift — report it, and let `status`/`deploy` handle it.
- Following marketplace: a run is authoritative only for what it inspected, so `--limit` or a failed download never prunes; and remote functions absent from the tree are **reported, never deleted**.

**There is no bulk download, and no download command at all.** This is the part that makes `pull` a real batch job rather than one request, and it needs saying because the shape isn't obvious:

- `GET /functions` returns `PaginatedResponseFunctionResponse{Items []FunctionResponse}`, and `FunctionResponse` **has no URL field**. Only `FunctionWithLinkResponse` carries `Url`, and that comes from `GET /functions/{id}`.
- So the code for each function costs **two more requests**: one to get the signed URL, one to fetch it. A full pull is `⌈N/100⌉ + 2N` — roughly 4,120 requests for marketplace's 2,049 functions. It ran that at concurrency 48.
- The URL is a Fernet token for Notte-managed functions, decrypted with a key derived client-side: `sha256(f"api_key:{key}:workflow_id:{id}:dumb")[:64]`, passed as `?decryption_key=`.
- **`notte functions download` does not exist.** `functions show` already calls `FunctionDownloadUrl`, prints the metadata, and throws the URL away. So today this is entirely hand-rolled — marketplace reimplements the key derivation and the fetch, which is how one secret-derivation rule ended up living in two repos.

**This needs no backend change — parallelise it.** An earlier draft asked for `url` on the list endpoint to halve the request count. That ask is unnecessary, and marketplace is the evidence: it runs the full walk at `concurrency: 48` (`marketplace-catalog.ts:2103`, overridable with `--concurrency`) over the largest corpus that exists, and the comment on its retry classifier records the measurement:

> *"Nothing hit a 429 while this was being measured, but a full pull is several times larger than any sample taken, and a retry is much cheaper than a half-written tree."*

4,120 requests, no rate limiting. The retry logic is defensive rather than a response to observed throttling. For a realistic project — tens of functions, not thousands — `1 + 2N` at concurrency 48 is a second or two. Adding a backend dependency to optimise that would be trading real coordination cost for an imperceptible win.

So what's actually needed is client-side, and all of it is CLI work:

- **`notte functions download --function-id <id>`** as a primitive in its own right, deriving the decryption key internally rather than exposing a flag users must understand. `pull` becomes a bounded-concurrency loop over it, and marketplace's hand-rolled fetch and duplicated key derivation both get deleted.
- Retry with backoff on 429/5xx honouring `Retry-After` — cheap, and the failure it prevents is a half-written tree.
- A **complete** page walk before anything is reported as a remote extra. A listing that stops early is indistinguishable from one where functions were deleted.

**And `check` shouldn't pay this cost at all in the common case.** The lock already stores `artifact_sha256` per env, so the default gate — *"you changed sources and didn't deploy"* — is a local build plus a hash comparison, with no downloads and no page walk. `--verify-remote` does the full download walk to catch out-of-band edits made in the console. marketplace always downloads in `check` because it is a mirror with no separate source hash to trust; we have one, so we can be cheap by default and thorough on request. That also matches its own observation that a scheduled `check` against prod is a staleness alarm rather than a build gate.

### Deferred, and why

An earlier draft proposed eighteen commands. Roughly half were aspirational or gated on backend work that doesn't exist yet, and a large surface is its own cost — it has to be documented, tab-completed, kept coherent, and lived with. Everything below is deliberately *not* in v1:

| Command | Why deferred |
|---|---|
| `run`, `logs`, `secrets`, `schedule` | already exist under `notte functions`; project-aware twins aren't needed on day one |
| `build` | folded into `deploy` and `check`. Expose separately only once someone actually wants the artifact without the diff |
| `promote`, `rollback` | need `--version` and `versions[]` exposed on the CLI first |
| `dev` | genuinely valuable, but it's a second execution model and deserves its own design rather than a line in this table |
| `whoami`, `auth login --env` | blocked on `GET /me` (backend ask #3). These ship *with* that endpoint — `auth login --env` is a prerequisite for the fail-closed credential rule below, not an optional extra |

The credential resolution rules in the next section apply to all six v1 commands regardless; `--env` is not deferred.

`notte.toml`:
```toml
#:schema https://notte.cc/schema/notte-v1.json

[project]
name = "anything-api"
functions_dir = "functions"

[env.dev]
api_url = "https://us-dev.notte.cc"
api_key = "${env:NOTTE_API_KEY_DEV}"

[env.staging]
api_url = "https://us-staging.notte.cc"
api_key = "${env:NOTTE_API_KEY_STAGING}"

[env.prod]
api_url = "https://api.notte.cc"
api_key = "${env:NOTTE_API_KEY_PROD}"
confirm = true                                   # never deploy here without an explicit yes

[env.preview]
extends = "dev"
headers = { "x-db-preview" = "${git:branch}" }   # generalizes managed-auth's preview mode

[functions.amazon_search]
name        = "Amazon AE product search"
description = "…"
shared      = true
cron        = "cron(0 9 * * ? *)"
secrets     = ["AMAZON_PARTNER_TAG"]             # in addition to what the AST scan finds
```

### Credentials resolve *from* the environment, never beside it

API keys are **not** literals in `notte.toml`. More importantly, **the key and the URL must be resolved as one unit.** Selecting an env fixes `api_url`, and every candidate key is then derived from that same `api_url` — there is no step in the chain that can hand back a credential belonging to a different endpoint:

1. `--api-key` (explicit, and the operator owns the consequences)
2. the `api_key = "${env:…}"` reference declared **inside that env's own block**
3. the keyring entry under `KeyringKeyForEnv(hostToEnvLabel(api_url))` — the label computed from the resolved URL, not from ambient state
4. **stop.** Fail with `no credential for env 'staging' (https://us-staging.notte.cc) — set NOTTE_API_KEY_STAGING or run 'notte auth login --env staging'`.

The two fallbacks the global CLI uses today — a bare `NOTTE_API_KEY` and `~/.notte/cli/config.json` — are **deliberately not in this chain**, because neither is tied to an endpoint. A developer with `NOTTE_API_KEY` exported for prod running `notte deploy --env staging` would otherwise authenticate to staging with a prod key: it fails closed if the orgs differ, but it succeeds and writes to the *wrong org* whenever they don't. `notte deploy` is the command where that matters most.

This is the same bug `marketplace-catalog.ts` already documents having hit from the other direction — its `NOTTE_API_URLS` table exists precisely because reusing a helper that read ambient `NOTTE_API_URL` made `pull prod` silently read dev. Its `createNotteRunner` then passes `NOTTE_API_KEY` and `NOTTE_API_URL` to the subprocess explicitly, together, never ambient. Same rule, enforced one level up.

`notte auth login --env <name>` and `notte whoami --env <name>` are the paired ergonomics that make failing closed tolerable, and `notte status` should print the resolved org for each configured env so a misconfiguration is visible before a deploy rather than after.

---

## The bundler

### Algorithm

1. Parse `main.py`; collect relative imports (`from .x import a`, `from ..y.z import b`).
2. Resolve to files inside `functions_dir`; recurse. Anything outside the package, or non-relative, is left alone and checked against the allowlist.
3. Topologically sort. Cycle → error naming the cycle.
4. Emit: header, then `from __future__ import annotations` if any module had it (exactly one, first statement), then hoisted+deduped third-party imports, then each dependency's body in topological order with its relative-import lines **replaced** (see aliases below), then `main.py`'s body last.
5. Hash the artifact. Write `.notte/build/<env>/<name>.py` + a source map.

**Aliased relative imports keep their binding.** A relative import line is not simply deleted — it is replaced in place by an assignment per aliased name:

```python
from .parse import parse_rows as pr, clean          # source
pr = parse_rows                                     # artifact (clean needs nothing)
```

Deleting the line outright would drop `pr` and the artifact would die with `NameError` at run time, which is the worst possible failure mode: it passes the bundler, passes upload validation, and fails in production. Unaliased names need no assignment because the flattened definition already carries that name, and dependency bodies are emitted before the body that imports them, so the right-hand side is always bound by the time the assignment runs.

**Collisions are an error, not a rename.** If `_shared/http.py` and `parse.py` both define `clean`, fail with `_shared/http.py:12 and parse.py:8 both define 'clean' — rename one`. This is the pivotal simplification: **no reference rewriting is ever needed**, so no full-fidelity Python parser is needed, and the artifact stays byte-readable — which matters because that artifact is what the console shows and what tracebacks point at.

The collision set is every top-level binding **plus every alias introduced above** — `from .parse import clean as fetch` collides with a `fetch` defined in `_shared/http.py` exactly as a second `def fetch` would, and must be reported the same way.

Rejected in v1, each with a fix-it message:
- `from . import mod` then `mod.f()` → *"use `from .mod import f`"*. Neither existing codebase does this.
- `from .x import *` → *"star imports can't be flattened"*.
- Import cycles.
- Relative imports inside a function body or `if TYPE_CHECKING`.

### Where it runs

| | Go-native | Shell out to Python/`uv` | Server-side |
|---|---|---|---|
| Parse fidelity | Purpose-built tokenizer. Sufficient **because collisions error out**. `go-python/gpython` is a Python 3.4 grammar — no f-strings, no walrus, no `match` — so it isn't an option. | Perfect: Python's own `ast`. | Perfect. |
| Runtime deps | None. Brew binary works in CI, in a bare container, everywhere. | Needs `python3`/`uv` present. | None. |
| local build offline | Yes | Yes | **No** — you lose local preview and the CI gate |
| Agreement with `ScriptValidator` | Must mirror the allowlist as data (drifts) | Same problem — the validator lives in `notte-api`, not in a pip package | Authoritative by construction |
| Cost | ~600 lines Go + tests | ~200 lines Python, `//go:embed`-ed, run via `uv run --script` | Backend work: accept a tar, bundle, validate |
| Failure mode | A weird import form is rejected with a clear message | "python3: not found" on a machine where the CLI otherwise works | Slow loop; can't check in a PR without credentials |

**Recommendation: Go-native.** A brew-installed Go binary that silently requires Python is the worse DX, and the collisions-error-out design shrinks the problem to import discovery + topological sort + top-level binding extraction — all line-oriented at indent level 0. The parse-fidelity gap only bites on constructs we reject anyway.

Two things make this safe rather than optimistic:
- **Ship the allowlist as data, refreshed from the API** (see backend asks). Then `notte check` fails with `functions/x/main.py:3: import os is not allowed — use 'from notte_sdk.types import os'` instead of after a multipart upload. Copy managed-auth's `--allow-api-behind` + exit-code-2 handling for when the CLI is newer than the API.
- **Ask for `POST /functions?dry_run=true`.** managed-auth's preview→guard→apply depends on the server saying what *it* thinks before you write. Functions has no equivalent, so `notte check` can only be locally authoritative.

### Two hashes, because bundling breaks the round trip

marketplace's core invariant — *"files are byte-identical to what prod serves"* — cannot survive a bundler. Replace it with two hashes in the lock:

- `source_sha256` — over the canonicalized *set* of contributing source files (sorted `path:sha256` pairs). Drives create-vs-update.
- `artifact_sha256` — the bundled bytes. Drives the diff shown before confirming, against what's deployed now.

And now that byte-fidelity is gone, a generated header is free and should be there:
```python
# generated by notte 0.1.0 — do not edit
# sources: functions/amazon_search/{main,parse}.py, functions/_shared/http.py
# source-sha256: 4f2a…
```

---

## Secrets

The API shape (`notte-api/src/notte_api/secrets/endpoints.py`, table `tenant_secrets`):

- `POST /secrets {namespace, name, value}` → 201. Namespaces: `function_env`, `llm_provider`.
- `GET /secrets?namespace=` → metadata only: `{id, namespace, name, key_hint, created_at, last_used_at}`.
- `GET /secrets/{name}?namespace=` → **plaintext value**.
- `DELETE /secrets/{secret_id}` → 204.

Four facts that shape the design:

1. **`(namespace, name)` is UNIQUE** — enforced by partial indexes, scoped org-or-user. That is the best natural key in the whole API and it makes secrets genuinely declarable.
2. **`function_env` names must match `^[A-Z_][A-Z0-9_]*$`**, ≤128, and must not be in `{NOTTE_API_KEY, NOTTE_API_URL, NOTTE_BASE_URL, NOTTE_ENV, ENVIRONMENT, NOTTE_DB_PREVIEW_BRANCH}`. Validate all of that client-side.
3. **`POST` is not an upsert.** It's a bare insert; a duplicate is a **409**. And **delete is by UUID, not name.** So "update a secret" means LIST → map name→id → DELETE → POST. Non-atomic, and there's a window where the secret doesn't exist.
4. **Secrets are org-scoped, not per-function.** Since each env is a different key/org, per-env separation comes free — but it also means the CLI's current placement of these under `notte functions secrets` is misleading. Promote to top-level `notte secrets`.

### The model

**Names in git, values in a gitignored `.env.<env>`** (the same split Supabase uses).

The declared set for an env = union of every deployed function's `required_secrets` (which the server already computes from the AST scan and returns on `FunctionResponse`) **plus** any explicit `[functions.x] secrets = [...]` for names computed at runtime that no scanner can see.

```
$ notte secrets diff --env prod
  missing on prod (declared, not set):
    AMAZON_PARTNER_TAG      required by amazon_search
    STOCKX_TOKEN            required by stockx_search, stockx_bid
  set on prod, not declared:
    OLD_SCRAPER_KEY         (not deleted — pass --prune to remove)
```

- `notte secrets push --env prod` reads `.env.prod`, POSTs what's missing. For a name that exists with a different `key_hint`, it DELETEs then POSTs — and **says so explicitly**, because that sequence is not atomic.
- **Diff on `key_hint`, never on values.** Reading a value back writes an `audit_events` row (`entity_type="tenant_secret"`, `action="read"`) and bumps `last_used_at`. A `diff` that silently audits every secret on every CI run is a bad neighbour.
- **Never prune by default.** Same rule as `reportExtraRemote`: report extras, delete only on `--prune`.
- Never print values, in any output mode.

### Deploy preflight

`preflight_required_secrets` already raises **422** with the missing names at run time, before a session starts (so nothing is charged). Pull that forward: after `notte deploy` writes a function, read back `required_secrets`, diff against the env, and warn — **especially before setting a cron**, since otherwise the first sign of trouble is a 09:00 job failing on a Sunday.

---

## Schedules

`POST /functions/{id}/schedule {cron, variables}` is genuinely good for reconciliation: an upsert, idempotent for the same cron, guarded by a Postgres advisory lock plus optimistic `schedule_revision` CAS, with EventBridge rollback compensation. It validates `variables` keys against the function's declared `variables`.

**The blocker: there is no read endpoint.** The data exists — `functions.schedule_cron`, `schedule_variables`, `schedule_state`, `schedule_paused_reason`, `schedule_paused_at`, `schedule_revision` are all on the row and all on `SupabaseFunctionResponse` — but the API's `FunctionResponse` model drops them, so FastAPI never serialises them, so the generated Go client can't see them. The CLI has `schedule`/`unschedule` and nothing else.

Consequence: `notte status` cannot show the current cron, and `notte deploy` cannot distinguish "already correct" from "about to change". Two options:

- **Preferred:** add the three fields to `FunctionResponse` (`functions/endpoints.py:50-63`) — additive, mirrors exactly how `published` and `required_secrets` were added, and regenerates through OpenAPI → Go client. Then cron reconciles properly.
- **Until then:** record the last-applied cron in `notte.lock.json` and treat the lock as truth, printing a caveat that a change made in the console is invisible to `status`.

Two rules regardless:

- **Validate the cron client-side.** It must be six-field AWS EventBridge form (`cron(m h dom mon dow year)`) or `rate(...)`. A five-field crontab string is the natural thing to write and the docs currently show one.
- **Never fight `schedule_state`.** The system pauses schedules for `credit_exhausted` or `function_inactive`. A reconciler that re-POSTs because state ≠ enabled will thrash against the billing system. Reconcile on `cron` + `variables` only; surface `schedule_paused_reason` in `status` as information.

---

## Managed auth — deferred, and why it will be easy

**Templates are already the declarative design you're describing.** `POST /managed-auth/templates/import?dry_run=true` returns a real field-level diff — `{slug, action: create|update|no_change, previous_revision, revision, metadata_changes[], login_changed, verifier_changed, bundle_sha256, target_state_sha256}` — and the apply is guarded by `expected_target_state_sha256` from the preview. `GET /managed-auth/templates/{slug}/export` round-trips it. `slug` is UNIQUE. It is strictly the best-designed surface in the API, and `connectors/<slug>.json` is already a git-versioned manifest.

Three things to know before adopting it:
- The router is **`include_in_schema=False`** (`main.py:719`), so managed-auth is absent from the OpenAPI spec and therefore absent from the generated Go client entirely. Flip that, or hand-write the client.
- Import is gated to one hard-coded org: `_require_connector_organization()` demands `org_id == "4dbf683a-…"`. Fine for you; a blocker if customers should ever declare their own connectors.
- A **connection** is not a template. Creating one runs a real browser login, spends money, and provisions a vault + a browser profile as side effects; `PATCH` covers only `label` and `schedule`; `credentials`, `vault_id`, `mailbox_id`, `two_fa_method`, `domain` are all create-only and immutable; and most of its fields (`status`, `last_login_at`, `last_failure_code`) are observed runtime state. Reconciling a connection means delete-and-re-login. **Keep connections imperative.** What a CLI can usefully add is operational commands (`list`, `check`, `reauthenticate`, `reset-profile`) and CI fixtures — the `ci-longlived-<slug>` / `ci-smoke-<slug>-<hex>` pattern `smoke.py` already implements, including its hard refusal to delete anything with the long-lived prefix.

Related: vaults, personas, and profiles all have server-generated UUIDs with non-unique names, so none of them are name-addressable. Profiles are the closest — `GET /profiles?name=` supports filtering, which makes find-or-create viable. Vault *credentials* are keyed by `(vault_id, root domain of url)`, which is a real natural key and effectively an upsert. Worth knowing, not worth building yet.

---

## The DX ideas worth building

**Source maps.** Emit `.notte/build/<env>/<name>.map.json` mapping artifact line ranges → `source:line`, and have `notte logs` / `run-metadata` rewrite tracebacks through it. A traceback that says `line 612` in a 900-line concatenated file is the single worst thing about any bundler, and nobody in Python serverless fixes it. Highest-leverage item here.

**Blast radius in `notte status`.** `_shared/contract.py` is inlined into every function that imports it, so editing it changes N artifacts. managed-auth papered over exactly this with `scripts/check_revision_bumps.py` (122 lines) plus a note in four separate docs. The CLI knows the import graph:
```
$ notte status --env prod
  functions/_shared/contract.py changed → 9 functions affected
    ✗ google_login          drifted  (source 4f2a… ≠ deployed 8c31…)
    ✗ bluesky_login         drifted
    …
```
Auto-derived. `check_revision_bumps.py` and the manual `revision` field both disappear.

**`notte promote` moves bytes, not source.** Download the artifact deployed to staging, upload those exact bytes to prod, record both hashes. Guarantees what you tested is what ships — stronger than re-running the build, and it's Vercel's model.

**`notte check` as the CI gate `anything-api` designed and never wired up.** Writes nothing, exits non-zero on drift; `notte init` drops the GitHub Action in. Heed marketplace's warning: on a PR it's a genuine gate; on a schedule against prod it's a *staleness alarm*, since the catalog changes whenever anyone publishes.

**`notte dev <name>`.** Run the entrypoint locally against real cloud sessions, `--var` → `run()` kwargs. The current inner loop is deploy-to-test. This is `supabase functions serve` / `wrangler dev`, and it's where `drive_login.py` and the whole `login/*.py` exploratory-recording corpus want to live.

**Prod guard.** `[env.prod] confirm = true` → interactive confirm plus a banner; non-interactive requires `--yes`. marketplace's `push` already refuses without a TTY, with a message naming all three ways out (`--yes`, `--apply`, `--dry-run`). Keep that wording.

**Expose what's already in the generated client.** `--version` on update and `versions[]` on show → `notte rollback --to v20260821_162138`. `--decryption-key`, or just derive it automatically → deletes the duplicated `sha256("api_key:{k}:workflow_id:{id}:dumb")[:64]` that currently lives in two repos.

**Agent-native scaffolding.** `notte init` writes `AGENTS.md` with the real contract — `run()` returns a `BaseModel`, the import allowlist, `from notte_sdk.types import os`, six-field cron — and registers the `notte-browser` skill. Encore does exactly this (`encore app create` asks which AI tool and writes the rules file). The material exists as `notte-skills/plugins/notte-cli/skills/notte-browser/references/function-management.md`; it needs the project layout added and to stay in sync (it's a submodule).

**`notte init --from-session <ses_id>`.** `sessions workflow-code` already emits a deployable `run()`. Record a workflow in a browser → scaffold a project around it. An onboarding path nothing else in the prior-art table can offer.

**Testing.** Two layers.

*For user projects:* `test_*.py` colocated in the function dir, never bundled, run with pytest; `notte check --test` runs them. managed-auth's `ShippedConnectorsTest` — asserting properties of the real checked-in catalog, e.g. *"every connector still parses once inlined"* — generalizes into `notte check` itself and stops being something each repo hand-writes.

*For the CLI itself:* the bundler is the part where a wrong answer is silent, so it ships with a golden-file suite before it ships at all — `internal/bundle/testdata/<case>/{in/,want.py}`, one directory per case, following `marketplace-catalog.ts`'s convention of naming each test after the invariant it protects. The cases that must exist on day one:

| Case | Asserts |
|---|---|
| `alias-preserved` | `from .parse import f as g` emits `g = f`; the artifact defines `g` |
| `alias-collides` | an alias colliding with another module's top-level name is reported, not silently shadowed |
| `collision-reported` | two modules defining `clean` fail with both file:line locations |
| `topo-order` | a dependency's body precedes every body that imports it |
| `diamond` | a module reached by two paths is emitted exactly once |
| `cycle-rejected` | the error names the cycle |
| `future-annotations` | emitted once, first statement, even when three modules declare it |
| `import-hoist-dedup` | `import requests` in four modules yields one line |
| `star-import`, `from-dot-import`, `import-in-function` | each rejected with its fix-it message |
| `disallowed-import` | `import os` fails locally with the `notte_sdk.types` hint, before any upload |
| `deterministic` | bundling twice byte-identical — `artifact_sha256` is load-bearing for the whole diff model |
| `source-map` | every artifact line maps to a real `source:line` |

`alias-preserved` and `alias-collides` exist because that gap was found in review of this document rather than in a test — which is the argument for the table.

---

## Migration

**`managed-auth`** — the cleaner fit for the bundler. `contract.py` → `functions/_shared/contract.py`; `login/bluesky_login.py` → `functions/bluesky_login/main.py`; `verifier/bluesky.py` → `functions/bluesky_verify/main.py`. `from contract import LoginResult` becomes `from .._shared.contract import LoginResult`, and `inline_contract()` — including its `re.sub` escape-expansion war story — is deleted. `login/email_2fa.py` becomes `functions/_shared/email_2fa.py` and the four hand-copied `_verification_code()` loops collapse into one import. `revision` and `check_revision_bumps.py` are replaced by `source_sha256`. What doesn't map in v1: the connector concept (one slug = a login + verifier deployed transactionally) and `/managed-auth/templates/import`. Keep a thin `deploy.py` for the template bundle, let `notte` own the two functions, and revisit when managed-auth joins the project model.

**`marketplace`** — 2,049 files, zero relative imports, so bundling is a no-op for every one of them. The value is deleting `marketplace-catalog.ts`: the `MAKECMDGOALS` filtering, `createNotteRunner` + `detectCliError`, `redact()`, the org preflight, the decryption-key derivation, the `pool`/`retry` helpers — all CLI-native. `manifest.json` maps almost field-for-field onto `notte.lock.json`; `envs[env].{function_id, functions_version, versions, code_sha256}` is already the right shape. The gap it exposes: **`name`, `description`, `categories` are only editable upstream** — `push` only ever sends `--file`. `[functions.<name>]` should own them and `notte deploy` should push them, which is a real capability gain over what exists.

---

## Backend asks (ordered by how much they unblock)

1. **Add `schedule_cron` / `schedule_variables` / `schedule_state` to `FunctionResponse`** (`functions/endpoints.py:50-63`). ~2 lines, additive, exactly how `published` and `required_secrets` were added. Without it, cron cannot be reconciled — only blindly re-applied.
2. **Make secrets updatable by name**: `PUT /secrets/{namespace}/{name}` as an upsert, and `DELETE` by `(namespace, name)`. Today a secret rotation is LIST → DELETE-by-uuid → POST, which is three calls and a window where the secret doesn't exist.
3. **`GET /me`** → `{user_id, org_id, org_name, org_role, plan_type}`. A `notte.toml` committed to git gets applied by different keys; without this, "am I about to deploy to the right org?" is unanswerable. It also deletes marketplace's hack of reading the org id out of the first path segment of a signed download URL.
4. **`POST /functions?dry_run=true`** returning a managed-auth-style diff (`action`, `previous_version`, `changed`, `target_state_sha256`). Enables preview→guard→apply for functions and makes `notte check` server-authoritative.
5. **`GET /functions/capabilities`** exporting **both** import allowlists — the upload one (`notte_api.ast.ALLOWED_IMPORTS`) and the runtime one (`_LAMBDA_ALLOWED_IMPORTS` + third-party, minus `tempfile`) — plus `FORBIDDEN_CALLS` and the forbidden-node list, so `notte check` fails locally with the same rules the server enforces instead of vendoring a copy that drifts. The two lists differing is precisely why this should be served rather than copied: `worker.py`'s own comment notes that a name on only one of them *"either rejects code that would have run or accepts code that then fails inside the sandbox."*
6. *(later, for managed auth)* Flip `include_in_schema` on the managed-auth router so the Go client can be generated, and decide whether template import stays gated to the connectors org.

---

## Open questions

1. **Should `safe_import` remain the runtime import guard?** Execution already runs with `restricted=False`, so the RestrictedPython AST policy is off and `safe_import` is the only thing standing between a deployed function and arbitrary imports. It is also, incidentally, what makes off-the-shelf bundlers unusable. Relaxing it would make them work — but that trade should be evaluated on its own merits, not taken as a side effect of a bundling convenience, and this RFC does not need it either way.
2. **`notte functions` vs the project commands.** The current commands are function-id-centric with global `~/.notte/cli/current_function` state; the new ones are project-centric. Proposal: `GetCurrentFunctionID()` gains a fourth source — the project lock, resolved from cwd — ahead of the global state file, so both surfaces stay coherent.
3. **Is `preview` (dev + `x-db-preview: <branch>`) Notte-wide or managed-auth-specific?** Modeled above as generic `[env.*] headers`, which may be over-general.
4. **Function grouping.** managed-auth needs "these two deploy together, transactionally." Does a `[bundle]` concept belong in the model now, or is it deferred with managed auth?
