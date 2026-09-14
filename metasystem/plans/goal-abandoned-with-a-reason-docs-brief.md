Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-14

# Goal

Build section 11 of metasystem/plans/goal-abandoned-with-a-reason-design.md
(revision 7), the doc amendment. Slices 1 and 2 of this goal landed at
345d809c, so the verbs the page documents exist on trunk; the landing side
lands separately. This round changes documentation only. Do not commit.

# What to write

metasystem/docs/backlog-mechanism.md, the section "The drop rule", becomes
the two cases the page gives, in the page's own words: the want evaporated
(conclude it, because done means the ledger's promise is discharged and
this case discharges it honestly) and the want survives while the pursuit
stops (abandon it, with the command form the page shows; an abandoned goal
leaves the live set, carries its reason forever, satisfies no dependency,
is never pruned, and a human can reopen it into the queue unranked and
unapproved; never conclude such a goal, because a conclusion that says the
work was not done corrupts every count of completed work; never park it,
because parked means resume later).

The same file's "Concluding a goal" section gains one sentence: "An
abandoned goal writes no journey chapter; its reason on the record is its
story."

Take the wording from section 11 of the page, which quotes the replacement
text in full. Keep the file's voice and its existing structure; change no
other section. metasystem/AGENTS.md names no state list and needs no
change, as the page says.

# What the round must satisfy

- `( cd metasystem && scripts/audit-metasystem.sh . )` passes; the
  traveling-surface audit refuses a bare "a gate" or "the job" in prose
  under docs.
- `( cd metasystem && scripts/agents/go-gate.sh --fast )` passes.
- No file outside metasystem/docs/backlog-mechanism.md changes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with `{command, observed, level}` evidence replayable from the worktree's
repository root: `git -C metasystem status --short`, the audit, and the
fast gate. whatWasDone says which sections changed. Every path in your
return starts with `metasystem/`.

# Constraints

Wall clock: 30 minutes.

# Gap Rule

stop and report a gap; never fill it silently.
