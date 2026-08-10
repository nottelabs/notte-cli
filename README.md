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

- **Browser sessions** - headless or headed Chromium/Chrome with full control
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
SESSION_ID=$(notte sessions start -o json | jq -r '.session_id')
```

Sessions are headless by default. Add `--headed` for a visible browser, and
watch it through the `ViewerUrl` in the output. Save the returned `session_id`
and pass it explicitly to every command that targets the session.

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
notte sessions status --session-id <id>                 # Get session status
notte sessions stop --session-id <id>                   # Stop session
notte sessions cookies --session-id <id>                # Get all cookies
notte sessions cookies-set --session-id <id> --file cookies.json
notte sessions network --session-id <id>                # View network activity logs
notte sessions replay --session-id <id>                 # Get session replay data
notte sessions workflow-code --session-id <id>          # Export steps as Python workflow code
notte sessions viewer --session-id <id>                 # Open session viewer in browser
notte sessions code --session-id <id>                   # Get Python script for session steps
```

Every command that targets a session requires `--session-id <session-id>`.

#### Session Start Options

```bash
notte sessions start \
  --browser-type chromium|chrome  # Browser type (default: chromium)
  --headed                                # Show a browser window (headless is the default)
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
  --cdp-url <url>                         # CDP URL of remote session provider
  --profile-id <id>                       # Profile ID to use for session
  --profile-persist                       # Save browser state to profile on close
  --screenshot-type <type>                # Screenshot type (raw, full, last_action)
  --chrome-args <args>                    # Chrome instance arguments (repeatable)
```

### Page Actions

Interact with pages using simplified commands. Every page command requires
`--session-id <id>`:

```bash
notte page observe --session-id <id>                    # Get page state and available actions
notte page scrape --session-id <id> --instructions "..." # Scrape content from the page
notte page click --session-id <id> "@B3"                # Click an element by ID
notte page fill --session-id <id> "@I1" "text"          # Fill an input field
notte page goto --session-id <id> "https://example.com" # Navigate to a URL
notte page back --session-id <id>                       # Go back in history
notte page forward --session-id <id>                    # Go forward in history
notte page scroll-down --session-id <id> [amount]       # Scroll down the page
notte page scroll-up --session-id <id> [amount]         # Scroll up
notte page press --session-id <id> "Enter"              # Press a key
notte page screenshot --session-id <id>                 # Take a screenshot
notte page select --session-id <id> <element> "option"  # Select dropdown option
notte page check --session-id <id> <element>            # Check/uncheck checkbox
notte page upload --session-id <id> <element> --file <name>  # Fill a file input. <name> is a file in your
                                      # uploads store, not a local path - send it with
                                      # `notte files upload` first
notte page download --session-id <id> <element>         # Download by clicking. The file lands in the
                                      # session store; retrieve it with
                                      # `notte files download <name> --from session --session-id <id>`
notte page new-tab --session-id <id> <url>              # Open URL in new tab
notte page switch-tab --session-id <id> <index>         # Switch to tab by index
notte page close-tab --session-id <id>                  # Close current tab
notte page reload --session-id <id>                     # Reload page
notte page wait --session-id <id> <seconds>             # Wait for duration
notte page captcha-solve --session-id <id> <type>       # Solve captcha
```

### Functions

```bash
notte functions list [--page N] [--page-size N] [--include-deleted]  # List functions
notte functions create --file workflow.py  # Create a new function
notte functions show --function-id <id>                    # View function details
notte functions update --function-id <id> --file workflow.py
notte functions delete --function-id <id>                  # Delete function
notte functions fork --function-id <id>                    # Fork function to new version
notte functions run --function-id <id>                     # Execute function
notte functions runs --function-id <id> [--page N] [--page-size N] [--running]
notte functions run-stop --function-id <id> --run-id <run-id>
notte functions run-metadata --function-id <id> --run-id <run-id>
notte functions schedule --function-id <id> --cron "0 9 * * *"
notte functions unschedule --function-id <id>
```

Every command that targets a function requires `--function-id <function-id>`.

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
notte files list --from session --session-id <id>              # List files produced by a session
notte files download <filename> --from session --session-id <id> # Download a session file
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
# Start a session and capture its ID
SESSION_ID=$(notte sessions start -o json | jq -r '.session_id')

# Navigate to page
notte page goto --session-id "$SESSION_ID" "https://news.ycombinator.com"

# Extract structured data
notte page scrape --session-id "$SESSION_ID" --instructions "Extract top 10 stories with title and URL"

# Cleanup
notte sessions stop --session-id "$SESSION_ID"
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
# Start browser with specific configuration and capture its ID
SESSION_ID=$(notte sessions start -o json \
  --browser-type chrome \
  --viewport-width 1920 \
  --viewport-height 1080 | jq -r '.session_id')

# Navigate and interact
notte page goto --session-id "$SESSION_ID" "https://example.com"
notte page click --session-id "$SESSION_ID" "#login-button"
notte page fill --session-id "$SESSION_ID" "#username" "user@example.com"

# Get page state with available actions
notte page observe --session-id "$SESSION_ID"

# Stop when done
notte sessions stop --session-id "$SESSION_ID"
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
1. `SESSION_ID=$(notte sessions start -o json | jq -r '.session_id')` - Start a browser session and capture its ID
2. `notte page goto --session-id "$SESSION_ID" <url>` - Navigate to a URL
3. `notte page observe --session-id "$SESSION_ID"` - Get interactive elements with IDs (@B1, @B2)
4. `notte page click --session-id "$SESSION_ID" "@B1"` / `notte page fill --session-id "$SESSION_ID" "@I1" "text"` - Interact using element IDs
5. `notte page scrape --session-id "$SESSION_ID" --instructions "..."` - Extract structured data
6. `notte sessions stop --session-id "$SESSION_ID"` - Clean up when done
```

### Tips

- **Viewing sessions**: When you start a session, the output includes a `ViewerUrl` - open it to watch the browser live, headless or not
- **Session lifetime**: sessions close after **3 minutes idle** or **15 minutes total** by default. Raise `--idle-timeout-minutes`/`--max-duration-minutes` for anything slow, or the next command fails with `Session closed`
- **Element selectors**: If element IDs from `observe` (like `@B1`) don't work, use Playwright selectors: `#id`, `.class`, `button:has-text('Submit')`
- **Multiple matches**: Use `>> nth=0` suffix to select the first match: `button:has-text('OK') >> nth=0`
- **Closing modals**: `notte page press --session-id <id> "Escape"` reliably dismisses most dialogs

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
