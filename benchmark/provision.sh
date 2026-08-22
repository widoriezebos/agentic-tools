#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: benchmark/provision.sh --case <caseId>@<caseVersion> --config <configId>@<configVersion> --target <dir-or-name>
       benchmark/provision.sh --spec <legacy-id> --target <dir-or-name>      (alias mode)

A benchmark RUN is one benchmark CASE version (what is built and judged:
benchmark/cases/<id>/<version>/) under one benchmark CONFIGURATION version (who
builds it and under what limits: benchmark/configurations/<id>/<version>.json).
Both are pinned by version; the kit refuses an unpinned reference. `--spec` names
a retired spec id from benchmark/aliases.json and provisions the same pair with
the legacy naming (mission id, contract, tag) kept, so cohorts begun before the
migration stay uniform. See benchmark/README.md.

A bare NAME (no slash) resolves under the trials root: $METASYSTEM_TRIALS_ROOT
if set, else the first line of benchmark/trials-root.local if present, else
the repository's parent directory (the historical default). A path containing
a slash is honored verbatim.

Prepare a fresh local repository for one benchmark mission. The target must
not exist. Provisioning copies only the manifest-declared seed and instruments,
adopts the metasystem, creates and pushes to a sibling bare origin, writes an
unsigned mission contract, and arms supervision. Sealing and signing remain
human actions.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

# The kit lives beside the metasystem, never inside it: dependencies point
# one way, kit into equipment. root is the metasystem checkout.
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../metasystem" && pwd -P)
kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# The engine decides identity, leases, and the census. A snapshot carries no
# built binary (bin/ is ignored), so build one exactly as adoption does.
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
if [[ ! -x "$ms" ]]; then
  command -v go >/dev/null 2>&1 \
    || { echo "provision refused: the metasystem engine is not built and go is unavailable" >&2; exit 1; }
  (cd "$root" && go build -o bin/metasystem ./cmd/metasystem) \
    || { echo "provision refused: could not build the metasystem engine" >&2; exit 1; }
  ms="$root/bin/metasystem"
fi
spec_arg=
case_arg=
config_arg=
target_arg=

