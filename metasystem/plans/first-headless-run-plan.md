# First headless run — the one-page plan (m0, 2026-09-03)

Goal `first-headless-run` (Wido, R-66-m1): run the metasystem for real on m0
with no Claude session on top. This page is step 1 of the recipe: the
runner's real entry point, what it needs to start on a host, the goal it
runs, and the exact sequence. Everything below was read from the code on
main at `aa9ca65c`, not inferred.

## 1. The runner's real entry point

`metasystem mission start --root <project root> --mission <id>` (cmd/
metasystem/missionrunner_verbs.go:246). It arms supervision under its own
identity, runs the contract preflight, then spawns the detached loop
`mission run-loop` (Setsid; log at
`artifacts/agents/missions/runners/<id>.log`, record `<id>.json`,
heartbeat `<id>.heartbeat`). `mission resume` restarts a parked one,
`mission status --mission <id>` prints `mission=<id> status=<s> reason=<r>`,
`mission answer --ask <id> --answer <text>` answers a park.

The runner is a MEASURED-OPTIMIZATION loop: each cycle assembles a host
turn (`scripts/agents/hosts/claude.sh` → `claude -p --output-format json
--model <host.model> --json-schema orchestrator.schema.json`), the host
turn dispatches delegates (mission-scoped via `METASYSTEM_MISSION_TURN`),
and the runner MEASURES the candidate branch tip against the sealed gate
(`metric=<name>=<decimal>` lines), keeps a stop-loss ledger, and parks
with an ask when it needs a human. Its "serving goal" is whatever goal
THIS MACHINE holds a claim on (`goal.Store.ServingProjection`, by machine).

## 2. What it needs on the host (all verified on m0)

- A sealed, SIGNED contract at `plans/mission-<id>.contract.md`, with its
  bytes present verbatim on origin's default branch (preflight
  `verifyOrigin`), and `refs/remotes/origin/HEAD` declared (it was not on
  m0 — `git remote set-head origin -a` fixes it).
- Gate instruments committed AND tagged: `gate.ref` must resolve to a
  commit; `gate.paths`/`truth.paths` globs must match files at that tag
  (integrity-hashed at seal; changing them after sealing fails preflight).
  Sealing RUNS the gate once against the candidate branch tip in a temp
  worktree (`bash --noprofile --norc -c <gate.command>` in the project
  root) and records the baseline.
- `mission ledger-init --file <missionDir>/ledger.md --cycle-budget N
  --no-gain-budget M`, then `mission state-init --state
  <root>/artifacts/agents/missions/<id>/state.json --contract
  <plans path> --ledger <ledger> --baseline <tree>` where the baseline is
  the LIVE filtered projection tree id (Snapshot("HEAD") minus the
  mission ledger — computed through the engine, not guessed).
- The Claude host adapter present and authed: `claude` 2.1.259 at
  `~/.local/bin/claude`, logged in (the m0 account); the adapter assembles
  its argv through `metasystem adapter claude-command`.
- Supervision: the runner arms it AS ITSELF (session
  `mission-runner-<id>-<pid>`, tag `mission-runner.sh`). Arming REFUSES
  while another owner is recorded ("supervision state names another
  owner"), so m0's session owner must be shut down first
  (`scripts/agents/arm-supervision.sh --repo . --shutdown` stops the
  owner only; the steward runner and this Claude process are untouched).
- The mission lease `artifacts/agents/missions/<id>/lease.d` must NOT
  pre-exist (preflight takes and releases it as a probe).
- Envelope: `envelope.dispatch-allow=codex:gpt-5.6-sol,claude:claude-fable-5-1`
  (the roster pairs; only pre-authorizable categories per
  docs/project-rules.md), `exposure=EUR:<n>`.
- Codex handshake budget is 30 s (scripts/agents/adapters/codex.sh,
  `sessionEstablishedTimeoutSec: 30`).

## 3. The goal to run

The recipe named `adoption-inventory-from-install-set` or
`merge-stage-critic-close`; NEITHER has a landed design or build brief
(both next-steps still say "design round"). Nine queued goals do. The
pick: **`job-record-birth-token`** — small mechanical item, 4h box,
design landed (`plans/job-record-birth-token-design.md`), design and fold
briefs present, unclaimed, next step is Sol critique → build in
`internal/dispatch` record writers → code critique → land. Alternate:
`host-health-role` (4h, design paragraph still to write).

It is expressible as a measurable gate: the design names six Go fixtures
(`TestBirthTokenMintGrammar`, `TestRecordCreateMintsBirthTokenIgnoringCallerValue`,
`TestBirthTokenSameSecondReuseIsDistinct`,
`TestRecordSetupCarriesBirthTokenAndRefusesADifferentOne`,
`TestPreContractRecordNeverGainsBirthToken`,
`TestRecordCASRefusesBirthTokenPatch`). None exist today, so the baseline
is 0 and the mission reaches its target when the build lands them green.

Gate (validated on m0: 0 at baseline, 27 on a positive control):
```
go test ./internal/dispatch/ -run 'BirthToken' -json -count=1 \
  | grep '"Test":' | grep -c '"Action":"pass"'   →  metric=birth-token-fixtures=<n>
```
`gate.threshold.birth-token-fixtures=>=6`, `gate.direction=max`,
`gate.noise-floor.birth-token-fixtures=0`. Guard (regression tripwire):
`go build ./... && go vet ./internal/dispatch/` → `metric=build=1`,
floor 1, noise 0. The shell fixtures the design also names
(`record-protocol-fixtures.sh`, `return-schema-fixtures.sh`) are the
BUILD's obligations checked by the code review, not mission guards (a
control-plane write under runner ancestry may classify UNTRUSTED).

