#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/receipt.sh add --type <implement|refactor|improve|review|design|investigate|other> \
      --outcome <shipped|reworked|blocked|parked> [--skills a,b] \
      [--verify clean|caught|skipped] [--corrections N] [--stop-loss yes|no] \
      [--note "text"] [--file <path>]
  scripts/receipt.sh check [--max-age-days N] [--max-receipts N] [--file <path>]
  scripts/receipt.sh stats [--all] [--file <path>]
  scripts/receipt.sh retro "summary of instruction changes" [--file <path>]

add: append one task receipt line at completion — the evidence base for
tuning the harness from real use.
check: exit 1 when a harness retro is due (receipts or age since the last
retro exceed the backstop), 0 otherwise.
stats: print the period numbers (since the last retro, or --all) as
key=value lines — recorded per retro so periods stay comparable.
retro: record that a retro ran and reset the cadence.

Defaults: --file plans/receipts.log, --max-age-days 30 (override with
HARNESS_RETRO_MAX_AGE_DAYS), --max-receipts 25 (HARNESS_RETRO_MAX_RECEIPTS).
Exit codes: 0 ok; 1 retro due; 2 usage error.
USAGE
}

file=plans/receipts.log
max_age_days=${HARNESS_RETRO_MAX_AGE_DAYS:-30}
max_receipts=${HARNESS_RETRO_MAX_RECEIPTS:-25}
type= outcome= skills=none verify=skipped corrections=0 stop_loss=no note= summary= all_flag=0

cmd=${1:-}
[[ -n "$cmd" ]] || { usage; exit 2; }
shift

if [[ "$cmd" == retro && $# -gt 0 && "$1" != --* ]]; then
  summary=$1
  shift
fi

while (($#)); do
  case "$1" in
    --file) file=${2:-}; shift 2 ;;
    --type) type=${2:-}; shift 2 ;;
    --outcome) outcome=${2:-}; shift 2 ;;
    --skills) skills=${2:-}; shift 2 ;;
    --verify) verify=${2:-}; shift 2 ;;
    --corrections) corrections=${2:-}; shift 2 ;;
    --stop-loss) stop_loss=${2:-}; shift 2 ;;
    --note) note=${2:-}; shift 2 ;;
    --all) all_flag=1; shift ;;
    --max-age-days) max_age_days=${2:-}; shift 2 ;;
    --max-receipts) max_receipts=${2:-}; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

now_epoch=$(date -u +%s)
now_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)

case "$cmd" in
  add)
    case "$type" in implement|refactor|improve|review|design|investigate|other) ;; *) echo "invalid --type: $type" >&2; exit 2 ;; esac
    case "$outcome" in shipped|reworked|blocked|parked) ;; *) echo "invalid --outcome: $outcome" >&2; exit 2 ;; esac
    case "$verify" in clean|caught|skipped) ;; *) echo "invalid --verify: $verify" >&2; exit 2 ;; esac
    case "$stop_loss" in yes|no) ;; *) echo "invalid --stop-loss: $stop_loss" >&2; exit 2 ;; esac
    [[ "$corrections" =~ ^[0-9]+$ ]] || { echo "invalid --corrections: $corrections" >&2; exit 2; }
    mkdir -p "$(dirname "$file")"
    printf '%s|%s|RECEIPT|type=%s|outcome=%s|skills=%s|verify=%s|corrections=%s|stop_loss=%s|note=%s\n' \
      "$now_epoch" "$now_utc" "$type" "$outcome" "${skills//|/;}" "$verify" "$corrections" "$stop_loss" "${note//|/;}" >>"$file"
    echo "receipt recorded in $file"
    "$0" check --file "$file" >/dev/null 2>&1 || echo "note: a harness retro is due — run skills/retro (scripts/receipt.sh check for details)" >&2
    ;;
  retro)
    [[ -n "$summary" ]] || { echo "retro requires a summary of the instruction changes made" >&2; exit 2; }
    mkdir -p "$(dirname "$file")"
    printf '%s|%s|RETRO|note=%s\n' "$now_epoch" "$now_utc" "${summary//|/;}" >>"$file"
    echo "retro recorded; cadence reset"
    ;;
  stats)
    [[ -f "$file" ]] || { echo "receipts=0"; exit 0; }
    if (( all_flag )); then
      segment=$(cat "$file")
    else
      last_retro=$(grep -n '|RETRO|' "$file" | tail -1 | cut -d: -f1 || true)
      if [[ -n "$last_retro" ]]; then
        segment=$(tail -n +$((last_retro + 1)) "$file")
      else
        segment=$(cat "$file")
      fi
    fi
    printf '%s\n' "$segment" | awk -F'|' '
      $3 == "RECEIPT" {
        n++
        if (first == 0) first = $1
        last = $1
        for (i = 4; i <= NF; i++) {
          eq = index($i, "=")
          k = substr($i, 1, eq - 1); v = substr($i, eq + 1)
          if (k == "outcome") out[v]++
          else if (k == "type") typ[v]++
          else if (k == "corrections") corr += v
          else if (k == "verify" && v == "caught") caught++
          else if (k == "stop_loss" && v == "yes") sl++
        }
      }
      END {
        printf "receipts=%d\n", n
        no = split("shipped reworked blocked parked", oo, " ")
        for (i = 1; i <= no; i++) if (oo[i] in out) printf "outcome_%s=%d\n", oo[i], out[oo[i]]
        nt = split("implement refactor improve review design investigate other", tt, " ")
        for (i = 1; i <= nt; i++) if (tt[i] in typ) printf "type_%s=%d\n", tt[i], typ[tt[i]]
        printf "corrections=%d\n", corr
        printf "caught_by_verify=%d\n", caught
        printf "stop_loss_triggered=%d\n", sl
        if (n > 0) printf "span_days=%.1f\n", (last - first) / 86400
      }'
    ;;
  check)
    [[ -f "$file" ]] || { echo "no receipts recorded yet"; exit 0; }
    last_retro=$(grep -n '|RETRO|' "$file" | tail -1 | cut -d: -f1 || true)
    if [[ -n "$last_retro" ]]; then
      since=$(tail -n +$((last_retro + 1)) "$file")
      ref_epoch=$(sed -n "${last_retro}p" "$file" | cut -d'|' -f1)
    else
      since=$(cat "$file")
      ref_epoch=$(head -1 "$file" | cut -d'|' -f1)
    fi
    [[ "$ref_epoch" =~ ^[0-9]+$ ]] || { echo "receipts file is malformed: $file" >&2; exit 2; }
    receipts=$(grep -c '|RECEIPT|' <<<"$since" || true)
    age_days=$(( (now_epoch - ref_epoch) / 86400 ))
    if (( receipts > max_receipts )); then
      echo "harness retro due: $receipts receipts since the last retro (max $max_receipts)" >&2
      exit 1
    fi
    if (( age_days > max_age_days )); then
      echo "harness retro due: $age_days days since the last retro (max $max_age_days)" >&2
      exit 1
    fi
    echo "retro not due: $receipts receipts, $age_days days since last retro"
    ;;
  *)
    usage
    exit 2
    ;;
esac
