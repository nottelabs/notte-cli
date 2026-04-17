# Troubleshooting & Advanced Tips

Consult this reference when the top-level SKILL.md flags an issue here
(inconsistent observe, modals, bot detection, headless viewing).

## Handling Inconsistent `observe` Output

The `observe` command may sometimes return stale or partial DOM state, especially with dynamic content, modals, or single-page applications. If the output seems wrong:

1. **Use screenshots to verify**: `notte page screenshot` always shows the current visual state
2. **Fall back to Playwright selectors**: Instead of `@ID` references, use standard selectors like `#id`, `.class`, or `button:has-text('Submit')`
3. **Add a brief wait**: `notte page wait 500` before observing can help with dynamic content

## Selector Syntax

Both element IDs from `observe` and Playwright selectors are supported:

```bash
# Using element IDs from observe output
notte page click "@B3"
notte page fill "@I1" "text"

# Using Playwright selectors (recommended when @IDs don't work)
notte page click "#submit-button"
notte page click ".btn-primary"
notte page click "button:has-text('Submit')"
notte page click "[data-testid='login']"
notte page fill "input[name='email']" "user@example.com"
```

**Handling multiple matches** — use `>> nth=0` to select the first match:

```bash
notte page click "button:has-text('OK') >> nth=0"
notte page click ".submit-btn >> nth=0"
```

## Working with Modals and Dialogs

- **Close modals with Escape**: `notte page press "Escape"` reliably dismisses most dialogs and modals
- **Wait after modal actions**: Add `notte page wait 500` after closing a modal before the next action
- **Check for overlays**: If clicks aren't working, a modal or overlay might be blocking — use screenshot to verify

```bash
# Common pattern for handling unexpected modals
notte page press "Escape"
notte page wait 500
notte page click "#target-element"
```

## Viewing Headless Sessions

Running with `--headless` (the default) doesn't mean you can't see the browser:

- **ViewerUrl**: When you start a session, the output includes a `ViewerUrl` — open it in your browser to watch the session live
- **Viewer command**: `notte sessions viewer` opens the viewer directly
- **Non-headless mode**: Use `--headless=false` only if you need a local browser window (not available on remote/CI environments)

```bash
# Start headless session and get viewer URL
notte sessions start -o json | jq -r '.viewer_url'

# Or open viewer for current session
notte sessions viewer
```

## Bot Detection / Stealth

If you're getting blocked or seeing CAPTCHAs, try these approaches (requires restarting the session with new parameters):

1. **Change browser type**:
   ```bash
   notte sessions stop
   notte sessions start --browser-type firefox
   ```

2. **Enable proxies**:
   ```bash
   notte sessions stop
   notte sessions start --proxies
   ```

3. **Firefox + CAPTCHA solving** (works best with Firefox):
   ```bash
   notte sessions stop
   notte sessions start --browser-type firefox --solve-captchas
   ```

4. **Combine strategies**:
   ```bash
   notte sessions stop
   notte sessions start --browser-type firefox --proxies --solve-captchas
   ```

**Note**: Always stop the current session before starting a new one with different parameters. Session configuration cannot be changed mid-session.
