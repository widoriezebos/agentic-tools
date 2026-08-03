#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/harness-config.sh get --key <key> [--mode <mode>] \
      [--flag <value>] [--default <value>]
  scripts/harness-config.sh validate

Resolution order is an explicitly supplied flag, the mechanically derived
HARNESS_ environment variable, a mode-scoped key, the plain key, then the
explicit default. Dots and dashes in a key become underscores in its
upper-cased environment name.

Exit codes: 0 success; 1 missing value or invalid configuration; 2 usage.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
config="$root/harness.conf"

conf_value() { # key
  python3 - "$config" "$1" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
key = sys.argv[2]
try:
    lines = path.read_text(encoding="utf-8").splitlines()
except OSError as error:
    print(f"cannot read harness configuration: {path}: {error}", file=sys.stderr)
    raise SystemExit(1)

found = []
for raw in lines:
    line = raw.strip()
    if not line or line.startswith("#") or "=" not in line:
        continue
    candidate, value = line.split("=", 1)
    if candidate.strip() == key:
        found.append(value.strip())
if len(found) > 1:
    print(f"duplicate harness configuration key: {key}", file=sys.stderr)
    raise SystemExit(1)
if found:
    print(found[0])
    raise SystemExit(0)
raise SystemExit(3)
PY
}

