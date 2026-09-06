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

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/metasystem.conf (those two lines and their comment only)
Must not touch: anything else.

# Constraints

- One round, at most 20 minutes. Hazard MECHANICAL: two configuration
  values; the orchestrator proves it by dispatching a critic and
  reading its recorded permissions.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `grep -n 'dispatch.permissions' metasystem.conf` (expected: the two critic lines say critic, the rest unchanged)
- `bin/metasystem config get --key dispatch.permissions.code-critic` after `bash scripts/agents/go-build.sh` (expected: critic; if the verb spells its flags differently, say the exact invocation)
- `git diff --stat` (expected: only metasystem.conf, two or three lines)

# Acceptance Criteria

1. The two keys say critic; nothing else in the file changed.

# Gap Rule

stop and report a gap; never fill it silently.
