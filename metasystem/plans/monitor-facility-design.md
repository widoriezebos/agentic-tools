# The monitor facility: tracked long-running work with terminal-state watching

Working Mode: design

Owner: main session (delegate), goal monitor-facility (detail notes
at plans/backlog-notes.md item 15). Status: r3 — folds the ten r2
findings (critiques at plans/monitor-facility-critique-r{1,2}.md;
trajectory 12/10). Human rulings fixed as input: the exchangeability
doctrine, the verbatim monitor-pattern intent, and backlog item 1's
LITERAL waiter contract, which r3 stops substituting for.

## The problem, in the human's words and two incidents

A detached suite run is invisible to the metasystem: the census
flags it UNTRACKED, no record says what it is or what should happen
when it ends, and only one runtime's harness knows to care. The
harness's 10-minute cap forced detached launches; a monitor caught
a 205-second failure a fallback timer would have slept 15 minutes
on. The pattern works — it must become metasystem behavior.

## One mechanism, two record kinds, the waiter honored (r2 finding 1)

Backlog item 1 folds in AS WRITTEN, not replaced:

- `dispatch.sh dispatch` WITHOUT `--wait` prints the exact waiter
  command for the job it created: `scripts/agents/dispatch.sh watch
  --job <id>` — the agent never invents a polling loop.
- `dispatch.sh watch --job <id>` is a NEW blocking verb (the poll
  decision in Go): it blocks until the job record is terminal and
  exits with the terminal status, which is what every runtime's
  background facility turns into a wake-up.
- The waiter REGISTERS ITSELF: `artifacts/agents/waiters/<job>.json`
  {pid, pidStartedAt, session, mainId}, identity-verified at write,
  removed on exit; a dead waiter's record is provably dead by the
  same three-way rule as everything else.
- The turn-end rule is then computable and per-session: an in-flight
  job whose record's mainId equals the CALLER's mainId with no LIVE
  identity-verified waiter record, or a running run this mainId
  registered, blocks the turn end ONCE. `report turn-verdict` gains
  `--main-id` (the hook already computes main_id); TurnVerdict does
  the waiter join internally.

Run records exist only for NON-JOB work (suites, cohorts, detached
processes); jobs keep their one lifecycle. The goal system carries
WHAT next; this carries WHEN; the verdict is where both speak.
Flight-recorder events narrate; they are never the wake authority
(the recorder contract permits lost finals) — the wake is the
blocking waiter deciding from the record and kernel state.

## The run record: artifacts/agents/runs/<run-id>.json

Engine-written, per-record flock + compare-and-swap over
(status, generation) + legal transitions. Schema (v1):

    {"schemaVersion": 1,
     "runId": "<kebab, unique, ≤64>",           // collision refuses
     "kind": "suite" | "cohort" | "custom",
     "display": "<from --display only, ≤200>",   // never argv join:
                                                  // argv can carry secrets
     "custody": "wrapped" | "adopted-verified" | "adopted-unverified",
     "generation": <int, starts 1>,               // adoption fences on it
     "pid": <int|null>, "pidStartedAt": <int|null>,
     "pgid": <int|null>,                          // null while launching
     "launchNonce": "<32 hex>",                   // in the wrapper's ARGV:
                                                  // the third identity factor
     "log": "<path ≤512, exists-or-creatable at bind>",
     "startedAt": "<ISO>",
     "mainId": "<str|null>", "ownerLineage": "<str|null>",
     "claimEpoch": <int|null>,                    // null for HUMAN callers:
                                                  // human runs have no epoch
                                                  // and are swept only by
                                                  // identity-death
     "sessionId": "<normalized>",
     "goalId": "<informational, never authority>",
     "staleAfterMin": <int 1..1440>,
     "hungSince": "<ISO|null>",                   // a flag on running,
                                                  // cleared by log activity
     "windDownMin": <int, default 10>,
     "evidence": {"mode": "exit-sidecar" | "pattern" | "none",
                  "verdictPattern": "<RE2 ≤256, adopted records only>"},
     "expect": {"green": "≤240", "red": "≤240",
                "hung": "≤240", "unknown": "≤240"},
     "status": "launching" | "running" | "draining"
              | "green" | "red" | "ended-unknown" | "launch-failed",
     "acked": false, "error": "<str|null>",
     "exitCode": <int|null>, "endedAt": "<ISO|null>"}

