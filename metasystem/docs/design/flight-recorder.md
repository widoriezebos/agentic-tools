# The flight recorder: the event-stream contract

The durable rules (D-1..D-8) of the per-checkout append-only event
stream, promoted from `plans/flight-recorder.md` (which keeps the
requirement history and landing order). `internal/events` implements
the emitter; `scripts/agents/event-registry.json` is the closed event
catalogue. A change to any D rule is a contract change, not a cleanup.

## D-1. One append-only event stream per checkout

`artifacts/agents/events.jsonl` — one file per checkout (the harness has
one; every benchmark target has its own). One event per line.

### D-1a. The schema, fully stated

Required on every event:

- `schemaVersion` (integer, 1), `ts` (string, ISO-8601 UTC with
  milliseconds), `component` (string, CLOSED set: lease, census, reaper,
  dispatch, runner, adapter, host, driver, suite, arming), `event` (string,
  from the registry), `level` (string: info|warning|error), `summary`
  (string, REQUIRED but may be empty after truncation — it is never
  omitted, resolving the earlier drop-vs-require ambiguity), `pid`
  (integer), `pidStartedAt` (integer, epoch seconds), `seq` (integer >= 1,
  see D-1c for whose counter it is).

Identity is ATTRIBUTION, not proof: the writer self-reports its (pid,
pidStartedAt). The first draft claimed kernel-fact identity, which the
emitter cannot verify without becoming heavyweight; a witness may carry
self-reports, because nothing authoritative reads them (D-5). Anything
needing proven identity uses the records, as today.

Required WHEN APPLICABLE, and the registry says where — all typed here,
not deferred: `missionId`, `jobId`, `turnId`, `cohortId`, `executionId`
are strings (the ids their owning records already define, caps per D-2);
`repetitionIndex` is an integer and uses the DRIVER'S existing field name
— the draft's `repetition` is gone, one name across driver state and
events. An event's registry entry names the SET of components allowed to
emit it (turn events: {adapter, host}; everything else: exactly one), so
shared events are legal without loosening attribution. The
`executionId` is OPTIONAL and, when present, IS THE COHORT ID — nothing is
minted at all (round 3 killed the minting idea: the cohort driver is not
one persistent process; provisioning exits and each --resume is a fresh
invocation, so an environment value cannot survive the human boundary).
The cohort id already persists in the cohort state file, is read by every
driver invocation, and is unique; each invocation exports it to everything
it SPAWNS — and ONLY to what it spawns. SUPERVISION COMPONENTS NEVER CARRY
executionId (round-confirm's catch): a watcher or reaper may already be
alive from an earlier invocation, joined rather than restarted, and no
resume can reach into its environment. Rather than three narratives
depending on timing, the rule is uniform: supervision events are
checkout-scoped and join the cohort narrative through the checkout
identity the bundle already groups by; driver, runner, host, and adapter
events carry the executionId their spawning invocation exported. The
registry expresses this as CONDITIONAL requiredness — `executionId:
required-when-exported` — meaning: REQUIRED whenever the emitting process
has the driver's export in its environment, FORBIDDEN to invent otherwise.
For that rule and "supervision never carries it" to BOTH hold, the arming
SCRUBS the export from the environment it hands supervision components
(one explicit unset at the launch boundary) — a driver-spawned runner
inherits the export, its arming call inherits it too, and without the
scrub a freshly spawned watcher or reaper would be required to carry the
id the design forbids it.
A cohort implementation that never emits it fails the conformance fixture
(which runs one driver-spawned mission and asserts executionId on every
driver/runner/adapter event AND its ABSENCE on the supervision events of
the freshly spawned watcher and reaper — the scrub proven, not assumed),
and an interactive mission's events pass without it (the same fixture's
second half). A plain
interactive session has NO outer driver and therefore NO executionId; its
events join the ordinary way, by checkout, ids, and time. Re-arming
neither mints nor changes the value.

The EVENT REGISTRY is a MACHINE-READABLE deliverable —
`scripts/agents/event-registry.json` — and the emitter validates against it:
for every event name, the component that may emit it, the required ids, and
EVERY payload field with its type, requiredness, and closed value set where
one exists. Prose in this plan illustrates; the registry file is the
contract, and a fixture asserts emitted events conform to it. The closed
component set gains `arming`, and the registry's first entries include
arming-started, stream-rotated(previousPath), and arming-complete, because
arming emits its own events. EVERY payload field in the registry carries
the same discipline as the envelope fields: a declared type, requiredness,
a closed value set where one exists, and a BYTE CAP with the visible `~`
truncation rule — free-form payloads (a runner-failed `error`, a
stream-rotated `previousPath`) cap at 256 bytes, so the maximal event is
bounded by construction and the 4096 cap holds arithmetically, not
hopefully. The initial registry content ships WITH this design's
implementation and is version-controlled beside the emitter; entries fixed
by the failures that motivated them, each with its exact payload (field:
type, R required / O optional; strings capped per D-2 unless noted):

