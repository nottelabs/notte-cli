# Notte CLI - Browser automation in your terminal

<div align="center">
  <p>
    Control browser sessions and web scraping through intuitive <strong>resource-based commands</strong> <br/>
    → Read more at: <a href="https://notte.cc?ref=github" target="_blank" rel="noopener noreferrer">Landing</a> • <a href="https://console.notte.cc/?ref=github" target="_blank" rel="noopener noreferrer">Console</a> • <a href="https://docs.notte.cc?ref=github" target="_blank" rel="noopener noreferrer">Docs</a> • <a href="https://x.com/nottecore?ref=github" target="_blank" rel="noopener noreferrer">X</a> • <a href="https://www.linkedin.com/company/nottelabsinc/?ref=github" target="_blank" rel="noopener noreferrer">LinkedIn</a>
  </p>
</div>

[![GitHub stars](https://img.shields.io/github/stars/nottelabs/notte-cli?style=social)](https://github.com/nottelabs/notte-cli/stargazers)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go 1.25+](https://img.shields.io/badge/go-1.25+-blue.svg)](https://go.dev/)
[![Homebrew](https://img.shields.io/badge/homebrew-nottelabs%2Fnotte--cli-blue.svg)](https://github.com/nottelabs/notte-cli)

---

# What is Notte CLI?

The Notte CLI brings the full power of [notte.cc](https://notte.cc?ref=github) to your terminal — letting you drive browser sessions and web scraping pipelines from the command line. Pair it with shell scripts, CI/CD pipelines, or AI coding assistants for repeatable, scriptable web automation.

## Features

- **Browser sessions** - remote Chromium/Chrome with full control
- **Files** - upload and download files to notte.cc
- **Output formats** - human-readable text or JSON for scripting
- **Personas** - create and manage digital identities with email, phone, and SMS
- **Secure credentials** - system keyring for API keys, vaults for website passwords
- **Web scraping** - structured data extraction with custom schemas
- **Functions** - schedule and execute repeatable automation tasks

## Installation

### Homebrew

```bash
brew tap nottelabs/notte-cli https://github.com/nottelabs/notte-cli.git
brew install notte
```

### Go Install

```bash
go install github.com/nottelabs/notte-cli/cmd/notte@latest
```

### Build from Source

```bash
git clone https://github.com/nottelabs/notte-cli.git
cd notte-cli
make build
```

## Quick Start

### 1. Authenticate

Specify the API key using one of three methods (checked in priority order):

```bash
# 1. Via environment variable (recommended for CI/CD)
export NOTTE_API_KEY="your-api-key"
# 2. Via system keyring (recommended for local development)
notte auth login
# 3. Via config file (~/.notte/cli/config.json)
# create ~/.notte/cli/config.json and add your API key
notte auth status
```

### 2. Start a Browser Session

```bash
notte sessions start
```

Watch the session live through the `ViewerUrl` in the output.

## Commands

### Authentication

```bash
notte auth login                     # Store API key in system keychain
notte auth logout                    # Remove API key from keychain
notte auth status                    # Show authentication status
```

### Web Search

```bash
notte search <query>                                    # Search the web for a query
notte search <query> --depth fast|standard|deep         # Tune search depth (default: standard)
notte search <query> --output-type sourcedAnswer        # Get an LLM answer with sources
```

The query may be quoted (`notte search "what is anthropic"`) or passed as separate
words (`notte search what is anthropic`). Use `--output json` to get the raw API
response for scripting.

### Browser Sessions

```bash
notte sessions list [--page N] [--page-size N] [-a|--all]  # List running sessions (-a includes stopped)
notte sessions start [flags]          # Start a new session
notte sessions status                 # Get current session status
notte sessions stop                   # Stop current session
notte sessions cookies                # Get all cookies from current session
notte sessions cookies-set --file cookies.json  # Set cookies in current session
notte sessions network                # View network activity logs
notte sessions replay                 # Get session replay data
notte sessions workflow-code          # Export session steps as Python code
notte sessions viewer                 # Open session viewer in browser
notte sessions code                   # Get Python script for session steps
```

**Note:** When you start a session, it automatically becomes the "current" session. All subsequent commands use this session by default. Use `--session-id <session-id>` only when you need to manage multiple sessions simultaneously or reference a specific session.

#### Session Start Options

```bash
notte sessions start \
  --browser-type chromium|chrome  # Browser type (default: chromium)
  --idle-timeout-minutes <minutes>        # Idle timeout (default: 3)
  --max-duration-minutes <minutes>        # Maximum session lifetime (default: 15)
  --user-agent <string>                   # Custom user agent
  --viewport-width <pixels>               # Viewport width
  --viewport-height <pixels>              # Viewport height
  --proxy                                  # Use default proxy rotation
  --proxy-country <code>                  # Proxy with specific country (e.g. us, gb, fr)
  --no-solve-captchas                     # Turn OFF captcha solving (on by default)
  --no-file-storage                       # Detach FileStorage (attached by default).
                                          # Disables page download and files --from session
  --advanced-stealth                      # Highest-fidelity browser for sites with
                                          # sophisticated bot detection (approved workspaces)
  --cdp-url <url>                         # CDP URL of remote session provider
  --profile-id <id>                       # Profile ID to use for session
  --profile-persist                       # Save browser state to profile on close
  --vault-id <id>                         # Vault used to resolve credential fields
  --screenshot-type <type>                # Screenshot type (raw, full, last_action)
  --chrome-args <args>                    # Chrome instance arguments (repeatable)
```

### Page Actions

Interact with pages using simplified commands (requires an active session). Start
the session with `--vault-id <id>` before using `page fill --vault-field`:

```bash
notte page observe                    # Get page state and available actions
notte page scrape --instructions "..." # Scrape content from the page 
notte page click "@B3"            # Click an element by ID
notte page fill "@I1" "text"    # Fill an input field
notte page fill "#email" --vault-field email       # Fill from the session vault
notte page fill "#password" --vault-field password # Supports email, username, password, and mfa
notte page goto "https://example.com" # Navigate to a URL
notte page back                       # Go back in history
notte page forward                    # Go forward in history
notte page scroll-down [amount]       # Scroll down the page
notte page scroll-up [amount]         # Scroll up
notte page press "Enter"              # Press a key
notte page screenshot                 # Take a screenshot
notte page select <id> "option"       # Select dropdown option
notte page check <id>                 # Check/uncheck checkbox
notte page upload <id> --file <name>  # Fill a file input. <name> is a file in your
                                      # uploads store, not a local path - send it with
                                      # `notte files upload` first
notte page download <id>              # Download by clicking. The file lands in the
                                      # session store; retrieve it with
                                      # `notte files download <name> --from session`
notte page new-tab <url>              # Open URL in new tab
notte page switch-tab <index>         # Switch to tab by index
notte page close-tab                  # Close current tab
notte page reload                     # Reload page
notte page wait <seconds>             # Wait for duration
notte page captcha-solve              # Solve captcha
notte page eval-js "document.title"   # Evaluate JavaScript in the page
```

#### Evaluating JavaScript

`page eval-js` prints the evaluated value **alone on stdout** — objects and
arrays as JSON, a JS `null` as `null` — with the status line on stderr, so it
captures and pipes without post-processing:

```bash
title=$(notte page eval-js "document.title")

notte page eval-js "JSON.stringify([...document.querySelectorAll('a')].map(a => a.href))" | jq length
```

Return `JSON.stringify(...)` when the answer is structured. `console.log` output
is discarded — only the returned value comes back. A failing script exits
non-zero and reports the actual JavaScript error; use `-o json` to get the full
execution result instead of the bare value.

### Functions

```bash
notte functions list [--page N] [--page-size N] [--include-deleted]  # List functions
notte functions create --file workflow.py  # Create a new function
notte functions show                  # View current function details
notte functions show --function-id <id>  # View specific function details (different from current function)
notte functions download workflow.py [--version <version>]  # Download current function code to disk
notte functions create --file workflow.py --response-format @schema.json  # ... with its response documented
notte functions update --file workflow.py  # Update current function code
notte functions update --file workflow.py --response-format @schema.json  # ... and re-document its response
notte functions configure --run-instructions "..." --self-healing  # Set usage notes and self-healing
notte functions rollback --version <version>  # Restore an earlier version (see `versions` in show)
notte functions health                # Runtime health: Python version, installed packages, reachability
notte functions delete                # Delete current function
notte functions fork                  # Fork current function to new version
notte functions run                   # Execute current function
notte functions run --no-stream       # ... returning only the final response, without streamed logs
notte functions runs [--page N] [--page-size N] [--running]  # List runs for current function (--running = in-flight only)
notte functions run-stop --run-id <id>  # Stop a running function execution
notte functions run-metadata --run-id <id>  # Get run logs and results
notte functions schedule --cron "0 12 ? * * *"  # Schedule current function (six-field cron: daily at noon UTC)
notte functions unschedule            # Remove schedule from current function
```

### Personas, Profiles and Usage

```bash
notte personas update --persona-id <id> --name "checkout tester"  # Rename a persona
notte profiles cookies --profile-id <id>                          # Read a profile's cookies
notte profiles cookies-set --profile-id <id> --file cookies.json  # Import cookies into a profile
notte usage logs [--endpoint /sessions/start] [--page N]          # List API requests made with your key
```

`profiles cookies-set` takes either a bare array of cookies — what Playwright's
`storageState` and the browser extensions export — or an object with a `cookies`
key. Add `--source-format chrome` if they came from Chrome, and `--mode append`
to add to the profile's cookies rather than replace them.

`--response-format` takes a JSON Schema describing what `run()` returns, as
inline JSON, `@file.json`, or `-` for stdin. The API never derives it, so a
function created without it has no documented response — which is what the
console reads to show callers the shape they will get back. From a pydantic
return model:

```bash
python -c 'import json, typing, client; print(json.dumps(typing.get_type_hints(client.run)["return"].model_json_schema()))' > schema.json
notte functions create --file client.py --response-format @schema.json
```

`--run-instructions` is documentation for whoever *calls* the function — how long a
run takes, what each variable is for, which sites it trips over:

```bash
notte functions configure --run-instructions "Takes ~3 min, so call it async. \
Hits a captcha on the login page every few runs. \
\`query\` is the search term; \`max_items\` caps the results."
```

It is not input to the self-healing agent, which is the separate
`--self-healing` flag.

`configure` sends only the flags you pass, so setting `--run-instructions` leaves
self-healing untouched. Disable self-healing with `--self-healing=false`: the
API treats an absent field as "leave it alone" rather than "off". Note that it
can only be enabled on functions an agent built — a CLI-created function has no
thread for the healer to resume, and the API rejects it.

**Note:** When you create a function, it automatically becomes the "current" function. All subsequent commands use this function by default. Use `--function-id <function-id>` only when you need to manage multiple functions simultaneously or reference a specific function.

### Vaults

```bash
notte vaults list [--page N] [--page-size N] [--include-deleted]  # List all vaults
notte vaults create                                   # Create a new vault
notte vaults update --vault-id <id>                   # Update vault metadata
notte vaults delete --vault-id <id>                   # Delete a vault
notte vaults credentials list --vault-id <id>         # List all credentials
notte vaults credentials add --vault-id <id>          # Add credentials
notte vaults credentials get --vault-id <id>          # Get credentials for URL
notte vaults credentials delete --vault-id <id>       # Delete credentials
```

### Personas

```bash
notte personas list [--page N] [--page-size N] [--include-deleted]  # List all personas
notte personas create                    # Create a new persona
notte personas show --persona-id <id>    # View persona details
notte personas delete --persona-id <id>  # Delete a persona
notte personas emails --persona-id <id>  # List emails
notte personas sms --persona-id <id>     # List SMS messages
```

### Profiles

```bash
notte profiles list [--page N] [--page-size N] [--name "..."] [--include-deleted]  # List all profiles
notte profiles create                    # Create a new profile
notte profiles show --profile-id <id>    # View profile details
notte profiles delete --profile-id <id>  # Delete a profile
```

### Files

```bash
notte files upload <path>                                      # Upload a persistent input file
notte files list --from uploads                                # List persistent input files
notte files download <filename> --from uploads                 # Download a persistent input file
notte files list --from session [--session-id <id>]            # List files produced by a session
notte files download <filename> [--session-id <id>]            # Download a file produced by a session
```

### Utilities

```bash
notte usage                          # View API usage statistics
notte health                         # Check API health status
notte version                        # Show CLI version
```

## Output Formats

### Text

Human-readable tables with colors and formatting:

```bash
$ notte sessions list
ID                        STATUS    BROWSER     CREATED
ses_abc123def456          ACTIVE    chromium    2024-01-15 10:30:00
ses_xyz789uvw012          STOPPED   chrome      2024-01-15 09:15:00
```

### JSON

Machine-readable output:

```bash
$ notte sessions list --output json
{
  "sessions": [
    {
      "id": "ses_abc123def456",
      "status": "ACTIVE",
      "browser": "chromium",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

Data goes to stdout, errors and progress to stderr for clean piping.

## Examples

### Automated Web Scraping Pipeline

```bash
# Start session (automatically becomes the current session)
notte sessions start

# Navigate to page
notte page goto "https://news.ycombinator.com"

# Extract structured data
notte page scrape --instructions "Extract top 10 stories with title and URL"

# Cleanup
notte sessions stop
```

### Running a Workflow

```bash
# List functions to find ID
notte functions list

# Run workflow
notte functions run --function-id func_abc123
```

### Managing Credentials Securely

```bash
# Create a vault for production credentials
VAULT_ID=$(notte vaults create --name "Production Sites" -o json | jq -r '.id')

# Add website credentials
notte vaults credentials add --vault-id $VAULT_ID \
  --username "admin@example.com" \
  --password "$SECURE_PASSWORD" \
  --url "https://app.example.com"

# List stored credentials
notte vaults credentials list --vault-id $VAULT_ID
```

### Multi-Step Browser Automation

```bash
# Start browser with specific configuration
notte sessions start \
  --browser-type chrome \
  --viewport-width 1920 \
  --viewport-height 1080

# Navigate and interact
notte page goto "https://example.com"
notte page click "#login-button"
notte page fill "#username" "user@example.com"

# Get current page state with available actions
notte page observe

# Stop when done
notte sessions stop
```

### JQ Filtering

```bash
# Get only active sessions (using built-in filter)
notte sessions list --all

# Paginate through results
notte sessions list --page 2 --page-size 5

# Extract session IDs with jq
notte sessions list --output json | jq -r '.sessions[].id'
```

## Usage with AI Agents

### Just Ask the Agent

The simplest approach - just tell your agent to use it:

> Use notte to test the login flow. Run `notte --help` to see available commands.

The `--help` output is comprehensive and most agents can figure it out from there.

### AI Coding Assistants

Add the skill to your AI coding assistant for richer context:

```bash
npx skills add nottelabs/notte-skills
```

This works with Claude Code, Cursor, Windsurf, and other MCP-compatible assistants.

### AGENTS.md / CLAUDE.md

For more consistent results, add to your project or global instructions file:

```markdown
## Browser Automation

Use `notte` for web automation. Run `notte --help` for all commands.

Core workflow:
1. `notte sessions start` - Start a browser session
2. `notte page goto <url>` - Navigate to a URL
3. `notte page observe` - Get interactive elements with IDs (@B1, @B2)
4. `notte page click "@B1"` / `notte page fill "@I1" "text"` - Interact using element IDs
5. `notte page scrape --instructions "..."` - Extract structured data
6. `notte sessions stop` - Clean up when done
```

### Tips

- **Viewing sessions**: When you start a session, the output includes a `ViewerUrl` - open it to watch the browser live
- **Session lifetime**: sessions close after **3 minutes idle** or **15 minutes total** by default. Raise `--idle-timeout-minutes`/`--max-duration-minutes` for anything slow, or the next command fails with `Session closed`
- **Element selectors**: If element IDs from `observe` (like `@B1`) don't work, use Playwright selectors: `#id`, `.class`, `button:has-text('Submit')`
- **Multiple matches**: Use `>> nth=0` suffix to select the first match: `button:has-text('OK') >> nth=0`
- **Closing modals**: `notte page press "Escape"` reliably dismisses most dialogs

### Skills Documentation

For comprehensive documentation including templates and reference guides, see the [notte-skills/plugins/notte-cli/skills/notte-browser](notte-skills/plugins/notte-cli/skills/notte-browser/SKILL.md) folder (vendored as a submodule from [nottelabs/notte-skills](https://github.com/nottelabs/notte-skills)).

## Security

### Credential Storage

API keys are stored securely in your system's keychain:
- **macOS**: Keychain Access
- **Linux**: Secret Service (GNOME Keyring, KWallet)
- **Windows**: Credential Manager

### Best Practices

- Never pass API keys on the command line
- Use vaults for website passwords and payment cards
- Rotate API keys regularly from notte.cc dashboard
- Use `notte auth logout` to remove stored keys

## Shell Completions

Generate shell completions for your preferred shell:

### Bash

```bash
# macOS (Homebrew):
notte completion bash > $(brew --prefix)/etc/bash_completion.d/notte

# Linux:
notte completion bash > /etc/bash_completion.d/notte

# Or source directly:
source <(notte completion bash)
```

### Zsh

```zsh
notte completion zsh > "${fpath[1]}/_notte"
```

### Fish

```fish
notte completion fish > ~/.config/fish/completions/notte.fish
```

### PowerShell

```powershell
notte completion powershell | Out-String | Invoke-Expression
```

## Development

After cloning, install git hooks:

```bash
make setup
```

This installs [lefthook](https://github.com/evilmartians/lefthook) pre-commit and pre-push hooks for linting and testing.

## License

This project is licensed under the MIT License.

## Links

- [Landing](https://notte.cc?ref=github)
- [Console](https://console.notte.cc/?ref=github)
- [Documentation](https://docs.notte.cc?ref=github)
- [Main repository (nottelabs/notte)](https://github.com/nottelabs/notte)

Copyright © 2025 Notte Labs, Inc.

## Coverage guards

Two checks keep the CLI in step with what surrounds it. Both read a live source
of truth, so both need the network; `make` runs them with `-strict`, where an
unreachable source is a failure. The pre-commit hooks run them without it, so an
offline commit warns and proceeds.

```bash
make check-endpoints   # every API endpoint is reachable or recorded as skipped
make check-skills      # every command is documented in nottelabs/notte-skills
make check-coverage    # both
```

`check-endpoints` compares the API's OpenAPI spec against the commands. An
endpoint is covered when a command calls its generated client method; anything
else has to be listed in [`scripts/endpoint-coverage.txt`](scripts/endpoint-coverage.txt)
with a reason, as `manual` (a command builds the request itself) or `skip` (not
exposed on purpose). A line whose endpoint the API no longer serves fails too,
so the file cannot rot the way the flag generator's endpoint map did.

`check-skills` walks the cobra tree and requires every non-hidden command to be
mentioned somewhere under `plugins/notte/skills/` in the skills repository. Pass
`-skills-dir <path>` to check against a working copy before pushing it:

```bash
go run ./scripts/checkcoverage -check skills -skills-dir ../notte-skills -strict
```
