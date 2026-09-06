Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-critic-shell-network-deny, tier 2, hazard MECHANICAL)
Date: 2026-09-06

# Goal

Since goal code-critic-runtime-has-no-shell (07d18614) a claude critic
has a sandboxed shell, and the read-only permission preset it
dispatches under (metasystem/scripts/agents/permissions/none.json)
grants network allow, so a critic's shell can push with the user's
stored credentials, upload, or install packages. The first critique of
that goal named it (ccs-critic1, F-3). The orchestrator probed the
existing deny shape on 2026-09-06 with a code-critic record whose
requested network was deny: the settings builder emitted the
non-resolving sentinel (allowedDomains metasystem.invalid), and from
the critic's shell `curl https://example.com/` and
`curl https://api.github.com/` were both refused "CONNECT tunnel
failed, response 403" with a recorded sandbox violation, while
`go test ./internal/refusal` still ran. The loopback allowance landed
separately (bf3d3417) and is unaffected by the egress rule.

When you are done, code-critic and design-critic dispatch under a
zero-write, network-deny preset by default, the warden's forced preset
is that same file, the none preset keeps granting network (a
repository may still narrow every role with the existing
dispatch.permissions.network floor), an implementer's rule is
unchanged, and the dispatch fixtures pin the critic's effective
network as deny.

# The change

1. A new preset file, critic.json, beside none.json in the permissions directory (scripts/agents/permissions):
   readRoots ["."], writeRoots [], network "deny", approvals "deny",
   tools "read-only" (none.json with network deny).
2. metasystem/scripts/agents/dispatch.sh: the preset default for a role
   comes from `dispatch.permissions.<role>` with default none; make the
   default `critic` for code-critic and design-critic (an explicit
   config key still wins), and point the warden's forced absolute path
   at critic.json instead of none.json, keeping its comment's law (a
   zero-write preset forced by path). Nothing else in dispatch changes.
3. metasystem/scripts/agents/dispatch-fixtures.sh: beside the existing
   preset assertions (the none and workspace presets keep granting
   network, unchanged), add: the critic preset's network field is deny;
   and after a code-critic fixture dispatch, the job record's
   permissions.effective.network is deny while an implementer fixture
   dispatch's is allow. Reuse the bed's existing helpers for dispatching
   fixture roles; do not add a scenario that needs a live provider.
4. metasystem/scripts/agents/roles/code-critic.md, design-critic.md and
   warden.md: one sentence outside every quote block saying the shell
   has no network egress, only loopback.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: the new file scripts/agents/permissions/critic.json
May touch: metasystem/scripts/agents/dispatch.sh (the preset default and the warden path only)
May touch: metasystem/scripts/agents/dispatch-fixtures.sh
May touch: metasystem/scripts/agents/roles/code-critic.md
May touch: metasystem/scripts/agents/roles/design-critic.md
May touch: metasystem/scripts/agents/roles/warden.md
Must not touch: the adapters, internal/adapter, internal/dispatch, none.json, workspace.json, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 45 minutes.
  Hazard MECHANICAL: a preset file and its default selection; the
  claude side is proven by probe; the orchestrator runs the dispatch
  fixture bed seat-side and proves the live envelope through a
  dispatched critic whose curl is refused.
- The preamble-quotes validation checks the packets' quote blocks byte
  for byte; add your sentence outside them.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/dispatch.sh` and `bash -n ./scripts/agents/dispatch-fixtures.sh` (expected: clean)
- `bin/metasystem json get --file scripts/agents/permissions/critic.json --field network` after `bash scripts/agents/go-build.sh` (expected: deny)
- `bin/metasystem validate preamble-quotes --root . --roles-dir scripts/agents/roles` (expected: passes)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. A code-critic or design-critic dispatch without an explicit
   permissions key records effective network deny; the warden the
   same; an implementer records allow; none.json unchanged.
2. The fixture bed pins those; the packets carry the sentence.

# Gap Rule

stop and report a gap; never fill it silently.
