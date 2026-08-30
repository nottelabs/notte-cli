# RFC 0002 — Managed-auth connectors in `notte stack`

| | |
|---|---|
| **Status** | Draft — design only, nothing implemented |
| **Date** | 2026-08-30 |
| **Depends on** | [RFC 0001](0001-notte-project-scaffolding-and-deploy.md) |

---

## Context

`apps/back/managed-auth` deploys nine connectors with **1,502 lines of bespoke machinery**:

| | Lines | What it does |
|---|---|---|
| `scripts/deploy.py` | 996 | bundling, contract checks, env resolution, target selection, the template import |
| `contract.py` | 271 | the shared runtime contract, spliced into every source by regex |
| `Makefile` | 113 | positional-goal parsing with a `%::` catch-all so URLs containing `:` survive |
| `scripts/check_revision_bumps.py` | 122 | enforces that a changed connector had its `revision` incremented |

RFC 0001 already absorbs most of that for plain functions. What it does not cover is the thing managed-auth actually is: **a connector is two functions plus metadata, deployed together**. So `deploy.py` survives today for one reason — `POST /managed-auth/templates/import` — while carrying a thousand lines that the CLI now duplicates.

Two goals, and the second is the more interesting one:

1. Delete the machinery. A connector should be a directory, not a manifest plus a regex.
2. **Let customers ship their own authenticated connectors**, not just Notte internally. Today `_require_connector_organization()` hard-codes a single org UUID, so the capability exists but only we can reach it.

---

## The shape

The entrypoint filename declares the kind. That is the whole rule.

```
functions/
  _shared/
    contract.py            LoginResult, VerifierResult, classify_login_failure
    email_2fa.py           the mailbox polling loop, written once
                           — both become an importable package later; see below
  amazon_search/
    main.py                → a function
  bluesky/
    login.py               → a connector: run(session_id) -> LoginResult
    verifier.py            →             run(session_id) -> VerifierResult
    helpers.py             connector-local, bundled into both roles
```

**Discovery gains one sentence, and it is depth-independent.** *Any* directory under `functions/` containing `main.py` is a function; any containing `login.py` and `verifier.py` is a connector. Neither, or only one of the pair, is an error naming both options — the same treatment a directory without `main.py` already gets.

Depth-independence is what lets grouping be a convention rather than a rule. `functions/bluesky/` and `functions/auth/bluesky/` both work, so a project with nine connectors can stay flat and one with fifty can group, without the CLI reserving the name `auth` or anyone migrating. Verified that the bundler already resolves three- and four-dot relative imports, so the deeper layout needs no change to it.

The slug is the directory name, never the path: `functions/auth/bluesky/` deploys as `bluesky`. The catalog slug is globally unique, so a grouping directory must not leak into it.

**This costs nothing in the bundler.** Each role is an ordinary entrypoint, and the existing flattener already handles two of them in one directory sharing local helpers. Verified:

```
bluesky/login.py    -> [bluesky/helpers.py  _shared/contract.py  bluesky/login.py]
bluesky/verifier.py -> [bluesky/helpers.py  _shared/contract.py  bluesky/verifier.py]
```

Relative imports stay two dots, exactly as in a function: `from .._shared.contract import LoginResult`.

### On grouping directories

An earlier draft argued against `functions/auth/<slug>/` on the grounds that it pushes shared imports to three dots. That objection was too strong: the extra dot is cosmetic, and it was outweighed by something the flat rule genuinely costs — **kind is invisible in a listing**. `ls functions/` showing `amazon_search/ bluesky/ google/ hn_scraper/` tells you nothing about which are connectors. At nine connectors that is fine; at fifty mixed with twenty functions it is not.

Sibling top-level directories (`functions/` beside `auth/`) were also considered and are the weaker option. The bundler roots at `functions_dir`, so siblings force the root up to the repository, putting `notte.toml`, `.notte/` and `pyrightconfig.json` inside the Python package root. They also split shared code: `contract.py` would have to live in one tree and be reached from the other as `from ..functions._shared.http import`, which is worse than the depth it was avoiding.

Depth-independence makes the choice the project's rather than the CLI's.

---

## The shared contract should be a package, not files in the tree

