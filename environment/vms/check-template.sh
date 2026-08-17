#!/usr/bin/env bash
# check-template.sh — validate the metasystem VM templates properly.
#
#   ./check-template.sh                        both arch templates
#   ./check-template.sh metasystem-vm-amd64.yaml   just one
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

# With no argument, check every arch template rather than defaulting to one of
# them: the two are meant to stay in sync, and a default that silently skipped
# the other would let a broken sibling through. Re-invokes itself per file so the
# validation body below stays single-template.
if [ $# -eq 0 ]; then
  rc=0
  for t in "$(dirname "$0")"/metasystem-vm-arm64.yaml \
           "$(dirname "$0")"/metasystem-vm-amd64.yaml; do
    [ -f "$t" ] || continue
    echo "### $(basename "$t")"
    "$0" "$t" || rc=1
  done
  exit "$rc"
fi

TEMPLATE="$1"

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

# Block 14 warms ~/.m2 for benchmark bm-1 from versions PINNED in this template,
# because an image build has no copy of the repository to read them from. That
# duplication is the risk: a bumped dependency in the spec's pom would leave the
# VM priming the old artifacts, and the failure surfaces hours into a mission as
# an unsatisfiable offline gate. So compare the two here, where it is cheap.
echo "==> bm-1 pinned versions match the spec seed"
python3 - "$TEMPLATE" "$(dirname "$0")/../../benchmark/specs/bm-1" <<'PY'
import re, sys
from pathlib import Path

template, spec = Path(sys.argv[1]), Path(sys.argv[2])
pom = spec / "seed" / "pom.xml"
wrapper = spec / "seed" / ".mvn" / "wrapper" / "maven-wrapper.properties"
if not pom.is_file():
    print(f"  spec seed absent ({pom}) -- nothing to compare"); raise SystemExit(0)

tpl, pom_text = template.read_text(), pom.read_text()

def param(name):
    m = re.search(rf'^\s*{name}:\s*"([^"]+)"', tpl, re.M)
    return m.group(1) if m else None

def plugin(artifact):
    m = re.search(rf'<artifactId>{artifact}</artifactId>\s*<version>([^<]+)</version>',
                  pom_text)
    return m.group(1) if m else None

expected = {
    "BM1_JUNIT_VERSION":
        (re.search(r"<junit\.version>([^<]+)</junit\.version>", pom_text) or [None, None])[1],
    "BM1_SUREFIRE_VERSION": plugin("maven-surefire-plugin"),
    "BM1_JAR_PLUGIN_VERSION": plugin("maven-jar-plugin"),
    "BM1_COMPILER_PLUGIN_VERSION": plugin("maven-compiler-plugin"),
    "BM1_COMPILER_RELEASE":
        (re.search(r"<maven\.compiler\.release>([^<]+)</", pom_text) or [None, None])[1],
}
if wrapper.is_file():
    url = re.search(r"distributionUrl=.*apache-maven-([0-9][^-]*)-bin\.zip",
                    wrapper.read_text())
    if url:
        expected["BM1_WRAPPER_MAVEN_VERSION"] = url.group(1)

bad = 0
for name, want in expected.items():
    got = param(name)
    if want is None:
        print(f"  {name}: spec value not found -- check the comparison, not the template")
        bad += 1
    elif got != want:
        print(f"  {name}: template has {got!r}, spec seed says {want!r}")
        bad += 1
    else:
        print(f"  {name}: {got} ok")

print(f"==> {len(expected)} pinned versions checked, {bad} mismatched")
sys.exit(1 if bad else 0)
PY
