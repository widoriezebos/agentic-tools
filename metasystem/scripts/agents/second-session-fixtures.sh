#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "second-session fixtures: binary absent; run the go gate first" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-second-session.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
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
# The copy-verification and symlink-into-primary refusal legs retired to
# the go gate (script-fixtures-015): internal/validate's
# TestSessionIsolationCopiesAndResolvesHarness and
# TestSessionIsolationRejectsSymlinkIntoPrimary prove the same
# properties. What stays here is what shell owns: the adapters'
# local-config-paths manifest above, and WC-8's human-shell bootstrap.

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
# The paved script resolves its engine as <harness>/bin/metasystem. The stub
# keeps process visibility deterministic (a fixed start time) and hands every
# other verb to the real engine, replacing the retired process-census.py stub.
mkdir -p "$bootstrap_harness/bin"
cat >"$bootstrap_harness/bin/metasystem" <<SH
#!/usr/bin/env bash
set -euo pipefail
if [[ "\${1:-} \${2:-}" == "proc started-at" ]]; then
  printf '1786104000\n'
  exit 0
fi
exec "$ms" "\$@"
SH
cat >"$bootstrap_harness/scripts/agents/adapters/fake.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ ${1:-} == local-config-paths ]] || exit 2
SH
cat >"$bootstrap_harness/scripts/agents/arm-supervision.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" >"$METASYSTEM_SECOND_SESSION_ARM_LOG"
SH
chmod +x "$bootstrap_harness/scripts/agents/second-session.sh" \
  "$bootstrap_harness/scripts/agents/adapters/fake.sh" \
  "$bootstrap_harness/bin/metasystem" \
  "$bootstrap_harness/scripts/agents/arm-supervision.sh"
git -C "$bootstrap_source" init -q -b main
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
