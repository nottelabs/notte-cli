"""Validate a bundled artifact against the runtime's own rules.

Reads {"source": str, "allowed_imports": [str]} on stdin and writes a verdict
as JSON on stdout. Never raises for a rejected script: a non-zero exit would
be indistinguishable from the interpreter or the SDK being broken.

Why the allow list is injected rather than trusted:

The published notte-sdk carries its own ALLOWED_IMPORTS, and it is stale. At
1.8.31 it is an explicit 41-entry list with no httpcloak, httpx, bs4 or tqdm,
and it *includes* tempfile, which the runner discards. Running it unmodified
rejects functions that are deployed and serving traffic. So the validator's
machinery is reused for what it gets right - the entry point, forbidden nodes,
forbidden calls, relative imports - while the import question is answered from
GET /functions/health, which asks the runner.
"""

import ast
import json
import sys


def main() -> None:
    request = json.load(sys.stdin)
    source = request["source"]

    try:
        from notte_core.ast import ScriptValidator
    except Exception as exc:  # pragma: no cover - environment, not input
        json.dump({"ok": False, "stage": "import", "errors": [f"cannot import notte_core.ast: {exc}"]}, sys.stdout)
        return

    # The runtime's list replaces the SDK's, and the SDK's denylist is emptied
    # so it cannot veto a name the runtime allows. Anything not in the runtime
    # list is simply absent, and check_valid_import rejects it.
    ScriptValidator.ALLOWED_IMPORTS = set(request["allowed_imports"])
    if hasattr(ScriptValidator, "DISALLOWED_STDLIB_IMPORTS"):
        ScriptValidator.DISALLOWED_STDLIB_IMPORTS = set()

    errors = []

    # parse_script accepts two top-level run() definitions where the server
    # rejects them, so that is checked here rather than delegated.
    try:
        tree = ast.parse(source)
        runs = [n for n in tree.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)) and n.name == "run"]
        if len(runs) > 1:
            lines = ", ".join(str(n.lineno) for n in runs)
            errors.append(f"multiple top-level run() definitions (lines {lines}); the runtime accepts exactly one")
    except SyntaxError as exc:
        json.dump({"ok": False, "stage": "syntax", "errors": [f"line {exc.lineno}: {exc.msg}"]}, sys.stdout)
        return

    variables = []
    try:
        info = ScriptValidator.parse_script(source, restricted=True)
        variables = [
            {"name": v.name, "type": v.type, "default": v.default}
            for v in info.variables
        ]
    except Exception as exc:
        errors.append(f"{type(exc).__name__}: {exc}")

    json.dump({"ok": not errors, "stage": "validate", "errors": errors, "variables": variables}, sys.stdout)


main()
