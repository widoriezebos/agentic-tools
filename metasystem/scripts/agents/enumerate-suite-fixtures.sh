#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/enumerate-suite-fixtures.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# The selector is the suite's section ledger. A guarded section that is not in
# that ledger would disappear from enumeration; a stale ledger row would pass
# without running any section body.
sed -n 's/.*section_selected \([a-z0-9-]*\).*/\1/p' \
  "$root/scripts/validate-metasystem.sh" | sort -u >"$tmp/guarded-sections"
bash "$root/scripts/agents/validate-section-selector.sh" catalog \
  | cut -f1 | sort -u >"$tmp/selected-sections"
diff -u "$tmp/guarded-sections" "$tmp/selected-sections" \
  || { echo "enumeration fixture: validator guards and selector rows disagree" >&2; exit 1; }

# Context is repository identity, not a nearby filesystem coincidence. Copy
# the implementation under test over a fresh clone because the working change
# is not committed during local validation; the identity facts still come
# only from the clone's committed tree.
checkout=$(git -C "$root" rev-parse --show-toplevel)
prefix=$(git -C "$root" rev-parse --show-prefix)
template_clone="$tmp/template-clone"
git clone -q --no-local "$checkout" "$template_clone"
template_root=$template_clone/${prefix%/}
cp "$root/scripts/agents/validate-section-selector.sh" \
  "$template_root/scripts/agents/validate-section-selector.sh"
# The checkout file is not the identity proof; the blob in HEAD is. Leaving
# it absent here makes the fixture fail if classification regresses to an
# untracked or worktree-only marker.
rm "$template_clone/development/metasystem-design.md"
[[ $(bash "$template_root/scripts/agents/validate-section-selector.sh" context) == template ]] \
  || { echo "enumeration fixture: an independent template clone classified as adopted" >&2; exit 1; }
[[ $(bash "$template_root/scripts/agents/validate-section-selector.sh" list | wc -l | tr -d ' ') == 42 ]] \
  || { echo "enumeration fixture: an independent template clone did not select 42 sections" >&2; exit 1; }

adopted_repo="$tmp/adopted-repo"
adopted_root="$adopted_repo/metasystem"
mkdir -p "$adopted_root/scripts/agents" "$adopted_repo/development"
cp "$root/scripts/agents/validate-section-selector.sh" \
  "$adopted_root/scripts/agents/validate-section-selector.sh"
cp "$root/go.mod" "$adopted_root/go.mod"
git -C "$adopted_repo" init -q -b main
git -C "$adopted_repo" add metasystem
git -C "$adopted_repo" -c user.name=metasystem -c user.email=metasystem@example.invalid \
  commit -qm adopted
# This is the old marker shape, deliberately left untracked.
printf 'not repository identity\n' >"$adopted_repo/development/metasystem-design.md"
[[ $(bash "$adopted_root/scripts/agents/validate-section-selector.sh" context) == adopted ]] \
  || { echo "enumeration fixture: an untracked marker promoted an adopted installation" >&2; exit 1; }
[[ $(bash "$adopted_root/scripts/agents/validate-section-selector.sh" list | wc -l | tr -d ' ') == 38 ]] \
  || { echo "enumeration fixture: an adopted installation did not select 38 sections" >&2; exit 1; }

selector="$tmp/selector.sh"
cat >"$selector" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case ${1:-} in
  list)
    printf 'first-clean\tfirst clean section\n'
    printf 'first-defect\tfirst defect section\n'
    printf 'last-defect\tlast defect section\n'
    ;;
  run)
    case ${2:-} in
      first-clean) echo 'clean marker' ;;
      first-defect)
        [[ "${ENUMERATION_FIXTURE_BED:-failing}" == clean ]] && exit 0
        echo 'first defect tail marker' >&2
        exit 7
        ;;
      last-defect)
        [[ "${ENUMERATION_FIXTURE_BED:-failing}" == clean ]] && exit 0
        printf 'last defect tail marker\nwith a second line\n' >&2
        exit 9
        ;;
      *) exit 2 ;;
    esac
    ;;
  *) exit 2 ;;
esac
EOF

failing_report="$tmp/failing.tsv"
failing_out="$tmp/failing.out"
failing_rc=0
if bash "$root/scripts/validate-metasystem.sh" --enumerate \
    --report "$failing_report" --selector "$selector" >"$failing_out"; then
  failing_rc=0
else
  failing_rc=$?
fi
[[ $failing_rc -eq 1 ]] \
  || { echo "enumeration fixture: two-defect bed exited $failing_rc instead of 1" >&2; exit 1; }
grep -Fq $'section\tfirst-defect\tfirst defect section\tfail\t7\tfirst defect tail marker' "$failing_report" \
  || { echo "enumeration fixture: report lost the first defect" >&2; exit 1; }
