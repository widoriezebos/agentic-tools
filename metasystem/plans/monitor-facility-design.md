# The monitor facility: tracked long-running work with terminal-state watching

Working Mode: design

Owner: main session (delegate), goal monitor-facility (the ledger's
Current goal; detail notes at plans/backlog-notes.md item 15).
Status: r1 draft, awaiting first critique. Human rulings fixed as
input: the exchangeability doctrine (any runtime fills any seat;
mechanisms are files, engine verbs, and plain prompt text), and the
human's verbatim intent — the launch-detached + monitor + resume
pattern proven during the D33 work "is generic behavior that should
end up in the meta system."

## The problem, in the human's words and two incidents

A detached suite run is invisible to the metasystem today: the
census flags its processes UNTRACKED, no record says what it is or
what should happen when it ends, and only the orchestrating agent's
own harness (a Claude Code Monitor watching a log tail) knows to
care. Two same-day incidents motivate the shape: the harness's
10-minute cap killed tracked suite runs, forcing detached launches;
and a monitor caught a 205-second suite failure immediately where
the fallback timer would have slept 15 minutes on it. The pattern
worked — but it lives in one runtime's harness and in hand-written
continuation prompts, which is exactly the exchangeability failure
the doctrine forbids.

## One mechanism or two (the composition ruling this design makes)

Backlog item 1 (a turn may not end with unwatched work) and this
item are ONE mechanism with two enforcement points. The RUN RECORD
is the mechanism: a registered fact that long-running work exists,
who watches it, and what happens at each terminal state. Item 1's
"do not end the turn blind" is then a VERDICT rule over run records
— the same ScanResult/turn-verdict machinery item 14 shipped —
not a second tracker: a session that dispatched work still in
flight with no watcher armed ends its turn against a block, the
same once-only shape as open work. The goal system carries WHAT
next; the run record carries WHEN; the turn verdict is the one
place both speak.

## The run record: artifacts/agents/runs/<run-id>.json

One JSON object per registered run, engine-written (verbs below),
atomic, schema-versioned:

    {"schemaVersion": 1,
     "runId": "<kebab, unique, ≤64 bytes>",
     "kind": "suite" | "cohort" | "mission" | "custom",
     "command": "<one display line, ≤200 bytes>",
     "pid": <int>, "pidStartedAt": <int>,        // kernel identity
     "log": "<path>",
     "startedAt": "<ISO>",
     "expect": {"green": "<one line ≤240>",       // continuation notes,
                "red": "<one line ≤240>",         // plain text consumed
                "hung": "<one line ≤240>"},       // by any runtime
     "staleAfterMin": <int>,                      // hang threshold
     "status": "running" | "green" | "red" | "hung" | "vanished",
     "exitCode": <int|null>,
     "endedAt": "<ISO|null>",
     "sessionId": "<normalized owner session, may be empty>",
     "goalId": "<the Current goal at registration, may be empty>"}