Each entry below reads: event {payload} [emitters | required ids]. Job
events REQUIRE jobId and carry missionId only when the job belongs to a
mission (stated per event, not guessed); job-verdict and verdict-refused
allow MULTIPLE emitters — dispatch and lease both legitimately win or lose
terminal CAS writes — so multi-emitter is a registry property any event
may declare, not a turn-event exception.

- lease-claimed {epoch: int R} [lease | —]; lease-renewed {epoch: int R}
  [lease | —];
  lease-takeover {reason: enum{holder-death} R, predecessor: string R,
  epoch: int R} [lease | —]; lease-refused {holder: string R}
  [lease | —]; sweep-completed {epoch: int R, sweptCount: int R}
  [lease | —].
- census-verdict {verdict: enum{SUCCESS,FAILED} R, generation: int R}
  [census | —]; census-untracked {observedPid: int R, argvSummary:
  string R} [census | —]; census-writer-claimed {} /
  census-writer-released {} [census | —].
- job-created/job-setup/job-pending/job-running {} [dispatch | jobId R,
  missionId O — present iff the job record carries one]; job-verdict
  {verdict: enum{completed,failed,timeout,cancelled} R, reason: string O}
  [dispatch, reaper, lease | jobId R, missionId O as above];
  verdict-refused {attempted: string R, observed: string R}
  [dispatch, reaper | jobId R, missionId O as above] (D-3a). The STANDING
  REAPER is the `reaper` component even though its code lives in
  dispatch.sh — attribution follows the role, not the file. `lease` emits
  job-verdict for its takeover sweep but NEVER verdict-refused: the sweep
  writes under the record lock and cannot lose a compare.
- runner-started {} [runner | missionId R]; turn-launched {} /
  turn-result {outcome: string R} [runner | missionId R, turnId R];
  fence-check {fence: string R, remaining: string O} [runner |
  missionId R]; mission-parked {parkReason: string R} [runner |
  missionId R]; runner-failed {error: string R} [runner | missionId R];
  wind-down {action: enum{sigterm,sigkill,skipped-unowned} R, reason:
  string O} [runner | missionId R, turnId O].
- driver-phase {phase: enum{provisioning,awaiting-approval,
  mission-running,grading,extracting,complete} R} [driver | cohortId R,
  repetitionIndex O].
- arming-started {} / stream-rotated {previousPath: string R} /
  arming-complete {} [arming | —].
- turn-started {} / session-established {sessionId: string R} /
  activity {note: string O} / repair-attempted {} / repair-outcome
  {outcome: enum{usable,failed,not-attempted} R} / turn-ended {verdict:
  string R, usageSummary: string O} [adapter, host | ADAPTER emits with
  jobId R (a delegate round is a job; it has no turnId), HOST emits with
  missionId R + turnId R (a host turn is a mission turn; it has no
  jobId) — the identity rule is per EMITTER for shared events, stated
  here so no prose generalization overrides the table].
- suite-fixture {name: string R, verdict: enum{passed,failed} R,
  durationMs: int R, evidencePath: string O, attempt: int O}
  [suite | —].

### D-1c. Whose sequence it is

The WRITER is the calling component's process, never the helper's: the
wrappers pass the CALLER's (pid, pidStartedAt) explicitly, and `seq` is
CALLER-OWNED state — a counter in the shell component's own process (an
environment variable the wrapper increments) or in the python component's
module — starting at 1 and passed to the helper as an argument. A helper
spawned per event has its own pid and no continuity; recording it would
make every event writer #1 of a one-event stream. A NEW PROCESS IS A NEW
WRITER with a fresh counter, which is exactly right: writer identity is
(pid, pidStartedAt), and loss detection is per that identity.

### D-1b. Payload discipline

`ref` (optional) points at the artifact holding the full truth, as a path
RELATIVE TO THE CHECKOUT ROOT, no symlink escape, no `..`. The event carries
a one-line summary, never a payload. If the required fields alone would
exceed the size cap, the emitter truncates `summary` first, then drops
optional fields, and ALWAYS emits a syntactically complete minimal event —
an oversize situation degrades detail, never validity.

