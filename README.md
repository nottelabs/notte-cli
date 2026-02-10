# Notte CLI

Browser automation in your terminal.

Control browser sessions, AI agents, and web scraping through intuitive resource-based commands.

## Features

- **AI agents** - run and monitor AI-powered browser functions
- **Browser sessions** - headless or headed Chrome/Firefox with full control
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
notte sessions start --headless
```

## Commands

### Authentication

```bash
notte auth login                     # Store API key in system keychain
notte auth logout                    # Remove API key from keychain
notte auth status                    # Show authentication status
```

### Browser Sessions

```bash
notte sessions list [--page N] [--page-size N] [--only-active]  # List sessions
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
  --browser-type chromium|chrome|firefox  # Browser type (default: chromium)
  --headless                              # Run in headless mode (default: true)
  --idle-timeout-minutes <minutes>        # Idle timeout (closes after inactivity)
  --max-duration-minutes <minutes>        # Maximum session lifetime
  --user-agent <string>                   # Custom user agent
  --viewport-width <pixels>               # Viewport width
  --viewport-height <pixels>              # Viewport height
  --proxies                               # Use default proxy rotation
  --solve-captchas                        # Automatically solve captchas
  --use-file-storage                      # Enable file storage for downloads
  --cdp-url <url>                         # CDP URL of remote session provider
  --profile-id <id>                       # Profile ID to use for session
  --profile-persist                       # Save browser state to profile on close
  --screenshot-type <type>                # Screenshot type (raw, full, last_action)
  --chrome-args <args>                    # Chrome instance arguments (repeatable)
```

### Page Actions

Interact with pages using simplified commands (requires an active session):

```bash
notte page observe                    # Get page state and available actions
notte page scrape --instructions "..." # Scrape content from the page 
notte page click "@B3"            # Click an element by ID
notte page fill "@I1" "text"    # Fill an input field
notte page goto "https://example.com" # Navigate to a URL
notte page back                       # Go back in history
notte page forward                    # Go forward in history
notte page scroll-down [amount]       # Scroll down the page
notte page scroll-up [amount]         # Scroll up
notte page press "Enter"              # Press a key
notte page screenshot                 # Take a screenshot
notte page select <id> "option"       # Select dropdown option
notte page check <id>                 # Check/uncheck checkbox
notte page upload <id> <file>         # Upload a file
notte page download <id>              # Download file by clicking element
notte page new-tab <url>              # Open URL in new tab
notte page switch-tab <index>         # Switch to tab by index
notte page close-tab                  # Close current tab
notte page reload                     # Reload page
notte page wait <seconds>             # Wait for duration
notte page captcha-solve              # Solve captcha
```

### AI Agents

```bash
notte agents list [--page N] [--page-size N] [--only-active] [--only-saved]  # List agents
notte agents start --task "..."       # Start a new AI agent (auto-uses current session)
notte agents status                   # Get agent status (uses current agent)
notte agents stop                     # Stop an agent (uses current agent)
notte agents workflow-code            # Get agent's workflow code
notte agents replay                   # Get agent execution replay
```

**Note:** When you start an agent, it automatically becomes the "current" agent. All subsequent commands use this agent by default. Use `--agent-id <agent-id>` only when you need to manage multiple agents. If a session is active, `agents start` will automatically use that session unless `--session-id` is specified.

### Functions

```bash
notte functions list [--page N] [--page-size N] [--only-active]  # List functions
notte functions create --file workflow.py  # Create a new function
notte functions show                  # View current function details
notte functions show --function-id <id>  # View specific function details (different from current function)
notte functions update --file workflow.py  # Update current function code
notte functions delete                # Delete current function
notte functions fork                  # Fork current function to new version
notte functions run                   # Execute current function
notte functions runs [--page N] [--page-size N] [--only-active]  # List runs for current function
notte functions run-stop --run-id <id>  # Stop a running function execution
notte functions run-metadata --run-id <id>  # Get run logs and results
notte functions schedule --cron "0 9 * * *"  # Schedule current function
notte functions unschedule            # Remove schedule from current function
```

**Note:** When you create a function, it automatically becomes the "current" function. All subsequent commands use this function by default. Use `--function-id <function-id>` only when you need to manage multiple functions simultaneously or reference a specific function.

### Vaults

```bash
notte vaults list [--page N] [--page-size N] [--only-active]  # List all vaults
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
notte personas list [--page N] [--page-size N] [--only-active]  # List all personas
notte personas create                    # Create a new persona
notte personas show --persona-id <id>    # View persona details
notte personas delete --persona-id <id>  # Delete a persona
notte personas emails --persona-id <id>  # List emails
notte personas sms --persona-id <id>     # List SMS messages
```

### Profiles

```bash
notte profiles list [--page N] [--page-size N] [--name "..."]  # List all profiles
notte profiles create                    # Create a new profile
notte profiles show --profile-id <id>    # View profile details
notte profiles delete --profile-id <id>  # Delete a profile
```

### Files

```bash
notte files list                     # List uploaded files
notte files upload <path>            # Upload a file
notte files download <id>            # Download a file by ID
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
notte sessions start --headless

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
  --viewport-height 1080 \
  --solve-captchas

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
notte sessions list --only-active

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
npx skills add nottelabs/notte-cli
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

- **Viewing headless sessions**: When you start a session, the output includes a `ViewerUrl` - open it to watch your headless browser live
- **Element selectors**: If element IDs from `observe` (like `@B1`) don't work, use Playwright selectors: `#id`, `.class`, `button:has-text('Submit')`
- **Multiple matches**: Use `>> nth=0` suffix to select the first match: `button:has-text('OK') >> nth=0`
- **Closing modals**: `notte page press "Escape"` reliably dismisses most dialogs

### Skills Documentation

For comprehensive documentation including templates and reference guides, see the [skills/notte-browser](skills/notte-browser/SKILL.md) folder.

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

MIT

## Links

- [Notte API Documentation](https://notte.cc/docs)