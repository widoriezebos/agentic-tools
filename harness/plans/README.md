# Plans

Task-local plans, obligation matrices, and investigation ledgers live here while work is in flight. They are evidence, not policy: promote a stable lesson into its canonical owner via `wow.md`, then delete or archive the plan. Generated run artifacts belong in a gitignored artifact directory, not here.

Four files are standing ledgers, not task-local evidence, and are exempt from delete-when-shipped: `receipts.log` (task receipts feeding the retro), `instruction-ledger.md` (instruction changes with expected effects and verdicts), `refactor-baseline` (last gate-accepted state), and `frontier` (best-known improvement state). They stay committed and current for the life of the project.

## Handoff Notes

Any stream of work expected to span more than one session keeps a note at `plans/handoff-<stream>.md`, updated before the session ends. Its job is to make the next session start warm instead of re-deriving context:

```markdown
# <stream>

- Owner: <agent, session, branch>
- Goal and current status:
- In flight right now:
- Decisions made (and who made them):
- Dead ends (do not retry without new evidence):
- Next step:
```

Rules:

- Update the note before ending a session on unfinished work; a stale note is worse than none.
- Notes are owned. Do not advance a stream whose note another agent owns — see the peer-agents section of `docs/orchestration.md`.
- Record decisions and dead ends, not narration. The next session needs what was settled and what must not be retried, not a diary.
- Delete the note when the stream ships; anything durable in it moves to code, docs, or its canonical owner first.
