#!/usr/bin/env bash
set -euo pipefail

# Isolated witness boundary proofs. Each refusal tree has a broken gofmt, so
# accepting a witness accidentally returns green while the lawful full fallback
# fails at the proof the witness would have omitted.
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source_engine=$root/bin/metasystem
[[ -x "$source_engine" ]] \
  || { echo "witness-gate fixture: current source engine is absent; run the Go gate first" >&2; exit 1; }

tmp=$(mktemp -d)
foreign_controller=
cleanup() {
  if [[ -n "$foreign_controller" ]]; then
    kill "$foreign_controller" 2>/dev/null || true
    wait "$foreign_controller" 2>/dev/null || true
  fi
  rm -rf "$tmp" 2>/dev/null || true
}
trap cleanup EXIT

make_leg() { # name
  leg_tree=$tmp/$1/tree
  leg_bin=$tmp/$1/bin
  mkdir -p "$leg_tree/scripts/agents" "$leg_tree/internal/fixture" \
    "$leg_tree/cmd/metasystem" "$leg_tree/docs" "$leg_tree/bin" "$leg_bin"
  cp "$root/scripts/agents/go-gate.sh" "$leg_tree/scripts/agents/go-gate.sh"
  chmod +x "$leg_tree/scripts/agents/go-gate.sh"
  printf 'module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.24\n' >"$leg_tree/go.mod"
  printf 'package fixture\n' >"$leg_tree/internal/fixture/fixture.go"
  printf 'package main\n' >"$leg_tree/cmd/metasystem/main.go"
  printf 'payload baseline\n' >"$leg_tree/docs/payload.md"
  printf '#!/usr/bin/env bash\nset -euo pipefail\nprintf "%%s\\n" "$METASYSTEM_BUILD_STAMP" >"$WITNESS_FIXTURE_BUILD_MARKER"\n' \
    >"$leg_tree/scripts/agents/go-build.sh"
  chmod +x "$leg_tree/scripts/agents/go-build.sh"
  printf '#!/usr/bin/env bash\necho "fixture: gofmt proof is broken" >&2\nexit 79\n' >"$leg_bin/gofmt"
  chmod +x "$leg_bin/gofmt"
  cat >"$leg_bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  run)
    [[ ${2:-} == ./cmd/metasystem ]]
    shift 2
    exec "$WITNESS_FIXTURE_SOURCE_ENGINE" "$@" ;;
  version)
    printf 'go version go1.fixture darwin/arm64\n' ;;
  env)
    shift
    if [[ "$*" == 'GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN' ]]; then
      printf 'darwin\narm64\n\noff\n\n0\nauto\n'
    else
      echo "fixture go: unsupported env request: $*" >&2
      exit 86
    fi ;;
  list)
    printf '%s\n' "$PWD/internal/fixture" "$PWD/cmd/metasystem" ;;
  *) echo "fixture go: unsupported command: $*" >&2; exit 86 ;;
esac
GO
  chmod +x "$leg_bin/go"
  cat >"$leg_tree/bin/metasystem" <<'STALE'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}/${2:-}" in
  gate/register|gate/fence) exit 0 ;;
  behavior-surface/skip-allowed)
    echo 'stale binary policy denies every skip' >&2
    exit 91 ;;
  *) exit 0 ;;
esac
STALE
  chmod +x "$leg_tree/bin/metasystem"
}

fixture_toolchain_identity() { # tree, fixture bin
  (
    cd "$1"
    export PATH="$2:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine"
    { go version; go env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN; } \
      | { shasum -a 256 2>/dev/null || sha256sum; } | awk '{print $1}'
  )
}