while (($#)); do
  case "$1" in
    --spec) [[ $# -ge 2 ]] || { usage; exit 2; }; spec_arg=$2; shift 2 ;;
    --case) [[ $# -ge 2 ]] || { usage; exit 2; }; case_arg=$2; shift 2 ;;
    --config) [[ $# -ge 2 ]] || { usage; exit 2; }; config_arg=$2; shift 2 ;;
    --target) [[ $# -ge 2 ]] || { usage; exit 2; }; target_arg=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

[[ -n "$target_arg" ]] || { usage; exit 2; }
if [[ -n "$case_arg" || -n "$config_arg" ]]; then
  [[ -z "$spec_arg" ]] || die 2 "provision refused: --spec cannot be combined with --case/--config"
  [[ -n "$case_arg" && -n "$config_arg" ]] || die 2 "provision refused: both --case <id>@<version> and --config <id>@<version> are required"
else
  [[ -n "$spec_arg" ]] || { usage; exit 2; }
fi

# Resolve WHAT runs. The one resolver (benchmark/pairs.py) turns a pinned
# case version + configuration version — or a legacy alias — into a merged
# manifest of exactly the shape this script has always consumed, plus the
# pair record (ids, versions, and the git object ids that pin the bytes).
# The case's files are extracted from the pinned tree object, never read from
# the working directory, so an ignored or uncommitted file can neither ride
# along nor evade the pin.
pair_scratch=$(mktemp -d)
trap 'rm -rf -- "$pair_scratch"' EXIT
if true; then
  [[ -z "$spec_arg" || ! -d "$spec_arg" ]] \
    || die 2 "provision refused: --spec names a retired spec id (an alias), not a directory; benchmark/specs no longer exists — use --case <id>@<version> --config <id>@<version>"
  if [[ -n "$spec_arg" ]]; then
    resolve_out=$(python3 "$kit/pairs.py" resolve --kit "$kit" --spec "$spec_arg" --out "$pair_scratch/manifest.json") || exit 2
  else
    resolve_out=$(python3 "$kit/pairs.py" resolve --kit "$kit" --case "$case_arg" --config "$config_arg" --out "$pair_scratch/manifest.json") || exit 2
  fi
  case_tree=$(printf '%s' "$resolve_out" | python3 -c 'import json,sys; print(json.load(sys.stdin)["pair"]["caseTree"])')
  pair_record=$(printf '%s' "$resolve_out" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["pair"]))')
  mkdir -p "$pair_scratch/case"
  # git archive of a bare tree object must run from the repository toplevel;
  # from a subdirectory it treats the cwd as a subtree path and refuses.
  kit_top=$(git -C "$kit" rev-parse --show-toplevel)
  git -C "$kit_top" archive "$case_tree" | tar -x -C "$pair_scratch/case" \
    || die 1 "provision refused: could not extract the pinned case tree $case_tree from the kit repository"
  spec=$pair_scratch/case
  manifest=$pair_scratch/manifest.json
fi

# Where trials live. A BARE NAME (no slash) resolves under the trials root;
# an explicit path (absolute or containing a slash) is honored verbatim, so
# existing callers keep their behavior. The trials root itself resolves:
#   1. $METASYSTEM_TRIALS_ROOT when set
#   2. the first line of benchmark/trials-root.local when present (gitignored,
#      the human's standing choice)
#   3. the repository's parent directory — the previous behavior, unchanged.
target=$(python3 - "$target_arg" "$root/.." <<'PY'
import os, sys
from pathlib import Path
arg, kit_root = sys.argv[1], Path(sys.argv[2])
if "/" in arg or arg in (".", ".."):
    print(Path(arg).resolve(strict=False)); raise SystemExit
env = os.environ.get("METASYSTEM_TRIALS_ROOT", "").strip()
local = kit_root / "benchmark" / "trials-root.local"
if env:
    root = Path(env).expanduser()
elif local.is_file():
    first = local.read_text().splitlines()
    root = Path(first[0].strip()).expanduser() if first and first[0].strip() else kit_root.parent
else:
    root = kit_root.parent
print((root / arg).resolve(strict=False))
PY
)
mkdir -p "$(dirname "$target")"
origin=$target.origin.git
evidence_root=$target.evidence
[[ ! -e "$target" && ! -L "$target" ]] \
  || die 1 "provision refused: target already exists: $target"
[[ ! -e "$origin" && ! -L "$origin" ]] \
  || die 1 "provision refused: local origin already exists: $origin"
[[ ! -e "$evidence_root" && ! -L "$evidence_root" ]] \
  || die 1 "provision refused: evidence root already exists: $evidence_root"

# Validate every copy source before creating any destination. In particular,
# the held-out grader boundary is checked by resolved path and through seed
# symlinks, even though adopt.sh independently excludes the whole benchmark kit.
manifest_facts=$(python3 - "$manifest" "$spec" <<'PY'
import json
import os
import re
import sys
from pathlib import Path

manifest_path = Path(sys.argv[1])
spec = Path(sys.argv[2]).resolve()

def refuse(message: str) -> None:
    raise SystemExit(f"provision refused: {message}")

try:
    value = json.loads(manifest_path.read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError) as error:
    refuse(f"manifest is unreadable: {error}")

def object_at(*parts: str) -> dict:
    current = value
    for part in parts:
        if not isinstance(current, dict) or part not in current:
            refuse("manifest is missing " + ".".join(parts))
        current = current[part]
    if not isinstance(current, dict):
        refuse("manifest field is not an object: " + ".".join(parts))
    return current

def text_at(mapping: dict, key: str, label: str) -> str:
    result = mapping.get(key)
    if not isinstance(result, str) or not result.strip() or "\n" in result or "\r" in result:
        refuse(f"{label} must be one non-empty line")
    return result

def relative_source(raw: object, label: str, base: Path) -> Path:
    if not isinstance(raw, str) or not raw or "\n" in raw or "\r" in raw:
        refuse(f"{label} must be one non-empty relative path")
    candidate = Path(raw)
    if candidate.is_absolute() or ".." in candidate.parts:
        refuse(f"{label} escapes the spec directory: {raw}")
    resolved = (base / candidate).resolve()
    try:
        resolved.relative_to(base)
    except ValueError:
        refuse(f"{label} escapes the spec directory: {raw}")
    return resolved

def fixture_spec() -> Path:
    """The directory the seed, instruments and grader are copied from: the
    case version tree, extracted from its pinned object by the resolver.
    (A case version is self-contained by design; the old fixtureSpec
    indirection between spec directories is gone with benchmark/specs.)"""
    if value.get("fixtureSpec") is not None:
        refuse("fixtureSpec is not a manifest key any more: a case version is self-contained (benchmark/cases/<id>/<version>)")
    return spec

def overlaps(first: Path, second: Path) -> bool:
    try:
        first.relative_to(second)
        return True
    except ValueError:
        pass
    try:
        second.relative_to(first)
        return True
    except ValueError:
        return False

identifier = text_at(value, "id", "id")
version = text_at(value, "version", "version")
if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", identifier):
    refuse("id is not a mission identifier")
if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", version):
    refuse("version cannot form an instrument tag")

copy_spec = fixture_spec()
seed_block = object_at("seed")
grader_block = object_at("grader")
seed = relative_source(seed_block.get("path"), "seed.path", copy_spec)
grader = relative_source(grader_block.get("path"), "grader.path", copy_spec)
if grader_block.get("heldOut") is not True:
    refuse("grader.heldOut must be true")
if not seed.is_dir():
    refuse(f"seed.path is not a directory: {seed}")
if not grader.is_dir():
    refuse(f"grader.path is not a directory: {grader}")
if overlaps(seed, grader):
    refuse(f"held-out grader path would be copied from seed.path: {grader}")

for directory, names, files in os.walk(seed, followlinks=False):
    for name in [*names, *files]:
        source = Path(directory, name)
        resolved = source.resolve()
        try:
            resolved.relative_to(copy_spec)
        except ValueError:
            refuse(f"seed path escapes the fixture spec directory: {source}")
        if overlaps(resolved, grader):
            refuse(f"held-out grader path would be copied through seed: {source}")

contract = object_at("missionContract")
instruments = contract.get("instruments")
if not isinstance(instruments, list) or not instruments:
    refuse("missionContract.instruments must be a non-empty list")
destinations = set()
for index, raw in enumerate(instruments):
    instrument = relative_source(raw, f"missionContract.instruments[{index}]", copy_spec)
    if not instrument.is_file():
        refuse(f"instrument is not a file: {instrument}")
    if overlaps(instrument, grader):
        refuse(f"held-out grader path would be copied as an instrument: {instrument}")
    destination = instrument.name
    if destination in destinations or (seed / destination).exists():
        refuse(f"instrument destination would overwrite another provisioned path: {destination}")
    destinations.add(destination)

gate = object_at("missionContract", "gate")
guard = object_at("missionContract", "guard")
truth = object_at("missionContract", "truth")
streams = object_at("missionContract", "streams")
envelope = object_at("missionContract", "envelope")
fences = object_at("fences")
completion = object_at("completionGate")
roster = object_at("roster")
host = object_at("roster", "host")
delegates = object_at("roster", "delegates")
environment = object_at("environment")

# A spec may pin itself to one operating system (machineConstraint.os);
# the devin acceptance probes are linux-vm-only by the human's D83
# ruling, and the refusal belongs HERE so no operator habit can start
# an untrusted-orchestrator run on the wrong machine.
constraint = value.get("machineConstraint")
if constraint is not None:
    import platform
    wanted = constraint.get("os", "")
    if not isinstance(wanted, str) or not wanted:
        refuse("machineConstraint.os must be a non-empty string when machineConstraint is present")
    running = platform.system().lower()
    if running != wanted:
        reason = constraint.get("reason", "")
        refuse(f"this spec is constrained to os={wanted} and refuses to provision on {running}: {reason}")

for mapping, key, label in (
    (gate, "command", "missionContract.gate.command"),
    (gate, "metric", "missionContract.gate.metric"),
    (gate, "direction", "missionContract.gate.direction"),
    (gate, "threshold", "missionContract.gate.threshold"),
    (gate, "paths", "missionContract.gate.paths"),
    (truth, "paths", "missionContract.truth.paths"),
    (truth, "certification", "missionContract.truth.certification"),
    (guard, "name", "missionContract.guard.name"),
    (guard, "metric", "missionContract.guard.metric"),
    (guard, "command", "missionContract.guard.command"),
    (contract, "exposure", "missionContract.exposure"),
    (completion, "command", "completionGate.command"),
    (host, "runtime", "roster.host.runtime"),
    (host, "model", "roster.host.model"),
):
    text_at(mapping, key, label)
if gate.get("refKind") != "tag":
    refuse("missionContract.gate.refKind must be tag")
if guard.get("name") != guard.get("metric"):
    refuse("missionContract.guard.name must equal its emitted metric")
if gate.get("direction") not in {"max", "min"}:
    refuse("missionContract.gate.direction must be max or min")
if truth.get("certification") not in {"candidate", "certified"}:
    refuse("missionContract.truth.certification must be candidate or certified")
if not re.fullmatch(r"(?:>=|<=|>|<)-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?", str(gate.get("threshold"))):
    refuse("missionContract.gate.threshold has invalid grammar")
for mapping, key, label, allow_zero in (
    (gate, "noiseFloor", "missionContract.gate.noiseFloor", True),
    (guard, "floor", "missionContract.guard.floor", True),
    (guard, "noise", "missionContract.guard.noise", True),
    (guard, "cadence", "missionContract.guard.cadence", False),
):
    number = mapping.get(key)
    if isinstance(number, bool) or not isinstance(number, (int, float)) or number < (0 if allow_zero else 1):
        refuse(f"{label} has an invalid numeric value")
for key in ("cycles", "jobs", "concurrency", "jobCapMin", "hostTurnCapMin", "ledgerCycleBudget", "ledgerNoGainBudget"):
    number = fences.get(key)
    if isinstance(number, bool) or not isinstance(number, int) or number < 1:
        refuse(f"fences.{key} must be a positive integer")
wall_clock = fences.get("wallClockHours")
if isinstance(wall_clock, bool) or not isinstance(wall_clock, (int, float)) or wall_clock <= 0:
    refuse("fences.wallClockHours must be positive")
if not streams or any(not re.fullmatch(r"[a-z0-9][a-z0-9-]*", key) or not isinstance(goal, str) or not goal.strip() for key, goal in streams.items()):
    refuse("missionContract.streams must contain named non-empty goals")
if not envelope or any(not re.fullmatch(r"[a-z0-9][a-z0-9-]*", key) or not isinstance(bound, str) or not bound.strip() for key, bound in envelope.items()):
    refuse("missionContract.envelope must contain bounded categories")

supported = {"claude", "codex", "devin"}
runtime_id = re.compile(r"[a-z0-9][a-z0-9-]*")
role_id = re.compile(r"[a-z0-9][a-z0-9-]*")
model_id = re.compile(r"[A-Za-z0-9][A-Za-z0-9._:/-]*")
host_runtime = host["runtime"]
host_model = host["model"]
if host_runtime not in supported or not runtime_id.fullmatch(host_runtime) or not model_id.fullmatch(host_model):
    refuse("roster.host contains an unsupported runtime or invalid model")
runtimes = [host_runtime]
models = {host_runtime: host_model}
DELEGATED_ROLES = ("design-critic", "implementer", "code-critic")
# The per-runtime model slots (role.*.model.<runtime>) belong to the
# DELEGATES; the host's model lives in the contract's host.model. So a
# delegate entry for the host's own runtime overrides the host model in
# that slot -- one runtime, two model identities, host and delegates
# distinct -- which is exactly what a Devin-hosted Opus/GPT roster needs.
for runtime, model in delegates.items():
    if runtime not in supported or not runtime_id.fullmatch(runtime) or not isinstance(model, str) or not model_id.fullmatch(model):
        refuse(f"roster.delegates contains an unsupported runtime or invalid model: {runtime}")
    models[runtime] = model
    if runtime not in runtimes:
        runtimes.append(runtime)
delegate_roles = roster.get("delegateRoles")
if delegate_roles is not None:
    # Per-role resolution is OPTIONAL and, when present, must be complete and
    # agree with roster.delegates: every consumer that predates delegateRoles
    # (the extractor, cohort records, benchmark-identity) still reads
    # delegates, so the two are one roster in two projections, never two
    # authorities.
    if not isinstance(delegate_roles, dict) or set(delegate_roles) != set(DELEGATED_ROLES):
        refuse(f"roster.delegateRoles must name exactly the delegated roles {list(DELEGATED_ROLES)}")
    for role in DELEGATED_ROLES:
        resolution = delegate_roles[role]
        if not isinstance(resolution, dict):
            refuse(f"roster.delegateRoles.{role} must be an object")
        runtime = text_at(resolution, "runtime", f"roster.delegateRoles.{role}.runtime")
        model = text_at(resolution, "model", f"roster.delegateRoles.{role}.model")
        if runtime not in supported or not runtime_id.fullmatch(runtime) or not model_id.fullmatch(model):
            refuse(f"roster.delegateRoles contains an unsupported runtime or invalid model: {role}")
        if delegates.get(runtime) != model:
            refuse(f"roster.delegateRoles.{role} ({runtime}:{model}) is not covered by roster.delegates; the two projections must agree")
        if runtime not in runtimes:
            runtimes.append(runtime)
independence = roster.get("independence")
if independence is not None and independence != "session-only":
    refuse("roster.independence admits only the literal session-only; omit the key otherwise")
# The collaboration loop refuses to merge when implementer and code critic
# share an effective model unless independence=session-only is declared
# (docs/orchestration.md step 4, validate/conformance.go). Catch that here,
# where it costs nothing, instead of in cycle 2 of a run.
if delegate_roles is not None:
    implementer_pair = (delegate_roles["implementer"]["runtime"], delegate_roles["implementer"]["model"])
    critic_pair = (delegate_roles["code-critic"]["runtime"], delegate_roles["code-critic"]["model"])
    same_model = implementer_pair == critic_pair
    if same_model and independence != "session-only":
        refuse("roster.delegateRoles puts the implementer and the code critic on one effective model without roster.independence=session-only; the merge check would refuse every critique (docs/orchestration.md step 4)")
elif len(delegates) == 1 and independence != "session-only":
    # The pending ruling landed (issue #7): a single-model roster without
    # an explicit independence declaration is impossible to provision, not
    # discovered in cycle 2 of a paid run. Every shipped single-model spec
    # now declares session-only by name; bm-1 carries a per-role critic.
    refuse("roster.delegates resolves the implementer and the code critic to one effective model and roster.independence is not declared; give the code critic a distinct model via roster.delegateRoles or declare roster.independence=session-only (docs/orchestration.md step 4, issue #7)")
network = environment.get("delegateNetwork")
if network not in {"allowed", "denied"}:
    refuse("environment.delegateNetwork must be allowed or denied")

print(",".join(runtimes))
print(f"{identifier}-instruments-v{version}")
print(identifier)
print(host_runtime)
print(copy_spec)
print(independence or "")
PY
)
runtimes=$(printf '%s\n' "$manifest_facts" | sed -n '1p')
instrument_tag=$(printf '%s\n' "$manifest_facts" | sed -n '2p')
mission_id=$(printf '%s\n' "$manifest_facts" | sed -n '3p')
host_runtime=$(printf '%s\n' "$manifest_facts" | sed -n '4p')
fixture_spec=$(printf '%s\n' "$manifest_facts" | sed -n '5p')
roster_independence=$(printf '%s\n' "$manifest_facts" | sed -n '6p')
[[ -n "$runtimes" && -n "$instrument_tag" && -n "$mission_id" && -n "$host_runtime" && -n "$fixture_spec" ]] \
  || die 1 "provision refused: manifest validation returned incomplete facts"

scratch=$(mktemp -d)
trap 'rm -rf "$scratch" "$pair_scratch"' EXIT

# PI-R1-004 (plans/provisioning-identity.md D-P1.1): a target that already
# exists and carries a lease record is residue of a dead provisioning; the
# next attempt IS the recovery. A live or unproven holder refuses loudly.
if [[ -e "$target" ]] && [[ -n "$(ls -A "$target" 2>/dev/null)" ]]; then
  # Proof and deletion are ONE operation in the engine: the verb refuses a
  # path not shaped like a checkout, a target without a lease record, and a
  # live or unprovable holder — so no edit here can reorder the guards away
  # from the delete.
  "$ms" lease reclaim --target "$target" \
    || die 1 "provision refused: target exists and could not be reclaimed: $target"
fi
mkdir -p "$target"
git -C "$target" init -q -b main
seed_path=$(python3 - "$manifest" <<'PY'
import json, sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["seed"]["path"])
PY
)
cp -R "$fixture_spec/$seed_path/." "$target/"

if ! "$root/scripts/adopt.sh" "$target" --runtimes "$runtimes" >"$scratch/adopt.log" 2>&1; then
  cat "$scratch/adopt.log" >&2
  die 1 "provision failed while adopting the metasystem"
fi

# The run's benchmark identity: which case version under which configuration
# version, pinned by git object id, and the legacy id when provisioned
# through an alias. run-cohort folds it into benchmark-identity.json and the
# extractor reads it; a legacy directory-mode provision writes none.
if [[ -n "${pair_record:-}" ]]; then
  mkdir -p "$target/artifacts/agents"
  printf '%s\n' "$pair_record" | python3 -c 'import json,sys; d=json.load(sys.stdin); json.dump(d, open(sys.argv[1],"w"), indent=2, sort_keys=True); open(sys.argv[1],"a").write("\n")' "$target/artifacts/agents/benchmark-pair.json"
fi

# D-P1.2 (plans/provisioning-identity.md): the provisioner is the target's
# first main. Announce AFTER adoption (the helpers now exist), then VERIFY
# holdership — announce alone does not fail against a live holder.
provisioner_start=$("$ms" proc started-at --pid $$) \
  || die 1 "provision refused: cannot read the provisioner's own start time"
"$ms" lease announce --root "$target" \
  --session "provision-$mission_id" --pid $$ --start "$provisioner_start" \
  --tag "metasystem-provisioner-$mission_id" --runtime "$host_runtime" >/dev/null \
  || die 1 "provision refused: could not announce the provisioner in the target"
"$ms" lease require-holder --root "$target" --caller-pid $$ >/dev/null \
  || die 1 "provision refused: the provisioner did not become the target's lease holder"

mkdir -p "$evidence_root"
python3 - "$target/metasystem.conf" "$manifest" "$evidence_root" "$roster_independence" <<'PY'
import json
import os
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
manifest = json.load(open(sys.argv[2], encoding="utf-8"))
evidence = str(Path(sys.argv[3]).resolve())
independence = sys.argv[4]
roster = manifest["roster"]
host = roster["host"]
# The per-runtime model slots are the roster's FIRST projection and the one
# every existing spec is provisioned from: host first, then each delegate
# runtime, exactly as before delegateRoles existed. model.tier.1 keeps the
# host pair (it always has) so the dispatch cost ranking is unchanged.
models = {host["runtime"]: host["model"], **roster["delegates"]}
tier_pairs = [(host["runtime"], host["model"])]
for runtime, model in roster["delegates"].items():
    if (runtime, model) not in tier_pairs:
        tier_pairs.append((runtime, model))
qualified = ",".join(f"{runtime}:{model}" for runtime, model in tier_pairs)
# The per-role projection is OPTIONAL. When present it is complete (the
# manifest validation above refused anything else) and it agrees with the
# per-runtime slots, so writing role.<role>.runtime / role.<role>.model.<rt>
# only makes explicit what the runtime slots already imply -- it never
# touches role.default.*, which stays whatever adoption tailored, exactly
# as legacy provisioning left it.
delegate_roles = roster.get("delegateRoles") or {}
role_resolutions = {
    role: (resolution["runtime"], resolution["model"])
    for role, resolution in delegate_roles.items()
}
network = {"allowed": "allow", "denied": "deny"}[manifest["environment"]["delegateNetwork"]]
model_key = re.compile(r"^(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$")
role_runtime_key = re.compile(r"^role\.([a-z0-9-]+)\.runtime$")
role_model_key = re.compile(r"^role\.([a-z0-9-]+)\.model\.([a-z0-9-]+)$")
seen_models = set()
seen_role_runtimes = set()
seen_role_models = set()
seen_network = False
seen_independence = False
lines = []
for raw in path.read_text(encoding="utf-8").splitlines():
    if "=" not in raw or raw.lstrip().startswith("#"):
        lines.append(raw)
        continue
    key, old = raw.split("=", 1)
    if key == "evidence.root":
        value = evidence
    elif key == "model.tier.1":
        value = qualified
    elif re.fullmatch(r"model\.tier\.[2-9][0-9]*", key):
        value = ""
    elif (match := role_runtime_key.fullmatch(key)) and match.group(1) in role_resolutions:
        value = role_resolutions[match.group(1)][0]
        seen_role_runtimes.add(match.group(1))
    elif (match := role_model_key.fullmatch(key)) and match.group(1) in role_resolutions and match.group(2) == role_resolutions[match.group(1)][0]:
        value = role_resolutions[match.group(1)][1]
        seen_role_models.add(match.group(1))
        seen_models.add(match.group(2))
    elif (match := model_key.fullmatch(key)) and match.group(1) in models:
        runtime = match.group(1)
        value = models[runtime]
        seen_models.add(runtime)
    elif key == "dispatch.permissions.network":
        value = network
        seen_network = True
    elif key == "independence":
        value = independence
        seen_independence = True
    else:
        value = old
    lines.append(f"{key}={value}")
if set(models) != seen_models:
    missing = ", ".join(sorted(set(models) - seen_models))
    raise SystemExit(f"provision refused: adopted configuration has no model slot for runtime(s): {missing}")
for role, (runtime, model) in role_resolutions.items():
    if role not in seen_role_runtimes:
        lines.append(f"role.{role}.runtime={runtime}")
    if role not in seen_role_models:
        lines.append(f"role.{role}.model.{runtime}={model}")
if not seen_network:
    lines.append(f"dispatch.permissions.network={network}")
if independence and not seen_independence:
    lines.append(f"independence={independence}")
temporary = path.with_name(path.name + ".provision.tmp")
temporary.write_text("\n".join(lines) + "\n", encoding="utf-8")
os.replace(temporary, path)
PY
"$target/scripts/metasystem-config.sh" validate

# The adopted target must survive its own landing boundary: the
# instruments commit below rides the target's commit.sh, whose IL-28
# static re-proof audits docs/project-rules.md for template
# placeholders. A benchmark target's answers are all knowable here, so
# provisioning fills the template's placeholders with the mission's
# concrete facts — keeping every machine-read section (the envelope
# eligibility table the contract preflight parses) byte-intact.
python3 - "$target" "$manifest" <<'PY'
import json, sys
from pathlib import Path
target = Path(sys.argv[1])
manifest = json.load(open(sys.argv[2], encoding="utf-8"))
contract = manifest["missionContract"]
gate = contract["gate"]
rules_path = target / "docs" / "project-rules.md"
text = rules_path.read_text(encoding="utf-8")
evidence = "recorded in metasystem.conf (evidence.root, filled by the provisioner)"
fills = {
    "<template sha>": "benchmark scratch target; the adopt record carries the template identity",
    "<one paragraph>": "The benchmark mission's product: a scratch target provisioned for one sealed mission contract, graded by a held-out grader afterwards.",
    "<paths and ownership>": "none",
    "<paths>": "named by the sealed mission contract",
    "<path outside the repository>": evidence,
    "<durable evidence root, outside the repository>": evidence,
    "<command>": gate["command"],
    "<list them here>": "none",
    "<sources and handling>": "none; the seed, spec, and sealed contract are the only inputs",
    "<forbidden list>": "none beyond the runtime defaults",
    "<policy>": "no egress beyond the roster runtimes' own APIs",
    "<location>": "metasystem.conf",
    "<amount and period>": "none; cohort spend is ruled at the human seal",
    "<warning threshold>": "not applicable in a benchmark target",
    "<who approves>": "the human at the seal boundary",
    "<usage source>": "the adapters' usage records under artifacts/",
    "<cheapest model class>": "per the sealed contract's roster",
    "<middle model class>": "per the sealed contract's roster",
    "<costliest model class>": "per the sealed contract's roster",
}
for marker, value in fills.items():
    text = text.replace(marker, value)
rules_path.write_text(text, encoding="utf-8")
PY

while IFS= read -r instrument; do
  [[ -n "$instrument" ]] || continue
  cp "$fixture_spec/$instrument" "$target/$(basename "$instrument")"
done < <(python3 - "$manifest" <<'PY'
import json, sys
for path in json.load(open(sys.argv[1], encoding="utf-8"))["missionContract"]["instruments"]:
    print(path)
PY
)

git init -q --bare -b main "$origin"
git -C "$target" remote add origin "$origin"
git -C "$target" add -A
(cd "$target" && scripts/agents/commit.sh -qm "Provision benchmark $mission_id instruments") \
  || die 1 "provision refused: the instruments commit was not wrapper-carried"
git -C "$target" tag "$instrument_tag"

contract_rel=plans/mission-$mission_id.contract.md
contract=$target/$contract_rel
python3 - "$manifest" "$contract" "$instrument_tag" <<'PY'
import json
import os
import sys
from pathlib import Path

manifest = json.load(open(sys.argv[1], encoding="utf-8"))
output = Path(sys.argv[2])
instrument_tag = sys.argv[3]
contract = manifest["missionContract"]
gate = contract["gate"]
guard = contract["guard"]
truth = contract["truth"]
fences = manifest["fences"]
host = manifest["roster"]["host"]
grader_path = manifest["grader"]["path"]

def scalar(value: object) -> str:
    return str(value)

lines = [
    f"# {manifest['id']} Mission Contract",
    "",
    "# Intent",
    "",
    f"Build {manifest['title']}.",
    f"Completion is the manifest's completion gate: {manifest['completionGate']['command']}.",
    "",
    "# Non-goals",
    "",
    f"Do not copy, inspect, or run the held-out grader at `{grader_path}` during the mission.",
    "",
    "# Initial streams",
    "",
]
for stream, goal in contract["streams"].items():
    lines.append(f"- `{stream}`: {goal}")
lines.extend([
    "",
    "```mission",
    f"gate.command={gate['command']}",
    f"gate.ref={instrument_tag}",
    f"gate.paths={gate['paths']}",
    f"truth.paths={truth['paths']}",
    f"truth.certification={truth['certification']}",
    f"gate.direction={gate['direction']}",
    f"gate.threshold.{gate['metric']}={gate['threshold']}",
    f"gate.noise-floor.{gate['metric']}={scalar(gate['noiseFloor'])}",
    f"guard.{guard['name']}.command={guard['command']}",
    f"guard.{guard['name']}.floor={scalar(guard['floor'])}",
    f"guard.{guard['name']}.noise={scalar(guard['noise'])}",
    f"guard.cadence={scalar(guard['cadence'])}",
    f"ledger.cycle-budget={fences['ledgerCycleBudget']}",
    f"ledger.no-gain-budget={fences['ledgerNoGainBudget']}",
    f"fence.wall-clock-hours={scalar(fences['wallClockHours'])}",
    f"fence.cycles={fences['cycles']}",
    f"fence.jobs={fences['jobs']}",
    f"fence.concurrency={fences['concurrency']}",
    f"fence.job-cap-min={fences['jobCapMin']}",
    f"host.runtime={host['runtime']}",
    f"host.model={host['model']}",
    f"host.turn-cap-min={fences['hostTurnCapMin']}",
])
# The sealed host caps (issue #6/#8): the manifest is the single authority
# for mission-contract policy, so the adapter's native turn and budget
# caps become sealed values instead of unsealed defaults. The binary-gate
# fuse acknowledgment rides only when a manifest explicitly declares it —
# bm-1's ruling raised the no-gain budget to the cycle fence instead.
if "hostMaxTurns" in fences:
    lines.append(f"host.max-turns={fences['hostMaxTurns']}")
if "hostMaxBudgetUsd" in fences:
    lines.append(f"host.max-budget-usd={fences['hostMaxBudgetUsd']}")
if fences.get("acceptBinaryGateFuse") is True:
    lines.append("ledger.accept-binary-gate-fuse=true")
for stream, goal in contract["streams"].items():
    lines.append(f"stream.{stream}={goal}")
for category, bound in contract["envelope"].items():
    lines.append(f"envelope.{category}={bound}")
lines.extend([f"exposure={contract['exposure']}", "```", ""])
output.parent.mkdir(parents=True, exist_ok=True)
temporary = output.with_name(output.name + ".provision.tmp")
temporary.write_text("\n".join(lines), encoding="utf-8")
os.replace(temporary, output)
PY

"$target/scripts/assert-mission.sh" --file "$contract" >/dev/null
git -C "$target" add "$contract_rel"
# The contract is a deliberately added plan file; say so through the guard's
# front door instead of weakening the guard.
(cd "$target" && METASYSTEM_ALLOW_NEW_PLAN=1 scripts/agents/commit.sh -qm "Add unsigned $mission_id mission contract") \
  || die 1 "provision refused: the contract commit was not wrapper-carried"
git -C "$target" push -q -u origin main
git -C "$target" push -q origin "refs/tags/$instrument_tag"
git -C "$target" remote set-head origin main

# D-P1.4: the provisioner's identity ends with its own invocation; the
# human's later seal/sign commits are sovereign, and the runner's resume
# establishes its own identity.
"$ms" lease retire --root "$target" \
  --session "provision-$mission_id" --pid $$ --start "$provisioner_start" >/dev/null \
  || die 1 "provision refused: could not retire the provisioner's announcement"
# ... and RELEASE the checkout the way a departing main does (the S4-8
# precedent): the lease record goes with the retired announcement, because a
# lease naming a retired-but-live pid locks the next identity out (KI-33) —
# the arming below establishes its own identity on an unheld checkout.
rm -f "$target/artifacts/agents/mains/worktree-lease.json"

provision_started=$("$ms" proc started-at --pid "$$")
if ! METASYSTEM_AGENT_RUNTIME="$host_runtime" \
  "$target/scripts/agents/arm-supervision.sh" --repo "$target" \
    --session "benchmark-provision-$mission_id-$$" --pid "$$" \
    --start-time "$provision_started" --tag "benchmark/provision.sh" \
    >"$scratch/arm.log" 2>&1; then
  cat "$scratch/arm.log" >&2
  "$target/scripts/agents/arm-supervision.sh" --repo "$target" --shutdown >/dev/null 2>&1 || true
  die 1 "provision failed while arming supervision"
fi
if ! grep -q ' ARMED ' "$scratch/arm.log"; then
  "$target/scripts/agents/arm-supervision.sh" --repo "$target" --shutdown >/dev/null 2>&1 || true
  die 1 "provision failed: supervision returned without an ARMED verdict"
fi

printf 'Review %s\n' "$contract"
printf 'Seal it: (cd %q && scripts/assert-mission.sh --seal --file %q)\n' "$target" "$contract_rel"
printf 'Sign it: add the Approval line using the hash printed by --seal, then commit and push the signed contract.\n'