## D-2. Concurrency safety, stated precisely

Many processes write one file via POSIX `O_APPEND`, each event as ONE
`write()`, hard-capped at 4096 BYTES (truncation respects UTF-8
boundaries). The fine print, which is part of the contract:

- LEADING-NEWLINE FRAMING (round 2's torn-prefix finding): every write is
  `\n` + event, no trailing newline. A short write leaves a torn fragment
  that the NEXT writer's leading newline terminates into its own
  unparseable line — so a torn write can never make another writer's event
  unparseable. Readers skip empty and unparseable lines by contract.
- FIELD BOUNDS make the minimal event always fit: every id field carries a
  hard cap (component 16, event 40, ids 160 each, level 8), which bounds
  the no-summary minimal event far below 4096; `summary` gets whatever
  room remains and is dropped entirely before any required field is
  touched. An id that somehow exceeds its cap is itself truncated with a
  trailing `~` — visible, valid, never fatal.
- SHORT OR FAILED WRITES ARE NOT RETRIED. A retry could interleave with
  another writer mid-line. The event is simply lost, and that is
  acceptable BECAUSE the log is a witness (D-5): a writer that detects a
  short write or an error (ENOSPC included) records nothing further about
  it beyond its own stderr. A MID-stream loss shows as a `seq` gap; the
  loss of a writer's FINAL event is invisible by construction, which is
  one reason silence is never a verdict (D-5).
- Local filesystem only; NFS is out of contract.
- No locks, no daemon, no broker. A crashed writer loses at most its own
  last line; readers skip any unparseable line.
- Ordering is by timestamp plus per-writer `seq`; cross-process ordering
  within the same millisecond is not promised and nothing may depend on it.

## D-3. The emitter and its failure contract

The emitter is TWO artifacts with one absolute contract — no call may ever
fail its caller — and round 2 correctly placed the burden AT THE CALLER
BOUNDARY, not inside the helper:

- the SHELL function swallows everything by construction, including the
  failures that happen BEFORE any helper runs: missing python3, missing
  helper file, unwritable destination, argument errors. Its last token is
  `|| true` and its lookup steps are individually guarded, so a caller
  under `set -e` cannot be aborted by any emit path.
- the PYTHON function (used by the mission runner and the lease helper)
  catches BaseException around its entire body INCLUDING import, spawn,
  and argument construction — the wrapper is defined in the caller's own
  process, so there is no lookup that can fail outside the trap.

Fixtures prove the boundary, not just the destination: a chmod-000 events
file, a REMOVED helper file, and a PATH without python3 each leave a lease
claim, a dispatch, a reap, and a mission step with outcomes identical to a
run without the recorder.

The existing per-component stderr logs REMAIN; the stream is the structured
layer above them.

### D-3a. Committed events, and losers who say so

Verdict-shaped events are emitted AFTER their record write succeeds
(post-CAS), so the stream never asserts a verdict that never became state.
The OTHER side is equally required: a writer whose CAS was REFUSED emits
`verdict-refused` naming what it wanted to write and WHAT THE COMPARE
OBSERVED — which requires one small interface change, called out here as
part of this design: `record_cas` returns the observed status on refusal
(today it returns only an exit code, and a later re-read could not honestly
say what the atomic compare saw). Winner and loser events correlate by
`jobId` — NOT by adjacency, which a no-lock stream cannot promise and the
proof does not assert. That correlated pair is what makes a KI-29-class
race visible instead of a mystery. (The first draft assigned
stale-claim-epoch to the reaper; it is the LEASE HELPER that sweeps, and
the registry assigns each event to its true writer.)

## D-4. Rotation has exactly one owner

Rotation makes NO semantic claim at all — round 3 removed the last one.
Events carry their own generation (census-verdict carries it; every event
carries its writer identity and timestamp), so a stream file is just a
container, and one file spanning several supervision generations is
perfectly fine. What remains is mechanics, stated exactly:

- WRITERS KEEP NO STANDING FDS. The emitter opens, appends one framed
  write, closes, per event. A writer that opened just before a rename
  lands its ONE in-flight event in the renamed old file — accepted and
  stated: the rotation boundary is approximate by up to one in-flight
  event per writer, which costs nothing because the boundary carries no
  meaning. Writers create the path if missing (O_APPEND|O_CREAT), so
  nobody needs to be "first" and arming's stream-rotated event makes no
  first-line claim.
- ROTATION HAPPENS ONLY in the public arming path that establishes or
  takes over supervision ownership — a JOIN rotates nothing, and the
  owner's internal component restarts (launch_set) rotate nothing either:
  they are not rotation points precisely because rotation means nothing.
  Rotation exists so files do not grow forever, and that is its whole job.
- LIVE VIEW IS `tail -F`, by name, everywhere this design mentions
  following the stream — following an inode across a rename defeats the
  purpose (round 3 caught a leftover `-f` in D-6; the contract is -F).

Retention, concretely and with an owner: rotated files move to
`artifacts/agents/events-archive/events-<UTC compact timestamp,
YYYYMMDDTHHMMSSZ>-<rotating pid>.jsonl` — the pid makes same-second
rotations collision-free, and if the name somehow exists the rotator
appends `-2`, `-3`, … rather than overwrite or refuse. evidence-gc, on
each run, copies archive files to the durable evidence root under the SAME
per-checkout namespace the chain-evidence mirror already writes to (same
root configuration key, same checkout-identity derivation — this design
adds an `events/` subdirectory there and invents no new fingerprint), and
deletes a local copy only when BOTH conditions hold: a verified durable
copy exists AND the filename age is at least 14 days — copy-then-keep is
the norm, not copy-then-delete (copy failure = keep local, retry next
pass; a young file is copied early but retained locally until it ages).
Age is measured as: the UTC timestamp PARSED FROM THE FILENAME, against
the gc run's current UTC clock — no mtime, no second clock. The CURRENT stream is
never eligible for either. The archive file IS its own manifest —
self-describing JSONL, named by rotation time.

## D-5. The log is a witness, never an authority

No machinery decision may read the event stream as input: verdicts come
from records, liveness from the kernel, custody from the lease. A missing
or damaged log degrades diagnosis, never correctness. Two corollaries the
first draft itself violated, kept here as permanent warnings:

- SILENCE IS NOT A VERDICT. A dead run is detected by status_command's
  kernel-and-record check (exit 13), never by event age — a healthy
  five-minute inference is silent, and a killed process cannot emit its own
  obituary. The log EXPLAINS a death after the authoritative check found
  it; the Proof section is written accordingly.
- THE BUNDLE'S CONTENTS COME FROM THE TREE, NOT THE LOG. The collector
  (D-6) walks the ARTIFACT TREE as ground truth and ships it; event `ref`s
  are an index into the bundle, not the selection rule. A torn event can
  therefore never remove evidence from a bundle.

## D-6. Live view and the cross-machine bundle

- LIVE: `tail -F artifacts/agents/events.jsonl` is the human's window. The
  status surface described in adapter-streaming D-5 is a FORMATTER over
  this stream plus the state files.
- BUNDLE: `scripts/agents/collect-observability.sh --out <archive>` walks a
  checkout (and, for a cohort, its targets, joined by `executionId`) and
  ships: every events.jsonl (current + rotated within retention), the state
  files, and the artifact tree per D-5's corollary — with a MANIFEST
  (paths, sizes, hashes) so absence is detectable, a missing-file entry
  recorded as a gap rather than a failure, and paths stored
  checkout-relative so the bundle is portable. Secrets: the bundle contains
  exactly what the artifacts contain; the standing project rule that
  secrets never enter commits, logs, or plans is the boundary, and the
  collector adds a redaction pass only if that rule is ever found violated
  — it does not silently half-promise one.
- The acceptance test is the human's scenario: the bundle alone, on another
  machine, reconstructs the 2026-08-09 runner-death timeline without
  guessing.

## D-7. What this does NOT claim to diagnose

Honesty about limits, from the critique:

- KI-25 (the agent-fixture git lock contention) needs GIT-COMMAND-BOUNDARY
  events inside the fixture harness (who ran which git command in which
  repo, when) to identify the two contending writers. D-8's verdict events
  will graph its RECURRENCE; root-causing it is targeted instrumentation
  to add when that fixture is next opened, and this design does not
  pretend otherwise.
- Live "usage so far" for an in-flight turn is out of scope: adapters
  meter usage at turn end, and the status surface shows completed-round
  usage only (adapter-streaming D-5 is revised to say so).

## D-8. The validation suite is a subject too

The suite has ONE temp root and ONE exit trap, not per-fixture sandboxes,
so the first draft's assumptions are replaced with the suite's real
boundaries:

- STRUCTURED FIXTURE VERDICTS, scoped to what exists: the suite's named
  helper boundaries (run_agent_fixture and the section echoes) each emit a
  `suite-fixture` event (component: suite) with name, verdict, duration,
  and attempt number for retried fixtures. Inline checks BETWEEN named
  fixtures attribute to the enclosing section name. The event count per
  run is therefore the helper-invocation count, a number the suite can
  assert against itself.
- PRESERVE FAILURE EVIDENCE, scoped likewise: on failure the suite copies
  THE FAILING FIXTURE'S subtree when the helper knows it (run_agent_fixture
  does), and otherwise the smallest enclosing directory it can name, to
  `artifacts/agents/suite-failures/<timestamp>-<name>/`, prints the path,
  and the failure event carries it. The whole-root copy is the explicit
  fallback when attribution is impossible, size be damned — evidence beats
  disk.

