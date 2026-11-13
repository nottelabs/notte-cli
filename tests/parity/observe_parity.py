"""Compare `notte page observe` (Go CLI) with the Python SDK's observation of the same page.

One session, a few act-bench pages (pages.json), each put into a known state. For each page the
Python SDK observes it, then the CLI observes it (text and `-o json`), and this checks:

- the CLI lists the same lines, in the same order, with the same ids, tags and non-state attributes;
- every live state in the SDK's `element_states` shows on the CLI's line, and nothing else changed;
- a line the API already wrote with its state is left exactly as the API wrote it;
- a typed secret (a password) never appears in the CLI's output;
- `-o json` carries the same `element_states` as the SDK.

Usage: NOTTE_API_KEY=... NOTTE_API_URL=https://us-staging.notte.cc python observe_parity.py --cli ./notte
"""

import argparse
import html
import json
import re
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from notte_sdk import NotteClient
from notte_sdk.types import ObserveResponse

HERE = Path(__file__).resolve().parent

FLAGS = ("checked", "mixed", "pressed", "selected", "expanded", "filled", "invalid")
STATE_ATTRS = ("value", "error", "sort")
NO_VALUE_TYPES = {"checkbox", "radio", "button", "submit", "reset", "image", "file", "hidden"}
MAX_VALUE_LEN = 60
ELEMENT = re.compile(r"^([A-Za-z]+[0-9]+)\[:\]<([A-Za-z][A-Za-z0-9-]*)")


@dataclass
class Line:
    """One description line. For an element: its id, tag, attribute tokens and what follows the tag."""

    raw: str
    id: str | None = None
    tag: str = ""
    tokens: list[str] = field(default_factory=list)
    rest: str = ""

    def names(self) -> list[str]:
        return [t.split("=", 1)[0].lower() for t in self.tokens]

    def has(self, name: str) -> bool:
        return name in self.names()

    def attr(self, name: str) -> str | None:
        for token in self.tokens:
            key, _, value = token.partition("=")
            if key.lower() == name:
                return html.unescape(value.strip('"'))
        return None

    def without_state(self) -> tuple[str, list[str], str]:
        kept = [t for t in self.tokens if t.split("=", 1)[0].lower() not in FLAGS + STATE_ATTRS]
        return self.tag, kept, self.rest


def parse(line: str) -> Line:
    m = ELEMENT.match(line)
    if not m:
        return Line(raw=line)
    start, in_quote = len(m.group(0)), False
    for i in range(start, len(line)):
        if line[i] == '"':
            in_quote = not in_quote
        elif line[i] == ">" and not in_quote:
            tokens, cur, q = [], "", False
            for ch in line[start:i]:
                if ch == '"':
                    q = not q
                if ch == " " and not q:
                    if cur:
                        tokens.append(cur)
                    cur = ""
                else:
                    cur += ch
            if cur:
                tokens.append(cur)
            return Line(raw=line, id=m.group(1), tag=m.group(2).lower(), tokens=tokens, rest=line[i:])
    return Line(raw=line, id=m.group(1), tag=m.group(2).lower(), rest="")


def cut(value: str, browser_cut: bool) -> str:
    if len(value) > MAX_VALUE_LEN:
        return value[:MAX_VALUE_LEN] + "..."
    return value + "..." if browser_cut else value


def expected_state(line: Line, state: dict[str, Any]) -> list[str]:
    """What the line must say about `state`, as problems found (empty when it says it all)."""
    problems: list[str] = []
    pressed = state.get("pressed")
    mixed = state.get("mixed") is True or pressed == "mixed"
    want = {
        "checked": state.get("checked") is True and not mixed,
        "mixed": mixed,
        "pressed": pressed is True,
        "selected": state.get("selected") is True,
        "invalid": state.get("invalid") is True,
        # the CLI says a password holds a value (its value is never sent); the API does not write this
        "filled": state.get("filled") is True and state.get("value") is None,
    }
    known = {
        "checked": state.get("checked") is not None,
        "mixed": state.get("mixed") is not None or pressed == "mixed",
        "pressed": pressed is not None,
        "selected": state.get("selected") is not None,
        "invalid": state.get("invalid") is not None,
        "filled": state.get("filled") is not None,
    }
    if not line.has("aria_expanded"):
        want["expanded"], known["expanded"] = state.get("expanded") is True, state.get("expanded") is not None
    for flag, on in want.items():
        if on and not line.has(flag):
            problems.append(f"missing `{flag}`")
        if not on and known[flag] and line.has(flag):
            problems.append(f"shows `{flag}`, state says it is not")

    input_type = (line.attr("type") or "").lower()
    if line.tag == "select" or state.get("options") is not None:
        labels = [o["label"] for o in state.get("options") or [] if o.get("selected")]
        if labels and line.attr("value") != ", ".join(labels):
            problems.append(f"value {line.attr('value')!r}, chosen {', '.join(labels)!r}")
    elif input_type not in NO_VALUE_TYPES and line.tag not in ("button", "a") and state.get("value") is not None:
        value = state["value"]
        shown = line.attr("value")
        if value == "" and shown is not None:
            problems.append(f"shows value {shown!r} for an empty field")
        elif value != "" and shown is None:
            problems.append(f"missing value {value!r}")
        elif value != "" and shown != cut(value, state.get("valueTruncated") is True):
            # the API may cut a served value it rendered itself: a prefix ending in "..." matches
            if not (shown and shown.endswith("...") and value.startswith(shown[:-3])):
                problems.append(f"value {shown!r}, field holds {value!r}")
    return problems


