#!/usr/bin/env bash
set -euo pipefail

# The public half records one commit and creates an independent clone. It then
# executes the controller copied from that commit into the run directory, so a
# later checkout, rebase, or edit cannot replace code the running controller
# still needs.

# Inherited environments do not grant control over launch timing, identity
# publication paths, or launch locators. Those overrides remain inert unless
# their single capability is explicitly enabled for this invocation.
if [[ ${BATTERY_FIXTURE_SEAMS:-0} != 1 ]]; then
  unset BATTERY_FIXTURE_SEAMS BATTERY_LAUNCH_HOLD_FILE \
    BATTERY_VALIDATOR_PGID_FILE BATTERY_VALIDATOR_PGID_STAGE \
    BATTERY_VALIDATOR_LAUNCH_PID_FILE BATTERY_VALIDATOR_PUBLICATION_STALL_DIR
fi

usage() {
  cat <<'USAGE' >&2
Usage: scripts/agents/milestone-battery.sh [--subject <commit>]
       [--evidence-root <absolute-dir>] [--force-red]

Runs the milestone battery against an independent clone detached at one
recorded commit. --force-red is the evidence-lifecycle fixture: it publishes a
red envelope without running the expensive validator.
USAGE
}

die() { echo "milestone battery: $2" >&2; exit "$1"; }

