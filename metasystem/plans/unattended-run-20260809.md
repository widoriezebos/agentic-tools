# Unattended run, 2026-08-09 → 10 hours

- Goal and current status: the human's mission, given 2026-08-09
  ~20:50Z before a 10-hour absence, verbatim intent: finish the
  supervision-lifecycle stream (critique chain to its close rule),
  implement the Go application fully unit-tested to the recorded
  engineering standard, get everything green and pushed, then run
  the two benchmark cohorts — bm-2 with host Opus 5 and delegates
  `gpt-5.6-sol`, and bm-2 with host Opus 5 and delegates Devin
  `swe-1.7`, 2 repetitions each. "Complete this mission. Do not
  stop until fully done."
- Next step: none
- In flight right now: nothing in this checkout's job records —
  cohort A repetition 1 (bm-2s-20260810t010831z-53362) runs
  through grading as a tracked background task; the orchestrator
  session holds the waiter.
- Waiting on the human: nothing.

## Rulings collected before departure (AskUserQuestion, 2026-08-09)

1. CLOSE RULE for the critique chain: fixtures-as-arbiter — the
   first round returning only fold-consistency residue (no new
   interleaving) closes the chain; HARD CAP three more rounds
   (11-13), then close regardless, open findings carried as named
   implementation risks. Recorded in the design's critique record.
2. CODEX'S CAPS WORK: validate via the full suite; commit if green
   (as Codex's work, separate commit from the supervision
   documents); if red, set aside on a branch with the failures
   reported for Codex.
3. BENCHMARKS: two bm-2 cohorts, host `claude-opus-5` in both;
   delegates `gpt-5.6-sol` (cohort A) and devin `swe-1.7`
   (cohort B); 2 repetitions each; same shape and fences as the
   completed bm-2 cohort; contracts sealed and signed under the
   standing bm-2 pre-authorization.

Standing rules that govern the whole run: Fable for claude-side
work; critic is codex `gpt-5.6-sol`, named explicitly per dispatch;
no untracked background processes; gates read from chain exits
(IL-17: suite && kit gate && push in one chain); the operational
workaround "kill owners before components" stands until D-4 lands;
do not touch `plans/supervision-lifecycle.md`'s settled matter
(D-6 numbers, ACCEPTED consequences, dead ends).

## Checklist (update as states change)

1. DONE — round 9 adjudicated and folded (dispositions round-9).
2. DONE — round 10 adjudicated and folded (dispositions round-10);
   Go ruling + engineering standard + unit-test standard recorded.
3. DONE — the critique chain is CLOSED at the cap (rounds 11:
   6, 12: 4, 13: 4 — all adjudicated, verified, and folded
   same-day; dispositions round-11..13). Closed honestly NOT as
   converged: round 13's four folds carry no critique pass — the
   named risk. The Proof list is the arbiter; design defects
   exposed by implementation get one defect-driven sol round
   each. The slc-r4 worktree is KEPT for those rounds.
4. DONE — caps loop closed GREEN: suite run 10 passed end to end
   after ten runs burned down seven classes (census-gate order,
   dormant cap, fence-state stamps x7 missions, two message
   contracts, the ask inversion, the AUTH-R2-005 race, the
   KI-31-shaped guard fixture). All Codex rulings honored (three
   rounds, two ratifications). COMMITTED as the ruled split:
   43265c1 (supervision chain documents), 31e09eb (Codex's caps
   authority core + integrations), 9ffac42 (Go foundation).
   Gates+push chain running for the backlog of commits.
5. IN PROGRESS — Go implementation. DONE: module
   (github.com/widoriezebos/agentic-tools/metasystem, go 1.26.5);
   internal/registry — REG-1 framing with two-part repair and
   run-tolerance, REG-2 records/validator, REG-3 reduction
   (generation pairing, watermark, custody binding) and
   compaction (skeleton retention incl. terminal), 91.8%
   coverage; internal/lock — REG-4 rename-born acquisition,
   three-way death-only takeover fenced by a kernel flock with
   in-fence re-verification (a two-winners takeover race was
   caught by the unit test and fixed), race-detector green x5.
   DONE also: internal/identity (darwin kernel prober —
   microsecond start times via kern.proc.pid, argv via
   kern.procargs2, three-way AliveRef; x/sys/unix dependency;
   proven against live processes) and internal/supervise's
   DECISION CORE (decide.go: D-1 Classify table, D-2 Breaker on
   one clock with capped backoff, CeilingVerdict, Establishment
   bound, RetireWatermark — 96.6% coverage, every related Proof
   row is a table test). NEXT: the owner loop around that core
   (observation gathering, launch_set write-ahead, teardown by
   held identity, shutdown-intent latch), then gate+janitor
   (D-4), checkout-wide shutdown, cmd/metasystem verbs, wrapper
   wiring, suite gates (gofmt/vet/build/test + 90% floor).
   Remaining per the design's Implementation order
   (registry → D-1 → D-2 → D-4+D-3 → D-5), in
   `metasystem/` as one module, one multi-verb binary, packages
   named from the glossary, 90% domain coverage gate, race
   detector, gofmt/vet/build/unit-tests wired into the suite AHEAD
   of fixtures; wrappers keep today's verbs; bin/ is gitignored
   and built by the suite.
6. TODO — extend the shell fixtures to the Proof list; suite green.
7. TODO — cohort driver: teardown ledger + entry recovery +
   completion continuation (D-3; bash — driver logic, not
   process-critical).
8. DONE — GATES-AND-PUSH-GREEN: suite && kit gate && push, one
   chain; origin/main at 18eab0b, ZERO unpushed commits (26
   banked, including the prior session's 21).
8a. DONE — KI-31 CLOSED AT THE ROOT (plans/provisioning-identity.md,
   one sol round, six findings folded; commit 0fe6a1c): the
   provisioner is the target's first main, wrapper-carried commits,
   departing-main release (KI-33 second occurrence recorded), kit
   gate green from a live agent session for the first time. The
   seal step is explicitly the simulated-human act; preflight's pin
   asserted against contract bytes.
9. IN PROGRESS — cohort A = bm-2s-20260810t010831z-53362 (host
   Opus 5, delegates gpt-5.6-sol, 2 reps). Repetition 1: target
   provisioned through the NEW identity-carried path, sealed
   (9810af0b...), signed per the pre-auth, pushed, lease released
   (departing-main), RESUMED ~01:15Z — running through grading
   with a tracked waiter. On completion: provision rep 2, same
   flow, then cohort B (bm-2, Devin).
10. TODO — benchmark cohort B (host Opus 5, delegates devin
    swe-1.7, 2 reps): same.
11. TODO — final report for the human: chain outcome, Go
    implementation state, suite/gate evidence, both scorecards,
    costs, and anything parked.

## Dead ends and cautions for this run

- Job ids are single-use; briefs live OUTSIDE
  `artifacts/agents/<job-id>/` (the r10 id was burned by that).
- The critique chain's job-id offset: critique round N runs as job
  `supervision-lifecycle-r(N+1)` from round 10 onward.
- This session (pid 90299) holds BOTH checkouts' leases: main and
  the slc-r4 worktree (epoch 3, took over the dead prior holder).
- Devin metering is ACU, never converted (glossary); Devin runs
  uncontained per the recorded acceptance.
- If implementation exposes a design defect: fold it and send the
  design back to sol for ONE more round (the design's own rule) —
  the close rule's cap does not bar defect-driven rounds, only
  convergence-seeking ones.
