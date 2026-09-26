# Manual-review critique dispositions

Root owns the design and dispositions; Fable reports findings. Round 1 reviewed
SHA256 d5c739fce4904f2cc9abfcdbf82a93c50f52268104de16aebd2f43143b1d4525,
retained in commit 28a4761a6. The report was written concurrently with that commit;
its stated 55229e925 baseline is the read-start checkpoint, not the final Git tip.
Provider session a0fd8140-8bd1-4cea-ac45-4fb2604f1d18, launch
20260926t095707-28b7fc9e0f, actual 7m6.1s, 20 model calls, 19 tool calls.
The report's own tool-count estimate differs; launch measurements govern.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IM-C1 | accepted | Existing range/CommitStaged/Push own installed work and push recovery; a second manual registry and invented ordinal duplicate those owners. Root rejects the overly broad suggestion that an empty index or whole-branch tree alone proves replay: source may be a different dirty checkout, branch tip contains later units/read records, and CommitRequest.OpID is not in unit trailers (commit.go:234-245). Compare against the selected unit's actual parent/current version under the existing checkout token. | Deleted manual registry; manual correction names --after COMMIT supplied publicly; exact per-unit candidate comparison and existing Push reconciliation. |
| IM-C2 | accepted | commit.go:327-345, 407-443, 489-507, 565-570 consume mutually consistent staged paths, patch and index. Patch-only injection is unsafe. Index-only apply to a distinct destination also leaves working files behind the staged tree; use --index there, checked staging of already-present captured paths when source equals destination. | Deleted CommitPatch; stage and call unchanged CommitStaged under its existing token; preserve/refuse foreign data and staged conflicts. |
| IM-C3 | accepted | Subject words collide with valid goal ids. The report's proposed review G --work NAME still spells review changes and does not resolve that collision. | Explicit review goal G for all goal ids, reserved subjects, collision-safe continuations and tests. |
| N1 | noted | Read-partition extraction must preserve the existing build behavior; parent contract already requires preserved capabilities. | Existing unit-read tests remain required; no extra framework. |
| N2 | noted | Outputs must survive temporary reader checkout cleanup. | Existing launch custody remains the report owner; cleanup follows retention, not vice versa. |
| N3 | noted | A linked worktree shares Git configuration, so absence of ignored files is not filesystem isolation. | Precision correction: no ignored source files/conf.local, no sandbox claim. |
| N4 | noted | Source-equals-destination has documented commit effects and must keep current owner guards. | Existing explicit contract retained and staging specified. |
| N5 | noted | Amendment already removes the target read and replays later work. | Existing owner retained. |
| N6 | noted | Diagnostic read cannot write Goal-Read or closure. | Existing explicit prohibition retained. |

Round 2 is the final declared round and challenges the smaller owner composition,
particularly concrete staging/replay and no invented counter. No third prose round.

## Final round and implementation exit

Round 2 reviewed SHA25680e94a9d3f832e9bacb107efe6184243cffdfdcb22f54341eb0b662c360b7f29.
Report read in full. Launch20260926t100811-91db57f15c, same provider, actual5m28.9s;
cumulative30 model/28 tool calls, 9 new tools and10 current turns. MATERIAL2;
trajectory3->2, both bounded clauses. No third prose round.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IM-C5 | accepted | gittree.Snapshot deliberately scopes its Workspace.Dir prefix; complete-checkout capture must construct it at Git top level. | Explicit top-level capture, paths and replay; TestManualCaptureFromNestedCheckout (IM-1). |
| IM-C6 | accepted | The destination is persistent; copying scratch --3way could leave conflict stages. Plain --index --binary checks all hunks first. | Explicit no --3way in destination, scratch unchanged; TestManualStageRefusalPreservesCheckout (IM-3). |
| IM-C4 | accepted | Root strengthens the critic's non-material classification: the new stage-plus-commit section mutates shared index/files before update-ref CAS, so publication CAS alone cannot protect those inputs from concurrent staging/rollback. withGoalBranchCommitTokenAt:1025-1046 writes/removes one identity file and takes no flock. lease.LockBounded is the existing bounded exclusion primitive; RequireHolder normally reads an already-established holder. | Establish holder first, use existing checkout-mutation lock and recheck, mint token inside it; shared goal commit wrapper serializes all relevant calls, no nested lock or lock across push/review. Named concurrent case in TestManualSubmissionReplayAndAmend (IM-4). This is bounded existing-owner locking, not a new workflow store. |
| IM-C7 | accepted | Safe refusal is insufficient for the explicit replay contract after adoption. CommitStaged uses scratch --3way; plain Apply differs. | Reuse owner scratch application for per-unit tree comparison; adopt-then-resubmit case in TestManualSubmissionReplayAndAmend (IM-4). |

Closed at final round on the named executable obligations, plus mandatory full Sol
code critique. Root agrees on the smaller implementation: no manual registry,
no invented attempt counter, no CommitPatch, no alternative attestation system.
If code reveals lock reentrancy or lost-input defects, correct them before calling
IM-4 done; the fixture exit does not certify unbuilt behavior.
