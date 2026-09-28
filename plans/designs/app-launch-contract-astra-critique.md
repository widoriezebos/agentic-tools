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
