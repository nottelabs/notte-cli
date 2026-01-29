#!/bin/bash
# Authenticated Session Template
# Login to a website using vault credentials with optional MFA support
#
# Usage: ./authenticated-session.sh
#
# Prerequisites:
#   - notte CLI installed and authenticated (notte auth login)
#   - Vault created with credentials for the target site:
#     notte vaults create --name "MyVault"
#     notte vaults credentials add --id <vault-id> \
#       --url "https://example.com" \
#       --email "user@example.com" \
#       --password "password" \
#       --mfa-secret "TOTP_SECRET"  # Optional

set -euo pipefail

# Configuration - customize these for your site
LOGIN_URL="https://example.com/login"
DASHBOARD_URL="https://example.com/dashboard"  # URL after successful login
VAULT_ID="${NOTTE_VAULT_ID:-}"  # Set via env or edit here

# Login form selectors (use @ID from observe or CSS selectors)
EMAIL_SELECTOR="@email"
PASSWORD_SELECTOR="@password"
SUBMIT_SELECTOR="@login-button"
MFA_SELECTOR="@mfa-code"  # Optional: selector for MFA input

# Credentials - leave empty to use vault auto-fill
EMAIL=""
PASSWORD=""

# Cookie persistence
SAVE_COOKIES=true
COOKIES_FILE="./session_cookies.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }

cleanup() {
    if [[ "${KEEP_SESSION:-false}" != "true" ]]; then
        log_info "Stopping session..."
        notte sessions stop --yes 2>/dev/null || true
    else
        log_info "Keeping session alive (KEEP_SESSION=true)"
    fi
}

trap cleanup EXIT

check_login_success() {
    local current_url
    current_url=$(notte sessions observe -o json | jq -r '.url // empty')

    if [[ "$current_url" == *"dashboard"* ]] || [[ "$current_url" == *"home"* ]]; then
        return 0
    fi

    # Check for common login failure indicators
    local page_content
    page_content=$(notte sessions scrape --instructions "Check if there are any error messages about login failure" 2>/dev/null || echo "")

    if echo "$page_content" | grep -qiE "(invalid|incorrect|failed|error|wrong)"; then
        return 1
    fi

    return 0
}

load_credentials_from_vault() {
    if [[ -z "$VAULT_ID" ]]; then
        log_warn "No VAULT_ID set, skipping vault credential lookup"
        return 1
    fi

    log_step "Loading credentials from vault..."
    local creds
    creds=$(notte vaults credentials get --id "$VAULT_ID" --url "$LOGIN_URL" -o json 2>/dev/null || echo "{}")

    EMAIL=$(echo "$creds" | jq -r '.email // empty')
    PASSWORD=$(echo "$creds" | jq -r '.password // empty')

    if [[ -n "$EMAIL" && -n "$PASSWORD" ]]; then
        log_info "Credentials loaded from vault"
        return 0
    else
        log_warn "No credentials found in vault for $LOGIN_URL"
        return 1
    fi
}

restore_cookies() {
    if [[ -f "$COOKIES_FILE" ]]; then
        log_step "Restoring saved cookies..."
        if notte sessions cookies-set --file "$COOKIES_FILE" 2>/dev/null; then
            log_info "Cookies restored"
            return 0
        fi
    fi
    return 1
}

save_cookies() {
    if [[ "$SAVE_COOKIES" == "true" ]]; then
        log_step "Saving session cookies..."
        notte sessions cookies -o json > "$COOKIES_FILE"
        log_info "Cookies saved to $COOKIES_FILE"
    fi
}

perform_login() {
    log_step "Navigating to login page..."
    notte sessions observe --url "$LOGIN_URL" > /dev/null
    notte page wait 1000

    # Fill email/username
    if [[ -n "$EMAIL" ]]; then
        log_info "Filling email: ${EMAIL:0:3}***"
        notte page fill "$EMAIL_SELECTOR" "$EMAIL"
        notte page wait 300
    fi

    # Fill password
    if [[ -n "$PASSWORD" ]]; then
        log_info "Filling password: ****"
        notte page fill "$PASSWORD_SELECTOR" "$PASSWORD"
        notte page wait 300
    fi

    # Submit login form
    log_step "Submitting login form..."
    notte page click "$SUBMIT_SELECTOR"
    notte page wait 2000

    # Check for MFA prompt
    local observe_result
    observe_result=$(notte sessions observe -o json)

    if echo "$observe_result" | grep -qiE "(mfa|two.?factor|verification|authenticator|2fa)"; then
        log_step "MFA detected, handling..."
        handle_mfa
    fi
}

handle_mfa() {
    # If vault has MFA secret, TOTP should be auto-generated
    # This is a placeholder for manual handling if needed

    log_info "Waiting for MFA auto-fill from vault..."
    log_info "MFA input selector: $MFA_SELECTOR"
    notte page wait 3000

    # Check if still on MFA page
    local current_url
    current_url=$(notte sessions observe -o json | jq -r '.url // empty')

    if echo "$current_url" | grep -qiE "(mfa|verify|2fa)"; then
        log_warn "MFA may require manual intervention"
        log_warn "If vault has --mfa-secret, TOTP should auto-fill to $MFA_SELECTOR"
    fi
}

main() {
    log_info "=== Authenticated Session ==="
    log_info "Target: $LOGIN_URL"

    # Load credentials if not set
    if [[ -z "$EMAIL" || -z "$PASSWORD" ]]; then
        load_credentials_from_vault || true
    fi

    # Validate we have credentials
    if [[ -z "$EMAIL" || -z "$PASSWORD" ]]; then
        log_error "No credentials available. Set EMAIL/PASSWORD or configure vault."
        log_info "To set up vault:"
        log_info "  notte vaults create --name 'MyVault'"
        log_info "  notte vaults credentials add --id <vault-id> \\"
        log_info "    --url '$LOGIN_URL' \\"
        log_info "    --email 'your@email.com' \\"
        log_info "    --password 'yourpassword'"
        exit 1
    fi

    # Start session
    log_step "Starting browser session..."
    notte sessions start > /dev/null
    log_info "Session started"

    # Try to restore cookies first (skip login if still valid)
    if restore_cookies; then
        log_step "Checking if session is still valid..."
        notte page goto "$DASHBOARD_URL"
        notte page wait 2000

        if check_login_success; then
            log_info "Session restored from cookies!"
            save_cookies  # Refresh cookies
            log_info "=== Login successful (from cookies) ==="
            return 0
        else
            log_warn "Saved session expired, performing fresh login..."
        fi
    fi

    # Perform login
    perform_login

    # Verify login success
    if check_login_success; then
        log_info "=== Login successful ==="
        save_cookies
    else
        log_error "=== Login failed ==="
        exit 1
    fi

    # Navigate to dashboard
    log_step "Navigating to dashboard..."
    notte page goto "$DASHBOARD_URL"
    notte page wait 1000

    log_info "Ready for authenticated actions"
    log_info "Session ID: $(notte sessions status -o json | jq -r '.session_id // .sessionId // .id')"

    # Example: Scrape data from authenticated page
    # notte sessions scrape --instructions "Extract user profile information"
}

main "$@"