Legal transitions (CAS on status AND generation; refusals loud):
- launching → running (the wrapper binds identity via launchNonce)
- launching → launch-failed (the launcher's error path OR the
  bounded launch fence, default 2 min — an error note, NEVER
  deletion: the dispatch precedent keeps failed reservations, and
  deletion races the wrapper's bind; prune ages failures out)
- running → draining (evidence recorded; the group may survive)
- draining → green | red | ended-unknown (group provably empty or
  windDownMin expired; survivors then surface as UNTRACKED — the
  honest answer, they are)
- running → ended-unknown (dead + no evidence; VANISHED IS DROPPED
  — r2 caught the contradiction; one dead-no-evidence verdict)
- adopt: identity fields replaced on a RUNNING record, generation
  incremented, hungSince cleared; a stale-generation conclusion
  fails its CAS by construction. Sidecars are generation-scoped
  (below), so an old sidecar can never conclude a new generation.

## Launch topology (r2 finding 7 closed)

`run launch` writes the PENDING record (nullable identity, nonce
minted) BEFORE any process exists; spawns the wrapper via setsid
(`metasystem run wrap --nonce <hex> ...` — the nonce is visible in
argv, giving census and sweep their third identity factor); the
wrapper's first act is the CAS launching→running binding pid/start/
pgid; its last act is the atomic exit sidecar; the record then
drains. Fast commands cannot escape (the record precedes the
process); a killed launcher leaves launching → the fence concludes
launch-failed with the note.

## Exit evidence (r2 findings 4+5 closed)

The wrapper writes `runs/<run-id>.g<generation>.exit.json`
atomically: {"runId","generation","nonce","exitCode","endedAt"}.
The sidecar is believed only when nonce AND generation match the
record. exitCode 0 → green; nonzero (incl. 128+n signals) → red.
The `.g<n>.exit.json` suffix is excluded from the record glob by
filename grammar, so a sidecar can never be read as a malformed
record. `verdictPattern` exists only for adopted records: RE2 ≤256
against a bounded 64 KiB tail; match → green; no-match →
ENDED-UNKNOWN, never a red guess; an unreadable or missing log at
conclusion → ended-unknown AND the path surfaces via Unreadable.
Unknown identity concludes NOTHING — it surfaces and stays.
Prune removes a record's sidecars with it; id reuse after prune is
legal (nonce + generation disambiguate).

## The watcher and its attestation (r2 finding 2 closed)

The run pass lives in the Go WatcherPass armed supervision actually
executes (supervise component --component watcher), beside the
census. Each pass: three-way identity per launching/running/
draining record; the launch fence; evidence rules on death; the
drain check; hungSince set/cleared by log mtime vs staleAfterMin;
ONE flight-recorder event per transition. After every SUCCESSFUL
pass the watcher atomically writes
`artifacts/agents/supervision/runs-pass.json` {completedAt, ok,
scanned} — the ATTESTATION. The verdict's unwatched rule requires
this attestation fresh; a fresh component heartbeat alone never
suffices (the heartbeat precedes work and errors are logged-and-
continue — attestation is the fact that the run reader RAN).

## Census custody (r2 finding 3 closed)

A process belongs to a run when its pgid equals the record's pgid
AND the leader's identity verifies (pid + start + the argv nonce;
ArgvKnown=false is absence of evidence — the process surfaces
rather than being claimed or condemned). Census owns launching,
running, AND draining records; terminal records own nothing —
draining is precisely the bounded window that keeps surviving
descendants accounted for until they are provably gone or the
wind-down expires.

## Verbs and authority (r2 finding 6 closed)

All mutating run verbs execute under `lease run-held` — the
existing operation-spanning fence — not just a point-in-time check:
- `run launch` / `run register` / `run adopt` / `run ack` /
  `run prune` — holder-only. HUMAN callers pass with nullable
  coordinates (the authority matrix already admits HUMAN; the
  record stores null mainId/epoch and is exempt from epoch sweep).
- `run register` (already-running work): group-membership or
  ancestry proof upgrades custody to adopted-verified; without
  proof the record is adopted-unverified — census owns it, the
  label is honest, the act is evented.
- `run conclude` — record-writer (the watcher concludes; holders
  may). Evidence rules only.
- `run status` / `run list` — open reads; {"schemaVersion":1,
  "runs":[...]} sorted by startedAt.
- `run prune` — acked terminal records older than 14 days, drops
  reported (the events are the audit trail).

Takeover (stale claimEpoch, running/draining): WITH the argv-nonce
group proof — SIGTERM the group, bounded drain, conclude
ended-unknown unacked (the successor sees it at the next turn end).
WITHOUT proof — including every adopted-unverified record — the
sweep REFUSES loudly and only surfaces; it never signals and never
silently terminalizes. Live untracked work is a surfaced fact, not
a casualty.

## Scanner and verdict (r2 finding 8 closed)

ScanResult gains `Runs []RunFact{Id, Status, ProbeState(alive|dead|
unknown), OwnedByCaller, Watched, Acked, HungSince, Continuation
≤240}`. Composition, stated against the ladder AS SHIPPED (Busy
suppresses the goal/open/waiting/unreadable ladder — all of it):

