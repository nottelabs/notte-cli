import requests

# ── _shared/http.py ──
def fetch_json(q):
    return requests.get(q).json()

# ── fn/main.py ──
def run(q: str = "x"):
    return fetch_json(q)
