# Material findings

## FC-1 — The observed dead-launcher/live-test shape remains outside the design

- **Page section:** 3.1, 3.4, and `Unchecked` (lines 306–308).
- **Evidence:** The survivor predicate in 3.4 requires the owner in `METASYSTEM_FIXTURE_OWNER` to be dead. In the third observed shape, the owner test binary is still alive after its bash launcher dies, so neither that predicate nor a post-run action by the now-dead launcher can select it. The page expressly says this shape was not investigated and that its proposed launcher census does not cover the binary. The preserved specimen records that this test continued to its own 45-minute timeout.
- **Failure caused:** A launcher can die while its test binary and fixture descendants continue, leaving one of the three observed escape shapes unchanged. The page therefore does not prove “a fixture's children die with it” at the run boundary.
- **Smallest change that fixes it:** Define a run-owner identity above the test binary and a parent-loss mechanism that does not require the dead bash launcher or the live test to cooperate. Make the test binary and its fixture descendants subject to that owner. Add a seconds-long witness that starts a launcher, starts a test plus a tagged child through it, kills only the launcher, and observes an external custodian kill both test and child. Route that mechanism and witness into the units and DONE proof.

## FC-2 — The named abort teardown owners can die in the aborts they are meant to survive

- **Page section:** 3.3–3.4 and witnesses 2, 3, and 5.
- **Evidence:** `t.Cleanup` and the bed's EXIT trap require their own process to run. The proposed fallback for non-leashed production grandchildren is the proof-run launcher, the bed runner, and health. `internal/proofrun/launcher.go` is an ordinary process that can itself be killed; its completion path also reaches `copies.Wait()` after waiting for the suite/watchdog, while the existing comment at lines 278–285 notes that detached fixture children may retain suite descriptors. Health is later detection, not teardown. A FIFO EOF only helps a process that reads it: it does not reach the Go/`Setsid` grandchildren behind `TestPendingWaitFromChildShell`, and a SIGSTOPped runner cannot consume EOF. Witness 2 kills a helper whose direct shell is reading the leash, and witness 3A again makes the grandchild leashed; neither exercises the non-cooperating cases assigned to the external census.
- **Failure caused:** Killing the test binary, cancelling the Codex job, killing the delegate at its session limit, or killing the bed/launcher can also remove the only process expected to run the census. Detached Go grandchildren and a stopped runner can remain. Thus the abort escape route is still open even though the direct shell witness passes.
- **Smallest change that fixes it:** Give cleanup to one exact, kill-capable custodian outside the suite/bed/delegate kill domains (the existing sibling watchdog is a candidate only if its own survival and launcher-death behavior are specified). It must reap by tag before any pipe drain that a survivor can hold, and it must react to exact owner/launcher death. Add bounded witnesses for: test binary killed by PID, launcher killed while test lives, cancellation of the enclosing process group/job, delegate/session-owner death, a non-leashed `Setsid` grandchild, and a SIGSTOPped child. Each witness must kill the process that currently owns cooperative cleanup and still observe no survivor.

## FC-3 — The executable-path census fallback can classify and kill a legitimate active run

- **Page section:** 3.4 and risk lines 226–227.
- **Evidence:** The mechanism selects “any process whose exe path lies under a `metasystem-build-cache/go-tmp` directory.” That is also the normal path of a currently running test helper. The risk section says it will scope to caches “no live launcher owns,” but no cache-to-launcher record, exact identity join, or ownership algorithm appears in the mechanism or units. `--reap` is wired into every proof-run launcher, so concurrent runs make this more than a diagnostic false positive.
- **Failure caused:** One suite finishing can report or SIGKILL a legitimate long-running helper belonging to another live suite. Conversely, making an ad hoc age exception to avoid that would miss the eight observed orphans. The census therefore does not yet distinguish fixture survivors from legitimate long-running processes as required.
- **Smallest change that fixes it:** Persist an exact run/cache ownership record before launch and require a dead or identity-mismatched owner before the executable-path fallback is actionable. Keep an unowned path match diagnostic-only if ownership is indeterminate. Add one fixture-table witness containing an orphan in a dead owner's cache and an otherwise identical process in a live owner's cache; `--reap` must select only the orphan.

## FC-4 — Both the owner tag and recorded-PID kill lose exact process identity

