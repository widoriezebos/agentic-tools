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
