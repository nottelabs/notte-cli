# ── fn/parse.py ──
def clean(s):
    return s.strip()


def parse_rows(s):
    return [s]

# ── fn/main.py ──
pr = parse_rows


def run(q: str = "x"):
    return pr(clean(q))
