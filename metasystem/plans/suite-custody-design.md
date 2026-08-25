# Suite custody: the override dies with its leg, and teardown survives signals (suite-custody)

Working Mode: design

Owner: m2 coordinator brief, v3. Rounds: fd8a (8 material), c9ff (5
material, incl. the critical that forced a real diagnosis). Failsafe
round 3 next; this v3 is grounded in a LIVE two-sided probe, not
hypothesis. Delivery per Wido's discipline: codex builds from the
converged text; fresh critique to AGREE; fast tests only.

## The diagnosed mechanism (probe evidence, 2026-08-25)

The S4 census leg (~761-778) writes a fake process identity onto the
SUITE SHELL'S OWN PID into METASYSTEM_FAKE_PROCESS_IDENTITY_FILE
("fixture-supervisor metasystem-job-owned") so the reaper can prove
the owned-job custodian — and never removes it. Every later classify
of $$ then sees an agent-shaped command that no longer matches the
shell's announcement (the announce records the REAL argv), so the
announcement match fails and classification falls to ancestry:

- attended: ancestry reaches the operator's announced session →
  class DELEGATE → RequireHolder passes DELEGATE through UNGATED →
  the takeover arm proceeds → green BY ACCIDENT (probe: class
  DELEGATE pid 37250, suite rc=0).
- detached: ancestry reaches launchd → UNTRUSTED → ownedElsewhere
  refusal → the KI-43 red (probe: class UNTRUSTED, same identity
  table, same bytes).

KI-43's register entry is updated by this landing: the root cause is
the leaked one-leg identity override plus class-divergent gating,
not generic ancestry dependence; the operational no-nohup guidance
stays until this lands.

## Half 1 — the KI-43 fix (one honest restoration)

After the census leg's assertions complete (immediately after its
last inventory_has/wait_for_census consumer of the override, before
any later gated call), the fixture RESTORES the identity file to {}:
the override was that leg's instrument, and it dies with the leg —
exactly like the leg's other artifacts. Same staged-mv discipline
the writer used (mktemp + mv, because the census reads mid-pass).
No engine change; no arm-supervision change; no new device.

## Half 2 — teardown survives signals (folding c9ff R2-002/003/004/005)

- SIGNAL SEMANTICS (R2-002 — the critic's own repro showed a shared
  trap returning 0): per-signal handlers, not a shared list:
  trap 'on_signal 2' INT; trap 'on_signal 15' TERM; on_signal runs
  the existing cleanup once (re-entry guard), then
  trap - EXIT; exit $((128+$1)). The EXIT trap body stays exactly
  as shipped (status preserved). Cleanup chatter to stderr only.
- DEFERRED DELIVERY IS ACCEPTED SEMANTICS (R3-001): bash delivers a
  trapped signal only when the foreground child returns, so a
  synchronous leg finishes before teardown begins. This design
  GUARANTEES custody at exit, not instant death: acceptance kills
  assert eventual reap and the 128+signal status, never latency.
  Immediate hard-kill of a stuck leg is out of scope here and
  RESIDUE for steward-owned-execution (a runner that owns escalation
  can send the second, unforgiving signal); recorded on the goal at
  landing.
- THE REAL REGISTRIES (R2-003 correction): supervision-fixtures.sh
  owns owned_pids plus fixture_harness_roots (populated by
  make_repo, swept by its cleanup at ~198-233); validate-metasystem.sh
  owns its EXIT trap at ~919-975. track_armed_supervision was a v2
  fabrication and appears nowhere in this design. The change to
  each script is ONLY the per-signal handlers wired to its EXISTING
  cleanup body.
- ORDER HAZARD (R2-004, synchronized per R3-002): cleanup must
  tolerate being entered before late globals are set (guard the
  owned_pids sweep against an unset repo; kill by recorded
  pid+start only). The window test is DETERMINISTIC via a tiny
  test-only hook: when METASYSTEM_SUITE_TEST_PAUSE_AT=post-owned-pids
  is set, the fixture sleeps briefly at exactly that point (between
  the owned_pids append and the repo assignment) — the acceptance
  kill lands inside the advertised pause, so the test proves the
  window instead of hoping to hit it. The hook is inert unset and
  its name says test.
- ACCEPTANCE DIFF SCOPE (R2-005, hardened per R3-003): transient
  announce-then-retire would evade any before/after snapshot, so
  the invariant is proven STATICALLY, not temporally: a check greps
  both scripts for every announce/become_main call site and asserts
  each target root derives from the run's scratch variables ($tmp,
  $repo, bed paths) — a new call site against a non-scratch root
  fails the check by construction. The before/after announcement
  snapshot stays as a smoke, demoted from proof to canary.

## Constraints

- bash-3.2 clean; no process groups, no setsid.
- No engine changes; only supervision-fixtures.sh and
  validate-metasystem.sh.
- Attended stdout verdict lines byte-unchanged on green runs.
- battery.sh untouched (gate-run-freeze, m1).

## Acceptance (fast tests, no battery)

1. Detached (nohup, launcher exits, ppid=1) supervision-fixtures.sh
   runs S4-1..16 GREEN — and the takeover-point classify (probe
   line, removed after verification or kept behind a debug env) is
   class MAIN, not an ungated DELEGATE: the identity is correct,
   not merely tolerated. Attended run stays green with identical
   verdict lines.
2. kill -TERM mid-suite: cleanup runs once, exit status 143, no
   surviving owned pids, no leaked beds in $tmp.
3. kill -TERM inside the 433-461 window (owned_pids populated, repo
   unset): no cleanup crash, same reap guarantees.
4. kill -TERM a validate-metasystem.sh run mid-leg: existing EXIT
   cleanup effects occur, status 143.
5. Announcement-file scoped before/after check on the source
   checkout across a green run: no suite-authored announcements
   outside scratch.
6. bash -n both scripts; the census leg and takeover leg run green
   individually attended.

## Evidence

The two-sided probe (2026-08-25): detached
{"class":"UNTRUSTED","holder":false} vs attended
{"class":"DELEGATE","holder":false,"pid":37250} at the same line
with the same identity table {"$$": "fixture-supervisor
metasystem-job-owned"}; the override writer at
supervision-fixtures.sh:777 with its own comment naming the device;
RequireHolder's DELEGATE pass-through vs UNTRUSTED refusal
(internal/lease/verbs.go:287-291 vs :310-312); c9ff's trap repro
(shared-trap returns 0).

## Round-3 disposition (failsafe)

R3-001 folded as accepted semantics + RESIDUE (deferred delivery
documented; instant-kill escalation recorded on
steward-owned-execution). R3-002 folded (deterministic window via
the inert test-only pause hook). R3-003 folded (static call-site
check replaces the temporal diff as the proof).

LANDED AT THE FAILSAFE: rounds ran 8 → 5 → 3, the diagnosis
survived round 3 intact, and one residue is recorded. Build begins.

## Dispositions

fd8a R1-1..8: folded in v2 (scope cut to two scripts; battery out;
registry teardown), carried here. c9ff R2-001 folded by diagnosis
(the become_main plan is withdrawn; the override restoration
replaces it, aimed at the proven cause). R2-002 folded (per-signal
handlers, critic's repro honored). R2-003 folded (real registries
named; fabricated helper stricken). R2-004 folded (unset-safe
cleanup + the window kill in acceptance). R2-005 folded
(announcement-scoped diff).