Bounds at the source, exactly like the goal ledger: every projected
field carries its byte cap. The continuation ("expect") is PLAIN
TEXT for the next reader — never executable, never auto-run: the
facility tells any runtime's agent what was intended; acting stays
an agent decision (the honest limit from item 1's notes: nothing
outside a session can wake it, so the mechanism enforces "do not
end blind", not "you will be woken").

## Verbs (family `run`, mirroring the goal family's discipline)

- `run register --id --kind --command --pid --log --stale-after-min
  [--expect-green --expect-red --expect-hung]`: writes the record
  after verifying the pid's kernel identity (nonempty-or-error,
  gaterun's corrected contract); records the caller's session and
  the Current goal via the exported goal parser (read-only).
- `run status [--id]`: read-only JSON.
- `run conclude --id`: re-probes and finalizes a record whose
  process already ended (the watcher's verb; also callable by hand).
- `run adopt --id --pid`: re-binds a record to a successor process
  (a resumed suite), same identity verification.
- `run prune`: drops terminal records older than a bounded age,
  reporting drops (the ledger-honesty rule; composes with item 19).

Registration is holder-gated like every record write
(record-writer authority path); reads are open.

## The watcher: one added classification in the existing pass

watch-background-jobs.sh (and its Go census core) already trips on
DONE/STALE/CAPPED/VANISHED for job records. The run watcher is the
SAME pass over runs/: a run whose recorded process is provably dead
concludes as green/red by exit evidence (see below); alive but
log-quiet past staleAfterMin emits hung (once per transition, not
per pass); a record whose identity cannot be verified follows the
three-way discipline — Unknown never concludes anything, it
surfaces. Terminal transitions emit flight-recorder events
(component "run", event "run-terminal") — the narrator's raw
material and the monitor's wake signal on runtimes that can tail.

Exit evidence, honestly: a detached process's exit CODE dies with
its parent. The registered command is therefore wrapped by the
engine at launch — `run launch` (below) appends the exit line to
the log — or, for runs registered after launch, the conclusion
falls back to log-verdict matching: the record may carry
`verdictPattern` (green iff the log's tail matches). Both paths are
stated in the record; a record with neither concludes only as
"ended (verdict unknown)" — surfaced, never guessed.

- `run launch --id --kind --log ... -- <command...>`: the composed
  path — setsid, redirect, register, write the exit line at the
  end. Shell plumbing composing engine verbs, per the boundary
  doctrine.

## Census: registered runs are accounted for

The census ownership walk (CUSTODY/ANNOUNCED/UNTRACKED) gains one
source: a process whose pid+start matches a running run record is
OWNED ("RUN <id>"), never UNTRACKED. The suite's detached self-run
therefore stops tripping the watchdog. The match is identity-exact;
a pid-reused impostor fails the start-time check and stays
UNTRACKED (the same rule everywhere).

## The turn-verdict rule (item 1, landed as one ScanResult field)

ScanResult.Busy already carries live runs' facts once the scanner
reads runs/ (kind "run", the display detail). Two additions:
- A RUNNING run whose owner session is THIS session and which has
  no live watcher (the watcher's heartbeat is stale or absent)
  makes the turn end block ONCE: "work you launched is unwatched:
  <run-id>; arm the watcher or conclude the run" — the same
  per-session block-once state file item 14 shipped
  (blockedGoalRevisions' sibling slot, keyed on the run-id set
  digest).
- A TERMINAL-unacknowledged run (status red/hung, not yet
  acknowledged) surfaces in the display with its continuation note
  verbatim: "suite run <id> went red; the run record says: <expect.
  red>". Acknowledgment is `run conclude --ack` — recorded, so the
  message stops.

The goal composition (GOAL-12's seam, now consumed): the verdict's
goal clause and the run clause meet in the display — "open work is
done; the goal file names the next step: X" only when no run is
running, red-unacknowledged, or unwatched. Precedence: Busy (runs
included) → open-work → waits → unreadable → runs-unacknowledged →
goal → all-clear.

## Exchangeability

Everything above is files and engine verbs. A Claude session arms
the shell watcher; a codex or devin session arms the SAME shell
watcher; any runtime's agent reads `run status` by instruction.
The one per-runtime nicety — a harness-native monitor that wakes
the agent — remains optional sugar on top of the same records,
exactly as the delivery contract treats hooks.

## Blast radius

internal/run (NEW: record schema, verbs, conclusion rules),
internal/report (scanner reads runs/ into ScanResult.Busy + the two
verdict rules), internal/goal (turnverdict: the unwatched-run and
unacknowledged slots), internal/census (the RUN ownership source),
scripts/watch-background-jobs.sh (the run pass), scripts/agents/
run-launch plumbing, cmd/metasystem (verb rows), fixtures, docs.

## Design-obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| MON-01 | CRITICAL | The run record | register verifies kernel identity nonempty-or-error; records are atomic, bounded, schema-versioned | internal/run | internal/run/run.go | run_test.go TestRegisterVerifiesIdentity | fixture leg registering a live and a dead pid | PARTIAL | implement |
| MON-02 | CRITICAL | The watcher | terminal transitions conclude by exit-line or verdict-pattern evidence, never guessed; Unknown surfaces and concludes nothing | internal/run + watcher | run.go Conclude + watch-background-jobs.sh | run_test.go TestConcludeThreeWay | fixture: green, red, and unknown-identity runs through the watcher pass | PARTIAL | implement |
| MON-03 | CRITICAL | Census | a running run record's exact identity owns its process; pid reuse stays UNTRACKED | internal/census | run.go ownership source | census run_test.go TestRunOwnership | supervision fixture leg: registered detached run not UNTRACKED | PARTIAL | implement |
| MON-04 | HIGH | Turn verdict | an unwatched running run owned by this session blocks once; red/hung unacknowledged surfaces its continuation verbatim | internal/goal + internal/report | turnverdict.go + scan.go | turnverdict_test.go TestUnwatchedRunBlocksOnce | supervision-fixtures.sh run leg through the real hook | PARTIAL | implement |
| MON-05 | HIGH | Launch plumbing | run launch wraps setsid+log+register+exit-line; a wrapper failure is loud | scripts/agents + internal/run | run-launch path | fixture: launch, kill, conclude round-trip | dispatch-fixtures.sh or its own fixture file | PARTIAL | implement |
| MON-06 | MEDIUM | Ledger honesty | run prune reports drops; records are not an audit log (events are) | internal/run | run.go Prune | run_test.go TestPruneReportsDrops | — | PARTIAL | implement |
| MON-07 | MEDIUM | Goal seam | the run clause consumes the exported goal parser read-only; goalId is informational, never authority | internal/run | run.go register | run_test.go TestGoalIdInformational | — | PARTIAL | implement |
