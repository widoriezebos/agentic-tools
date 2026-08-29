#!/usr/bin/env bash
set -euo pipefail

# Focused fixtures for section 3.11. Every wait in this file goes through a
# named ceiling so a broken supervisor fails loudly instead of hanging (IL-1).

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$source_root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$source_root"
fixture_ceiling_sec=$(harness_fixture_cap supervision-wait)

tmp=$(mktemp -d)
owned_pids=()
fixture_harness_roots=()
# Every detached owner inherits this run-scoped registry home. A direct fixture
# run must never need write access to, or append evidence into, the operator's
# real home directory.
export METASYSTEM_SUPERVISION_REGISTRY_HOME="$tmp/registry-home"
mkdir -p "$METASYSTEM_SUPERVISION_REGISTRY_HOME"

ms="${METASYSTEM_BIN:-$source_root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "supervision fixtures: binary absent; run the go gate first" >&2; exit 1; }

assert_scratch_scoped_announcement_calls() {
  local actual expected
  actual=$(awk '
    BEGIN {
      helper = "become_" "main"
      lease = "lea" "se"
      announce = "ann" "ounce"
      word_left = "(^|[^[:alnum:]_])"
      word_right = "([^[:alnum:]_]|$)"
    }
    function helper_occurrences(text, file, line_number,    scan, hit, token_at, tail, root) {
      scan = text
      while (match(scan, word_left helper word_right)) {
        hit = substr(scan, RSTART, RLENGTH)
        token_at = index(hit, helper)
        tail = substr(scan, RSTART + token_at - 1 + length(helper))
        root = ""
        if (tail ~ /^[[:space:]]/) {
          sub(/^[[:space:]]+/, "", tail)
          root = tail
          sub(/[[:space:];&|].*$/, "", root)
        }
        print file ":call:" root
        scan = substr(scan, RSTART + token_at - 1 + length(helper))
      }
    }
    function direct_occurrences(text, file, line_number,    scan, occurrence_start, occurrence_length, tail, command, root) {
      scan = text
      while (match(scan, word_left lease "[[:space:]]+" announce word_right)) {
        occurrence_start = RSTART
        occurrence_length = RLENGTH
        tail = substr(scan, occurrence_start + occurrence_length)
        command = tail
        sub(/[;&|].*$/, "", command)
        root = ""
        if (match(command, /(^|[[:space:]])--root[[:space:]]+[^[:space:];&|]+/)) {
          root = substr(command, RSTART, RLENGTH)
          sub(/^.*--root[[:space:]]+/, "", root)
        }
        print file ":direct:" root
        scan = substr(scan, occurrence_start + occurrence_length)
      }
    }
    {
      file = substr(FILENAME, length(source_prefix) + 1)
      helper_occurrences($0, file, FNR)
      direct_occurrences($0, file, FNR)
    }
  ' source_prefix="$source_root/" \
    "$source_root/scripts/agents/supervision-fixtures.sh" \
    "$source_root/scripts/validate-metasystem.sh" | LC_ALL=C sort)
  # The allowlist keys on (file, kind, root) — NEVER line numbers,
  # which rot under any unrelated edit above a call site (found live
  # 2026-08-25: the perl removal shifted this very file and broke the
  # nested adopt validation). Multiplicity still guards: sort keeps
  # duplicates, so a second call with an allowlisted root shape still
  # mismatches the exact inventory.
  expected=$(printf '%s\n' \
    'scripts/agents/supervision-fixtures.sh:call:announced' \
    'scripts/agents/supervision-fixtures.sh:call:"$repo"' \
    'scripts/agents/supervision-fixtures.sh:call:"$foreign/repo"' \
    'scripts/agents/supervision-fixtures.sh:direct:"$stop_root"' \
    'scripts/agents/supervision-fixtures.sh:direct:"$stop_root"' \
    'scripts/agents/supervision-fixtures.sh:call:' \
    'scripts/agents/supervision-fixtures.sh:direct:"$repo"' \
    'scripts/agents/supervision-fixtures.sh:call:"$gate_repo"' | LC_ALL=C sort)
  [[ "$actual" == "$expected" ]] || {
    echo "announcement call sites are not exactly scratch-scoped:" >&2
    printf '%s\n' "$actual" >&2
    return 1
  }
}
assert_scratch_scoped_announcement_calls

process_started_at() {
  "$ms" proc started-at --pid "$1"
}

process_identity_alive() { # pid, start
  "$ms" proc alive --pid "$1" --start-time "$2" \
    --root "${repo:-$source_root}" >/dev/null 2>&1
}

wait_until() { # name, shell predicate...
  local name=$1 started=$SECONDS deadline=$((SECONDS + fixture_ceiling_sec)) elapsed
  shift
  until "$@"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "supervision fixture timed out: $name (elapsed: ${elapsed}s; scaled cap: ${fixture_ceiling_sec}s)" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

# --- Attempt-based patience for census waits (records/patience/patience-attempts.md) ------
# `wait_until` above is wall-clock: correct for OS-event waits (a PID dying),
# but load-fragile for waits on a CENSUS PASS's effect (KI-37). `wait_for_census`
# measures patience in the census actor's ATTEMPTS (published scan passes,
# counted by the monotonic `scanSeq` the watcher stamps) instead of seconds:
#  - Tier 1 (semantic): succeed when the predicate holds on a PROVABLY-FRESH pass
#    (scanSeq >= base+2, which — the watcher loop being single-flight — proves the
#    pass read input written after `base` was sampled). Fail once K fresh passes
#    (base+2 .. base+1+K) have completed still-false: a genuine defect, not a slow
#    box. Load-independent for the cumulative wait.
#  - Tier 2 (failsafe): if NO pass has COMPLETED for a generous silence window
#    (derived from the verdict's own intervalSec, never an env guess), abort so a
#    wedged census can't hang CI. Coarse and honestly labelled.
# Predicates are LEVEL-TRIGGERED: the fixture plants state that persists across
# passes, so once true it stays true until observed (what makes freshness and
# skipped-publication safe). Each poll snapshots last-census.json ONCE so the
# marker and the predicate bind to the SAME verdict.
CENSUS_ATTEMPT_BUDGET=${METASYSTEM_CENSUS_ATTEMPT_BUDGET:-2}   # K: fresh passes allowed
CENSUS_SILENCE_FLOOR_SEC=${METASYSTEM_CENSUS_SILENCE_FLOOR_SEC:-60}
CENSUS_SILENCE_MULT=${METASYSTEM_CENSUS_SILENCE_MULT:-30}

census_field() { # snapshot-file, field -> value or empty (never fails the caller)
  "$ms" json get --file "$1" --field "$2" 2>/dev/null || true
}

wait_for_census() { # name, predicate-fn, [predicate-args...]
  local name=$1 predicate=$2; shift 2
  local snap="$tmp/wait-census-snapshot.json"
  local base m interval t_silence last_marker last_adv rebaselines=0
  local max_rebaselines=${METASYSTEM_CENSUS_MAX_REBASELINES:-5}
  cp "$last" "$snap" 2>/dev/null || : >"$snap"
  base=$(census_field "$snap" scanSeq); [[ "$base" =~ ^[0-9]+$ ]] || base=0
  last_marker=$base; last_adv=$SECONDS
  while :; do
    cp "$last" "$snap" 2>/dev/null || : >"$snap"   # atomic-rename source => a consistent snapshot
    m=$(census_field "$snap" scanSeq)
    if [[ "$m" =~ ^[0-9]+$ ]]; then
      if (( m < last_marker )); then
        # A fresh writer took over (seed-from-file normally prevents this);
        # re-baseline so the new writer earns its own K fresh passes. Bound it:
        # a writer that OSCILLATES scanSeq would otherwise reset both tiers
        # forever, so repeated regressions are themselves a failure (an unstable
        # writer), not something to wait out indefinitely.
        rebaselines=$((rebaselines + 1))
        if (( rebaselines > max_rebaselines )); then
          echo "$name: census scanSeq regressed $rebaselines times (writer unstable; last $last_marker -> $m)" >&2
          return 1
        fi
        base=$m; last_marker=$m; last_adv=$SECONDS
      elif (( m > last_marker )); then
        last_marker=$m; last_adv=$SECONDS
      fi
      if (( m >= base + 2 )) && "$predicate" "$snap" "$@"; then
        return 0
      fi
      if (( m >= base + 1 + CENSUS_ATTEMPT_BUDGET )) && ! "$predicate" "$snap" "$@"; then
        echo "$name: still wrong after $CENSUS_ATTEMPT_BUDGET fresh census passes (scanSeq $base -> $m)" >&2
        # The census snapshot that failed the predicate is the diagnosis;
        # a swept bed leaves the next reader nothing, so say it here.
        echo "$name: failing census snapshot follows" >&2
        cat "$snap" >&2 || true
        return 1
      fi
    fi
    interval=$(census_field "$snap" intervalSec)
    [[ "$interval" =~ ^[1-9][0-9]*$ ]] || interval=1
    t_silence=$(( CENSUS_SILENCE_MULT * interval ))
    (( t_silence < CENSUS_SILENCE_FLOOR_SEC )) && t_silence=$CENSUS_SILENCE_FLOOR_SEC
    if (( SECONDS - last_adv >= t_silence )); then
      echo "$name: no completed census pass for ${t_silence}s (census wedged, or one pass exceeded it; scanSeq stuck at $last_marker)" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

# Level-triggered census predicates. Each takes the snapshot file as $1; the
# planted fixture state makes them stay true until observed. `inventory_has`
# (defined below) is already file-first, so it is used directly as a predicate.
pred_any() { # snapshot (ignored): satisfied by any fresh pass — "the census ran again"
  return 0
}
pred_verdict_is() { # snapshot, expected-verdict
  [[ "$(census_field "$1" verdict)" == "$2" ]]
}
# The two array predicates below match against `json get`'s compact
# rendering of a string array. The needles used here carry no
# JSON-escapable characters, so a substring hit is exactly "some element
# contains the needle"; an element START is a quote right after the
# opening bracket or a separating comma, and a mid-element quote renders
# escaped, so the anchored form is exactly "some element starts with the
# needle". Needles must stay free of ", \, and ERE metacharacters.
census_array_contains() { # snapshot, array field, substring
  local rendered
  rendered=$(census_field "$1" "$2")
  [[ "$rendered" == \[* ]] && grep -Fq -- "$3" <<<"$rendered"
}
census_array_has_prefix() { # snapshot, array field, element prefix
  local rendered
  rendered=$(census_field "$1" "$2")
  [[ "$rendered" == \[* ]] && grep -Eq -- "(\[|,)\"$3" <<<"$rendered"
}
pred_error_contains() { # snapshot, substring
  census_array_contains "$1" errors "$2"
}
pred_verdict_and_error_prefix() { # snapshot, verdict, error-prefix
  [[ "$(census_field "$1" verdict)" == "$2" ]] \
    && census_array_has_prefix "$1" errors "$3"
}
pred_raced_exit() { # snapshot: a RACED-EXIT diagnostic is present (freshness via scanSeq)
  census_array_has_prefix "$1" diagnostics RACED-EXIT
}
pred_path_absent() { # snapshot (ignored), path
  [[ ! -e "$2" ]]
}
# -----------------------------------------------------------------------------

stop_owned_pid() { # name, pid, start
  local name=$1 pid=$2 start=$3 started deadline elapsed
  process_identity_alive "$pid" "$start" || return 0
  kill -TERM "$pid" 2>/dev/null || true
  started=$SECONDS
  deadline=$((SECONDS + fixture_ceiling_sec))
  while process_identity_alive "$pid" "$start"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "supervision fixture stop timed out: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${fixture_ceiling_sec}s)" >&2
      kill -KILL "$pid" 2>/dev/null || true
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  wait "$pid" 2>/dev/null || true
}

wait_for_child_exit() { # name, child pid
  local name=$1 pid=$2 result
  if ! wait_until "$name" bash -c '! kill -0 "$1" 2>/dev/null' _ "$pid"; then
    kill -TERM "$pid" 2>/dev/null || true
    return 124
  fi
  # The child is already observed dead under the named ceiling above; this
  # wait only reaps its immediately available status.
  if wait "$pid"; then result=0; else result=$?; fi
  return "$result"
}

cleanup_started=0
cleanup() {
  (( cleanup_started )) && return 0
  cleanup_started=1
  local harness_path tuple pid start
  if [[ -n "${operator_harness:-}" && -x "$operator_harness/scripts/agents/arm-supervision.sh" ]]; then
    if declare -p operator_env >/dev/null 2>&1; then
      "${operator_env[@]}" "$operator_harness/scripts/agents/arm-supervision.sh" \
        --repo "${operator_scope:-$operator_harness}" --shutdown >/dev/null 2>&1 || true
    else
      "$operator_harness/scripts/agents/arm-supervision.sh" \
        --repo "$operator_harness" --shutdown >/dev/null 2>&1 || true
    fi
  fi
  for harness_path in ${fixture_harness_roots[@]+"${fixture_harness_roots[@]}"}; do
    if [[ -x "$harness_path/scripts/agents/arm-supervision.sh" ]]; then
      "$harness_path/scripts/agents/arm-supervision.sh" --repo "$harness_path" --shutdown >/dev/null 2>&1 || true
    fi
  done
  for tuple in ${owned_pids[@]+"${owned_pids[@]}"}; do
    IFS=: read -r pid start <<<"$tuple"
    stop_owned_pid cleanup "$pid" "$start" >/dev/null 2>&1 || true
  done
  # Backstop the owned-pid list: any process still rooted under this run's
  # unique temp dir is this fixture's own leak (a supervision component or
  # synthetic child that outran its stop), and leaving it running adds
  # process pressure that flakes later runs. $tmp is a private mktemp dir,
  # so nothing else can match its path.
  if [[ -n "$tmp" && "$tmp" == /*/tmp.* ]]; then
    local strays
    strays=$(pgrep -f "$tmp" 2>/dev/null || true)
    if [[ -n "$strays" ]]; then
      kill -TERM $strays 2>/dev/null || true
      sleep 1
      kill -KILL $(pgrep -f "$tmp" 2>/dev/null || true) 2>/dev/null || true
    fi
  fi
  if [[ -n "${METASYSTEM_KEEP_SUPERVISION_FIXTURE:-}" ]]; then
    echo "kept supervision fixture: $tmp" >&2
  else
    rm -rf "$tmp"
  fi
}
on_signal() {
  local signal=$1
  (( cleanup_started )) && return 0
  cleanup
  trap - EXIT
  exit $((128 + signal))
}
trap cleanup EXIT
trap 'on_signal 2' INT
trap 'on_signal 15' TERM

enroll_fixture_engine() { # repository state root, engine path
  local state_root=$1 engine=$2 digest identity_dir
  state_root=$(cd "$state_root" && pwd -P)
  engine=$(cd "$(dirname "$engine")" && pwd -P)/$(basename "$engine")
  digest=$($engine util sha256 --file "$engine")
  identity_dir=$state_root/artifacts/agents/steward
  mkdir -p "$identity_dir"
  printf '{"repoIdentity":"%s","generation":1,"installPath":"%s","installDigest":"sha256:%s","mintedAt":"1970-01-01T00:00:00Z"}\n' \
    "$state_root" "$engine" "$digest" >"$identity_dir/identity.json"
  chmod 0600 "$identity_dir/identity.json"
}

# An explicit identity fixture must invoke up below the process it announces.
# Keeping that process alive also gives later assertions a real live tuple.
live_arm_driver=$tmp/live-arm-driver.sh
cat >"$live_arm_driver" <<'SH'
#!/usr/bin/env bash
set -u
engine=$1 arm=$2 repo=$3 session=$4 tag=$5 output=$6 status_file=$7 ready=$8 release=$9
started=$("$engine" proc started-at --pid $$)
"$arm" --repo "$repo" --session "$session" --pid $$ --start-time "$started" --tag "$tag" >"$output" 2>&1
status=$?
printf '%s\n' "$status" >"$status_file"
touch "$ready"
(( status == 0 )) || exit "$status"
while [[ ! -e "$release" ]]; do
  sleep "${METASYSTEM_FIXTURE_POLL_INTERVAL_SEC:-0.05}"
done
SH

make_repo() { # destination
  local repo=$1 evidence=$tmp/evidence-$(basename "$repo")
  fixture_harness_roots+=("$repo")
  mkdir -p "$repo/scripts"
  cp -R "$source_root/scripts/agents" "$repo/scripts/"
  cp "$source_root/scripts/watch-background-jobs.sh" "$repo/scripts/"
  cp "$source_root/scripts/metasystem-config.sh" "$repo/scripts/"
  cp "$source_root/metasystem.conf" "$repo/metasystem.conf"
  # The engine owns fake-runtime conf tailoring (script-fixtures-020/D49);
  # only harness-specific overrides ride --set. The default fake model
  # keeps a role-free adopted source complete when tailoring adds the
  # default fake runtime. The old rewrite's model.tier clauses never
  # matched: the shipped conf carries no tier lines, and this harness
  # never grew an append. Investigator stays main here, as before.
  "$ms" config tailor --conf "$repo/metasystem.conf" --runtimes fake \
    --set evidence.root="$evidence" \
    --set role.default.model.fake=fake-model \
    --set watch.interval-sec=1 \
    --set census.log-max-bytes=350
  git -C "$repo" init -q -b main
  git -C "$repo" add .
  git -C "$repo" -c user.name=metasystem -c user.email=metasystem.invalid commit -qm fixture
  # Stage the engine the way production ships it: an untracked build artifact
  # at <repo>/bin/metasystem, added after the base commit.
  mkdir -p "$repo/bin"
  cp "$ms" "$repo/bin/metasystem"
  # Fixture setup supplies the standing human enrollment. Product arming is
  # then exercised only through its ambient, non-minting path.
  enroll_fixture_engine "$repo" "$repo/bin/metasystem"
}

json_field() { # file, dotted field (script-fixtures-022: the engine verb)
  "$ms" json get --file "$1" --field "$2"
}

json_array_items() { # file, top-level array field: one element per line
  local raw boundary=$'}\n{'
  raw=$("$ms" json get --file "$1" --field "$2" 2>/dev/null) || return 1
  [[ "$raw" == \[* ]] || return 1
  raw=${raw#\[}
  raw=${raw%\]}
  [[ -n "$raw" ]] || return 0
  # Census inventory items are flat objects and no fixture string carries
  # "},{", so this boundary split is exact here.
  printf '%s\n' "${raw//"},{"/$boundary}"
}

inventory_has() { # last-census, class, pid
  local item
  while IFS= read -r item; do
    [[ "$("$ms" json get --value "$item" --field pid 2>/dev/null)" == "$3" ]] || continue
    [[ "$("$ms" json get --value "$item" --field class 2>/dev/null)" == "$2" ]] || continue
    return 0
  done < <(json_array_items "$1" inventory)
  return 1
}

inventory_has_pid() { # last-census, pid (any class)
  local item
  while IFS= read -r item; do
    [[ "$("$ms" json get --value "$item" --field pid 2>/dev/null)" == "$2" ]] && return 0
  done < <(json_array_items "$1" inventory)
  return 1
}

# Atomically replace one composite-valued top-level field of a JSON object
# file, leaving every other field exactly as the file's parser sees it.
# `json set` covers string and integer fields; this covers the object and
# array fields it cannot spell. The whole file and the field are rendered
# by the same engine encoder, so the needle below is byte-exact; a failed
# splice or an unparseable result refuses instead of writing.
json_replace_field() { # file, top-level field, replacement JSON value
  local file=$1 field=$2 new=$3 compact old out staged
  compact=$("$ms" json get --value "{\"root\":$(cat "$file")}" --field root) \
    || { echo "json_replace_field: $file did not parse" >&2; return 1; }
  old=$("$ms" json get --file "$file" --field "$field") \
    || { echo "json_replace_field: $file has no $field" >&2; return 1; }
  out=${compact/"\"$field\":$old"/"\"$field\":$new"}
  if [[ "$out" == "$compact" && "$new" != "$old" ]]; then
    echo "json_replace_field: could not locate $field in $file" >&2
    return 1
  fi
  "$ms" util json-validate --value "$out" \
    || { echo "json_replace_field: editing $field left $file unparseable" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.replace.XXXXXX") || return 1
  printf '%s\n' "$out" >"$staged"
  mv "$staged" "$file"
}

edit_state_owner() { # state file, json set edits applied to the owner object
  local state=$1 owner_staged
  shift
  owner_staged=$(mktemp "$(dirname "$state")/.owner-edit.XXXXXX") || return 1
  "$ms" json get --file "$state" --field owner >"$owner_staged" \
    || { rm -f "$owner_staged"; return 1; }
  "$ms" json set --file "$owner_staged" "$@" \
    || { rm -f "$owner_staged"; return 1; }
  json_replace_field "$state" owner "$(cat "$owner_staged")" \
    || { rm -f "$owner_staged"; return 1; }
  rm -f "$owner_staged"
}

set_custody_process() { # job record, pidStartedAt, instanceTag
  local record=$1 started=$2 tag=$3 item pid
  item=$(json_field "$record" custodyProcesses) || return 1
  item=${item#\[}
  item=${item%\]}
  pid=$("$ms" json get --value "$item" --field pid) || return 1
  json_replace_field "$record" custodyProcesses \
    "[{\"pid\":$pid,\"pidStartedAt\":$started,\"instanceTag\":\"$tag\"}]"
}

write_process_fixture() { # output followed by pid|ppid|pgid|started|argv|cwd rows
  # Rows are fixture-controlled: argv and cwd never carry JSON-escapable
  # characters, so the rendering below is plain printf. Rename into place:
  # the census reads this table mid-pass and a torn read is a known flake
  # class.
  local output=$1 staged row pid ppid pgid started argv cwd sep='['
  shift
  staged=$(mktemp "$(dirname "$output")/.process-fixture.XXXXXX")
  for row in "$@"; do
    IFS='|' read -r pid ppid pgid started argv cwd <<<"$row"
    if [[ "$cwd" == UNRESOLVED ]]; then
      printf '%s{"pid":%s,"ppid":%s,"pgid":%s,"pidStartedAt":%s,"argv":"%s","cwd":null,"cwdError":true,"alive":true}' \
        "$sep" "$pid" "$ppid" "$pgid" "$started" "$argv" >>"$staged"
    else
      printf '%s{"pid":%s,"ppid":%s,"pgid":%s,"pidStartedAt":%s,"argv":"%s","cwd":"%s","cwdError":false,"alive":true}' \
        "$sep" "$pid" "$ppid" "$pgid" "$started" "$argv" "$cwd" >>"$staged"
    fi
    sep=','
  done
  [[ "$sep" == , ]] || printf '[' >>"$staged"
  printf ']\n' >>"$staged"
  mv "$staged" "$output"
}

# The ordinary operator layout keeps the vendored metasystem one directory below
# the Git toplevel. Its primary branch uses the shipped configuration and real
# process sources: no fake census table, fake identity table, or rewritten
# metasystem.conf. Restricted hosts keep the same path regression through the
# existing fake-process fallback below.
operator_scope=$tmp/operator-repo
operator_harness=$operator_scope/metasystem
mkdir -p "$operator_scope"
# Copy the shipped tree only. artifacts/ holds this session's live runtime
# state, including the supervision lock naming the operator's own owner pid;
# carrying it into the sandbox makes the fixture's shutdown stop a supervisor
# it does not own, which silently disarms the machine running the suite.
mkdir -p "$operator_harness"
(cd "$source_root" && for entry in * .[!.]*; do
  [[ -e "$entry" ]] || continue
  [[ "$entry" == artifacts || "$entry" == .git || "$entry" == metasystem.conf.local ]] && continue
  cp -R "$entry" "$operator_harness/"
done)
operator_scope=$(cd "$operator_scope" && pwd -P)
operator_harness=$(cd "$operator_harness" && pwd -P)
git -C "$operator_scope" init -q -b main
git -C "$operator_scope" add metasystem
git -C "$operator_scope" -c user.name=metasystem -c user.email=metasystem.invalid commit -qm fixture
fixture_harness_roots+=("$operator_harness")
operator_arm=$operator_harness/scripts/agents/arm-supervision.sh
operator_engine=$operator_harness/bin/metasystem
enroll_fixture_engine "$operator_scope" "$operator_engine"

# A plain shell with no matching ancestor must refuse, but the refusal tells an
# operator both supported ways forward. Restricting discovery to the fake
# signature makes this deterministic without replacing any process source.
set +e
(
  cd "$operator_harness"
  METASYSTEM_AGENT_RUNTIME=fake "$operator_arm" --repo "$PWD"
) >"$tmp/operator-identity.out" 2>&1 &
operator_identity_driver=$!
wait_for_child_exit "nested operator identity refusal" "$operator_identity_driver"
operator_identity_rc=$?
set -e
[[ $operator_identity_rc -ne 0 ]] \
  || { echo "nested operator identity inference unexpectedly succeeded" >&2; exit 1; }
grep -Fq 'component=session-identity outcome=failed' "$tmp/operator-identity.out" \
  && grep -Fq 'runtime-signature ancestry proof failed' "$tmp/operator-identity.out" \
  && grep -Fq 'pass --pid <session-pid> and --start-time <epoch-seconds>' "$tmp/operator-identity.out" \
  || { echo "nested operator identity refusal is not actionable" >&2; cat "$tmp/operator-identity.out" >&2; exit 1; }

operator_env=(env -u METASYSTEM_CENSUS_PROCESS_FILE -u METASYSTEM_FAKE_PROCESS_IDENTITY_FILE
  -u METASYSTEM_FAKE_AGENT_ANCESTOR_PID -u METASYSTEM_AGENT_RUNTIME
  -u METASYSTEM_SESSION_ID -u METASYSTEM_INSTANCE_TAG -u METASYSTEM_WATCH_INTERVAL_SEC
  -u METASYSTEM_CENSUS_LOG_MAX_BYTES -u METASYSTEM_CENSUS_MAX_INTERVAL_SHARE_PERCENT)
if [[ "${METASYSTEM_SUPERVISION_OPERATOR_EMPTY_RUNTIME_FIXTURE_ONLY:-0}" == 1 ]]; then
  conf_edit "$operator_harness/metasystem.conf" replace-line-first \
    '^metasystem[.]runtimes=.*$' 'metasystem.runtimes='
  printf 'metasystem.runtimes=codex\n' >"$operator_harness/metasystem.conf.local"
fi
# Census reads the committed configuration directly, so process-source
# selection reads the same file without applying local or environment layers.
operator_runtimes=$(awk '
  {
    separator = index($0, "=")
    if (!separator) next
    name = substr($0, 1, separator - 1)
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
    if (name != "metasystem.runtimes") next
    matches++
    value = substr($0, separator + 1)
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
  }
  END {
    if (matches != 1) exit 1
    printf "%s", value
  }
' "$operator_harness/metasystem.conf")
if [[ -z "$operator_runtimes" ]] \
    || [[ -n "${METASYSTEM_SUPERVISION_OPERATOR_FIXTURE_FAKE:-}" ]] \
    || ! ps -p "$$" -o lstart= >/dev/null 2>&1 \
    || ! ps -axo pid=,ppid=,pgid=,lstart=,command= >/dev/null 2>&1; then
  # Restricted execution sandboxes may prohibit ps entirely. Keep the nested
  # path/registry regression active through the restricted-CI source. A copied
  # configuration without runtime signatures requires the same deterministic
  # source; configured hosts with an ordinary process table use the real source.
  if [[ -z "$operator_runtimes" ]]; then
    echo "nested ordinary operator fixture: copied configuration lists no runtimes; using restricted-CI identity source" >&2
  else
    echo "nested ordinary operator fixture: real process source unavailable; using restricted-CI identity source" >&2
  fi
  operator_process_fixture=$tmp/operator-processes.json
  operator_identity_fixture=$tmp/operator-identities.json
  printf '[]\n' >"$operator_process_fixture"
  printf '{}\n' >"$operator_identity_fixture"
  conf_edit "$operator_harness/metasystem.conf" replace-line-first '^metasystem[.]runtimes=.*$' 'metasystem.runtimes=fake'
  printf 'role.default.model.fake=fake-model\n' >>"$operator_harness/metasystem.conf"
  conf_edit "$operator_harness/metasystem.conf" replace-line-first '^watch[.]interval-sec=.*$' 'watch.interval-sec=1'
  operator_env=(env METASYSTEM_CENSUS_PROCESS_FILE="$operator_process_fixture"
    METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$operator_identity_fixture")
fi
if [[ "${METASYSTEM_SUPERVISION_OPERATOR_EMPTY_RUNTIME_FIXTURE_ONLY:-0}" == 1 ]]; then
  [[ -z "$operator_runtimes" ]]
  [[ "$(env -u METASYSTEM_METASYSTEM_RUNTIMES \
    "$operator_harness/scripts/metasystem-config.sh" get --key metasystem.runtimes --default '')" == codex ]]
  grep -Fqx 'metasystem.runtimes=fake' "$operator_harness/metasystem.conf"
  [[ "${operator_env[*]}" == *"METASYSTEM_CENSUS_PROCESS_FILE=$operator_process_fixture"* ]]
  echo "nested ordinary operator empty-runtime source fixture passed"
  exit 0
fi
operator_start=$("${operator_env[@]}" "$operator_engine" proc started-at --pid "$$")
(
  cd "$operator_harness"
  "${operator_env[@]}" \
    "$operator_arm" --repo "$PWD" --session operator-path --pid "$$" \
      --start-time "$operator_start" --tag operator-path
) >"$tmp/operator-arm.out" 2>&1 &
operator_driver=$!
operator_driver_start=$("${operator_env[@]}" "$operator_engine" proc started-at --pid "$operator_driver")
owned_pids+=("$operator_driver:$operator_driver_start")
if [[ "${METASYSTEM_SUITE_TEST_PAUSE_AT:-}" == post-owned-pids ]]; then
  echo "supervision fixture test pause: post-owned-pids" >&2
  sleep 5
fi
wait_for_child_exit "nested ordinary operator arming" "$operator_driver" \
  || { operator_driver_rc=$?; cat "$tmp/operator-arm.out" >&2; exit "$operator_driver_rc"; }
grep -Fq 'up outcome=armed authority=writer' "$tmp/operator-arm.out" \
  || { cat "$tmp/operator-arm.out" >&2; exit 1; }
[[ -s "$operator_scope/artifacts/agents/supervision/last-census.json" ]] \
  || { echo "nested operator path did not write state at the Git repository scope" >&2; exit 1; }
[[ ! -e "$operator_harness/artifacts" ]] \
  || { echo "nested operator path split state beneath the vendored installation" >&2; exit 1; }
if grep -Eq 'metasystem\.runtimes has no signature adapters|No such file or directory' "$tmp/operator-arm.out"; then
  cat "$tmp/operator-arm.out" >&2
  exit 1
fi
(
  cd "$operator_harness"
  "$operator_arm" fingerprint --repo "$PWD"
) >/dev/null
printf '{"session_id":"operator-path","cwd":"%s","hook_event_name":"Stop"}\n' "$operator_harness" \
  | (cd "$operator_harness" && "${operator_env[@]}" scripts/agents/supervision-hook.sh fake stop) \
  >"$tmp/operator-hook.out"
if grep -Fq '$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh' \
    "$operator_harness"/scripts/enforcement/*hooks.json; then
  echo "nested operator hook configuration still resolves metasystem scripts from the Git toplevel" >&2
  exit 1
fi
"${operator_env[@]}" "$operator_arm" --repo "$operator_scope" --shutdown \
  >"$tmp/operator-shutdown.out" 2>&1 \
  || { echo "nested operator shutdown failed" >&2; cat "$tmp/operator-shutdown.out" >&2; exit 1; }

echo "nested ordinary operator supervision fixture passed" >&2

repo=$tmp/repo
mkdir -p "$repo"
make_repo "$repo"
arm="$repo/scripts/agents/arm-supervision.sh"
# One writer per checkout. Phases that arm a DIFFERENT main than the phase
# before them release the checkout first, the way a departing main does.
# Supervision components never hold the lease, so dropping the lease record
# is a complete release and leaves the running supervision set untouched.
release_checkout() { # repo
  rm -f "$1/artifacts/agents/mains/worktree-lease.json" \
        "$1/artifacts/agents/mains/reaped-after-claim.json"
}

# Control-plane writes belong to the checkout's announced holder, so a fixture
# that drives one must BE a main rather than borrow whatever class its ambient
# ancestry happens to produce. Ambient class is not stable: the same phase
# classifies as HUMAN from a terminal, as DELEGATE under an agent that runs the
# suite, and as DELEGATE again whenever a census fixture puts a simulated agent
# command on a real ancestor's process identifier. Announcing this shell claims
# the checkout as a side effect, which is what a starting main does.
become_main() { # repository state root, session, optional engine
  local repo=$1 session=$2 engine=${3:-$1/bin/metasystem}
  "$engine" lease announce --root "$repo" \
    --session "$session" --pid $$ --start "$(process_started_at $$)" \
    --tag "fixture-$session" --runtime fake >/dev/null
}
watcher="$repo/scripts/watch-background-jobs.sh"
census_engine="$repo/bin/metasystem"
process_fixture=$repo/process-fixture.json
identity_fixture=$repo/process-identities.json
printf '[]\n' >"$process_fixture"
printf '{}\n' >"$identity_fixture"

# Census cost must be proportional to agent-shaped processes, not to the host
# process count. The retired python leg counted the module's cwd-resolver
# calls under a stubbed ps; the engine enforces the same rule structurally
# (internal/census/production.go resolves cwds only for MATCHED processes).
# What stays observable from outside is the classification boundary: feed
# 1,000 unrelated rows and two agent rows through the fixture enumeration
# and the inventory must contain exactly the two agent processes.
{
  printf '['
  for ((index = 0; index < 1000; index++)); do
    printf '{"pid":%s,"ppid":1,"pgid":%s,"pidStartedAt":100,"argv":"/usr/bin/non-agent-%s --flag value","cwd":"%s","cwdError":false,"alive":true},' \
      "$((50000 + index))" "$((50000 + index))" "$index" "$repo"
  done
  printf '{"pid":61001,"ppid":1,"pgid":61001,"pidStartedAt":100,"argv":"metasystem-fake-agent first","cwd":"%s","cwdError":false,"alive":true},' "$repo"
  printf '{"pid":61002,"ppid":1,"pgid":61002,"pidStartedAt":100,"argv":"/tool/metasystem-fake-agent second","cwd":"%s","cwdError":false,"alive":true}]' "$repo"
} >"$tmp/enumerate-filter-resolve-procs.json"
METASYSTEM_CENSUS_PROCESS_FILE="$tmp/enumerate-filter-resolve-procs.json" \
  "$census_engine" proc census --root "$repo" --repo "$repo" \
  --fingerprint ordering-fixture --interval 60 \
  --output "$tmp/enumerate-filter-resolve.json" >/dev/null
efr_pids=""
while IFS= read -r efr_item; do
  efr_pid=$("$ms" json get --value "$efr_item" --field pid) \
    || { echo "inventory item carries no pid: $efr_item" >&2; exit 1; }
  efr_pids="$efr_pids$efr_pid "
done < <(json_array_items "$tmp/enumerate-filter-resolve.json" inventory)
[[ "$efr_pids" == "61001 61002 " ]] \
  || { echo "inventory is not exactly the two agent processes: ${efr_pids:-empty}" >&2
       cat "$tmp/enumerate-filter-resolve.json" >&2; exit 1; }
efr_duration=$(json_field "$tmp/enumerate-filter-resolve.json" durationMs) \
  && [[ "$efr_duration" =~ ^[0-9]+$ ]] \
  || { echo "durationMs is not a non-negative integer: ${efr_duration:-absent}" >&2; exit 1; }
echo "enumerate-filter-resolve census fixture passed" >&2

export METASYSTEM_CENSUS_PROCESS_FILE="$process_fixture"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$identity_fixture"
export METASYSTEM_WATCH_INTERVAL_SEC=1
export METASYSTEM_CENSUS_LOG_MAX_BYTES=350

# S4-7: all four adapters own a strict, line-oriented POSIX-ERE signature
# grammar. Exclude wins ties, malformed declarations fail the whole census,
# and lookalikes from each runtime stay out.
# The vectors are PROVIDER-OWNED declarations (agnosticism B1, ric
# critique r3-5): the registry serves them and this loop iterates the
# declared adapter population with no shared runtime branch — a future
# runtime joins by declaration, not by editing this fixture.
s47_population=$("$census_engine" runtime list --with-adapter) \
  || { echo "S4-7: the adapter population query refused" >&2; exit 1; }
s47_count=0
while IFS= read -r runtime; do
  adapter="$source_root/scripts/agents/adapters/$runtime.sh"
  signature=$($adapter signature)
  [[ -n "$signature" ]] || { echo "S4-7: $runtime returned no signatures" >&2; exit 1; }
  while IFS= read -r line; do
    [[ "$line" == match\ * || "$line" == exclude\ * ]] \
      || { echo "S4-7: malformed $runtime signature line: $line" >&2; exit 1; }
  done <<<"$signature"
  vectors=$("$census_engine" runtime signature-vectors "$runtime")
  positive=$("$census_engine" json get --value "$vectors" --field positive)
  lookalike=$("$census_engine" json get --value "$vectors" --field lookalike)
  [[ -n "$positive" && -n "$lookalike" ]] \
    || { echo "S4-7: $runtime declared no signature vectors" >&2; exit 1; }
  "$census_engine" proc signature-check --adapter "$adapter" --positive "$positive" \
    --lookalike "$lookalike" >/dev/null
  s47_count=$((s47_count + 1))
done <<<"$s47_population"
(( s47_count >= 4 )) \
  || { echo "S4-7: only $s47_count adapter runtimes exercised — the declared population went missing" >&2; exit 1; }
for failure in malformed invalid-ere adapter-failed exclude-tie; do
  bad_adapter="$tmp/signature-$failure.sh"
  case "$failure" in
    malformed) printf '#!/usr/bin/env bash\nprintf "bogus .*\\n"\n' >"$bad_adapter" ;;
    invalid-ere) printf '#!/usr/bin/env bash\nprintf "match [\\n"\n' >"$bad_adapter" ;;
    adapter-failed) printf '#!/usr/bin/env bash\nexit 9\n' >"$bad_adapter" ;;
    exclude-tie) printf '#!/usr/bin/env bash\nprintf "match ^tie$\\nexclude ^tie$\\n"\n' >"$bad_adapter" ;;
  esac
  chmod +x "$bad_adapter"
  if "$census_engine" proc signature-check --adapter "$bad_adapter" --positive tie \
      --lookalike lookalike >/dev/null 2>&1; then
    echo "S4-7: $failure signature adapter did not fail closed" >&2
    exit 1
  fi
done

# S4-8: omitted identity arguments have an executable rule. Run arming below a
# fake agent-signature ancestor and require inferred session/process identity.
cat >"$repo/metasystem-fake-agent" <<'SH'
#!/usr/bin/env bash
METASYSTEM_FAKE_AGENT_ANCESTOR_PID=$$ "$1" --repo "$2"
while [[ ! -e "$3" ]]; do sleep "${METASYSTEM_FIXTURE_POLL_INTERVAL_SEC:?}"; done
SH
chmod +x "$repo/metasystem-fake-agent"
infer_release=$tmp/inferred-agent.release
# One writer per checkout: this phase arms a different main than the phase
# before it, so it releases the checkout first the way a departing main does.
# Supervision components never hold the lease, so removing the lease record
# once supervision is stopped is a complete release.
release_checkout "$repo"
METASYSTEM_SESSION_ID=inferred-session METASYSTEM_AGENT_RUNTIME=fake \
  "$repo/metasystem-fake-agent" "$arm" "$repo" "$infer_release" >"$tmp/inferred-arm.out" 2>&1 &
infer_driver=$!
infer_driver_start=$(process_started_at "$infer_driver")
owned_pids+=("$infer_driver:$infer_driver_start")
wait_until "S4-8 inferred announcement" bash -c 'compgen -G "$1/artifacts/agents/mains/inferred-session-*.json" >/dev/null' _ "$repo"
touch "$infer_release"
wait_for_child_exit "S4-8 inferred arming" "$infer_driver"
grep -Fq 'up outcome=armed authority=writer' "$tmp/inferred-arm.out" \
  || { cat "$tmp/inferred-arm.out" >&2; exit 1; }

last="$repo/artifacts/agents/supervision/last-census.json"
state="$repo/artifacts/agents/supervision/state.json"
wait_until "first complete census" test -s "$last"
[[ "$(json_field "$last" verdict)" == SUCCESS ]] || { echo "first census did not succeed" >&2; exit 1; }

# S4-5 and S4-10: the authoritative verdict is single-writer, schema-versioned,
# identifies its owner and instances, and the cross-component fields exist.
# One snapshot of each artifact, so every assertion binds to the same bytes
# the way the retired one-shot reader did; both files publish by atomic
# rename, so a cp is a consistent copy.
cp "$last" "$tmp/s45-last.json"
cp "$state" "$tmp/s45-state.json"
[[ "$(json_field "$tmp/s45-last.json" schemaVersion)" == 2 ]] \
  || { echo "S4-5: schemaVersion is not 2" >&2; cat "$tmp/s45-last.json" >&2; exit 1; }
[[ "$(json_field "$tmp/s45-last.json" writer)" == watch-background-jobs.sh ]] \
  || { echo "S4-5: the verdict does not name its writer" >&2; exit 1; }
case "$(json_field "$tmp/s45-last.json" verdict)" in
  SUCCESS | CENSUS-FAILED) ;;
  *) echo "S4-5: verdict is neither SUCCESS nor CENSUS-FAILED" >&2; exit 1 ;;
esac
[[ "$(json_field "$tmp/s45-last.json" completedAtEpoch)" =~ ^-?[0-9]+$ ]] \
  || { echo "S4-5: completedAtEpoch is not an integer" >&2; exit 1; }
[[ "$(json_field "$tmp/s45-last.json" intervalSec)" =~ ^-?[0-9]+$ ]] \
  || { echo "S4-5: intervalSec is not an integer" >&2; exit 1; }
[[ "$(json_field "$tmp/s45-last.json" durationMs)" =~ ^[0-9]+$ ]] \
  || { echo "S4-5: durationMs is not a non-negative integer" >&2; exit 1; }
[[ -n "$(json_field "$tmp/s45-last.json" fingerprint)" ]] \
  || { echo "S4-5: fingerprint is absent or empty" >&2; exit 1; }
s45_generation=$(json_field "$tmp/s45-last.json" generation) \
  || { echo "S4-5: the verdict carries no generation" >&2; exit 1; }
[[ "$s45_generation" == "$(json_field "$tmp/s45-state.json" generation)" ]] \
  || { echo "S4-5: census generation does not match the state" >&2; exit 1; }
[[ "$(json_field "$tmp/s45-last.json" stateDigest)" == "$("$ms" util sha256 --file "$tmp/s45-state.json")" ]] \
  || { echo "S4-5: stateDigest does not attest the state bytes" >&2; exit 1; }
json_field "$tmp/s45-last.json" counts >"$tmp/s45-counts.json" \
  || { echo "S4-5: the verdict carries no counts" >&2; exit 1; }
[[ "$("$ms" json strip --file "$tmp/s45-counts.json" --key CUSTODY --key ANNOUNCED --key UNTRACKED)" == '{}' ]] \
  || { echo "S4-5: counts carries keys beyond the three classes" >&2; cat "$tmp/s45-counts.json" >&2; exit 1; }
for s45_class in CUSTODY ANNOUNCED UNTRACKED; do
  json_field "$tmp/s45-counts.json" "$s45_class" >/dev/null \
    || { echo "S4-5: counts is missing $s45_class" >&2; exit 1; }
done
json_field "$tmp/s45-state.json" components >"$tmp/s45-components.json" \
  || { echo "S4-5: the state carries no components" >&2; exit 1; }
[[ "$("$ms" json strip --file "$tmp/s45-components.json" --key watcher --key reaper)" == '{}' ]] \
  || { echo "S4-5: state components beyond watcher and reaper" >&2; cat "$tmp/s45-components.json" >&2; exit 1; }
for s45_component in watcher reaper; do
  for s45_key in pid pidStartedAt instanceTag heartbeat; do
    json_field "$tmp/s45-state.json" "components.$s45_component.$s45_key" >/dev/null \
      || { echo "S4-5: component $s45_component is missing $s45_key" >&2; exit 1; }
  done
done
if "$watcher" --dir "$repo/artifacts/agents/jobs" --scope "$repo" \
    --state "$tmp/second-writer.state" --interval 1 --once --census \
    --supervision-dir "$repo/artifacts/agents/supervision" \
    --heartbeat "$tmp/second-writer.heartbeat" --instance-tag second-writer \
    >"$tmp/second-writer.out" 2>&1; then
  echo "S4-5: a second live census writer was accepted" >&2
  exit 1
fi
grep -Fq 'live census writer already owns' "$tmp/second-writer.out" \
  || { echo "S4-5: duplicate writer refusal did not name the live owner" >&2
       echo "--- second writer said:" >&2; cat "$tmp/second-writer.out" >&2
       echo "--- lock state:" >&2; ls -la "$repo/artifacts/agents/supervision/census-writer.d" >&2 2>/dev/null || true
       cat "$repo/artifacts/agents/supervision/census-writer.d/owner.json" >&2 2>/dev/null || echo "(no owner.json)" >&2
       exit 1; }

# Duration is part of the authoritative artifact, and the watcher names an
# over-interval scan as a supervision defect instead of silently looping it.
warning_repo=$tmp/warning-repo
make_repo "$warning_repo"
conf_edit "$warning_repo/scripts/agents/adapters/fake.sh" insert-after-first \
  '^  signature[)]$' '    sleep 1.1'
warning_supervision=$warning_repo/artifacts/agents/supervision
warning_process_fixture=$warning_repo/processes.json
warning_identity_fixture=$warning_repo/identities.json
mkdir -p "$warning_supervision" "$warning_repo/artifacts/agents/jobs"
printf '[]\n' >"$warning_process_fixture"
printf '{}\n' >"$warning_identity_fixture"
touch "$warning_supervision/jobs.state"
cat >"$warning_supervision/state.json" <<'JSON'
{"owner":{"pid":71001,"pidStartedAt":1,"instanceTag":"warning-owner"},
 "components":{
  "watcher":{"pid":71002,"pidStartedAt":1,"instanceTag":"warning-watcher"},
  "reaper":{"pid":71003,"pidStartedAt":1,"instanceTag":"warning-reaper"}}}
JSON
METASYSTEM_CENSUS_PROCESS_FILE="$warning_process_fixture" \
METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$warning_identity_fixture" \
METASYSTEM_CENSUS_MAX_INTERVAL_SHARE_PERCENT=50 \
  "$warning_repo/scripts/watch-background-jobs.sh" \
    --dir "$warning_repo/artifacts/agents/jobs" --scope "$warning_repo" \
    --state "$warning_supervision/jobs.state" --interval 1 --once --census \
    --supervision-dir "$warning_supervision" \
    --heartbeat "$warning_supervision/watcher.heartbeat.json" \
    --instance-tag warning-fixture >"$tmp/slow-census.out" 2>&1
grep -Fq 'WARNING CENSUS-SLOW' "$tmp/slow-census.out" \
  && grep -Fq 'defect=scan-exceeds-interval' "$tmp/slow-census.out" \
  || { echo "slow census was not surfaced as a supervision defect" >&2; cat "$tmp/slow-census.out" >&2; exit 1; }
warning_duration=$(json_field "$warning_supervision/last-census.json" durationMs)
warning_interval=$(json_field "$warning_supervision/last-census.json" intervalSec)
(( warning_duration > warning_interval * 1000 )) \
  || { echo "slow census did not record an over-interval duration" >&2
       cat "$warning_supervision/last-census.json" >&2; exit 1; }

grep -Fq 'pidStartedAt' "$repo/scripts/agents/dispatch.sh" \
  || { echo "S4-1: job record does not carry pidStartedAt" >&2; exit 1; }
grep -Fq 'pidStartedAt' "$source_root/docs/orchestration.md" \
  || { echo "S4-1/S4-10: host-turn contract does not document pidStartedAt" >&2; exit 1; }
for owner_asset in \
  scripts/agents/dispatch.sh scripts/watch-background-jobs.sh \
  scripts/agents/adapters/runtime-common.sh scripts/agents/supervision-hook.sh \
  scripts/enforcement/claude-code-hooks.json scripts/enforcement/codex-hooks.json \
  scripts/enforcement/devin-hooks.json; do
  [[ -f "$source_root/$owner_asset" ]] \
    || { echo "S4-10: cross-component owner asset is missing: $owner_asset" >&2; exit 1; }
done

# Double arming joins the same repository set and duplicate starts collapse.
owner_before=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
owner_start=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pidStartedAt)
release_checkout "$repo"
"$arm" --repo "$repo" --session duplicate --pid "$$" \
  --start-time "$(process_started_at "$$")" --tag fixture-main >/dev/null
"$arm" --repo "$repo" --session duplicate --pid "$$" \
  --start-time "$(process_started_at "$$")" --tag fixture-main >/dev/null
owner_after=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
[[ "$owner_before" == "$owner_after" ]] || { echo "double arming replaced a live set" >&2; exit 1; }
[[ $(find "$repo/artifacts/agents/mains" -name 'duplicate-*.json' | wc -l | tr -d ' ') -eq 1 ]] \
  || { echo "duplicate session starts did not collapse" >&2; exit 1; }

# A second live session announces and verifies the shared rings but never
# displaces the checkout holder. Its typed advisor outcome names both the
# holder and the isolated-worktree remedy.
advisor_output_file=$tmp/advisor-arm.out
advisor_status_file=$tmp/advisor-arm.status
advisor_ready=$tmp/advisor-arm.ready
advisor_release=$tmp/advisor-arm.release
bash "$live_arm_driver" "$repo/bin/metasystem" "$arm" "$repo" advisor-session advisor-session \
  "$advisor_output_file" "$advisor_status_file" "$advisor_ready" "$advisor_release" &
advisor_pid=$!
advisor_start=$(process_started_at "$advisor_pid")
owned_pids+=("$advisor_pid:$advisor_start")
wait_until "advisor descendant arming" test -e "$advisor_ready"
[[ $(cat "$advisor_status_file") == 0 ]] \
  || { echo "advisor descendant failed to arm" >&2; cat "$advisor_output_file" >&2; exit 1; }
holder_before=$(json_field "$repo/artifacts/agents/mains/worktree-lease.json" holderMainId)
advisor_output=$(cat "$advisor_output_file")
holder_after=$(json_field "$repo/artifacts/agents/mains/worktree-lease.json" holderMainId)
[[ "$holder_before" == "$holder_after" ]] \
  || { echo "advisor up displaced the live checkout holder" >&2; exit 1; }
grep -Fq 'component=checkout-lease outcome=advisor' <<<"$advisor_output" \
  && grep -Fq 'up outcome=advisor authority=read-only' <<<"$advisor_output" \
  && grep -Fq 'scripts/agents/second-session.sh' <<<"$advisor_output" \
  || { echo "second-session up did not return the typed advisor outcome" >&2; echo "$advisor_output" >&2; exit 1; }
touch "$advisor_release"
wait_for_child_exit "advisor descendant release" "$advisor_pid"

# Complete three-class inventory, self-announcement before first census, stale
# custody rejection (S4-2), worktree scope inclusion, peer exclusion, embedded
# and not-yet-created argv paths (S4-9), and per-process unresolved cwd (S4-6).
mkdir -p "$repo/artifacts/agents/worktrees/w" "$tmp/peer"
now=$(date +%s)
raw_pid=41001 announced_pid=41002 custody_pid=41003 worktree_pid=41004 peer_pid=41005 argv_pid=41006 unresolved_pid=41007 relative_pid=41008
raw_start=$((now - 10)); announced_start=$((now - 9)); custody_start=$((now - 8)); worktree_start=$((now - 7)); peer_start=$((now - 6)); argv_start=$((now - 5)); unresolved_start=$((now - 4)); relative_start=$((now - 3))
write_process_fixture "$process_fixture" \
  "$raw_pid|1|$raw_pid|$raw_start|metasystem-fake-agent raw|$repo" \
  "$announced_pid|1|$announced_pid|$announced_start|metasystem-fake-agent announced|$repo" \
  "$custody_pid|1|$custody_pid|$custody_start|metasystem-fake-agent child --tag metasystem-job-owned|$repo" \
  "$worktree_pid|1|$worktree_pid|$worktree_start|metasystem-fake-agent worktree|$repo/artifacts/agents/worktrees/w" \
  "$peer_pid|1|$peer_pid|$peer_start|metasystem-fake-agent peer|$tmp/peer" \
  "$argv_pid|1|$argv_pid|$argv_start|metasystem-fake-agent --workspace=$repo/not-created/yet|$tmp/peer" \
  "$unresolved_pid|1|$unresolved_pid|$unresolved_start|metasystem-fake-agent --repo=$repo|UNRESOLVED" \
  "$relative_pid|1|$relative_pid|$relative_start|metasystem-fake-agent --workspace=../repo/not-created/relative|$tmp/peer"
mkdir -p "$repo/artifacts/agents/mains" "$repo/artifacts/agents/jobs"
# Rename announcement and job records into place: the census and reaper
# read these directories mid-pass and a torn read is a known flake class.
announced_staged=$(mktemp "$repo/artifacts/agents/mains/.announced.XXXXXX")
printf '{"sessionId":"announced","pid":%s,"pidStartedAt":%s,"pgid":%s,"runtime":"fake","instanceTag":"main-announced","announcedAt":"%s"}\n' \
  "$announced_pid" "$announced_start" "$announced_pid" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$announced_staged"
mv "$announced_staged" "$repo/artifacts/agents/mains/announced-$announced_pid.json"
supervisor_start=$(process_started_at "$$")
supervisor_announcement="$repo/artifacts/agents/mains/duplicate-$$.json"
supervisor_pgid=$(json_field "$supervisor_announcement" pgid)
[[ "$supervisor_pgid" =~ ^[0-9]+$ ]] \
  || { echo "cannot read this shell's announced process group" >&2; exit 1; }
mkdir -p "$repo/artifacts/agents/hb"
owned_tag=metasystem-job-owned
owned_cap=$(harness_fixture_semantic_cap dormant-job-minutes)
owned_staged=$(mktemp "$repo/artifacts/agents/jobs/.owned.XXXXXX")
printf '{"jobId":"owned","status":"running","runtime":"fake","workspaceRoot":"%s","pid":%s,"pidStartedAt":%s,"pgid":%s,"instanceTag":"%s","ownershipProof":{"pid":%s,"pidStartedAt":%s,"pgid":%s,"instanceTag":"%s"},"startedAt":"2099-01-01T00:00:00Z","capMin":%s,"custodyProcesses":[{"pid":%s,"pidStartedAt":%s,"instanceTag":"%s"}]}\n' \
  "$repo" "$$" "$supervisor_start" "$supervisor_pgid" "$owned_tag" \
  "$$" "$supervisor_start" "$supervisor_pgid" "$owned_tag" \
  "$owned_cap" "$custody_pid" "$((custody_start - 1))" "$owned_tag" >"$owned_staged"
mv "$owned_staged" "$repo/artifacts/agents/jobs/owned.json"
printf '{"pid":%s,"instanceTag":"%s"}\n' "$$" "$owned_tag" >"$repo/artifacts/agents/hb/owned"
# The reaper proves the record's custodian (this shell) by pid+start+tag.
# This shell's real command line does not carry the job tag, so the fixture
# identity source supplies it — the same one-source override the census
# uses; kernel death still vetoes it. The table still holds its initial {}
# here, so the merged result is this one entry; rename into place because
# the census reads the table mid-pass.
identity_staged=$(mktemp "$(dirname "$identity_fixture")/.identities.XXXXXX")
printf '{"%s":{"pidStartedAt":%s,"command":"fixture-supervisor metasystem-job-owned"}}\n' \
  "$$" "$supervisor_start" >"$identity_staged"
mv "$identity_staged" "$identity_fixture"
wait_for_census "three-class census" inventory_has ANNOUNCED "$announced_pid"
inventory_has "$last" UNTRACKED "$raw_pid"
inventory_has "$last" UNTRACKED "$custody_pid"
inventory_has "$last" UNTRACKED "$worktree_pid"
inventory_has "$last" UNTRACKED "$argv_pid"
inventory_has "$last" UNTRACKED "$unresolved_pid"
inventory_has "$last" UNTRACKED "$relative_pid"
if inventory_has_pid "$last" "$peer_pid"
then echo "peer repository process entered scope" >&2; exit 1; fi
# The engine surfaces the per-process unresolved cwd in the verdict's
# diagnostics (the shell watcher's census.log transcript retired with it).
census_array_contains "$last" diagnostics "UNRESOLVED-CWD pid=$unresolved_pid" \
  || { echo "S4-6: unresolved cwd was not surfaced per process" >&2; exit 1; }

# Correcting the child start alone must not gain custody while the third join
# key is wrong; only the exact pid+start+tag triple may classify the real CLI
# child as CUSTODY (S4-2).
set_custody_process "$repo/artifacts/agents/jobs/owned.json" "$custody_start" wrong-tag
wait_for_census "S4-2 wrong-tag census pass" pred_any
inventory_has "$last" UNTRACKED "$custody_pid" \
  || { echo "S4-2: wrong instanceTag gained custody" >&2; exit 1; }
set_custody_process "$repo/artifacts/agents/jobs/owned.json" "$custody_start" \
  "$(json_field "$repo/artifacts/agents/jobs/owned.json" instanceTag)"
wait_for_census "S4-2 child custody exact join" inventory_has CUSTODY "$custody_pid"
identity_staged=$(mktemp "$(dirname "$identity_fixture")/.identities.XXXXXX")
printf '{}\n' >"$identity_staged"
mv "$identity_staged" "$identity_fixture"

# CENSUS-FAILED covers total enumeration failure and unresolved scope, while a
# process-table race that has already exited is a named exclusion (S4-6).
printf '{broken\n' >"$process_fixture"
wait_for_census "S4-6 enumeration failure" pred_verdict_is CENSUS-FAILED
# The engine surfaces the enumeration failure in the verdict's errors
# (the shell watcher's census.log transcript retired with it).
wait_for_census "S4-6 enumeration surfaced" pred_error_contains enumeration
printf '[]\n' >"$process_fixture"
wait_for_census "census recovery" pred_verdict_is SUCCESS

race_pid=41009
write_process_fixture "$process_fixture" "$race_pid|1|$race_pid|$now|metasystem-fake-agent raced|$repo"
# Flip the one row to already-exited: the same row write_process_fixture
# just rendered, with alive false, renamed into place.
race_staged=$(mktemp "$(dirname "$process_fixture")/.process-fixture.XXXXXX")
printf '[{"pid":%s,"ppid":1,"pgid":%s,"pidStartedAt":%s,"argv":"metasystem-fake-agent raced","cwd":"%s","cwdError":false,"alive":false}]\n' \
  "$race_pid" "$race_pid" "$now" "$repo" >"$race_staged"
mv "$race_staged" "$process_fixture"
wait_for_census "S4-6 raced-exit census" pred_raced_exit
[[ "$(json_field "$last" verdict)" == SUCCESS ]] \
  || { echo "S4-6: exited process race failed the census" >&2; exit 1; }

bad_argv_pid=41010
write_process_fixture "$process_fixture" "$bad_argv_pid|1|$bad_argv_pid|$now|metasystem-fake-agent --workspace='|$repo"
wait_for_census "S4-6 unreadable argv" pred_verdict_and_error_prefix CENSUS-FAILED argv-unreadable:

bad_start_pid=41011
write_process_fixture "$process_fixture" "$bad_start_pid|1|$bad_start_pid|-1|metasystem-fake-agent bad-start|$repo"
wait_for_census "S4-6 unreadable start time" pred_verdict_and_error_prefix CENSUS-FAILED start-time-unreadable:
printf '[]\n' >"$process_fixture"
wait_for_census "S4-6 partial-failure recovery" pred_verdict_is SUCCESS

# Fingerprint includes scripts, signatures, and relevant config. Mutating a
# static identity owner invalidates the old verdict (S4-3).
fingerprint_before=$(json_field "$last" fingerprint)
printf '\n# signature fingerprint fixture\n' >>"$repo/scripts/agents/adapters/fake.sh"
fingerprint_after=$($arm fingerprint --repo "$repo")
[[ "$fingerprint_before" != "$fingerprint_after" ]] \
  || { echo "S4-3: adapter change did not alter expected fingerprint" >&2; exit 1; }
old_generation_owner=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
replacement_output=$("$arm" --repo "$repo" --session generation-replacement --pid "$$" \
  --start-time "$(process_started_at "$$")" --tag fixture-main)
new_generation_owner=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
[[ "$new_generation_owner" != "$old_generation_owner" ]] \
  && grep -Fq 'component=supervision-owner outcome=replaced' <<<"$replacement_output" \
  || { echo "ordinary up did not replace the live older engine generation" >&2; echo "$replacement_output" >&2; exit 1; }
git -C "$repo" show HEAD:scripts/agents/adapters/fake.sh >"$repo/scripts/agents/adapters/fake.sh.restored"
mv "$repo/scripts/agents/adapters/fake.sh.restored" "$repo/scripts/agents/adapters/fake.sh"
chmod +x "$repo/scripts/agents/adapters/fake.sh"
# Restoring the accepted bytes is another generation change; ordinary up
# absorbs that replacement too, leaving later fixture legs on current code.
"$arm" --repo "$repo" --session generation-restored --pid "$$" \
  --start-time "$(process_started_at "$$")" --tag fixture-main >/dev/null
owner_before=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
owner_start=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pidStartedAt)

# Dispatch refuses absent, stale, failed, and fingerprint-mismatched verdicts.
gate_repo=$tmp/gate-repo
# The refusal message names the resolved path, and on macOS the temp
# directory is reached through a symlink, so the expectation must resolve too.
gate_repo_real=$gate_repo
mkdir -p "$gate_repo"
make_repo "$gate_repo"
brief=$gate_repo/brief.md
sed 's/^Working Mode:.*/Working Mode: design/' "$gate_repo/scripts/agents/templates/brief.md" >"$brief"
"$gate_repo/scripts/agents/adapters/fake.sh" probe >/dev/null
become_main "$gate_repo" gate-session
dispatch_fails() { # name, expected
  local name=$1 expected=$2
  set +e
  "$gate_repo/scripts/agents/dispatch.sh" dispatch --role design-critic --brief "$brief" --job-id "$name" >"$tmp/$name.out" 2>&1
  rc=$?
  set -e
  [[ $rc -ne 0 ]] || { echo "dispatch accepted $name census" >&2; exit 1; }
  grep -Fq "$expected" "$tmp/$name.out" || { cat "$tmp/$name.out" >&2; exit 1; }
}
dispatch_succeeds() { # name
  local name=$1
  "$gate_repo/scripts/agents/dispatch.sh" dispatch --role design-critic --brief "$brief" --job-id "$name" \
    >"$tmp/$name.out" 2>&1 \
    || { echo "dispatch refused $name census" >&2; cat "$tmp/$name.out" >&2; exit 1; }
}
set_gate_census() { # age, interval, fingerprint
  "$ms" json set --file "$gate_repo/artifacts/agents/supervision/last-census.json" \
    --int completedAtEpoch=$(( $(date +%s) - $1 )) \
    --int intervalSec="$2" \
    --field verdict=SUCCESS \
    --field fingerprint="$3" \
    --int generation="$(json_field "$gate_repo/artifacts/agents/supervision/state.json" generation)"
}
assert_stale_shape() { # name, window
  local name=$1 window=$2 output="$tmp/$1.out"
  grep -Eq 'age=[0-9]+s' "$output" \
    && grep -Fq "window=${window}s" "$output" \
    && grep -Fq 'retry in a moment' "$output" \
    && grep -Fq "re-arm with $gate_repo_real/scripts/agents/arm-supervision.sh --repo $gate_repo_real if supervision is dead" "$output" \
    || { echo "stale census refusal did not carry the common diagnostic shape" >&2; cat "$output" >&2; exit 1; }
}
dispatch_fails no-census 'census verdict is absent'
mkdir -p "$gate_repo/artifacts/agents/supervision"
cp "$state" "$gate_repo/artifacts/agents/supervision/state.json"
cp "$last" "$gate_repo/artifacts/agents/supervision/last-census.json"
gate_fingerprint=$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")
cp "$gate_repo/artifacts/agents/supervision/state.json" "$tmp/gate-state.json"
conf_edit "$gate_repo/metasystem.conf" replace-line-first '^watch[.]stale-min=.*$' 'watch.stale-min=21'
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" != "$gate_fingerprint" ]] \
  || { echo "S4-3: relevant configuration did not alter the fingerprint" >&2; exit 1; }
git -C "$gate_repo" show HEAD:metasystem.conf >"$gate_repo/metasystem.conf"
edit_state_owner "$gate_repo/artifacts/agents/supervision/state.json" \
  --field instanceTag="$(json_field "$gate_repo/artifacts/agents/supervision/state.json" owner.instanceTag)-changed"
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" == "$gate_fingerprint" ]] \
  || { echo "S4-3: supervisor instance identity altered the static fingerprint" >&2; exit 1; }
cp "$tmp/gate-state.json" "$gate_repo/artifacts/agents/supervision/state.json"
printf '\n# watcher fingerprint fixture\n' >>"$gate_repo/scripts/watch-background-jobs.sh"
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" != "$gate_fingerprint" ]] \
  || { echo "S4-3: watcher code did not alter the fingerprint" >&2; exit 1; }
git -C "$gate_repo" show HEAD:scripts/watch-background-jobs.sh >"$gate_repo/scripts/watch-background-jobs.sh"
chmod +x "$gate_repo/scripts/watch-background-jobs.sh"

# Freshness is one capped window. Inside proceeds; the exact boundary and one
# second past refuse with age, window, and both remedies. A configured interval
# above the cap still refuses at 180 seconds.
set_gate_census 0 10 "$gate_fingerprint"
dispatch_succeeds inside-census-window
set_gate_census 20 10 "$gate_fingerprint"
dispatch_fails census-window-boundary 'census verdict is stale'
gate_repo_real=$(cd "$gate_repo" && pwd -P)
assert_stale_shape census-window-boundary 20
set_gate_census 21 10 "$gate_fingerprint"
dispatch_fails stale-census 'census verdict is stale'
assert_stale_shape stale-census 20
conf_edit "$gate_repo/metasystem.conf" replace-line-first '^watch[.]interval-sec=.*$' 'watch.interval-sec=200'
capped_fingerprint=$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")
set_gate_census 180 200 "$capped_fingerprint"
dispatch_fails capped-census-window 'census verdict is stale'
assert_stale_shape capped-census-window 180
git -C "$gate_repo" show HEAD:metasystem.conf >"$gate_repo/metasystem.conf"

# A census from the prior arming generation uses that same refusal shape and
# additionally names both generations.
set_gate_census 0 10 "$gate_fingerprint"
"$ms" json set --file "$gate_repo/artifacts/agents/supervision/state.json" \
  --int generation=$(( $(json_field "$gate_repo/artifacts/agents/supervision/state.json" generation) + 1 ))
dispatch_fails stale-census-generation 'census verdict is stale'
assert_stale_shape stale-census-generation 20
grep -Fq 'censusGeneration=' "$tmp/stale-census-generation.out" \
  && grep -Fq 'armedGeneration=' "$tmp/stale-census-generation.out" \
  || { echo "generation-stale refusal did not name both generations" >&2; cat "$tmp/stale-census-generation.out" >&2; exit 1; }
cp "$tmp/gate-state.json" "$gate_repo/artifacts/agents/supervision/state.json"
set_gate_census 0 10 "$gate_fingerprint"
"$ms" json set --file "$gate_repo/artifacts/agents/supervision/last-census.json" \
  --int completedAtEpoch="$(date +%s)" --field verdict=CENSUS-FAILED
dispatch_fails failed-census 'CENSUS-FAILED'
"$ms" json set --file "$gate_repo/artifacts/agents/supervision/last-census.json" \
  --int completedAtEpoch="$(date +%s)" --field verdict=SUCCESS --field fingerprint=wrong
dispatch_fails fingerprint-census 'fingerprint does not match'
edit_state_owner "$gate_repo/artifacts/agents/supervision/state.json" \
  --int pid=999999 --int pidStartedAt=1
# The hook reports on the main it belongs to, so it must run as one: the fake
# runtime's ancestor variable names this shell, which become_main announced and
# which therefore holds this checkout. Without it the hook correctly reports
# that it is a read-only advisor and never reaches the drift it is asked about.
printf '{"session_id":"stale-surface","cwd":"%s","hook_event_name":"Stop"}\n' "$gate_repo" \
  | METASYSTEM_FAKE_AGENT_ANCESTOR_PID=$$ \
    "$gate_repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/stale-surface.out"
if grep -Fq 'code changed since arming' "$tmp/stale-surface.out"; then
  grep -Fq 'owner' "$tmp/stale-surface.out" && grep -Fq 'not running' "$tmp/stale-surface.out" \
    || { echo "S4-4: end-turn hook hid a dead supervision owner" >&2; exit 1; }
else
  # Stop now runs up before rendering. A live owner may repair the deliberately
  # stale publication during that transaction; prove reconciliation instead of
  # requiring the obsolete pre-repair warning.
  [[ "$(json_field "$gate_repo/artifacts/agents/supervision/state.json" fingerprint)" \
      == "$(json_field "$gate_repo/artifacts/agents/supervision/last-census.json" fingerprint)" ]] \
    && [[ "$(json_field "$gate_repo/artifacts/agents/supervision/state.json" owner.pid)" != 999999 ]] \
    || { echo "S4-3/S4-4: end-turn hook neither surfaced nor repaired fingerprint drift" >&2; cat "$tmp/stale-surface.out" >&2; exit 1; }
fi

# Continuous supervision has one owner and one cadence. Killing either
# component is detected and the whole instance-fingerprinted set is replaced
# (S4-4); a proven-dead lock owner is taken over.
component_replaced() { # component, old pid
  [[ "$(json_field "$state" "components.$1.pid")" != "$2" ]]
}
watcher_pid=$(json_field "$state" components.watcher.pid)
watcher_start=$(json_field "$state" components.watcher.pidStartedAt)
reaper_before_watcher_kill=$(json_field "$state" components.reaper.pid)
stop_owned_pid "S4-4 watcher" "$watcher_pid" "$watcher_start"
wait_until "S4-4 watcher recovery" component_replaced watcher "$watcher_pid"
wait_until "S4-4 watcher recovery replaces set" component_replaced reaper "$reaper_before_watcher_kill"
# The engine owner narrates the failing observation and the relaunch to
# owner.ndjson (the shell owner's supervisor.log retired with it).
grep -q '"observation":"failing"' "$repo/artifacts/agents/supervision/owner.ndjson" \
  && grep -q '"relaunch"' "$repo/artifacts/agents/supervision/owner.ndjson" \
  || { echo "watcher death was not surfaced" >&2; exit 1; }
reaper_pid=$(json_field "$state" components.reaper.pid)
reaper_start=$(json_field "$state" components.reaper.pidStartedAt)
watcher_before_reaper_kill=$(json_field "$state" components.watcher.pid)
stop_owned_pid "S4-4 reaper" "$reaper_pid" "$reaper_start"
wait_until "S4-4 reaper recovery" component_replaced reaper "$reaper_pid"
wait_until "S4-4 reaper recovery replaces set" component_replaced watcher "$watcher_before_reaper_kill"
# Surfaced the same way: a failing observation followed by a relaunch in the
# owner's narration (three generations now exist, so at least two relaunches).
[[ $(grep -c '"relaunch"' "$repo/artifacts/agents/supervision/owner.ndjson") -ge 2 ]] \
  || { echo "reaper death was not surfaced" >&2; exit 1; }

stop_owned_pid "supervision owner for takeover" "$owner_before" "$owner_start"
wait_until "proven-dead owner" bash -c '! "$1" proc alive --pid "$2" --start-time "$3" --root "$4" >/dev/null 2>&1' _ "$census_engine" "$owner_before" "$owner_start" "$repo"
printf '[]\n' >"$process_fixture"
release_checkout "$repo"
if [[ -n "${METASYSTEM_SUITE_DEBUG_TAKEOVER_CLASS:-}" ]]; then
  takeover_class=$("$census_engine" lease classify --root "$repo" --caller-pid $$)
  echo "takeover-point classification: $takeover_class" >&2
  [[ "$("$ms" json get --value "$takeover_class" --field class)" == MAIN ]] \
    || { echo "takeover-point caller is not class MAIN" >&2; exit 1; }
fi
takeover_output=$("$arm" --repo "$repo" --session takeover --pid "$$" \
  --start-time "$(process_started_at "$$")" --tag takeover-main)
new_owner=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
[[ "$new_owner" != "$owner_before" ]] || { echo "stale supervision lock was not taken over" >&2; exit 1; }
grep -Fq 'component=supervision-owner outcome=taken-over' <<<"$takeover_output" \
  || { echo "dead-owner takeover was not surfaced as a typed component outcome" >&2; echo "$takeover_output" >&2; exit 1; }

# Dead announcements are pruned; SessionEnd retires its own; log rotation and
# end-of-turn UNTRACKED/stale-supervisor surfacing remain visible.
# A real process in its own session that stays alive until stop_owned_pid
# terminates it below; its tag matches no runtime signature.
dead_output_file=$tmp/dead-main-arm.out
dead_status_file=$tmp/dead-main-arm.status
dead_ready=$tmp/dead-main-arm.ready
dead_release=$tmp/dead-main-arm.release
release_checkout "$repo"
bash "$live_arm_driver" "$repo/bin/metasystem" "$arm" "$repo" dead-main dead-main \
  "$dead_output_file" "$dead_status_file" "$dead_ready" "$dead_release" &
dead_pid=$!
dead_start=$(process_started_at "$dead_pid")
owned_pids+=("$dead_pid:$dead_start")
wait_until "dead-main descendant arming" test -e "$dead_ready"
[[ $(cat "$dead_status_file") == 0 ]] \
  || { echo "dead-main descendant failed to arm" >&2; cat "$dead_output_file" >&2; exit 1; }
stop_owned_pid "dead announcement" "$dead_pid" "$dead_start"
wait_for_census "dead announcement pruning" pred_path_absent "$repo/artifacts/agents/mains/dead-main-$dead_pid.json"

write_process_fixture "$process_fixture" "$raw_pid|1|$raw_pid|$raw_start|metasystem-fake-agent raw|$repo"
wait_for_census "UNTRACKED end-turn source" inventory_has UNTRACKED "$raw_pid"
# The main that armed this checkout last was the dead one this phase buried, so
# this shell takes the checkout back before ending a turn in it: an end-of-turn
# report belongs to the main that holds the checkout.
become_main "$repo" surface
printf '{"session_id":"surface","cwd":"%s","hook_event_name":"Stop"}\n' "$repo" \
  | METASYSTEM_FAKE_AGENT_ANCESTOR_PID=$$ \
    "$repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/surface.out"
grep -q 'UNTRACKED' "$tmp/surface.out" || { echo "end-of-turn hook hid UNTRACKED" >&2; exit 1; }

# S4-15: a stop event inside a repository is NEVER silent. Silence reads
# identically to "still running", to "finished", and to "the hook never
# fired", which is the ambiguity this check exists to remove. Whatever the
# state — untracked agents, stale supervision, running jobs, or nothing at
# all — the hook must say something, and when nothing is left it must say so
# in plain words.
idle_repo=$tmp/idle-hook
mkdir -p "$idle_repo/artifacts/agents/jobs" "$idle_repo/artifacts/agents/supervision" "$idle_repo/plans" "$idle_repo/scripts" "$idle_repo/bin"
cp -R "$source_root/scripts/agents" "$idle_repo/scripts/"
cp "$source_root/metasystem.conf" "$idle_repo/"
cp "$ms" "$idle_repo/bin/metasystem"
# The hook refuses outside a git repository, correctly: it reports on a
# repository's work. The sandbox must be one.
git -C "$idle_repo" init -q -b main
printf '{"session_id":"idle","cwd":"%s","hook_event_name":"Stop"}\n' "$idle_repo" \
  | "$idle_repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/idle.out" 2>/dev/null || true
[[ -s "$tmp/idle.out" ]] \
  || { echo "turn-end hook was silent inside a repository" >&2; exit 1; }
grep -q 'systemMessage' "$tmp/idle.out" \
  || { echo "turn-end hook emitted no surfaced message" >&2; cat "$tmp/idle.out" >&2; exit 1; }

# And the idle wording itself: the two plain-words states now live in the
# verdict verb (goal-system GOAL-05 — the hook transports, the verdict
# decides), so the pin points at the decision's one owner.
grep -Fq 'NOTHING LEFT TO WORK ON' "$source_root/internal/goal/turnverdict.go" \
  && grep -Fq 'STILL WORKING' "$source_root/internal/goal/turnverdict.go" \
  || { echo "the turn verdict lost one of its two plain-words states" >&2; exit 1; }

# The open-work, stale-plan, and gate-marker legs retired to Go
# (script-fixtures-008/D45): the five basic cases were already
# internal/report's openwork tests; the four shell-only cases —
# chain-root round matching, per-stream staleness isolation, the
# plans/README exclusion, and the reporter-to-gate-marker integration
# with live-marker silencing and dead-marker pruning — were PORTED
# green first (TestNoStaleWhenClaimNamesTheChainRootOfALiveRound,
# TestStalenessIsPerStream, TestPlansReadmeIsNotAStream,
# TestOpenWorkGateMarkerIntegration). The supervision-hook legs below
# (S4-14, S4-15) keep exercising the shell hook itself, which has no
# Go home.

# The armed engine watcher publishes verdicts, not a census.log; the
# transcript log and its byte-capped rotation belong to the shell job
# watcher's --census mode, so rotation is proven there, in its own sandbox
# (the armed repository's census-writer lock is held by the live watcher).
rotation_repo=$tmp/rotation-repo
make_repo "$rotation_repo"
rotation_supervision=$rotation_repo/artifacts/agents/supervision
mkdir -p "$rotation_supervision" "$rotation_repo/artifacts/agents/jobs"
printf '[]\n' >"$rotation_repo/processes.json"
touch "$rotation_supervision/jobs.state"
# A healthy engine pass prints nothing, so the transcript only grows on
# warnings; a 1ms interval budget makes every pass a CENSUS-SLOW warning,
# which is exactly the content the transcript exists to keep.
rotation_passes=0
until [[ -f "$rotation_supervision/census.log.1" ]]; do
  (( rotation_passes < 40 )) \
    || { echo "census log rotation never happened after $rotation_passes passes" >&2; exit 1; }
  METASYSTEM_CENSUS_PROCESS_FILE="$rotation_repo/processes.json" \
  METASYSTEM_CENSUS_INTERVAL_MS=1 \
    "$rotation_repo/scripts/watch-background-jobs.sh" \
      --dir "$rotation_repo/artifacts/agents/jobs" --scope "$rotation_repo" \
      --state "$rotation_supervision/jobs.state" --interval 1 --once --census \
      --supervision-dir "$rotation_supervision" \
      --heartbeat "$rotation_supervision/watcher.heartbeat.json" \
      --instance-tag rotation-fixture >/dev/null 2>&1 || true
  rotation_passes=$((rotation_passes + 1))
done

# The arming event log proves write-announcement precedes the first census and
# therefore never labels the arming session UNTRACKED.
announce_line=$(grep -n -m1 -F 'announcement-written' "$repo/artifacts/agents/supervision/arming.log" | cut -d: -f1 || true)
census_line=$(grep -n -m1 -F 'first-census-complete' "$repo/artifacts/agents/supervision/arming.log" | cut -d: -f1 || true)
[[ -n "$announce_line" && -n "$census_line" && "$announce_line" -lt "$census_line" ]] \
  || { echo "arming log does not prove the announcement preceded the first census" >&2
       cat "$repo/artifacts/agents/supervision/arming.log" >&2; exit 1; }

# S4-11: a sandbox must never stop a supervisor another checkout armed. The
# operator sandbox above is built without artifacts/ for exactly this reason;
# both halves are asserted, because either one alone leaves the suite able to
# disarm the machine it runs on.
[[ ! -e "$operator_harness/artifacts/agents/supervision/lock.d/owner.json" ]] \
  || { echo "operator sandbox carries the operator's supervision lock" >&2; exit 1; }

foreign=$tmp/foreign-owner
mkdir -p "$foreign/repo"
(cd "$foreign/repo" && git init -q -b main .)
mkdir -p "$foreign/repo/metasystem/scripts/agents" "$foreign/repo/artifacts/agents/supervision/lock.d"
cp "$source_root/scripts/agents/arm-supervision.sh" \
  "$source_root/scripts/agents/preflight-commands.sh" \
  "$foreign/repo/metasystem/scripts/agents/"
# Shutting supervision down is a control-plane write, so the sandbox needs the
# engine (census identity, lease classification); without it this sandbox
# refuses for the wrong reason and the foreign-owner rule below is never
# actually exercised.
mkdir -p "$foreign/repo/metasystem/bin"
cp "$ms" "$foreign/repo/metasystem/bin/metasystem"
# The fixture-identity env rides this whole run; without a fake-mode
# conf the engine now refuses THAT first (agnosticism B1's
# leaked-fixture fence) and the foreign-owner rule below would never
# be exercised.
printf 'metasystem.runtimes=fake\nrole.default.model.fake=fake-model\n' > "$foreign/repo/metasystem/metasystem.conf"
# Third wrong-reason refusal, same lesson as the two above: the
# caller must BE this sandbox's announced main. Ambient ancestry
# answers agent under an agent-run suite, and require-holder then
# refuses UNTRUSTED before the foreign-owner rule is ever reached.
METASYSTEM_CENSUS_PROCESS_FILE= METASYSTEM_FAKE_PROCESS_IDENTITY_FILE= \
  become_main "$foreign/repo" foreign-owner-shutdown "$foreign/repo/metasystem/bin/metasystem"
foreign_sleep_pid=$(
  bash -c '"$1" util hold --tag metasystem-foreign-owner >/dev/null 2>&1 & echo $!' _ "$ms"
)
foreign_start=$(process_started_at "$foreign_sleep_pid")
owned_pids+=("$foreign_sleep_pid:$foreign_start")
printf '{"pid":%s,"pidStartedAt":%s,"instanceTag":"metasystem-supervision-owner-some-other-checkout-1-2","acquiredAt":"1970-01-01T00:00:00Z"}\n' \
  "$foreign_sleep_pid" "$foreign_start" \
  >"$foreign/repo/artifacts/agents/supervision/lock.d/owner.json"
set +e
"$foreign/repo/metasystem/scripts/agents/arm-supervision.sh" --repo "$foreign/repo" --shutdown \
  >"$tmp/foreign-shutdown.out" 2>&1
foreign_status=$?
set -e
(( foreign_status != 0 )) \
  || { echo "shutdown accepted a lock armed for another repository" >&2; exit 1; }
grep -Fq 'another repository' "$tmp/foreign-shutdown.out" \
  || { echo "foreign-owner refusal did not name the cause" >&2; cat "$tmp/foreign-shutdown.out" >&2; exit 1; }
kill -0 "$foreign_sleep_pid" 2>/dev/null \
  || { echo "shutdown stopped a process another checkout owned" >&2; exit 1; }
stop_owned_pid "foreign owner" "$foreign_sleep_pid" "$foreign_start" >/dev/null 2>&1 || true

# S4-14: the stop hook refuses a turn that ends with open work, once, and leaves
# evidence that it ran. A report was ignorable and was ignored four times in one
# session; an unbounded refusal is the loop the design forbids.
stop_root=$tmp/stop-hook
mkdir -p "$stop_root/plans" "$stop_root/artifacts/agents/jobs" "$stop_root/artifacts/agents/supervision" "$stop_root/scripts/agents"
cp "$source_root/scripts/agents/supervision-hook.sh" \
   "$source_root/scripts/agents/arm-supervision.sh" \
   "$source_root/scripts/agents/pre-commit-guard.sh" "$stop_root/scripts/agents/"
# The hook (and the announcement step below) derive the caller's
# main through the runtime-signature ancestor walk, and that walk
# reads the adapters from THIS root — without them find-ancestor
# refuses and every derived main is empty.
cp -R "$source_root/scripts/agents/adapters" "$stop_root/scripts/agents/adapters"
# The hook resolves its engine as <root>/bin/metasystem; the open-work,
# stop-block, identity, and lease helpers it used to need as .py files all
# live inside it now.
mkdir -p "$stop_root/bin"
cp "$ms" "$stop_root/bin/metasystem"
# The fixture-identity env rides this run: the sandbox needs the
# fake-mode conf or the engine's leaked-fixture fence (agnosticism B1)
# refuses classification before the hook logic under test runs.
printf 'metasystem.runtimes=fake\nrole.default.model.fake=fake-model\n' > "$stop_root/metasystem.conf"
cat >"$stop_root/plans/stream.md" <<'FIXTURE'
- In flight right now: nothing
- Waiting on the human: nothing blocking
- Next step: dispatch the runner
FIXTURE
git -C "$stop_root" init -q -b main
stop_payload=$(printf '{"session_id":"t","cwd":"%s","hook_event_name":"Stop"}' "$stop_root")
first=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$first" | grep -Fq 'Metasystem supervision arming failed:' \
  && printf '%s' "$first" | grep -Fq 'ENROLLMENT_DRIFT' \
  || { echo "the Stop payload did not carry the up failure" >&2; echo "$first" >&2; exit 1; }
printf '%s' "$first" | grep -q '"decision":"block"' \
  || { echo "the stop hook did not refuse a turn ending with open work" >&2; echo "$first" >&2; exit 1; }
printf '%s' "$first" | grep -Fq 'HEALTH ' \
  || { echo "the stop hook emitted no one-line health verdict" >&2; echo "$first" >&2; exit 1; }
second=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$second" | grep -q '"decision":"block"' \
  && { echo "the stop hook refused the same open work twice, which is the loop the design forbids" >&2; exit 1; }
[[ -s "$stop_root/artifacts/agents/supervision/hooks.log" ]] \
  || { echo "the stop hook left no evidence that it ran" >&2; exit 1; }
cat >"$stop_root/plans/stream.md" <<'FIXTURE'
- In flight right now: nothing
- Waiting on the human: nothing blocking
- Next step: none
FIXTURE
settled=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$settled" | grep -q '"decision":"block"' \
  && { echo "the stop hook refused a turn with no open work" >&2; exit 1; }

# S4-15: the goal thread through the same hook (goal-system GOAL-04/05).
# (a) Byte-identity: the block reason carries the verdict display
# verbatim — the first refusal above named the open step exactly.
printf '%s' "$first" | grep -Fq 'OPEN WORK (1)' \
  || { echo "the block reason does not carry the verdict display" >&2; echo "$first" >&2; exit 1; }
# (b) End to end, unseeded: with work settled, opening a goal makes the
# NEXT turn end block once pointing at the goal's next step — the
# incident's fix observed through the real hook.
# goal open and run launch below are holder-only control-plane
# writes, and the hook matches run ownership against the main IT
# derives — the nearest runtime-signature ancestor. Production
# truth: the announced main IS the runtime process. So when this
# suite runs under an agent, announce THAT ancestor as the
# sandbox's main (the writes authenticate through it and the run's
# recorded owner equals the hook's derived main). Without a runtime
# ancestor, the suite shell announces itself so detached runs carry
# the same holder-authenticated identity through the goal and run
# control-plane writes.
stop_main_identity=$("$stop_root/bin/metasystem" proc find-ancestor \
  --repo "$stop_root" --pid $$ --runtime claude 2>/dev/null || true)
if [[ -n "$stop_main_identity" ]]; then
  stop_main_pid=$("$ms" json get --value "$stop_main_identity" --field pid)
  "$stop_root/bin/metasystem" lease announce --root "$stop_root" \
    --session stop-hook-main --pid "$stop_main_pid" \
    --start "$(process_started_at "$stop_main_pid")" \
    --tag fixture-stop-hook --runtime claude >/dev/null
else
  "$stop_root/bin/metasystem" lease announce --root "$stop_root" \
    --session stop-hook-main --pid $$ \
    --start "$(process_started_at $$)" \
    --tag fixture-stop-hook --runtime claude >/dev/null
fi
"$stop_root/bin/metasystem" goal open --root "$stop_root" \
  --id fixture-goal --intent "Prove goal delivery" --next "Advance the fixture goal." >/dev/null
goal_block=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$goal_block" | grep -q '"decision":"block"' \
  && printf '%s' "$goal_block" | grep -Fq 'Advance the fixture goal.' \
  || { echo "a current goal did not reach the turn end through the hook" >&2; echo "$goal_block" >&2; exit 1; }
goal_again=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$goal_again" | grep -q '"decision":"block"' \
  && { echo "the same goal revision blocked twice" >&2; exit 1; }
printf '%s' "$goal_again" | grep -Fq 'NOTHING LEFT TO WORK ON' \
  || { echo "the spent goal revision did not read as the all-clear naming the goal" >&2; echo "$goal_again" >&2; exit 1; }
# (c) Session hygiene at the hook boundary: a path-shaped session id never
# reaches the state file.
evil_payload=$(printf '{"session_id":"../../evil","cwd":"%s","hook_event_name":"Stop"}' "$stop_root")
printf '%s' "$evil_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop >/dev/null
grep -q '\.\./' "$stop_root/artifacts/agents/turn-verdict-state.json" \
  && { echo "a path-shaped session id reached the verdict state" >&2; exit 1; }
# (d) The degraded path: a verb that cannot speak yields the hook's fixed
# message, never silence and never an all-clear.
chmod 0500 "$stop_root/artifacts/agents"
degraded=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
chmod 0755 "$stop_root/artifacts/agents"
printf '%s' "$degraded" | grep -Fq 'turn-verdict unavailable:' \
  || { echo "verb failure did not surface the fixed degraded message" >&2; echo "$degraded" >&2; exit 1; }
printf '%s' "$degraded" | grep -Fq 'NOTHING LEFT' \
  && { echo "the degraded path composed with an all-clear it cannot vouch for" >&2; exit 1; }

# S4-16: the monitor facility through the same hook (MON-04/05, D72).
# Launch a real wrapped run; the turn end refuses to walk away from it
# unwatched, once; a live watch clears the rule; conclusion surfaces the
# green continuation exactly once.
"$stop_root/bin/metasystem" run launch --root "$stop_root" --id fixture-run \
  --kind custom --display "the fixture run" --log fix-run.log \
  --expect-green "proceed to checkpoint seven" -- /bin/sleep 2 >/dev/null
mon1=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$mon1" | grep -q '"decision":"block"' \
  && printf '%s' "$mon1" | grep -Fq 'unwatched' \
  || { echo "an unwatched run did not block the turn end" >&2; echo "$mon1" >&2; exit 1; }
mon2=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$mon2" | grep -q '"decision":"block"' \
  && { echo "the same unwatched set blocked twice" >&2; exit 1; }
"$stop_root/bin/metasystem" run watch --id fixture-run --root "$stop_root" --poll-ms 200 &
mon_watch_pid=$!
sleep 1
mon3=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$mon3" | grep -Fq 'STILL WORKING' \
  && printf '%s' "$mon3" | grep -Fq 'run fixture-run' \
  || { echo "a live watched run did not read STILL WORKING" >&2; echo "$mon3" >&2; exit 1; }
sleep 2.5
"$stop_root/bin/metasystem" run conclude --root "$stop_root" --id fixture-run >/dev/null
mon_watch_rc=0
wait "$mon_watch_pid" || mon_watch_rc=$?
(( mon_watch_rc == 0 )) \
  || { echo "the run watch did not exit green (rc=$mon_watch_rc)" >&2; exit 1; }
mon4=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$mon4" | grep -Fq 'finished green' \
  && printf '%s' "$mon4" | grep -Fq 'proceed to checkpoint seven' \
  || { echo "the green continuation did not surface" >&2; echo "$mon4" >&2; exit 1; }
mon5=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$mon5" | grep -Fq 'finished green' \
  && { echo "the green surfaced twice" >&2; exit 1; }

echo "supervision fixtures passed (S4-1 through S4-16)"
