# stop-decisions-record-deadline-evidence

- State: claimed
- Risk: severity=2 novelty=3 exposure=3 accumulation=2 basis="severity 2: a refusal emitted without a durable decision behind it can neither be audited nor drained, and a decision recorded twice per deadline double-counts an incident; novelty 3: a version-2 stop decision record shared by the hook, its deadline parent and the steward is a new durable format with a new identity (turn generation plus deadline end); exposure 3: every stop of every seat; accumulation 2: the record is read by the steward's drain (member stop-incidents-reach-the-steward) and by the seven-day count"
- Tier: 3
- Intent: Member 2 of stop-hook-never-forces-an-empty-turn (its design page, section 7; this member's page plans/stop-decisions-record-deadline-evidence-design.md): each emitted refusal has a prior durable decision with its full up result and the checkout generation, and each incident episode is keyed to the hook's turn generation and deadline end, so the deadline parent and the worker never record the same stop twice and a later drain can tell episodes apart. DONE means: (1) the stop decision record (version 2) is written before any refusal or degraded notice is emitted, carrying the class, cause, component, the full up result as up prints it, the checkout's enrollment generation, the turn generation and the deadline end; (2) an episode is keyed by turn generation and deadline end, and the deadline parent's completion of a worker's decision is single-use: a second completion for the same key changes nothing and says so; (3) an emission whose decision could not be written is itself recorded in the hook log with the write failure and the notice says it; (4) TestArmingDetailSurvivesStop keeps passing: the arming detail reaches the notice byte for byte from the record. Fixtures: TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission, TestStopDeadlineCompletionIsSingleUse, TestArmingDetailSurvivesStop.
- Origin: main
- Next step: 2026-09-14 m1b: build round 26 in flight. Round 25 is correct: it restored the ordinary decision trail line and the infrastructure condition line and pinned each delivered route's exact log lines in a Go test. The seat first saw both legs still failing on a clean worktree of the tip, then kept the evidence and found both lines present; the beds' assertions pipe sed into grep -q under pipefail, and grep exiting at its match gives sed SIGPIPE (measured sed=141 grep=0, 3 of 3 on the chat-line log and 5 of 5 on the nested-root log; here-string form true). Round 25 made that race deterministic by writing the human lines before the durable JSON events. Round 26 converts every early-exiting-reader pipe in supervision-hook-fixtures.sh; M1c owns the class (pipelines-never-lose-a-truncated-producer) and the supervision-fixtures.sh sites. Tip-owned supervision failures excluded from this chain: stop-hook-monitor (same race), rearm-provenance (archive pipe, M1c), wait-restart and wait-stop-fake (a command-package capture helper deadlock from 57ba0586; a no-goal direct fix m1b-capture-a-1789409135 returned green and is being verified for landing). Then the rostered Opus read (brief refreshed), the delivery proof and the landing.
- OpenedAt: 2026-09-13T07:33:02Z
- Revision: 30
- Budget: elapsedLimit=2d attemptLimit=42 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 3
- Approved: by=human:Wido at=2026-09-14T16:47:25Z revision=27 opid=MHRQ4V49VF8E71RDWZTW4GC17M-m1b-c6925449 authority=proven digest=cf7a9b57e9452c56ee9397937c5c988e1a0e6278a7e04b334c13c16d18748232
- Sliced: machine=m1b lineage=main-1789191336-90295-e4b24b revision=3 at=2026-09-13T07:34:48Z
- Claimed: machine=m1b lineage=main-1789191336-90295-e4b24b at=2026-09-14T16:47:25Z revision=27 accountingRevision=27 episodeAt=2026-09-13T07:33:09Z episodeRevision=3 idleSeconds=77491
- StopCapability: generation=27 revision=27 machine=m1b claimEpoch=2 fenceEpoch=0

History:
- 2026-09-13T07:33:02Z 5HG893JMDPRP75G97Y7F9VH7Z8-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence,stop-hook-never-forces-an-empty-turn
- 2026-09-13T07:33:06Z FFGKF0S19KMMHEQGBWCYDSMNY5-m1b-30a7e141 approve actor=human:Wido targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:33:09Z XQ7QJ39E7D41YY8VYERHA1QJS0-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:34:48Z 1V6ZNPPCN7F28F6GZP8K8ZWF1B-m1b-30a7e141 slice-start actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:35:10Z DY0P1ET0V12XDX6SGPEKMY5HVC-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:51:36Z RBCG3XMYMYQEQJGXBXTXH7NKVW-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T08:18:07Z H2W3Y3JFJHQZCKHY4A67C2DF18-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T08:29:50Z 66603KZTYB1JCZTDW52M1N4M4N-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T11:47:57Z BNADWVQCJW9C16HE8C88VETA88-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T11:50:07Z WG1EVKYWX6H21VVP67BPF07KTZ-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T12:03:41Z R1PKMQR3EPA02CSRRM33M47E1S-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T12:03:45Z 2FDZKEDJ7W6WSST0JAWC5DFXEG-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T13:42:30Z C1WS9JPRN1T713KC5MQDMF8NX7-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T14:05:51Z 77M1J9DE68N6883F98DD4KXB3S-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T14:05:54Z JVZN3MCVBNA7XH02Y9BDXQX1KV-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T06:14:02Z 3E2BN9AGJ0FB2XWN2K33YA1ZW1-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T06:48:31Z 2K0ZJW6YE9CJAFH2MVVZCMTRPE-m1b-30a7e141 set-budget actor=human:Wido targets=stop-decisions-record-deadline-evidence
- 2026-09-14T08:46:53Z NYBRH2M8V7FJT60EDTADWGS9MY-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T10:21:24Z JJAK0N2007F6TEJBVXRCBRHZ28-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T11:45:09Z RDYFFB3TPGR1JEH37DGMX03DH4-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T12:34:57Z QH4C8ZXSVE8ME8QFD9NVXQ5Y7F-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T13:11:24Z A9VPY83WGXNM8AD9E0S6RQH2AY-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T14:24:30Z W8AXCYYKTPCAKXGV81PMPZJKH6-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T15:51:43Z DNW9DG3GFQWSWFFM8Y2PWG9HAT-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T16:25:57Z MQTKM6XNVEPNEY7J98EMK3VN26-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T16:45:56Z 6ETXP2P0HG2ZX57HGR0HG55PN9-m1e-c6925449 set-budget actor=human:Wido targets=stop-decisions-record-deadline-evidence displaced=m1b+main-1789191336-90295-e4b24b@2026-09-14T06:48:31Z
- 2026-09-14T16:47:25Z MHRQ4V49VF8E71RDWZTW4GC17M-m1b-c6925449 set-budget actor=human:Wido targets=stop-decisions-record-deadline-evidence displaced=m1b+main-1789191336-90295-e4b24b@2026-09-14T16:45:56Z
- 2026-09-14T17:04:20Z JN9BSG8SSNCZT03FEBHAQZQQF1-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T17:52:16Z XQF94GBQN2N5W4B0R9XJ4SE997-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-14T18:18:41Z 7D2YWE2AYB75R9PGNZG0Y6M4DF-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
Integrity: sha256=7d9aa50df52580a6b00a13cfe99a799c873f41826beb8b018020830a2f96818c
