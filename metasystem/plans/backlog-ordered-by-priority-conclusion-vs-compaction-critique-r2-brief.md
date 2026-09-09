Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal backlog-ordered-by-priority)
Date: 2026-09-09

# Review brief: second read of the conclusion-vs-compaction design (revision 2)

FINDING IDS: chain-unique, BCD-01, BCD-02, ... never F-n.

## What you are reading, and why this read exists

`metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md`,
revision 2, landed at commit 0260aef6, sha256
b1f93510884bfd408338a768f70ffb6ad6c1a49c1861d17421f8832b00cfde13. It folds
the first read, whose register with the coordinator's disposition is
`metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r1.md`.
This is fold-read cycle 1: the first read found a real hole (the split
parent), the fold answered it, and this read decides whether the answer holds.
If you find something material, say so plainly; the coordinator will then say
aloud that this loop has no natural exit and take the land-or-fold call to
Wido before any third cycle. Do not soften a finding to avoid that.

Write your register as this new file: `metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r2.md`

Read-only design critique; implement nothing, run no fixture bed. A material
finding is a place where two implementers would build different things, or a
claim the page rests on that the tree does not support.

## Mandate, in order of consequence

1. **The archive-line rule and its proof (section 1.3, the page's own
   riskiest part).** On an archived record the conclusion instant is the last
   history line whose verb is `done` or `split`. The page proves this from
   an enumeration of every writer that can append a line to an archived
   record. The coordinator re-checked before landing: compaction loops pass
   `t.Live` (`order.go`, `split.go:301`, `reconcilepub.go:124`,
   `verbs.go:1162`); reconcile's archive-side merge at
   `reconcilepub.go:126-130` is guarded by `Opid == r.opid()`; the only
   `delete(t.Done, …)` is `reopen` at `verbs.go:1409`; migrate appends
   only `migrate` lines. The page's stated risk is a writer that reaches an
   archived record through a pointer obtained elsewhere than the `t.Done`
   map. Hunt for exactly that: any `*GoalFile` held across an archive, any
   arc or member iteration that keeps a pointer after `t.Done[...] = `,
   any history append on a record not looked up by id at the time. If none
   exists, say what you searched.
2. **The split-parent canaries (section 1.5).** Three must fail on the
   untouched tree by dating the parent to its neighbour's conclusion. Their
   before and after numbers (12 building hours; 12 then 36 proving hours;
   shares 0.500 and 0.750) were hand-computed from the waiting-metric
   formulas in `metasystem/internal/metrics/compute.go`, not run. Recompute
   them from the code. A canary that fails for the wrong number is a defect
   in the page.
3. **The writer-side premise pins (section 1.5).** Two subtests in the goal
   package pin the writers the rule relies on, including the new one that
   produces the failing path by concluding the middle goal and then splitting
   the survivor. Confirm each pin asserts a fact the current writers actually
   produce, and that a pin failing on the untouched tree would mean what the
   page says it means.
4. **BCC-03 closure (section 1.4).** `historyTime` and `concludingEpoch`'s
   own scan are deleted; all four readers route through `concludedAt`. Check
   the deletion does not remove a reader the census missed, and that
   `ConcludedInWindow`, which has no callers, is handled honestly under the
   fast gate's staticcheck.
5. **BCC-02 closure (section 2.2 and the grounding row).** The page now says
   the missing-approval refusal cannot reach the frontier on a parsed tree
   and that the over-budget norm refusal is the Refused category's production
   cause set. Confirm against `metasystem/internal/goal/file.go` and the
   tree loader, and that no fixture still premises on the unreachable branch.
6. **The boundary (section 3).** The untouched list grew by the hand-edit
   mapper and the migration code. Confirm nothing else would have to widen,
   and that the Refused category decision, which this fold did not reopen,
   is unchanged from revision 1.

## Constraints

Wall-clock budget: 45 minutes. Return per the design-critic schema with the
sha256 above as the reviewed identity. Gap rule: stop and report a gap; never
fill it silently.
