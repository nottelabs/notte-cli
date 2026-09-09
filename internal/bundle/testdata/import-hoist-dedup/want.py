import requests
from pydantic import BaseModel

# ── fn/a.py ──
def a():
    return requests is not None

# ── fn/b.py ──
def b():
    return BaseModel is not None and requests is not None

# ── fn/main.py ──
class Response(BaseModel):
    ok: bool


def run():
    return Response(ok=bool(requests) and a() and b())