get_value() {
  local key= mode= flag= default= env_name value status
  local flag_set=0 default_set=0
  while (($#)); do
    case "$1" in
      --key) [[ $# -ge 2 ]] || { usage; exit 2; }; key=$2; shift 2 ;;
      --mode) [[ $# -ge 2 ]] || { usage; exit 2; }; mode=$2; shift 2 ;;
      --flag) [[ $# -ge 2 ]] || { usage; exit 2; }; flag=$2; flag_set=1; shift 2 ;;
      --default) [[ $# -ge 2 ]] || { usage; exit 2; }; default=$2; default_set=1; shift 2 ;;
      -h|--help) usage; exit 0 ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ "$key" =~ ^[a-z0-9][a-z0-9.-]*$ ]] || die 2 "invalid configuration key: $key"
  [[ -z "$mode" || "$mode" =~ ^[a-z0-9][a-z0-9-]*$ ]] || die 2 "invalid mode: $mode"

  if (( flag_set )); then
    printf '%s\n' "$flag"
    return
  fi
  env_name=$(printf 'HARNESS_%s' "$key" | tr '[:lower:]' '[:upper:]' | tr '.-' '__')
  if [[ ${!env_name+x} ]]; then
    printf '%s\n' "${!env_name}"
    return
  fi
  if [[ -n "$mode" && ( "$key" =~ ^role\.[a-z0-9-]+\.runtime$ || "$key" =~ ^role\.[a-z0-9-]+\.model\.[a-z0-9-]+$ ) ]]; then
    set +e
    value=$(conf_value "mode.$mode.$key")
    status=$?
    set -e
    if [[ $status -eq 0 ]]; then printf '%s\n' "$value"; return; fi
    [[ $status -eq 3 ]] || exit "$status"
  fi
  set +e
  value=$(conf_value "$key")
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then printf '%s\n' "$value"; return; fi
  [[ $status -eq 3 ]] || exit "$status"
  if (( default_set )); then
    printf '%s\n' "$default"
    return
  fi
  die 1 "no value configured for $key"
}

validate_config() {
  python3 - "$config" "$root" <<'PY'
import os
import re
import sys
from collections import Counter
from pathlib import Path

path = Path(sys.argv[1])
repo = Path(sys.argv[2]).resolve()
errors = []

try:
    lines = path.read_text(encoding="utf-8").splitlines()
except OSError as error:
    raise SystemExit(f"cannot read harness configuration: {path}: {error}")

values = {}
for number, raw in enumerate(lines, 1):
    line = raw.strip()
    if not line or line.startswith("#"):
        continue
    if "=" not in line:
        errors.append(f"{path}:{number}: expected key=value")
        continue
    key, value = (part.strip() for part in line.split("=", 1))
    if not re.fullmatch(r"[a-z0-9][a-z0-9.-]*", key):
        errors.append(f"{path}:{number}: invalid key {key!r}")
        continue
    if key in values:
        errors.append(f"{path}:{number}: duplicate key {key}")
        continue
    values[key] = value

for key in values:
    if key.startswith("mode.") and not re.fullmatch(
        r"mode\.[a-z0-9-]+\.role\.[a-z0-9-]+\.(?:runtime|model\.[a-z0-9-]+)", key
    ):
        errors.append(f"{key} is not a supported mode-scoped key")

if "harness.runtimes" not in values:
    errors.append("harness.runtimes is required")
runtimes = [item.strip() for item in values.get("harness.runtimes", "").split(",") if item.strip()]
if len(runtimes) != len(set(runtimes)):
    errors.append("harness.runtimes contains a duplicate runtime")
runtime_set = set(runtimes)
supported_runtimes = {"claude", "codex", "devin", "fake"}
for runtime in sorted(runtime_set - supported_runtimes):
    errors.append(f"harness.runtimes names unsupported runtime {runtime!r}")

runtime_key = re.compile(r"(?:^|\.)role\.[a-z0-9-]+\.runtime$")
for key, value in values.items():
    if runtime_key.search(key) and value != "main" and value not in runtime_set:
        errors.append(f"{key} names runtime {value!r} outside harness.runtimes")

tier_members = []
for key, value in values.items():
    if not re.fullmatch(r"model\.tier\.[1-9][0-9]*", key):
        continue
    for member in (item.strip() for item in value.split(",")):
        if not member:
            continue
        if ":" not in member:
            errors.append(f"{key} entry {member!r} is not runtime-qualified")
            continue
        runtime, model = member.split(":", 1)
        if not runtime or not model:
            errors.append(f"{key} entry {member!r} is not runtime-qualified")
            continue
        if runtime not in runtime_set:
            errors.append(f"{key} entry {member!r} names a runtime outside harness.runtimes")
        tier_members.append(member)
tier_counts = Counter(tier_members)

plain_model = re.compile(r"^role\.([a-z0-9-]+)\.model\.([a-z0-9-]+)$")
mode_model = re.compile(r"^mode\.([a-z0-9-]+)\.role\.([a-z0-9-]+)\.model\.([a-z0-9-]+)$")
configured_models = []
for key, value in values.items():
    match = plain_model.fullmatch(key) or mode_model.fullmatch(key)
    if match:
        runtime = match.groups()[-1]
        if runtime not in runtime_set:
            errors.append(f"{key} names runtime {runtime!r} outside harness.runtimes")
        configured_models.append((key, f"{runtime}:{value}"))
for key, qualified in configured_models:
    count = tier_counts.get(qualified, 0)
    if count != 1:
        errors.append(f"{key} model {qualified!r} appears in {count} model tiers; expected exactly one")

roles = set()
modes = set()
for key in values:
    match = re.fullmatch(r"role\.([a-z0-9-]+)\.(?:runtime|model\.[a-z0-9-]+)", key)
    if match and match.group(1) != "default":
        roles.add(match.group(1))
    match = re.fullmatch(r"mode\.([a-z0-9-]+)\.role\.([a-z0-9-]+)\.(?:runtime|model\.[a-z0-9-]+)", key)
    if match:
        modes.add(match.group(1))
        if match.group(2) != "default":
            roles.add(match.group(2))

def resolved(key, mode=None):
    if mode is not None and f"mode.{mode}.{key}" in values:
        return values[f"mode.{mode}.{key}"]
    return values.get(key)

for mode in [None, *sorted(modes)]:
    runtime = resolved("role.default.runtime", mode)
    if runtime and runtime != "main" and resolved(f"role.default.model.{runtime}", mode) is None:
        label = f"mode {mode!r} role.default" if mode else "role.default"
        errors.append(f"{label} resolves to {runtime} but has no model.{runtime} value")

for role in sorted(roles):
    for mode in [None, *sorted(modes)]:
        runtime = resolved(f"role.{role}.runtime", mode) or resolved("role.default.runtime", mode)
        if not runtime or runtime == "main":
            continue
        model = resolved(f"role.{role}.model.{runtime}", mode) or resolved(f"role.default.model.{runtime}", mode)
        label = f"mode {mode!r} role {role}" if mode else f"role {role}"
        if model is None:
            errors.append(f"{label} resolves to {runtime} but has no model.{runtime} value")

evidence = values.get("evidence.root", "")
if not evidence:
    errors.append("evidence.root is required")
elif not os.path.isabs(evidence):
    errors.append("evidence.root must be absolute")
else:
    resolved_evidence = Path(evidence).resolve(strict=False)
    try:
        resolved_evidence.relative_to(repo)
    except ValueError:
        pass
    else:
        errors.append("evidence.root must be outside the repository")

# Registration is adopted-repository state, not a template invariant. The fake
# runtime is a fixture adapter and deliberately has no external registration.
if not (repo / "meta" / "harness-design.md").is_file():
    registration_roots = {
        "claude": [".claude/skills", ".claude/agents"],
        "codex": [".agents/skills"],
        "devin": [".agents/skills", ".devin/skills", ".devin/agents"],
    }
    for runtime in sorted(runtime_set):
        for relative in registration_roots.get(runtime, []):
            if not (repo / relative).is_dir():
                errors.append(
                    f"harness.runtimes enables {runtime!r} but registration directory {relative} is missing"
                )

for error in errors:
    print(f"invalid harness configuration: {error}", file=sys.stderr)
raise SystemExit(1 if errors else 0)
PY
}

command=${1:-}
[[ -n "$command" ]] || { usage; exit 2; }
shift
case "$command" in
  get) get_value "$@" ;;
  validate) (($# == 0)) || { usage; exit 2; }; validate_config ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
