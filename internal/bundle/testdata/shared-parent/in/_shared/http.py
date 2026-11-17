import requests


def fetch_json(q):
    return requests.get(q).json()
