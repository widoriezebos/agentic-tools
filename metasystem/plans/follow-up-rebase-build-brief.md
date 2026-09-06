Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal delegate-follow-up-cannot-merge-main)
Date: 2026-09-06

# Goal

Goal delegate-follow-up-cannot-merge-main (tier 2, approved by Wido on
2026-09-06). Its record,
metasystem/plans/goals/delegate-follow-up-cannot-merge-main.md, is the
contract. In short: when main moves on a chain's own files while a
round runs, the next follow-up round cannot bring its worktree up to
date. The dispatcher only warns (WORKTREE-BEHIND, in
metasystem/scripts/agents/dispatch.sh near line 1847), git refuses a
merge over the round's uncommitted changes, and no lawful checkpoint
commit exists: the pre-commit guard admits only wrapper-token commits
and the wrapper is the landing lane, which a delegate in its sandbox
cannot even reach.

# Facts (read and ran on 2026-09-06)

- Worktrees are cut from the target checkout's HEAD and never commit,
  so a worktree's HEAD is always an ancestor of main: a fast-forward is
  always possible once the round's changes are set aside.
- Twice today the orchestrator did by hand what works: `git stash push
  -u -m <tag>` in the worktree (capturing the entry's hash), `git merge
  --ff-only main`, `git stash apply <hash>`; four of five files
  auto-merged each time and one file was left with conflict markers,
  which the next round's builder resolved as its first task. No commit
  was made anywhere. The shared stash stack is a hazard: entries must be
  tagged, applied by hash, and dropped by tag afterwards.
- The follow-up brief authority check runs against the worktree's tree,
  so a brief citing a file landed after the cut is refused until the
  worktree carries it; the rebase fixes that too.
- The chain's touched paths are the union of every round's diffBoundary
  (the conformance rule) plus the worktree's dirty paths.

# Decisions (the orchestrator's; decided, not open)

D1. The dispatcher rebases before a follow-up round. In the follow-up
path, after the chain-closed check and before the brief authority
check: if the worktree is behind the trunk AND the commits it is behind
touch any of the chain's paths (the union above), the dispatcher sets
the round's changes aside with a tagged stash (`<chain>-<round>-rebase`,
untracked files included), fast-forwards the worktree to the trunk,
re-applies the stash by its hash, and drops the entry by tag. Conflict
markers, if any, stay in the worktree; the dispatcher records on the
new round's record `rebasedFrom`, `rebasedTo` and `conflictedPaths`,
and prepends to the delivered brief one paragraph naming the trunk
commit and each conflicted path with "resolve first, keeping both
sides' behaviour". If the fast-forward is not possible or the stash
apply fails outright, the dispatcher restores the worktree (apply the
stash back onto the original HEAD) and refuses the follow-up with a
message naming the cause; a stash entry is never left behind.

D2. If the worktree is behind but no touched path moved, nothing
changes but the warning text, which then says so ("behind N commits,
none on this chain's files").

D3. The decision lives in Go, the plumbing in the script: add a verb
under the job family (for example `job follow-up-rebase-plan --root
<chain> --worktree <path> --trunk <ref>`) that computes the union of
paths, the behind count, the overlap, and prints the plan as JSON
(rebase yes or no, the reason, the overlapping paths); the script
performs the git steps only when the plan says so and records the
outcome through the existing record verbs. The Go side has unit tests;
the script side has the fixture in D5.

D4. Two stale files are deleted in this chain because they sit behind
the tier-1 floor of the litter goal that removed their sibling:
metasystem/internal/dispatch/finding_register.go.orig and
metasystem/internal/dispatch/record.go.orig. Nothing else changes for
them.

D5. Pins. Go tests for the plan verb: behind with overlap, behind
without overlap, not behind, unreadable rounds. In
metasystem/scripts/agents/dispatch-fixtures.sh, one scenario: a chain
whose first round changes a file, then a commit on the trunk that
changes the same file differently, then a follow-up: the round starts
with the worktree at the trunk's tip, the record names the conflicted
path, the delivered brief carries the paragraph, and no stash entry
remains; and one scenario where the trunk moved on another file only:
no rebase, the warning names it.

D6. Non-goals: no change to the pre-commit guard or the commit wrapper;
no checkpoint commits on agent branches; no change to conformance.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/dispatch/ -count=1`;
`bash -n scripts/agents/dispatch.sh`. Say so if the dispatch fixture bed
cannot run in the sandbox; the orchestrator reruns it.

# Constraints

Wall-clock budget: 60 minutes. DESIGN-BEARING reach: a dispatcher law;
a code critic reviews the tree next. Declare the boundary as every file
that differs from main. Gap rule: stop and report a gap with your
proposed contract written out; never fill it silently.
