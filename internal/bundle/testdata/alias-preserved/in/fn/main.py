from .parse import parse_rows as pr, clean


def run(q: str = "x"):
    return pr(clean(q))
