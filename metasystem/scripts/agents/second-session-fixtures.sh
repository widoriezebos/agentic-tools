#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
helper="$root/scripts/agents/second-session-isolation.py"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-second-session.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
source_checkout="$tmp/source"
isolated_checkout="$tmp/isolated"
mkdir -p "$source_checkout/metasystem" "$isolated_checkout/metasystem" \
  "$source_checkout/.codex" "$source_checkout/.claude" "$source_checkout/.devin"
printf 'model = "fixture"\n' >"$source_checkout/.codex/config.toml"
printf '{"theme":"dark"}\n' >"$source_checkout/.claude/settings.local.json"
printf '{"hooks":[]}\n' >"$source_checkout/.devin/hooks.v1.json"

manifest="$tmp/paths"
for adapter in "$root"/scripts/agents/adapters/*.sh; do
  [[ ${adapter##*/} != runtime-common.sh ]] || continue
  "$adapter" local-config-paths
done | sort -u >"$manifest"
python3 - "$manifest" <<'PY'
import sys
from pathlib import Path
assert Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() == [
    ".claude/settings.json",
    ".claude/settings.local.json",
    ".codex/config.toml",
    ".devin/config.json",
    ".devin/config.local.json",
    ".devin/hooks.v1.json",
]
PY
new_harness=$("$helper" --source-root "$source_checkout" \
  --destination-root "$isolated_checkout" --manifest "$manifest" \
  --harness-root "$source_checkout/metasystem")
[[ "$new_harness" == "$(cd "$isolated_checkout/metasystem" && pwd -P)" ]]
cmp "$source_checkout/.codex/config.toml" "$isolated_checkout/.codex/config.toml"
cmp "$source_checkout/.claude/settings.local.json" \
  "$isolated_checkout/.claude/settings.local.json"
cmp "$source_checkout/.devin/hooks.v1.json" "$isolated_checkout/.devin/hooks.v1.json"

# A pre-existing link back to the primary checkout is not accepted as a copy.
unsafe_checkout="$tmp/unsafe"
mkdir -p "$unsafe_checkout/metasystem" "$unsafe_checkout/.codex"
ln -s "$source_checkout/.codex/config.toml" "$unsafe_checkout/.codex/config.toml"
if "$helper" --source-root "$source_checkout" --destination-root "$unsafe_checkout" \
  --manifest "$manifest" --harness-root "$source_checkout/metasystem" \
  >"$tmp/unsafe.out" 2>"$tmp/unsafe.err"; then
  echo "second-session fixture: audit accepted a local-config link into the primary checkout" >&2
  exit 1
fi
grep -Fq 'isolation audit failed' "$tmp/unsafe.err"

# WC-8: the paved command must work from a human shell, whose ancestry has no
# runtime signature. Build the smallest committed source checkout and replace
# only process visibility and long-lived supervision with recording fakes.
bootstrap_parent="$tmp/bootstrap-parent"
bootstrap_source="$bootstrap_parent/source"
bootstrap_destination="$bootstrap_parent/human-session"
bootstrap_harness="$bootstrap_source/metasystem"
mkdir -p "$bootstrap_harness/scripts/agents/adapters"
cp "$root/scripts/agents/second-session.sh" \
  "$bootstrap_harness/scripts/agents/second-session.sh"
cp "$root/scripts/agents/second-session-isolation.py" \
  "$bootstrap_harness/scripts/agents/second-session-isolation.py"
cat >"$bootstrap_harness/scripts/agents/adapters/fake.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ ${1:-} == local-config-paths ]] || exit 2
SH
cat >"$bootstrap_harness/scripts/agents/process-census.py" <<'PY'
#!/usr/bin/env python3
import sys
assert sys.argv[1:3] == ["started-at", "--pid"]
assert int(sys.argv[3]) > 0
print(1786104000)
PY
cat >"$bootstrap_harness/scripts/agents/arm-supervision.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" >"$METASYSTEM_SECOND_SESSION_ARM_LOG"
SH
chmod +x "$bootstrap_harness/scripts/agents/second-session.sh" \
  "$bootstrap_harness/scripts/agents/second-session-isolation.py" \
  "$bootstrap_harness/scripts/agents/adapters/fake.sh" \
  "$bootstrap_harness/scripts/agents/process-census.py" \
  "$bootstrap_harness/scripts/agents/arm-supervision.sh"
git -C "$bootstrap_source" init -q
git -C "$bootstrap_source" add metasystem
git -C "$bootstrap_source" -c user.name=fixture \
  -c user.email=fixture@example.invalid commit -qm seed
METASYSTEM_SECOND_SESSION_ARM_LOG="$tmp/bootstrap-arm.log" \
  "$bootstrap_harness/scripts/agents/second-session.sh" human-session \
  >"$tmp/bootstrap.out"
bootstrap_destination=$(cd "$bootstrap_destination" && pwd -P)
grep -Fqx "cd '$bootstrap_destination'" "$tmp/bootstrap.out"
python3 - "$tmp/bootstrap-arm.log" "$bootstrap_destination" <<'PY'
import sys
from pathlib import Path

args = Path(sys.argv[1]).read_text(encoding="utf-8").splitlines()
destination = sys.argv[2]
pairs = dict(zip(args[::2], args[1::2]))
assert pairs["--repo"] == destination
assert pairs["--pid"].isdigit() and int(pairs["--pid"]) > 0
assert pairs["--start-time"] == "1786104000"
assert pairs["--session"].startswith("second-session-bootstrap-human-session-")
assert pairs["--tag"].startswith("metasystem-main-bootstrap-human-session-")
PY

echo "second-session isolation fixtures: PASSED"