- **Page section:** 3.1–3.2.
- **Evidence:** The owner is specified as `<pid>:<start ticks>:<test name>`, but `internal/identity/identity_darwin.go` reports `StartTicks == 0`; the repository's native exact Darwin identity is `StartedAtUnixMicro` (`identity.Ref`), while Linux uses start ticks plus boot ID. Separately, `Record(pidFile)` is described as retaining PIDs, and cleanup sends SIGKILL to those PIDs without an identity comparison immediately before the signal. A child that exits and whose PID is reused can therefore turn cleanup into a kill of an unrelated process.
- **Failure caused:** On the target macOS machine, the advertised owner key is not the exact identity claimed by the page. On every platform, delayed cleanup can signal a recycled PID. This violates reaping by the fixture child's own recorded identity and can kill a legitimate process.
- **Smallest change that fixes it:** Encode the full platform-native `identity.Ref` in the owner value (Darwin microsecond token; Linux ticks plus boot ID). When a PID file is registered, probe and store the child's exact ref, then compare the live ref immediately before SIGKILL; mismatch/absence means the recorded child is gone and must not be signalled. Add Darwin/Linux encoding fixtures and a recycled-PID witness that proves no signal is sent to the replacement process.

## FC-5 — Unit 5 can pass without exercising two converted fixture behaviors

- **Page section:** 3.6, section 6, and unit 5.
- **Evidence:** Unit 5 changes `suite-progress-fixtures.sh`, `fixture-bed-scenarios-fixtures.sh`, and `hosts/fake.sh`, but its only named witness is witness 5, the fixture-bed `hang` scenario. Witness 6 covers only the attention and wait-verb Go tests. No witness in section 6 runs the converted stopped-suite/detached-member scenario or the fake host's hold/ignore-TERM path. The proposed fake-host edit also says the leash read is “preceded” before the loop without pinning it after `host-ready`, so an incorrect placement can prevent readiness while the unit's named witness remains green.
- **Failure caused:** The detached-bed/stopped-runner scenario can be weakened, or the held host can stop becoming ready/stop ignoring TERM, without any required unit witness failing. This leaves two R-115-m1e bindings unproved.
- **Smallest change that fixes it:** Add focused, seconds-long unit-5 witnesses for both paths. The suite witness must show the stopped runner and detached TERM-ignoring member exist while the owner is live, must still fail if the watchdog's stopped-runner behavior regresses, and must leave neither process after owner loss. The fake-host witness must observe `host-ready`, prove TERM is ignored while the leash owner lives, close/kill the owner, and then observe the host gone. State that the leash read occurs after readiness is published.

# Non-material notes

- The page does cover all three originally stated local cleanup mistakes for the installed-hanging-git fixture on a cooperative return: it replaces the test defer with `t.Cleanup`, records direct child PIDs as kill targets, checks kill errors, and proves group leadership before retaining a negative-PID assertion. FC-2 is about the stronger abort cases, where none of those cooperative actions can run.
- The named TERM-ignoring family is enumerated: the attention wrappers, `suite-progress-fixtures.sh` `__detached`, the fixture-bed `hang` children, and `hosts/fake.sh`. The remaining defect is proof/abort ownership, not omission from the conversion list.
- The command spelling `metasystem census fixture-survivors` is inconsistent with the existing `metasystem proc census|alive|find-ancestor` namespace in `cmd/metasystem/census.go`. Resolve the spelling before implementation; this does not by itself change the safety mechanism.
- Unit dependency order is coherent. The 290-line estimate for unit 2 and 260-line estimate for unit 3 leave little credible margin because each includes cross-platform implementation, destructive-path tests, and several subprocess witnesses. Measure the computed diff per unit and split helper implementation from abort witnesses, or census core from launcher/health wiring, before either exceeds the 300-changed-line cap.
- “The next aborted deep proof” is useful field confirmation but cannot be the deterministic proof of DONE. Once FC-1 and FC-2 are fixed, their bounded witnesses should be the gate and the later field observation should remain supplemental.

# Unchecked items

- No proposed witness was run because the page is a pre-implementation design.
- I did not inspect files outside the goal, preserved evidence, review page, repository instructions/skill, and source locations cited by the page. Therefore I did not independently census every process-spawning fixture in the repository beyond the named TERM-ignoring family.
- The vanished `goal.test` specimen could not be re-inspected; FC-1 relies on the named preserved observation and the page's explicit admission that the shape is not covered.
- Exact changed-line totals cannot be checked before implementation; only the stated unit scopes and estimates were reviewed.

# Tool calls used

16 tool calls total: 15 shell/read calls (including two failed path-resolution attempts that made no changes) and one `apply_patch` call to create this file. No state-changing git command was used.
