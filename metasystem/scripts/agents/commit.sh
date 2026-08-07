#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
lease_helper=$root/scripts/agents/worktree-lease.py
token=$root/artifacts/agents/mains/worktree-commit-token.json

if [[ ${1:-} != __lease-held ]]; then
  result=$("$lease_helper" --root "$root" require-holder --caller-pid "$$") || exit $?
  epoch=$(python3 -c 'import json,sys; v=json.loads(sys.argv[1]); print("" if v.get("claimEpoch") is None else v["claimEpoch"])' "$result")
  if [[ -n "$epoch" ]]; then
    exec "$lease_helper" --root "$root" run-held --caller-pid "$$" \
      --expected-epoch "$epoch" -- "$0" __lease-held "$epoch" "$@"
  fi
  exec "$lease_helper" --root "$root" run-held --caller-pid "$$" -- "$0" __lease-held human "$@"
fi
shift
expected_epoch=${1:-}
[[ -n "$expected_epoch" ]] || exit 2
shift
if [[ "$expected_epoch" =~ ^[1-9][0-9]*$ ]]; then
  "$lease_helper" --root "$root" require-holder --caller-pid "$$" \
    --expected-epoch "$expected_epoch" >/dev/null
else
  [[ "$expected_epoch" == human ]] || exit 2
  "$lease_helper" --root "$root" require-holder --caller-pid "$$" >/dev/null
fi

started=$($root/scripts/agents/process-census.py started-at --pid $$) || {
  echo "agent commit wrapper refused: wrapper process start time is unreadable" >&2
  exit 1
}
nonce=$(python3 -c 'import secrets; print(secrets.token_hex(16))')
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