`contract.py` and `email_2fa.py` are not really *the user's* code. They are Notte's runtime contract — `LoginResult`, `VerifierResult`, the ~55 ordered failure phrases in `classify_login_failure`, and the mailbox polling loop. Copying them into every project is how the current design ends up with seven hand-copied 2FA loops and a regex that splices 271 lines into every artifact.

The alternative is an importable package the runner ships, so a connector reads:

```python
from notte_managed_auth import LoginResult, classify_login_failure
from notte_managed_auth.email import read_verification_code
```

**This works, and the part that looked like it would block it does not.** Three facts, checked rather than assumed:

| | |
|---|---|
| The server's contract check | only `parsed.variables == ["session_id"]` (`managed_auth/service.py:1396`). Nothing about the return type |
| `response_format` | connectors never send it, so `check_run_returns_pydantic_model`'s "declared in the same file" rule never applies to them |
| managed-auth's own check | a string comparison that the annotation reads literally `LoginResult` (`deploy.py:379`) — which an imported name satisfies |

So `contract.py` does not need flattening at all. It becomes an ordinary allowlisted import that survives into the artifact as an import, exactly like `pydantic`.

What that changes:

- **The 271-line splice disappears**, and with it the coupling where editing one shared file changes all nine bundle hashes.
- **The seven hand-copied 2FA loops disappear** without needing `_shared` to hold them.
- **`_shared/` gets much lighter**, which incidentally weakens the last argument against grouping directories — there is less left to reach across a dot.
- **Connectors become nearly single-file**, which is what they looked like before the contract was forced inline.

The requirement is that the package ships in the runner image and appears in its allowlist. That is now *checkable* rather than assumed: `GET /functions/health` lists what the runner has, and today it has no auth package — `google, gspread, httpcloak, httpx, litellm, loguru, notte, notte_agent, notte_browser, notte_core, notte_sdk, playwright, pydantic, requests, typing_extensions`. Adding one would show up there, versioned, with its install source.

### Timing

**Not yet.** The library is worth extracting once the shape has stopped moving, and the way to find that out is to keep building connectors against it internally. `classify_login_failure`'s phrase list in particular is still growing, annotated `(observed)` as each one is read off a live site — a published package would freeze an interface that is still learning.

The natural sequence: keep `_shared/contract.py` in-tree while connectors are built, watch which parts stop changing, and extract when the churn stops. A month or two of real use is a reasonable read. Documenting it as a library is what makes third-party connectors possible at all, so it is a prerequisite for the customer-facing question below, not a parallel track.

---

## Metadata

`connectors/<slug>.json` disappears into `notte.toml`:

```toml
[connectors.bluesky]
name            = "Bluesky"
domain          = "bsky.app"
category        = "Social"
color           = "#0085ff"
description     = "Decentralized social"
method          = "Email & password"
allowed_domains = []
supports_totp   = false
proxy_country   = "us"

login    = { name = "Bluesky managed login",  description = "Signs in with the connection's username and password." }
verifier = { name = "Bluesky login verifier", description = "Read-only check for the authenticated settings link." }
```

Three fields from today's manifest are deliberately absent:

- **`slug`** — it is the directory name. The old format required them to match and enforced it in code.
- **`login.path` / `verifier.path`** — the layout says where they are.
- **`revision`** — see below.

---

## Revisions become derived

Today `revision` is a hand-maintained integer, and `check_revision_bumps.py` (122 lines) exists to catch the case where someone edits a connector and forgets to increment it. It also encodes a tax the current design cannot avoid: because `contract.py` is spliced into every bundle, **editing one shared file means editing all nine manifests**, and the script special-cases exactly that.

The lockfile already stores `source_sha256` per environment. So:

> **The revision is the count of times the bundle hash has changed**, tracked in `notte.lock.json` and incremented by `deploy` when it moves.

That deletes `check_revision_bumps.py` outright, removes the shared-file tax, and keeps the server contract unchanged — it still receives a monotonically increasing integer, and still refuses a stale or same-revision-different-content import.

A `--revision` override stays for the rare case of adopting an existing connector whose upstream revision is already ahead.

---

## Deploy

`deploy.py`'s import flow is the good part of it and should move into the CLI rather than be discarded:

1. Build both roles, plus the template block from `notte.toml`.
2. `POST /managed-auth/templates/import?dry_run=true` — returns `action`, `metadata_changes[]`, `login_changed`, `verifier_changed`, and `target_state_sha256`.
3. Show that diff and confirm, exactly as `notte stack deploy` already does for functions.
4. Apply with `expected_target_state_sha256` from the preview — optimistic concurrency, unchanged.

