#!/usr/bin/env bash
# The exclusion slice's empirical proof (g1-s8 revision 5, "The guard"): an
# installed frontend dependency tree changes no verdict this engine renders.
# Two enumeration sweeps each claimed to have found every walker and each
# missed one, so completeness is MEASURED here and never searched: the
# repository's own validation runs once with the tree absent and once with an
# adversarial tree planted beneath internal/ui/web/_app/node_modules, and every
# exit status and verdict must be identical.
#
# Green first, then poison (K3). Nothing is planted until every absent-tree
# command has succeeded with its expected verdict; the guard refuses and names
# the first that has not. Revision 4 compared the two runs for equality without
# that precondition, so two identically broken validations would have passed.
#
# A first-run difference caused by any planted file is a walker the exclusion
# missed and a finding for the slice, never a reason to thin the corpus.
#
# Runs from metasystem/. Not a validation section: it runs the validation.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"
toplevel=$(cd "$(git rev-parse --show-toplevel)" && pwd -P)

ms=$root/bin/metasystem
app=$root/internal/ui/web/_app
tree=$app/node_modules
poison=$tree/poison
bin_dir=$tree/.bin
bin_link=$bin_dir/poison

guard_refuse() { # message
  echo "dependency-tree guard: $1" >&2
  exit 1
}

[[ -x "$ms" ]] || guard_refuse "bin/metasystem is not built; run scripts/agents/go-build.sh first"
[[ ! -e "$poison" && ! -L "$poison" ]] || guard_refuse "internal/ui/web/_app/node_modules/poison already exists"
[[ ! -e "$bin_link" && ! -L "$bin_link" ]] || guard_refuse "internal/ui/web/_app/node_modules/.bin/poison already exists"

