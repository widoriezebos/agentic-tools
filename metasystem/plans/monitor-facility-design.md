# The monitor facility: tracked long-running work with terminal-state watching

Working Mode: design

Owner: main session (delegate), goal monitor-facility (the ledger's
Current goal; detail notes at plans/backlog-notes.md item 15).
Status: r2 — folds the twelve r1 findings (critique at
plans/monitor-facility-critique-r1.md). Human rulings fixed as
input: the exchangeability doctrine, and the human's verbatim
intent — the launch-detached + monitor + resume pattern "is generic
behavior that should end up in the meta system."

## The problem, in the human's words and two incidents

A detached suite run is invisible to the metasystem today: the
census flags its processes UNTRACKED, no record says what it is or
what should happen when it ends, and only the orchestrating agent's
own harness knows to care. Two same-day incidents: the harness's
10-minute cap killed tracked suite runs, forcing detached launches;
and a monitor caught a 205-second suite failure immediately where a
fallback timer would have slept 15 minutes on it. The pattern works
— but it lives in one runtime's harness, which is the
exchangeability failure the doctrine forbids.

## One mechanism, two record kinds, one verdict (r1 finding 1)

Backlog item 1 (a turn may not end with unwatched work) folds in
WITHOUT duplicating lifecycles: delegate jobs already have records,
watchers, and terminal semantics — the RUN RECORD exists only for
NON-JOB work (suites, cohorts, any detached process). "Watched"
means what the arm-once doctrine already means: the checkout's
supervision is armed and its watcher heartbeat is fresh — the
design does not invent per-job waiter registration; it makes the
existing arm-once contract ENFORCEABLE at turn end. The verdict
rule spans both kinds: jobs THIS session dispatched still in
flight, and runs it registered, with supervision unarmed or its
heartbeat stale, block the turn end once. The goal system carries
WHAT next; the run record carries WHEN; the turn verdict is where
both speak. Flight-recorder events narrate transitions for the
narrator and any tailing monitor, but they are NEVER the wake
authority — the recorder's own contract permits lost finals; the
authoritative facts are the record and kernel state (r1 closing).

## The run record: artifacts/agents/runs/<run-id>.json

Engine-written, per-record flock + compare-and-swap + legal
transitions exactly like delegate job records (r1 finding 8; the
dispatch record layer is the pattern owner). Schema (v1):

    {"schemaVersion": 1,
     "runId": "<kebab, unique, ≤64>",            // collision refuses
     "kind": "suite" | "cohort" | "custom",
     "display": "<from --display only, ≤200>",    // never argv join:
                                                   // argv can carry secrets
     "pid": <int>, "pidStartedAt": <int>,          // wrapper identity
     "pgid": <int>,                                // the owned GROUP
     "launchNonce": "<hex>",                       // binds wrapper to record
     "log": "<path, ≤512, resolved at register>",
     "startedAt": "<ISO>",
     "mainId": "...", "ownerLineage": "...",       // lease coordinates:
     "claimEpoch": <int>,                          // runs are swept, fenced,
                                                   // and identified across
                                                   // takeovers like jobs
     "sessionId": "<normalized>",
     "goalId": "<informational, never authority>",
     "staleAfterMin": <int, 1..1440>,
     "hungSince": "<ISO|null>",                    // a FLAG on running,
                                                   // cleared by log activity
                                                   // — never a status
     "evidence": {"mode": "exit-sidecar" | "pattern" | "none",
                  "verdictPattern": "<RE2, adopted records only>"},
     "expect": {"green": "≤240", "red": "≤240", "hung": "≤240"},
     "status": "launching" | "running" | "green" | "red"
              | "ended-unknown" | "vanished" | "launch-failed",
     "acked": false,
     "exitCode": <int|null>, "endedAt": "<ISO|null>"}

Legal transitions (enforced at the CAS, refusals loud):
launching → running (the wrapper binds via launchNonce)
launching → launch-failed (the bounded launch fence, with rollback)
running → green | red | ended-unknown (evidence rules below)
running → vanished (identity provably gone, no evidence)
terminal → terminal is refused; adopt resets IDENTITY fields only
(pid/pidStartedAt/pgid) on a running record.

## Launch topology and the register race (r1 findings 3+4)