## Carried critique for the later legs (round 2)

- BUNDLE (D-6): FR2-008 — the collector needs an authoritative way to
  locate cohort targets (they live under METASYSTEM_TRIALS_ROOT, outside
  the repository; discovery must come from the cohort record and driver
  state, never from the witness stream) and a precise definition of which
  trees ship. FR2-009 — the disclosure boundary is NOT settled by the
  standing secrets rule; prompts, transcripts, raw provider output, and
  generated runtime configuration need an explicit export stance BEFORE the
  first bundle leaves a machine, not after the first violation.
- SUITE (D-8): FR2-015 — the suite's real boundaries are three
  delegate_process_section gates and one run_agent_fixture block; most
  inline checks have no fixture identity to attribute to. D-8's shape holds
  only after the suite grows genuine per-fixture boundaries, which is part
  of that leg's work, not an assumption it may import.

## Proof

- Concurrent writers: N processes appending in a tight loop produce a
  stream in which every writer's every event parses (leading-newline
  framing); per-writer `seq` is gapless when no write fails; a forced
  short-write case corrupts at most the victim's own event, the NEXT
  writer's event still parses, and the caller outcome is unchanged.
- Emitter harmlessness: with events.jsonl chmod 000 (and again with the
  directory read-only), a lease claim, a dispatch, a reap, and a mission
  step all succeed exactly as without the recorder.
