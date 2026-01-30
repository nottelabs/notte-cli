---
name: function-management
description: Guide to creating, managing, and scheduling workflow functions
---

# Function Management Reference

Complete guide to creating, managing, and scheduling workflow functions with the notte CLI.

## Overview

Functions are reusable python workflows that can be:
- Run on-demand (serverless) 
- Scheduled with cron expressions
- Shared publicly and forked by others
- Tracked with run history and metadata
- Can be triggered via HTTP POST requests

## Development Workflow

Building a function follows this iterative process:

### Step-by-Step Process

1. **Build interactively** - Use `notte sessions start` and `notte page` commands to develop your automation step-by-step in the terminal
2. **Export code** - Run `notte sessions workflow-code` to generate a working Python script from your session
3. **Create function** - Save the exported code as `my_function.py`, then upload it with `notte functions create --file my_function.py` (becomes current function)
4. **Test in cloud** - Run `notte functions run` to execute remotely and get a run ID
5. **Monitor logs** - Check execution output with `notte functions run-metadata --run-id <run-id>` and inspect the `logs` field
6. **Iterate** - Update your code based on results, then use `notte functions update --file my_function.py`
7. **Schedule** - When stable, add a cron schedule: `notte functions schedule --cron "0 9 * * *"`

### Complete Example

```bash
# 1. Build your automation interactively
notte sessions start --headless
notte page goto "https://news.ycombinator.com"
notte page observe
notte page scrape --instructions "Extract top 5 story titles and URLs"
notte sessions stop

# 2. Export the session as Python code
notte sessions workflow-code > hn_scraper.py

# 3. Edit the file to add the run() function and parameters
# hn_scraper.py should look like:
# from notte_sdk import NotteClient
# 
# client = NotteClient()
# 
# def run(max_stories: int = 5):
#     with client.Session() as session:
#         session.goto("https://news.ycombinator.com")
#         data = session.scrape(instructions=f"Extract top {max_stories} story titles and URLs")
#         return {"stories": data, "count": max_stories}

# 4. Create the function (automatically becomes current function)
notte functions create \
  --file hn_scraper.py \
  --name "HN Top Stories" \
  --description "Scrapes top stories from Hacker News"

# 5. Test the function
RUN_ID=$(notte functions run -o json | jq -r '.run_id')
echo "Started run: $RUN_ID"

# Wait a few seconds for execution
sleep 10

# 6. Check the logs and results
notte functions run-metadata --run-id "$RUN_ID" -o json | jq '{
  status: .status,
  logs: .logs,
  result: .result
}'

# 7. If needed, update and iterate
# Edit hn_scraper.py with improvements
notte functions update --file hn_scraper.py

# Test again
RUN_ID=$(notte functions run -o json | jq -r '.run_id')
sleep 10
notte functions run-metadata --run-id "$RUN_ID"

# 8. Schedule when ready (every day at 9 AM)
notte functions schedule --cron "0 9 * * *"
```

### Tips for Iterative Development

- **Start simple**: Build a minimal version first, then add features
- **Test frequently**: Run `notte functions run` after each change to catch issues early
- **Monitor logs**: The `logs` field in run-metadata shows print statements and errors
- **Use variables**: Add function parameters for flexibility (e.g., `max_stories` in the example)
- **Return data**: Always return structured data from your `run()` function for easy access via run-metadata

## Creating Functions

**Note:** When you create a function, it automatically becomes the "current" function. All subsequent commands (run, update, schedule, etc.) use this function by default. Use `--id <function-id>` only when you need to manage multiple functions simultaneously or reference a specific function.

### From a Python File

```bash
notte functions create --file function.py
```

### With Metadata

```bash
notte functions create \
  --file workflow.py \
  --name "Product Price Monitor" \
  --description "Monitors competitor prices daily" \
  --shared  # Make publicly available
```

### Function File Format

Function files define browser automation steps with the following requirements:

**Required:**
- Must contain a `def run()` function - this is the entry point
- Must create a session using `NotteClient().Session()`

