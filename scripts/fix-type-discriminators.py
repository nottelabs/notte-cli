"""Repair discriminator assignments to optional Type fields in generated Go."""

import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
source = path.read_text()
pointer_types = {
    name
    for name, fields in re.findall(r"^type (\w+) struct \{(.*?)^\}", source, re.M | re.S)
    if re.search(r"^\s+Type\s+\*string\b", fields, re.M)
}


def fix(match):
    body = match.group(0)
    if match.group(1) in pointer_types:
        body = re.sub(r'v\.Type = "([^"\n]+)"', r'tmp := "\1"; v.Type = &tmp', body)
    return body


source = re.sub(r"^func [^\n]*\(v (\w+)\) error \{.*?^\}", fix, source, flags=re.M | re.S)
# Request wrappers must retain a union model's generated JSON methods.
union_types = {
    name
    for name, fields in re.findall(r"^type (\w+) struct \{(.*?)^\}", source, re.M | re.S)
    if re.search(r"^\s+union\s+json\.RawMessage\b", fields, re.M)
}
source = re.sub(
    r"^type (\w+JSONRequestBody) (\w+)$",
    lambda match: f"type {match[1]} = {match[2]}" if match[2] in union_types else match[0],
    source,
    flags=re.M,
)
path.write_text(source)
