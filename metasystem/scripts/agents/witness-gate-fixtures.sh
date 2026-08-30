#!/usr/bin/env bash
set -euo pipefail

# Isolated witness boundary proofs. Each refusal tree has a broken gofmt, so
# accepting a witness accidentally returns green while the lawful full fallback
# fails at the proof the witness would have omitted.
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source_engine=$root/bin/metasystem
[[ -x "$source_engine" ]] \
  || { echo "witness-gate fixture: current source engine is absent; run the Go gate first" >&2; exit 1; }

source "$root/scripts/agents/fixture-budget.sh"
source "$root/scripts/agents/fixture-bed-scenarios.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario witness-gate "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO
if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios witness-gate "witness-gate fixtures passed (21 isolated legs)" \
    "$fixture_bed_script" authority-and-scope equality-and-weights frozen-consumers cost-banner
fi

tmp=$(mktemp -d)
foreign_controller=
mid_freeze_pid=
cleanup() {
  if [[ -n "$foreign_controller" ]]; then
    kill "$foreign_controller" 2>/dev/null || true
    wait "$foreign_controller" 2>/dev/null || true
  fi
  if [[ -n "$mid_freeze_pid" ]]; then
    kill "$mid_freeze_pid" 2>/dev/null || true
    wait "$mid_freeze_pid" 2>/dev/null || true
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
  printf '#!/usr/bin/env bash\nset -euo pipefail\n[[ "${GOFLAGS:-}" == -mod=readonly ]]\n[[ -z "${WITNESS_FIXTURE_EXPECT_GOMODCACHE:-}" || "${GOMODCACHE:-}" == "$WITNESS_FIXTURE_EXPECT_GOMODCACHE" ]]\nprintf "%%s\\n" "$METASYSTEM_BUILD_STAMP" >"$WITNESS_FIXTURE_BUILD_MARKER"\nif [[ -n "${WITNESS_FIXTURE_BUILD_ROOT_MARKER:-}" ]]; then printf "%%s\\n" "$PWD" >"$WITNESS_FIXTURE_BUILD_ROOT_MARKER"; fi\nif [[ -n "${WITNESS_FIXTURE_MUTATE_AFTER_CHECK:-}" ]]; then printf "mutated during build\\n" >>"$WITNESS_FIXTURE_MUTATE_AFTER_CHECK"; fi\nmkdir -p bin\ncp "$WITNESS_FIXTURE_SOURCE_ENGINE" bin/metasystem\nchmod +x bin/metasystem\n' \
    >"$leg_tree/scripts/agents/go-build.sh"
  chmod +x "$leg_tree/scripts/agents/go-build.sh"
  printf '#!/usr/bin/env bash\nif [[ -n "${WITNESS_FIXTURE_GOFMT_ROOT_MARKER:-}" ]]; then printf "%%s\\n" "$PWD" >"$WITNESS_FIXTURE_GOFMT_ROOT_MARKER"; fi\necho "fixture: gofmt proof is broken" >&2\nexit 79\n' >"$leg_bin/gofmt"
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

add_manifest_digest() { # witness path, digest
  local path=$1 digest=$2
  sed "s/\"engineDigest\":/\"manifestDigest\":\"$digest\",\"engineDigest\":/" \
    "$path" >"$path.new"
  mv "$path.new" "$path"
  chmod 600 "$path"
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
  local marker=$tmp/$name/build-marker build_root_marker=$tmp/$name/build-root-marker output=$tmp/$name/accept.out
  (
    cd "$tree"
    env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
      GOMODCACHE="$tmp/shared-module-cache" WITNESS_FIXTURE_EXPECT_GOMODCACHE="$tmp/shared-module-cache" \
      WITNESS_FIXTURE_BUILD_MARKER="$marker" METASYSTEM_ALLOW_CONCURRENT_GATE=1 \
      WITNESS_FIXTURE_BUILD_ROOT_MARKER="$build_root_marker" \
      METASYSTEM_GATE_WITNESS="$witness" METASYSTEM_GATE_WITNESS_ROOT="$state_root" \
      METASYSTEM_GATE_WITNESS_RUN="$run" METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
      bash scripts/agents/go-gate.sh
  ) >"$output" 2>&1 \
    || { echo "witness-gate fixture $name: matching ENGINE did not skip" >&2; sed -n '1,80p' "$output" >&2; exit 1; }
  [[ -f "$marker" ]] || { echo "witness-gate fixture $name: accepted witness did not rebuild" >&2; exit 1; }
  [[ -f "$build_root_marker" ]] || { echo "witness-gate fixture $name: accepted witness did not report its build root" >&2; exit 1; }
  [[ "$(cat "$build_root_marker")" != "$tree" ]] \
    || { echo "witness-gate fixture $name: accepted witness built from the live consumer" >&2; exit 1; }
  case $(cat "$build_root_marker") in
    */metasystem-witness-freeze-*/tree) ;;
    *) echo "witness-gate fixture $name: accepted witness built outside a private frozen export" >&2; exit 1 ;;
  esac
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