# Only the directories this run creates are removed again: a real installed
# tree, or a .bin beside it, is left exactly as it was found.
guard_created=()
work=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-dependency-tree-guard.XXXXXX")
guard_cleanup() {
  local status=$? index
  rm -rf "$poison"
  rm -f "$bin_link"
  for (( index=${#guard_created[@]} - 1; index >= 0; index-- )); do
    rmdir "${guard_created[$index]}" 2>/dev/null || true
  done
  # Both transcripts are kept, green or red: they are the slice's evidence.
  echo "dependency-tree guard: transcripts kept at $work" >&2
  return $status
}
trap guard_cleanup EXIT

absent=$work/absent
planted=$work/planted
mkdir -p "$absent" "$planted"

# The delivery probe is the one leg that needs an enrolled machine nickname
# (internal/goal/actor.go: ResolveMachine, no hostname fallback and no
# environment override). Where none is enrolled the two verbs cannot run at
# all, so they are declared unavailable HERE, before anything runs, and are
# then absent from the green-first checklist and from the comparison alike —
# the checklist stays one-to-one with what is compared. Sol's round 5 ruled
# this leg proves forced subtree parity only and no walker coverage at all
# (Pattern.Expand is reached by neither verb), so its absence costs the guard
# no walker proof; the fixtures own that. Any other refusal of either verb is
# a red like any other.
delivery_probe=1
if ! git config --get metasystem.goal.machine >/dev/null 2>&1; then
  delivery_probe=0
fi

guard_step() { # output directory, step name, command and arguments
  local dir=$1 name=$2
  shift 2
  local status=0
  "$@" >"$dir/$name.out" 2>"$dir/$name.err" || status=$?
  printf '%s\n' "$status" >"$dir/$name.status"
}

guard_status() { # output directory, step name
  cat "$1/$2.status"
}

guard_run_pass() { # output directory, label
  local dir=$1 label=$2 projection
  echo "dependency-tree guard: $label pass"
  guard_step "$dir" status git status --porcelain
  for projection in ENGINE LANDING PAYLOAD; do
    guard_step "$dir" "digest-$projection" "$ms" behavior-surface digest \
      --root "$toplevel" --prefix metasystem --projection "$projection" --endpoint check
  done
  guard_step "$dir" ratchet "$ms" audit parallel-ratchet
  guard_step "$dir" stop-surface "$ms" audit stop-decision-surface
  guard_step "$dir" metasystem-audit bash scripts/audit-metasystem.sh
  guard_step "$dir" go-gate bash scripts/agents/go-gate.sh
  guard_step "$dir" validation bash scripts/validate-metasystem.sh
  if (( delivery_probe )); then
    guard_step "$dir" test-plan "$ms" test plan --root "$root" --json
    guard_step "$dir" test-run "$ms" test run --root "$root" \
      --purpose diagnostic --groups section/brain-fixtures --result "$dir/test-run-result.json"
  fi
}

# Every command and semantic verdict here is compared again below, one to one
# (obligation 1). A red anything refuses before a poison path exists.
guard_require_green() { # output directory
  local dir=$1 projection
  [[ "$(guard_status "$dir" status)" == 0 ]] || guard_refuse "git status --porcelain refused with status $(guard_status "$dir" status)"
  [[ ! -s "$dir/status.out" ]] || guard_refuse "the checkout is not clean; run the guard on a clean tree"
  for projection in ENGINE LANDING PAYLOAD; do
    [[ "$(guard_status "$dir" "digest-$projection")" == 0 ]] \
      || guard_refuse "the $projection digest refused with status $(guard_status "$dir" "digest-$projection")"
    [[ "$(grep -c 'surfaceDigest' "$dir/digest-$projection.out")" == 1 ]] \
      || guard_refuse "the $projection digest did not print one digest"
  done
  [[ "$(guard_status "$dir" ratchet)" == 0 ]] || guard_refuse "audit parallel-ratchet refused with status $(guard_status "$dir" ratchet)"
  [[ "$(guard_status "$dir" stop-surface)" == 0 ]] || guard_refuse "audit stop-decision-surface refused with status $(guard_status "$dir" stop-surface)"
  [[ "$(guard_status "$dir" metasystem-audit)" == 0 ]] || guard_refuse "the metasystem audit refused with status $(guard_status "$dir" metasystem-audit)"
  [[ ! -s "$dir/metasystem-audit.err" ]] || guard_refuse "the metasystem audit reported violations"
  [[ "$(guard_status "$dir" go-gate)" == 0 ]] || guard_refuse "the Go gate refused with status $(guard_status "$dir" go-gate)"
  grep -q '^go gate: PASSED' "$dir/go-gate.out" || guard_refuse "the Go gate printed no green verdict line"
  [[ "$(guard_status "$dir" validation)" == 0 ]] || guard_refuse "the validation refused with status $(guard_status "$dir" validation)"
  grep -q 'SECTION GREEN: ' "$dir/validation.out" || guard_refuse "the validation printed no section verdict"
  ! grep -q 'SECTION RED: ' "$dir/validation.out" "$dir/validation.err" || guard_refuse "the validation printed a red section"
  if (( delivery_probe )); then
    [[ "$(guard_status "$dir" test-plan)" == 0 ]] || guard_refuse "test plan refused with status $(guard_status "$dir" test-plan)"
    # Transcript only (obligation 7): a selected group declaring
    # metasystem/internal/** proves forced subtree parity, never that
    # pathpattern.Pattern.Expand ran — a trailing /** parses into the subtree
    # flag and SnapshotRelevant takes its non-wildcard branch.
    if grep -q 'metasystem/internal/\*\*' "$dir/test-plan.out"; then
      echo "dependency-tree guard: the delivery plan names a group declaring metasystem/internal/** (forced subtree parity only)"
    else
      echo "dependency-tree guard: the delivery plan names no group declaring metasystem/internal/**; report it as a finding"
    fi
    [[ "$(guard_status "$dir" test-run)" == 0 ]] || guard_refuse "test run refused with status $(guard_status "$dir" test-run)"
    grep -q '"status":"passed"' "$dir/test-run-result.json" || guard_refuse "the diagnostic group result carries no green status"
  fi
  echo "dependency-tree guard: every absent-tree command is green"
}

guard_plant() { # creates the adversarial tree
  local directory space_name
  for directory in "$root/internal/ui" "$root/internal/ui/web" "$app" "$tree" "$bin_dir"; do
    if [[ ! -d "$directory" ]]; then
      mkdir -p "$directory"
      guard_created+=("$directory")
    fi
  done
  mkdir -p "$poison/node_modules/deeper" "$poison/bin" "$poison/.github/workflows"

  # Unformatted on purpose (space indentation), and holding each of the four
  # expressions brain-fixtures.sh scans cmd/metasystem and internal for.
  cat >"$poison/seam.go" <<'POISON_SEAM'
package poison

const seam = `
 goal.Actor{
 Actor.Human = "poison"
 classifyVerbCaller(
 lease.ClassifyVerbAt(e.Root
`
POISON_SEAM
  cp "$poison/seam.go" "$poison/node_modules/deeper/seam.go"

  # gofmt cannot parse this (exit 2) and no Go tool can build it.
  printf '%s\n' 'package poison' 'func {' >"$poison/broken.go"

  # The shape ScanParallelTests counts and discoverStopSurfaceFiles selects,
  # once under a plain name and once under a name with a space and a
  # non-ASCII letter, for every find, xargs and pathspec on the way. The
  # name is built from escapes so this script stays ASCII.
  cat >"$poison/serial_test.go" <<'POISON_TEST'
package poison

import "testing"

func TestPoison(t *testing.T) {}
POISON_TEST
  space_name=$(printf 'sp\303\244ce d_test.go')
  cp "$poison/serial_test.go" "$poison/$space_name"

  # The three tokens path-class-fixtures.sh searches for, written by
  # concatenation exactly as that script assembles them, so this script stays
  # clean under its own scan.
  printf '%s %s %s\n' 'register-carriage-'paths 'instruction-bearing-'paths 'neverDirect'Fix >"$poison/notes.txt"

  # The names the metasystem audit's instruction inventory collects.
  printf '%s\n' 'poison instruction file' >"$poison/AGENTS.md"
  printf '%s\n' 'poison instruction file' >"$poison/wow.md"
  printf '%s\n' 'poison instruction file' >"$poison/SKILL.md"
  printf '%s\n' 'poison instruction file' >"$poison/CLAUDE.md"

  # internal/, so a walker that followed links would loop or double-count.
  ln -s ../../../../.. "$poison/up"

  # Anything treating the tree as a module or an npm project shows itself.
  printf '%s\n' 'module poison' >"$poison/go.mod"
  cat >"$poison/package.json" <<'POISON_PACKAGE'
{"name":"poison","version":"0.0.0","scripts":{"postinstall":"exit 1"},"bin":{"poison":"bin/poison"}}
POISON_PACKAGE

  # (K4) The name matches both *.sh and *fixture*.sh, the two classes the
  # repository's own shell gate enumerates, and bash -n rejects the body.
  {
    printf '%s\n' '#!/usr/bin/env bash'
    printf '%s\n' 'if true; then'
    printf '%s %s %s\n' '# register-carriage-'paths 'instruction-bearing-'paths 'neverDirect'Fix
  } >"$poison/poison-fixtures.sh"
  chmod 0755 "$poison/poison-fixtures.sh"

  # (K4) The extensionless executable the bin entry names, and the link npm
  # would create for it, planted outside poison/ and removed with it.
  {
    printf '%s\n' '#!/usr/bin/env bash'
    printf '%s\n' 'if true; then'
  } >"$poison/bin/poison"
  chmod 0755 "$poison/bin/poison"
  ln -s ../poison/bin/poison "$bin_link"

  # (K4) Hidden configuration a dotfile walker would read.
  printf '%s\n' 'offline=true' 'registry=http://127.0.0.1:9/' 'ignore-scripts=false' >"$poison/.npmrc"

  # (K4) An attempt to re-include the tree from within, which Git refuses
  # because a file under an excluded directory cannot be re-included, so
  # git status --porcelain must stay empty.
  printf '%s\n' '!*' '!**' >"$poison/.gitignore"

  # (K4) A hidden directory holding the workflow name adoption checks for.
  printf '%s\n' 'on: push' >"$poison/.github/workflows/metasystem.yml"

  # (K4) The two exact-name contracts this repository keys on.
  printf '%s\n' '{"schemaVersion": 1, "groups": []}' >"$poison/testing.json"
  printf '%s\n' 'metasystem.runtimes=poison' >"$poison/metasystem.conf"
}

guard_same() { # what, absent file, planted file
  cmp -s "$2" "$3" || guard_refuse "$1 differs between the absent and planted runs"
}

guard_compare() {
  local name projection
  for name in status digest-ENGINE digest-LANDING digest-PAYLOAD ratchet stop-surface \
    metasystem-audit go-gate validation; do
    guard_same "the exit status of $name" "$absent/$name.status" "$planted/$name.status"
  done
  guard_same "the git status --porcelain output" "$absent/status.out" "$planted/status.out"
  for projection in ENGINE LANDING PAYLOAD; do
    guard_same "the $projection digest" "$absent/digest-$projection.out" "$planted/digest-$projection.out"
  done
  guard_same "the parallel-ratchet audit" "$absent/ratchet.out" "$planted/ratchet.out"
  guard_same "the stop-decision-surface audit" "$absent/stop-surface.out" "$planted/stop-surface.out"
  guard_same "the metasystem audit's violations" "$absent/metasystem-audit.err" "$planted/metasystem-audit.err"

  # The instruction inventory is a report line, never a verdict (G6), and the
  # four planted instruction files are required to be the WHOLE of its
  # difference: that is what makes G6's record empirical rather than argued.
  local inventory_added inventory_removed
  inventory_added=$(comm -13 <(LC_ALL=C sort "$absent/metasystem-audit.out") <(LC_ALL=C sort "$planted/metasystem-audit.out") || true)
  inventory_removed=$(comm -23 <(LC_ALL=C sort "$absent/metasystem-audit.out") <(LC_ALL=C sort "$planted/metasystem-audit.out") || true)
  [[ -z "$inventory_removed" ]] || guard_refuse "the planted run lost metasystem audit report lines: $inventory_removed"
  local expected_inventory
  expected_inventory=$(printf './internal/ui/web/_app/node_modules/poison/%s\n' AGENTS.md CLAUDE.md SKILL.md wow.md | LC_ALL=C sort)
  [[ "$(printf '%s\n' "$inventory_added" | LC_ALL=C sort)" == "$expected_inventory" ]] \
    || guard_refuse "the metasystem audit inventory differs by something other than the four planted instruction files: $inventory_added"

  guard_same "the Go gate's verdict line" \
    <(grep '^go gate: PASSED' "$absent/go-gate.out" | tail -n 1) \
    <(grep '^go gate: PASSED' "$planted/go-gate.out" | tail -n 1)
  guard_same "the validation's green section verdicts" \
    <(grep 'SECTION GREEN: ' "$absent/validation.out") \
    <(grep 'SECTION GREEN: ' "$planted/validation.out")
  guard_same "the validation's red section verdicts" \
    <(grep 'SECTION RED: ' "$absent/validation.err" || true) \
    <(grep 'SECTION RED: ' "$planted/validation.err" || true)

  if (( delivery_probe )); then
    for name in test-plan test-run; do
      guard_same "the exit status of $name" "$absent/$name.status" "$planted/$name.status"
    done
    # testingPlanOutput, testpolicy.Plan, Stage, Omission and RiskAssessment
    # declare no field that names a time, so the plan JSON compares whole.
    guard_same "the delivery plan" "$absent/test-plan.out" "$planted/test-plan.out"
    # A consistency check, never a walker proof: the group's inputs are
    # expanded in the detached checkout of the candidate, and the candidate is
    # the staged tree, which never holds an ignored file.
    guard_same "the diagnostic group's inputDigest" \
      <(grep -o '"inputDigest":"[0-9a-f]*"' "$absent/test-run-result.json" | head -n 1) \
      <(grep -o '"inputDigest":"[0-9a-f]*"' "$planted/test-run-result.json" | head -n 1)
  fi
}

if (( delivery_probe )); then
  echo "dependency-tree guard: the delivery probe (test plan, test run) is enrolled and runs"
else
  echo "dependency-tree guard: no machine nickname is enrolled (git config metasystem.goal.machine), so the delivery probe is unavailable and is run in neither pass; it carries no walker proof (Sol's round 5, obligation 7)"
fi

guard_run_pass "$absent" absent
guard_require_green "$absent"
guard_plant
echo "dependency-tree guard: the adversarial tree is planted at internal/ui/web/_app/node_modules/poison"
guard_run_pass "$planted" planted
guard_compare
echo "dependency-tree guard: PASSED (every verdict identical with the dependency tree absent and planted)"
