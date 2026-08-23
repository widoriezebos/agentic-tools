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
# The manifest is a contract: exactly these files, no more, no fewer.
printf '%s\n' \
  '.claude/settings.json' \
  '.claude/settings.local.json' \
  '.codex/config.toml' \
  '.devin/config.json' \
  '.devin/config.local.json' \
  '.devin/hooks.v1.json' >"$tmp/expected-paths"
diff -u "$tmp/expected-paths" "$manifest" >&2 \
  || { echo "second-session fixtures: adapter local-config-paths drifted from the declared manifest" >&2; exit 1; }
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
# The arm log is one flag or value per line; read it as flag/value pairs.
arm_args=()
while IFS= read -r arm_line; do arm_args+=("$arm_line"); done <"$tmp/bootstrap-arm.log"
arm_value() { # flag: print the flag's recorded value, refuse when absent
  local flag=$1 index found=0 value=
  for ((index = 0; index + 1 < ${#arm_args[@]}; index += 2)); do
    [[ "${arm_args[index]}" == "$flag" ]] || continue
    found=1; value=${arm_args[index + 1]}
  done
  (( found )) || { echo "second-session fixtures: arming was not passed $flag" >&2; return 1; }
  printf '%s\n' "$value"
}
[[ "$(arm_value --repo)" == "$bootstrap_destination" ]] \
  || { echo "second-session fixtures: arming did not target the new session checkout" >&2; exit 1; }
arm_pid=$(arm_value --pid)
[[ "$arm_pid" =~ ^[0-9]+$ && "$arm_pid" -gt 0 ]] \
  || { echo "second-session fixtures: arming did not carry a real pid: $arm_pid" >&2; exit 1; }
[[ "$(arm_value --start-time)" == 1786104000 ]] \
  || { echo "second-session fixtures: arming did not carry the recorded start time" >&2; exit 1; }
[[ "$(arm_value --session)" == second-session-bootstrap-human-session-* ]] \
  || { echo "second-session fixtures: arming session name lost its bootstrap prefix" >&2; exit 1; }
[[ "$(arm_value --tag)" == metasystem-main-bootstrap-human-session-* ]] \
  || { echo "second-session fixtures: arming tag lost its bootstrap prefix" >&2; exit 1; }

echo "second-session isolation fixtures: PASSED"
