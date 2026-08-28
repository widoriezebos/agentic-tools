#!/usr/bin/env bash
# The external-dependency ratchet (os-dependency-reduction slice
# three, Wido's 2026-08-25 ruling: dependencies must never regrow
# silently). The INVENTORY below is the declaration: interpreters
# beyond it refuse at the gate with the offending site named. The
# metasystem's own law today: bash, git, go, and coreutils are the
# platform; python3 exists for EXACTLY the declared fixture sites
# (the TTY escalation driver — "the metasystem itself does not need
# it"); perl left with the conf_edit landing and must not return.
# The benchmark kit's python3 scope is a separate ruling (parked on
# records/misc/extractor-port-design.md) and is not scanned here.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"

# Interpreters that must not appear AT ALL in metasystem scripts.
banned="perl ruby php node deno"

# python3 is allowed at exactly these declared sites (file basenames);
# a new file invoking python3 must either lose it or be declared here
# in the same landing, which is the ratchet's whole point.
python3_declared="validate-metasystem.sh preflight-commands.sh dispatch-fixtures.sh"

self_test=0
[[ "${1:-}" == --self-test ]] && self_test=1

scan_tree() { # root of the tree to scan; prints violations
  local tree=$1 word file hits
  for word in $banned; do
    # COMMAND-POSITION matching, comments stripped: a bare word like
    # "node" is an everyday variable name, so a hit requires the
    # interpreter in an executable position (line start or after
    # ; & | ` $( ) followed by whitespace/end — never $word= or
    # $variable expansions. The occurrence law still holds: awk
    # reports every hit, not the first (the announce-scanner lesson).
    hits=$(find "$tree/scripts" -name '*.sh' ! -name 'dependency-ratchet.sh' -exec awk -v word="$word" '
      {
        line = $0
        sub(/[[:space:]]#.*$/, "", line)   # trailing comments
        if (line ~ /^[[:space:]]*#/) next  # full-line comments
        if (line ~ ("(^|[;&|`(\\$\\(][[:space:]]*|[[:space:]])" word "([[:space:]]|$)")             && line !~ ("\\$" word) && line !~ (word "=")) {
          print FILENAME ":" FNR ":" $0
        }
      }' {} + 2>/dev/null || true)
    [[ -z "$hits" ]] || printf 'banned interpreter %s:\n%s\n' "$word" "$hits"
  done
  hits=$(grep -rln "\bpython3\b" "$tree/scripts" 2>/dev/null \
    | grep -v "dependency-ratchet.sh" || true)
  local undeclared=""
  for file in $hits; do
    case " $python3_declared " in
      *" $(basename "$file") "*) ;;
      *) undeclared+="$file"$'\n' ;;
    esac
  done
  [[ -z "$undeclared" ]] || printf 'python3 outside the declared sites (declare in dependency-ratchet.sh or remove):\n%s' "$undeclared"
}

if (( self_test )); then
  # The check must actually catch: an injected banned interpreter and
  # an undeclared python3 site each refuse in a scratch tree.
  tmp=$(mktemp -d)
  trap 'rm -rf -- "$tmp"' EXIT
  mkdir -p "$tmp/scripts/agents"
  printf '#!/usr/bin/env bash\nperl -e 1\n' >"$tmp/scripts/agents/bad-perl.sh"
  out=$(scan_tree "$tmp")
  [[ "$out" == *"banned interpreter perl"* ]] \
    || { echo "ratchet self-test: injected perl was not caught" >&2; exit 1; }
  rm "$tmp/scripts/agents/bad-perl.sh"
  printf '#!/usr/bin/env bash\npython3 -c 1\n' >"$tmp/scripts/agents/bad-python.sh"
  out=$(scan_tree "$tmp")
  [[ "$out" == *"python3 outside the declared sites"* ]] \
    || { echo "ratchet self-test: undeclared python3 was not caught" >&2; exit 1; }
  echo "dependency ratchet self-test passed"
  exit 0
fi

violations=$(scan_tree "$root")
if [[ -n "$violations" ]]; then
  echo "dependency ratchet refused: undeclared external interpreters" >&2
  printf '%s\n' "$violations" >&2
  exit 1
fi
echo "dependency ratchet passed (declared: python3 at $(echo $python3_declared | wc -w | tr -d ' ') fixture sites; banned interpreters absent)"
