from .._shared.http import fetch_json


def run(q: str = "x"):
    return fetch_json(q)