has_symlink_component() { # absolute path
  local component=$1
  while [[ "$component" != / ]]; do
    [[ -L "$component" ]] && return 0
    component=${component%/*}; [[ -n "$component" ]] || component=/
  done
  return 1
}

path_within() { # child, parent
  [[ "$1" == "$2" || "$1" == "$2"/* ]]
}

if [[ ${1:-} != __isolated_controller ]]; then
  script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
  metasystem_root=$(cd "$script_dir/../.." && pwd -P)
  checkout=$(git -C "$metasystem_root" rev-parse --show-toplevel 2>/dev/null) \
    || die 1 "the entry point is not inside a Git checkout"
  checkout=$(cd "$checkout" && pwd -P)
  prefix=$(git -C "$metasystem_root" rev-parse --show-prefix)
  prefix=${prefix%/}
  subject=
  evidence_root=
  force_red=0
  while (($#)); do
    case "$1" in
      --subject) [[ $# -ge 2 ]] || { usage; exit 2; }; subject=$2; shift 2 ;;
      --evidence-root) [[ $# -ge 2 ]] || { usage; exit 2; }; evidence_root=$2; shift 2 ;;
      --force-red) force_red=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) usage; exit 2 ;;
    esac
  done
  subject=${subject:-$(git -C "$checkout" rev-parse HEAD)}
  subject=$(git -C "$checkout" rev-parse --verify "$subject^{commit}") \
    || die 2 "subject is not a commit"
  if [[ -z "$evidence_root" ]]; then
    evidence_root=$("$metasystem_root/scripts/metasystem-config.sh" get --key evidence.root --default "" 2>/dev/null || true)
  fi
  [[ "$evidence_root" == /* ]] || die 2 "--evidence-root (or evidence.root) must be absolute"
  [[ -d "$evidence_root" ]] || die 2 "the durable evidence root must already be a directory"
  evidence_root=$(cd "$evidence_root" && pwd -P) || die 2 "the durable evidence root is unreadable"
  [[ "$evidence_root" != / ]] || die 2 "the filesystem root cannot be the durable evidence root"
  path_within "$evidence_root" "$checkout" \
    && die 2 "the durable evidence root must be outside the live checkout"

  run_stamp=$(date -u +%Y%m%dT%H%M%SZ 2>/dev/null || printf 'undated')
  run_id="${run_stamp}-${subject:0:12}-$$"
  envelope_parent=$evidence_root/suite-failures
  envelope=$envelope_parent/$run_id
  bootstrap_stage=$envelope_parent/.${run_id}.bootstrap.$$
  bootstrap_started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || printf 'unavailable')
  run_dir=
  clone_log=
  cleanup_bootstrap=1
  bootstrap_json_escape() { # value
    local value=$1
    value=${value//\\/\\\\}
    value=${value//\"/\\\"}
    value=${value//$'\n'/\\n}
    value=${value//$'\r'/\\r}
    value=${value//$'\t'/\\t}
    printf '%s' "$value"
  }
  bootstrap_sha256() { # file
    if command -v shasum >/dev/null 2>&1; then
      shasum -a 256 "$1" | awk '{print $1}'
    else
      sha256sum "$1" | awk '{print $1}'
    fi
  }
  publish_bootstrap_stage() { # setup exit
    local bootstrap_exit=$1 ended_at relative digest actual expected
    mkdir -p "$envelope_parent" || return 1
    [[ ! -e "$envelope" && ! -e "$bootstrap_stage" ]] || return 1
    mkdir "$bootstrap_stage" || return 1
    if [[ -n "$clone_log" && -f "$clone_log" ]]; then
      cp "$clone_log" "$bootstrap_stage/clone.log" || return 1
    else
      : >"$bootstrap_stage/clone.log" || return 1
    fi
    : >"$bootstrap_stage/validation.log" || return 1
    printf '{}\n' >"$bootstrap_stage/checkpoint.json" || return 1
    printf '{"policyVersion":0,"projection":"LANDING","endpoint":"unavailable","surfaceDigest":"unavailable"}\n' \
      >"$bootstrap_stage/surface.json" || return 1
    printf 'unavailable\n' >"$bootstrap_stage/toolchain.txt" || return 1
    printf '%s\n' "$subject" >"$bootstrap_stage/subject.sha" || return 1
    printf '{"setup":%d,"validation":-1}\n' "$bootstrap_exit" >"$bootstrap_stage/exit-codes.json" || return 1
    ended_at=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || printf 'unavailable')
    printf '{"startedAt":"%s","endedAt":"%s"}\n' "$bootstrap_started_at" "$ended_at" \
      >"$bootstrap_stage/timings.json" || return 1
    : >"$bootstrap_stage/copy-digests.nul" || return 1
    while IFS= read -r -d '' relative; do
      relative=${relative#./}
      [[ "$relative" == copy-digests.nul || "$relative" == report.json ]] && continue
      digest=$(bootstrap_sha256 "$bootstrap_stage/$relative") || return 1
      printf '%s\0%s\0' "$digest" "$relative" >>"$bootstrap_stage/copy-digests.nul" || return 1
    done < <(cd "$bootstrap_stage" && find . -type f -print0)
    exec 7<"$bootstrap_stage/copy-digests.nul"
    while IFS= read -r -d '' expected <&7 && IFS= read -r -d '' relative <&7; do
      actual=$(bootstrap_sha256 "$bootstrap_stage/$relative") || { exec 7<&-; return 1; }
      [[ "$actual" == "$expected" ]] || { exec 7<&-; return 1; }
    done
    exec 7<&-
    printf '{"runId":"%s","subjectSHA":"%s","surfaceProjection":"LANDING","surfacePolicyVersion":0,"surfaceDigest":"unavailable","toolchainIdentity":"unavailable","startedAt":"%s","endedAt":"%s","setupExit":%d,"validationExit":-1,"copyResult":"verified","copyDigestManifest":"copy-digests.nul","verdict":"bootstrap-failed","cloneLog":"clone.log","validationLog":"validation.log","failureArtifacts":"failure-artifacts"}\n' \
      "$run_id" "$subject" "$bootstrap_started_at" "$ended_at" "$bootstrap_exit" \
      >"$bootstrap_stage/report.json" || return 1
    mv "$bootstrap_stage" "$envelope" || return 1
    return 0
  }
  publish_bootstrap_outcome() { # setup exit
    local bootstrap_exit=$1 tmp=$envelope/.outcome.$$.json
    printf '{"runId":"%s","subject":"%s","verdict":"bootstrap-failed","setupExit":%d,"validationExit":-1,"resetExit":-1}\n' \
      "$run_id" "$subject" "$bootstrap_exit" >"$tmp" || return 1
    mv "$tmp" "$envelope/outcome.json"
  }
  publish_bootstrap_teardown() { # result, retained path
    local result=$1 retained=$2 tmp=$envelope/.teardown.$$.json
    printf '{"runId":"%s","subject":"%s","result":"%s","retainedPath":"%s"}\n' \
      "$run_id" "$subject" "$result" "$(bootstrap_json_escape "$retained")" >"$tmp" || return 1
    mv "$tmp" "$envelope/teardown.json"
  }
  bootstrap_cleanup() {
    local status=$? retained_path= teardown_result=run-directory-not-created
    trap - EXIT INT TERM
    if (( cleanup_bootstrap )); then
      if publish_bootstrap_stage "$status" && publish_bootstrap_outcome "$status"; then
        if [[ -n "$run_dir" ]]; then
          teardown_result=removed
          rm -rf -- "$run_dir" 2>/dev/null || true
          if [[ -e "$run_dir" ]]; then
            retained_path=$run_dir
            teardown_result=run-directory-removal-failed
            [[ $status == 130 ]] || status=1
          fi
        fi
        if publish_bootstrap_teardown "$teardown_result" "$retained_path"; then
          if [[ -n "$retained_path" ]]; then
            echo "milestone battery bootstrap-failed; retained run directory: subject=$subject path=$retained_path envelope=$envelope" >&2
          elif [[ -z "$run_dir" ]]; then
            echo "milestone battery bootstrap-failed: subject=$subject run-directory=not-created envelope=$envelope" >&2
          else
            echo "milestone battery bootstrap-failed: subject=$subject clone=removed envelope=$envelope" >&2
          fi
        else
          [[ $status == 130 ]] || status=1
          echo "milestone battery bootstrap evidence incomplete: subject=$subject envelope=$envelope" >&2
        fi
      else
        [[ $status == 130 ]] || status=1
        if [[ -n "$run_dir" ]]; then
          echo "milestone battery bootstrap evidence incomplete; retained run directory: subject=$subject path=$run_dir envelope=$envelope" >&2
        else
          echo "milestone battery bootstrap evidence incomplete before run-directory creation: subject=$subject envelope=$envelope" >&2
        fi
      fi
    fi
    exit "$status"
  }
  trap bootstrap_cleanup EXIT INT TERM

  command -v git >/dev/null 2>&1 || die 1 "git is required"
  command -v go >/dev/null 2>&1 || die 1 "the Go toolchain is required"
  shared_gocache=$(go env GOCACHE) || die 1 "the shared GOCACHE is unreadable"
  [[ -n "$shared_gocache" ]] || die 1 "go env GOCACHE returned empty"

  temp_parent=$(cd "${TMPDIR:-/tmp}" && pwd -P) || die 1 "the temporary parent is unreadable"
  run_dir=$(mktemp -d "$temp_parent/metasystem-battery.XXXXXXXX") \
    || die 1 "could not create a fresh run directory"
  canonical_run_dir=$(cd "$run_dir" && pwd -P) || die 1 "the fresh run directory is unreadable"
  run_dir=$canonical_run_dir
  clone_log=$run_dir/clone.log
  : >"$clone_log"
  has_symlink_component "$run_dir" && die 1 "run directory has a symlink component: $run_dir"
  if enclosing_checkout=$(git -C "$run_dir" rev-parse --show-toplevel 2>/dev/null); then
    die 1 "run directory is inside checkout $enclosing_checkout"
  fi
  while IFS= read -r worktree; do
    [[ -n "$worktree" ]] || continue
    worktree=$(cd "$worktree" && pwd -P)
    path_within "$run_dir" "$worktree" \
      && die 1 "run directory is inside checkout $worktree"
  done < <(git -C "$checkout" worktree list --porcelain | sed -n 's/^worktree //p')

  clone=$run_dir/subject
  git clone --no-hardlinks --no-checkout "$checkout" "$clone" >"$clone_log" 2>&1 \
    || die 1 "independent local clone failed (log: $clone_log)"
  git -C "$clone" checkout --detach "$subject" >>"$clone_log" 2>&1 \
    || die 1 "clone could not detach at subject $subject (retained log: $clone_log)"
  [[ -d "$clone/.git" && ! -f "$clone/.git" ]] \
    || die 1 "isolated subject is not an independent clone"
  [[ "$(git -C "$clone" rev-parse HEAD)" == "$subject" ]] \
    || die 1 "detached clone does not name the recorded subject"
  if git -C "$checkout" worktree list --porcelain | sed -n 's/^worktree //p' | grep -Fqx "$clone"; then
    die 1 "independent clone entered the linked-worktree inventory"
  fi
  clone_metasystem=$clone${prefix:+/$prefix}
  [[ -f "$clone_metasystem/scripts/agents/milestone-battery.sh" ]] \
    || die 1 "subject does not contain the battery controller"
  cp "$clone_metasystem/scripts/agents/milestone-battery.sh" "$run_dir/controller.sh"
  chmod 700 "$run_dir/controller.sh"
  cleanup_bootstrap=0
  trap - EXIT INT TERM
  exec env GOCACHE="$shared_gocache" \
    METASYSTEM_SUPERVISION_REGISTRY_HOME="$run_dir/supervision-home" \
      "$run_dir/controller.sh" __isolated_controller \
        --run-id "$run_id" \
        --run-dir "$run_dir" --clone "$clone" --clone-metasystem "$clone_metasystem" \
      --real-metasystem "$metasystem_root" --subject "$subject" \
      --evidence-root "$evidence_root" --original-home "${HOME:-}" \
      --shared-gocache "$shared_gocache" --force-red "$force_red"
fi

shift
run_id= run_dir= clone= clone_metasystem= real_metasystem= subject= evidence_root= original_home= shared_gocache= force_red=0
while (($#)); do
  case "$1" in
    --run-id) run_id=$2; shift 2 ;;
    --run-dir) run_dir=$2; shift 2 ;; --clone) clone=$2; shift 2 ;;
    --clone-metasystem) clone_metasystem=$2; shift 2 ;;
    --real-metasystem) real_metasystem=$2; shift 2 ;; --subject) subject=$2; shift 2 ;;
    --evidence-root) evidence_root=$2; shift 2 ;; --original-home) original_home=$2; shift 2 ;;
    --shared-gocache) shared_gocache=$2; shift 2 ;; --force-red) force_red=$2; shift 2 ;;
    *) die 2 "invalid internal controller argument: $1" ;;
  esac
done
for required in run_id run_dir clone clone_metasystem real_metasystem subject evidence_root shared_gocache; do
  [[ -n "${!required}" ]] || die 2 "internal controller is missing --${required//_/-}"
done
[[ "$run_dir" == /* && "$clone" == "$run_dir"/* && "$clone_metasystem" == "$clone"* ]] \
  || die 2 "internal controller paths do not share the validated run directory"
[[ "${HOME:-}" == "$original_home" ]] || die 1 "isolated startup changed process HOME"
[[ "$(go env GOCACHE)" == "$shared_gocache" ]] || die 1 "isolated startup did not preserve the declared shared GOCACHE"
[[ "${METASYSTEM_SUPERVISION_REGISTRY_HOME:-}" == "$run_dir/supervision-home" ]] \
  || die 1 "isolated startup did not preserve the run-scoped supervision registry home"

envelope_parent=$evidence_root/suite-failures
envelope=$envelope_parent/$run_id
stage=$envelope_parent/.${run_id}.stage.$$
isolated_evidence=$run_dir/isolated-evidence
registry_home=$run_dir/supervision-home
setup_log=$run_dir/setup.log
validation_log=$run_dir/validation.log
checkpoint_json=$run_dir/checkpoint.json
surface_json=$run_dir/surface.json
toolchain_file=$run_dir/toolchain.txt
controller_engine=$run_dir/metasystem
started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
ended_at=
setup_exit=1
validation_exit=-1
reset_rc=-1
surface_digest=unavailable
surface_policy_version=0
toolchain_identity=unavailable
final_verdict=setup-failed
checkpoint_open=0
terminalized=0
stage_published=0
retain_clone=0
should_abandon=0
abandon_best_effort=0
supervision_armed=0
validator_active=0
validator_launching=0
validator_pid=
validator_pgid=
validator_identity=
validator_identity_pid=
validator_identity_started_at=
validator_identity_start_ticks=
validator_identity_boot_id=
validator_pgid_file=${BATTERY_VALIDATOR_PGID_FILE:-$run_dir/validator-identity/validator.pgid}
validator_pgid_stage=${BATTERY_VALIDATOR_PGID_STAGE:-$run_dir/validator-identity/.validator-pgid.$$.stage}
validator_publication_dir=${validator_pgid_file%/*}
teardown_note=not-attempted

json_escape() { # value
  local value=$1
  value=${value//\\/\\\\}
  value=${value//\"/\\\"}
  value=${value//$'\n'/\\n}
  value=${value//$'\r'/\\r}
  value=${value//$'\t'/\\t}
  printf '%s' "$value"
}

sha256_file() { # file
  if [[ -x "$controller_engine" ]]; then
    "$controller_engine" util sha256 --file "$1"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

atomic_json_field() { # file field value
  "$controller_engine" json set --file "$1" --field "$2=$3" >/dev/null
}

atomic_json_int() { # file field integer
  "$controller_engine" json set --file "$1" --int "$2=$3" >/dev/null
}

publish_stage_one() { # setup exit, validation exit, verdict
  local stage_setup=$1 stage_validation=$2 initial_verdict=$3 relative digest actual expected rejected_link
  mkdir -p "$envelope_parent" || return 1
  [[ ! -e "$envelope" ]] || { stage_published=1; return 0; }
  rm -rf -- "$stage" 2>/dev/null || true
  mkdir "$stage" || return 1
  cp "$setup_log" "$validation_log" "$checkpoint_json" "$surface_json" "$toolchain_file" "$stage/" \
    || return 1
  printf '%s\n' "$subject" >"$stage/subject.sha" || return 1
  printf '{"setup":%d,"validation":%d}\n' "$stage_setup" "$stage_validation" >"$stage/exit-codes.json" || return 1
  printf '{"startedAt":"%s","endedAt":"%s"}\n' "$started_at" "$ended_at" >"$stage/timings.json" || return 1
  if [[ -f "$registry_home/.metasystem/armed-checkouts.jsonl" ]]; then
    cp "$registry_home/.metasystem/armed-checkouts.jsonl" "$stage/supervision-registry.jsonl" || return 1
  fi
  if [[ -f "$clone_metasystem/artifacts/agents/supervision/last-census.json" ]]; then
    cp "$clone_metasystem/artifacts/agents/supervision/last-census.json" "$stage/last-census.json" || return 1
  fi
  if [[ -d "$clone_metasystem/artifacts/agents/suite-failures" ]]; then
    mkdir -p "$stage/failure-artifacts/suite-failures"
    cp -R "$clone_metasystem/artifacts/agents/suite-failures/." "$stage/failure-artifacts/suite-failures/" || return 1
  fi
  if [[ -d "$clone_metasystem/artifacts/agents/gate-failures" ]]; then
    mkdir -p "$stage/failure-artifacts/gate-failures"
    cp -R "$clone_metasystem/artifacts/agents/gate-failures/." "$stage/failure-artifacts/gate-failures/" || return 1
  fi
  rejected_link=$(find "$stage" -type l -print -quit) || return 1
  if [[ -n "$rejected_link" ]]; then
    printf 'milestone battery evidence copy refused symlink: %s\n' "$rejected_link" >&2
    return 1
  fi
  : >"$stage/copy-digests.nul"
  while IFS= read -r -d '' relative; do
    relative=${relative#./}
    [[ "$relative" == copy-digests.nul || "$relative" == report.json ]] && continue
    digest=$(sha256_file "$stage/$relative") || return 1
    printf '%s\0%s\0' "$digest" "$relative" >>"$stage/copy-digests.nul"
  done < <(cd "$stage" && find . -type f -print0)
  exec 7<"$stage/copy-digests.nul"
  while IFS= read -r -d '' expected <&7 && IFS= read -r -d '' relative <&7; do
    actual=$(sha256_file "$stage/$relative") || { exec 7<&-; return 1; }
    [[ "$actual" == "$expected" ]] || { exec 7<&-; return 1; }
  done
  exec 7<&-
  printf '{"runId":"%s","subjectSHA":"%s","surfaceProjection":"LANDING","surfacePolicyVersion":%d,"surfaceDigest":"%s","toolchainIdentity":"%s","startedAt":"%s","endedAt":"%s","setupExit":%d,"validationExit":%d,"copyResult":"verified","copyDigestManifest":"copy-digests.nul","verdict":"%s","validationLog":"validation.log","failureArtifacts":"failure-artifacts"}\n' \
    "$run_id" "$subject" "$surface_policy_version" "$surface_digest" "$toolchain_identity" \
    "$started_at" "$ended_at" "$stage_setup" "$stage_validation" "$initial_verdict" >"$stage/report.json" \
    || return 1
  mv "$stage" "$envelope" || return 1
  stage_published=1
  return 0
}

launch_job_is_owned() { # pid
  local job_pid
  while IFS= read -r job_pid; do
    [[ "$job_pid" == "$1" ]] && return 0
  done < <(jobs -p)
  return 1
}

adopt_validator_identity() { # identity JSON, expected pid
  local identity=$1 expected_pid=$2 pid liveness started_at start_ticks boot_id
  pid=$("$controller_engine" json get --value "$identity" --field pid 2>/dev/null) || return 1
  liveness=$("$controller_engine" json get --value "$identity" --field liveness 2>/dev/null) || return 1
  started_at=$("$controller_engine" json get --value "$identity" --field startedAtUnix 2>/dev/null) || return 1
  start_ticks=$("$controller_engine" json get --value "$identity" --field startTicks 2>/dev/null) || return 1
  boot_id=$("$controller_engine" json get --value "$identity" --field bootId 2>/dev/null) || return 1
  [[ "$pid" =~ ^[1-9][0-9]*$ && "$pid" == "$expected_pid" ]] || return 1
  [[ "$liveness" == alive && "$started_at" =~ ^[1-9][0-9]*$ ]] || return 1
  [[ "$start_ticks" =~ ^[0-9]+$ ]] || return 1
  if (( start_ticks > 0 )); then
    [[ -n "$boot_id" ]] || return 1
  else
    [[ -z "$boot_id" ]] || return 1
  fi
  validator_identity=$identity
  validator_identity_pid=$pid
  validator_identity_started_at=$started_at
  validator_identity_start_ticks=$start_ticks
  validator_identity_boot_id=$boot_id
  validator_pid=$pid
  validator_pgid=$pid
  return 0
}

probe_and_adopt_validator_identity() { # pid
  local identity
  identity=$("$controller_engine" proc probe --pid "$1" 2>/dev/null) || return 1
  adopt_validator_identity "$identity" "$1"
}

validator_identity_matches() { # identity JSON
  local identity=$1 pid liveness started_at started_at_micro start_ticks boot_id recorded_micro
  pid=$("$controller_engine" json get --value "$identity" --field pid 2>/dev/null) || return 1
  liveness=$("$controller_engine" json get --value "$identity" --field liveness 2>/dev/null) || return 1
  started_at=$("$controller_engine" json get --value "$identity" --field startedAtUnix 2>/dev/null) || return 1
  start_ticks=$("$controller_engine" json get --value "$identity" --field startTicks 2>/dev/null) || return 1
  boot_id=$("$controller_engine" json get --value "$identity" --field bootId 2>/dev/null) || return 1
  [[ "$liveness" == alive && "$pid" == "$validator_identity_pid" ]] || return 1
  if (( validator_identity_start_ticks > 0 )); then
    [[ "$start_ticks" == "$validator_identity_start_ticks" \
      && "$boot_id" == "$validator_identity_boot_id" ]]
    return
  fi
  [[ "$started_at" == "$validator_identity_started_at" ]] || return 1
  # Darwin publishes microsecond start time even though proc alive retains the
  # repository's seconds-compatible record contract. Use that exact value when
  # authenticating the wrapper's publication against the controller probe.
  recorded_micro=$("$controller_engine" json get --value "$validator_identity" --field startedAtUnixMicro 2>/dev/null) || return 1
  started_at_micro=$("$controller_engine" json get --value "$identity" --field startedAtUnixMicro 2>/dev/null) || return 1
  [[ "$started_at_micro" == "$recorded_micro" ]]
}

validator_identity_is_alive() {
  local pair_args=()
  [[ "$validator_identity_pid" =~ ^[1-9][0-9]*$ \
    && "$validator_identity_started_at" =~ ^[1-9][0-9]*$ ]] || return 1
  if (( validator_identity_start_ticks > 0 )); then
    [[ -n "$validator_identity_boot_id" ]] || return 1
    pair_args=(--start-ticks "$validator_identity_start_ticks" --boot-id "$validator_identity_boot_id")
  fi
  "$controller_engine" proc alive --pid "$validator_identity_pid" \
    --start-time "$validator_identity_started_at" \
    "${pair_args[@]+"${pair_args[@]}"}" --root "$clone_metasystem" >/dev/null 2>&1
}

read_published_validator_identity() {
  [[ -f "$validator_pgid_file" ]] || return 1
  IFS= read -r validator_publication <"$validator_pgid_file" || return 1
  [[ -n "$validator_publication" ]]
}

revoke_validator_publication() {
  local publication_name=${validator_pgid_file##*/}
  local revoked_dir=${validator_publication_dir}.revoked.$$
  [[ "$validator_pgid_file" == /* && "$validator_publication_dir" != / \
    && "$publication_name" != "$validator_pgid_file" \
    && -d "$validator_publication_dir" && ! -e "$revoked_dir" ]] || return 1
  mv "$validator_publication_dir" "$revoked_dir" || return 1
  validator_pgid_file=$revoked_dir/$publication_name
  return 0
}

stop_validator() {
  (( validator_active || validator_launching )) || return 0
  local attempt candidate candidate_liveness direct_launch_pid group_valid=0
  local launch_job_dead=0 launch_job_owned=0 launch_pid launch_pid_valid=0
  local validator_publication=
  set +u
  direct_launch_pid=$!
  set -u
  launch_pid=$direct_launch_pid
  if [[ -n "${BATTERY_VALIDATOR_LAUNCH_PID_FILE:-}" \
    && -f "$BATTERY_VALIDATOR_LAUNCH_PID_FILE" ]]; then
    IFS= read -r launch_pid <"$BATTERY_VALIDATOR_LAUNCH_PID_FILE" || true
  fi
  if (( validator_launching && ! validator_active )); then
    # TERM can be delivered between the asynchronous command and the parent
    # assignments below. The child owns a process group under monitor mode, so
    # teardown waits for the identity it publishes before entering the clone.
    if [[ "$launch_pid" =~ ^[1-9][0-9]*$ ]] && (( launch_pid > 1 )); then
      launch_pid_valid=1
      [[ "$launch_pid" == "$direct_launch_pid" ]] && launch_job_owned=1
      for ((attempt=0; attempt<100; attempt++)); do
        validator_publication=
        if read_published_validator_identity \
          && adopt_validator_identity "$validator_publication" "$launch_pid"; then
          validator_active=1
          break
        fi
        if launch_job_is_owned "$launch_pid"; then
          candidate=$("$controller_engine" proc probe --pid "$launch_pid" 2>/dev/null) || candidate=
          candidate_liveness=$("$controller_engine" json get --value "$candidate" --field liveness 2>/dev/null || true)
          if [[ "$candidate_liveness" == alive ]]; then
            sleep 0.05
            continue
          fi
          # The job-table row is only a wait locator. Identity publication owns
          # signal authority; a failed publisher has no descendants because it
          # exits before cd/exec.
          wait "$launch_pid" 2>/dev/null || true
        fi
        break
      done
    fi
    if (( ! validator_active )); then
      # A poll can find a publication but cannot prove that one will never
      # arrive. Renaming the channel closes the wrapper's absolute destination
      # while preserving any publication that won the rename race.
      revoke_validator_publication || return 1
      # Revocation rejects every future path resolution, but a rename already
      # inside the kernel can still land in the moved directory. Wait until
      # that publication appears, the owned launch job dies, or the grace
      # expires into clone retention. Publication precedes cd/exec and exec
      # preserves the launch PID, so an absent publication after job-table
      # death and wait consumption proves that no validator entered the clone.
      # Adopt, refuse after death, or retain: there is no fourth outcome.
      for ((attempt=0; attempt<100; attempt++)); do
        validator_publication=
        if read_published_validator_identity; then
          if (( launch_pid_valid )) \
            && adopt_validator_identity "$validator_publication" "$launch_pid"; then
            validator_active=1
            break
          fi
          return 1
        fi
        launch_job_dead=0
        if (( launch_pid_valid )) && launch_job_is_owned "$launch_pid"; then
          launch_job_owned=1
          candidate=$("$controller_engine" proc probe --pid "$launch_pid" 2>/dev/null) || candidate=
          candidate_liveness=$("$controller_engine" json get --value "$candidate" --field liveness 2>/dev/null || true)
          [[ "$candidate_liveness" == dead ]] && launch_job_dead=1
        elif (( launch_job_owned )); then
          launch_job_dead=1
        fi
        if (( launch_job_dead )); then
          wait "$launch_pid" 2>/dev/null || true
          validator_publication=
          if read_published_validator_identity; then
            if adopt_validator_identity "$validator_publication" "$launch_pid"; then
              validator_active=1
              break
            fi
            return 1
          fi
          if [[ -n "${BATTERY_VALIDATOR_PUBLICATION_STALL_DIR:-}" ]]; then
            : >"$BATTERY_VALIDATOR_PUBLICATION_STALL_DIR/exit-observed" || return 1
          fi
          validator_launching=0
          return 0
        fi
        sleep 0.05
      done
    fi
    if (( ! validator_active )); then
      return 1
    fi
  fi
  if [[ "$validator_pgid" =~ ^[1-9][0-9]*$ && "$validator_pid" =~ ^[1-9][0-9]*$ \
    && "$validator_pgid" == "$validator_pid" ]] && (( validator_pgid > 1 )); then
    group_valid=1
  fi
  # Identity-verified signaling deliberately narrows the irreducible shell
  # race to the probe-then-kill microseconds and refuses on every mismatch.
  # This is the supervision census guarantee; POSIX gives a shell no stronger
  # primitive for signaling a process identity rather than a numeric PID.
  if (( group_valid )) \
    && "$controller_engine" proc group-exists --pgid "$validator_pgid" >/dev/null 2>&1; then
    if validator_identity_is_alive; then
      if ! kill -TERM -- "-$validator_pgid" 2>/dev/null; then
        validator_identity_is_alive && kill -TERM "$validator_pid" 2>/dev/null || true
      fi
    fi
  elif validator_identity_is_alive; then
    kill -TERM "$validator_pid" 2>/dev/null || true
  fi
  for ((attempt=0; attempt<100; attempt++)); do
    validator_identity_is_alive || break
    sleep 0.05
  done
  if (( group_valid )) \
    && "$controller_engine" proc group-exists --pgid "$validator_pgid" >/dev/null 2>&1; then
    if validator_identity_is_alive; then
      if ! kill -KILL -- "-$validator_pgid" 2>/dev/null; then
        validator_identity_is_alive && kill -KILL "$validator_pid" 2>/dev/null || true
      fi
    fi
  elif validator_identity_is_alive; then
    kill -KILL "$validator_pid" 2>/dev/null || true
  fi
  wait "$validator_pid" 2>/dev/null || true
  validator_active=0
  validator_launching=0
}

shutdown_supervision() {
  local owner_record=$clone_metasystem/artifacts/agents/supervision/lock.d/owner.json
  (( supervision_armed )) || [[ -f "$owner_record" ]] || return 0
  if METASYSTEM_BIN="$controller_engine" \
      "$clone_metasystem/scripts/agents/arm-supervision.sh" --repo "$clone" --shutdown \
      >>"$setup_log" 2>&1; then
    supervision_armed=0
    return 0
  fi
  return 1
}

abandon_checkpoint() { # reason
  local reason=$1 option=()
  (( checkpoint_open && ! terminalized )) || return 0
  (( abandon_best_effort )) && option=(--best-effort-appendix)
  if "$controller_engine" gate weight-abandon --root "$real_metasystem" \
      --run-id "$run_id" --reason "$reason" "${option[@]+"${option[@]}"}" \
      >"$run_dir/abandon.json" 2>>"$setup_log"; then
    terminalized=1
    checkpoint_open=0
    return 0
  fi
  return 1
}

publish_outcome() {
  local tmp=$envelope/.outcome.$$.json
  if [[ -x "$controller_engine" ]]; then
    printf '{}\n' >"$tmp" || return 1
    atomic_json_field "$tmp" runId "$run_id" || return 1
    atomic_json_field "$tmp" subject "$subject" || return 1
    atomic_json_field "$tmp" verdict "$final_verdict" || return 1
    atomic_json_int "$tmp" setupExit "$setup_exit" || return 1
    atomic_json_int "$tmp" validationExit "$validation_exit" || return 1
    atomic_json_int "$tmp" resetExit "$reset_rc" || return 1
  else
    printf '{"runId":"%s","subject":"%s","verdict":"%s","setupExit":%d,"validationExit":%d,"resetExit":%d}\n' \
      "$run_id" "$subject" "$final_verdict" "$setup_exit" "$validation_exit" "$reset_rc" >"$tmp" || return 1
  fi
  mv "$tmp" "$envelope/outcome.json"
}

publish_teardown() { # result, retained path
  local result=$1 retained=$2 tmp=$envelope/.teardown.$$.json
  if [[ -x "$controller_engine" ]]; then
    printf '{}\n' >"$tmp" || return 1
    atomic_json_field "$tmp" runId "$run_id" || return 1
    atomic_json_field "$tmp" subject "$subject" || return 1
    atomic_json_field "$tmp" result "$result" || return 1
    atomic_json_field "$tmp" retainedPath "$retained" || return 1
  else
    printf '{"runId":"%s","subject":"%s","result":"%s","retainedPath":"%s"}\n' \
      "$run_id" "$subject" "$result" "$(json_escape "$retained")" >"$tmp" || return 1
  fi
  mv "$tmp" "$envelope/teardown.json"
}

controller_finalize() { # incoming status
  local status=$1 abandonment_reason=$final_verdict retained_path=
  trap - EXIT
  trap '' INT TERM
  if ! stop_validator; then
    retain_clone=1
    teardown_note=validator-publication-revocation-failed
    [[ $status == 130 ]] || status=1
  fi
  ended_at=${ended_at:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
  if ! publish_stage_one "$setup_exit" "$validation_exit" "$final_verdict"; then
    stage_published=0
    retain_clone=1
    should_abandon=1
    abandon_best_effort=1
    final_verdict=evidence-incomplete
    status=1
  fi

  if ! shutdown_supervision; then
    retain_clone=1
    teardown_note=recorded-process-shutdown-failed
    [[ $status == 130 ]] || status=1
  fi

  if (( should_abandon && checkpoint_open )); then
    [[ "$final_verdict" == evidence-incomplete ]] && abandonment_reason=evidence-copy-failed
    if ! abandon_checkpoint "$abandonment_reason"; then
      final_verdict=$final_verdict/abandonment-unrecorded
      retain_clone=1
      [[ $status == 130 ]] || status=1
    fi
  fi

  if (( stage_published )); then
    if ! publish_outcome; then
      final_verdict=$final_verdict/outcome-unrecorded
      retain_clone=1
      [[ $status == 130 ]] || status=1
    fi
  fi

  if [[ "$teardown_note" == not-attempted ]]; then
    if (( retain_clone )); then
      teardown_note=retained-by-prior-failure
    else
      rm -rf -- "$clone" 2>/dev/null || true
      if [[ -e "$clone" ]]; then
        retain_clone=1
        teardown_note=clone-removal-failed
        [[ $status == 130 ]] || status=1
      else
        teardown_note=removed
      fi
    fi
  fi
  [[ -e "$clone" ]] && retained_path=$clone

  if (( stage_published )); then
    if ! publish_teardown "$teardown_note" "$retained_path"; then
      [[ $status == 130 ]] || status=1
      if [[ -n "$retained_path" ]]; then
        echo "milestone battery teardown evidence incomplete; retained clone: subject=$subject path=$retained_path envelope=$envelope" >&2
      else
        echo "milestone battery teardown evidence incomplete; clone already removed: subject=$subject envelope=$envelope" >&2
      fi
    fi
  fi

  if [[ "$final_verdict" != green && $status == 0 ]]; then status=1; fi
  if [[ -z "$retained_path" ]]; then
    rm -rf -- "$run_dir" 2>/dev/null || true
  fi
  if [[ -n "$retained_path" ]]; then
    echo "milestone battery $final_verdict; retained clone: subject=$subject path=$retained_path envelope=${envelope:-unpublished}" >&2
  elif [[ $status == 0 ]]; then
    echo "milestone battery green: subject=$subject envelope=$envelope"
  else
    echo "milestone battery $final_verdict: subject=$subject clone=removed envelope=${envelope:-unpublished}" >&2
  fi
  exit "$status"
}

controller_exit() {
  local status=$?
  controller_finalize "$status"
}

operator_abort() {
  trap '' INT TERM
  final_verdict=operator-abort
  validation_exit=130
  should_abandon=1
  printf 'operator abort requested; stopping validation before checkpoint terminalization\n' >>"$validation_log"
  exit 130
}

trap controller_exit EXIT
trap operator_abort INT TERM

# From the first fallible controller operation onward, EXIT owns evidence and
# teardown. Even failure to initialize a run-local artifact reaches the same
# finalizer; the stage publisher will retain the clone if it cannot assemble a
# complete envelope.
mkdir -p "$isolated_evidence" "$registry_home"
: >"$setup_log"
: >"$validation_log"
printf '{}\n' >"$checkpoint_json"
printf '{"policyVersion":0,"projection":"LANDING","endpoint":"unavailable","surfaceDigest":"unavailable"}\n' >"$surface_json"
printf '%s\n' "$toolchain_identity" >"$toolchain_file"

# Only committed generic values enter conf.local. The live checkout's local
# file is never opened or copied by the controller.
template=$clone_metasystem/scripts/agents/battery.conf.local.template
[[ -f "$template" ]] || die 1 "subject lacks battery.conf.local.template"
: >"$clone_metasystem/metasystem.conf.local"
while IFS= read -r line || [[ -n "$line" ]]; do
  if [[ "$line" == 'evidence.root=@RUN_EVIDENCE_ROOT@' ]]; then
    printf 'evidence.root=%s\n' "$isolated_evidence"
  else
    printf '%s\n' "$line"
  fi
done <"$template" >"$clone_metasystem/metasystem.conf.local"

set +e
{
  echo "runId=$run_id"
  echo "subject=$subject"
  echo "clone=$clone"
  echo "registryHome=$registry_home"
  echo "sharedGOCACHE=$shared_gocache"
  echo "processHOME=${HOME:-}"
  bash "$clone_metasystem/scripts/agents/go-build.sh" --out "$controller_engine"
} >>"$setup_log" 2>&1
setup_exit=$?
set -e
if [[ $setup_exit != 0 ]]; then
  final_verdict=setup-failed
  exit 1
fi
setup_exit=1
chmod 700 "$controller_engine"
# Supervision fingerprints the checkout's own binary, and a fresh
# clone ships none (bin/ is ignored scratch): the clone consumes the
# engine built from its own bytes, staged beside the target and
# renamed over it so no reader ever sees a torn binary.
mkdir -p "$clone_metasystem/bin"
cp "$controller_engine" "$clone_metasystem/bin/.metasystem.stage.$$"
mv -f "$clone_metasystem/bin/.metasystem.stage.$$" "$clone_metasystem/bin/metasystem"

clone_prefix=${clone_metasystem#"$clone"}; clone_prefix=${clone_prefix#/}
set +e
"$controller_engine" behavior-surface digest --root "$clone" --prefix "${clone_prefix:+$clone_prefix/}" \
  --projection LANDING --endpoint "isolated subject $subject" >"$surface_json" 2>>"$setup_log"
setup_exit=$?
set -e
if [[ $setup_exit != 0 ]]; then
  final_verdict=setup-failed
  exit 1
fi
setup_exit=1
surface_digest=$("$controller_engine" json get --file "$surface_json" --field surfaceDigest)
surface_policy_version=$("$controller_engine" json get --file "$surface_json" --field policyVersion)
toolchain_identity=$(cd "$clone_metasystem" \
  && { go version; go env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN; } \
  | "$controller_engine" util sha256)
printf '%s\n' "$toolchain_identity" >"$toolchain_file"

checkpoint_rc=0
"$controller_engine" gate weight-checkpoint --root "$real_metasystem" \
  --run-id "$run_id" --subject "$subject" --runner-pid $$ --envelope "$envelope" \
  >"$checkpoint_json" 2>>"$setup_log" || checkpoint_rc=$?
if [[ $checkpoint_rc != 0 ]]; then
  setup_exit=$checkpoint_rc
  final_verdict=checkpoint-refused
  if [[ $checkpoint_rc == 3 ]]; then
    echo "milestone battery refused: another live or unprovable runner owns the checkpoint; subject=$subject" >&2
  fi
  exit "$checkpoint_rc"
fi
checkpoint_open=1

set +e
METASYSTEM_BIN="$controller_engine" METASYSTEM_AGENT_RUNTIME="${METASYSTEM_AGENT_RUNTIME:-codex}" \
  "$clone_metasystem/scripts/agents/arm-supervision.sh" --repo "$clone" \
    --pid $$ --session "battery-$run_id" --tag "metasystem-battery-$run_id" \
    >>"$setup_log" 2>&1
arm_rc=$?
set -e
if [[ $arm_rc != 0 ]]; then
  setup_exit=$arm_rc
  final_verdict=supervision-arm-failed
  should_abandon=1
  exit 1
fi
supervision_armed=1
setup_exit=0

if (( force_red )); then
  mkdir -p "$clone_metasystem/artifacts/agents/suite-failures/forced-red"
  printf 'forced red for evidence lifecycle fixture\n' \
    >"$clone_metasystem/artifacts/agents/suite-failures/forced-red/failure.txt"
  printf 'forced red for evidence lifecycle fixture\n' >"$validation_log"
  validation_exit=97
else
  mkdir -p "$validator_publication_dir"
  rm -f -- "$validator_pgid_file" "$validator_pgid_stage"
  validator_active=0
  validator_identity=
  validator_identity_pid=
  validator_identity_started_at=
  validator_identity_start_ticks=
  validator_identity_boot_id=
  recorded_probe_ok=0
  recorded_publication_ok=0
  validator_launching=1
  set -m
  bash -c '
    stall_dir=$5
    if [[ -n "$stall_dir" ]]; then
      on_signal() { touch "$stall_dir/signaled"; exit 125; }
      trap on_signal TERM INT HUP
      printf "%s\n" "$$" >"$stall_dir/launch.pid" || exit
      : >"$stall_dir/waiting" || exit
      while [[ ! -e "$stall_dir/release" ]]; do sleep 0.01; done
      : >"$stall_dir/publication-attempted" || exit
    fi
    "$3" proc probe --pid "$$" >"$1" || exit
    mv "$1" "$2" || exit
    cd "$4" || exit
    exec bash scripts/validate-metasystem.sh
  ' validator-launch "$validator_pgid_stage" "$validator_pgid_file" \
    "$controller_engine" "$clone_metasystem" \
    "${BATTERY_VALIDATOR_PUBLICATION_STALL_DIR:-}" \
    >"$validation_log" 2>&1 &
  # Trap delivery between the asynchronous command and the PID assignments is
  # a distinct ownership state. A configured hold file keeps that window open
  # until its coordinator removes the file; an unset path adds no delay.
  if [[ -n "${BATTERY_LAUNCH_HOLD_FILE:-}" ]]; then
    while [[ -e "$BATTERY_LAUNCH_HOLD_FILE" ]]; do sleep 0.01; done
  fi
  validator_pid=$!
  if probe_and_adopt_validator_identity "$validator_pid"; then
    recorded_probe_ok=1
  fi
  if (( recorded_probe_ok )); then
    for ((attempt=0; attempt<100; attempt++)); do
      validator_publication=
      if read_published_validator_identity; then
        if validator_identity_matches "$validator_publication"; then
          recorded_publication_ok=1
        fi
        break
      fi
      validator_identity_is_alive || break
      sleep 0.01
    done
  fi
  if (( recorded_probe_ok && recorded_publication_ok )); then
    validator_active=1
  fi
  validator_launching=0
  if (( validator_active )); then
    printf 'validatorPid=%s validatorPgid=%s\n' "$validator_pid" "$validator_pgid" >>"$setup_log"
  fi
  set +m
  set +e
  wait "$validator_pid"
  validation_exit=$?
  set -e
  validator_active=0
fi
ended_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
final_verdict=red
[[ $validation_exit == 0 ]] && final_verdict=green

if ! publish_stage_one "$setup_exit" "$validation_exit" "$final_verdict"; then
  retain_clone=1
  should_abandon=1
  abandon_best_effort=1
  final_verdict=evidence-incomplete
  exit 1
fi

if [[ $validation_exit != 0 ]]; then
  should_abandon=1
  exit 1
fi

reset_rc=0
"$controller_engine" gate weight-reset --root "$real_metasystem" --run-id "$run_id" \
  >"$run_dir/reset-command.json" 2>>"$setup_log" || reset_rc=$?
case "$reset_rc" in
  0) terminalized=1; checkpoint_open=0; final_verdict=green; exit 0 ;;
  4) terminalized=1; checkpoint_open=0; final_verdict=green/reset-appendix-pending; exit 1 ;;
  *) final_verdict=green/reset-unrecorded; exit 1 ;;
esac
