#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/agents/check-preamble-quotes.sh [--roles-dir <directory>]

Checks every Markdown role preamble in the directory. A verbatim quote uses
the convention below, defined here once for every preamble:

  <!-- quote source="path/from/metasystem/root" -->
  byte-exact content copied from that source
  <!-- /quote -->

The content bytes, including Markdown punctuation and internal line endings,
must occur unchanged and contiguously in the named source document. The final
line ending before the closing marker is marker framing, not quote content.

Exit codes: 0 pass; 1 drift or malformed quote; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
roles_dir="$root/scripts/agents/roles"

while (($#)); do
  case "$1" in
    --roles-dir) [[ $# -ge 2 ]] || { usage; exit 2; }; roles_dir=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

# TODO(go-wiring): needs a preamble-quote verb that verifies every role
# preamble's `<!-- quote source=... -->` block is a byte-exact, contiguous
# substring of its named source under the metasystem root. A whole validator
# (byte-level regex over the preambles), so it stays here.
python3 - "$root" "$roles_dir" <<'PY'
import os
import re
import sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
roles_dir = Path(sys.argv[2]).resolve()
violations = []
pattern = re.compile(
    br'^<!-- quote source="([^"\r\n]+)" -->\n(.*?)^<!-- /quote -->$',
    re.MULTILINE | re.DOTALL,
)

if not roles_dir.is_dir():
    violations.append(f"roles directory does not exist: {roles_dir}")
else:
    preambles = sorted(roles_dir.glob("*.md"))
    if not preambles:
        violations.append(f"roles directory contains no Markdown preambles: {roles_dir}")
    for preamble in preambles:
        body = preamble.read_bytes()
        matches = list(pattern.finditer(body))
        start_count = body.count(b'<!-- quote source="')
        end_count = body.count(b'<!-- /quote -->')
        if not matches:
            violations.append(f"{preamble}: no verbatim quote block")
            continue
        if start_count != len(matches) or end_count != len(matches):
            violations.append(f"{preamble}: malformed or unpaired quote marker")
            continue
        for match in matches:
            source_name = match.group(1).decode("utf-8", errors="replace")
            source = (root / source_name).resolve()
            try:
                inside_root = os.path.commonpath((str(root), str(source))) == str(root)
            except ValueError:
                inside_root = False
            if not inside_root:
                violations.append(f"{preamble}: quote source escapes the metasystem root: {source_name}")
                continue
            if not source.is_file():
                violations.append(f"{preamble}: quote source does not exist: {source_name}")
                continue
            inner = match.group(2)
            quoted = inner[:-1] if inner.endswith(b"\n") else inner
            if not quoted:
                violations.append(f"{preamble}: quote from {source_name} is empty")
            elif quoted not in source.read_bytes():
                violations.append(f"{preamble}: quote drifted from {source_name}")

for item in violations:
    print(f"quote violation: {item}", file=sys.stderr)
sys.exit(1 if violations else 0)
PY