run_recheck_refusal() { # name, tree, fixture bin, witness, state root, run id, mutation path
  local name=$1 tree=$2 fixture_bin=$3 witness=$4 state_root=$5 run=$6 mutation=$7
  local output=$tmp/$name/recheck.out rc=0
  set +e
  (
    cd "$tree"
    env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
      WITNESS_FIXTURE_BUILD_MARKER="$tmp/$name/build-marker" \
      WITNESS_FIXTURE_MUTATE_AFTER_CHECK="$mutation" METASYSTEM_ALLOW_CONCURRENT_GATE=1 \
      METASYSTEM_GATE_WITNESS="$witness" METASYSTEM_GATE_WITNESS_ROOT="$state_root" \
      METASYSTEM_GATE_WITNESS_RUN="$run" METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
      bash scripts/agents/go-gate.sh
  ) >"$output" 2>&1
  rc=$?
  set -e
  [[ $rc != 0 ]] \
    || { echo "witness-gate fixture $name: a post-check mutation reused the witness" >&2; exit 1; }
  grep -Fq 'witness changed during the skip-path build; running the full gate' "$output" \
    || { echo "witness-gate fixture $name: post-build mismatch was not reported" >&2; sed -n '1,100p' "$output" >&2; exit 1; }
  grep -Fq 'gofmt itself failed' "$output" \
    || { echo "witness-gate fixture $name: post-build mismatch did not reach the full gate" >&2; sed -n '1,100p' "$output" >&2; exit 1; }
}

if [[ "$fixture_scenario" == authority-and-scope ]]; then
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

fi

if [[ "$fixture_scenario" == equality-and-weights ]]; then
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

# 14. A dirty source arms from a private frozen export instead of falling
# back to the live-tree gate.
make_leg dirty-tree-arms
cp "$root/scripts/agents/witness-gate.sh" "$leg_tree/scripts/agents/witness-gate.sh"
cat >"$leg_tree/scripts/agents/go-gate.sh" <<'GATE'
#!/usr/bin/env bash
set -euo pipefail
if [[ -n "${METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST:-}" ]]; then
  [[ "${GOFLAGS:-}" == -mod=readonly ]]
  [[ "${GOMODCACHE:-}" == "${WITNESS_FIXTURE_EXPECT_GOMODCACHE:-}" ]]
fi
mkdir -p bin
cp "$WITNESS_FIXTURE_SOURCE_ENGINE" bin/metasystem
chmod +x bin/metasystem
if [[ -n "${METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST:-}" ]]; then
  printf '{"manifestDigest":"%s","summary":"full gate in frozen proof tree"}\n' \
    "$METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST" >"$METASYSTEM_GATE_WITNESS_WRITE"
else
  printf '{"summary":"full gate in HEAD snapshot"}\n' >"$METASYSTEM_GATE_WITNESS_WRITE"
