#!/bin/bash
# Form Automation Template
# Fill and submit forms with the notte CLI
#
# Usage: ./form-automation.sh
#
# Prerequisites:
#   - notte CLI installed and authenticated (notte auth login)
#   - NOTTE_API_KEY environment variable set
#
# Customize the variables below for your form

set -euo pipefail

# Configuration - customize these for your form
TARGET_URL="https://example.com/contact"
FORM_DATA=(
    # Format: "selector|value"
    # Use @ID for element IDs from observe, or CSS selectors
    "@name|John Doe"
    "@email|john@example.com"
    "@message|Hello, this is a test message."
)
SUBMIT_SELECTOR="@submit"
SUCCESS_INDICATOR="Thank you"  # Text that appears on success

# Optional: Screenshot settings
TAKE_SCREENSHOTS=true
SCREENSHOT_DIR="./screenshots"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log_info "Cleaning up..."
    notte sessions stop --yes 2>/dev/null || true
}

# Ensure cleanup on exit
trap cleanup EXIT

main() {
    log_info "Starting form automation"

    # Create screenshot directory if needed
    if [[ "$TAKE_SCREENSHOTS" == "true" ]]; then
        mkdir -p "$SCREENSHOT_DIR"
    fi

    # Start browser session
    log_info "Starting browser session..."
    SESSION_RESULT=$(notte sessions start -o json)
    SESSION_ID=$(echo "$SESSION_RESULT" | jq -r '.session_id // .sessionId // .id')

    if [[ -z "$SESSION_ID" || "$SESSION_ID" == "null" ]]; then
        log_error "Failed to start session"
        exit 1
    fi
    log_info "Session started: $SESSION_ID"

    # Navigate to form page
    log_info "Navigating to: $TARGET_URL"
    notte sessions observe --url "$TARGET_URL" > /dev/null

    # Wait for page to load
    notte page wait 1000

    # Fill form fields
    log_info "Filling form fields..."
    for field in "${FORM_DATA[@]}"; do
        selector="${field%%|*}"
        value="${field#*|}"

        log_info "  Filling $selector"
        if ! notte page fill "$selector" "$value"; then
            log_warn "Failed to fill $selector, continuing..."
        fi
        notte page wait 200
    done

    # Take screenshot before submit
    if [[ "$TAKE_SCREENSHOTS" == "true" ]]; then
        log_info "Taking pre-submit screenshot..."
        # Note: Screenshot functionality depends on your notte setup
        # notte page screenshot --output "$SCREENSHOT_DIR/before_submit.png"
    fi

    # Submit form
    log_info "Submitting form..."
    notte page click "$SUBMIT_SELECTOR"

    # Wait for response
    notte page wait 2000

    # Verify submission
    log_info "Verifying submission..."
    SCRAPE_RESULT=$(notte sessions scrape --instructions "Check if the page shows a success message")

    if echo "$SCRAPE_RESULT" | grep -qi "$SUCCESS_INDICATOR"; then
        log_info "Form submitted successfully!"

        # Take success screenshot
        if [[ "$TAKE_SCREENSHOTS" == "true" ]]; then
            log_info "Taking success screenshot..."
            # notte page screenshot --output "$SCREENSHOT_DIR/after_submit.png"
        fi
    else
        log_warn "Could not verify success. Check the result manually."
        echo "Scrape result: $SCRAPE_RESULT"
    fi

    log_info "Form automation completed"
}

# Run main function
main "$@"
