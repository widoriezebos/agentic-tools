#!/bin/bash
set -u
usage() {
  echo "usage: $0 <tag> <goal> <page> <brief> <task-file> --out <dir> [--round <n>] [--worktree <dir>] [--model <model>]" >&2
  echo "codex runs in the foreground; --poll is not supported" >&2
  exit 2
}
[ $# -ge 7 ] || usage
tag=$1 goal=$2 page=$3 brief=$4 task_file=$5; shift 5
out= round=1 worktree= model=gpt-5.6-sol
while [ $# -gt 0 ]; do
  case $1 in --out|--round|--worktree|--model) [ $# -ge 2 ] || usage; key=$1; value=$2; shift 2;; *) usage;; esac
  case $key in --out) out=$value;; --round) round=$value;; --worktree) worktree=$value;; --model) model=$value;; esac
done
[ -n "$out" ] || usage
metasystem=$(cd "$(dirname "$0")/../.." && pwd -P); repo=$(cd "$metasystem/.." && pwd -P)
if [ -n "$worktree" ]; then case $worktree in /*) :;; *) worktree=$(pwd -P)/$worktree;; esac; else worktree=$repo/.claude/worktrees/crit-$tag; fi
[ -d "$worktree" ] || { git -C "$repo" fetch -q origin && git -C "$repo" worktree add --detach "$worktree" origin/main; } || exit 1
mkdir -p "$out" "$worktree/metasystem/plans" "$worktree/metasystem/artifacts/reports"
out=$(cd "$out" && pwd -P) || exit 1; worktree=$(cd "$worktree" && pwd -P) || exit 1
cp "$page" "$worktree/metasystem/plans/$(basename "$page")"
cp "$brief" "$worktree/metasystem/artifacts/reports/$tag-design-brief-r$round.md"
cp "$metasystem/scripts/agents/templates/design-common.md" "$worktree/metasystem/artifacts/reports/design-common.md"
task=$(cat "$task_file"; printf x); task=${task%x}
cd "$worktree/metasystem" || exit 1
codex exec -m "$model" -C "$worktree/metasystem" -s workspace-write -o "$out/$tag-crit-r$round.last" "$task" </dev/null >"$out/$tag-crit-r$round.log" 2>&1
codex_rc=$?
if [ "$codex_rc" -eq 0 ]; then status=completed; else status=failed; fi
echo "critique job: $status (codex exit code $codex_rc) at $(date -u +%H:%M:%SZ)"
report=$worktree/metasystem/artifacts/reports/$tag-critique-r$round.md
if [ -f "$report" ]; then cp "$report" "$out/$tag-critique-r$round.md"; echo "critique: $(wc -l <"$report") lines; material yes: $(grep -ciE 'material:? *yes' "$report")"; grep -iE '^VERDICT' "$report" | tail -1; else echo "no critique file"; fi
echo "$status" >"$out/$tag-crit-r$round.done"
