Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal backlog-ordered-by-priority)
Date: 2026-09-09

# Review brief: first independent critique of the conclusion-vs-compaction design

FINDING IDS: chain-unique, BCC-01, BCC-02, ... never F-n.

## What you are reading

`metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md`,
revision 1, landed at commit 8ca43596, sha256 4613d232c47fc70878b19432841fb57d9df13ce80305b9a7c722047888ac35c5. It was
authored on the Fable lane (job bolboc-design3) and answers the two findings
goal `backlog-ordered-by-priority` landed with, BOC-01 and BOQ-02. The parent
specification is `metasystem/plans/backlog-ordered-by-priority-design.md`
revision 2, whose critique ladder is closed; this page must not contradict it
and says where it extends it.

Write your register as this new file: `metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r1.md`

This is a read-only design critique. You implement nothing and run no
fixture bed. The standard is the one the parent chain applied: a material
finding is a place where two implementers would build different things, or a
claim the page rests on that the tree does not support.

## Mandate, in order of consequence

1. **The discriminator (section 1.3).** The page rests question 1 on one
   claim: under `State == done`, the last `done` verb in history is always the
   goal's own conclusion, because every compaction loop walks `t.Live`
   (`internal/goal/order.go`, `split.go`, `reconcilepub.go`) and `reopen`
   returns the record to the live set with a non-done state. The orchestrator
   verified the three call sites pass `t.Live` and that `reopen` sets
   `StateQueued`. Look for the case neither of us found: any path that writes
   a `done`-verb history line onto a record that is, or becomes, archived
   without that line being its own conclusion. Reconcile is the likeliest
   place. If no such path exists, say so and say what you searched.
2. **The reader census (section 1.2).** The page names four readers of the
   `done` verb in `internal/metrics` and one per-operation reader in the
   counselor, and routes the four through one helper. Confirm the census is
   complete for non-test code, and that the counselor's per-operation
   duplicate suppression really makes it immune.
3. **The Refused category (section 2.2, the page's own riskiest part).** An
   approved goal the claim gate refuses for a goal-fact cause becomes a fifth
   frontier category, populated from the gate call the frontier already makes.
   The page argues it is not Awaiting, not Blocked, not Ready. Test each
   exclusion against the parent page's category definitions and fixtures,
   including `over-norm`. Then test the boundary: the page says `APPROVAL_REQUIRED`
   and `GOAL_NORM_REFUSED` reach the frontier while `APPROVAL_EXPIRED` is
   filtered earlier and the brain fence stays outside. Is that partition
   exhaustive and stable, or can a cause land on the wrong side?
4. **The three surfaces (sections 2.3, 2.4).** The `goal next` none line
   gains a first clause, the channel `Next-for` line appends a skipped clause,
   the turn verdict prints one non-blocking line. For each, check the exact
   expected text the fixtures assert is producible from the named code, and
   that the seat's idle refusal neither blocks nor counts on a refused goal.
   The page keeps `goal list` unchanged and says why; judge whether that
   reasoning holds or hides a second traversal.
5. **The fail-before proofs (section 1.5, 2.5; return gaps 5 and 6).** The
   page names the exact pre-repair output of the question 1 canary
   (`c building_hours=12.000 proving_hours=12.000 waiting_share=0.500 epochs=1`)
   derived by reading `computeWaiting` and the formatters, not by running
   them. Check that derivation against the code. A canary that cannot fail
   for the named reason on the untouched tree is a defect in the page.
   Also: the page admits the question 2 unit fixtures compile only after the
   new field exists; judge whether the command and channel canaries carry
   enough fail-before proof on their own.
6. **What must not change (section 3).** No rank set, inferred or moved;
   `goal next` a read; one admission owner; no grammar change; `file.go`,
   `validate.go`, the writers and the goal records untouched. Name any place
   the build boundary would have to widen.

## Two facts so you do not re-derive them

- The orchestrator's own brief offered a wrong discriminator (a real
  conclusion's targets naming only itself) and the page refuted it correctly:
  `doneRequest` writes the compaction target set onto the departed goal's own
  event. Do not re-litigate that; it is settled in the page's favour.
- The critic on the parent chain graded BOC-01 low and material and BOQ-02
  not material. This page's job was to settle both; your job is whether it
  did, not whether they were worth settling.

## Constraints

Wall-clock budget: 45 minutes. Return per the design-critic schema with the
page digest above as the reviewed identity. Gap rule: stop and report a gap;
never fill it silently. Your sandbox cannot run the fixture beds; do not
treat that as evidence about the design.