- Run WARNINGS live OUTSIDE the ladder, always prepended to
  Display: terminal-unacknowledged red/ended-unknown, currently
  hung, unknown-identity, and run-reader unreadables — continuation
  verbatim ("suite run <id> went red; the run record says:
  <expect.red>"). Busy can never hide them.
- GREEN terminal surfaces at most once per session (greenDigests
  slot, ≤16 FIFO — one additive array in the item-14 state schema),
  informational, never blocking; acknowledgment stays `run ack`.
- The UNWATCHED block: the caller's in-flight jobs without live
  waiters, or its running runs, OR a stale/absent runs-pass
  attestation while such work exists → block ONCE, keyed on the
  sha256 of the sorted unwatched-id set (blockedUnwatchedDigests,
  ≤16 FIFO, additive).
- REQUIRED mixed-state display tests: Busy+Unreadable, Open+RunRed,
  Busy+RunHung — the display carries both parts in each.

## Events (r2 finding 9 closed)

scripts/agents/event-registry.json gains component "run" with
events run-launched, run-transition, run-swept; runId joins the
canonical identifier set in internal/events; authorized emitters:
the run verbs and the watcher. A conformance test proves the events
are emittable (the registry is a closed catalogue that silently
drops unknowns — the row exists so they are known).

## Exchangeability

Files and engine verbs only. Any runtime's session prints and runs
the same waiter command, arms the same supervision, reads
`run status` by instruction, and gets the same verdict through the
same hook contract.

## Blast radius

internal/run (NEW), internal/dispatch (watch verb decision core +
waiter records), scripts/agents/dispatch.sh (watch plumbing + the
printed waiter line), cmd/metasystem supervise_component.go
(WatcherPass run pass + attestation), internal/census (RUN group
custody), internal/goal (RunFacts, --main-id, the two new state
arrays), internal/report scan.go, internal/lease (run sweep),
internal/events + scripts/agents/event-registry.json,
cmd/metasystem (run family + watch + turn-verdict flag), fixtures,
docs.

## Design-obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| MON-01 | CRITICAL | Launch topology | Pending-before-process with nullable identity; wrapper binds via nonce CAS; fence and launcher error path both conclude launch-failed with a note, never deletion | internal/run | internal/run/run.go | run_test.go TestLaunchReservationAndFence | fixture leg: fast-exit concluded; killed launcher leaves launch-failed | PARTIAL | implement |
| MON-02 | CRITICAL | Evidence | Generation-scoped nonce-checked sidecars; pattern only on adopted records, no-match=ended-unknown; dead+no-evidence=ended-unknown (one verdict); Unknown concludes nothing | internal/run | run.go Conclude | run_test.go TestConcludeEvidenceTable | fixture: green, red, stale-generation sidecar ignored, unknown surfaces | PARTIAL | implement |
| MON-03 | CRITICAL | Census custody | Group custody with the argv-nonce third factor; draining keeps survivors owned until provably gone or wind-down expiry; ArgvKnown=false surfaces | internal/census | census run source | census run_test.go TestRunGroupCustodyAndDrain | supervision-fixtures.sh leg: detached run + children never UNTRACKED while draining | PARTIAL | implement |
| MON-04 | CRITICAL | The waiter contract | dispatch prints the watch command; watch blocks to terminal and registers an identity-verified waiter record removed on exit | internal/dispatch + scripts/agents/dispatch.sh | watch decision core + dispatch.sh | dispatch watch test + waiter record round-trip | dispatch-fixtures.sh watch leg | PARTIAL | implement |
| MON-05 | CRITICAL | Turn verdict | Unwatched (caller's jobs without live waiters, runs, or stale attestation) blocks once on the id-set digest; run warnings always prepend to Display; green once per session | internal/goal + internal/report | turnverdict.go + scan.go | turnverdict_test.go TestUnwatchedAndWarnings + mixed-state display tests | supervision-fixtures.sh run leg through the real hook | PARTIAL | implement |
| MON-06 | CRITICAL | Serialization + lease | CAS over (status,generation); adopt increments generation and clears hungSince; all mutations under lease run-held; sweep signals only with argv-nonce proof, refuses loudly without | internal/run + internal/lease | run.go + sweep.go | run_test.go TestGenerationFencing + sweep test | fixture: stale-epoch run swept only with proof; adopted-unverified only surfaced | PARTIAL | implement |
| MON-07 | HIGH | Watcher host + attestation | The run pass inside the Go WatcherPass; runs-pass.json written only after a successful pass; the verdict requires it fresh | cmd/metasystem + internal/run | supervise_component.go | component pass test + attestation freshness test | supervision fixture: armed repo concludes a run | PARTIAL | implement |
| MON-08 | HIGH | Authority | launch/register/adopt/ack/prune holder-only with nullable HUMAN coordinates; conclude record-writer; custody labels honest and evented | internal/run + cmd/metasystem | run.go + cmd wiring | run_test.go TestAuthorityMatrix | — | PARTIAL | implement |
| MON-09 | HIGH | Events | Registry gains the run component and events; runId a canonical identifier; emitters authorized; conformance test | internal/events + scripts/agents/event-registry.json | emit.go + registry | events conformance test | — | PARTIAL | implement |
| MON-10 | MEDIUM | Ledger honesty | prune acked-terminal >14d with sidecars, drops reported | internal/run | run.go Prune | run_test.go TestPruneReportsDrops | — | PARTIAL | implement |
| MON-11 | MEDIUM | Grammar | Every bound at the source: display 200 via --display, log 512 exists-or-creatable, nonce 32 hex, pattern 256, staleAfterMin 1..1440, continuations 240 incl. unknown; sidecar suffix excluded from the record glob; status output shape pinned | internal/run | run.go validation | run_test.go TestBounds | — | PARTIAL | implement |
