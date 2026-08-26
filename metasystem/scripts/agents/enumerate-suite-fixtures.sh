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
bash "$root/scripts/agents/validate-section-selector.sh" list \
  | cut -f1 | sort -u >"$tmp/selected-sections"
diff -u "$tmp/guarded-sections" "$tmp/selected-sections" \
  || { echo "enumeration fixture: validator guards and selector rows disagree" >&2; exit 1; }

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

echo "enumeration mode fixtures passed"
