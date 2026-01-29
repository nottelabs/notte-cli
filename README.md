# 🤖 Notte CLI — Browser automation in your terminal.

Control browser sessions, AI agents, and web scraping through intuitive resource-based commands.

## Features

- **AI agents** - run and monitor AI-powered browser workflows
- **Browser sessions** - headless or headed Chrome/Firefox with full control
- **Files** - upload and download files to notte.cc
- **Output formats** - human-readable text or JSON for scripting
- **Personas** - create and manage digital identities with email, phone, and SMS
- **Secure credentials** - system keyring for API keys, vaults for website passwords
- **Web scraping** - structured data extraction with custom schemas
- **Workflows** - schedule and execute repeatable automation tasks

## Installation

### Homebrew

```bash
brew install salmonumbrella/tap/notte-cli
```

### Go Install

```bash
go install github.com/salmonumbrella/notte-cli/cmd/notte@latest
```

### Build from Source

```bash
git clone https://github.com/salmonumbrella/notte-cli.git
cd notte-cli
make build
```

## Quick Start

### 1. Authenticate

```bash
notte auth login
# Enter your notte.cc API key when prompted
```

### 2. Test Authentication

```bash
notte auth status
```

### 3. Start a Browser Session

```bash
notte sessions start --headless
```

### 4. Scrape a Page

```bash
notte scrape https://news.ycombinator.com --instructions "Extract the top stories"
```

## Configuration

### API Key Storage

Specify the API key using one of three methods (checked in priority order):

```bash
# Via environment variable (recommended for CI/CD)
export NOTTE_API_KEY="your-api-key"
notte sessions list

# Via system keyring (recommended for local development)
notte auth login

# Via config file (~/.config/notte/config.json)
```

### Environment Variables

