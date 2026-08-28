import requests

# ── fn/main.py ──
def run():
    return requests.get("https://x.test").text
