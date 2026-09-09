import requests
from pydantic import BaseModel

from .a import a
from .b import b


class Response(BaseModel):
    ok: bool


def run():
    return Response(ok=bool(requests) and a() and b())
