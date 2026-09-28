# Astra's critique of the app launch contract, round 1

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against [app-launch-contract.md](app-launch-contract.md) at `647234149`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-app-launch.md`. Verbatim; the dispositions are at the design's foot.

---

**4 material findings.** Reviewed `plans/designs/app-launch-contract.md` at commit `6472341493bc39eab8839b8d5564311a5f581851`, round 1.

**Report not saved:** this session’s read-only filesystem permissions prevent writing `/Users/wido/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-r1-app-launch.md`. The complete report follows.

All evidence references below are repository-relative and refer to `647234149`. Failure cases are static deductions, not executed reproductions.

## Findings

### ALC-01 — High — material: yes

**Claim:** D2’s proposed cancellation precedent can signal an unrelated process after a recorded process-group identifier is reused.

**Evidence read:** `plans/designs/app-launch-contract.md:124–131`; `metasystem/internal/launch/launch.go:532–578`; `metasystem/internal/launch/process.go:103–121`; `metasystem/internal/identity/ref.go:131–150`; `metasystem/internal/ui/lifecycle/stop.go:26–71`.

**Concrete failure:** An app and its supervisor die, leaving a nonterminal record. An unrelated process later leads a group with the recorded numeric identifier. Following `launch cancel`, stop sends TERM to that group before checking recorded identities; its later death checks cannot undo the unauthorized signal. The existing cancellation implementation therefore does not establish the identity-at-use guarantee cited in §1.

**Design change:** Require native `identity.Ref` identities for recorded processes and identity verification immediately before every signal, including escalation; unknown identity must refuse signaling. Explicitly exclude cancellation through an unverified saved numeric process-group identifier.

**Test 1 — DIFFERENT/WRONG:** yes; changes the record and signal authorization.
**Test 2 — WORKS/SAFE without it:** fails **SAFE**; stop can kill a process it does not own.

### ALC-02 — Medium — material: yes

**Claim:** D2 leaves the detached app’s ownership handoff undefined when the initiating engine crashes before publishing its record.

**Evidence read:** `plans/designs/app-launch-contract.md:122–128`; `metasystem/internal/boundedexec/boundedexec.go:84–121`; `metasystem/internal/launch/launch.go:173–224`; `metasystem/internal/ui/lifecycle/serve.go:159–180`.

**Concrete failure:** Following the stated sequence, the engine detaches the foreground app and dies before writing the run record. The app survives and serves, but the next status has no record and reports “not running”; stop cannot identify its process. The cited `boundedexec.Run` does not supply a durable detached owner: it waits for completion and kills the process group at its deadline.

**Design change:** Name the app lifecycle owner and require a durable ownership handoff before the app can survive its initiating command, with interrupted starts reconciled by subsequent verbs. Distinguish readiness/build/stop deadlines from the lifetime of the serving app.

**Test 1 — DIFFERENT/WRONG:** yes; changes launch sequencing, persistence and ownership.
**Test 2 — WORKS/SAFE without it:** fails **WORKS** and **SAFE**; an interrupted first start can leave an uncontrollable app and a silently false status.

### ALC-03 — Medium — material: yes

**Claim:** D2’s exhaustive three status answers cannot truthfully describe an owned, live process whose readiness probe fails.

**Evidence read:** `plans/designs/app-launch-contract.md:127–135,181–187`; `metasystem/internal/ui/lifecycle/state.go:85–94,119–156`.

**Concrete failure:** The specified delayed-health fixture is alive and recorded but still returns a non-2xx response. A status call during that first startup cannot truthfully return any prescribed answer: it is neither “running … and answering,” “not running,” nor a stale record whose process is gone. The same state occurs when an already-started app stops answering.

**Design change:** Report process liveness and probe readiness separately, allowing “process alive, not ready/not answering” without discarding ownership. Rejoining an existing start must retain the readiness requirement before reporting successful startup.

**Test 1 — DIFFERENT/WRONG:** yes; changes observable status and start outcomes.
**Test 2 — WORKS/SAFE without it:** fails **WORKS** and **SAFE**; ordinary startup permits a false status answer.

### ALC-04 — Medium — material: yes

**Claim:** D1’s requirement that the probe go dark cannot prove stopping for its supported log-pattern readiness form.

**Evidence read:** `plans/designs/app-launch-contract.md:102–117,157–160`; `metasystem/internal/ui/lifecycle/launch.go:88–101`.

**Concrete failure:** A foreground app writes `READY` to its own log, becomes ready, and later exits successfully through its declared stop command. Every owned process is dead, but the retained log still matches `READY`. Requiring both a dark probe and a dead tree prevents successful stop—and therefore restart—even on this first normal lifecycle. Truncating the application log would destroy evidence.

**Design change:** Define log readiness as a startup observation scoped to the current run, not a reversible liveness probe. For log/none readiness, prove stop through owned-process death; specify separately when network-probe disappearance is also required.

**Test 1 — DIFFERENT/WRONG:** yes; changes readiness evaluation and stop completion.
**Test 2 — WORKS/SAFE without it:** fails **WORKS**; a supported readiness form cannot complete its normal stop contract.

## Deferred and non-material

### ALC-05 — Low — material: no

**Claim:** The existing testing merge driver is specific to testing contracts, not reusable unchanged for arbitrary JSON contracts.

**Evidence read:** `plans/designs/app-launch-contract.md:118–121`; `metasystem/internal/testpolicy/contractmerge/merge.go:45–90`.

**Concrete failure:** Passing a launch contract to `MergeBytes` sends it through `testpolicy.Decode`, rather than a launch-schema decoder.

**Design change:** Correct the reuse claim; ordinary Git merging is sufficient for Step 1. Defer a launch-specific semantic merge driver until needed.

**Test 1 — DIFFERENT/WRONG:** yes, if literal driver reuse was intended.
**Test 2 — WORKS/SAFE without it:** passes both; semantic merge support is unnecessary for the first app lifecycle.

### ALC-06 — Low — material: no

**Claim:** Mirroring testing-tool declarations does not inherit rejection of an unsupported tool version.

**Evidence read:** `plans/designs/app-launch-contract.md:89–92,176–179`; `metasystem/internal/testpolicy/contract.go:74–78`; `metasystem/cmd/metasystem/test.go:2769–2788`.

**Concrete failure:** An executable exists but has an incompatible version. The existing readiness check resolves executable availability; the declaration contains no accepted-version constraint.

**Design change:** Distinguish availability checks from version policy. Defer richer version constraints rather than inventing them to support the analogy.

**Test 1 — DIFFERENT/WRONG:** yes; changes what validation promises.
**Test 2 — WORKS/SAFE without it:** passes both for the configured first application; earlier diagnosis of an incompatible installation is preflight completeness.

### ALC-07 — Low — material: no

**Claim:** The bundle freshness test exists, but a Go build and successful health probe do not themselves execute that check.

**Evidence read:** `plans/designs/app-launch-contract.md:151–156,203–204`; `metasystem/internal/ui/web/bundle_test.go:104–140`; `metasystem/internal/ui/web/embed.go:10–16`; `metasystem/internal/ui/httpd/httpd.go:616–634`.

**Concrete failure:** A tree containing changed frontend source and an older committed bundle can compile and answer health while serving that committed bundle.

**Design change:** Qualify §6’s assurance: freshness is established by running the existing test against the candidate tree. This review does not promote that deferred assurance into another Step 1 mechanism.

**Test 1 — DIFFERENT/WRONG:** no for the specified build-with-committed-bundle behavior.
**Test 2 — WORKS/SAFE without it:** passes for the first use with a current committed bundle; broader candidate admission remains deferred.

D4’s browser integration, D7’s list and §6 are excluded from the material count.

## What I verified that holds

- **The contract premise is reasonable.** `settings check` already combines configuration validation with a project-owned command contract and tool availability checks (`metasystem/cmd/metasystem/intent_work.go:1440–1462`). A launch contract can retain project scripts while the engine owns lifecycle observation.
- **Self-hosting has an isolation path.** A binary built under the candidate’s `metasystem/bin/` resolves its own installation; lifecycle state and exclusivity are per resolved state root (`metasystem/cmd/metasystem/up.go:27–49`; `metasystem/internal/ui/lifecycle/roots.go:31–75`; `metasystem/internal/stateroot/stateroot.go:147–156`). No unavoidable collision with the standing UI was found.
- **The public grammar precedent holds.** The four `ui` actions have audience `both`, and intent commands receive `--repo` and `--json` centrally (`metasystem/cmd/metasystem/intent_process.go:179–199`; `metasystem/cmd/metasystem/intent.go:263–265`).
- **The proposed health endpoint exists**, and the existing bundle test detects source/bundle mismatch when run.

Limitations: static design and code review only; no tests, builds, process experiments, subagents or product edits. No private configuration or files under `artifacts/` were read. Deferred browser behavior was not exhaustively traced. The requested report file and receipt remain unwritten because filesystem writes are prohibited.

Proposed receipt: “Reviewed app-launch-contract at 647234149, round 1: four material findings concerning cancellation identity, startup ownership, truthful status and log-probe stop semantics; static evidence only.”

VERDICT: 4 material findings (fail test 2): ALC-01, ALC-02, ALC-03, ALC-04

Codex session ID: 01a0e6b3-8d93-73c1-8fb2-803da02f272e
Resume in Codex: codex resume 01a0e6b3-8d93-73c1-8fb2-803da02f272e


---

# Round 2, the declared failsafe (2026-09-28, same chain, `--resume-last`), against the folded design at `9eb7cf433`. Verbatim.

**One material finding remains: ALC-02. ALC-01, ALC-03 and ALC-04 are confirmed answered. No new material findings.**

Reviewed the complete folded design and dispositions at commit `9eb7cf433e1f081c0788b806b6bea185db6f94cb`. This is round 2, the declared failsafe, on the same chain. Scope remains Step 1: D1, D2, D3, D5 and D6.

All references below are repository-relative at that commit. Evidence was read; failure traces are static deductions, not executed reproductions.

## Round-1 dispositions

### ALC-01 — Confirmed — High originally — material: no remaining gap

**Claim:** The fold replaces cancellation by saved numeric group identifier with native process references and identity verification before every signal.

**Evidence read:** `plans/designs/app-launch-contract.md:158–176,270`; `metasystem/internal/identity/ref.go:131–150`; `metasystem/internal/identity/identity.go:193–222`; `metasystem/internal/ui/lifecycle/stop.go:26–71`.

**Failure addressed:** When another process occupies a recorded PID, `AliveRef` identifies the original process as gone, and `SignalExact` returns without signaling the replacement. The disposition expressly applies this requirement to escalation too.

**Design change:** None required for this finding.

**Test 1 — DIFFERENT/WRONG:** no further change required.  
**Test 2 — WORKS/SAFE without further correction:** passes both.

### ALC-02 — Held — Medium — material: yes

**Claim:** Introducing `app serve` supplies a persistent owner but does not close the interval in which that owner has spawned the application and has not yet recorded it.

**Evidence read:** `plans/designs/app-launch-contract.md:152–165,233–235,271`; `metasystem/internal/ui/lifecycle/launch.go:45–70,95–111,136–139`; `metasystem/internal/ui/lifecycle/serve.go:118–178`; `metasystem/internal/ui/lifecycle/state.go:133–156`.

**What the fold answers:** The serving application no longer inherits `boundedexec.Run`’s command deadline. A surviving detached supervisor can own it independently of the initiating CLI.

**Exact remaining gap:** D2 still starts the application before publishing its identity. The copied UI lifecycle owns the server process itself; the proposed lifecycle owns a supervisor whose child is the application. Killing the former directly ends the server. Killing the latter does not establish that its application child died.

**Concrete failure:**

1. `app start` launches supervisor S.
2. S launches foreground fixture application C in its own process group.
3. S dies after spawning C but before publishing the run record. Alternatively, the copied launcher’s readiness timeout kills S in that interval: `execChild.Kill` kills only its immediate process.
4. C survives. S’s lock is released, and no app record exists.
5. The copied inactive-state reader reports stopped, while C still runs; `app stop` has no recorded identity for C.

Consequently, “a run without a record is no run” is not established by the named owner. This is the original ALC-02 ownership gap.

**Design change:** Require recoverable ownership before the application can survive its launching owner, with startup failure and timeout retaining ownership information until child/group death is proven. Distinguish initiating-CLI failure from supervisor failure: a surviving supervisor may legitimately remain starting, while stopped requires evidence that the application is gone.

**Fixture obligation:** Interrupt the supervisor immediately after application creation and before record publication; assert that the application is either dead or remains discoverable and stoppable through recorded ownership.

**Test 1 — DIFFERENT/WRONG:** yes; changes startup ownership, failure cleanup and record publication/removal conditions.  
**Test 2 — WORKS/SAFE without it:** fails **WORKS** and **SAFE**; the first interrupted startup can leave an untracked application and a silently false stopped answer.

### ALC-03 — Confirmed — Medium originally — material: no remaining gap

**Claim:** The fold separates process liveness from readiness and explicitly represents starting and running-but-not-answering.

**Evidence read:** `plans/designs/app-launch-contract.md:165–171,228–232,272`; `metasystem/internal/ui/lifecycle/state.go:85–94,119–156`.

**Failure addressed:** A live fixture during its initial health delay now has a truthful starting/not-answering result. A second start rejoins the supervisor and must still wait for readiness.

**Design change:** None required for this finding.

**Test 1 — DIFFERENT/WRONG:** no further change required.  
**Test 2 — WORKS/SAFE without further correction:** passes both.

### ALC-04 — Confirmed — Medium originally — material: no remaining gap

**Claim:** The fold makes log readiness a run-scoped startup observation and limits probe-darkness requirements to HTTP/TCP.

**Evidence read:** `plans/designs/app-launch-contract.md:119–129,176–178,235–236,273`.

**Failure addressed:** After the application exits, a retained `READY` log line no longer prevents successful stop. The log can remain available for D6’s evidence copy.

**Design change:** None required for this finding.

**Test 1 — DIFFERENT/WRONG:** no further change required.  
**Test 2 — WORKS/SAFE without further correction:** passes both.

## Deferred and non-material

No new finding meets the materiality threshold; no ALC-08 onward is opened.

The remaining unconditional “probe going dark” wording at design lines 135–137 and “wrong version” validation wording at lines 220–223 are mechanical fold residue. The explicit rules at lines 124–129, 147–149 and 176–178 resolve their intended implementation. They remain deferred, without additional mechanism. **Test 1: no; Test 2: passes WORKS and SAFE.**

ALC-05 through ALC-07 retain their non-material disposition. D4’s browser half, D7 and §6 remain outside the material count.

## What verified holds and limitations

The named lifecycle code provides detached launching, a readiness pipe, lock-owned record publication, native process identity and identity-checked stopping. Its inactive-state reader distinguishes a held lock from an absent or stale run. These are suitable precedents, subject to ALC-02’s distinction between owning the application itself and owning its supervisor.

I read the full folded design, all dispositions, all four requested lifecycle files and `identity/ref.go`, plus the identity comparison and launcher tests. No tests, builds or process experiments ran. No files were changed, no subagents were used, and no private configuration or artifact contents were read. Proposed receipt, unwritten: “App-launch design round 2: ALC-01/03/04 confirmed; ALC-02 held; no new material findings; static review.”

VERDICT: 1 material findings (fail test 2): ALC-02

Codex session ID: 01a0e6b3-8d93-73c1-8fb2-803da02f272e
Resume in Codex: codex resume 01a0e6b3-8d93-73c1-8fb2-803da02f272e


---

# Round 3 (2026-09-28, fresh session with rounds 1 and 2 quoted via this file, under Wido's instruction to continue until nothing material remains), against the extended design at `265aa0118`. Verbatim.

**Three material findings remain.** Round 3 reviews Step 1—D1, D2, D3, D5 and D6—at `265aa0118cec13d88d5b3eb9555be739ad92de16`.

Evidence below was read at that commit. Failure traces are static deductions, not executed reproductions. “Design” refers to [app-launch-contract.md](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/app-launch-contract.md).

**ALC-08 — Medium — material: yes — The default stop cannot stop a valid foreground process tree**

**Claim:** Omitting `stop` selects a fallback that signals the direct child and supervisor, although the application can include surviving descendants.

**Evidence:** Design lines 118, 146–153, 185–189 and 208–214. `metasystem/internal/identity/ref.go:131–150` signals the identified PID; it does not recursively terminate descendants. The copied `metasystem/internal/ui/lifecycle/stop.go:26–71` owns the serving process itself.

**Concrete failure:** A valid foreground wrapper starts application C and waits for it. The supervisor records the wrapper. On `app stop`, TERM ends the wrapper without forwarding the signal; C remains in the group. The supervisor’s wait finishes and its orderly exit removes the record. Escalating against the dead wrapper cannot kill C. Stop cannot prove the required group disappearance, and a subsequent status can report stopped because ownership was removed. This does not require a start script that backgrounds its application and exits.

**Change:** Make the no-`stop` fallback terminate the owned application tree, preserving ownership until group death is proven. Use identity-proven ownership for descendant termination; do not restore signaling by an unverified group number. Add a foreground-wrapper fixture whose descendant survives TERM to its parent.

**Test 1 — DIFFERENT/WRONG:** Yes; changes termination targets, record-removal conditions and the stop fixture.

**Test 2 — WORKS/SAFE without it:** Fails **WORKS** and **SAFE**: an advertised start-only contract can leave its application running and subsequently produce a false stopped answer.

**ALC-09 — Medium — material: yes — `check` lacks the integration needed to check the selected live run**

**Claim:** Passing an address in the caller’s environment through the existing testing runner does not establish execution against that address, or fresh execution.

**Evidence:** Design lines 173–176 and 294–296. In `metasystem/cmd/metasystem/test.go`, lines 676 and 2869–2884 filter the environment through an allowlist containing no app-address variable. `metasystem/internal/proofrun/test_build.go:2752–2789` constructs explicit environments from the group’s declared map. `metasystem/internal/testpolicy/select.go:124–131` requires diagnostic canary mode for named groups. `test.go:1981–1994` can reuse group results; lines 337–340 expose existing fresh-execution controls.

**Concrete failure:** A named HTTP check reads the advertised address variable. Through ordinary runner invocation, the variable disappears; an explicit-environment group discards it independently. The check either fails immediately or falls back to another address and checks the standing app. Even after fixing injection, an earlier successful result can be reused after the application’s runtime state changes, without contacting it again.

**Change:** Specify the bridge to the existing testing owner: diagnostic named-group selection with prerequisites; an app-address binding that survives both environment modes and participates in execution identity; execution against the recorded run’s tree and identity; and fresh execution for each requested check. Store the actual group outcome and execution time against that run. Existing selection and freshness controls should remain the owners.

**Test 1 — DIFFERENT/WRONG:** Yes; changes runner invocation, environment construction, identity binding and result interpretation.

**Test 2 — WORKS/SAFE without it:** Fails **WORKS** and **SAFE**: the first check can target the wrong application, and a later check can silently report success without observing current runtime behavior.

**ALC-10 — Medium — material: yes — Orderly exit can erase the check before its promised evidence handoff**

**Claim:** The run record is the only specified owner of the last check, but the supervisor deletes it independently of the stop command that must preserve it.

**Evidence:** Design lines 175–176, 188–189 and 244–248. The copied lifecycle removes its record on exit at `metasystem/internal/ui/lifecycle/serve.go:178`; `state.go:133–156` treats an absent record with no owner as stopped. The existing evidence-copy precedent explicitly preserves evidence before disposing of its source (`metasystem/internal/proofrun/evidence.go:105–110`).

**Concrete failure:** Start a goal run, execute a successful check, and let the application exit normally. Its supervisor removes the record containing the check. A subsequent `app stop --goal G` cannot copy that check into goal evidence: its sole specified source has already disappeared. Normal self-termination is within the stated application model, which includes batch applications. Worktree cleanup can additionally remove an application-owned log before a stopped-run `log` invocation.

**Change:** Define a terminal handoff that retains the minimal run metadata and required log until evidence preservation has completed. Distinguish retained terminal information from live process ownership. Define stopped-run log lookup and perform evidence copying before disposable worktree removal; a general run-history subsystem is unnecessary.

**Test 1 — DIFFERENT/WRONG:** Yes; changes record lifetime, exit/stop coordination and cleanup ordering.

**Test 2 — WORKS/SAFE without it:** Fails **WORKS** and **SAFE**: ordinary exit destroys the only specified check record before the promised preservation operation can consume it.

**Deferred and non-material**

- Exact default ports and deadlines, ref-alias normalization, stopped `--follow` presentation and worktree reclamation schedules can be settled in implementation. Preserve separate per-ref ownership and satisfy the retention requirement above. **Test 1:** potentially changes bounded fixture expectations. **Test 2:** passes without additional architecture.
- Shared application data is deliberately permitted when preparation is absent. D3 still requires a private run state root; sharing must not redirect engine-provided control paths into the standing run. The blanket “data … never touched” wording cannot guarantee what application scripts do to deliberately shared storage. Stronger application sandboxing is deferred. **Test 1:** no additional isolation mechanism follows from the accepted shared default. **Test 2:** passes under that expressly declared default.
- ALC-05 through ALC-07 retain their prior dispositions. Browser integration remains deferred.

**What I verified holds**

- Step 1 and its deferred scope are explicit. The four requested additions belong to the declared slice.
- ALC-01’s identity-before-signal requirement remains explicit, including escalation and refusal when identity cannot be established.
- ALC-02’s revised startup sequence writes ownership before spawning and immediately records the child. The round-2 obligation remains explicit: interrupt between spawn and child publication, then prove the application dead or discoverable and stoppable by identity. That obligation is retained, not discharged by this review.
- ALC-03 still separates liveness from readiness and requires readiness when rejoining a start. ALC-04 still scopes log readiness to the current run and excludes log-pattern disappearance from stop proof.
- D3 distinguishes the named ref from its resolved commit, requires replacement after movement, and specifies one run per ref. Equal commits do not justify collapsing the standing run and separate named runs.
- The premise holds: `reset` composes existing lifecycle operations with project-owned preparation, supporting repeatable experiments. `check` can remain an app-facing verb while the testing contract owns group selection and execution; ALC-09 requires that boundary to be made concrete.

**VERDICT: 3 material findings (fail test 2): ALC-08, ALC-09, ALC-10**

Commit reviewed: `265aa0118cec13d88d5b3eb9555be739ad92de16`.

Limitations: static review only; no tests, builds, process experiments, subagents or edits. Neither private configuration nor files under `artifacts/` were read. The design and prior-round record matched the reviewed commit.

Proposed receipt, unwritten: “App-launch design round 3 at 265aa0118: three material findings—default tree termination, live-run testing integration, and terminal evidence retention; earlier startup fixture obligation retained.”

Codex session ID: 01a0e6f5-36ad-7de3-8730-0bae04d1ad9d
Resume in Codex: codex resume 01a0e6f5-36ad-7de3-8730-0bae04d1ad9d


---

# Round 4 (2026-09-28, fresh session with rounds 1 to 3 quoted via this file), against the design at `f70683e2b`. Verbatim. The loop closes here: zero material findings and the critic's agreement.

**No new material findings.** Round 4 reviewed Step 1—D1, D2, D3, D5 and D6—at commit `f70683e2ba432887f00bb2c4b6eee119024b70c5`, including the complete design, dispositions and three prior rounds. Evidence was read; process behavior below is static reasoning, not an executed reproduction.

No ALC-11 onward is opened.

**Deferred and non-material**

- **The stale-record wording needs no additional mechanism.** [Design lines 225–226](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/app-launch-contract.md:225) still describe removal under the lock. The existing [`readInactive`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/lifecycle/state.go:133) does exactly that. It cannot be copied unchanged: the design expressly restricts removal to stop, reset or replacement start, after evidence preservation, and retains ownership until stopping is proven. Those explicit requirements already determine the necessary adaptation for ended and orphaned records. **Test 1:** no additional implementation change beyond the specified retention rules. **Test 2:** passes WORKS and SAFE when those rules are implemented; this is wording residue, not another lifecycle mechanism.

- **“Group empty” necessarily excludes the supervising process while it is supervising.** The supervisor remains a member of its own group. Consequently, the transition to ended concerns the disappearance of its application members; final stop proof includes the supervisor’s death. Likewise, group KILL includes the supervisor, so it cannot perform subsequent bookkeeping itself. The retained record and the stop caller provide the already-specified observation and evidence handoff. **Test 1:** no new outcome or owner is required. **Test 2:** passes WORKS and SAFE; literal reuse of a group-existence check inside the supervisor would be an implementation error against the stated lifecycle.

- Exact defaults, stronger handling of applications that deliberately escape the owned group, and browser integration remain deferred. ALC-05 through ALC-07 retain their dispositions. **Test 1:** no additional Step 1 architecture follows. **Test 2:** passes under the declared foreground-process and project-owned-command contract.

**What I verified holds**

**ALC-08—the default tree stop:** The group-identity argument holds under Darwin and Linux process-group semantics. A supervisor launched with `setsid` leads a group whose identifier is its own PID. While that same supervisor executes the signal, its PID cannot simultaneously identify a replacement leader. Re-proving itself and signaling its own group therefore differs materially from an external engine signaling a saved group number after its owner has disappeared.

The existing launcher establishes the session at [`launch.go:101`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/lifecycle/launch.go:101). The unsafe saved-number precedent remains correctly excluded: [`SignalGroup`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/launch/process.go:103) performs no ownership proof. Keeping the supervisor after the direct child exits supplies the owner needed to terminate surviving foreground-wrapper descendants.

When the supervisor is gone, limiting signals to an independently recorded, re-proven child and reporting the remaining group preserves safety. It does not falsely promise that every orphan can be terminated.

**ALC-09—the check bridge:** The runner supports the proposed integration without a second identity scheme:

- Both environment modes converge on [`groupTestEnvironment`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/proofrun/test_build.go:2752), where the declared address overlay can be applied.
- The resulting environment digest enters the existing group execution identity; those group identities enter attempt identity inputs. Address changes can therefore propagate through the existing identity machinery.
- `--no-reuse` forces a new attempt and marks selected groups fresh. Freshness-episode comparison also prevents previous group results from being reused.
- Named canary selection is diagnostic, includes the runner’s canary floor and prerequisite closure, and can succeed without satisfying delivery requirements. That fits a human-requested runtime check. The recorded answer remains the actual named group’s outcome.

The design also explicitly refuses a check when the recorded run is not live and ready.

**ALC-10—the terminal handoff:** An ended record now has a defined lifetime independent of process liveness. It retains the exit information, log location and last check; goal evidence is copied before record removal and worktree reclamation. Ended-run log access therefore has a surviving source. The existing inactive-state reader requires adaptation, but that adaptation follows directly from the specified states and removal rules.

**Earlier rounds and premise:** Native identity verification, separate liveness and readiness, readiness on rejoin, and run-scoped log readiness remain explicit. The round-2 interruption fixture remains binding and **undischarged**: an application must be dead or discoverable and stoppable through proven ownership. Merely listing an unrecorded orphan does not satisfy it.

Step 1 and its exclusions are explicit. The start-only contract has defined fallbacks. Project scripts can continue owning application-specific commands while the engine owns lifecycle observation, isolation and evidence; I found no Step 1 requirement that refutes that premise.

**VERDICT: 0 material findings (fail test 2): none.**

I agree that nothing material requiring an additional change to what Step 1 builds remains.

Commit reviewed: `f70683e2ba432887f00bb2c4b6eee119024b70c5`.

Limitations: static review only; no tests, builds, process experiments, subagents or edits. Darwin system-call documentation was read locally; Linux behavior was assessed from the shared process-group semantics, without a Linux execution. No private configuration or files under `artifacts/` were read.

Proposed receipt, unwritten: “App-launch design round 4 at f70683e2b: zero new material findings; round-3 folds supported by static review; existing startup fixture obligation retained.”

Codex session ID: 01a0e6ff-4600-7523-a0c4-6716d9209bc1
Resume in Codex: codex resume 01a0e6ff-4600-7523-a0c4-6716d9209bc1