- `NOTTE_API_KEY` - API key for authentication
- `NOTTE_API_URL` - Override API endpoint (default: https://api.notte.cc)

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

## Commands

### Authentication

```bash
notte auth login                     # Store API key in system keychain
notte auth logout                    # Remove API key from keychain
notte auth status                    # Show authentication status
```

### Browser Sessions

```bash
notte sessions list                  # List all active sessions
notte sessions start [flags]         # Start a new session
notte session status --id <id>       # Get session status
notte session stop --id <id>         # Stop a session
notte session observe --id <id>      # Watch session in real-time
notte session execute --id <id>      # Execute browser actions
notte session scrape --id <id>       # Scrape content from current page
notte session cookies --id <id>      # Get all cookies
notte session cookies-set --id <id>  # Set cookies from JSON file
notte session network --id <id>      # View network activity logs
notte session debug --id <id>        # Get debug information
notte session replay --id <id>       # Get session replay data
```

#### Session Start Options

```bash
notte sessions start \
  --browser chromium|chrome|firefox  # Browser type (default: chromium)
  --headless                         # Run in headless mode (default: true)
  --timeout <minutes>                # Session timeout 1-15 min (default: 3)
  --user-agent <string>              # Custom user agent
  --viewport-width <pixels>          # Viewport width
  --viewport-height <pixels>         # Viewport height
  --proxies                          # Use default proxy rotation
  --solve-captchas                   # Automatically solve captchas
  --cdp-url <url>                    # CDP URL of remote session provider
```

### AI Agents

```bash
notte agents list                    # List all AI agents
notte agents start                   # Start a new AI agent
notte agent status --id <id>         # Get agent status
notte agent stop --id <id>           # Stop an agent
notte agent workflow-code --id <id>  # Get agent's workflow code
notte agent replay --id <id>         # Get agent execution replay
```

### Workflows

```bash
notte workflows list                 # List all workflows
notte workflows create               # Create a new workflow
notte workflow show --id <id>        # View workflow details
notte workflow update --id <id>      # Update workflow configuration
notte workflow delete --id <id>      # Delete a workflow
notte workflow fork --id <id>        # Fork workflow to new version
notte workflow run --id <id>         # Execute workflow
notte workflow runs --id <id>        # List workflow runs
notte workflow run-stop --id <id>    # Stop a running workflow
notte workflow schedule --id <id>    # Schedule recurring execution
notte workflow unschedule --id <id>  # Remove schedule
```

### Vaults

```bash
notte vaults list                              # List all vaults
notte vaults create                            # Create a new vault
notte vault update --id <id>                   # Update vault metadata
notte vault delete --id <id>                   # Delete a vault
notte vault credentials list --id <id>         # List all credentials
notte vault credentials add --id <id>          # Add credentials
notte vault credentials get --id <id>          # Get credentials for URL
notte vault credentials delete --id <id>       # Delete credentials
notte vault card --id <id>                     # Manage payment cards
```

### Personas

```bash
notte personas list                  # List all personas
notte personas create                # Create a new persona
notte persona show --id <id>         # View persona details
notte persona delete --id <id>       # Delete a persona
notte persona emails --id <id>       # Manage email addresses
notte persona sms --id <id>          # Manage SMS numbers
notte persona phone --id <id>        # Manage phone numbers
```

### Files

```bash
notte files list                     # List uploaded files
notte files upload <path>            # Upload a file
notte files download <id>            # Download a file by ID
```

### Web Scraping

```bash
notte scrape <url> [flags]           # Scrape with structured extraction
notte scrape-html <url>              # Get raw HTML content

# Scraping options
notte scrape <url> \
  --instructions <text>              # Extraction instructions
  --only-main-content                # Extract only main content area
```

### Usage & Monitoring

```bash
notte usage                          # View API usage statistics
notte usage logs                     # View detailed usage logs
```

### Utilities

```bash
notte health                         # Check API health status
notte prompt-improve                 # Improve a prompt with AI
notte prompt-nudge                   # Get prompt optimization suggestions
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
# Start session, navigate, scrape, and cleanup
SESSION_ID=$(notte sessions start --headless -o json | jq -r '.id')

# Navigate to page (stdin also supported: --action @file.json or --action -)
notte session execute --id $SESSION_ID << 'EOF'
{"type": "goto", "url": "https://news.ycombinator.com"}
EOF

# Extract structured data
notte session scrape --id $SESSION_ID \
  --instructions "Extract top 10 stories with title and URL"

# Cleanup
notte session stop --id $SESSION_ID
```

### Running a Workflow

```bash
# List workflows to find ID
notte workflows list

# Run workflow
notte workflow run --id wfl_abc123
```

### Managing Credentials Securely

```bash
# Create a vault for production credentials
VAULT_ID=$(notte vaults create --name "Production Sites" -o json | jq -r '.id')

# Add website credentials
notte vault credentials add --id $VAULT_ID \
  --username "admin@example.com" \
  --password "$SECURE_PASSWORD" \
  --url "https://app.example.com"

# List stored credentials
notte vault credentials list --id $VAULT_ID
```

### Multi-Step Browser Automation

```bash
# Start browser with specific configuration
SESSION_ID=$(notte sessions start \
  --browser chrome \
  --viewport-width 1920 \
  --viewport-height 1080 \
  --solve-captchas \
  -o json | jq -r '.id')

# Execute multiple actions
notte session execute --id $SESSION_ID '{"type": "goto", "url": "https://example.com"}'
notte session execute --id $SESSION_ID '{"type": "click", "selector": "#login-button"}'
notte session execute --id $SESSION_ID '{"type": "form_fill", "selector": "#username", "text": "user@example.com"}'

# Get current page state
notte session observe --id $SESSION_ID

# Stop when done
notte session stop --id $SESSION_ID
```

### JQ Filtering

```bash
# Get only active sessions
notte sessions list --output json | jq '.sessions[] | select(.status=="ACTIVE")'

# Extract session IDs
notte sessions list --output json | jq -r '.sessions[].id'
```

### Advanced Usage

#### Heredoc for Complex JSON

For multi-line JSON payloads, use heredoc syntax:

```bash
# Execute a complex action with heredoc
notte session execute --id $SESSION_ID --action - << 'EOF'
{
  "action": "fill_form",
  "fields": [
    {"selector": "#name", "value": "John Doe"},
    {"selector": "#email", "value": "john@example.com"},
    {"selector": "#message", "value": "Hello,\nThis is a multi-line message."}
  ]
}
EOF

# Update workflow metadata with heredoc
notte workflow run-metadata-update --id $WORKFLOW_ID --run-id $RUN_ID --data - << 'EOF'
{
  "status": "processing",
  "progress": 75,
  "results": {
    "items_processed": 150,
    "errors": []
  }
}
EOF
```

## Usage with AI Agents

### Just Ask the Agent

The simplest approach - just tell your agent to use it:

> Use notte to test the login flow. Run `notte --help` to see available commands.

The `--help` output is comprehensive and most agents can figure it out from there.

### AI Coding Assistants

Add the skill to your AI coding assistant for richer context:

```bash
npx @anthropic-ai/claude-code-mcp add nottelabs/notte-cli
```

This works with Claude Code, Cursor, Windsurf, and other MCP-compatible assistants.

### AGENTS.md / CLAUDE.md

For more consistent results, add to your project or global instructions file:

```markdown
## Browser Automation

Use `notte` for web automation. Run `notte --help` for all commands.

Core workflow:
1. `notte sessions start` - Start a browser session
2. `notte sessions observe --url <url>` - Navigate and get interactive elements with IDs (@B1, @B2)
3. `notte page click @B1` / `notte page fill @B2 "text"` - Interact using element IDs
4. `notte sessions scrape --instructions "..."` - Extract structured data
5. `notte sessions stop` - Clean up when done
```

### Skills Documentation

For comprehensive documentation including templates and reference guides, see the [skills/notte-browser](skills/notte-browser/SKILL.md) folder.

## Global Flags

All commands support these flags:

- `-o, --output <format>` - Output format: `text` or `json` (default: text)
- `--no-color` - Disable colored output
- `-v, --verbose` - Enable verbose logging
- `--timeout <seconds>` - API request timeout (default: 30)
- `-h, --help` - Show help for any command

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