Gate and truth files live under `plans/first-headless-run/` (path class
RECORD, landable by seat carriage; `scripts/` is BEHAVIOR = the
never-direct-fix floor, so a gate script there would need a chain).

Under human-approval-for-execution, `job-record-birth-token` must be
APPROVED by Wido before m0 can claim it (its 4h/6/240m tuple stands).

## 4. The one-claim consequence

The runner serves the goal THIS MACHINE holds. m0 holds
`first-headless-run` (the run's own record goal). To make the runner
work `job-record-birth-token`, m0 releases `first-headless-run` and claims
`job-record-birth-token` before starting the runner, then re-claims
`first-headless-run` afterwards to record the run. This is a finding for
the record: one claim per machine forces the meta-goal to step aside
while the runner serves the target.

## 5. Contract skeleton (`plans/mission-birth-token.contract.md`)

```
# Intent
Land goal job-record-birth-token: every job record incarnation carries a
mandatory, immutable, machine-minted birth token, proven by the six
named fixtures.
# Non-goals
No changes outside internal/dispatch and the named fixture scripts.
# Initial streams
Keep one stream active.
```mission
gate.command=bash plans/first-headless-run/gate.sh
gate.ref=first-headless-run-instruments
gate.paths=plans/first-headless-run/*.sh
truth.paths=plans/first-headless-run/fixtures.txt
truth.certification=candidate
gate.direction=max
gate.threshold.birth-token-fixtures=>=6
gate.noise-floor.birth-token-fixtures=0
guard.build.command=bash plans/first-headless-run/guard.sh
guard.build.floor=1
guard.build.noise=0
guard.cadence=1
ledger.cycle-budget=6
ledger.no-gain-budget=3
fence.wall-clock-hours=8
fence.cycles=6
fence.jobs=8
fence.concurrency=1
fence.job-cap-min=180
host.runtime=claude
host.model=claude-fable-5-1
host.turn-cap-min=60
stream.primary=Land job-record-birth-token with its six fixtures green.
envelope.dispatch-allow=codex:gpt-5.6-sol,claude:claude-fable-5-1
exposure=EUR:40
```
Approval: name=Wido; date=2026-09-03; contract-sha256=<from mission contract-hash>
```

## 6. The sequence

1. Wido approves `job-record-birth-token` (relayed form is fine).
2. Commit gate.sh, guard.sh, fixtures.txt under `plans/first-headless-run/`;
   tag `first-headless-run-instruments`; land via seat carriage.
3. Author the contract; `mission contract-validate`; `contract-seal`
   (runs the gate, baseline 0); `contract-hash`; Wido signs (Approval
   line); land the signed contract on main; push.
4. `git remote set-head origin -a`; `ledger-init`; compute the live
   baseline through the engine; `state-init`.
5. Release `first-headless-run`, claim `job-record-birth-token`.
6. Shut down m0's session supervision owner; stop this Claude session.
7. From a plain shell: `metasystem mission start --root <metasystem> --mission birth-token`.
8. Watch from outside only: `mission status`, the runner log, `asks/`,
   `metasystem health`, the ledger. Each stop is a finding: fix forward,
   `mission resume`, until the goal lands with a chain landing signed by
   the runner.
9. Record the run in `records/misc/first-headless-run.md` (what needed a
   person, what it did alone); open the follow-ups (a `mission start`
   wrapper that does steps 2–4 for a goal; the fleet-join bootstrap).

## 7. Known first-run refusals already found (before starting)

- `refs/remotes/origin/HEAD` not declared → preflight refuses (fixed by
  set-head).
- Supervision owner recorded for m0's session → runner cannot arm until
  it is shut down.
- The serving goal is by machine claim → step 5 above.
- The turn-verdict runtime state (`artifacts/agents/turn-verdict-state.json`)
  trips conformance on every delegate round (goal
  conformance-runtime-state-litter) — the runner's turns will hit it;
  expect to fix forward there first.
