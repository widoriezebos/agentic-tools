#!/usr/bin/env bash
set -euo pipefail

fail() { printf 'oldest-bash-gate self-test: %s\n' "$*" >&2; exit 1; }

self_test() {
  local self scratch passed=0 root allow out err row bash_major
  self=$(cd "$(dirname "$0")" && pwd -P)/$(basename "$0")
  scratch=$(mktemp -d "${TMPDIR:-/tmp}/oldest-bash-gate.XXXXXX")
  trap 'rm -rf "$scratch"' EXIT HUP INT TERM

  new_root() { root=$scratch/$1; mkdir -p "$root/scripts/agents"; allow=$root/allow.tsv; : >"$allow"; }
  rejects() { # witness, expected text
    local witness=$1 expected=$2 status
    out=$root/out; err=$root/err
    if /bin/bash "$self" --root "$root" --allowlist "$allow" >"$out" 2>"$err"; then
      fail "$witness failed: expected refusal"
    else
      status=$?
    fi
    test "$status" -eq 1 || fail "$witness failed: expected status 1, got $status"
    grep -F "$expected" "$err" >/dev/null || fail "$witness failed: missing $expected"
    passed=$((passed + 1))
  }
  accepts() { # witness
    local witness=$1
    out=$root/out; err=$root/err
    if ! /bin/bash "$self" --root "$root" --allowlist "$allow" >"$out" 2>"$err"; then
      fail "$witness failed: unexpected refusal: $(tr '\n' ' ' <"$err")"
    fi
    test ! -s "$err" || fail "$witness failed: unexpected stderr"
    passed=$((passed + 1))
  }

  new_root w1
  printf '%s\n' '#!/usr/bin/env bash' 'set -e' ')' >"$root/scripts/agents/w1-fixtures.sh"
  rejects W1 'scripts/agents/w1-fixtures.sh:3: syntax:'

  new_root w1b
  printf '%s\n' '#!/usr/bin/env bash' 'set -e' 'echo x |& cat' >"$root/scripts/agents/w1-fixtures.sh"
  bash_major=$(/bin/bash -c 'printf "%s\n" "${BASH_VERSINFO[0]}"')
  # The native syntax gate rejects |& in Bash 3 and accepts it beginning with Bash 4.
  if test "$bash_major" -eq 3; then
    rejects W1b 'scripts/agents/w1-fixtures.sh:3: syntax:'
  elif test "$bash_major" -ge 4; then
    accepts W1b
  else
    fail "W1b failed: unsupported /bin/bash major $bash_major"
  fi

  new_root w2a
  printf '%s\n' '#!/usr/bin/env bash' 'set -eu' 'wait_stop_real_runtime=()' \
    'if [[ "${METASYSTEM_REAL_RUNTIME_BEDS:-0}" == 1 ]]; then' \
    '  wait_stop_real_runtime+=(wait-stop-claude)' 'fi' \
    'printf "%s\n" "${wait_stop_real_runtime[@]}"' >"$root/scripts/agents/w2-fixtures.sh"
  rejects W2a 'scripts/agents/w2-fixtures.sh:7: empty-array:'

  new_root w2b
  printf '%s\n' '#!/usr/bin/env bash' 'set -eu' 'wait_stop_real_runtime=()' \
    'if [[ "${METASYSTEM_REAL_RUNTIME_BEDS:-0}" == 1 ]]; then' \
    '  wait_stop_real_runtime+=(wait-stop-claude)' 'fi' \
    'printf "%s\n" ${wait_stop_real_runtime[@]+"${wait_stop_real_runtime[@]}"}' >"$root/scripts/agents/w2-fixtures.sh"
  accepts W2b

  new_root w2c
  printf '%s\n' '#!/usr/bin/env bash' 'set -u' 'x=()' \
    'y=("${x[@]+"${x[@]}"}")' 'printf "%s" "${x[@]}"' >"$root/scripts/agents/w2c-fixtures.sh"
  rejects W2c 'scripts/agents/w2c-fixtures.sh:5: empty-array:'
  if grep -F 'scripts/agents/w2c-fixtures.sh:4: empty-array:' "$err" >/dev/null; then fail 'W2c failed: guarded line 4 was reported'; fi

  new_root w3a
  printf '%s\n' '#!/usr/bin/env bash' 'x=x' '[[ -n "$x" ]]' >"$root/scripts/agents/w3-fixtures.sh"
  rejects W3a 'scripts/agents/w3-fixtures.sh:3: bare-test:'

  new_root w3b
  printf '%s\n' '#!/usr/bin/env bash' 'count=1' '(( count > 0 ))' >"$root/scripts/agents/w3-fixtures.sh"
  rejects W3b 'scripts/agents/w3-fixtures.sh:3: bare-test:'

  new_root w3c
  printf '%s\n' '#!/usr/bin/env bash' '[[ a ]] || exit 1' \
    'if [[ a ]]; then :; fi' 'x=$(( 1 + 2 ))' 'true &&' '  [[ a ]]' \
    'if' '  [[ a ]]' 'then :; fi' >"$root/scripts/agents/w3-fixtures.sh"
  accepts W3c

  new_root w4a
  printf '%s\n' '#!/usr/bin/env bash' 'x=x' '[[ -n "$x" ]]' >"$root/scripts/agents/first-fixtures.sh"
  /bin/bash "$self" --root "$root" --allowlist "$allow" --list >"$allow" 2>"$root/list.err" || fail 'W4a failed: could not build row'
  accepts W4a

  new_root w4b
  printf '%s\n' '#!/usr/bin/env bash' 'x=x' '[[ -n "$x" ]]' '[[ -n "$x" ]]' >"$root/scripts/agents/first-fixtures.sh"
  /bin/bash "$self" --root "$root" --allowlist "$allow" --list | sed -n '1p' >"$root/one-row"
  mv "$root/one-row" "$allow"
  rejects W4b 'scripts/agents/first-fixtures.sh:4: bare-test:'

  new_root w4c
  printf '%s\n' '#!/usr/bin/env bash' 'x=x' '[[ -n "$x" ]]' >"$root/scripts/agents/first-fixtures.sh"
  /bin/bash "$self" --root "$root" --allowlist "$allow" --list >"$allow"
  printf '%s\n' '#!/usr/bin/env bash' 'x=x' >"$root/scripts/agents/first-fixtures.sh"
  printf '%s\n' '#!/usr/bin/env bash' 'x=x' '[[ -n "$x" ]]' >"$root/scripts/agents/second-fixtures.sh"
  rejects W4c 'scripts/agents/second-fixtures.sh:3: bare-test:'

  new_root w5
  git -C "$root" init -q
  root=$(cd "$root" && pwd -P); allow=$root/allow.tsv
  printf '%s\n' '#!/bin/bash' 'x=1' '[[ -n "$x" ]]' 'echo x |& cat' >"$root/scripts/agents/z-fixtures.sh"
  rejects W5 "$root: no tracked shell scripts found under scripts/"

  rm -rf "$scratch"
  trap - EXIT HUP INT TERM
  printf 'oldest-bash-gate self-test: %s witnesses passed\n' "$passed"
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
allowlist=
list=0
while (($#)); do
  case "$1" in
    --root) (($# >= 2)) || { printf 'oldest-bash-gate: --root needs a directory\n' >&2; exit 2; }; root=$2; shift 2 ;;
    --allowlist) (($# >= 2)) || { printf 'oldest-bash-gate: --allowlist needs a file\n' >&2; exit 2; }; allowlist=$2; shift 2 ;;
    --list) list=1; shift ;;
    --self-test) self_test; exit ;;
    *) printf 'oldest-bash-gate: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
done
test -x /bin/bash || { printf 'oldest-bash-gate: /bin/bash is missing or not executable\n' >&2; exit 1; }
root=$(cd "$root" && pwd -P)
allowlist=${allowlist:-$root/scripts/agents/oldest-bash-allowlist.tsv}
tmp=$(mktemp -d "${TMPDIR:-/tmp}/oldest-bash-gate.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
shells=$tmp/shells fixtures=$tmp/fixtures syntax=$tmp/syntax meta=$tmp/meta
: >"$syntax"; : >"$meta"

if git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  git -C "$root" ls-files 'scripts/*.sh' 'scripts/**/*.sh' >"$shells"
  git -C "$root" ls-files 'scripts/*fixture*.sh' 'scripts/**/*fixture*.sh' >"$fixtures"
else
  (cd "$root" && find scripts -type f -name '*.sh' -print | LC_ALL=C sort) >"$shells"
  (cd "$root" && find scripts -type f -name '*fixture*.sh' -print | LC_ALL=C sort) >"$fixtures"
fi
enumeration_failed=0
test -s "$shells" || { printf 'oldest-bash-gate: %s: no tracked shell scripts found under scripts/\n' "$root" >&2; enumeration_failed=1; }
test -s "$fixtures" || { printf 'oldest-bash-gate: %s: no tracked fixture files found under scripts/\n' "$root" >&2; enumeration_failed=1; }
((enumeration_failed == 0)) || exit 1

while IFS= read -r path; do
  if ! /bin/bash -n "$root/$path" 2>"$tmp/bash.err"; then
    line=0 message='syntax check failed'
    while IFS= read -r bash_message; do
      if [[ "$line" == 0 && "$bash_message" =~ line[[:space:]]+([0-9]+):[[:space:]]*(.*) ]]; then
        line=${BASH_REMATCH[1]}; message=${BASH_REMATCH[2]}
      fi
    done <"$tmp/bash.err"
    printf '%s\t%s\tsyntax\t%s\n' "$path" "$line" "$message" >>"$syntax"
  fi
done <"$shells"

fixture_files=()
while IFS= read -r path; do fixture_files+=("$root/$path"); done <"$fixtures"
cat >"$tmp/scan.awk" <<'AWK'
function trim(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s); return s }
function delimiter_delta(s, opening, closing, i,c,d) {
  d=0
  for (i=1;i<=length(s);i++) { c=substr(s,i,1); if (scan_quote!="") { if (c==scan_quote) scan_quote=""; else if (scan_quote=="\"" && c=="\\") i++; continue }
    if (c=="\"" || c=="\047") { scan_quote=c; continue } else if (c=="\\") { i++; continue } else if (c=="#" && (i==1 || substr(s,i-1,1) ~ /[[:space:]]/)) break
    if (substr(s,i,2)==opening) { d++; i++ } else if (substr(s,i,2)==closing) { d--; i++ }
  } return d
}
function add(rule,path,line,identity,detail, content,file) {
  found++; file=identity_dir "/" sprintf("%06d",found); printf "%s", identity >file; close(file)
  content=identity; gsub(/\t/," ",content)
  printf "%06d\t%s\t%s\t%d\t%s\t%s\n",found,rule,path,line,detail,content >>meta_file
}
function condition_context(s) {
  s=trim(s); sub(/[[:space:]]+#[^\n]*$/, "", s); s=trim(s); sub(/;[[:space:]]*$/, "", s); s=trim(s)
  return s ~ /(&&|\|\||\|)[[:space:]]*$/ || s ~ /(^|[;[:space:]])(!|if|elif|while|until)[[:space:]]*$/
}
function finish_statement(path,first,text,identity,previous, bare) {
  bare=trim(text); sub(/[[:space:]]+#[^\n]*$/, "", bare); bare=trim(bare); sub(/;[[:space:]]*$/, "", bare); bare=trim(bare)
  if (!condition_context(previous) && ((bare ~ /^\[\[/ && bare ~ /\]\]$/) || (bare ~ /^\(\(/ && bare ~ /\)\)$/)))
    add("bare-test",path,first,identity,"bash 3.2 errexit ignores a false [[ ]] or (( )) statement; write it as a check that fails (for example [[ ... ]] || fixture_check_failed ...)")
}
{
  if (!(FILENAME in seen_file)) { seen_file[FILENAME]=1; files[++file_count]=FILENAME }
  lines[FILENAME SUBSEP FNR]=$0; max[FILENAME]=FNR
  if ($0 ~ /^[[:space:]]*#/) next
  rest=$0
  while (match(rest,/[A-Za-z_][A-Za-z0-9_]*[[:space:]]*=[[:space:]]*\(\)/)) {
    word=substr(rest,RSTART,RLENGTH); sub(/[[:space:]]*=.*/,"",word); empty[FILENAME SUBSEP word]=1
    rest=substr(rest,RSTART+RLENGTH)
  }
  declaration=trim($0)
  if (declaration ~ /^(local|declare)[[:space:]]+-a[[:space:]]+/) {
    sub(/^(local|declare)[[:space:]]+-a[[:space:]]+/,"",declaration); n=split(declaration,words,/[[:space:]]+/)
    for (i=1;i<=n;i++) { sub(/;$/,"",words[i]); if (words[i] ~ /^#/) break; if (words[i] ~ /^[A-Za-z_][A-Za-z0-9_]*$/) empty[FILENAME SUBSEP words[i]]=1 }
  }
}
END {
  for (fi=1;fi<=file_count;fi++) {
    file=files[fi]; path=file; if (substr(path,1,length(root)+1)==root "/") path=substr(path,length(root)+2); statement=identity=previous=""; square=arithmetic=first=exp_depth=quote_id=next_quote_id=0; quote_char=""
    for (ln=1;ln<=max[file];ln++) {
      physical=lines[file SUBSEP ln]
      if (physical !~ /^[[:space:]]*#/) {
        if (exp_depth==0) { quote_char=""; quote_id=0 }
        for (position=1;position<=length(physical);position++) {
          character=substr(physical,position,1)
          if (quote_char=="\047") { if (character=="\047") { quote_char=""; quote_id=0 } continue }
          if (character=="\\") { position++; continue }
          if (character=="\"") { if (quote_char=="\"" && exp_depth>0 && quote_id==exp_quote[exp_depth]) { quote_parent[++next_quote_id]=quote_id; quote_id=next_quote_id } else if (quote_char=="\"") { quote_id=quote_parent[quote_id]; if (!quote_id) quote_char="" } else { quote_char="\""; quote_parent[++next_quote_id]=quote_id; quote_id=next_quote_id } continue }
          if (character=="\047" && quote_char=="") { quote_char="\047"; quote_id=++next_quote_id; continue }
          rest=substr(physical,position)
          if (match(rest,/^\$\{[A-Za-z_][A-Za-z0-9_]*\[[@*]\]\}/)) {
            expression=substr(rest,RSTART,RLENGTH); name=expression; sub(/^\$\{/ ,"",name); sub(/\[[@*]\]\}$/,"",name); guarded=0
            for (depth=1;depth<=exp_depth;depth++) if (guard_name[depth]==name) guarded=1
            if ((file SUBSEP name) in empty && !guarded) {
            subscript=(expression ~ /\[@\]/ ? "@" : "*"); guard="${" name "[" subscript "]+\"${" name "[" subscript "]}\"}"
            add("empty-array",path,ln,trim(physical),expression " expands an array this file can leave empty; bash 3.2 under set -u dies with " name "[" subscript "]: unbound variable; write " guard)
            }
            position+=RLENGTH-1; continue
          }
          if (match(rest,/^\$\{[A-Za-z_][A-Za-z0-9_]*\[[@*]\]\+/)) { guard_name[++exp_depth]=substr(rest,1,RLENGTH); sub(/^\$\{/ ,"",guard_name[exp_depth]); sub(/\[[@*]\]\+$/,"",guard_name[exp_depth]); exp_quote[exp_depth]=quote_id; position++; continue }
          if (exp_depth>0 && substr(rest,1,2)=="${") { guard_name[++exp_depth]=""; exp_quote[exp_depth]=quote_id; position++; continue }
          if (character=="}" && exp_depth>0 && exp_quote[exp_depth]==quote_id) { delete guard_name[exp_depth]; delete exp_quote[exp_depth]; exp_depth-- }
        }
      }
      if (physical ~ /^[[:space:]]*#/) continue
      piece=trim(physical); if (statement=="" && piece=="") continue
      if (statement=="") { first=ln; identity=piece; statement=piece; scan_quote="" } else { identity=identity "\\n" piece; statement=statement "\n" piece }
      if (statement ~ /^\[\[/) square+=delimiter_delta(physical,"[[","]]" ); else if (statement ~ /^\(\(/) arithmetic+=delimiter_delta(physical,"((","))")
      if (piece !~ /\\$/ && square<=0 && arithmetic<=0) {
        finish_statement(path,first,statement,identity,previous); previous=statement; statement=identity=""; square=arithmetic=first=0
      }
    }
    if (statement!="") finish_statement(path,first,statement,identity,previous)
  }
}
AWK

if ((${#fixture_files[@]})); then
  LC_ALL=C awk -v root="$root" -v identity_dir="$tmp" -v meta_file="$meta" -f "$tmp/scan.awk" \
    "${fixture_files[@]+"${fixture_files[@]}"}"
fi

catalog=$tmp/catalog
: >"$catalog"
if [[ -s "$meta" ]]; then
  identity_files=()
  while IFS=$'\t' read -r id rest; do identity_files+=("$tmp/$id"); done <"$meta"
  shasum -a 256 "${identity_files[@]+"${identity_files[@]}"}" >"$tmp/hashes"
  awk '{ file=$2; sub(/^.*\//,"",file); print file "\t" $1 }' "$tmp/hashes" >"$tmp/hash-map"
  awk -F '\t' 'NR==FNR { hash[$1]=$2; next } { print $2 FS $3 FS hash[$1] FS $4 FS $5 FS $6 }' \
    "$tmp/hash-map" "$meta" >"$catalog"
fi

if ((list)); then
  awk -F '\t' '{ print $1 FS $2 FS $3 FS $6 }' "$catalog" | LC_ALL=C sort
  [[ ! -s "$syntax" ]] || { while IFS=$'\t' read -r path line rule message; do printf 'oldest-bash-gate: %s:%s: %s: %s\n' "$path" "$line" "$rule" "$message" >&2; done <"$syntax"; exit 1; }
  exit 0
fi

test -f "$allowlist" || { printf 'oldest-bash-gate: allowlist is missing: %s\n' "$allowlist" >&2; exit 1; }
unallowed=$tmp/unallowed
: >"$unallowed"
awk -F '\t' -v output="$unallowed" '
  FILENAME==ARGV[1] { if ($0=="" || substr($0,1,1)=="#") next; key=$1 FS $2 FS $3; allowed[key]++; rows[++row_count]=key; next }
  { key=$1 FS $2 FS $3; seen[key]++; if (seen[key] <= allowed[key]) used[key]++; else print $2 FS $4 FS $1 FS $5 >output }
  END { for (i=1;i<=row_count;i++) { key=rows[i]; if (used[key]>0) used[key]--; else { split(key,v,FS); print "oldest-bash-gate: note: stale allowlist row " v[1] " " v[2] " " v[3] >"/dev/stderr" } } }
' "$allowlist" "$catalog"
errors=$tmp/errors
cat "$syntax" "$unallowed" >"$errors"
if [[ -s "$errors" ]]; then
  LC_ALL=C sort -t $'\t' -k1,1 -k2,2n -k3,3 "$errors" | while IFS=$'\t' read -r path line rule message; do
    printf 'oldest-bash-gate: %s:%s: %s: %s\n' "$path" "$line" "$rule" "$message" >&2
  done
  exit 1
fi
