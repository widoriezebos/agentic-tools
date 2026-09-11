#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
source "$root/scripts/agents/fixture-bed-scenarios.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario brain "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO
unset GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES

scenarios=(brain-boot-caps brain-boot-bound brain-boot-errors brain-boot-stalled brain-declare-quiescence brain-declare-race brain-second-declaration-refuses brain-verbs-human-only brain-actor-seam-coverage)
if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios brain "brain fixtures: PASSED" "$fixture_bed_script" "${scenarios[@]}"
fi
case "$fixture_scenario" in
  brain-boot-caps | brain-boot-bound | brain-boot-errors | brain-boot-stalled | brain-declare-quiescence | brain-declare-race | brain-second-declaration-refuses | brain-verbs-human-only | brain-actor-seam-coverage) ;;
  *) echo "brain fixtures: unknown scenario: $fixture_scenario" >&2; exit 64 ;;
esac

harness_fixture_warn_if_engine_stale "$root"
ms=${METASYSTEM_BIN:-$root/bin/metasystem}
[[ -x "$ms" ]] || { echo "brain fixtures: bin/metasystem is not built" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-brain.XXXXXX")
cleanup() {
  local status=$? keep
  if [[ $status -ne 0 && -d "$tmp" ]]; then
    keep="$root/artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-brain-$$"
    mkdir -p "$(dirname "$keep")"
    mv "$tmp" "$keep" 2>/dev/null && echo "brain fixture evidence preserved: $keep" >&2
    return 0
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

bed_origin= bed_clone= bed_identity=
setup_ledger() {
  local label=$1 machine=$2 digest ledger manifest migrate_out fixture_start
  bed_origin=$tmp/$label-origin.git
  bed_clone=$tmp/$label-clone
  git init -q --bare "$bed_origin"
  git init -q -b main "$bed_clone"
  git -C "$bed_clone" config user.name fixture
  git -C "$bed_clone" config user.email fixture@example.invalid
  git -C "$bed_clone" config metasystem.goal.machine "$machine"
  git -C "$bed_clone" remote add origin "$bed_origin"
  git -C "$bed_clone" commit -q --allow-empty -m seed
  mkdir -p "$bed_clone/plans" "$bed_clone/scripts/agents" "$bed_clone/records/misc"
  cp "$root/scripts/agents/pre-commit-guard.sh" "$bed_clone/scripts/agents/"
  cp -R "$root/scripts/agents/adapters" "$bed_clone/scripts/agents/"
  cp "$root/records/misc/fleet-coordinator-brain-role-packet.md" "$bed_clone/records/misc/"
  cat >"$bed_clone/plans/goals.md" <<'LEDGER'
# Goals

## Current goal: ship-widget — Ship the widget
- Origin: main
- Next step: Let a node finish it.
LEDGER
  ledger=$(cat "$bed_clone/plans/goals.md" && printf x) && ledger=${ledger%x}
  "$ms" json object ledger="$ledger" sha256="$(shasum -a 256 "$bed_clone/plans/goals.md" | cut -d' ' -f1)" >"$bed_clone/plans/goals-accepted.json"
  "$ms" json set --file "$bed_clone/plans/goals-accepted.json" --int schemaVersion=1
  printf '%s\n' 'metasystem.runtimes=fake' >"$bed_clone/metasystem.conf"
  git -C "$bed_clone" add plans scripts records metasystem.conf
  git -C "$bed_clone" commit -qm "legacy ledger"
  git -C "$bed_clone" push -q origin main
  fixture_start=$("$ms" proc started-at --pid "$$")
  "$ms" lease announce --root "$bed_clone" --session "brain-$label" --pid "$$" --start "$fixture_start" --tag "brain-$label" --runtime fake --owner-lineage fixture-lineage >/dev/null
  digest=$("$ms" goal source-digest --root "$bed_clone")
  manifest=$tmp/$label-manifest.md
  cat >"$manifest" <<MANIFEST
# Queue amendments

MIGRATION_EPOCH: 2026-09-07T00:00:00Z
REVIEWED_SOURCE_SHA256: $digest
MANIFEST
  migrate_out=$(cd "$bed_clone" && METASYSTEM_OWNER_LINEAGE=fixture-lineage "$ms" goal migrate --root "$bed_clone" --source-digest "$digest" --manifest "$manifest" --by Wido)
  bed_identity=$(sed -n 's/.*"identity": "\([^"]*\)".*/\1/p' <<<"$migrate_out" | head -1)
  [[ ${#bed_identity} -eq 26 ]] || { echo "brain fixture migration reported no ledger identity: $migrate_out" >&2; exit 1; }
  git -C "$bed_clone" fetch -q origin
  git -C "$bed_clone" reset -q --hard origin/main
  git -C "$bed_clone" update-ref refs/metasystem/goals/accepted origin/main
  export METASYSTEM_OWNER_LINEAGE=fixture-lineage
}

release_current() {
  "$ms" goal release --root "$bed_clone" --id ship-widget >/dev/null
  git -C "$bed_clone" fetch -q origin
  git -C "$bed_clone" reset -q --hard origin/main
  git -C "$bed_clone" update-ref refs/metasystem/goals/accepted origin/main
}

assert_refused() {
  local expected=$1 output rc
  shift
  set +e
  output=$("$@" 2>&1)
  rc=$?
  set -e
  [[ $rc -eq 2 && "$output" == *"$expected"* ]] || { echo "expected exit 2 containing '$expected', got rc=$rc: $output" >&2; exit 1; }
}

clone_same_ledger() {
  local destination=$1 machine=$2
  git clone -q "$bed_origin" "$destination"
  git -C "$destination" config user.name fixture
  git -C "$destination" config user.email fixture@example.invalid
  git -C "$destination" config metasystem.goal.machine "$machine"
  git -C "$destination" update-ref refs/metasystem/goals/accepted origin/main
}

if [[ "$fixture_scenario" == brain-boot-caps ]]; then
  setup_ledger caps brain
  registry=$tmp/registry; mkdir -p "$registry"; export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  git -C "$bed_clone" config metasystem.goal.machine "$(printf 'm%.0s' {1..33})"
  assert_refused "32-byte cap" "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority
  git -C "$bed_clone" config metasystem.goal.machine brain
  assert_refused "64-byte cap" "$ms" brain declare --root "$bed_clone" --by "$(printf 'b%.0s' {1..65})" --fixture-human-authority
  assert_refused "control characters" "$ms" brain declare --root "$bed_clone" --by $'Wi\ndo' --fixture-human-authority
  release_current
  git -C "$bed_clone" config metasystem.goal.machine "$(printf 'm%.0s' {1..32})"
  "$ms" brain declare --root "$bed_clone" --by "$(printf 'b%.0s' {1..64})" --fixture-human-authority >/dev/null
  boot=$("$ms" brain boot --root "$bed_clone" --repo "$bed_clone" --bytes 10000 --deadline-ms 5000)
  payload=$("$ms" json get --value "$boot" --field payload)
  header=${payload%%$'\n'*}
  header_bytes=$(LC_ALL=C printf '%s' "$header" | wc -c | tr -d ' ')
  (( header_bytes <= 320 )) || { echo "maximal declaration produced a $header_bytes-byte header" >&2; exit 1; }
fi

declare_boot_bed() {
  local label=$1
  setup_ledger "$label" brain
  registry=$tmp/registry
  mkdir -p "$registry"
  export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  release_current
  "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority >/dev/null
}

assert_boot_payload_bound() { # object, bound
  local object=$1 bound=$2 bytes payload sentinel
  bytes=$("$ms" json get --value "$object" --field bytes)
  payload_sentinel=$("$ms" json get --value "$object" --field payload; printf x)
  payload=${payload_sentinel%x}
  payload=${payload%$'\n'}
  [[ "$bytes" =~ ^[0-9]+$ && "$bytes" -le "$bound" ]] || { echo "boot reported $bytes bytes against bound $bound" >&2; exit 1; }
  [[ $(printf '%s' "$payload" | wc -c | tr -d ' ') -eq "$bytes" ]] || { echo "boot byte count does not match payload" >&2; exit 1; }
}

if [[ "$fixture_scenario" == brain-boot-bound ]]; then
  declare_boot_bed bound
  mkdir -p "$bed_clone/artifacts/agents/channel/questions" "$bed_clone/artifacts/agents/jobs" "$bed_clone/artifacts/agents/supervision"
  wants=$(printf 'w%.0s' {1..2000})
  for index in $(seq 1 60); do
    printf '{"id":"ask-%02d","goal":"goal-%02d","kind":"other","machine":"node","openedAt":"2026-09-07T00:%02d:00Z","wants":"%s","state":"open"}\n' \
      "$index" "$index" "$((index % 60))" "$wants" >"$bed_clone/artifacts/agents/channel/questions/ask-$index.json"
  done
  for index in $(seq 1 80); do
    printf '{"jobId":"job-%02d","status":"running","role":"implementer","goalId":"ship-widget"}\n' "$index" \
      >"$bed_clone/artifacts/agents/jobs/job-$index.json"
  done
  now_epoch=$(date -u +%s)
  printf '{"verdict":"SUCCESS","completedAtEpoch":%s,"counts":{"CUSTODY":2,"ANNOUNCED":1,"UNTRACKED":0}}\n' "$now_epoch" \
    >"$bed_clone/artifacts/agents/supervision/last-census.json"
  for index in $(seq 1 400); do
    printf '2026-09-07T00:00:00Z HIGHLIGHT — digest line %03d (source: fixture bound)\n' "$index"
  done >"$bed_clone/records/narrator-digest.log"
  boot=$("$ms" brain boot --root "$bed_clone" --repo "$bed_clone" --bytes 10000 --deadline-ms 5000)
  assert_boot_payload_bound "$boot" 10000
  [[ "$("$ms" json get --value "$boot" --field sections.fleet)" == cut ]] || { echo "bounded boot did not cut fleet" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.digest)" == cut ]] || { echo "bounded boot did not cut digest" >&2; exit 1; }
  payload=$("$ms" json get --value "$boot" --field payload)
  while IFS= read -r line; do
    [[ "$line" != ask-* ]] || {
      line_bytes=$(printf '%s' "$line" | wc -c | tr -d ' ')
      (( line_bytes <= 160 )) || { echo "ask line is $line_bytes bytes" >&2; exit 1; }
      [[ "$line" == *'…' ]] || { echo "long ask line lacks ellipsis" >&2; exit 1; }
    }
  done <<<"$payload"
  packet_sentinel=$(cat "$bed_clone/records/misc/fleet-coordinator-brain-role-packet.md"; printf x)
  packet_bytes=${packet_sentinel%x}
  [[ "$payload" == *"$packet_bytes"* ]] || { echo "full packet bytes were not intact in 10000-byte boot" >&2; exit 1; }

  minimum=$("$ms" brain boot --root "$bed_clone" --repo "$bed_clone" --bytes 2048 --deadline-ms 5000)
  assert_boot_payload_bound "$minimum" 2048
  minimum_payload=$("$ms" json get --value "$minimum" --field payload)
  grep -Fq 'PACKET TOO LARGE FOR THIS CHANNEL' <<<"$minimum_payload" || { echo "minimum boot omitted packet-size notice" >&2; exit 1; }
  grep -Fq '## The standing instruction' <<<"$minimum_payload" || { echo "minimum boot omitted standing-instruction section" >&2; exit 1; }
  assert_refused "below the minimum 2048" "$ms" brain boot --root "$bed_clone" --repo "$bed_clone" --bytes 2047 --deadline-ms 5000
fi

if [[ "$fixture_scenario" == brain-boot-errors ]]; then
  declare_boot_bed errors
  mkdir -p "$bed_clone/artifacts/agents/steward" "$bed_clone/artifacts/agents/channel/questions" "$bed_clone/artifacts/agents/jobs"
  printf '%s\n' '{broken' >"$bed_clone/artifacts/agents/steward/narrator-digest-brain-cursor.json"
  printf '%s\n' '{broken' >"$bed_clone/artifacts/agents/channel/questions/broken.json"
  printf '%s\n' '{broken' >"$bed_clone/artifacts/agents/jobs/broken.json"
  cursor_before=$(shasum -a 256 "$bed_clone/artifacts/agents/steward/narrator-digest-brain-cursor.json" | cut -d' ' -f1)
  boot=$("$ms" brain boot --root "$bed_clone" --repo "$bed_clone" --bytes 10000 --deadline-ms 5000)
  payload=$("$ms" json get --value "$boot" --field payload)
  grep -Fq 'unreadable question files' <<<"$payload" || { echo "question error absent from boot" >&2; exit 1; }
  grep -Fq 'unreadable job records' <<<"$payload" || { echo "job error absent from boot" >&2; exit 1; }
  grep -Fq 'CENSUS absent or unreadable' <<<"$payload" || { echo "census error absent from boot" >&2; exit 1; }
  grep -Fq 'DIGEST unreadable' <<<"$payload" || { echo "digest error absent from boot" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.asks)" == error ]] || { echo "asks did not mark error" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.held)" == complete ]] || { echo "held did not complete" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.fleet)" == error ]] || { echo "fleet did not mark error" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.digest)" == error ]] || { echo "digest did not mark error" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field digestEmitted)" == false ]] || { echo "unreadable digest was marked emitted" >&2; exit 1; }
  [[ "$cursor_before" == "$(shasum -a 256 "$bed_clone/artifacts/agents/steward/narrator-digest-brain-cursor.json" | cut -d' ' -f1)" ]] || { echo "error boot changed brain cursor" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == brain-boot-stalled ]]; then
  declare_boot_bed stalled
  mkdir -p "$bed_clone/artifacts/agents/channel/questions" "$bed_clone/artifacts/agents/supervision"
  printf '%s\n' '{"id":"kept-ask","goal":"ship-widget","kind":"other","machine":"node","openedAt":"2026-09-07T00:00:00Z","wants":"kept before the stall","state":"open"}' >"$bed_clone/artifacts/agents/channel/questions/kept-ask.json"
  printf '%s\n' '{"verdict":"SUCCESS","completedAtEpoch":1788739200,"counts":{"CUSTODY":1,"ANNOUNCED":0,"UNTRACKED":0}}' >"$bed_clone/artifacts/agents/supervision/last-census.json"
  fifo="$bed_clone/records/narrator-digest.log"
  mkfifo "$fifo"
  started=$SECONDS
  boot=$("$ms" brain boot --root "$bed_clone" --repo "$bed_clone" --bytes 10000 --deadline-ms 1500)
  elapsed=$((SECONDS - started))
  (( elapsed < 3 )) || { echo "stalled boot took ${elapsed}s" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.asks)" == complete ]] || { echo "completed asks section was not kept" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.held)" == complete ]] || { echo "completed held section was not kept" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.fleet)" == complete ]] || { echo "completed fleet section was not kept" >&2; exit 1; }
  [[ "$("$ms" json get --value "$boot" --field sections.digest)" == skipped ]] || { echo "stalled digest section was not skipped" >&2; exit 1; }
  payload=$("$ms" json get --value "$boot" --field payload)
  grep -Fq 'kept-ask goal ship-widget' <<<"$payload" || { echo "deadline boot discarded the completed asks payload" >&2; exit 1; }
  grep -Fq 'BOOT DEADLINE: digest not read' <<<"$payload" || { echo "deadline did not name only the stalled digest" >&2; exit 1; }
  [[ "$payload" != *'BOOT DEADLINE: asks'* && "$payload" != *'BOOT DEADLINE: held'* && "$payload" != *'BOOT DEADLINE: fleet'* ]] || { echo "deadline named a completed section" >&2; exit 1; }
  if ! ps -axo command= >"$tmp/processes.after-stalled"; then
    echo "brain-boot-stalled could not verify child-process cleanup because process inspection is unavailable" >&2
    exit 1
  fi
  if awk -v needle="brain boot-inputs --root $bed_clone" 'index($0, needle) { found=1 } END { exit found ? 0 : 1 }' "$tmp/processes.after-stalled"; then
    echo "stalled boot left its boot-inputs process alive" >&2
    exit 1
  fi
