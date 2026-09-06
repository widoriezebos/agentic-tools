Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-critic-shell-network-deny, tier 2, hazard MECHANICAL, fold round two of chain ccn-fix1)
Date: 2026-09-06

# Goal

Round one of chain ccn-fix1 set dispatch.permissions.code-critic and
design-critic to critic in metasystem/metasystem.conf and added two
effective-network assertions. The orchestrator ran the whole dispatch
fixture bed on it: the "happy" leg, which dispatches a design-critic
through the fake runtime, still asserts the recorded
permissions.requested.preset is none ("happy record did not request
the none preset") and failed; every other scenario passed. The bed
copies the real configuration into its fixture repository, so the
assertion must follow the configuration.

When you are done, that assertion expects critic for the design-critic
happy dispatch, any other preset assertion tied to a critic role in
the bed follows suit, and the assertions tied to implementer or
investigator dispatches are untouched.

# The fold

In metasystem/scripts/agents/dispatch-fixtures.sh: change the happy
leg's preset assertion (the line that reads permissions.requested.preset
from the happy record) to expect critic, with its message saying "the
critic preset"; grep the bed for every other `requested.preset` or
`effective.preset` assertion and make each one that follows a
code-critic, design-critic or warden dispatch expect critic, leaving
the rest as they are. Change nothing else.

# Workspace

The same job worktree, on top of round one.
May touch: metasystem/scripts/agents/dispatch-fixtures.sh (preset assertions only)
Must not touch: anything else.

# Constraints

- Bash 3.2 clean. One round, at most 20 minutes. The orchestrator
  reruns the bed seat-side.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/dispatch-fixtures.sh` (expected: clean)
- `grep -n 'requested.preset' ./scripts/agents/dispatch-fixtures.sh` (expected: the critic-role lines say critic)
- `git diff --stat` (expected: metasystem.conf and dispatch-fixtures.sh only, across both rounds)

# Acceptance Criteria

1. The happy leg expects critic; no implementer or investigator assertion changed.

# Gap Rule

stop and report a gap; never fill it silently.
