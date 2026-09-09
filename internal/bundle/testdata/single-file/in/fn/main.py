import requests


def run():
    return requests.get("https://x.test").text