fi

if [[ "$fixture_scenario" == brain-declare-quiescence ]]; then
  setup_ledger quiescence brain
  registry=$tmp/registry; mkdir -p "$registry"; export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  assert_refused "goal release" "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority
  release_current
  mkdir -p "$bed_clone/artifacts/agents/jobs"
  printf '%s\n' '{"jobId":"pending-job","status":"pending","role":"implementer","runtime":"fake"}' >"$bed_clone/artifacts/agents/jobs/pending-job.json"
  assert_refused "delegate --cancel pending-job" "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority
  rm "$bed_clone/artifacts/agents/jobs/pending-job.json"
  printf '%s\n' '{"jobId":"pending-setup-job","status":"pending-setup","role":"implementer","runtime":"fake"}' >"$bed_clone/artifacts/agents/jobs/pending-setup-job.json"
  assert_refused "delegate --cancel pending-setup-job" "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority
  rm "$bed_clone/artifacts/agents/jobs/pending-setup-job.json"
  mkdir -p "$bed_clone/artifacts/agents/runs"
  printf '%s\n' '{"schemaVersion":1,"runId":"live-run","kind":"custom","display":"fixture","custody":"wrapped","generation":1,"launchNonce":"0123456789abcdef0123456789abcdef","log":"","startedAt":"2026-09-07T00:00:00Z","sessionId":"fixture","goalId":"","staleAfterMin":10,"windDownMin":1,"evidence":{"mode":"none"},"expect":{"green":"","red":"","hung":"","unknown":""},"status":"launching","acked":false}' >"$bed_clone/artifacts/agents/runs/live-run.json"
  assert_refused "run watch" "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority
  rm "$bed_clone/artifacts/agents/runs/live-run.json"
  mkdir -p "$bed_clone/artifacts/agents/missions/runners"
  fixture_start=$("$ms" proc started-at --pid "$$")
  printf '{"missionId":"fixture-mission","status":"running","pid":%s,"pidStartedAt":%s}\n' "$$" "$fixture_start" >"$bed_clone/artifacts/agents/missions/runners/fixture-mission.json"
  assert_refused "mission status" "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority
  rm "$bed_clone/artifacts/agents/missions/runners/fixture-mission.json"
  "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority >/dev/null
