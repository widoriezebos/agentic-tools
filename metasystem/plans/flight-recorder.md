# The flight recorder

- Goal and current status: THE observability stream of the metasystem — a
  single, safely-appended, machine-readable event log such that one file (or
  one collected bundle) from any machine is enough to diagnose an end-to-end
  session: what ran, what went well, where it broke, and why. Named by the
  human 2026-08-09, who also set the priority: this stream is NEXT, ahead of
  further benchmark attempts, because the day measured roughly four hours of
  finding-out per hour of fixing. This file is the STREAM HEAD; the
  companion [adapter-streaming](adapter-streaming.md) design is folded in as
  this stream's turn-interior leg and no longer sequences independently.
- Next step: critique this design (with adapter-streaming.md, as one
  design) with sol, then implement the CORE leg
- In flight right now: nothing
- Waiting on the human: nothing

## Legs, in landing order (the human's reprioritization, amended)

1. CORE (before benchmark attempt 3): the emitter (D-1..D-3) wired into the
   components that burned us — lease, reaper, dispatch, mission runner,
   cohort driver — plus `tail -f` as the live view. Attempt 3 then runs
   UNDER the recorder: if the attempt fails, the log it leaves is the
   recorder's own acceptance test against reality.
2. TURN INTERIOR: adapter-streaming's transports (codex tail, claude
   stream-json, devin ACP-per-turn), emitting throttled activity events into
   this stream. Its liveness sidecar is subsumed per D-6 unless the critique
   revives it.
3. BUNDLE AND SURFACES: collect-observability (D-4), the status formatter,
   and the suite's structured verdicts and preserved failure evidence (D-7).
   The failure-evidence preservation may land earlier under the direct-fix
   bar; it is mechanical and purely diagnostic.

## The requirement, in the human's terms

Stated 2026-08-09: full observability into the working of the entire
metasystem; a single log of a full end-to-end session, safe to write
concurrently, watchable live by tailing the file; and shippable from another
machine so that issues can be diagnosed and the metasystem improved without
access to the machine that ran it. "I don't think we have perfect
observability yet. And that's one thing I really want."

They are right that we do not have it. The evidence is the same morning's
diagnosis: reconstructing why a mission died silently took correlating a
dozen files by hand — the runner record, its heartbeat, an empty runner log,
the arming log, the census log, the turn directory, the job records, and two
ad-hoc watch logs. Every one of those held a fragment; none held the story.
A 4.5-hour stall was invisible because no single place said "nothing has
happened since 03:09".

## What exists today, honestly

