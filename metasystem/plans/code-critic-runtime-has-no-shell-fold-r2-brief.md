Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal code-critic-runtime-has-no-shell, tier 3, hazard DESIGN-BEARING, fold round two of chain ccs-build1)
Date: 2026-09-06

# Goal

Round one of chain ccs-build1 built
metasystem/plans/code-critic-runtime-has-no-shell-brief.md exactly. The
orchestrator then launched a claude delegate with the candidate
engine's own settings and argv (adapter claude-settings and adapter
claude-command from a code-critic record with no write roots) and the
probe refuted two premises of that brief:

1. With `sandbox.filesystem.allowWrite` empty, Bash still writes inside
   the WORKING DIRECTORY and inside every `--add-dir` root: a `touch`
   in the reviewed tree succeeded, from the tree as cwd and from a
   scratch cwd with the tree added as a read root. The claude sandbox
   treats cwd and added directories as writable; allowWrite only adds
   paths. And metasystem/scripts/agents/dispatch.sh gives only
   implementers a job worktree (line 1470); every other role, the
   critics included, runs with workspaceRoot = the live repository
   root (line 1500). So round one, as built, would let a critic's
   shell write the live checkout.
2. The system temporary directory is not writable in that sandbox:
   `go test` failed "failed to initialize build cache ... operation not
   permitted" with GOCACHE under it, so round one's GOCACHE export does
   nothing.

A second probe found the remedy the sandbox honours: with
`sandbox.filesystem.denyWrite` naming the reviewed tree and
`allowWrite` naming one private directory outside it, with GOCACHE and
GOTMPDIR under that directory, `go test -count=1 ./internal/refusal`
passed and a `touch` in the tree and a `touch` in the cwd were both
refused "Operation not permitted".

When you are done, a claude critic's shell can run evidence commands
and cannot write the workspace or any read root, every claude delegate
has one private writable scratch directory outside the repository for
its Go caches and temp files, and the tests pin the settings shape.

# The fold

1. One private scratch directory per claude delegate round, outside
   every repository root: `<system temp dir>/metasystem-claude/<job>-<round>`
   (use the same spelling on both sides). In
   metasystem/scripts/agents/adapters/claude.sh the launch subshell
   creates it and exports `TMPDIR`, `GOCACHE=<scratch>/go-cache` and
   `GOTMPDIR=<scratch>/go-tmp` (replace round one's GOCACHE lines; the
   comment says why: the sandbox does not let a delegate write the
   user's caches or the system temp dir, only what allowWrite names).
   The adapter passes the directory to the settings builder with a new
   flag, `--scratch <dir>`, on `adapter claude-settings`
   (metasystem/cmd/metasystem/adapter_runtime_verbs.go, where the verb
   parses its flags; add the flag there and thread it into
   BuildClaudeSettings as a new parameter).
2. BuildClaudeSettings in metasystem/internal/adapter/claude.go:
   `sandbox.filesystem.allowWrite` = the requested writeRoots plus the
   scratch directory (for every role; an empty scratch flag adds
   nothing, so existing callers and tests keep their shape). For the
   critic set with empty writeRoots, add
   `sandbox.filesystem.denyWrite` = the record's workspaceRoot plus
   every requested readRoot (the same list ClaudeReadRoots produces),
   deduplicated, absolute. Other roles get no denyWrite (out of this
   goal's scope; the orchestrator backlogs the implementer hole
   separately).
3. The comment at the tool-list selection in BuildClaudeCommand says
   the true reason a shell is safe: the settings deny every write to
   the workspace and the read roots, the only writable place is the
   private scratch directory, and the role packet forbids editing.
   Remove the sentence that says the sandbox denies writes outside the
   requested roots by itself; it does not.
4. Tests: extend the code-critic settings test in
   metasystem/internal/adapter/runtime_test.go to assert denyWrite
   equals workspaceRoot plus the read roots and allowWrite equals the
   scratch directory; add one implementer case asserting allowWrite is
   the write roots plus the scratch directory and no denyWrite key;
   keep every existing test unchanged.
5. Everything else from round one stays (the critic tool list, the
   role set, the packets).

# Workspace

The same job worktree, on top of round one.
May touch: metasystem/internal/adapter/claude.go
May touch: metasystem/internal/adapter/runtime_test.go
May touch: metasystem/internal/adapter/claudecommand_test.go
May touch: metasystem/cmd/metasystem/adapter_runtime_verbs.go
May touch: metasystem/scripts/agents/adapters/claude.sh
Must not touch: the codex, devin or fake adapters, adjudicate.go, dispatch.sh, the permission presets, the role packets, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 60 minutes
  of wall clock.
- The orchestrator repeats the live probe with your engine before the
  critique: a go test must run and a touch in the workspace, in a read
  root, and in the cwd must be refused.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/adapter ./cmd/metasystem` (expected: ok)
- `go vet ./internal/adapter ./cmd/metasystem` and `gofmt -l ./internal/adapter ./cmd/metasystem` (expected: clean)
- `bash -n ./scripts/agents/adapters/claude.sh` (expected: clean)
- `bash scripts/agents/go-build.sh && bin/metasystem adapter claude-settings --record <a code-critic job record> --output /tmp/x.json --metasystem-bin bin/metasystem --scratch /tmp/s` then `grep -c denyWrite /tmp/x.json` (expected: 1)
- `git diff --stat` (expected: the round-one files plus adapter_runtime_verbs.go)

# Acceptance Criteria

1. A code-critic record with empty write roots yields settings with
   Bash allowed, Edit/Write/NotebookEdit denied, denyWrite = workspace
   plus read roots, allowWrite = the scratch directory.
2. An implementer record yields allowWrite = write roots plus scratch
   and no denyWrite.
3. The adapter creates the scratch directory and exports TMPDIR,
   GOCACHE and GOTMPDIR under it for every claude delegate.
4. No existing test changed.

# Gap Rule

stop and report a gap; never fill it silently.
