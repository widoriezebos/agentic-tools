#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
token=$root/artifacts/agents/mains/worktree-commit-token.json

if [[ ${1:-} != __lease-held ]]; then
  result=$("$ms" lease require-holder --root "$root" --caller-pid "$$") || exit $?
  # TODO(go-wiring): `metasystem json get --field claimEpoch` prints the literal
  # "null" (rc 0) for a null field, but this reader collapses null -> "" so the
  # human-commit branch below is taken. json get is NOT identical to this reader,
  # so leave it until a null-collapsing read (or empty-on-null json get) exists.
  epoch=$(python3 -c 'import json,sys; v=json.loads(sys.argv[1]); print("" if v.get("claimEpoch") is None else v["claimEpoch"])' "$result")
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
# TODO(go-wiring): random nonce generation is not in the CLI map; no binary verb
# emits a token_hex nonce, so this stays python3 until one exists.
nonce=$(python3 -c 'import secrets; print(secrets.token_hex(16))')
# TODO(go-wiring): the block below assembles and atomically writes the live
# wrapper token (worktree-commit-token.json) with a computed createdAt timestamp.
# This is state assembly, not a JSON field read, and no binary verb writes this
# token; it stays python3 until a `commit-token write` verb exists.
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