fi

if [[ "$fixture_scenario" == brain-declare-race ]]; then
  setup_ledger race brain-one; release_current
  clone_two=$tmp/race-two; clone_same_ledger "$clone_two" brain-two
  registry=$tmp/registry; mkdir -p "$registry"; export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  set +e
  "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority >"$tmp/race-one.out" 2>&1 & p1=$!
  "$ms" brain declare --root "$clone_two" --by Wido --fixture-human-authority >"$tmp/race-two.out" 2>&1 & p2=$!
  wait "$p1"; r1=$?; wait "$p2"; r2=$?
  set -e
  [[ "$r1,$r2" == "0,2" || "$r1,$r2" == "2,0" ]] || { echo "brain declare race exits were $r1,$r2" >&2; exit 1; }
  loser_out=$tmp/race-two.out; winner=$bed_clone
  [[ $r1 -eq 0 ]] || { loser_out=$tmp/race-one.out; winner=$clone_two; }
  grep -q "already has a brain" "$loser_out" || { echo "race loser did not name the winner" >&2; exit 1; }
  pointer=$registry/.metasystem/brain/$bed_identity
  [[ "$(cat "$pointer")" == "$(cd "$winner" && pwd -P)" ]] || { echo "race pointer does not name winner" >&2; exit 1; }
  "$ms" brain withdraw --root "$winner" --by Wido --fixture-human-authority >/dev/null
  mkdir -p "$pointer.lock.d"
  fixture_start=$("$ms" proc started-at --pid "$$")
  printf '{"pid":%s,"pidStartedAt":%s,"instanceTag":"foreign"}\n' "$$" "$fixture_start" >"$pointer.lock.d/owner.json"
  assert_refused "another brain declaration is in progress" "$ms" brain declare --root "$clone_two" --by Wido --fixture-human-authority