fi
chmod 600 "$METASYSTEM_GATE_WITNESS_WRITE"
GATE
chmod +x "$leg_tree/scripts/agents/go-gate.sh" "$leg_tree/scripts/agents/witness-gate.sh"
git -C "$leg_tree" init -q -b main
git -C "$leg_tree" config user.name fixture
git -C "$leg_tree" config user.email fixture@example.invalid
git -C "$leg_tree" add .
git -C "$leg_tree" commit -qm baseline
(
  cd "$leg_tree"
  export PATH="$leg_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine"
  root=$(pwd -P)
  cd "$root"
  delivery_contract=0
  WITNESS_GATE_FALLBACK=none source scripts/agents/witness-gate.sh
  [[ -z "${METASYSTEM_GATE_WITNESS_EXPORT:-}" ]]
  ! grep -Fq 'manifestDigest' "$METASYSTEM_GATE_WITNESS"
  grep -Fq '"summary":"full gate in HEAD snapshot"' "$METASYSTEM_GATE_WITNESS"
  rm -rf "$witness_state"
) >"$tmp/dirty-tree-arms/clean.out" 2>&1 \
  || { echo "witness-gate fixture dirty-tree-arms: clean tree did not retain the HEAD snapshot path" >&2; sed -n '1,120p' "$tmp/dirty-tree-arms/clean.out" >&2; exit 1; }
printf 'package fixture\nvar Dirty = true\n' >"$leg_tree/internal/fixture/fixture.go"
(
  cd "$leg_tree"
  export PATH="$leg_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine"
  export GOMODCACHE="$tmp/dirty-tree-arms/shared-module-cache"
  export WITNESS_FIXTURE_EXPECT_GOMODCACHE="$GOMODCACHE"
  root=$(pwd -P)
  cd "$root"
  delivery_contract=0
  WITNESS_GATE_FALLBACK=none source scripts/agents/witness-gate.sh
  [[ -n "${METASYSTEM_GATE_WITNESS_EXPORT:-}" && -d "$METASYSTEM_GATE_WITNESS_EXPORT" ]]
  grep -Fq 'var Dirty = true' "$METASYSTEM_GATE_WITNESS_EXPORT/internal/fixture/fixture.go"
  grep -Eq '"manifestDigest":"[a-f0-9]{64}"' "$METASYSTEM_GATE_WITNESS"
  rm -rf "$witness_state"
) >"$tmp/dirty-tree-arms/arming.out" 2>&1 \
  || { echo "witness-gate fixture dirty-tree-arms: dirty tree did not arm from a frozen export" >&2; sed -n '1,120p' "$tmp/dirty-tree-arms/arming.out" >&2; exit 1; }
grep -Fq 'gate witness armed from frozen dirty export' "$tmp/dirty-tree-arms/arming.out" \
  || { echo "witness-gate fixture dirty-tree-arms: frozen arming was not reported" >&2; exit 1; }
set +e
(
  cd "$leg_tree"
  export PATH="$leg_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine"
  root=$(pwd -P)
  cd "$root"
  delivery_contract=0
  GOFLAGS='-overlay=outside.json' WITNESS_GATE_FALLBACK=none source scripts/agents/witness-gate.sh
) >"$tmp/dirty-tree-arms/overlay.out" 2>&1
dirty_overlay_rc=$?
set -e
[[ $dirty_overlay_rc != 0 ]] \
  || { echo "witness-gate fixture dirty-tree-arms: dirty producer accepted a Go overlay" >&2; exit 1; }
grep -Fq 'GOFLAGS may not contain -modfile or -overlay' "$tmp/dirty-tree-arms/overlay.out" \
  || { echo "witness-gate fixture dirty-tree-arms: overlay refusal was not explicit" >&2; exit 1; }


fi
freeze_fixture_tree() { # name
  local name=$1 freeze_output
  make_leg "$name"
  mkdir -p "$tmp/$name/freeze-tmp"
  freeze_output=$(TMPDIR="$tmp/$name/freeze-tmp" "$source_engine" gate witness-freeze --root "$leg_tree")
  read -r frozen_digest frozen_tree <<<"$freeze_output"
  [[ "$frozen_digest" =~ ^[a-f0-9]{64}$ && -d "$frozen_tree" ]]
  write_witness "$name" "$frozen_tree" "$leg_bin" $$
  add_manifest_digest "$witness_path" "$frozen_digest"
  frozen_witness=$witness_path
  frozen_root=$witness_root
  frozen_run=$witness_run
}