write_witness() { # name, tree, fixture bin, controller pid
  local name=$1 tree=$2 fixture_bin=$3 controller_pid=$4
  witness_root=$tmp/$name/state
  witness_path=$witness_root/witness.json
  witness_run=run-$name
  mkdir -p "$witness_root"
  chmod 700 "$witness_root"
  "$source_engine" behavior-surface list --root "$tree" --projection PAYLOAD --nul \
    >"$witness_root/payload-paths.nul"
  chmod 600 "$witness_root/payload-paths.nul"
  local engine_report payload_report version engine_digest payload_digest toolchain
  engine_report=$("$source_engine" behavior-surface digest --root "$tree" \
    --projection ENGINE --endpoint "fixture producer $name")
  payload_report=$("$source_engine" behavior-surface digest --root "$tree" \
    --projection PAYLOAD --endpoint "fixture producer $name" \
    --paths-from "$witness_root/payload-paths.nul")
  version=$("$source_engine" json get --value "$engine_report" --field policyVersion)
  engine_digest=$("$source_engine" json get --value "$engine_report" --field surfaceDigest)
  payload_digest=$("$source_engine" json get --value "$payload_report" --field surfaceDigest)
  toolchain=$(fixture_toolchain_identity "$tree" "$fixture_bin")
  local started ticks boot
  read -r started ticks boot < <("$source_engine" proc started-at --pid "$controller_pid" --emit pair)
  [[ "$boot" == - ]] && boot=
  umask 077
  printf '{"policyVersion":%s,"engineDigest":"%s","payloadDigest":"%s","payloadManifest":"payload-paths.nul","toolchainIdentity":"%s","runId":"%s","controller":{"pid":%s,"startedAtSec":%s,"startTicks":%s,"bootId":"%s"}}\n' \
    "$version" "$engine_digest" "$payload_digest" "$toolchain" "$witness_run" \
    "$controller_pid" "$started" "$ticks" "$boot" >"$witness_path"
  chmod 600 "$witness_path"
}

run_engine_acceptance() { # name, tree, fixture bin, witness, state root, run id
  local name=$1 tree=$2 fixture_bin=$3 witness=$4 state_root=$5 run=$6
  local marker=$tmp/$name/build-marker output=$tmp/$name/accept.out
  (
    cd "$tree"
    env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
      WITNESS_FIXTURE_BUILD_MARKER="$marker" METASYSTEM_ALLOW_CONCURRENT_GATE=1 \
      METASYSTEM_GATE_WITNESS="$witness" METASYSTEM_GATE_WITNESS_ROOT="$state_root" \
      METASYSTEM_GATE_WITNESS_RUN="$run" METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
      bash scripts/agents/go-gate.sh
  ) >"$output" 2>&1 \
    || { echo "witness-gate fixture $name: matching ENGINE did not skip" >&2; sed -n '1,80p' "$output" >&2; exit 1; }
  [[ -f "$marker" ]] || { echo "witness-gate fixture $name: accepted witness did not rebuild" >&2; exit 1; }
  case $(cat "$marker") in
    witness-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
    *) echo "witness-gate fixture $name: rebuild did not carry the witness stamp" >&2; exit 1 ;;
  esac
  grep -Fq 'outer witness' "$output" \
    || { echo "witness-gate fixture $name: acceptance did not report witness reuse" >&2; exit 1; }
}

run_refusal() { # name, tree, fixture bin, witness, state root, run id, scope
  local name=$1 tree=$2 fixture_bin=$3 witness=$4 state_root=$5 run=$6 scope=$7
  local output=$tmp/$name/refusal.out rc=0
  local command=(env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine"
    WITNESS_FIXTURE_BUILD_MARKER="$tmp/$name/unexpected-build" METASYSTEM_ALLOW_CONCURRENT_GATE=1)
  [[ -z "$witness" ]] || command+=(METASYSTEM_GATE_WITNESS="$witness")
  [[ -z "$state_root" ]] || command+=(METASYSTEM_GATE_WITNESS_ROOT="$state_root")
  [[ -z "$run" ]] || command+=(METASYSTEM_GATE_WITNESS_RUN="$run")
  [[ -z "$scope" ]] || command+=(METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE="$scope")
  set +e
  (cd "$tree" && "${command[@]}" bash scripts/agents/go-gate.sh) >"$output" 2>&1
  rc=$?
  set -e
  [[ $rc != 0 ]] \
    || { echo "witness-gate fixture $name: refusal skipped the deliberately broken proof" >&2; exit 1; }
  grep -Fq 'gofmt itself failed' "$output" \
      || { echo "witness-gate fixture $name: refusal did not reach the broken full proof" >&2; sed -n '1,80p' "$output" >&2; exit 1; }
}

