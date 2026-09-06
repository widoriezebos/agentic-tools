Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-implementer-read-roots-writable, tier 3, hazard DESIGN-BEARING, fold round two of chain cir-build1)
Date: 2026-09-06

# Goal

Round one built metasystem/plans/claude-implementer-read-roots-writable-brief.md
(tree d1969a3e). Before any critique, the orchestrator launched an
implementer probe with the candidate engine's own settings and argv
(a real implementer record whose write root is its job worktree
under the repository, cwd that worktree): the touch in the live root
was refused as intended, but the touch in the implementer's OWN
WORKTREE was refused too ("Operation not permitted"), and the scratch
directory stayed writable. The claude sandbox lets a denyWrite entry
win over an allowWrite entry nested under it. Since the fleet's job
worktrees live under the repository root
(artifacts/agents/worktrees/<job>), denying the repository root
denies every implementer its worktree. Round one cannot ship.

When you are done, an implementer's worktree is writable, everything
else under its read roots that is not an ancestor of a write root is
denied, and a live probe shows a touch in the worktree succeeding and
a touch anywhere else in the repository refused.

# The fold

1. In BuildClaudeSettings (metasystem/internal/adapter/claude.go),
   replace the denyWrite construction: for each candidate root (the
   workspace root and every read root, resolved, deduplicated):
   - if no write root lies at or under it, deny the root itself (as
     today);
   - if a write root lies under it, do not deny the root; instead
     deny each immediate child of that root (files and directories,
     from a directory read) that is not an ancestor of, or equal to,
     any write root; then repeat for the child that is such an
     ancestor, down to the write root's parent. The write root itself
     and its ancestors are never denied; every other entry along the
     way is.
   - a root equal to or under a write root is dropped (as today).
   Say in the comment what this leaves open: a delegate can still
   create NEW entries directly inside an ancestor directory of its
   worktree (the repository root, metasystem/, artifacts/, agents/,
   worktrees/), since those directories themselves must stay writable
   for the nested allow to work; every existing entry there is
   denied. Emit the key only when non-empty. Read directory entries
   with os.ReadDir; an unreadable ancestor is an error, not a silent
   gap.
2. The critic shape (no write roots) is unchanged: the roots
   themselves are denied.
3. Tests in metasystem/internal/adapter/runtime_test.go: build a
   temporary tree root/{a.txt, metasystem/{go.mod, artifacts/agents/worktrees/{job1, job2}}}
   with write root job1 and read roots [root]; assert denyWrite
   contains root/a.txt, root/metasystem/go.mod, root/metasystem/artifacts/agents/worktrees/job2
   and none of root, root/metasystem, .../artifacts, .../agents,
   .../worktrees, job1; keep round one's critic case and the
   nested-read-root case; change no existing assertion.

# Workspace

The same job worktree, on top of round one.
May touch: metasystem/internal/adapter/claude.go
May touch: metasystem/internal/adapter/runtime_test.go
Must not touch: the command builder, the adapters' shell scripts, dispatch.sh, anything under plans.

# Constraints

- Never weaken a test. One round, at most 45 minutes. The
  orchestrator repeats the live implementer probe with your engine.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/adapter` (expected: ok)
- `go vet ./internal/adapter` and `gofmt -l ./internal/adapter` (expected: clean)
- `git diff --stat` (expected: only the two May-touch files)

# Acceptance Criteria

1. An implementer record with a worktree under the repository root
   yields denyWrite naming the repository's entries around the
   worktree's ancestor chain, never an ancestor or the worktree.
2. A critic record yields today's list; tests pin both.

# Gap Rule

stop and report a gap; never fill it silently.
