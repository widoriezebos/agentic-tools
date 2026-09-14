# publication-owners-hint-the-waiter

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a dropped hint only delays a wait to its ten-second reread and a false hint triggers one read, but a wrapper that changes an exit mapping breaks every caller of job watch, run watch and delegate --wait; novelty 2: hints after durable writes and wrappers over a working verb are new, the owners and mappings are not; exposure 3: every seat's delegate and proof waits; accumulation 2: three publication owners, three wrappers and their fixtures"
- Tier: 2
- Intent: Member 2 of coordinator-wakes-on-events-not-polls (design page plans/coordinator-wakes-on-events-not-polls-design.md, section 6 row 2). DONE means: the job transition owner, the proof attempt's terminal commit and the confirmed ledger publication each hint the waiter after their durable write; a dropped or false hint changes no wait result; job watch, run watch and delegate --wait become wrappers over the installed metasystem wait and keep their current exit mappings exactly. Fixtures: the hint portions of section 5 row 4, TestWaitHintsOnlyTriggerReads, TestWaitCompatibilityMappings; bed legs wait-native-hint, wait-no-native and wait-compatibility for those assertions.
- Origin: main
- Next step: 2026-09-14 m1c: this is the first slice of the program Wido joined today - coordinator-wakes-on-events-not-polls with seat-work-continues-past-the-runtime-stop-cap - and it is the only thing standing between the fleet and a mechanical answer to 'a seat stops only when there is genuinely no way to continue'. It is approved, tier 2, unclaimed, and it is this parent's own member 2, so nothing external blocks the program. m1c takes it as soon as stop-refusal-fits-on-one-screen concludes (one claim per machine). Why it is now urgent rather than queued: the design critique of 2026-09-14 established from the runtimes that an ALLOWED Stop shows its text to the human and schedules no further model inference on Claude or current Codex, and that Devin has no report-bearing Stop mapping at all, so no Stop message can compel a seat that has been allowed to stop. Waits are the mechanism; the message is not. Wido also relayed approval authority to m1c for this program on 2026-09-14 - his message in the m1c session, not a verified human act - and any use of it will be named where it is used.
- OpenedAt: 2026-09-13T13:41:26Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=14 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T06:31:43Z revision=2 opid=AR3A65PKQ7EGPTV76Y1DQ2MGTV-m1b-30a7e141 authority=proven digest=7e140a1e2c9a97074aff3300d348e376ad5eb3853dea53f7ce00db74236111da

History:
- 2026-09-13T13:41:26Z CNPC58KYDPJJ4N34NVWCYTBFN9-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=publication-owners-hint-the-waiter,coordinator-wakes-on-events-not-polls
- 2026-09-14T06:31:43Z AR3A65PKQ7EGPTV76Y1DQ2MGTV-m1b-30a7e141 approve actor=human:Wido targets=publication-owners-hint-the-waiter
- 2026-09-14T10:33:10Z KJ606R54SXVDTVA64S1ZJGG7P3-m1c-69c9e454 edit actor=m1c+main-1789191340-90689-8975a9 targets=publication-owners-hint-the-waiter
Integrity: sha256=2bb68e9f5f4fb35dfdeadca2ce2941523ddf46c5ee936218768ecf96e9f2a549