# 1. A live PID carrying a different start identity has no ancestry authority.
make_leg wrong-start-identity
write_witness wrong-start-identity "$leg_tree" "$leg_bin" $$
wrong_start_probe=$("$source_engine" proc probe --pid $$)
wrong_start_pid=$("$source_engine" json get --value "$wrong_start_probe" --field pid)
wrong_start_started=$("$source_engine" json get --value "$wrong_start_probe" --field startedAtUnix)
wrong_start_ticks=$("$source_engine" json get --value "$wrong_start_probe" --field startTicks)
wrong_start_fabricated_started=$((wrong_start_started + 1))
wrong_start_fabricated_ticks=$wrong_start_ticks
if (( wrong_start_fabricated_ticks > 0 )); then
  wrong_start_fabricated_ticks=$((wrong_start_fabricated_ticks + 1))
fi
sed -e "s/\"startedAtSec\":[0-9][0-9]*/\"startedAtSec\":$wrong_start_fabricated_started/" \
  -e "s/\"startTicks\":[0-9][0-9]*/\"startTicks\":$wrong_start_fabricated_ticks/" \
  "$witness_path" >"$witness_path.new"
mv "$witness_path.new" "$witness_path"; chmod 600 "$witness_path"
[[ "$wrong_start_pid" == $$ ]]
[[ "$("$source_engine" json get --file "$witness_path" --field controller.pid)" == $$ ]]
[[ "$("$source_engine" json get --file "$witness_path" --field controller.startedAtSec)" != "$wrong_start_started" ]]
if (( wrong_start_ticks > 0 )); then
  [[ "$("$source_engine" json get --file "$witness_path" --field controller.startTicks)" != "$wrong_start_ticks" ]]
fi
run_refusal wrong-start-identity "$leg_tree" "$leg_bin" \
  "$witness_path" "$witness_root" "$witness_run" ENGINE
