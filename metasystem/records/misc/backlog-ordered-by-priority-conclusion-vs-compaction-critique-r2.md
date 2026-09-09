# conclusion-vs-compaction design critique — round 2 (revision 2)

Chain: revision 2 (landed 0260aef6, sha256 b1f93510884bfd408338a768f70ffb6ad6c1a49c1861d17421f8832b00cfde13) -> critic bcc-crit2 (design-critic, codex gpt-5.6-sol, xhigh, read-only; the harness observed gpt-5.6-sol and the return claimed nothing different). Reviewed at worktree commit 40924132. 0 findings, 0 material. The coordinator carried the return here verbatim because the critic's sandbox is read-only and it wrote no register file.

## Findings

None.

## What the critic verified, in the brief's order of consequence

- (ran) shasum -a 256 metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md; git rev-parse HEAD; git diff --name-only 0260 => The design digest is b1f93510884bfd408338a768f70ffb6ad6c1a49c1861d17421f8832b00cfde13, matching the brief. The final reviewed commit is 4092413287dab57f4c5b947730729d48464c6ccb. Since the design landed, only its round-two brief and the goal record changed; the reviewed implementation sources did not.
- (read) rg -n over non-test Go files for History appends, t.Done accesses, Live-to-Done moves, GoalFile pointer collections, touch calls, and Render => No indirect pointer can append done or split after a record's archive act. The done path retains its former live pointer after inserting it into Done only to append that goal's own done line. Split retains its archived parent pointer only to update the existing split line's targets. Reconcile re-looks up an archived record by identifier, requires the current reconcile operation to have archived it, and can append only edit. Its archive-side compaction mutation is guarded by the same operation identifier. Arc and member collections contain only live records, and compaction pointers are constructed from the current Live map.
- (inferred) nl -ba metasystem/internal/metrics/compute.go; nl -ba metasystem/internal/metrics/model.go; nl -ba metasystem/internal/metrics/report.go => The waiting calculation is last landing minus claim for building time, conclusion minus last landing for proving time, and proving divided by claim-to-conclusion time for waiting share. For the split parent, claim at T0, landing 12 hours later, the incorrect T2 conclusion, and the correct T3 split produce exactly 12 building hours, proving time changing from 12 to 36 hours, and waiting share changing from 0.500 to 0.750. The T2 and T3 windows also select the old done line and new split line exactly as specified.
- (read) nl -ba metasystem/internal/goal/order_test.go; nl -ba metasystem/internal/goal/verbs.go; nl -ba metasystem/internal/goal/split.go; nl -ba me => All three writer-premise groups describe current behavior. Done leaves survivor c live and writes matching operation identifier, timestamp, verb, actor, and targets on departed b and survivor c. Split archives its parent on a split line without a done line while survivor c remains live with a matching split compaction line. Concluding b and then splitting surviving c yields exactly one earlier done compaction line followed by c's own split archive line.
- (ran) rg -n '\bhistoryTime\(|\bconcludingEpoch\(|\bConcludedInWindow\(' metasystem/ --glob '*.go'; nl -ba metasystem/scripts/agents/go-gate.sh => historyTime has exactly three production callers: goalBounds, selectedGoals, and ConcludedInWindow. concludingEpoch has its own scan and is called only by computeWaiting. Revision 2 deletes both old scans and routes all four semantic readers through concludedAt. ConcludedInWindow has no caller in the tree; retaining this exported function is honest and does not leave the unexported unused declaration that staticcheck would reject.
- (read) nl -ba metasystem/internal/goal/file.go; nl -ba metasystem/internal/goal/verbs.go; nl -ba metasystem/internal/goal/approval.go; nl -ba metas => ParseFile rejects an approved goal without an approval record, without a budget, or with an invalid approval binding or digest, and loadTree refuses those parse problems before Next. Expiry is classified as Awaiting before admission. GOAL_NORM_REFUSED is therefore the only production refusal reaching this frontier. Every proposed Refused fixture creates a valid approval over an over-box budget and does not rely on the unreachable APPROVAL_REQUIRED path.
- (ran) git diff --unified=5 8ca43596..0260aef6 -- metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md => Revision 2 changes the refusal section only to correct the production cause census and fixture premise. The Refused category, storage, reader behavior, output mapping, and unchanged owners remain the revision-one decision.
- (ran) git status --short; test -e metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r2.md => The requested round-two register file is absent. The only working-tree modification reported at completion is the unrelated, user-owned metasystem/records/narrator-digest.log, which was not touched.

## Gaps the critic named

- The requested register file metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r2.md could not be created because this critic has a read-only workspace and its role forbids repository edits. Consequently, the declared-output SHA-256 digest 31b6d07c615458df13fe11322a71e0e4ebea4e9b96dfc93179e62825d3beb65d could not be verified against a persisted artifact.
- No fixture bed was run, as required by the brief. The canary outcomes and exact numbers were recomputed from source rather than observed at runtime.
- The launcher classifies this broad-read job as advisory and cannot prove context isolation or independent examination. No runtime session identifier was exposed.

## Coordinator disposition (m1b, 2026-09-09)

No findings; the design loop's stop criterion is met. The read went to the
page's own riskiest part first, the enumeration behind the archive-line
rule, and found no indirect pointer that can append an archive verb after
the archive act; it recomputed the split-parent canary numbers from the
waiting-metric formulas rather than trusting the page's hand arithmetic;
and it confirmed the writer-premise pins describe current behaviour, which
is what makes the metrics fixtures honest copies rather than inventions.

Design CLOSED at revision 2. Fold-read cycles: 1. The land-or-fold call did
not need to reach Wido. Next: the build behind section 3, briefed to the
implementation lane from the page, canaries first and red on the untouched
tree, premise pins green first; the process-owning beds run on the
orchestrator's side.