fi

if [[ "$fixture_scenario" == brain-second-declaration-refuses ]]; then
  setup_ledger primary brain-one; release_current
  primary_clone=$bed_clone; primary_identity=$bed_identity
  second=$tmp/primary-two; clone_same_ledger "$second" brain-two
  home_one=$tmp/home-one; home_two=$tmp/home-two; mkdir -p "$home_one" "$home_two"
  METASYSTEM_SUPERVISION_REGISTRY_HOME=$home_one "$ms" brain declare --root "$primary_clone" --by Wido --fixture-human-authority >/dev/null
  assert_refused "already the brain" env METASYSTEM_SUPERVISION_REGISTRY_HOME="$home_one" "$ms" brain declare --root "$primary_clone" --by Wido --fixture-human-authority
  primary_canonical=$(cd "$primary_clone" && pwd -P)
  assert_refused "$primary_canonical" env METASYSTEM_SUPERVISION_REGISTRY_HOME="$home_one" "$ms" brain declare --root "$second" --by Wido --fixture-human-authority
  METASYSTEM_SUPERVISION_REGISTRY_HOME=$home_two "$ms" brain declare --root "$second" --by Wido --fixture-human-authority >/dev/null
  setup_ledger other other-brain; release_current
  METASYSTEM_SUPERVISION_REGISTRY_HOME=$home_one "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority >/dev/null
  METASYSTEM_SUPERVISION_REGISTRY_HOME=$home_two "$ms" brain withdraw --root "$second" --by Wido --fixture-human-authority >/dev/null
  METASYSTEM_SUPERVISION_REGISTRY_HOME=$home_one "$ms" brain withdraw --root "$primary_clone" --by Wido --fixture-human-authority >/dev/null
  METASYSTEM_SUPERVISION_REGISTRY_HOME=$home_one "$ms" brain declare --root "$second" --by Wido --fixture-human-authority >/dev/null
  [[ -f "$home_one/.metasystem/brain/$primary_identity" ]] || { echo "second checkout did not replace the withdrawn host pointer" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == brain-verbs-human-only ]]; then
  setup_ledger human-only brain; release_current
  registry=$tmp/registry; mkdir -p "$registry"; export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  agent=$tmp/metasystem-fake-agent
  cat >"$agent" <<'SH'
