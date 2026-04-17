---
name: notte-browser
description: Command-line interface for browser automation, web scraping, and AI-powered web interactions using the notte.cc platform.
version: 1.1.0
allowed-tools: Bash(notte:*)
---

# Notte Browser CLI Skill

Command-line interface for browser automation, web scraping, and AI-powered web interactions using the notte.cc platform.

## Quick Start

```bash
# 1. Authenticate
notte auth login

# 2. Start a browser session
notte sessions start

# 3. Goto and observe
notte page goto "https://example.com"
notte page observe
notte page screenshot

# 4. Execute actions (use @IDs from observe, or Playwright selectors)
notte page click "@B3"
notte page fill "@I1" "hello world"
# If @IDs don't work, use Playwright selectors:
# notte page click "button:has-text('Submit')"

# 5. Scrape content
notte page scrape --instructions "Extract all product names and prices"

# 6. Stop the session (uses current session; pass --session-id only for another)
notte sessions stop
```

> **There is no top-level `notte scrape <url>` command** despite what `notte --help` implies. Scraping always goes through `notte page scrape` inside an active session (after `goto`).
>
> **All resource IDs are passed as named flags** (`--session-id`, `--agent-id`, `--persona-id`, `--profile-id`, `--vault-id`, `--function-id`, `--run-id`), never positional. `notte personas delete <id>` fails with `unknown command`; use `notte personas delete --persona-id <id>`.
>
> **Flag-name traps:** `notte page scrape` takes `--instructions` (plural),
> not `--instruction`. `notte sessions start` has **no `--url`** flag — start
> the session, then `notte page goto <url>`. `notte page goto` takes a
> positional URL, not `--url`.

## Command Categories

### Session Management

Control browser session lifecycle:

```bash
# Start a new session
notte sessions start [flags]
  --headless                 Run in headless mode
  --browser-type             Browser type: chromium, chrome, firefox, chrome-nightly, chrome-turbo (default: chromium)
  --idle-timeout-minutes     Idle timeout in minutes (resets on each operation)
  --max-duration-minutes     Maximum session lifetime in minutes (absolute)
  --proxy                    Use default proxies
  --proxy-country            Proxy country code (e.g. us, gb, fr). Implies --proxy
  --solve-captchas           Automatically solve captchas
  --viewport-width           Viewport width in pixels
  --viewport-height          Viewport height in pixels
  --user-agent               Custom user agent string
  --cdp-url                  CDP URL of remote session provider
  --use-file-storage         Enable file storage for the session
  --profile-id               Profile ID to use (for persistent browser state)
  --profile-persist          Save browser state to profile on session close
  --vault-id                 Vault to use for credential access
  --web-bot-auth             Use web bot authentication
  --screenshot-type          Screenshot type: raw, full, last_action
  --chrome-args              Overwrite chrome args (repeatable)
  --extra-http-headers       Extra HTTP headers as JSON

# Get current session status
notte sessions status

# Stop current session
notte sessions stop

# List sessions (with optional pagination and filters)
notte sessions list [--page N] [--page-size N] [--only-active]
```

**Note:** When you start a session, it automatically becomes the "current" session (saved to `~/.notte/cli/current_session`; can also be overridden with `NOTTE_SESSION_ID` env var). All subsequent commands use this session by default. Use `--session-id <session-id>` only when you need to manage multiple sessions simultaneously or reference a specific session.

> **Stale current-session trap (common in fresh shells / CI / eval runs).** The
> "current session" pointer persists across shells and can outlive the session
> it points to. Symptoms:
> - `Error 410: Session not found` on a command that worked seconds ago.
> - `Error 500: Browser or context expired or closed`.
> - `Error 422: Extra inputs are not permitted` on `observe` *right after* a
>   successful `goto` (the 422 the skill flags elsewhere for missing goto, but
>   here the real cause is a dead session).
> - `observe` returning content from a completely different site than you just
>   navigated to (a leftover session was reused).
> - `sessions start` printing `"Session <old-id> is currently active. A new
>   session will be created either way"` — the pointer was stale.
>
> **Recovery pattern.** Capture the fresh session id from `-o json` and pass
> `--session-id` explicitly on every dependent call. Chain dependent commands
> in a single shell so the session can't die between invocations:
>
> ```bash
> SID=$(notte sessions start -y -o json | python3 -c "import sys,json;print(json.load(sys.stdin)['session_id'])")
> notte page goto "https://example.com" --session-id "$SID" \
>   && notte page observe --session-id "$SID" \
>   && notte page click "@L1" --session-id "$SID"
> ```
>
> Use `-y` on `sessions start` to skip the confirmation prompt when a prior
> session is still marked "current".