copy_frozen_consumer() { # name, frozen tree
  local name=$1 from=$2
  frozen_consumer=$tmp/$name/consumer
  mkdir -p "$frozen_consumer"
  cp -R "$from/." "$frozen_consumer"
}

run_check_only_acceptance() { # tree, fixture bin, witness, state root, run id
  local tree=$1 fixture_bin=$2 witness=$3 state_root=$4 run=$5
  local banner
  banner=$(cd "$tree" && env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
    METASYSTEM_ALLOW_CONCURRENT_GATE=1 METASYSTEM_GATE_WITNESS="$witness" \
    METASYSTEM_GATE_WITNESS_ROOT="$state_root" METASYSTEM_GATE_WITNESS_RUN="$run" \
    "$source_engine" proof-run banner --suite fixture --root "$tree" \
      --progress "$tree/progress.jsonl" --log "$tree/suite.log")
  [[ "$banner" == *'witness=armed duration=minutes'* ]] \
    || { echo "witness-gate fixture: usable witness banner was not armed" >&2; exit 1; }
  banner=$(cd "$tree" && env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
    METASYSTEM_ALLOW_CONCURRENT_GATE=1 METASYSTEM_GATE_WITNESS="$witness" \
    METASYSTEM_GATE_WITNESS_ROOT="$state_root" METASYSTEM_GATE_WITNESS_RUN="$run" \
    METASYSTEM_GATE_WITNESS_EXPORT="$tree" \
    "$source_engine" proof-run banner --suite fixture --root "$tree" \
      --progress "$tree/progress.jsonl" --log "$tree/suite.log")
  [[ "$banner" == *'witness=frozen duration=minutes'* ]] \
    || { echo "witness-gate fixture: usable exported witness banner was not frozen" >&2; exit 1; }
  (
    cd "$tree"
    env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
      METASYSTEM_GATE_WITNESS="$witness" METASYSTEM_GATE_WITNESS_ROOT="$state_root" \
      METASYSTEM_GATE_WITNESS_RUN="$run" METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
      bash scripts/agents/go-gate.sh --witness-check-only >/dev/null
  )
}

run_frozen_flag_refusal() { # tree, fixture bin, witness, state root, run id
  local tree=$1 fixture_bin=$2 witness=$3 state_root=$4 run=$5
  local output=$tmp/frozen-flag-refusal.out rc=0
  set +e
  (
    cd "$tree"
    env PATH="$fixture_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine" \
      GOFLAGS='-modfile=outside.mod' METASYSTEM_GATE_WITNESS="$witness" \
      METASYSTEM_GATE_WITNESS_ROOT="$state_root" METASYSTEM_GATE_WITNESS_RUN="$run" \
      METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
      bash scripts/agents/go-gate.sh --witness-check-only
  ) >"$output" 2>&1
  rc=$?
  set -e
  [[ $rc != 0 ]] \
    || { echo "witness-gate fixture: frozen consumer accepted an alternate modfile" >&2; exit 1; }
  grep -Fq 'GOFLAGS may not contain -modfile or -overlay' "$output" \
    || { echo "witness-gate fixture: alternate modfile refusal was not explicit" >&2; exit 1; }
}


if [[ "$fixture_scenario" == frozen-consumers ]]; then
# 15. A byte-identical copy accepts the full-tree witness.
freeze_fixture_tree frozen-identical
copy_frozen_consumer frozen-identical "$frozen_tree"
run_check_only_acceptance "$frozen_consumer" "$leg_bin" \
  "$frozen_witness" "$frozen_root" "$frozen_run"
run_frozen_flag_refusal "$frozen_consumer" "$leg_bin" \
  "$frozen_witness" "$frozen_root" "$frozen_run"
run_engine_acceptance frozen-identical "$frozen_consumer" "$leg_bin" \
  "$frozen_witness" "$frozen_root" "$frozen_run"

# 16. One changed byte in the included closure reaches the full gate.
freeze_fixture_tree frozen-closure-mutation
copy_frozen_consumer frozen-closure-mutation "$frozen_tree"
printf 'package fixture\nvar Changed = true\n' >"$frozen_consumer/internal/fixture/fixture.go"
run_refusal frozen-closure-mutation "$frozen_consumer" "$leg_bin" \
  "$frozen_witness" "$frozen_root" "$frozen_run" ENGINE
