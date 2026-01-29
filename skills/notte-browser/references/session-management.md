# Session Management Reference

Complete guide to managing browser sessions with the notte CLI.

## Session Lifecycle

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   start     │ -> │   observe   │ -> │   execute   │ -> │    stop     │
│             │    │   /scrape   │    │   /page     │    │             │
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
notte sessions start --browser chromium

# Google Chrome
notte sessions start --browser chrome

# Firefox
notte sessions start --browser firefox
```

### Session Configuration

```bash
notte sessions start \
  --headless=false \              # Show browser window
  --browser chromium \            # Browser type
  --idle-timeout 10 \             # Close after 10 min of inactivity
  --max-duration 60 \             # Maximum 60 min session lifetime
  --proxies \                     # Use rotating proxies
  --solve-captchas \              # Auto-solve CAPTCHAs
  --viewport-width 1920 \         # Custom viewport
  --viewport-height 1080 \
  --user-agent "Custom UA" \      # Custom user agent
  --file-storage                  # Enable file storage for downloads
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
# Session ID saved to ~/.config/notte/current_session

# These commands use the current session automatically:
notte sessions observe
notte sessions scrape
notte page click "@B3"
notte sessions stop
```

### Explicit Session ID

```bash
# Via --id flag
notte sessions observe --id sess_abc123

# Via environment variable
export NOTTE_SESSION_ID=sess_abc123
notte sessions observe
```

### Priority Order

1. `--id` flag (highest)
2. `NOTTE_SESSION_ID` environment variable
3. Current session file (set by `sessions start`)

## Observing Page State

The `observe` command returns the current page state including available actions:

```bash
# Observe current page
notte sessions observe

# Navigate and observe
notte sessions observe --url "https://example.com"
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

### Via sessions execute (Raw JSON)

```bash
# Navigate
notte sessions execute --action '{"type": "goto", "url": "https://example.com"}'

# Click
notte sessions execute --action '{"type": "click", "id": "B3"}'

# Fill
notte sessions execute --action '{"type": "fill", "id": "B1", "value": "hello"}'

# From file
notte sessions execute --action @action.json

# From stdin
echo '{"type": "goto", "url": "https://example.com"}' | notte sessions execute
```

### Via page commands (Recommended)

The `page` commands provide a cleaner interface:

```bash
notte page goto "https://example.com"
notte page click "@B3"
notte page fill "@B1" "hello"
```

See the main SKILL.md for complete page command reference.

## Scraping Content

### Basic Scraping

```bash
# Scrape entire page
notte sessions scrape

# With extraction instructions
notte sessions scrape --instructions "Extract all product names and prices as JSON"

# Only main content (skip headers, footers, ads)
notte sessions scrape --only-main-content
```

### Structured Extraction

The `--instructions` parameter accepts natural language:

```bash
notte sessions scrape --instructions "Extract:
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
notte sessions start --idle-timeout 10
```

Activity includes any command: observe, execute, scrape, etc.

### Max Duration

Absolute maximum session lifetime:

```bash
# Session closes after 60 minutes regardless of activity
notte sessions start --max-duration 60
```

### Combining Timeouts

```bash
# Close after 10 min idle OR 60 min total, whichever comes first
notte sessions start --idle-timeout 10 --max-duration 60
```

## Debugging Sessions

### Debug Info

Get detailed session state:

```bash
notte sessions debug
```

Returns browser state, memory usage, active tabs, etc.

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

### Workflow Code Export

Export session steps as reusable code:

```bash
notte sessions workflow-code
```

Generates a workflow script from your session actions.

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
notte sessions list
```

## Stopping Sessions

```bash
# Stop current session
notte sessions stop

# Stop specific session
notte sessions stop --id sess_abc123

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
notte sessions start --idle-timeout 2 --max-duration 5

# Long task (data collection)
notte sessions start --idle-timeout 15 --max-duration 120
```

### 3. Observe Before Acting

Always observe to get current element IDs:

```bash
notte sessions observe --url "https://example.com"
# Now you know the element IDs
notte page click "@B3"
```

### 4. Use JSON Output for Scripts

```bash
# Parse response in scripts
RESULT=$(notte sessions observe -o json)
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
