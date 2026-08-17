#!/usr/bin/env bash
# check-template.sh — validate metasystem-vm.yaml properly.
#
# `limactl validate` only parses YAML. It happily accepts provision scripts that
# are broken bash, and you only find out minutes into a build. This also runs
# `bash -n` over every `script: |` block.
#
# Real bug this caught: a comment placed after a line-continuation backslash,
#   apt-get install -y -qq \
#   # comment
#     git bash coreutils ...
# which bash joins into the comment, turning the package list into a separate
# command (`git bash coreutils ...`) that fails at provision time.
set -euo pipefail

TEMPLATE="${1:-$(dirname "$0")/metasystem-vm.yaml}"

echo "==> limactl validate"
limactl validate "$TEMPLATE"

echo "==> bash -n over provision scripts"
python3 - "$TEMPLATE" <<'PY'
import re, subprocess, sys, tempfile, os
path_in = sys.argv[1]
src = open(path_in).read()

blocks, cur, indent = [], None, None
for line in src.splitlines():
    if cur is not None:
        if line.strip() == '' or line.startswith(indent):
            cur.append(line[len(indent):] if line.startswith(indent) else '')
            continue
        blocks.append('\n'.join(cur)); cur = None; indent = None
    m = re.match(r'^(\s*)script:\s*\|\s*$', line)
    if m:
        indent = m.group(1) + '  '
        cur = []
if cur:
    blocks.append('\n'.join(cur))

bad = 0
for i, b in enumerate(blocks, 1):
    # Go-template placeholders are not bash; substitute a plain token.
    b2 = re.sub(r'\{\{[^}]*\}\}', 'TPLVAL', b)
    with tempfile.NamedTemporaryFile('w', suffix='.sh', delete=False) as f:
        f.write(b2); p = f.name
    r = subprocess.run(['bash', '-n', p], capture_output=True, text=True)
    if r.returncode != 0:
        bad += 1
        print(f"  block {i}: SYNTAX ERROR")
        for l in r.stderr.strip().splitlines():
            print("   ", l.replace(p, f"block{i}"))
    else:
        print(f"  block {i}: ok")
    os.unlink(p)

print(f"==> {len(blocks)} scripts checked, {bad} with errors")
sys.exit(1 if bad else 0)
PY
