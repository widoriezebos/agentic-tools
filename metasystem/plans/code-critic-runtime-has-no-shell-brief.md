Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal code-critic-runtime-has-no-shell, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Every Fable code-critic round dispatched today (eight of them, on chains
rgr-build1, shr-build1, bcd-build1, lsr-build1, lrr-build1 and
fgr-build1) reported the same gap: the claude runtime exposed only file
reading and searching tools, no shell, so none of the evidence commands
the review brief allowed (focused go tests, vet, gofmt, bash -n, git
diff) could run, and every evidence entry was read or inferred. The
code-critique skill expects the critic to run focused checks that
separate a suspected defect from a hypothetical one, and the
orchestrator ran them in the critic's place each time. A second,
smaller gap rides along: the same runtime's sandbox cannot write the
Go build cache in the user's home, so every implementer that ran go
test had to invent a private GOCACHE by hand first.

The cause is in metasystem/internal/adapter/claude.go. The command
builder (BuildClaudeCommand) selects `claudeReadOnlyTools`
("Read,Glob,Grep") whenever the record's requested writeRoots is
empty, and the settings builder (BuildClaudeSettings) puts Bash in the
settings' permissions deny list on the same condition. The critic
roles are dispatched with no write roots, so both narrowings hit them.
The role's "never edit" law is right; the narrowing that enforces it
also removes the shell the role needs. The sandbox itself already
keeps the reviewed tree read-only: its filesystem allowWrite list is
the requested writeRoots, empty for a critic, and the working directory
is not writable by default (today's implementer rounds could write
only their own worktree and the system temporary directory).

When you are done, a code-critic, design-critic or warden (the three
read-only roles, the set at line 165 of
metasystem/internal/adapter/adjudicate.go) dispatched on the claude
runtime can run the brief's evidence commands through a sandboxed
shell, still cannot write into the reviewed tree, every claude delegate
has a Go build cache it can write, and Go tests pin the envelope per
role.

# The design

1. A third tool list in claude.go, `claudeCriticTools` =
   "Bash,Read,Glob,Grep". BuildClaudeCommand selects it when the
   record's `role` is code-critic, design-critic or warden AND the
   requested writeRoots is empty; the read-only list stays for any
   other role with empty writeRoots; the full list stays for any role
   with write roots. The permission mode stays dontAsk for the critic
   set. Read the role from the record's `role` field, the same field
   adjudicate.go reads.
2. BuildClaudeSettings makes the same decision from the same fields:
   for the critic set with empty writeRoots, Bash moves from the deny
   list to the allow list; Edit, Write and NotebookEdit stay denied;
   the sandbox block is unchanged (allowWrite stays the empty
   requested writeRoots, autoAllowBashIfSandboxed stays true, the
   network rule is unchanged). Put the role set in one place both
   builders read (a small function or a package-level set) so the two
   files cannot disagree.
3. The comment where the tool list is chosen says why a shell is safe
   for a critic: the sandbox denies every write outside the requested
   roots, the certified diff is persisted before any critic runs, and
   the role's law that it never edits stays in its packet.
4. The Go build cache: in metasystem/scripts/agents/adapters/claude.sh,
   in the subshell that changes into the workspace and executes the
   command, export GOCACHE to a private directory under the system
   temporary directory named for the job and round (for example
   `${TMPDIR:-/tmp}/metasystem-go-cache/<job>-<round>`), created there
   before the exec, for every claude delegate, not only critics. Say
   in a comment why: the sandbox cannot write the user's cache
   directory, and the temporary directory is the one place every
   delegate may write. Do not change TMPDIR, GOPATH or GOMODCACHE;
   module reads from the shared module cache are fine.
5. Tests. In metasystem/internal/adapter/claudecommand_test.go add one
   table test pinning the tool list and permission mode per role and
   writeRoots shape (implementer with write roots: full tools,
   acceptEdits; each critic role with none: critic tools, dontAsk; any
   other role with none: read-only tools, dontAsk). In
   metasystem/internal/adapter/runtime_test.go add one test that a
   code-critic record with no write roots yields a settings file with
   Bash allowed, Edit, Write and NotebookEdit denied, and an empty
   allowWrite. Change no existing test.
6. The role packets metasystem/scripts/agents/roles/code-critic.md,
   design-critic.md and warden.md gain one sentence saying the shell is
   for running the brief's evidence commands and never for editing.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/adapter/claude.go
May touch: metasystem/internal/adapter/claudecommand_test.go
May touch: metasystem/internal/adapter/runtime_test.go
May touch: metasystem/scripts/agents/adapters/claude.sh
May touch: metasystem/scripts/agents/roles/code-critic.md
May touch: metasystem/scripts/agents/roles/design-critic.md
May touch: metasystem/scripts/agents/roles/warden.md
Must not touch: the codex, devin or fake adapters, adjudicate.go, the
permissions presets, the dispatcher, anything under plans.

# Constraints

- The preamble quote blocks in the role packets are byte-exact quotes
  of their sources (the preamble-quotes validation checks them); add
  your sentence outside any quote block.
- Bash 3.2 clean for the shell change. Never weaken a test. One round,
  at most 90 minutes of wall clock.
- Hazard DESIGN-BEARING: a permission envelope changes; an independent
  critique follows, and the orchestrator proves the live envelope
  seat-side by launching a claude delegate from the candidate's engine
  and settings and asking it to run a go test and to touch a file in
  the reviewed tree (the test must run, the touch must be refused).

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/adapter` (expected: ok)
- `go vet ./internal/adapter` and `gofmt -l ./internal/adapter` (expected: clean, no output)
- `bash -n ./scripts/agents/adapters/claude.sh` (expected: clean)
- `bin/metasystem validate preamble-quotes` after `bash scripts/agents/go-build.sh` (expected: passes; say the exact invocation the verb takes if it differs)
- `grep -n 'claudeCriticTools' ./internal/adapter/claude.go` (expected: the constant and its one selection)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. A record with role code-critic and empty writeRoots yields
   "--tools Bash,Read,Glob,Grep --allowedTools Bash,Read,Glob,Grep
   --permission-mode dontAsk" and a settings file with Bash allowed
   and the three editing tools denied; an implementer record yields the
   full list and acceptEdits; another role with empty writeRoots yields
   the read-only list.
2. The new tests pin those shapes; no existing test changed.
3. Every claude delegate exports a private GOCACHE under the system
   temporary directory.
4. The three role packets carry the sentence; preamble quotes still
   validate.

# Gap Rule

stop and report a gap; never fill it silently.
