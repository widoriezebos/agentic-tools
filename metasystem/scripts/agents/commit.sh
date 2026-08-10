#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
token=$root/artifacts/agents/mains/worktree-commit-token.json

if [[ ${1:-} != __lease-held ]]; then
  result=$("$ms" lease require-holder --root "$root" --caller-pid "$$") || exit $?
  # --default "" collapses an absent or null claimEpoch to empty, so the
  # human-commit branch below is taken when there is no epoch.
  epoch=$("$ms" json get --value "$result" --field claimEpoch --default "")
  if [[ -n "$epoch" ]]; then
    exec "$ms" lease run-held --root "$root" --caller-pid "$$" \
      --expected-epoch "$epoch" -- "$0" __lease-held "$epoch" "$@"
  fi
  exec "$ms" lease run-held --root "$root" --caller-pid "$$" -- "$0" __lease-held human "$@"
fi
shift
expected_epoch=${1:-}
[[ -n "$expected_epoch" ]] || exit 2
shift
if [[ "$expected_epoch" =~ ^[1-9][0-9]*$ ]]; then
  "$ms" lease require-holder --root "$root" --caller-pid "$$" \
    --expected-epoch "$expected_epoch" >/dev/null
else
  [[ "$expected_epoch" == human ]] || exit 2
  "$ms" lease require-holder --root "$root" --caller-pid "$$" >/dev/null
fi

started=$("$ms" identity started-at --pid $$) || {
  echo "agent commit wrapper refused: wrapper process start time is unreadable" >&2
  exit 1
}
nonce=$("$ms" util token-hex --bytes 16)
# TODO(go-wiring): the block below atomically writes the live wrapper token
# (worktree-commit-token.json) with a computed createdAt timestamp. Pending a
# `lease commit-token` verb (state assembly, not a field read).
python3 - "$token" "$$" "$started" "$nonce" <<'PY'
import json,os,sys,tempfile
from datetime import datetime,timezone
from pathlib import Path
path,pid,started,nonce=Path(sys.argv[1]),int(sys.argv[2]),int(sys.argv[3]),sys.argv[4]
path.parent.mkdir(parents=True,exist_ok=True)
value={"wrapperPid":pid,"wrapperPidStartedAt":started,"nonce":nonce,"createdAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}
fd,temp=tempfile.mkstemp(prefix=path.name+".",suffix=".tmp",dir=path.parent)
with os.fdopen(fd,"w") as handle:
    json.dump(value,handle,sort_keys=True); handle.write("\n"); handle.flush(); os.fsync(handle.fileno())
os.replace(temp,path)
PY
trap 'rm -f -- "$token"' EXIT
git -C "$root" commit "$@"