Session debugging and export:

```bash
# Get network logs
notte sessions network [--path /output/dir] [--urls-only]

# Download session replay video (MP4)
notte sessions replay [--path output.mp4]

# Export session steps as Python workflow code
notte sessions workflow-code

# Get Python script for session steps
notte sessions code

# Get session offset info
notte sessions offset

# Open session viewer in browser
notte sessions viewer
```

Cookie management:

```bash
# Get all cookies
notte sessions cookies

# Set cookies from JSON file
notte sessions cookies-set --file cookies.json
```

### Page Actions

Simplified commands for page interactions:

**Element Interactions:**
```bash
# Click an element (use either the ids from an observe, or a selector)
notte page click "@B3"
notte page click "#submit-button"
  --timeout     Timeout in milliseconds
  --enter       Press Enter after clicking

# Fill an input field
notte page fill "@I1" "hello world"
  --clear       Clear field before filling
  --enter       Press Enter after filling

# Check/uncheck a checkbox
notte page check "#my-checkbox"
  --value       true to check, false to uncheck (default: true)

# Select dropdown option
notte page select "#dropdown-element" "Option 1"

# Download file by clicking element
notte page download "@L5"

# Upload file to input
notte page upload "#file-input" --file /path/to/file

# Run JavaScript in the Page
- Escape single quotes if needed.
- Don’t use logging (output won’t be captured).
- Use a single statement or a function that returns a value.

# Single expression
notte page eval-js 'document.title'

# Function with return value
notte page eval-js '
() => {
  const els = document.querySelectorAll("a");
  return els.length;
}
'
```

**Navigation:**
```bash
# URL is a positional argument — do NOT use --url
notte page goto "https://example.com"
notte page new-tab "https://example.com"
notte page back
notte page forward
notte page reload
```

> **`page observe` before a `goto` returns HTTP 422** with the misleading message `Extra inputs are not permitted ... ('body', 'url')`. The error is about server state (no page loaded), **not** about the `--url` flag. Always `notte page goto <url>` first, then `notte page observe`.

**Scrolling:**
```bash
notte page scroll-down [amount]
notte page scroll-up [amount]
```

> If `scroll-down`/`scroll-up` returns `"Scroll failed. Either the page is not
> scrollable or there is a focused element blocking the scroll"`, use the
> keyboard instead — keys always work: `notte page press "PageDown"` /
> `"PageUp"` / `"End"` / `"Home"`.

**Keyboard:**
```bash
notte page press "Enter"
notte page press "Escape"
notte page press "Tab"
```

**Tab Management:**
```bash
notte page switch-tab 1
notte page close-tab
```

> Tabs re-index immediately after `close-tab`. If you close tab 1, the only
> remaining tab is index 0 — `switch-tab 1` will then fail with "Tab index out
> of range". Track expected count, or call `notte page observe` in between.

**Page State:**
```bash
# Observe page state and available actions
notte page observe

# Save a screenshot (defaults to tmp folder; optional output path)
notte page screenshot [output-path]

# Scrape content with instructions
notte page scrape --instructions "Extract all links" [--only-main-content]
```

**Utilities:**
```bash
# Wait for specified duration
notte page wait 1000

# Solve CAPTCHA
notte page captcha-solve "recaptcha"

# Mark task complete
notte page complete "Task finished successfully" [--success]

# Run JavaScript on the page
notte page eval-js "document.title"
notte page eval-js "document.querySelectorAll('a').length"

# Fill form with JSON data.
# `form-fill` only accepts a fixed schema of semantic keys, NOT arbitrary field labels.
# Valid keys include: title, first_name, middle_name, last_name, full_name, email,
#   company, address1, address2, address3, city, state, zip, country, phone, username,
#   password, dob (and similar identity/address fields).
# For fields outside this schema (radios, checkboxes, non-standard labels), use
# `notte page fill "#selector" "value"` / `notte page check` / `notte page select` instead.
notte page form-fill --data '{"full_name": "Alice", "email": "a@b.c", "phone": "555-0100"}'
```

### File Storage

Upload and manage files in notte.cc account-scoped storage (not per-session):

```bash
notte files upload <local-path>            # Upload a file
notte files list [--uploads|--downloads]   # List files (default: --downloads)
notte files download <filename> [--path output-path]  # Download a file
```