**Function Variables (Parameters):**
- Parameters in the `run()` function become POST body parameters when triggering the function
- Use type hints to document expected types (e.g., `str`, `int`, `bool`, `list`, `dict`)
- Default values make parameters optional when triggering

**Return Values:**
- Data returned from `run()` is stored and accessible via `notte functions run-metadata`
- Return structured data (dict, list) for easy parsing
- The return value appears in the `result` field of run-metadata

**Basic Example:**

```python
# function.py
from notte_sdk import NotteClient

client = NotteClient()

def run(url: str):
    """Simple function with one required parameter."""
    with client.Session() as session:
        session.goto(url)
        data = session.scrape()
        return data

if __name__ == "__main__":
    run("https://notte.cc/pricing")
```

**Advanced Example with Variables:**

```python
# price_monitor.py
from notte_sdk import NotteClient

client = NotteClient()

def run(
    url: str,
    max_items: int = 10,
    only_discounted: bool = False,
    categories: list[str] = None
):
    """
    Function parameters become POST body parameters.
    
    Args:
        url: Required parameter (no default)
        max_items: Optional with default value
        only_discounted: Optional boolean
        categories: Optional list
    """
    with client.Session() as session:
        session.goto(url)
        
        # Build extraction instructions dynamically
        instructions = f"Extract up to {max_items} products"
        if only_discounted:
            instructions += " that are on sale"
        if categories:
            instructions += f" in categories: {', '.join(categories)}"
        
        products = session.scrape(instructions=instructions)
        
        # Return structured data
        return {
            "success": True,
            "url": url,
            "products": products,
            "count": len(products) if products else 0,
            "filters": {
                "max_items": max_items,
                "only_discounted": only_discounted,
                "categories": categories
            }
        }

if __name__ == "__main__":
    # Test locally with default values
    result = run(
        url="https://example.com/products",
        max_items=5,
        only_discounted=True,
        categories=["electronics", "accessories"]
    )
    print(result)
```

**Triggering with Parameters:**

When running the function, pass parameters as JSON in the POST body or via the CLI:

```bash
# Run with default parameters
notte functions run

# The function will be triggered via HTTP POST with parameters in body:
# POST /functions/{id}/run
# {
#   "url": "https://example.com/products",
#   "max_items": 5,
#   "only_discounted": true,
#   "categories": ["electronics"]
# }
```

**Accessing Return Values:**

```bash
# Get the result from run-metadata
notte functions run-metadata --run-id <run-id> -o json | jq '.result'

# Output:
# {
#   "success": true,
#   "url": "https://example.com/products",
#   "products": [...],
#   "count": 5,
#   "filters": {
#     "max_items": 5,
#     "only_discounted": true,
#     "categories": ["electronics"]
#   }
# }
```

## Managing Functions

### List Functions

```bash
notte functions list
```

Output includes function ID, name, description, and creation date.

### View Function Details

```bash
notte functions show
```

Returns function metadata and download URL for the workflow file for the current function.

### Update Function Code

```bash
notte functions update --file workflow_v2.py
```

Updates the workflow code while preserving function ID and schedule.

### Delete Function

```bash
notte functions delete
```

Prompts for confirmation. Use `--yes` to skip.

## Running Functions

### Run On-Demand

```bash
notte functions run
```

Starts a new function run and returns the run ID.

### Check Run Status

```bash
# List all runs for current function
notte functions runs
```

Output includes:
- Run ID
- Status (running, completed, failed)
- Start time
- End time (if finished)

### Stop a Running Function

```bash
notte functions run-stop --run-id <run-id>
```

## Run Metadata

Store and retrieve custom data for function runs:

### Get Metadata

```bash
notte functions run-metadata --run-id <run-id>
```

### Metadata Use Cases

- Track progress during long-running jobs
- Store results summary
- Record error details
- Pass data between scheduled runs

## Scheduling Functions

### Set a Cron Schedule

```bash
notte functions schedule --cron "0 9 * * *"
```

### Cron Expression Format

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
│ │ │ │ │
* * * * *
```

### Common Cron Examples

```bash
# Every hour
notte functions schedule --cron "0 * * * *"

# Every day at 9 AM
notte functions schedule --cron "0 9 * * *"

