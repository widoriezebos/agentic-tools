#!/usr/bin/env bash
# IL-14: the mechanical form of a rule that failed twice as prose.
#
# With a peer agent session holding work in the same tree, `git add` on a
# directory once committed a peer's plan file minutes after this session
# promised to leave it alone (0b9ca1b), and the explicit-paths rule written in
# response was then violated all period by its own author. A brand-new file
# appearing under plans/ in the staged set is exactly the shape of that
# accident, so it needs an explicit, per-commit acknowledgment:
#
#   METASYSTEM_ALLOW_NEW_PLAN=1 git commit ...
#
# Modifying a tracked plan stays free; only additions are challenged, because
# only additions can smuggle in a file the committer has never read.
set -euo pipefail

guard_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
git rev-parse --show-toplevel >/dev/null 2>&1 || exit 0
lease_helper=$guard_root/scripts/agents/worktree-lease.py
if [[ -x "$lease_helper" ]]; then
  classification=
  caller_class=
  if classification=$("$lease_helper" --root "$guard_root" classify --caller-pid "$$" 2>/dev/null) \
    && caller_class=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["class"])' "$classification" 2>/dev/null); then
    :
  fi
  # Human commits are sovereign. A broken classifier cannot positively prove
  # agent ancestry, so this hook leaves the commit untouched; valid agent
  # classifications still require the live wrapper token below.
  if [[ -n "$caller_class" && "$caller_class" != HUMAN ]]; then
    token=$guard_root/artifacts/agents/mains/worktree-commit-token.json
    if ! python3 - "$token" "$guard_root/scripts/agents/process-census.py" "$$" <<'PY'
import json,subprocess,sys
from pathlib import Path
path,helper,current=Path(sys.argv[1]),sys.argv[2],int(sys.argv[3])
try: token=json.loads(path.read_text(encoding="utf-8"))
except (OSError,ValueError): raise SystemExit(1)
pid,start,nonce=token.get("wrapperPid"),token.get("wrapperPidStartedAt"),token.get("nonce")
if type(pid) is not int or type(start) is not int or not isinstance(nonce,str) or len(nonce)!=32: raise SystemExit(1)
seen=set()
while current>0 and current not in seen:
    seen.add(current)
    if current==pid:
        try: actual=int(subprocess.check_output([helper,"started-at","--pid",str(pid)],text=True,stderr=subprocess.DEVNULL).strip())
        except (OSError,ValueError,subprocess.SubprocessError): raise SystemExit(1)
        raise SystemExit(0 if actual==start else 1)
    try: current=int(subprocess.check_output(["ps","-p",str(current),"-o","ppid="],text=True,stderr=subprocess.DEVNULL).strip())
    except (OSError,ValueError,subprocess.SubprocessError): break
raise SystemExit(1)
PY
    then
      echo "pre-commit guard: agent commit requires scripts/agents/commit.sh; the live wrapper ancestry token is missing" >&2
      exit 1
    fi
  fi
fi

[[ "${METASYSTEM_ALLOW_NEW_PLAN:-}" == "1" ]] && exit 0

# An unborn branch has no peer work to capture: the initial commit stages the
# entire payload by design (adoption and provisioning both do), and refusing
# it broke provisioning the first time the two mechanisms met.
git rev-parse --verify HEAD >/dev/null 2>&1 || exit 0

added=$(git diff --cached --name-status --diff-filter=A | cut -f2- \
  | grep -E '(^|/)plans/[^/]+\.md$' || true)
[[ -z "$added" ]] && exit 0

echo "pre-commit guard: refusing to commit NEW plan file(s):" >&2
printf '  %s\n' $added >&2
echo "A new plan in the staged set is how a peer session's file gets committed" >&2
echo "by accident (0b9ca1b). If this addition is deliberate, acknowledge it:" >&2
echo "  METASYSTEM_ALLOW_NEW_PLAN=1 git commit ..." >&2
exit 1
