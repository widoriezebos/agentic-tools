# Sol's read of the launch contract build, round 1

Produced 2026-09-28 by Codex on `gpt-6-sol`, read-only, against branch `app-launch-contract` at `2a33fe79c` (base `149c3b7a2`), brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/sol-code-read-brief-app-launch.md`. Verbatim; the dispositions are in the design's Built section.

---

# Code review: application launch contract

**Five material findings. I would not certify step 1 yet.** The diff builds the seven verbs and most of the specified supervisor and testing bridge, but the paths below can produce a false lifecycle answer, lose goal evidence, or run the wrong candidate. I read the changed functions at whole-function depth against the design and `149c3b7a2..2a33fe79c`; I made no edits.

## Findings

**SOL-01 — High · material: yes. Group inspection can certify a live descendant as gone.** `KernelGroup` discards a process in the recorded group when its arguments cannot be read or its identity probe is uncertain; the supervisor also treats a group-read error as an empty group and can write an ended record. Stop uses that same filtered membership to prove death. [Group inspection](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/state.go:90), [supervisor completion](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/supervise.go:290), [stop proof](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/stop.go:286). A foreground wrapper can leave a descendant alive while status says ended and stop removes the record. Keep uncertain members in the liveness census and refuse an empty-group proof until their death is known; never signal an unidentified member. **R-124:** the first app that leaves a descendant can receive a silent false stop answer, so the slice is not safe without this fix.

**SOL-02 — High · material: yes. Equivalent `--at` spelling loses a goal run’s evidence.** `appRef` assigns a goal only for `--goal G`, though it maps both forms to the same run key. Evidence copying checks the invocation’s `run.goal`, not the recorded `Goal`; `endRun` then removes the record. [Run selection](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/intent_app.go:108), [evidence copy](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:411), [record removal](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:450). Start with `--goal G`, then stop or replace the ended run with `--at goal/G`: its log tail and last check are not copied. Copy under `record.Goal` whenever the existing record names a goal. **R-124:** this loses review evidence on a public form declared equivalent by the design.

**SOL-03 — High · material: yes. A missing evidence root is treated as a successful copy.** `preserveRunEvidence` prints “nothing was copied” and returns `nil` when `evidence.root` is absent or nonabsolute; `endRun` then deletes the goal record. [Evidence copy](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:411), [record removal](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:450). A goal run can start without `settings check`, then stop and permanently lose its last check. Refuse closure until the required copy succeeds, leaving the record intact. **R-124:** first use with incomplete configuration loses data rather than failing safely.

**SOL-04 — Medium · material: yes. `--goal G` can run a stale local branch.** `resolveCommitFor` resolves `goal/G` before `origin/goal/G`; the moved-tip check uses the same function. [Commit resolution](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:204), [replacement decision](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/intent_app.go:369). If both refs exist and the local one lags, start runs the old commit and says it is the goal candidate. Resolve the goal’s origin tip for `--goal`, and use that commit for both replacement and checkout. **R-124:** a reviewer can inspect the wrong build on the first candidate run.

**SOL-05 — Medium · material: yes. An occupied standing address can yield false readiness.** Address allocation checks availability for candidate ports but assigns the standing address without checking it. `AwaitReady` accepts a successful HTTP or TCP probe before checking whether the spawned child exited. [Address allocation](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:343), [readiness decision](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/probe.go:101). If another service already answers the configured health URL, start can report that service as the new app while the new child fails to bind. Refuse a taken standing address before launch and require the owned child to remain alive when readiness is accepted. **R-124:** this gives a wrong “started and answering” result on an ordinary developer-host collision.

**SOL-06 — Low · material: no.** On an unreadable readiness response, `LaunchSupervisor` calls `child.Kill()` rather than `SignalExact`, departing from the design’s literal no-bare-number rule. [Launcher](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/launch.go:50). The direct child remains unreaped at that call, so its PID cannot yet have been reused; this does not make the first use unsafe. An exact-reference signal would bring the code into literal conformance. **R-124:** defer.

## Departures from the design

| Design rule | Implementation |
| --- | --- |
| Keep the supervisor until its group is empty and prove every member dead ([D2](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/plans/designs/app-launch-contract.md:199)). | Uncertain group members can disappear from the count, and a read error can end the wait ([SOL-01](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/supervise.go:290)). |
| `--goal G` is the goal branch’s tip, and `--at goal/G` is its equivalent ([D3](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/plans/designs/app-launch-contract.md:251)). | A local branch wins over origin ([SOL-04](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:204)); the equivalent spelling does not retain recorded goal evidence ([SOL-02](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:411)). |
| Copy a goal run’s evidence before removing its record ([D6](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/plans/designs/app-launch-contract.md:279)). | No configured absolute evidence root skips the copy yet permits removal ([SOL-03](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:415)). |
| Refuse and name an occupied address ([self-grade](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/plans/designs/app-launch-contract.md:367)). | The standing address is not checked ([SOL-05](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:351)). |
| Never signal a bare number ([D2](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/plans/designs/app-launch-contract.md:231)). | The launcher’s direct-child kill is the nonmaterial exception ([SOL-06](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/launch.go:77)). |

I found no undisclosed extra product feature in the changed paths. The builder disclosed that a real end-to-end `app check` against a testing group was not run; the linked checks cover its argument, parsing, environment, and identity separately.

## Deferred

SOL-06 can be aligned with the exact-reference convention after the material fixes. The candidate’s contract is read from the invoking checkout, an interpretation the builder disclosed and the design does not settle. Neither point changes the first-use verdict here.

## Verification and limits

By reading against `149c3b7a2`, I checked three of the builder’s failed-first claims:

- `TestStartAloneIsAWorkingContract` cannot compile at base: the entire `internal/applaunch` package was added by this diff. [Test](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/contract_test.go:22).
- `TestAppContractOfStartAloneWorksWithEveryVerb` cannot compile at base: `intent_app.go` and the `app` registration were absent. [Test](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app_verbs_test.go:607).
- The extended settings test would fail at base: it expects invalid `launch.json` to be refused, while the base `runIntentSettingsCheck` returned after testing-contract validation. [Assertion](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/intent_questions_test.go:444), [new validation](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/intent_work.go:1464).

The read also supports the record-before-spawn order, separate liveness and readiness, log-offset readiness, start-only contract test, fresh canary arguments, address overlay in both environment modes, and coverage entries in both ratchets. I read the builder’s reported green static gate and self-hosted run as **reported evidence**, not independent execution. `git diff --check 149c3b7a2..HEAD` produced no diff-error output; Git did print sandbox cache warnings. I did not run Go tests or reproduce process behavior in this read-only lane, and did not check every failed-first test.

**VERDICT: 5 material findings: SOL-01, SOL-02, SOL-03, SOL-04, SOL-05.** Commit read: `2a33fe79cf18664a8c8499d6f0cb2658db720475`. Limitations: static code read and sampled base-test comparison only; no independent runtime test.

Codex session ID: 01a0e7b0-c48f-7233-93da-28e9a2fb186f
Resume in Codex: codex resume 01a0e7b0-c48f-7233-93da-28e9a2fb186f


---

# Sol's re-read of fix round 1 (2026-09-28), against `d77aa1287`. Verbatim. Three fixed, two held; fix round 2 follows.

# Code re-read: application launch fix round

**Two material findings remain: SOL-01 and SOL-02.** The fixes for SOL-03 through SOL-05 address their reported failure modes. I read the four-commit diff through `d77aa1287` and the changed functions; I made no edits.

| Finding | Result | Evidence |
| --- | --- | --- |
| **SOL-01 — group death proof** | **HELD** | The census now retains uncertain members, and a group-read error no longer ends `awaitGroupEmpty`. But [the readiness-failure path](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/supervise.go:248) calls `finish`, whose bounded loop [writes an ended record after `stopMs` even if members remain or inspection still fails](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/supervise.go:329). [Status trusts that ended marker before checking the group](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/state.go:185). A start command that leaves a descendant and exits before readiness can therefore leave a live process under an ended record; the next start may remove that record. **R-124: material**—the first such app is neither safely owned nor accurately reported. The builder’s supervisor test fails against the base as reported, but [it checks before cancellation and makes no record assertion after the supervisor finishes](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/census_test.go:189). |
| **SOL-02 — equivalent goal spelling** | **HELD** | The record’s goal now controls copying when a run was started with `--goal G` and stopped with `--at goal/G`; the named test addresses that path and would fail at the base. But [a start using `--at goal/G` still records an empty goal](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/intent_app.go:108), and [evidence copying returns without a copy when both the record and invocation have no goal](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:427). Stopping that run with the same `--at` spelling removes its record. **R-124: material**—a documented public spelling loses the first goal review’s log and check evidence. |
| **SOL-03 — missing evidence root** | **FIXED** | [An absent or nonabsolute `evidence.root` now returns an error](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:438) before record removal. The named retry test’s refusal assertion would fail against the base. |
| **SOL-04 — lagging local goal branch** | **FIXED** | [Goal refs resolve the origin tip first](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:209); start and moved-tip detection use that resolution. The named test’s commit assertion would fail against the base’s local-first order. |
| **SOL-05 — occupied standing address** | **FIXED** | [Start checks the standing address before launch](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:353), and [readiness checks child liveness before accepting a probe](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/probe.go:122). Both named assertions would fail against the base paths. |

**New defects introduced by the fixes:** none found in the changed paths, so no SOL-07 identifier is assigned. SOL-06 remains deferred and unchanged.

**Builder’s noted `--at goal/G` gap: yes, material under R-124.** The design and public flag describe `--goal G` as sugar for `--at goal/G`. Losing evidence solely because the user chose the equivalent spelling violates that first-use contract; this is the remaining SOL-02 gap, counted once.

**VERDICT: 2 material findings: SOL-01, SOL-02.** Commit read: `d77aa1287682713fd9c848454f0f3b3a3ca60205`. Evidence level: code and diff read; the builder’s failed-first and green test results were reported evidence. `git diff --check` was clean. I did not independently run Go tests or reproduce the process scenarios. Proposed receipt, unwritten: “Fix-round read held group completion and equivalent goal-spelling evidence; three prior defects closed.”

Codex session ID: 01a0e7c7-ba95-7200-b292-3ca22b677d0f
Resume in Codex: codex resume 01a0e7c7-ba95-7200-b292-3ca22b677d0f


---

# Sol's re-read of fix round 2 (2026-09-28), against `65687bce8`. Verbatim. Both held findings fixed; no new material; the build is certified.

# Application launch: fix round 2

**SOL-01: FIXED. SOL-02: FIXED.** I found no new material defect introduced by the two fixes.

| Finding | Result | Evidence and R-124 judgment |
| --- | --- | --- |
| **SOL-01 — group death proof** | **FIXED** | **Read:** [finish](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/supervise.go:342) writes `Ended` only after the group reads empty; a live member or read error keeps the supervisor waiting. The readiness-failure path also calls `finish`, and a signal there makes the supervisor end its own group. [Status](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/state.go:185) checks the group before trusting an `Ended` marker and reports a live or unreadable group as `child-ended`, which prevents the next start from removing its record. **R-124:** the reported unsafe ownership and false-finished behavior no longer remains. |
| **SOL-02 — equivalent goal spelling** | **FIXED** | **Read:** [appRef](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/intent_app.go:109) derives `G` from `--at goal/G`, just as it does from `--goal G`. That goal reaches the [record and serve arguments](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:495); [evidence copying](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/app.go:427) uses the recorded goal. **R-124:** the documented spelling no longer loses a goal run’s evidence. |

**Failed-start consequence:** [LaunchSupervisor](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/internal/applaunch/launch.go:52) waits at most the supplied settle bound after a supervisor reports failure; `app start` supplies `stopMs + 5 seconds`, while other `Reap` callers retain two minutes. A supervisor still owning a descendant is left alive. [appStart](/Users/wido/LocalStorage/GitHub/agentic-tools-alc/metasystem/cmd/metasystem/intent_app.go:358) then returns a failure with status lines that identify the live descendant. Its existing next-step hint says “before it stopped,” which is inaccurate in this case, but the status and refusal of another start preserve the first use’s safety. **R-124: nonmaterial wording; it does not hold SOL-01.**

**New defects from these fixes:** none found; no SOL-08 identifier assigned.

**VERDICT: 0 material findings: none.** Commit read: `65687bce89205f285fadc9f9bb45f3b0dfa30c24`, including the diff from `d77aa1287` and both intervening commits. `git diff --check` was clean. The builder reports green focused tests; my independent Go test attempts could not start because the read-only sandbox denied Go’s temporary build directory. I made no edits and did not reproduce the process scenarios independently. Proposed receipt, unwritten: “Fix-round re-read closed group death proof and equivalent goal spelling; no new material findings.”

Codex session ID: 01a0e7df-addb-7ac2-90ce-437ede400171
Resume in Codex: codex resume 01a0e7df-addb-7ac2-90ce-437ede400171