Two properties worth preserving explicitly, because both were learned the hard way:

- **The pair is transactional.** A connector whose login updates and whose verifier fails is worse than one that did not deploy, and the server already flips both inside one transaction. The CLI must not split them.
- **Function ids are never rotated on update.** Run rows and console links point at them.

`notte stack deploy` grows no new flags: a connector is just another unit in the plan, printed as one line with its two roles.

---

## The per-role contract

A function's contract is `run()` returning a `BaseModel` declared in the same file. A connector role is stricter, and the server enforces it:

| | Requirement |
|---|---|
| Parameters | exactly `session_id`, no more, no `*args`/`**kwargs` |
| Return | literally `LoginResult` for login, `VerifierResult` for verifier |
| Body | no module-level `run()` call — that is the shape of an exploratory recording |

`ScriptValidator.parse_script` already rejects a script whose module-level variables are not exactly `["session_id"]`. The return-type check is `managed-auth`'s own, in `check_contract()`, and it is an AST check the CLI can run against the artifact in the same pass as everything else.

**This is the point at which flattening earns itself here.** The annotation must be literally `LoginResult`, and after inlining `contract.py` that class *is* in the same file — which is exactly what the API's "declared in the same file" rule wants, and what the regex splice was faking.

---

## What this deletes

| | Today | After |
|---|---|---|
| `scripts/deploy.py` | 996 lines | the template-import call only, or nothing if it moves into the CLI |
| `scripts/check_revision_bumps.py` | 122 lines | gone — revisions are derived |
| `Makefile` | 113 lines | gone — `notte stack` has subcommands |
| `connectors/*.json` | 9 files | `[connectors.*]` in `notte.toml` |
| `contract.py` regex splice | `inline_contract()` | an ordinary import |
| the 2FA polling loop | **hand-copied into 7 login files** | imported — from `_shared/` now, from the package later |

That last row is the clearest sign the current model is wrong. `login/email_2fa.py` exists and is imported by exactly two *undeployed* recordings, because a deployed connector is a single file and cannot import it. Seven shipped connectors carry a copy instead.

---

## The customer-facing question

The user-facing goal is that **customers ship their own authenticated connectors**, not just us. The machinery already supports it; the gates do not:

1. **`_require_connector_organization()`** hard-codes one org UUID. It needs a customer-scoped equivalent, and a decision about whether customer connectors are private to their org or can be published.
2. **The managed-auth router is `include_in_schema=False`** (`main.py:719`), so it is absent from the OpenAPI spec and therefore from the generated Go client. Nothing here can be built until that flips or the client is hand-written.
3. **Templates are a global catalog** with a `slug` unique across all orgs. Customer connectors need either namespacing or a separate scope, or the first customer to claim `shopify` takes it.

Worth deciding early, because it changes whether `[connectors.*]` describes a *catalog entry* or a *private connector*, and those want different metadata.

---

## Open questions

1. **Where does `{{MAILBOX_ID}}` templating live?** The API substitutes `{{TARGET_URL}}`, `{{DOMAIN}}`, `{{MAILBOX_ID}}`, `{{PROFILE_ID}}`, `{{VAULT_ID}}` — but only for custom connections; a catalog connector ships the literal string and relies on a backend fallback. Should the CLI know about these at all, or is a placeholder just text it passes through?
2. **Should `notte stack check` run the connector contract?** It is a different shape from the function contract, so either `check` grows a per-kind rule or connectors get their own validation pass.
3. **Do connectors need `notte stack dev`?** Driving a real login locally is exactly what `scripts/drive_login.py` does today, and it is how selectors and refusal phrases get discovered. That is a strong argument for `dev` being connector-shaped first.
4. **Does the smoke suite move too?** `scripts/smoke.py` (1,000 lines) drives real logins against a live environment with a shared test account, a mailbox mutex and a long-lived connection per environment. It is genuinely managed-auth-specific and probably stays — but it is the other half of the workflow.

---

## What I would not do

**Do not model connections.** A connection is one workspace's instance of a connector: real credentials, a browser profile, a vault, and a status that only the runtime can know. Creating one runs a real browser login and spends money. RFC 0001 already puts it on the imperative side, and nothing here changes that.