`run launch` follows the delegate-job reservation pattern: it
writes the PENDING record (status launching, launchNonce minted)
BEFORE any process exists; then starts the wrapper via setsid — the
wrapper IS the recorded pid and the process-group leader; the
wrapper's first act is the CAS launching→running binding its
kernel identity; its last act is the atomic exit SIDECAR. A
launching record older than a bounded fence (default 2 minutes)
with no binding concludes launch-failed, and the launcher's error
path removes its own pending record (rollback). Fast commands
cannot escape: the record exists before the process.

Census custody covers the GROUP (r1 finding 3): a process belongs
to a run when its pgid equals the record's pgid AND the record's
leader identity verifies (pid+start). Agent-shaped descendants of a
detached suite are therefore OWNED ("RUN <id>"), not UNTRACKED; a
pid-reused impostor fails the leader check and stays UNTRACKED.
Census owns launching and running records only — terminal records
own nothing.

## Exit evidence: the sidecar wire contract (r1 finding 10)

The wrapper writes `runs/<run-id>.exit.json` atomically:
{"runId", "nonce", "exitCode", "endedAt"} — nonce must equal the
record's launchNonce or the sidecar is ignored as forged/stale. No
in-log exit lines: logs are the workload's channel, descendants may
hold the fd, rotation exists — the sidecar is ours alone. Verdict:
exitCode 0 → green; nonzero (incl. 128+n signal deaths) → red.
`verdictPattern` exists ONLY for adopted records (no wrapper, no
sidecar): RE2 grammar, matched against a bounded 64 KiB log tail at
conclusion; match → green, no-match → ENDED-UNKNOWN, never a red
guess. Dead process + no sidecar + no pattern → ended-unknown.
Unknown identity concludes NOTHING (three-way discipline): it
surfaces and stays.

## The watcher: the Go supervised component (r1 finding 2)

The run pass lands in WatcherPass — the `supervise component
--component watcher` work function armed supervision ACTUALLY runs
— beside the census, not in watch-background-jobs.sh (which has no
production caller; the shell script gains nothing). Each pass: for
every launching/running record, verify identity three-way; apply
the launch fence; apply evidence rules on death; set/clear
hungSince by log mtime against staleAfterMin; emit one
flight-recorder event per TRANSITION (never per pass). Conclusion
runs under the record's flock+CAS, so a concurrent manual
`run conclude` cannot double-write.

## Verbs and authority (r1 finding 9 — the matrix, per verb)