The artifact tree is excellent GROUND TRUTH and poor NARRATIVE. Job records,
mission state, the lease, transcripts, and ledgers say what things ARE; only
scattered per-component stderr logs (arming.log, census.log, reaper.log,
runner .log, host.log, dispatch's job logs) say what HAPPENED, each in its
own format, its own place, its own clock. Diagnosis is archaeology. Nothing
is shippable as a unit.

## D-1. One append-only event stream per checkout

`artifacts/agents/events.jsonl` — one file per checkout (the harness has
one; every benchmark target has its own). One event per line:

    {"ts": "<ISO-8601 UTC, milliseconds>", "component": "dispatch",
     "pid": 123, "pidStartedAt": 456, "event": "job-verdict",
     "level": "info", "missionId": "bm-2", "jobId": "…", "turnId": "…",
     "summary": "timeout: budget-cap after 913s (cap 900s)",
     "ref": "artifacts/agents/jobs/….json", "schemaVersion": 1}

- Identity fields are optional per event; `ts`, `component`, `pid`,
  `pidStartedAt`, `event`, `summary`, `schemaVersion` are required. The
  (pid, pidStartedAt) pair is the same kernel-fact identity the rest of the
  system uses — an event is attributable to a process, not a claim.
- `ref` points at the artifact that holds the full truth. The event carries
  a one-line summary, never a payload: prompts, transcripts, and diffs stay
  in their files. (Size discipline is what keeps the log tailable and
  shippable; the collect bundle in D-4 carries the referenced files.)

## D-2. Concurrency safety, stated precisely

Many processes write one file. The mechanism is POSIX `O_APPEND` with each
event written as ONE `write()` of ONE line, hard-capped under 4096 bytes
(summaries truncate to fit). On a local filesystem, such appends do not
interleave. Stated assumptions, which the critique should attack:

- Local filesystem only. NFS does not guarantee atomic O_APPEND; the design
  declares the artifact tree local, which is already true everywhere the
  metasystem runs (worktrees, benchmark targets).
- No locks, no daemon, no broker. A crashed writer can lose ITS OWN last
  event, never corrupt another's. A torn line (power loss) is skipped by
  readers: every reader treats an unparseable line as absent, because the
  log is observational (D-5), never authoritative.
- Ordering is by timestamp plus file order per writer; cross-process order
  within the same millisecond is not promised and nothing may depend on it.

## D-3. Who emits, and what

One tiny shared emitter — a shell function backed by a python helper — used
by every component. The existing per-component logs REMAIN as raw stderr
(they are the debugging of last resort); the event stream is the structured
layer above them. Decision points that must emit, drawn from this week's
real failures — each of these was a fragment someone had to dig for:

- lease: claim, renewal (same lineage), takeover (with predecessor, reason,
  epoch), refusal (OWNED-ELSEWHERE), interrupted-sweep completion.
- census: verdict transitions, every UNTRACKED finding, writer-lock events.
- reaper: every verdict with its reason (process-lost, budget-cap,
  handshake_timeout, abandoned-setup, stale-claim-epoch) and what it wound
  down; the KI-29 class of bugs becomes visible as adjacent contradictory
  events rather than a mystery.
- dispatch: record transitions (created, setup, pending, running, terminal),
  handshake deadlines set and hit, chain lock acquisition failures.
- mission runner: start, per-turn launch and result, fence checks, park,
  ask/answer, runner failure WITH the error (the empty runner log of
  2026-08-09 is exactly what this line abolishes), wind-down decisions.
- adapters: turn started, session established, streamed activity (throttled
  to at most one event per N seconds per round — the adapter-streaming
  design's liveness data lands HERE), repair turn attempted/outcome, turn
  ended with verdict and usage summary.
- cohort driver and gates: phase transitions, provisioning, seal/sign
  boundary reached, grading, extraction, gate verdicts.

## D-4. Live view and the cross-machine bundle

- LIVE: `tail -f artifacts/agents/events.jsonl` is the human's window — the
  exact alternative the human proposed. The status surface described in
  adapter-streaming D-5 becomes a FORMATTER over this stream plus the state
  files; it stops needing any mechanism of its own.
- BUNDLE: `scripts/agents/collect-observability.sh --out <archive>` walks a
  checkout (and, for a cohort, its targets), and produces one archive:
  every events.jsonl, the state files (jobs, missions, lease, census), and
  every artifact a collected event `ref`s. A merge tool renders the combined
  chronological narrative. The acceptance test is the human's own scenario:
  the bundle alone, on another machine, must be sufficient to reconstruct
  this week's runner-death timeline without guessing.

## D-5. The log is a witness, never an authority

No machinery decision may read the event stream as input: verdicts come from
records, liveness from the kernel, custody from the lease. This is the same
separation as the liveness sidecar's observation-is-not-a-kill-signal rule,
and it is what keeps the log free to be lossy, truncated, or absent without
making the system wrong — and keeps writers free to emit without locks. A
missing or damaged log degrades diagnosis, never correctness.

## D-6. Relationship to adapter-streaming.md

The two designs meet where the adapter emits throttled activity events:

- The liveness sidecar (adapter-streaming D-3) MAY BE SUBSUMED by this
  stream: "last event for round X" is a query over events.jsonl, and the
  human's tail-the-file alternative does the sidecar's job with one
  mechanism instead of two. The critique decides: sidecar, stream, or both.
  The default position of this draft: stream only, unless the critique finds
  a reader for whom a per-round file is materially cheaper than a filtered
  tail.
- Everything else in adapter-streaming (transports, ACP-per-turn, the
  interface boundary, the proofs) stands on its own and is unaffected.

## D-7. The validation suite is a subject too

Added 2026-08-09 after a day that proved it: five suite runs, each failure
diagnosed by grepping prose, locating the fixture in a four-thousand-line
script, and reading the code under test — roughly three hours, of which most
was finding out what happened rather than fixing anything. Two changes:

- STRUCTURED FIXTURE VERDICTS. The suite emits one event per fixture to its
  own events.jsonl (same emitter, same schema; component "suite"): fixture
  name, verdict, duration, and on failure the path to its evidence. The
  human-readable output stays; the structured layer is for diagnosis and for
  comparing runs (which fixtures got slower, which flake recurs, at what
  rate — KI-25 and KI-29 would have been graphs, not anecdotes).
- PRESERVE FAILURE EVIDENCE. Today every fixture's temp tree dies in a
  `trap rm -rf EXIT` INCLUDING ON FAILURE: when the runner-unverified
  fixture failed, the runner log, mission state, and job records needed for
  diagnosis were already deleted, and the diagnosis ran on the three lines
  the fixture chose to print. On failure, the tree moves to a kept location
  under artifacts/ and the failure event carries the path. This is the
  single highest-leverage observability change available and is mechanical
  (a candidate for the direct-fix bar once the current cohort is done).

## Rotation, size, and lifecycle

- The stream rotates per session/mission boundary (a fresh mission renames
  the previous file aside with its end timestamp) and evidence-gc owns aging
  the rotated files, same as other artifacts.
- The 4KB line cap plus payload-by-reference keeps a full mission in the
  low megabytes; a cohort bundle is bounded by its transcripts, not by the
  log.

## Proof

- Concurrent writers: N processes appending in a tight loop produce a file
  with zero torn or interleaved lines and all N counters complete.
- Narrative reconstruction: a scripted mission (fake adapter) produces a
  stream from which a checker reconstructs the exact expected phase
  sequence — provision, arm, claim, turns, verdicts, park — with no access
  to anything but the log.
- The 2026-08-09 regression, replayed: a runner killed mid-mission leaves a
  stream whose last events name the failure (`runner-failed`,
  `lost ownership proof …`), and the "dead run" is detectable from the log
  alone by last-event age — the 4.5-hour blind stall becomes a one-line
  query.
- Bundle sufficiency: collect on a finished fixture cohort, move the archive
  to a scratch directory, and run the narrative checker there — the bundle
  alone must reconstruct the run.
- Non-authority: a run with the event stream deleted mid-mission completes
  with identical verdicts and artifacts.
- Tail safety: a reader tailing during heavy concurrent writes never parses
  a partial line as an event.
- Suite half: a deliberately failed fixture leaves a preserved evidence tree
  at the path its failure event names, and the per-fixture verdict events
  for a full run reconstruct the run's shape (count, order, durations)
  without reading the prose log.