- Narrative reconstruction: a scripted mission (fake adapter) produces a
  stream from which a checker reconstructs the exact expected phase
  sequence — provision, arm, claim, turns, verdicts, park — with no access
  to anything but the log.
- The 2026-08-09 regression, replayed: a runner killed mid-mission is
  detected by STATUS (exit 13, the authority), and the stream then EXPLAINS
  it: the checker asserts the events PRESENT (runner-started, turn-launched,
  turn-result) tell the story up to the kill, and asserts NOTHING about how
  the record ends — a killed writer's final loss is invisible by
  construction (D-2), seq cannot expose a missing last event, and any
  checker that inferred death from the stream would make silence
  authoritative again. Detection is status's job alone.
- Verdict-race visibility: the REAL race shape, staged — a dispatcher
  records its timeout verdict (post-CAS, job-verdict) and the standing
  reaper's subsequent CAS on the now-terminal record is refused and emits
  verdict-refused carrying the observed terminal status. The checker
  asserts BOTH events exist correlated by jobId, in ANY order and at any
  distance — a staged double-reap cannot produce this pair at all (the
  per-job lifecycle lock serializes reapers, and the second simply reads a
  terminal record), so the fixture stages the dispatcher-vs-reaper shape
  that KI-29 actually was.
- Rotation: a stream rotated at arming leaves the old file complete and
  parseable, and a stream-rotated event EXISTS in the new file — at any
  position, since another writer may legally append first (the boundary
  makes no ordering claim); a follower using `tail -F` picks up the new
  file.
- Bundle sufficiency and manifest: collect on a finished fixture cohort,
  move the archive to a scratch directory, run the narrative checker there;
  delete one referenced artifact before collecting and the manifest records
  the gap without failing the bundle.
- Retention mechanics: a same-second double rotation yields -2 suffixing,
  never overwrite; a young archive with a verified durable copy is
  RETAINED locally; an old archive without a verified copy is retained
  and retried; only old-AND-copied is deleted; age comes from the
  filename (a touched mtime changes nothing).
- Non-authority: a run with the event stream deleted mid-mission completes
  with identical verdicts and artifacts.
- Suite half: a deliberately failed fixture leaves a preserved evidence
  tree at the path its failure event names; the per-fixture verdict events
  for a full run match the helper-invocation count the suite asserts.
