---
name: session-management
description: Complete guide to managing browser sessions with the notte CLI
---

# Session Management Reference

Complete guide to managing browser sessions with the notte CLI.

## Session Lifecycle

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   start     │ -> │   observe   │ -> │    page     │ -> │    stop     │
│  sessions   │    │   (page)    │    │  commands   │    │  sessions   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## Starting Sessions

### Basic Start

```bash
# Start with defaults (headless chromium)
notte sessions start

# Start with visible browser
notte sessions start --headless=false
```

### Browser Selection

```bash
# Chromium (default)
notte sessions start --browser-type chromium

# Google Chrome
notte sessions start --browser-type chrome

# Firefox
notte sessions start --browser-type firefox
```

### Session Configuration

```bash
notte sessions start \
  --headless=false \              # Show browser window
  --browser-type chromium \       # Browser type
  --idle-timeout-minutes 10 \     # Close after 10 min of inactivity
  --max-duration-minutes 60 \     # Maximum 60 min session lifetime
  --proxy \                       # Use rotating proxies
  --solve-captchas \              # Auto-solve CAPTCHAs
  --viewport-width 1920 \         # Custom viewport
  --viewport-height 1080 \
  --user-agent "Custom UA" \      # Custom user agent
  --use-file-storage              # Enable file storage for downloads
```

### Remote Browser Connection

Connect to an external browser via CDP (Chrome DevTools Protocol):

```bash
notte sessions start --cdp-url "ws://localhost:9222/devtools/browser/..."
```

## Session ID Management

### Current Session

When you start a session, it becomes the "current session" automatically:

```bash
notte sessions start
# Session ID saved to ~/.notte/cli/current_session

# These commands use the current session automatically:
notte page observe
notte page click "@B3"
notte page scrape
notte sessions stop
```

### Explicit Session ID

```bash
# Via --session-id flag
notte page observe --session-id sess_abc123

# Via environment variable
export NOTTE_SESSION_ID=sess_abc123
notte page observe
```

### Priority Order

1. `--session-id` flag (highest)
2. `NOTTE_SESSION_ID` environment variable
3. Current session file (set by `sessions start`)

## Observing Page State

The `observe` command returns the current page state including available actions:

```bash
# Observe current page
notte page observe

# Navigate and observe
notte page observe https://example.com
```

### Observe Response

The response includes:
- **url**: Current page URL
- **title**: Page title
- **actions**: Available interactive elements with IDs

Example response (JSON output):
```json
{
  "url": "https://example.com/login",
  "title": "Login - Example",
  "actions": [
    {"id": "B1", "type": "input", "description": "Email input field"},
    {"id": "B2", "type": "input", "description": "Password input field"},
    {"id": "B3", "type": "button", "description": "Login button"}
  ]
}
```

Use these IDs with the `@` prefix in page commands:
```bash
notte page fill "@B1" "user@example.com"
notte page fill "@B2" "password"
notte page click "@B3"
```

## Executing Actions

Use the `page` commands for interacting with the browser:

```bash
# Navigate
notte page goto "https://example.com"

# Click
notte page click "@B3"

# Fill
notte page fill "@B1" "hello"

# Select dropdown
notte page select "@dropdown" "Option 1"

# Press key
notte page press "Enter"
```

See the main SKILL.md for complete page command reference.

## Scraping Content

### Basic Scraping

```bash
# Scrape entire page
notte page scrape

# With extraction instructions
notte page scrape --instructions "Extract all product names and prices as JSON"

# Only main content (skip headers, footers, ads)
notte page scrape --only-main-content
```

### Structured Extraction

Extraction instructions accept natural language:

```bash
notte page scrape --instructions "Extract:
- Article title
- Author name
- Publication date
- Main content (first 500 words)"
```

## Session Timeouts

### Idle Timeout

Session closes after period of inactivity:

```bash
# Close after 10 minutes of no activity
notte sessions start --idle-timeout-minutes 10
```

Activity includes any command: observe, execute, scrape, etc.

### Max Duration

Absolute maximum session lifetime:

```bash
# Session closes after 60 minutes regardless of activity
notte sessions start --max-duration-minutes 60
```

### Combining Timeouts

```bash
# Close after 10 min idle OR 60 min total, whichever comes first
notte sessions start --idle-timeout-minutes 10 --max-duration-minutes 60
```

### Network Logs

View all network requests:

```bash
notte sessions network
```

Useful for debugging API calls, failed requests, etc.

### Session Replay

Get replay data for session recording:

```bash
notte sessions replay
```

Returns data that can be used to replay the session.

### Export Code

Export session steps as reusable code:

```bash
notte sessions workflow-code
```

Generates a function script from your session actions.

## Cookie Management

### Get Cookies

```bash
notte sessions cookies
```

Returns all cookies for the current session.

### Set Cookies

Restore cookies from a previous session:

```bash
# cookies.json format:
# [{"name": "session", "value": "abc123", "domain": ".example.com", ...}]

notte sessions cookies-set --file cookies.json
```

### Cookie Persistence Pattern

```bash
# Save cookies after login
notte sessions cookies -o json > cookies.json

# Restore in new session
notte sessions start
notte sessions cookies-set --file cookies.json
notte page goto "https://example.com/dashboard"  # Already logged in
```

## Session Status

Check if session is still active:

```bash
notte sessions status
```

### List All Sessions

```bash
# List all sessions
notte sessions list

# With pagination and filters
notte sessions list --page 2 --page-size 10 --only-active
```

## Stopping Sessions

```bash
# Stop current session
notte sessions stop

# Stop specific session
notte sessions stop --session-id sess_abc123

# Skip confirmation prompt
notte sessions stop --yes
```

## Best Practices

### 1. Always Stop Sessions

Sessions consume resources. Always stop when done:

```bash
# In scripts, use trap for cleanup
trap 'notte sessions stop --yes 2>/dev/null' EXIT
```

### 2. Use Appropriate Timeouts

Set timeouts based on your use case:

```bash
# Short task (login check)
notte sessions start --idle-timeout-minutes 2 --max-duration-minutes 5

# Long task (data collection)
notte sessions start --idle-timeout-minutes 15 --max-duration-minutes 120
```

### 3. Observe Before Acting

Always observe to get current element IDs:

```bash
notte page goto "https://example.com"
notte page observe
# Now you know the element IDs
notte page click "@B3"
```

### 4. Use JSON Output for Scripts

```bash
# Parse response in scripts
RESULT=$(notte page observe -o json)
URL=$(echo "$RESULT" | jq -r '.url')
```

### 5. Handle Errors Gracefully

```bash
if ! notte page click "@submit"; then
  echo "Click failed, retrying..."
  notte page wait 1000
  notte page click "@submit"
fi
```