> **No `delete` subcommand exists.** `notte files` supports only `upload`,
> `list`, and `download`. The REST endpoint `DELETE /storage/uploads/<name>`
> is also not implemented (returns 405). If a task requires deleting an
> uploaded file, report "not supported via CLI" rather than chasing
> browser-agent or raw-API workarounds — they don't work either.
>
> `list --downloads` is the default and requires an active session (downloads
> are session-scoped). Use `list --uploads` for account-scoped uploaded files
> (works without a session).

### AI Agents

Start and manage AI-powered browser agents:

```bash
# List all agents (with optional pagination and filters)
notte agents list [--page N] [--page-size N] [--only-active] [--only-saved]

# Start a new agent (auto-uses current session if active)
notte agents start --task "Navigate to example.com and extract the main heading"
  --session-id             Session ID (uses current session if not specified)
  --url                    URL the agent should start on (optional)
  --vault-id               Vault ID for credential access
  --persona-id             Persona ID for identity
  --max-steps              Maximum steps for the agent
  --reasoning-model        Custom reasoning model
  --use-vision             Use vision for the agent (not all models support it)
  --response-format-json   Response format config (JSON file path, e.g. @config.json)
  --session-offset         Step from which agent gathers session info (experimental)

# Get current agent status
notte agents status

# Stop current agent
notte agents stop

# Export agent steps as workflow code
notte agents workflow-code

# Get agent execution replay
notte agents replay
```

**Note:** When you start an agent, it automatically becomes the "current" agent (saved to `~/.notte/cli/current_agent`). All subsequent commands use this agent by default. Use `--agent-id <agent-id>` only when you need to manage multiple agents simultaneously or reference a specific agent.

**Agent ID Resolution:**
1. `--agent-id` flag (highest priority)
2. `NOTTE_AGENT_ID` environment variable
3. `~/.notte/cli/current_agent` file (lowest priority)

> **One agent per session.** A session can run only one agent at a time. If `notte agents start` returns HTTP 409 `"already has an active agent"`, call `notte agents stop` (or wait with `notte agents status`) before starting a new one.

### Functions (Workflow Automation)

Create, manage, and schedule reusable workflows:

```bash
# List all functions (with optional pagination and filters)
notte functions list [--page N] [--page-size N] [--only-active]

# Create a function from a workflow file
notte functions create --file workflow.py [--name "My Function"] [--description "..."] [--shared]

# Show current function details
notte functions show

# Update current function code
notte functions update --file workflow.py

# Delete current function
notte functions delete

# Run current function (pass variables with --var or --vars)
notte functions run [--var key=value ...] [--vars '{"key":"value"}']

# List runs for current function (with optional pagination and filters)
notte functions runs [--page N] [--page-size N] [--only-active]

# Stop a running function execution
notte functions run-stop --run-id <run-id>

# Get run logs and results
notte functions run-metadata --run-id <run-id>

# Schedule current function — cron is 6-field AWS-style (min hour dom month dow year)
notte functions schedule --cron "0 9 * * ? *"

# Remove schedule from current function
notte functions unschedule

# Fork a shared function to your account
notte functions fork [--function-id <shared-function-id>]
```

**Note:** When you create a function, it automatically becomes the "current" function. All subsequent commands use this function by default. Use `--function-id <function-id>` only when you need to manage multiple functions simultaneously or reference a specific function (like when forking a shared function).