if [[ "${WITNESS_WRONG_START_IDENTITY_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "witness-gate wrong-start-identity fixture passed"
  exit 0
fi

# 2. ENGINE equality survives a PAYLOAD-only change; DELIVERY equality does not.
make_leg changed-payload
write_witness changed-payload "$leg_tree" "$leg_bin" $$
changed_payload_tree=$leg_tree changed_payload_bin=$leg_bin
changed_payload_witness=$witness_path changed_payload_root=$witness_root changed_payload_run=$witness_run
printf 'payload changed after ENGINE proof\n' >"$changed_payload_tree/docs/payload.md"
run_engine_acceptance changed-payload "$changed_payload_tree" "$changed_payload_bin" \
  "$changed_payload_witness" "$changed_payload_root" "$changed_payload_run"
set +e
(
  cd "$changed_payload_tree"
  env PATH="$changed_payload_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
    METASYSTEM_GATE_WITNESS="$changed_payload_witness" METASYSTEM_GATE_WITNESS_ROOT="$changed_payload_root" \
    METASYSTEM_GATE_WITNESS_RUN="$changed_payload_run" METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=DELIVERY \
    bash scripts/agents/go-gate.sh --witness-check-only
) >"$tmp/changed-payload/delivery.out" 2>&1
changed_payload_delivery_rc=$?
set -e
[[ $changed_payload_delivery_rc == 3 ]] \
  || { echo "witness-gate fixture: changed PAYLOAD authorized DELIVERY reuse" >&2; exit 1; }
run_refusal changed-payload "$changed_payload_tree" "$changed_payload_bin" \
  "$changed_payload_witness" "$changed_payload_root" "$changed_payload_run" DELIVERY

# 3. Changed ENGINE runs the full proof and finds the defect.
make_leg changed-engine
write_witness changed-engine "$leg_tree" "$leg_bin" $$
printf 'package fixture\nvar Changed = true\n' >"$leg_tree/internal/fixture/fixture.go"
run_refusal changed-engine "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" "$witness_run" ENGINE

# 4. A live but foreign controller is correlation without ancestry authority.
sleep 60 & foreign_controller=$!
make_leg foreign-ancestry
write_witness foreign-ancestry "$leg_tree" "$leg_bin" "$foreign_controller"
run_refusal foreign-ancestry "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" "$witness_run" ENGINE
kill "$foreign_controller" 2>/dev/null || true
wait "$foreign_controller" 2>/dev/null || true
foreign_controller=

# 5. A consumer that does not state ENGINE or DELIVERY cannot borrow either.
make_leg absent-scope
write_witness absent-scope "$leg_tree" "$leg_bin" $$
run_refusal absent-scope "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" "$witness_run" ""

# 6. Policy version is an independent equality field.
make_leg policy-version
write_witness policy-version "$leg_tree" "$leg_bin" $$
sed 's/"policyVersion":[0-9][0-9]*/"policyVersion":999/' "$witness_path" >"$witness_path.new"
mv "$witness_path.new" "$witness_path"; chmod 600 "$witness_path"
run_refusal policy-version "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" "$witness_run" ENGINE

# 7. Skip authority comes from the consuming source engine, not the stale bin.
make_leg prospective-policy
write_witness prospective-policy "$leg_tree" "$leg_bin" $$
run_engine_acceptance prospective-policy "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" "$witness_run"

# 8. Toolchain identity is independent of byte equality.
make_leg toolchain-mismatch
write_witness toolchain-mismatch "$leg_tree" "$leg_bin" $$
sed 's/"toolchainIdentity":"[^"]*"/"toolchainIdentity":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"/' \
  "$witness_path" >"$witness_path.new"
mv "$witness_path.new" "$witness_path"; chmod 600 "$witness_path"
run_refusal toolchain-mismatch "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" "$witness_run" ENGINE

# 9. Run id is correlation owned by the live controller environment.
make_leg foreign-run
write_witness foreign-run "$leg_tree" "$leg_bin" $$
run_refusal foreign-run "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" other-run ENGINE

# 10. State-root containment is owned by the live controller environment.
make_leg foreign-root
write_witness foreign-root "$leg_tree" "$leg_bin" $$
foreign_root=$tmp/foreign-root/other-state
mkdir -p "$foreign_root"; chmod 700 "$foreign_root"
run_refusal foreign-root "$leg_tree" "$leg_bin" "$witness_path" "$foreign_root" "$witness_run" ENGINE

# 11. Deleting runtime acceleration state falls back to the complete proof.
make_leg deleted-witness
write_witness deleted-witness "$leg_tree" "$leg_bin" $$
rm "$witness_path"
run_refusal deleted-witness "$leg_tree" "$leg_bin" "$witness_path" "$witness_root" "$witness_run" ENGINE

# 12. With no witness at all, the broken Go proof is still a hard failure.
make_leg broken-full-fallback
run_refusal broken-full-fallback "$leg_tree" "$leg_bin" "" "" "" ""

# 13. Only FULL publishes reset provenance; WITNESS-ASSISTED abandons intact.
assisted_root=$tmp/weight-assisted
assisted_envelope=$tmp/weight-assisted-envelope
mkdir -p "$assisted_envelope"
printf '1\t0\tdocs/a.md\0' | "$source_engine" gate weight-add \
  --root "$assisted_root" --commit assisted >/dev/null
"$source_engine" gate weight-checkpoint --root "$assisted_root" --run-id assisted \
  --subject assisted --runner-pid $$ --envelope "$assisted_envelope" >/dev/null
set +e
"$source_engine" gate weight-reset --root "$assisted_root" --run-id assisted \
  --run-class WITNESS-ASSISTED >"$tmp/assisted-reset.out" 2>&1
assisted_reset_rc=$?
set -e
[[ $assisted_reset_rc == 3 && ! -e "$assisted_envelope/reset.json" ]]
"$source_engine" gate weight-abandon --root "$assisted_root" --run-id assisted \
  --reason witness-assisted >/dev/null
[[ -f "$assisted_envelope/abandoned.json" && ! -e "$assisted_envelope/reset.json" ]]
[[ "$("$source_engine" json get --file "$assisted_root/artifacts/agents/battery-weight.json" --field accumulated)" == 1 ]]

full_root=$tmp/weight-full
full_envelope=$tmp/weight-full-envelope
mkdir -p "$full_envelope"
printf '1\t0\tdocs/a.md\0' | "$source_engine" gate weight-add \
  --root "$full_root" --commit full >/dev/null
"$source_engine" gate weight-checkpoint --root "$full_root" --run-id full \
  --subject full --runner-pid $$ --envelope "$full_envelope" >/dev/null
"$source_engine" gate weight-reset --root "$full_root" --run-id full --run-class FULL >/dev/null
[[ -f "$full_envelope/reset.json" ]]
[[ "$("$source_engine" json get --file "$full_envelope/reset.json" --field runClass)" == FULL ]]
[[ "$("$source_engine" json get --file "$full_root/artifacts/agents/battery-weight.json" --field accumulated)" == 0 ]]

echo "witness-gate fixtures passed (13 isolated legs)"
