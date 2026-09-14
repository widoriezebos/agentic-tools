Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-14

# Goal

One correction to metasystem/plans/goal-abandoned-with-a-reason-design.md
(revision 7 in your worktree). Section 11, the doc amendment, quotes a
command that cannot run, and the documentation was transcribed from it.
Correct the page so the two agree; change nothing else and do not raise
the revision for this, beyond the line the page keeps for what changed.

# The finding, as the read of the documentation wrote it

GAWR-DOC-01 (medium). "The drop rule's first case tells the reader to
conclude a goal with `goal done --concluded \"<what changed so this is no
longer wanted>\"`. That command cannot work. The done verb has no
`--concluded` flag (the flag is `--conclude`), and it also needs `--id`,
which the example leaves out. A reader who follows the rule gets 'flag
provided but not defined: -concluded' and exit code 2 on any checkout."

The documentation file now reads `goal done --id <goal> --conclude "<what
changed so this is no longer wanted>"`, checked against
`metasystem goal done --help` in the worktree. The abandon example in the
same section was checked the same way and is correct as it stands.

# What to change

- Section 11's quoted replacement text carries the corrected conclude
  command, so the page and metasystem/docs/backlog-mechanism.md say the
  same thing.
- Check the command against the verb in your own worktree rather than
  trusting this brief.
- Nothing else on the page changes.

# Workspace

Your job worktree, branch agent/<job>. Modify exactly one file, the design
page. Do not commit. Do not write code.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with exactly two evidence items: `git -C metasystem status --short`, and
the command check you ran against the verb. whatWasDone quotes the
corrected line. Every path in your return starts with `metasystem/`.

# Constraints

Wall clock: 20 minutes.

# Gap Rule

stop and report a gap; never fill it silently.