- `run launch` — holder-only. Pending record + wrapper + sidecar.
- `run register` — holder-only; binds an ALREADY-RUNNING process:
  requires the caller to prove group membership (the target's pgid
  session contains the caller's ancestry) or the record is stamped
  `custody: "adopted-unverified"` — census still owns it, the label
  is honest, and the act is an explicit holder decision, evented.
- `run adopt` — holder-only; rebinding identity on a RUNNING record
  (a resumed suite), same proof-or-label rule; epoch rechecked.
- `run ack` — holder-only; sets acked on a terminal record.
- `run conclude` — record-writer (supervision's watcher concludes;
  the holder may too); evidence rules only, never opinion.
- `run status` / `run list` — open reads; JSON: {"schemaVersion":1,
  "runs":[<records>]} sorted by startedAt.
- `run prune` — holder-only; drops ACKED terminal records older
  than 14 days, reporting every drop (ledger honesty; the events
  are the audit trail, not the records).

Takeover: the lease sweep treats stale-epoch running runs exactly
like stale jobs — prove group ownership by tag/nonce before any
signal, refuse without proof (the corrected sweep discipline).

## Scanner and verdict (r1 findings 7+11 — typed facts, compositional precedence)

ScanResult gains a TYPED field: `Runs []RunFact{Id, Status,
OwnedByThisSession, Watched, Acked, HungSince, Continuation}` with
Continuation ≤240 carried whole (the 200-byte Busy detail is not
the carrier). Live runs ALSO project into Busy items for display
continuity. Precedence is COMPOSITIONAL, not a longer ladder:

- Run WARNINGS — terminal-unacknowledged in {red, vanished,
  ended-unknown} or currently hung — ALWAYS join the display, above
  everything, continuation verbatim: "suite run <id> went red; the
  run record says: <expect.red>". Green terminal unacknowledged
  surfaces once as information (expect.green verbatim), never
  blocks.
- The UNWATCHED-OWN-WORK block: jobs this session dispatched in
  flight or runs it registered running, with supervision unarmed or
  the watcher heartbeat stale → block ONCE, keyed on the sha256 of
  the sorted unwatched id set, stored in a sibling slot of the
  session state (blockedUnwatchedDigests, ≤16 FIFO — the item-14
  state file gains one array, schema unchanged v1 additive).
- Busy (runs included) suppresses only the GOAL clause, exactly as
  today — it can never hide a run warning.
- Unreadable/malformed run records or sidecars append to
  ScanResult.Unreadable (the two-sided veto item 14 shipped —
  never silently skipped, r1 finding 11; the census run-custody
  reader refuses the silent-skip pattern for the same reason).

## Exchangeability

Files and engine verbs only. Any runtime's session arms the same
supervision, reads `run status` by instruction, and gets the same
turn-end verdict through the same hook contract. Harness-native
monitors remain optional sugar over the same records.

## Blast radius

internal/run (NEW: schema, CAS, verbs, evidence rules),
cmd/metasystem supervise_component.go WatcherPass (the run pass),
internal/census (the RUN group-ownership source; no silent skips),
internal/goal (turnverdict: RunFact rules + blockedUnwatchedDigests),
internal/report scan.go (runs into ScanResult), internal/lease
(sweep covers runs), cmd/metasystem (run family rows),
scripts/agents run-launch wrapper plumbing, fixtures, docs.

## Design-obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| MON-01 | CRITICAL | Launch topology | Pending-before-process reservation; wrapper binds via launchNonce CAS; bounded launch fence concludes stale pending launch-failed with rollback | internal/run | internal/run/run.go | run_test.go TestLaunchReservationAndFence | fixture leg: fast-exit command still concluded; killed launcher rolls back | PARTIAL | implement |
| MON-02 | CRITICAL | Evidence | Sidecar nonce-bound exit verdicts; pattern only on adopted records with no-match=ended-unknown; Unknown concludes nothing | internal/run | run.go Conclude | run_test.go TestConcludeEvidenceTable (sidecar green/red/forged; pattern match/no-match; unknown surfaces) | fixture: green, red, and unknown-identity runs through the watcher pass | PARTIAL | implement |
| MON-03 | CRITICAL | Census | Group custody: pgid match + leader identity verifies; pid reuse stays UNTRACKED; unreadable run records surface, never silently skip | internal/census | census run source | census run_test.go TestRunGroupOwnership | supervision-fixtures.sh leg: registered detached run and its children not UNTRACKED | PARTIAL | implement |
| MON-04 | CRITICAL | Turn verdict | Run warnings always surface with continuation verbatim; unwatched-own-work blocks once on the id-set digest; Busy never hides a warning | internal/goal + internal/report | turnverdict.go + scan.go | turnverdict_test.go TestRunWarningsAndUnwatchedBlock | supervision-fixtures.sh run leg through the real hook | PARTIAL | implement |
| MON-05 | CRITICAL | Serialization + lease | Per-record flock+CAS+legal transitions; adopt resets identity only; lease coordinates stamped; takeover sweep proves before signaling | internal/run + internal/lease | run.go + sweep.go | run_test.go TestTransitionTableAndRaces + lease sweep test | fixture: stale-epoch run swept only with proof | PARTIAL | implement |
| MON-06 | HIGH | Watcher host | The run pass runs inside the Go WatcherPass armed supervision executes; one event per transition | cmd/metasystem + internal/run | supervise_component.go | component test: pass over a fixture runs dir | supervision fixture: armed repo concludes a run | PARTIAL | implement |
| MON-07 | HIGH | Authority | launch/register/adopt/ack/prune holder-only; conclude record-writer; custody proof or adopted-unverified label, evented | internal/run + cmd/metasystem | run.go + goal-family-style cmd wiring | run_test.go TestAuthorityMatrix | — | PARTIAL | implement |
| MON-08 | HIGH | Item-1 fold | Jobs this session dispatched count in the unwatched rule from their EXISTING records; no second lifecycle | internal/report + internal/goal | scan.go job facts | turnverdict_test.go TestUnwatchedCoversJobs | supervision fixture leg | PARTIAL | implement |
| MON-09 | MEDIUM | Ledger honesty | prune acked-terminal >14d only, reporting drops; events are the audit trail | internal/run | run.go Prune | run_test.go TestPruneReportsDrops | — | PARTIAL | implement |
| MON-10 | MEDIUM | Grammar | Every bound enforced at the source: display 200 via --display only, log 512 resolved, staleAfterMin 1..1440, continuation 240; status output shape pinned | internal/run | run.go validation | run_test.go TestBounds | — | PARTIAL | implement |
