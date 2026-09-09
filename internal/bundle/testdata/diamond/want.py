# ── fn/shared.py ──
def shared():
    return 0

# ── fn/left.py ──
def left():
    return shared()

# ── fn/right.py ──
def right():
    return shared() + 1

# ── fn/main.py ──
def run():
    return left() + right()
