Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-implementer-read-roots-writable, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

A claude implementer delegate gets its extra read roots (the live
repository root and everything in the record's requested readRoots
beyond its worktree) as --add-dir arguments (BuildClaudeCommand and
ClaudeReadRoots in metasystem/internal/adapter/claude.go), and the
claude sandbox treats every added directory as a writable working
directory: on 2026-09-06 (goal code-critic-runtime-has-no-shell) a live
probe showed Bash touching a file inside an --add-dir root with
sandbox.filesystem.allowWrite empty. An implementer's settings allow
Bash, so its shell can write the live checkout, not only its worktree;
the Edit and Write tools respect allowWrite, Bash does not. The same
goal landed (07d18614) sandbox.filesystem.denyWrite for the critic set
(workspace root plus read roots) and proved the sandbox honours it.

When you are done, every claude delegate's settings deny writes to
every read root that is not a write root, and to the workspace root
when it is not a write root; a settings test pins it; the critic
shape is unchanged.

# The design

1. BuildClaudeSettings (metasystem/internal/adapter/claude.go): build
   the denyWrite list for EVERY role, not only the critic set: the
   record's workspaceRoot plus every requested readRoot (the list
   ClaudeReadRoots produces), minus any path that equals or lies
   under a requested writeRoot, resolved and deduplicated as today.
   For the critic set with empty write roots the result is exactly
   today's list (workspace plus read roots). For an implementer whose
   workspace is its worktree and whose write roots name that
   worktree, the list is the read roots outside it (the live root).
   Emit the denyWrite key only when the list is non-empty.
2. The scratch directory stays in allowWrite for every role; nothing
   else in the settings, the argv or the adapter changes.
3. The comment where denyWrite is built says why: the sandbox makes
   cwd and every --add-dir writable, so the settings must deny what
   the envelope only meant as readable.

# Tests

In metasystem/internal/adapter/runtime_test.go: the existing critic
settings test keeps its assertions; add an implementer case: write
root = the worktree, read roots = the worktree plus the live root,
workspace = the worktree; assert denyWrite = [the live root] and
allowWrite = [worktree, scratch]; and a case where a read root lies
under the write root, asserting it is not denied. Change no existing
assertion.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/adapter/claude.go
May touch: metasystem/internal/adapter/runtime_test.go
Must not touch: the command builder, the adapters' shell scripts, adjudicate.go, anything under plans.

# Constraints

- Never weaken a test. One round, at most 45 minutes of wall clock.
  Hazard DESIGN-BEARING: a permission envelope changes; an
  independent critique follows, and the orchestrator proves the live
  envelope by dispatching a claude implementer probe from the landed
  engine (a touch in the live root must be refused, a touch in its
  worktree must succeed).

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/adapter` (expected: ok)
- `go vet ./internal/adapter` and `gofmt -l ./internal/adapter` (expected: clean)
- `git diff --stat` (expected: only the two May-touch files)

# Acceptance Criteria

1. An implementer record yields denyWrite naming the read roots
   outside its write roots; a critic record yields today's list.
2. Tests pin both; no existing assertion changed.

# Gap Rule

stop and report a gap; never fill it silently.