# The sourced wrapper must preserve that same frozen fallback. In particular,
# refusing an inherited witness must not scrub it and restart against the live
# consumer tree.
cp "$root/scripts/agents/witness-gate.sh" "$frozen_consumer/scripts/agents/witness-gate.sh"
frozen_wrapper_gofmt_root=$tmp/frozen-closure-mutation/wrapper-gofmt-root
set +e
(
  cd "$frozen_consumer"
  export PATH="$leg_bin:$PATH" WITNESS_FIXTURE_SOURCE_ENGINE="$source_engine"
  export WITNESS_FIXTURE_GOFMT_ROOT_MARKER="$frozen_wrapper_gofmt_root"
  export METASYSTEM_GATE_WITNESS="$frozen_witness" METASYSTEM_GATE_WITNESS_ROOT="$frozen_root"
  export METASYSTEM_GATE_WITNESS_RUN="$frozen_run"
  root=$(pwd -P)
  delivery_contract=0
  WITNESS_GATE_FALLBACK=plain source scripts/agents/witness-gate.sh
) >"$tmp/frozen-closure-mutation/wrapper.out" 2>&1
frozen_wrapper_rc=$?
set -e
[[ $frozen_wrapper_rc != 0 && -f "$frozen_wrapper_gofmt_root" ]] \
  || { echo "witness-gate fixture frozen-closure-mutation: wrapper fallback did not reach the broken full proof" >&2; sed -n '1,100p' "$tmp/frozen-closure-mutation/wrapper.out" >&2; exit 1; }
[[ "$(cat "$frozen_wrapper_gofmt_root")" != "$frozen_consumer" ]] \
  || { echo "witness-gate fixture frozen-closure-mutation: wrapper fallback ran against the live consumer" >&2; exit 1; }
case $(cat "$frozen_wrapper_gofmt_root") in
  */metasystem-witness-freeze-*/tree) ;;
  *) echo "witness-gate fixture frozen-closure-mutation: wrapper fallback ran outside a private frozen export" >&2; exit 1 ;;
esac

# 17. Runtime state below artifacts/ is outside the manifest and still reuses.
freeze_fixture_tree frozen-artifacts-mutation
copy_frozen_consumer frozen-artifacts-mutation "$frozen_tree"
mkdir -p "$frozen_consumer/artifacts"
printf 'runtime-only mutation\n' >"$frozen_consumer/artifacts/state"
run_engine_acceptance frozen-artifacts-mutation "$frozen_consumer" "$leg_bin" \
  "$frozen_witness" "$frozen_root" "$frozen_run"

# 18. Right bytes do not compensate for a live foreign controller.
sleep 60 & foreign_controller=$!
freeze_fixture_tree frozen-foreign-controller
copy_frozen_consumer frozen-foreign-controller "$frozen_tree"
write_witness frozen-foreign-controller "$frozen_tree" "$leg_bin" "$foreign_controller"
add_manifest_digest "$witness_path" "$frozen_digest"
run_refusal frozen-foreign-controller "$frozen_consumer" "$leg_bin" \
  "$witness_path" "$witness_root" "$witness_run" ENGINE
kill "$foreign_controller" 2>/dev/null || true
wait "$foreign_controller" 2>/dev/null || true
foreign_controller=

# 19. A mutation performed by the skip-path build fails the second digest and
# falls through to the complete gate.
freeze_fixture_tree frozen-consumer-recheck
copy_frozen_consumer frozen-consumer-recheck "$frozen_tree"
run_recheck_refusal frozen-consumer-recheck "$frozen_consumer" "$leg_bin" \
  "$frozen_witness" "$frozen_root" "$frozen_run" \
  "internal/fixture/fixture.go"

# 20. A source mutation after the first manifest read but before publication
# voids the freeze and leaves no export path to consume.
mid_source=$tmp/mid-freeze/source
mid_tmp=$tmp/mid-freeze/tmp
mkdir -p "$mid_source/internal/z-many" "$mid_tmp"
printf 'before\n' >"$mid_source/internal/a-mutated"
for mid_n in $(seq 1 2000); do
  printf '%s\n' "$mid_n" >"$mid_source/internal/z-many/$mid_n"