#!/usr/bin/env bash
"${BRAIN_FIXTURE_ENGINE:?}" brain "$@"
SH
  chmod +x "$agent"
  assert_refused "human act" env BRAIN_FIXTURE_ENGINE="$ms" "$agent" declare --root "$bed_clone" --by Wido
  "$ms" brain declare --root "$bed_clone" --by Wido --fixture-human-authority >/dev/null
  assert_refused "human act" env BRAIN_FIXTURE_ENGINE="$ms" "$agent" withdraw --root "$bed_clone" --by Wido
  printf '%s\n' '{broken' >"$bed_clone/artifacts/agents/brain.json"
  "$ms" brain withdraw --root "$bed_clone" --by Wido --fixture-human-authority >/dev/null
  [[ ! -e "$registry/.metasystem/brain/$bed_identity" ]] || { echo "withdraw left the host pointer" >&2; exit 1; }
  no_ledger=$tmp/no-ledger
  mkdir -p "$no_ledger/artifacts/agents" "$registry/.metasystem/brain"
  printf '%s\n' 'metasystem.runtimes=fake' >"$no_ledger/metasystem.conf"
  printf '%s\n' '{broken' >"$no_ledger/artifacts/agents/brain.json"
  printf '%s\n' untouched >"$registry/.metasystem/brain/keep"
  empty_withdraw=$("$ms" brain withdraw --root "$no_ledger" --by Wido --fixture-human-authority)
  [[ "$empty_withdraw" == *'"state":"undeclared"'* && ! -e "$no_ledger/artifacts/agents/brain.json" ]] || { echo "empty-ledger withdraw did not remove and report the record: $empty_withdraw" >&2; exit 1; }
  [[ "$(cat "$registry/.metasystem/brain/keep")" == untouched && ! -e "$registry/.metasystem/brain.lock.d" ]] || { echo "empty-ledger withdraw touched the pointer directory" >&2; exit 1; }