> **Function file contract** (enforced by `functions create`/`update`; each violation is a separate 400):
> - File must be `.py` (not `.yaml`, `.json`, `.sh`).
> - Must define a top-level **synchronous** `def run(...)`. `async def run` is rejected (`AsyncFunctionDef not allowed`). `def _run` is rejected (names can't start with `_`).
> - Must create a notte session using one of: `notte.Session(...)`, `n.Session(...)`, `c.Session(...)`, `cli.Session(...)`, `client.Session(...)`.
> - **Cron is 6-field AWS-style**, not standard 5-field: `"minute hour day-of-month month day-of-week year"`. Example: `--cron "0 9 * * ? *"` for daily at 09:00 UTC. `"0 9 * * *"` (5 fields) is rejected.


### Account Management

**Personas** - Auto-generated identities with email:

```bash
# List personas (with optional pagination and filters)
notte personas list [--page N] [--page-size N] [--only-active]

# Create a persona (identity is auto-generated; there is no --name flag)
notte personas create [--create-phone-number] [--create-vault]

# Show persona details
notte personas show --persona-id <persona-id>

# Delete a persona
notte personas delete --persona-id <persona-id>

# List emails received by persona
notte personas emails --persona-id <persona-id>

# List SMS messages received
notte personas sms --persona-id <persona-id>
```

**Vaults** - Store your own credentials:

```bash
# List vaults (with optional pagination and filters)
notte vaults list [--page N] [--page-size N] [--only-active]

# Create a vault
notte vaults create [--name "My Vault"]

# Update vault name
notte vaults update --vault-id <vault-id> --name "New Name"

# Delete a vault
notte vaults delete --vault-id <vault-id>

# Manage credentials
notte vaults credentials list --vault-id <vault-id>
notte vaults credentials add --vault-id <vault-id> --url "https://site.com" --password "pass" [--email "..."] [--username "..."] [--mfa-secret "..."]
notte vaults credentials get --vault-id <vault-id> --url "https://site.com"
notte vaults credentials delete --vault-id <vault-id> --url "https://site.com"
```

**Profiles** — Persistent browser state across sessions:

```bash
# List profiles
notte profiles list [--name "filter"] [--page N] [--page-size N]

# Create a new profile
notte profiles create [--name "My Profile"]

# Show profile details
notte profiles show --profile-id <profile-id>

# Delete a profile
notte profiles delete --profile-id <profile-id>
```

> Use `--profile-id` on `sessions start` to load a profile, and `--profile-persist` to save state back on session close.

## Global Options

Available on all commands:

```bash
--output, -o    Output format: text, json (default: text)
--timeout       API request timeout in seconds (default: 60)
--no-color      Disable color output
--verbose, -v   Verbose output
--yes, -y       Skip confirmation prompts
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NOTTE_API_KEY` | API key for authentication |
| `NOTTE_SESSION_ID` | Default session ID (avoids --session-id flag) |
| `NOTTE_API_URL` | Custom API endpoint URL |

## Session ID Resolution

Session ID is resolved in this order:
1. `--session-id` flag
2. `NOTTE_SESSION_ID` environment variable
3. Current session file (set automatically by `sessions start`)

## Examples

### Basic Web Scraping

```bash
# Scrape with session
notte sessions start --headless
notte page goto "https://news.ycombinator.com"
notte page scrape --instructions "Extract top 10 story titles"
notte sessions stop

# Multi-page scraping
notte sessions start --headless
notte page goto "https://example.com/products"
notte page observe
notte page scrape --instructions "Extract product names and prices"
notte page click "@L3"
notte page scrape --instructions "Extract product names and prices"
notte sessions stop
```

### Form Automation

```bash
notte sessions start
notte page goto "https://example.com/signup"
notte page fill "#email-field" "user@example.com"
notte page fill "#password-field" "securepassword"
notte page click "#submit-button"
notte sessions stop
```

### Authenticated Session with Vault

```bash
# Setup credentials once
notte vaults create --name "MyService"
notte vaults credentials add --vault-id <vault-id> \
  --url "https://myservice.com" \
  --email "me@example.com" \
  --password "mypassword" \
  --mfa-secret "JBSWY3DPEHPK3PXP"

# Use in automation (vault credentials auto-fill on matching URLs)
notte sessions start
notte page goto "https://myservice.com/login"
# Credentials from vault are used automatically
notte sessions stop
```

### Scheduled Data Collection

```bash
# Create workflow file
cat > collect_data.py << 'EOF'
# Notte workflow script
# ...
EOF

# Upload as function
notte functions create --file collect_data.py --name "Daily Data Collection"

# Schedule to run every day at 9 AM
notte functions schedule --function-id <function-id> --cron "0 9 * * ? *"

# Check run history
notte functions runs --function-id <function-id>
```

## Tips & Troubleshooting

### Troubleshooting

For stale `observe` output, selector syntax (`@ID` vs Playwright, `>> nth=0`),
modals/dialogs, viewing headless sessions via `ViewerUrl`, and bot-detection
/ CAPTCHA / proxy strategies, see [references/troubleshooting.md](references/troubleshooting.md).

## Additional Resources

- [Session Management Reference](references/session-management.md) - Detailed session lifecycle guide
- [Function Management Reference](references/function-management.md) - Workflow automation guide
- [Account Management Reference](references/account-management.md) - Personas and vaults guide

### Templates

Ready-to-use shell script templates:

- [Form Automation](templates/form-automation.sh) - Fill and submit forms
- [Authenticated Session](templates/authenticated-session.sh) - Login with credential vault
- [Data Extraction](templates/data-extraction.sh) - Scrape structured data
