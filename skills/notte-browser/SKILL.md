# Notte Browser CLI Skill

Command-line interface for browser automation, web scraping, and AI-powered web interactions using the notte.cc platform.

## Quick Start

```bash
# 1. Authenticate
notte auth login

# 2. Start a browser session
notte sessions start

# 3. Navigate and observe
notte sessions observe --url "https://example.com"

# 4. Execute actions
notte page click "@B3"
notte page fill "@input" "hello world"

# 5. Scrape content
notte sessions scrape --instructions "Extract all product names and prices"

# 6. Stop the session
notte sessions stop
```

## Command Categories

### Session Management

Control browser session lifecycle:

```bash
# Start a new session
notte sessions start [flags]
  --headless           Run in headless mode (default: true)
  --browser            Browser type: chromium, chrome, firefox (default: chromium)
  --idle-timeout       Idle timeout in minutes
  --max-duration       Maximum session lifetime in minutes
  --proxies            Use default proxies
  --solve-captchas     Automatically solve captchas
  --viewport-width     Viewport width in pixels
  --viewport-height    Viewport height in pixels
  --user-agent         Custom user agent string
  --cdp-url            CDP URL of remote session provider
  --file-storage       Enable file storage for the session

# List active sessions
notte sessions list

# Get session status
notte sessions status [--id <session-id>]

# Observe page state and available actions
notte sessions observe [--id <session-id>] [--url <url>]

# Execute an action (raw JSON)
notte sessions execute --action '{"type": "goto", "url": "https://example.com"}'

# Scrape content from current page
notte sessions scrape [--instructions "..."] [--only-main-content]

# Stop a session
notte sessions stop [--id <session-id>]
```

Session debugging and export:

```bash
# Get debug info
notte sessions debug [--id <session-id>]

# Get network logs
notte sessions network [--id <session-id>]

# Get replay URL/data
notte sessions replay [--id <session-id>]

# Export session steps as workflow code
notte sessions workflow-code [--id <session-id>]
```

Cookie management:

```bash
# Get all cookies
notte sessions cookies [--id <session-id>]

# Set cookies from JSON file
notte sessions cookies-set --file cookies.json [--id <session-id>]
```

### Page Actions

Simplified commands for page interactions (syntactic sugar for `sessions execute`):

**Element Interactions:**
```bash
# Click an element (use @ prefix for element IDs from observe)
notte page click "@B3"
notte page click "#submit-button"
  --timeout     Timeout in milliseconds
  --enter       Press Enter after clicking

# Fill an input field
notte page fill "@input" "hello world"
  --clear       Clear field before filling
  --enter       Press Enter after filling

# Check/uncheck a checkbox
notte page check "@checkbox"
  --value       true to check, false to uncheck (default: true)

# Select dropdown option
notte page select "@dropdown" "Option 1"

# Download file by clicking element
notte page download "@download-link"

# Upload file to input
notte page upload "@file-input" --file /path/to/file
```

**Navigation:**
```bash
notte page goto "https://example.com"
notte page new-tab "https://example.com"
notte page back
notte page forward
notte page reload
```

**Scrolling:**
```bash
notte page scroll-down [amount]
notte page scroll-up [amount]
```

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

**Utilities:**
```bash
# Wait for specified duration
notte page wait 1000

# Scrape with instructions
notte page scrape "Extract all links" [--main-only]

# Solve CAPTCHA
notte page captcha-solve "recaptcha_v2"

# Mark task complete
notte page complete "Task finished successfully" [--success=true]

# Fill form with JSON data
notte page form-fill --data '{"email": "test@example.com", "name": "John"}'
```

### Quick Scrape

Scrape without creating a session:

```bash
# Scrape a URL directly
notte scrape "https://example.com" [--instructions "..."] [--only-main-content]

# Scrape from local HTML file
notte scrape-html --file page.html [--instructions "..."]
```

### Functions (Workflow Automation)

Create, manage, and schedule reusable workflows:

```bash
# List all functions
notte functions list

# Create a function from a workflow file
notte functions create --file workflow.py [--name "My Function"] [--description "..."] [--shared]

# Show function details
notte functions show --id <function-id>

# Update function code
notte functions update --id <function-id> --file workflow.py

# Delete a function
notte functions delete --id <function-id>

# Run a function
notte functions run --id <function-id>

# List function runs
notte functions runs --id <function-id>

# Stop a running function
notte functions run-stop --id <function-id> --run-id <run-id>

# Get/update run metadata
notte functions run-metadata --id <function-id> --run-id <run-id>
notte functions run-metadata-update --id <function-id> --run-id <run-id> --data '{"key": "value"}'

# Schedule with cron expression
notte functions schedule --id <function-id> --cron "0 9 * * *"

# Remove schedule
notte functions unschedule --id <function-id>

# Fork a shared function
notte functions fork --id <function-id>
```

### Account Management

**Personas** - Auto-generated identities with email/phone:

```bash
# List personas
notte personas list

# Create a persona
notte personas create [--create-phone-number] [--create-vault]

# Show persona details
notte personas show --id <persona-id>

# Delete a persona
notte personas delete --id <persona-id>

# List emails received by persona
notte personas emails --id <persona-id>

# List SMS messages received
notte personas sms --id <persona-id>

# Manage phone numbers
notte personas phone-create --id <persona-id>
notte personas phone-delete --id <persona-id>
```

**Vaults** - Store your own credentials:

```bash
# List vaults
notte vaults list

# Create a vault
notte vaults create [--name "My Vault"]

# Update vault name
notte vaults update --id <vault-id> --name "New Name"

# Delete a vault
notte vaults delete --id <vault-id>

# Manage credentials
notte vaults credentials list --id <vault-id>
notte vaults credentials add --id <vault-id> --url "https://site.com" --password "pass" [--email "..."] [--username "..."] [--mfa-secret "..."]
notte vaults credentials get --id <vault-id> --url "https://site.com"
notte vaults credentials delete --id <vault-id> --url "https://site.com"

# Manage credit card
notte vaults card --id <vault-id>
notte vaults card-set --id <vault-id> --number "..." --expiry "12/25" --cvv "..." --name "John Doe"
notte vaults card-delete --id <vault-id>
```

## Global Options

Available on all commands:

```bash
--output, -o    Output format: text, json (default: text)
--timeout       API request timeout in seconds (default: 30)
--no-color      Disable color output
--verbose, -v   Verbose output
--yes, -y       Skip confirmation prompts
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NOTTE_API_KEY` | API key for authentication |
| `NOTTE_SESSION_ID` | Default session ID (avoids --id flag) |
| `NOTTE_API_URL` | Custom API endpoint URL |

## Session ID Resolution

Session ID is resolved in this order:
1. `--id` flag
2. `NOTTE_SESSION_ID` environment variable
3. Current session file (set automatically by `sessions start`)

## Examples

### Basic Web Scraping

```bash
# Quick scrape
notte scrape "https://news.ycombinator.com" --instructions "Extract top 10 story titles"

# With session for multi-page scraping
notte sessions start --headless
notte sessions observe --url "https://example.com/products"
notte sessions scrape --instructions "Extract product names and prices"
notte page click "@next-page"
notte sessions scrape --instructions "Extract product names and prices"
notte sessions stop
```

### Form Automation

```bash
notte sessions start
notte page goto "https://example.com/signup"
notte page fill "@email" "user@example.com"
notte page fill "@password" "securepassword"
notte page click "@submit"
notte sessions stop
```

### Authenticated Session with Vault

```bash
# Setup credentials once
notte vaults create --name "MyService"
notte vaults credentials add --id <vault-id> \
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
notte functions schedule --id <function-id> --cron "0 9 * * *"

# Check run history
notte functions runs --id <function-id>
```

## Additional Resources

- [Session Management Reference](references/session-management.md) - Detailed session lifecycle guide
- [Function Management Reference](references/function-management.md) - Workflow automation guide
- [Account Management Reference](references/account-management.md) - Personas and vaults guide

### Templates

Ready-to-use shell script templates:

- [Form Automation](templates/form-automation.sh) - Fill and submit forms
- [Authenticated Session](templates/authenticated-session.sh) - Login with credential vault
- [Data Extraction](templates/data-extraction.sh) - Scrape structured data
