# ── fn/base.py ──
def base():
    return 1

# ── fn/mid.py ──
def middle():
    return base() + 1

# ── fn/main.py ──
def run():
    return middle()
