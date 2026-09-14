Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-14

# Goal

Fix one factual error the read of the documentation amendment found, in
both places that carry it. The documentation file only; no code, no plans.
Do not commit.

# The finding, as the reader wrote it

GAWR-DOC-01 (medium). "The drop rule's first case tells the reader to
conclude a goal with `goal done --concluded \"<what changed so this is no
longer wanted>\"`. That command cannot work. The done verb has no
`--concluded` flag (the flag is `--conclude`), and it also needs `--id`,
which the example leaves out. A reader who follows the rule gets 'flag
provided but not defined: -concluded' and exit code 2 on any checkout."

# What to change

- metasystem/docs/backlog-mechanism.md, "The drop rule", first case: the
  command becomes the one the engine accepts, naming the goal and using
  the flag that exists. Check it against `metasystem goal done` in the
  worktree rather than trusting this brief; the verb also carries the
  identity requirement every mutation has, so the example must be one a
  reader can run.
The design page carries the same wrong example, and the page is not yours
to edit: a design delegate corrects it separately. Change only the
documentation file.
- The abandon example in the same section already names `--id`, `--by` and
  `--because`; check it the same way and correct it only if it is wrong.

# What the round must satisfy

- `( cd metasystem && scripts/audit-metasystem.sh . )` passes.
- `( cd metasystem && scripts/agents/go-gate.sh --fast )` passes.
- Only metasystem/docs/backlog-mechanism.md changes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with `{command, observed, level}` evidence replayable from the worktree's
repository root: `git -C metasystem status --short`, the command check you
ran against the verb, the audit and the fast gate. whatWasDone quotes the
corrected command. Every path in your return starts with `metasystem/`.

# Constraints

Wall clock: 20 minutes.

# Gap Rule

stop and report a gap; never fill it silently.
