#!/bin/bash
# Data Extraction Template
# Scrape structured data from websites with the notte CLI
#
# Usage: ./data-extraction.sh [url]
#
# Prerequisites:
#   - notte CLI installed and authenticated (notte auth login)
#   - NOTTE_API_KEY environment variable set
#
# Examples:
#   ./data-extraction.sh "https://news.ycombinator.com"
#   ./data-extraction.sh "https://example.com/products"

set -euo pipefail

# Configuration
DEFAULT_URL="https://news.ycombinator.com"
TARGET_URL="${1:-$DEFAULT_URL}"

# Extraction instructions - customize for your data
EXTRACTION_INSTRUCTIONS="Extract the following as a JSON array:
- title: the headline or name
- link: the URL if available
- description: a brief summary or subtitle
- metadata: any relevant dates, authors, or categories"

# Output settings
OUTPUT_FORMAT="json"  # json or text
OUTPUT_FILE=""  # Set to filename to save output, empty for stdout
ONLY_MAIN_CONTENT=true

# Pagination settings
PAGINATE=false
MAX_PAGES=5
NEXT_PAGE_SELECTOR="@next"  # Selector for next page button

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1" >&2; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1" >&2; }

cleanup() {
    log_info "Cleaning up..."
    notte sessions stop --yes 2>/dev/null || true
}

trap cleanup EXIT

# Quick scrape without session (for simple single-page extraction)
quick_scrape() {
    local url="$1"
    local instructions="$2"

    log_step "Quick scraping: $url"

    local flags=""
    if [[ "$ONLY_MAIN_CONTENT" == "true" ]]; then
        flags="--only-main-content"
    fi

    local result
    # shellcheck disable=SC2086
    result=$(notte scrape "$url" --instructions "$instructions" $flags -o "$OUTPUT_FORMAT")

    echo "$result"
}

# Session-based scrape (for complex or multi-page extraction)
session_scrape() {
    local url="$1"
    local instructions="$2"

    log_step "Starting browser session..."
    notte sessions start > /dev/null

    log_step "Navigating to: $url"
    notte sessions observe --url "$url" > /dev/null
    notte page wait 1500

    local all_results="[]"
    local page_num=1

    while true; do
        log_step "Scraping page $page_num..."

        local flags=""
        if [[ "$ONLY_MAIN_CONTENT" == "true" ]]; then
            flags="--only-main-content"
        fi

        local page_result
        # shellcheck disable=SC2086
        page_result=$(notte sessions scrape --instructions "$instructions" $flags -o json)

        # Merge results (assuming JSON array output)
        if command -v jq &> /dev/null; then
            all_results=$(echo "$all_results" "$page_result" | jq -s '.[0] + (.[1] | if type == "array" then . else [.] end)')
        else
            # Fallback: just append
            all_results="$all_results
$page_result"
        fi

        log_info "Page $page_num scraped"

        # Check pagination
        if [[ "$PAGINATE" != "true" ]] || [[ $page_num -ge $MAX_PAGES ]]; then
            break
        fi

        # Try to click next page
        log_step "Looking for next page..."
        if ! notte page click "$NEXT_PAGE_SELECTOR" 2>/dev/null; then
            log_info "No more pages found"
            break
        fi

        notte page wait 2000
        page_num=$((page_num + 1))
    done

    echo "$all_results"
}

# Scrape multiple URLs
batch_scrape() {
    local urls=("$@")
    local all_results="[]"

    log_step "Starting browser session for batch scrape..."
    notte sessions start > /dev/null

    for url in "${urls[@]}"; do
        log_step "Scraping: $url"
        notte sessions observe --url "$url" > /dev/null
        notte page wait 1500

        local flags=""
        if [[ "$ONLY_MAIN_CONTENT" == "true" ]]; then
            flags="--only-main-content"
        fi

        local result
        # shellcheck disable=SC2086
        result=$(notte sessions scrape --instructions "$EXTRACTION_INSTRUCTIONS" $flags -o json)

        # Add source URL to result
        if command -v jq &> /dev/null; then
            result=$(echo "$result" | jq --arg url "$url" '. + {source_url: $url}')
            all_results=$(echo "$all_results" "[$result]" | jq -s '.[0] + .[1]')
        fi

        log_info "Completed: $url"
    done

    echo "$all_results"
}

format_output() {
    local data="$1"

    if [[ "$OUTPUT_FORMAT" == "json" ]] && command -v jq &> /dev/null; then
        echo "$data" | jq '.'
    else
        echo "$data"
    fi
}

save_output() {
    local data="$1"
    local file="$2"

    if [[ -n "$file" ]]; then
        echo "$data" > "$file"
        log_info "Output saved to: $file"
    else
        echo "$data"
    fi
}

main() {
    log_info "=== Data Extraction ==="
    log_info "Target: $TARGET_URL"
    log_info "Instructions: ${EXTRACTION_INSTRUCTIONS:0:50}..."

    local result

    # Choose extraction method based on configuration
    if [[ "$PAGINATE" == "true" ]]; then
        log_info "Mode: Multi-page session scrape"
        result=$(session_scrape "$TARGET_URL" "$EXTRACTION_INSTRUCTIONS")
    else
        # Try quick scrape first (faster, no session needed)
        log_info "Mode: Quick scrape"
        result=$(quick_scrape "$TARGET_URL" "$EXTRACTION_INSTRUCTIONS")
    fi

    # Format and output
    local formatted
    formatted=$(format_output "$result")

    save_output "$formatted" "$OUTPUT_FILE"

    log_info "=== Extraction complete ==="
}

# Handle batch mode if multiple URLs provided
if [[ $# -gt 1 ]]; then
    log_info "Batch mode: ${#} URLs"
    result=$(batch_scrape "$@")
    formatted=$(format_output "$result")
    save_output "$formatted" "$OUTPUT_FILE"
else
    main
fi
