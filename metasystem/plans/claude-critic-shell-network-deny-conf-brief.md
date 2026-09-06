Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-critic-shell-network-deny, tier 2, hazard MECHANICAL)
Date: 2026-09-06

# Goal

Chain ccn-build1 landed (d471864f): a critic.json preset with network
deny, and metasystem/scripts/agents/dispatch.sh defaulting code-critic
and design-critic to it unless `dispatch.permissions.<role>` is set.
The tracked repository configuration metasystem/metasystem.conf sets
exactly those keys explicitly (lines near 85: design-critic=none,
code-critic=none, investigator=none, implementer=workspace,
verifier=workspace), so on this repository the explicit keys win and
the first proof critic dispatched after the landing (ccn-proof1) was
still recorded with preset none and network allow. The orchestrator's
brief did not survey the configuration; this round repairs it.

When you are done, the shipped configuration names the critic preset
for code-critic and design-critic, and a critic dispatched from this
repository records effective network deny.

# The change

In metasystem/metasystem.conf, change `dispatch.permissions.design-critic=none`
and `dispatch.permissions.code-critic=none` to `=critic`; leave the
investigator, implementer and verifier keys and every other line
untouched. If a comment above those lines describes the presets,
extend it by one sentence naming critic as the zero-write,
network-deny preset.

Second, the proof critic ccn-proof1 noted that the dispatch fixture bed
pins the effective network only for a code-critic (flag-runtime) and an
implementer (review-target). In metasystem/scripts/agents/dispatch-fixtures.sh,
beside that assertion, add the same effective-network deny assertion for
a design-critic fixture dispatch (the bed already dispatches design
critics, for example net-default) and for a warden fixture dispatch if
the bed has one (say so if it does not). The bed copies the real
metasystem.conf into its fixture repository, so the conf change above
must not break the bed's own conf edits around the critic key.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/metasystem.conf (those two lines and their comment only)
May touch: metasystem/scripts/agents/dispatch-fixtures.sh (the effective-network assertions only)
Must not touch: anything else.

# Constraints

- One round, at most 30 minutes. Hazard MECHANICAL: two configuration
  values; the orchestrator proves it by dispatching a critic and
  reading its recorded permissions.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `grep -n 'dispatch.permissions' metasystem.conf` (expected: the two critic lines say critic, the rest unchanged)
- `bin/metasystem config get --key dispatch.permissions.code-critic` after `bash scripts/agents/go-build.sh` (expected: critic; if the verb spells its flags differently, say the exact invocation)
- `bash -n ./scripts/agents/dispatch-fixtures.sh` (expected: clean)
- `git diff --stat` (expected: metasystem.conf and dispatch-fixtures.sh only)

# Acceptance Criteria

1. The two keys say critic; nothing else in the file changed.
2. The bed asserts effective network deny for a design-critic (and a warden if dispatched) beside the code-critic.

# Gap Rule

stop and report a gap; never fill it silently.
