# Function Management Reference

Complete guide to creating, managing, and scheduling workflow functions with the notte CLI.

## Overview

Functions are reusable python workflows that can be:
- Run on-demand (serverless) 
- Scheduled with cron expressions
- Shared publicly and forked by others
- Tracked with run history and metadata
- Can be triggered via HTTP POST requests

## Creating Functions

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

Function files define browser automation steps with the following rules:
- must contain a `def run(**variables)`
- must create a session using the `NotteClient`
- variables defined in the `run` function will be exposed as POST body parameters


Example:

```python
# function.py
from notte_sdk import NotteClient

client = NotteClient()

def run(url: str):
    with client.Session() as session:
        session.goto(url)
        # scrape data as markdown format
        data = session.scrape()
        return data

if __name__ == "__main__":
    run("https://notte.cc/pricing")
```

## Managing Functions

### List Functions

```bash
notte functions list
```

Output includes function ID, name, description, and creation date.

### View Function Details

```bash
notte functions show --id <function-id>
```

Returns function metadata and download URL for the workflow file.

### Update Function Code

```bash
notte functions update --id <function-id> --file workflow_v2.py
```

Updates the workflow code while preserving function ID and schedule.

### Delete Function

```bash
notte functions delete --id <function-id>
```

Prompts for confirmation. Use `--yes` to skip.

## Running Functions

### Run On-Demand

```bash
notte functions run --id <function-id>
```

Starts a new function run and returns the run ID.

### Check Run Status

```bash
# List all runs for a function
notte functions runs --id <function-id>
```

Output includes:
- Run ID
- Status (running, completed, failed)
- Start time
- End time (if finished)

### Stop a Running Function

```bash
notte functions run-stop --id <function-id> --run-id <run-id>
```

## Run Metadata

Store and retrieve custom data for function runs:

### Get Metadata

```bash
notte functions run-metadata --id <function-id> --run-id <run-id>
```

### Update Metadata

```bash
# Direct JSON
notte functions run-metadata-update \
  --id <function-id> \
  --run-id <run-id> \
  --data '{"items_processed": 150, "errors": 2}'

# From file
notte functions run-metadata-update \
  --id <function-id> \
  --run-id <run-id> \
  --data @metadata.json

# From stdin
echo '{"status": "completed"}' | notte functions run-metadata-update \
  --id <function-id> \
  --run-id <run-id>
```

### Metadata Use Cases

- Track progress during long-running jobs
- Store results summary
- Record error details
- Pass data between scheduled runs

## Scheduling Functions

### Set a Cron Schedule

```bash
notte functions schedule --id <function-id> --cron "0 9 * * *"
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
notte functions schedule --id <id> --cron "0 * * * *"

# Every day at 9 AM
notte functions schedule --id <id> --cron "0 9 * * *"

# Every Monday at 6 PM
notte functions schedule --id <id> --cron "0 18 * * 1"

# Every 15 minutes
notte functions schedule --id <id> --cron "*/15 * * * *"

# First day of each month at midnight
notte functions schedule --id <id> --cron "0 0 1 * *"

# Weekdays at 8 AM
notte functions schedule --id <id> --cron "0 8 * * 1-5"
```

### Remove Schedule

```bash
notte functions unschedule --id <function-id>
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
# Upload and schedule
FUNC_ID=$(notte functions create --file price_monitor.py --name "Price Monitor" -o json | jq -r '.id')
notte functions schedule --id "$FUNC_ID" --cron "0 9 * * *"
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
# Schedule for Monday mornings
FUNC_ID=$(notte functions create --file weekly_report.py --name "Weekly Report" -o json | jq -r '.id')
notte functions schedule --id "$FUNC_ID" --cron "0 8 * * 1"
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
notte functions run-metadata --id <func-id> --run-id <run-id> -o json
```

### 3. Monitor Run History

```bash
# Check for failed runs
notte functions runs --id <func-id> -o json | jq '.[] | select(.status == "failed")'
```

### 4. Test Before Scheduling

```bash
# Run manually first
notte functions run --id <func-id>

# Check it completed successfully
notte functions runs --id <func-id>

# Then schedule
notte functions schedule --id <func-id> --cron "0 9 * * *"
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

# Delete unused
notte functions delete --id <old-func-id> --yes
```