fi

if [[ "$fixture_scenario" == brain-actor-seam-coverage ]]; then
	actor_sites=$(cd "$root" && find cmd/metasystem internal -type f -name '*.go' ! -name '*_test.go' \
		-exec grep -H -E 'goal\.Actor\{|Actor\.Human[[:space:]]*=[^=]|classifyVerbCaller\(|lease\.ClassifyVerbAt\(e\.Root' {} + \
		| grep -v '^cmd/metasystem/goalsync_mutations.go:[[:space:]]*Endpoint: e, Actor:' \
		| LC_ALL=C sort)
	expected_actor_sites=$(cat <<'ACTOR_SITES'
cmd/metasystem/brain.go:	classification, err := classifyVerbCaller(root, int64(os.Getppid()))
cmd/metasystem/census.go:	view, err := classifyVerbCaller(*root, parent)
cmd/metasystem/dispatch_verbs.go:	caller, err := classifyVerbCaller(*root, int64(os.Getppid()))
cmd/metasystem/goal.go:			return goal.Actor{}, 0, fmt.Errorf("the Stop main %q does not match the announced checkout holder %q", mainID, holder.MainId)
cmd/metasystem/goal.go:			return goal.Actor{}, 0, fmt.Errorf("the announced checkout holder could not be resolved: %w", err)
cmd/metasystem/goal.go:			return goal.Actor{}, 0, fmt.Errorf("the checkout holder has no readable main announcement and lineage")
cmd/metasystem/goal.go:			return goal.Actor{}, 0, fmt.Errorf("the seat machine could not be resolved: %w", err)
cmd/metasystem/goal.go:		return goal.Actor{Machine: machine, Lineage: holder.OwnerLineage}, holder.ClaimEpoch, nil
cmd/metasystem/goal.go:	view, err := classifyVerbCaller(root, callerPid)
cmd/metasystem/goalsync_mutations.go:		classification, classErr := classifyVerbCaller(f.root, int64(os.Getppid()))
cmd/metasystem/goalsync_mutations.go:		req.Actor.Human = f.by
cmd/metasystem/goalsync_mutations.go:	classification, classifyErr := classifyVerbCaller(root, int64(os.Getppid()))
cmd/metasystem/goalsync_verbs.go:		return goal.Actor{}, err
cmd/metasystem/goalsync_verbs.go:	return goal.Actor{Machine: machine, Lineage: lineage, Human: human}, nil
cmd/metasystem/process_verbs.go:func classifyVerbCaller(root string, callerPid int64) (lease.ClassifyResult, error) {
cmd/metasystem/proof_run.go:	classification, err := classifyVerbCaller(root, int64(os.Getppid()))
cmd/metasystem/proof_run.go:	classifiedCaller, err := classifyVerbCaller(request.ControlRoot, int64(os.Getppid()))
cmd/metasystem/proof_run.go:	classifiedCaller, err = classifyVerbCaller(request.ControlRoot, int64(os.Getppid()))
cmd/metasystem/run.go:		view, err := classifyVerbCaller(root, int64(os.Getpid()))
cmd/metasystem/run.go:	view, err := classifyVerbCaller(root, callerPid)
internal/channel/poll.go:				approved, approveErr := goal.Approve(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage, Human: a.UserID}, Ulid: a.ApprovalULID, Now: a.At}, []string{q.Goal}, q.Budget, &proof)
internal/channel/poll.go:		published, err := goal.Answer(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage}, Ulid: a.ULID, Now: a.At}, q.Goal, q.ID, a.Text, wants, goal.AnswerProof{Provider: c.ProviderName, User: a.UserID, Ref: a.Ref.ThreadID + "/" + a.Ref.ID, Step: a.Step})
internal/channel/poll.go:		published, err := goal.Approve(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage, Human: "wido"}, Ulid: ulid, Now: c.Now}, []string{status.GoalID}, nil, &proof)
internal/channel/question.go:		published, e := goal.Asked(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: r.Machine, Lineage: r.Lineage}, Ulid: ulid, Now: r.Now}, r.Goal, id, r.Kind, r.Facts[0])
internal/dispatch/claim.go:		Endpoint: endpoint, Actor: goal.Actor{Machine: binding.Machine, Lineage: binding.Lineage},
internal/dispatch/finding_register.go:			req := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: lineage}, Ulid: deterministicULID(rootJob), Now: time.Now().UTC(), ClaimEpoch: epoch}
internal/dispatch/stop.go:				Actor:    goal.Actor{Machine: binding.Machine, Lineage: stopCustodianLineage},
internal/dispatch/stop.go:	actor := goal.Actor{Machine: binding.Machine, Lineage: stopCustodianLineage}
internal/goal/recover.go:		r.Actor.Human = by
internal/missionrunner/launch.go:	if view, err := lease.ClassifyVerbAt(e.Root, e.classifierInstallation(), int64(pid)); err == nil {
internal/steward/revive.go:		Actor: goal.Actor{
internal/steward/validation_window.go:	request := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage}, Ulid: ulid, Now: now}
ACTOR_SITES
)
	[[ "$actor_sites" == "$expected_actor_sites" ]] || {
		echo "brain actor seam allow-list changed; inspect every added Actor or caller-classification site" >&2
		printf '%s\n' "$actor_sites" >&2
		exit 1
	}
fi
