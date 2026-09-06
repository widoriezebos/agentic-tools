Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-delegate-scratch-cleanup, tier 2, hazard MECHANICAL)
Date: 2026-09-06

# Goal

Since 07d18614 every claude delegate round gets a private scratch
directory (metasystem/scripts/agents/adapters/claude.sh, the
scratch_dir local in supervise, composed as
`${TMPDIR:-/tmp}/metasystem-claude/<job>-<round>`, created in the
launch subshell with go-cache and go-tmp under it and exported as
TMPDIR, GOCACHE and GOTMPDIR) and nothing removes it: one adapter
test run leaves 120 MB, a suite day leaves gigabytes (critique
ccs-critic2 F-4, proof ccs-proof1 F-3). Two more defects ride on the
same lines: the host TMPDIR ends in a slash on macOS, so the path is
spelled with a double slash (proof ccs-proof1 F-4); and /var/folders
is a symlink to /private/var/folders, so the unresolved path makes
five adapter collect-port tests fail inside a delegate shell on a
temp-path normalisation mismatch while they pass with a resolved
GOTMPDIR (proof lpb-proof1 F-1). And no test pins the four sandbox
switches (enabled, failIfUnavailable, autoAllowBashIfSandboxed,
allowUnsandboxedCommands), so a flip to unsandboxed Bash would pass
the suite (critique ccs-critic2 F-7).

When you are done, the scratch path is resolved and slash-clean, it
is removed when the round ends on every exit path, a fixture proves
the removal, and a settings test pins the four switches.

# The change

1. claude.sh: compose scratch_dir from the resolved temp base
   (`cd "${TMPDIR:-/tmp}" && pwd -P`, trailing slash gone) so the
   settings' allowWrite entry, GOCACHE and GOTMPDIR carry the real
   path; keep the job-and-round naming.
2. claude.sh: remove the scratch directory when the round ends: after
   the result is derived and appended (the claude-derive-result and
   claude-append-result lines) on the normal path, and from the
   function's existing failure exits (fail_pending returns) so a
   handshake failure or a killed CLI does not leave it; `rm -rf` of
   exactly that directory, never its parent. A comment says why the
   caches are per round and thrown away (a delegate must not inherit
   another round's cache).
3. metasystem/internal/adapter/runtime_test.go: in the code-critic
   settings test assert sandbox.enabled true, failIfUnavailable true,
   autoAllowBashIfSandboxed true, allowUnsandboxedCommands false.
4. A fixture: the dispatch fixture bed's adapter-selftest scenario
   (metasystem/scripts/agents/dispatch-fixtures.sh) or whichever bed
   drives claude.sh with a fake runtime (metasystem/scripts/agents/telemetry-census-fixtures.sh
   and metasystem/scripts/agents/supervision-hook-fixtures.sh name the
   adapter); add one assertion that after a claude fixture round the
   round's scratch directory does not exist and that the settings
   file's allowWrite scratch entry has no double slash and no
   unresolved symlink prefix. If no bed drives claude.sh end to end,
   say so (gap rule) and put the path assertions in a bash -n plus a
   sourced-function test the way the bed tests other adapter helpers.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/scripts/agents/adapters/claude.sh
May touch: metasystem/internal/adapter/runtime_test.go
May touch: one fixture bed among those named above
Must not touch: internal/adapter/claude.go, the other adapters, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 45 minutes.
  Hazard MECHANICAL: cleanup and path hygiene; the orchestrator
  proves it live by dispatching a claude delegate and checking the
  scratch directory is gone afterwards and the settings path is clean.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/adapters/claude.sh` (expected: clean)
- `go test -count=1 -run 'Settings' ./internal/adapter` (expected: ok)
- the fixture leg's own command, or the gap statement
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. After a claude delegate round the scratch directory is gone on the
   normal and the failure paths.
2. The scratch path in the settings, GOCACHE and GOTMPDIR is resolved
   and has no double slash.
3. The four sandbox switches are pinned; the fixture pins the removal.

# Gap Rule

stop and report a gap; never fill it silently.