grep -Fq $'section\tlast-defect\tlast defect section\tfail\t9\tlast defect tail marker\\nwith a second line' "$failing_report" \
  || { echo "enumeration fixture: report lost the last defect or its escaped tail" >&2; exit 1; }
grep -Fq '3 sections run / 2 failed; failed sections: first defect section, last defect section' "$failing_out" \
  || { echo "enumeration fixture: failing summary did not name both defects" >&2; exit 1; }

clean_report="$tmp/clean.tsv"
clean_out="$tmp/clean.out"
ENUMERATION_FIXTURE_BED=clean \
  bash "$root/scripts/validate-metasystem.sh" --enumerate \
    --report "$clean_report" --selector "$selector" >"$clean_out"
[[ $(grep -c $'\tpass\t0\t' "$clean_report") -eq 3 ]] \
  || { echo "enumeration fixture: clean report did not pass all three sections" >&2; exit 1; }
grep -Fq '3 sections run / 0 failed; failed sections: none' "$clean_out" \
  || { echo "enumeration fixture: clean summary was not green" >&2; exit 1; }

# A selected section that records no row is invalid, even when every body the
# validator did run was green or lawfully gated.
empty_selector="$tmp/empty-selector.sh"
cat >"$empty_selector" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case ${1:-} in
  list) printf 'selected-empty-section\tselected empty section\n' ;;
  run)
    [[ ${2:-} == selected-empty-section ]] || exit 2
    METASYSTEM_VALIDATION_STAGE_RESULTS_OUT=${METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT:?} \
    METASYSTEM_VALIDATION_STAGE_RESULTS_WRITER=1 \
    METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=${METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY:-unproven} \
    METASYSTEM_ENUMERATION_DRIVER=1 \
      bash "$ENUMERATION_FIXTURE_ROOT/scripts/validate-metasystem.sh" \
        --enumeration-section selected-empty-section
    ;;
  *) exit 2 ;;
esac
EOF
empty_report="$tmp/empty.tsv"
empty_out="$tmp/empty.out"
empty_rc=0
ENUMERATION_FIXTURE_ROOT="$root" \
  bash "$root/scripts/validate-metasystem.sh" --enumerate \
    --report "$empty_report" --selector "$empty_selector" >"$empty_out" 2>&1 \
  || empty_rc=$?
[[ $empty_rc -eq 2 ]] \
  || { echo "enumeration fixture: selected empty section exited $empty_rc instead of INVALID rc 2" >&2; exit 1; }
grep -Fq $'section\tselected-empty-section\tselected empty section\tinvalid\t2\t' "$empty_report" \
  || { echo "enumeration fixture: selected empty section was not recorded invalid" >&2; exit 1; }
grep -Fq 'selected section selected-empty-section recorded 0 results; exactly one is required' "$empty_report" \
  || { echo "enumeration fixture: invalid result did not name the missing selected row" >&2; exit 1; }

# Dependency state comes only from the Go gate's recorded row. A failed gate
# must make the later engine consumer gated, never passed by process default.
dependency_selector="$tmp/dependency-selector.sh"
cat >"$dependency_selector" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case ${1:-} in
  list)
    printf 'go-engine-gate\tGo engine gate\n'
    printf 'engine-consumer\tengine consumer\n'
    ;;
  run)
    result=${METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT:?}
    printf 'format\tmetasystem-validation-stage-results-v1\n' >"$result"
    printf 'columns\tkind\tid\tstatus\texit_code\tfailure_tail\n' >>"$result"
    case ${2:-} in
      go-engine-gate)
        printf 'section\tgo-engine-gate\tfail\t7\trecorded gate failure\n' >>"$result"
        exit 1
        ;;
      engine-consumer)
        [[ ${METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY:-} == failed ]] \
          || { echo "engine consumer did not inherit the recorded failed gate" >&2; exit 2; }
        printf 'section\tengine-consumer\tgated\t0\tneeds engine: recorded gate failed\n' >>"$result"
        ;;
      *) exit 2 ;;
    esac
    ;;
  *) exit 2 ;;
esac
EOF
dependency_report="$tmp/dependency.tsv"
dependency_rc=0
bash "$root/scripts/validate-metasystem.sh" --enumerate \
  --report "$dependency_report" --selector "$dependency_selector" >/dev/null 2>&1 \
  || dependency_rc=$?
[[ $dependency_rc -eq 1 ]] \
  || { echo "enumeration fixture: recorded gate failure exited $dependency_rc instead of 1" >&2; exit 1; }
grep -Fq $'section\tgo-engine-gate\tGo engine gate\tfail\t7\trecorded gate failure' "$dependency_report" \
  || { echo "enumeration fixture: recorded Go gate failure was lost" >&2; exit 1; }
grep -Fq $'section\tengine-consumer\tengine consumer\tgated\t0\tneeds engine: recorded gate failed' "$dependency_report" \
  || { echo "enumeration fixture: engine consumer did not remain gated" >&2; exit 1; }

echo "enumeration mode fixtures passed"
