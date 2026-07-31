# Plans

Task-local plans, obligation matrices, and investigation ledgers live here while work is in flight. They are evidence, not policy: promote a stable lesson into its canonical owner via `wow.md`, then delete or archive the plan. Generated run artifacts belong in the gitignored `artifacts/` directory at the repository root, never here.

Evidence that must survive (paid-run raws, acceptance proof) is mirrored to the project's durable evidence root (declared in `docs/project-rules.md`, outside the repository and every build tree), with content hashes verified on the copy, before the originals count as disposable. A directory whose lifecycle contract is "safe to wipe" (build output, caches, anything `git clean` reaches) never holds the only copy of anything: a rule telling actors not to wipe it treats the symptom, while moving the asset treats the hazard.

Five files are standing ledgers, not task-local evidence, and are exempt from delete-when-shipped: `receipts.log` (task receipts feeding the retro), `instruction-ledger.md` (instruction changes with expected effects and verdicts), `known-issues.md` (recorded-but-unscheduled defects, capability ceilings, and do-not-retry dead ends), `refactor-baseline` (last gate-accepted state), and `frontier` (best-known improvement state). They stay committed and current for the life of the project.

With peer agents on parallel branches: `receipts.log` merges by union (the shipped `.gitattributes` rule), so concurrent appends do not conflict. Concurrent changes to `refactor-baseline` or `frontier` are real races (two competing claims about trusted state), and those merge conflicts go to the human by design. `known-issues.md` also merges normally: its entries are amended in place, so conflicts must surface instead of unioning into two competing versions of one entry.

## Handoff Notes

Any stream of work expected to span more than one session keeps a note at `plans/handoff-<stream>.md`, updated before the session ends. Its job is to make the next session start warm instead of re-deriving context:

```markdown
# <stream>

- Owner: <agent, session, branch>
- Goal and current status:
- In flight right now:
- Decisions made (and who made them):
- Waiting on the human (open escalations, reviews, reserved decisions):
- Dead ends (do not retry without new evidence):
- Next step:
```

Rules:

- Update the note before ending a session on unfinished work; a stale note is worse than none.
- Notes are owned. Do not advance a stream whose note another agent owns; see the peer-agents section of `docs/orchestration.md`.
- Record decisions and dead ends, not narration. The next session needs what was settled and what must not be retried, not a diary.
- Keep the waiting-on-the-human line current: every open escalation, review, and reserved decision stays listed until answered. An ask that lives only in a chat transcript is lost to the next session.
- Delete the note when the stream ships; anything durable in it moves to code, docs, or its canonical owner first. Dead ends worth remembering beyond the stream move to `known-issues.md`.
