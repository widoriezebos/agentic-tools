#!/bin/bash
set -u
usage() {
  echo "usage: $0 <tag> --dir <step-dir> --lock-lib <path> [--seat <name>] [--idle <seconds>] [--poll <seconds>] [--repo <metasystem-dir>]" >&2
  exit 2
}
[ $# -ge 5 ] || usage
tag=$1
shift
step_dir=
lock_lib=
seat=$tag
idle=2400
poll=3
repo=$(cd "$(dirname "$0")/../.." && pwd -P)
while [ $# -gt 0 ]; do
  case $1 in
    --dir|--lock-lib|--seat|--idle|--poll|--repo)
      [ $# -ge 2 ] || usage
      key=$1
      value=$2
      shift 2
      ;;
    *)
      usage
      ;;
  esac
  case $key in
    --dir)
      step_dir=$value
      ;;
    --lock-lib)
      lock_lib=$value
      ;;
    --seat)
      seat=$value
      ;;
    --idle)
      idle=$value
      ;;
    --poll)
      poll=$value
      ;;
    --repo)
      repo=$value
      ;;
  esac
done
[ -n "$step_dir" ] && [ -f "$lock_lib" ] || usage
lock_lib=$(cd "$(dirname "$lock_lib")" && pwd -P)/$(basename "$lock_lib"); mkdir -p "$step_dir" || exit 1
step_dir=$(cd "$step_dir" && pwd -P) || exit 1; repo=$(cd "$repo" && pwd -P) || exit 1; rm -f "$step_dir/state" "$step_dir/quit" "$step_dir"/step-* "$step_dir/.steps"
source "$lock_lib" || exit 1
finish() { rc=$?; trap - EXIT; testrun_lock_release; echo "exited $(date -u +%FT%TZ)" >"$step_dir/state"; exit "$rc"; }
trap finish EXIT
echo "queued $(date -u +%FT%TZ)" >"$step_dir/state"
if ! testrun_lock_acquire "$seat" "landing lane: $tag" 14400; then
  echo lockfail >"$step_dir/state.fail"
  exit 1
fi
echo "locked $(date -u +%FT%TZ)" >"$step_dir/state"; last=$SECONDS
while [ $((SECONDS - last)) -lt "$idle" ]; do
  [ -e "$step_dir/quit" ] && break
  (cd "$step_dir" && find . -maxdepth 1 -type f -name 'step-*.sh' | grep -E '^\./step-[0-9]+\.sh$' | sort -t- -k2,2n) >"$step_dir/.steps"
  while IFS= read -r step; do
    name=${step#./}; step=$step_dir/$name; done_file=${name%.sh}.done; [ -e "$step_dir/$done_file" ] && continue
    echo "running ${name%.sh} $(date -u +%FT%TZ)" >"$step_dir/state"
    (cd "$repo" && bash "$step" </dev/null) >"$step_dir/${name%.sh}.out" 2>&1
    rc=$?
    echo "rc=$rc" >"$step_dir/$done_file"; echo "locked idle $(date -u +%FT%TZ)" >"$step_dir/state"; last=$SECONDS
  done <"$step_dir/.steps"
  sleep "$poll"
done