def cli(binary: str, session_id: str, *args: str) -> str:
    out = subprocess.run(
        [binary, "page", "observe", "--session-id", session_id, *args],
        capture_output=True,
        text=True,
        timeout=120,
    )
    if out.returncode != 0:
        raise RuntimeError(f"notte page observe {' '.join(args)} exited {out.returncode}: {out.stderr.strip()[:300]}")
    return out.stdout


def states_of(raw: dict[str, Any] | None) -> dict[str, dict[str, Any]]:
    """element_states as plain dicts with the API's (camelCase) keys and no defaulted fields."""
    if not raw:
        return {}
    normalized: dict[str, dict[str, Any]] = {}
    for element_id, state in raw.items():
        if hasattr(state, "model_dump"):
            state = state.model_dump(by_alias=True, mode="json")
        normalized[element_id] = {k: v for k, v in state.items() if v is not None and not (k.endswith("runcated") and v is False)}
    return normalized


def compare(page: dict[str, Any], sdk: ObserveResponse, text: str, as_json: dict[str, Any]) -> list[str]:
    problems: list[str] = []
    sdk_states = states_of(sdk.element_states)

    head, sep, body = text.partition("\n\n")
    if not sep:
        head, body = "", text
    if sdk.metadata.url not in head.split("\n")[0]:
        problems.append(f"header {head.splitlines()[:1]} does not name the page URL {sdk.metadata.url}")

    sdk_lines = [parse(line) for line in sdk.space.description.rstrip("\n").split("\n")]
    cli_lines = [parse(line) for line in body.rstrip("\n").split("\n")]
    if len(sdk_lines) != len(cli_lines):
        problems.append(f"{len(cli_lines)} lines from the CLI, {len(sdk_lines)} from the SDK")
    for n, (s, c) in enumerate(zip(sdk_lines, cli_lines, strict=False), start=1):
        where = f"line {n} ({s.id or 'text'})"
        if s.id is None or s.id != c.id:
            if s.raw != c.raw:
                problems.append(f"{where}: SDK {s.raw!r} / CLI {c.raw!r}")
            continue
        if s.without_state() != c.without_state():
            problems.append(f"{where}: CLI changed more than state: SDK {s.raw!r} / CLI {c.raw!r}")
        state = sdk_states.get(s.id)
        if state is None:
            if s.raw != c.raw:
                problems.append(f"{where}: no state, but the CLI changed it: {c.raw!r}")
            continue
        if not expected_state(s, state) and s.raw != c.raw:
            problems.append(f"{where}: the API already shows its state, the CLI rewrote it: SDK {s.raw!r} / CLI {c.raw!r}")
        for problem in expected_state(c, state):
            problems.append(f"{where}: {problem}: {c.raw!r} (state {json.dumps(state)})")

    for secret in page.get("secrets", []):
        if secret in text or secret in json.dumps(as_json):
            problems.append(f"a typed secret appears in the CLI output: {secret[:3]}...")

    if states_of(as_json.get("element_states")) != sdk_states:
        problems.append("`-o json` element_states differ from the SDK's")
    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--cli", default="./notte", help="the notte CLI binary to check")
    parser.add_argument("--pages", default=str(HERE / "pages.json"))
    parser.add_argument("--only", default="", help="comma-separated page names")
    args = parser.parse_args()

    pages = json.loads(Path(args.pages).read_text())
    if args.only:
        wanted = set(args.only.split(","))
        pages = [p for p in pages if p["name"] in wanted]

    failed: dict[str, list[str]] = {}
    started = time.monotonic()
    client = NotteClient()
    with client.Session(timeout_minutes=10) as session:
        for page in pages:
            t0 = time.monotonic()
            try:
                session.execute(type="goto", url=page["url"])
                if page.get("setup_js"):
                    session.evaluate_js(page["setup_js"])
                sdk = session.observe()
                assert isinstance(sdk, ObserveResponse)
                text = cli(args.cli, session.session_id)
                as_json = json.loads(cli(args.cli, session.session_id, "-o", "json"))
                problems = compare(page, sdk, text, as_json)
            except Exception as e:  # a page that cannot be loaded or observed fails the gate too
                problems = [f"{type(e).__name__}: {e}"]
            states = "-"
            if not problems:
                states = str(len(states_of(sdk.element_states)))
            print(f"{'PASS' if not problems else 'FAIL'} {page['name']:<18} {time.monotonic() - t0:5.1f}s  states={states}")
            for problem in problems:
                print(f"     {problem}")
            if problems:
                failed[page["name"]] = problems
    print(f"{len(pages) - len(failed)}/{len(pages)} pages agree in {time.monotonic() - started:.0f}s")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
