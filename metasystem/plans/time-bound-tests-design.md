# time-bound-tests-run-on-artificial-clocks

- Owner: m1e, claimed 2026-09-12; Wido's word R-104-m1e ("all time-bound tests use artificial clocks ... that is the only way forward").
- Goal and current status: inventory first, then conversion in slices, each landed by human commit with the loaded sweep as its proof.
- In flight right now: the inventory (this page, section 1).
- Decisions made (and who made them): Wido, 2026-09-12: a test that fails under load is a defect of the test, never a reason to widen a floor or retry; the fix is an injected clock and an injected process prober, with a few named end-to-end wiring proofs left.
- Waiting on the human: nothing.
- Dead ends: none yet.
- Next step: classify the 29 sleeping and 36 killing test files (section 1) into the three dispositions; design the seams for missionrunner and proofrun first, where seven of the eight 2026-09-12 defects lived.

## 1. Inventory (2026-09-12, tip d25c2ee0)

Raw universe, test files under internal and cmd: 113 spawn a process (`exec.Command`), 107 read the wall clock (`time.Now`), 29 sleep, 27 kill through `Process.Kill`, 12 through `syscall.Kill`, 2 read `groupAlive`. The wall-clock readers are mostly stamps and are not the class; the sleepers and killers are the prime suspects because each waits for or asserts a process or timer state.

Sleeping test files (29): cmd/metasystem/identity_probes_test.go, cmd/metasystem/proof_run_test.go, internal/adapter/selftest_test.go, internal/boundedexec/boundedexec_test.go, internal/channel/fake/shutdown_test.go, internal/channel/telegram/telegram_test.go, internal/dispatch/proof_attempt_test.go, internal/dispatch/watch_test.go, internal/goal/attention_test.go, internal/goal/turnverdict_idle_test.go, internal/identity/identity_test.go, internal/identity/survivors_test.go, internal/landing/receipt_test.go, internal/lease/claim_test.go, internal/lease/classify_test.go, internal/lease/refusals_test.go, internal/lease/sweep_test.go, internal/lock/lock_test.go, internal/missionrunner/host_process_test.go, internal/missionrunner/launch_progress_test.go, internal/missionrunner/preflight_fixture_test.go, internal/missionrunner/winddown_test.go, internal/proofrun/test_command_test.go, internal/report/stopblock_test.go, internal/run/run_test.go, internal/steward/ledgerattention_test.go, internal/steward/runner_test.go, internal/supervise/arming_test.go, internal/supervise/watcher_test.go.

Killing test files (36): the sleepers above that spawn, plus cmd/metasystem/goal_test.go, hold_test.go, landing_verbs_test.go, internal/dispatch/ownerlock_direct_test.go, internal/gaterun/fence_test.go, gaterun_test.go, guard_test.go, internal/goal/journal_test.go, recover_test.go, internal/identity/enumerate_darwin_test.go, internal/janitor/killproof_test.go, internal/lease/hook_delegate_test.go, internal/missionrunner/stop_test.go, internal/proofrun/supervisor_darwin_test.go, supervisor_test.go, internal/steward/runner_test.go, internal/stoptransition/proof_attempt_inventory_test.go, internal/supervise/owner_test.go, internal/up/up_test.go.

Seams that already exist: 66 production files carry an injectable clock or prober (`Now func() time.Time`, `Prober identity.Prober`, `Signal func(int, syscall.Signal) error`); the watchdog, the stop ladder, the arming owner and the reconciler landed today are driven that way. The shell fixture beds: 33 scripts wait on `sleep` or a deadline loop; those are wiring proofs by nature and get bounds an order of magnitude above their expected time, not fakes.

## 2. Dispositions

Each file in section 1 gets one of three, recorded here as the slices land:

- converted: the logic under test now takes a clock and a prober (or a signal function) and the test drives them; no wall time passes.
- wiring proof: a real process is required to prove the seam itself (a real fork, a real signal, a real pipe); the test is named here, its bound is at least ten times the expected duration, and it waits for the fact it asserts, never for an instant.
- redundant: the behaviour is proven by a converted test elsewhere; the file's real-process case is deleted.

## 3. Order

missionrunner and proofrun first (seven of the eight defects of 2026-09-12), then supervise, identity, landing, lease, janitor, census; then the cmd tests; then the beds' bounds. Each slice: convert, run the package three times under the race detector beside eight burners on the eighteen-core box, land by human commit with one independent critic (R-98-m1e).

## 4. Critique record

None yet.
