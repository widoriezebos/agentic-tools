# stop-decisions-record-deadline-evidence

- State: claimed
- Risk: severity=2 novelty=3 exposure=3 accumulation=2 basis="severity 2: a refusal emitted without a durable decision behind it can neither be audited nor drained, and a decision recorded twice per deadline double-counts an incident; novelty 3: a version-2 stop decision record shared by the hook, its deadline parent and the steward is a new durable format with a new identity (turn generation plus deadline end); exposure 3: every stop of every seat; accumulation 2: the record is read by the steward's drain (member stop-incidents-reach-the-steward) and by the seven-day count"
- Tier: 3
- Intent: Member 2 of stop-hook-never-forces-an-empty-turn (its design page, section 7; this member's page plans/stop-decisions-record-deadline-evidence-design.md): each emitted refusal has a prior durable decision with its full up result and the checkout generation, and each incident episode is keyed to the hook's turn generation and deadline end, so the deadline parent and the worker never record the same stop twice and a later drain can tell episodes apart. DONE means: (1) the stop decision record (version 2) is written before any refusal or degraded notice is emitted, carrying the class, cause, component, the full up result as up prints it, the checkout's enrollment generation, the turn generation and the deadline end; (2) an episode is keyed by turn generation and deadline end, and the deadline parent's completion of a worker's decision is single-use: a second completion for the same key changes nothing and says so; (3) an emission whose decision could not be written is itself recorded in the hook log with the write failure and the notice says it; (4) TestArmingDetailSurvivesStop keeps passing: the arming detail reaches the notice byte for byte from the record. Fixtures: TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission, TestStopDeadlineCompletionIsSingleUse, TestArmingDetailSurvivesStop.
- Origin: main
- Next step: 2026-09-14 m1b: build round 24 in flight. Rounds 22 and 23 verified outside the delegate sandbox on clean worktrees of the tip: the corrupt-refusal-record leg and the malformed-verdict-state leg pass; the hook bed reaches line 1604 of 2263 (the killed-worker route owes its 'outcome=invalid-worker-output-allow' trail line; round 24 adds it and sweeps every delivery route's trail line). Round 24 also fixes a landing bed defect that is the tip's, not the build's: the seeding pipe 'git archive | tar -x' gets SIGPIPE on git for some tree sizes under pipefail (git=0 at 67adc663, git=141 at 113b3bcf and 6921c8a8), so section/land-fixtures is red for every seat until file-based seeding lands; M1c and M1e were told and will not touch the file. The attempt budget was raised to 42 in Wido's name inside the tier box; the first raise ran from another seat's checkout and re-bound the claim to that lease's epoch, refusing every dispatch until it was redone from this checkout. Thirty-three build defects found by the beds and fixed since round 6. Then the rostered Opus read, the delivery proof and the landing.
- OpenedAt: 2026-09-13T07:33:02Z
- Revision: 28
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
Integrity: sha256=45033e0dc5885d3b36e3f06a2101ab1c886ebff5666618b4229845840f5006c0
