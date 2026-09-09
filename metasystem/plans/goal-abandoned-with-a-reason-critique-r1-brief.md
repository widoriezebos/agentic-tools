Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Review brief: first independent critique of the abandoned-goal design

FINDING IDS: chain-unique, GAW-01, GAW-02, ... never F-n.

## What you are reading

`metasystem/plans/goal-abandoned-with-a-reason-design.md`, revision 1,
landed at commit 03f94dfc, sha256
1d6245c80dbae6d1ec8dbc95ae4c52dfed7143ad0c4e8fddd6021ad1682cdbb5, authored
on the Fable lane. The goal record is
`metasystem/plans/goals/goal-abandoned-with-a-reason.md`; the brief it
answered is `metasystem/plans/goal-abandoned-with-a-reason-design-brief.md`.

Write your register as this new file: `metasystem/records/misc/goal-abandoned-with-a-reason-critique-r1.md`

Read-only design critique; implement nothing, run no bed. A material
finding is a place where two implementers would build different things, or
a claim the page rests on that the tree does not support. This is fold-read
cycle 0; a material finding here is expected, not resented. The change
touches the ledger's record grammar, its tree shape and the stop machinery's
evidence at rest; a defect here reaches every seat, so assume there is one.

## Settled, do not re-derive

The eight constraints in the brief, each verified in the tree by the
orchestrator today: `depState` calls every archived id done; the
claim-clearing helper refuses under a stop fence and every exit verb uses it;
the record grammar is closed and archived records must be done; prune's
retention closure; the one-claim quota; the doc's drop rule. Wido's shape and
Codex's argument that a marker on done is a state by another name. The two
specimens as the page describes them.

## Mandate, in order of consequence

1. **The third map (section 3), and whether its census is complete.** The
   page classifies eight archive lookups as "completed set" or "archived
   record". The orchestrator's own census of every non-test reader of the
   archive map outside package goal also finds
   `metasystem/cmd/metasystem/goalsync_mutations.go` line 1130 (in func runGoalSplit),
   `metasystem/internal/counselor/sources.go` line 542, the listing and show
   sites in `metasystem/cmd/metasystem/goal.go` at 316, 361 and 450, and the
   legacy migration at `metasystem/internal/goal/migrate.go` line 366. Say
   for each whether the page's twenty-two-row table covers it and classifies
   it correctly; one classified wrongly is the page's own stated risk. Then
   judge the shape itself: does a third map fail closed everywhere the page
   claims, and is `Archived()`'s Done-first order ever wrong?
2. **A frozen fence at rest (section 8, rule 6).** Today stop authority may
   exist only on a claimed record (`metasystem/internal/goal/file.go` line
   468). The page keeps a breach-stopped goal's `StopFence` and
   `StopCapability` byte-identical on the abandoned record. Walk every
   reader of a fence or capability outside the ledger's own validator — the
   stop verbs and their inventory, the steward's breach and idle logic, the
   fleet enumeration, `VerifyStopBatchComplete`, resume — and say whether
   any of them treats "a record with a fence" as "a claimed goal that is
   stopped" and would act on an abandoned one. This is the highest-hazard
   interaction on the page.
3. **The transition (section 4).** Refusal 3 sees only this checkout's job
   records; the page relies on dispatch admission and landing requiring a
   claimed goal to make a foreign running job inert. Confirm both sites in
   the code and say what happens to that job's budget accounting and its
   eventual return. Then the dependency flags: after `--waive`, what
   exactly changes on the dependent's record, and does validation rule 5 of
   section 8 still fire on it? After `--carried`, is the edge re-pointed on
   every dependent or on the abandoned record?
4. **Reopen (section 6).** The Abandoned record is removed and the stop id
   stays in history. What happens to `StopFence` and `StopCapability` on
   reopen: if they stay, the record is queued with stop authority, which
   rule 6 forbids; if they go, say where the page says so. And "no stop-batch
   verification, and why": is the why sound?
5. **Prune (section 7) and validation (section 8).** Retention becomes the
   closure of live, newest-N done and abandoned goals with their
   prerequisites; abandoned goals sit outside the newest-N allowance. Check
   the closure walks through abandoned records' `BlockedBy` and that a
   done goal blocked by an abandoned one (impossible today by the rule at
   `metasystem/internal/goal/validate.go` line 289) stays impossible.
6. **Rollout (section 10).** No shim, no flag, a printed warning, and a
   procedural order. Judge whether "print and continue" is acceptable for a
   change whose failure wedges every older seat's ledger reads, or whether
   the first abandon should refuse without a recorded fleet-wide ruling.
7. **The canaries (section 13).** Most fail on the untouched tree at the
   parser boundary rather than at the reader they name, and the page says
   so per canary. Judge whether that is acceptable fail-before, and whether
   the dependency-reader canary and the specimen test can fail for the right
   reason.
8. **The wall (section 15).** Done keeps its meaning and its counts; abandoned
   is not a frontier category; stop proofs are never rewritten; no history
   is truncated.

## Constraints

Wall-clock budget: 45 minutes. Return per the design-critic schema with the
page digest above as the reviewed identity. Gap rule: stop and report a gap;
never fill it silently. Your sandbox cannot run the fixture beds; do not
treat that as evidence about the design.