done
set +e
TMPDIR="$mid_tmp" "$source_engine" gate witness-freeze --root "$mid_source" \
  >"$tmp/mid-freeze/out" 2>"$tmp/mid-freeze/err" &
mid_freeze_pid=$!
set -e
mid_deadline=$((SECONDS + 10))
mid_tree_seen=0
while (( SECONDS < mid_deadline )); do
  for mid_candidate in "$mid_tmp"/metasystem-witness-freeze-*/tree; do
    if [[ -d "$mid_candidate" ]]; then mid_tree_seen=1; break 2; fi
  done
  sleep 0.01
done
(( mid_tree_seen )) \
  || { echo "witness-gate fixture mid-freeze: private export did not appear before the fixture ceiling" >&2; exit 1; }
printf 'after\n' >"$mid_source/internal/a-mutated"
set +e
wait "$mid_freeze_pid"
mid_freeze_rc=$?
set -e
mid_freeze_pid=
[[ $mid_freeze_rc != 0 ]] \
  || { echo "witness-gate fixture mid-freeze: a changing source published an export" >&2; exit 1; }
grep -Fq 'frozen export voided because the source changed while it was copied' "$tmp/mid-freeze/err" \
  || { echo "witness-gate fixture mid-freeze: mutation refusal was not loud" >&2; cat "$tmp/mid-freeze/err" >&2; exit 1; }

fi

if [[ "$fixture_scenario" == cost-banner ]]; then
# 21. The one-line cost banner classifies clean, dirty-frozen, inherited, and
# inherited-frozen witness states before the suite begins doing work.
banner_root=$tmp/banner-states
mkdir -p "$banner_root/artifacts/agents/supervision/suite-logs" "$banner_root/internal"
git -C "$banner_root" init -q -b main
printf 'tracked\n' >"$banner_root/internal/tracked"
git -C "$banner_root" add internal/tracked
git -C "$banner_root" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm initial
banner_args=(proof-run banner --suite fixture --root "$banner_root" \
  --progress "$banner_root/artifacts/agents/supervision/suite-progress.jsonl" \
  --log "$banner_root/artifacts/agents/supervision/suite-logs/run.log")
"$source_engine" "${banner_args[@]}" >"$tmp/banner-unarmed"
grep -qxF 'suite-cost suite=fixture witness=unarmed duration=full-gate heartbeat=artifacts/agents/supervision/suite-progress.jsonl logs=artifacts/agents/supervision/suite-logs/run.log' "$tmp/banner-unarmed" \
  || { echo "witness-gate fixture banner: clean unarmed state was misclassified" >&2; exit 1; }
printf 'dirty\n' >>"$banner_root/internal/tracked"
"$source_engine" "${banner_args[@]}" >"$tmp/banner-frozen"
grep -q 'witness=frozen duration=minutes' "$tmp/banner-frozen" \
  || { echo "witness-gate fixture banner: dirty frozen state was misclassified" >&2; exit 1; }
METASYSTEM_GATE_FORCE=1 "$source_engine" "${banner_args[@]}" >"$tmp/banner-forced"
grep -q 'witness=unarmed duration=full-gate' "$tmp/banner-forced" \
  || { echo "witness-gate fixture banner: forced full gate was misclassified" >&2; exit 1; }
METASYSTEM_GATE_WITNESS=dummy "$source_engine" "${banner_args[@]}" >"$tmp/banner-armed"
grep -q 'witness=unarmed duration=full-gate' "$tmp/banner-armed" \
  || { echo "witness-gate fixture banner: unusable inherited witness was misclassified" >&2; exit 1; }
METASYSTEM_GATE_WITNESS=dummy METASYSTEM_GATE_WITNESS_EXPORT=/private/frozen \
  "$source_engine" "${banner_args[@]}" >"$tmp/banner-inherited-frozen"
grep -q 'witness=unarmed duration=full-gate' "$tmp/banner-inherited-frozen" \
  || { echo "witness-gate fixture banner: unusable exported witness was misclassified" >&2; exit 1; }
fi