# Every Monday at 6 PM
notte functions schedule --cron "0 18 * * 1"

# Every 15 minutes
notte functions schedule --cron "*/15 * * * *"

# First day of each month at midnight
notte functions schedule --cron "0 0 1 * *"

# Weekdays at 8 AM
notte functions schedule --cron "0 8 * * 1-5"
```

### Remove Schedule

```bash
notte functions unschedule
```

Function remains but will no longer run automatically.

## Sharing Functions

### Make Public

```bash
# When creating
notte functions create --file workflow.py --shared

# Public functions can be discovered and forked by others
```

### Fork a Shared Function

Copy a shared function to your account:

```bash
notte functions fork --id <shared-function-id>
```

Creates a new function with the same code under your account.

## Example Workflows

### Daily Price Monitor

```python
# price_monitor.py
from notte_sdk import NotteClient

client = NotteClient()

def run(competitor_url: str = "https://competitor.com/products"):
    with client.Session() as session:
        session.goto(competitor_url)
        prices = session.scrape(instructions="Extract all product prices as JSON")
        return {"prices": prices, "count": len(prices) if prices else 0}

if __name__ == "__main__":
    run()
```

```bash
# Create and schedule
notte functions create --file price_monitor.py --name "Price Monitor"
notte functions schedule --cron "0 9 * * *"
```

### Weekly Report Generator

```python
# weekly_report.py
from notte_sdk import NotteClient

client = NotteClient()

vault = client.Vault("my-vault-id")

def run(dashboard_url: str = "https://dashboard.example.com"):
    with client.Session(enable_file_storage=True) as session:
        # Login using vault credentials (vault auto-fills credentials)
        session.goto(f"{dashboard_url}/login")

        agent = client.Agent(session, vault=vault, max_steps=5)
        agent.run(task="Login to dashboard")

        session.goto(f"{dashboard_url}/reports/weekly")

        report = session.scrape(instructions="Extract the weekly summary statistics")

        # Download PDF report
        session.execute(type="click", selector="@download-pdf-button")

        return report

if __name__ == "__main__":
    run()
```

```bash
# Create and schedule for Monday mornings
notte functions create --file weekly_report.py --name "Weekly Report"
notte functions schedule --cron "0 8 * * 1"
```

### Error Monitoring with Retries

```python
# monitor_with_retry.py
from notte_sdk import NotteClient
import time

client = NotteClient()

def run(status_url: str = "https://app.example.com/status", max_retries: int = 3):
    for attempt in range(max_retries):
        try:
            with client.Session() as session:
                session.goto(status_url)
                status = session.scrape(instructions="Extract system status as JSON")

                if status and status.get("healthy"):
                    return {"success": True, "message": "All systems operational"}
                else:
                    return {"success": False, "alert": True, "status": status}

        except Exception as e:
            if attempt < max_retries - 1:
                time.sleep(30)
            else:
                return {"success": False, "error": f"Failed after {max_retries} attempts: {e}"}

if __name__ == "__main__":
    run()
```

## Best Practices

### 1. Use Descriptive Names

```bash
notte functions create \
  --file workflow.py \
  --name "Daily Competitor Price Check" \
  --description "Monitors prices on competitor.com every morning at 9 AM"
```

### 2. Return Important Data from Functions

```bash
# Functions return data that can be retrieved via run metadata
notte functions run-metadata --run-id <run-id> -o json
```

### 3. Monitor Run History

```bash
# Check for failed runs
notte functions runs -o json | jq '.[] | select(.status == "failed")'
```

### 4. Test Before Scheduling

```bash
# Run manually first
notte functions run

# Check it completed successfully
notte functions runs

# Then schedule
notte functions schedule --cron "0 9 * * *"
```

### 5. Use Appropriate Schedules

- Don't schedule more frequently than needed
- Consider time zones
- Avoid peak hours if possible
- Account for function runtime when scheduling

### 6. Clean Up Unused Functions

```bash
# List functions and review
notte functions list

# Switch to the function you want to delete
notte functions show --id <old-func-id>

# Delete it
notte functions delete --yes
```
