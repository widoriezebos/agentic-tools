# Proof runs: priced up front, alive throughout, paid once (design, round 1 folded)

Goal: proof-run-cost-and-liveness (Wido's order 2026-08-27; his
condition recorded: mechanical prevention is the acceptance bar).
Critique: FAILSAFE round 3. Round 1
(design-critic-20260827t170927z-e2fc): 11 material (5 critical),
every fold marked [PCL-0NN]. Builds ON m1's witness gate, section
selector, and controller-descendant verb.

## Root cause (verified): witness arms only on clean trees, so
pre-landing nested validations each re-pay the full race gate; and
proof runs have no progress contract — stall and progress are
indistinguishable. Live evidence: the 2026-08-27 adopt run paid the
unarmed gate 5+ times over 4+ hours, including inside
expected-refusal legs.

## Part 0 (new, from the live run): cheap preconditions first

validate-metasystem hoists its cheap static refusals — placeholder
checks, conf validation, routed-asset presence — AHEAD of the go
gate. An expected-refusal leg (adopt's placeholder proofs) then
costs seconds, not a full gate. This is a leg-ordering change with
no proof-semantics change: everything still runs on a passing
suite, only refusal-time moves.

## Part 1: the staged-batch witness, round-2 folds

- CLOSURE INVERTED [PCL-R2-001]: the digest covers the ENTIRE
  metasystem tree EXCEPT declared non-inputs (artifacts/, bin/,
  .git, and nothing else initially). Over-inclusion only costs
  reuse misses and is always sound; the round-1 under-inclusion
  (tests reading plans/ and skills/ files) is impossible by
  construction. The canary inverts: mutate a file under artifacts/
  → still reuses; mutate ANY tracked byte → full gate.
- MANIFEST SPEC [PCL-R2-002]: one record per entry: NUL-framed
  relative path (raw bytes), entry kind byte (f/l/d), executable
  bit only as mode, symlink target raw bytes (entry never
  followed), sha256 of file bytes; records sorted by raw path
  bytes; each record length-framed; witness digest = sha256 over
  the concatenated records. Stated in the code as the normative
  comment.
- AUTHORITY REFRAMED [PCL-R2-003]: the ancestry check blocks
  OUTSIDERS. An insider under the live controller who "waits and
  satisfies the check" holds the authority to run the real gate
  and mint a real proof — the witness grants no authority an
  insider lacks. The threat model is therefore: outsiders (blocked
  by controller-descendant), byte drift (blocked by the digest),
  mid-run mutation (blocked by pre/post equality). Stated as the
  design's explicit security argument for the critic to attack.
- CONSUMER TOCTOU [PCL-R2-004]: check-build-recheck — the consumer
  re-digests after its skip-path build/copy completes and voids
  reuse (falls back to the full gate) on any mismatch. A racing
  writer can waste time, never fake a proof.

## Part 0 narrowed [PCL-R2-010]

Only the STATIC textual placeholder scan (pure grep, no engine
involvement) hoists ahead of the go gate. Engine-owned refusals
(conf validation, audit grammar) stay post-gate where the
prospective engine owns them. The adopt expected-refusal legs fail
on the static scan in seconds; no refusal changes owner or
grammar.

## Part 2: the progress contract, round-2 folds

- HEARTBEAT = OUTPUT GROWTH [PCL-R2-005]: liveness is not section
  boundaries. Every section already writes through tee/redirect;
  the watchdog watches total output-byte growth across the run's
  log files (paths listed in the heartbeat header). The race gate
  prints package completions every few minutes; a 30-minute
  windows kills nothing healthy. Section start/end lines remain in
  the JSONL for structure assertions, not liveness.
- KILL VIA SUPERVISION [PCL-R2-006]: the watchdog's sequence
  reuses the machinery that owns Setsid components:
  arm-supervision --shutdown for registered components (graceful,
  existing), then CONT/TERM/KILL on the suite's process group,
  then the existing census sweep asserts nothing scoped to the
  suite root survives. No new signal choreography.
- WATCHDOG LIFECYCLE [PCL-R2-007]: launcher setpgid's the suite,
  records suite pid+start with the watchdog at spawn; watchdog
  re-verifies pid+start before ANY kill (no recycling kills). On
  normal exit the launcher writes a done-file and waits for the
  watchdog (which exits 0 on seeing it); the launcher owns the
  reap. Silence after the final section cannot be misread: the
  done-file precedes the last log close.
- EVIDENCE BEFORE SIGKILL [PCL-R2-008]: the watchdog preserves
  evidence ITSELF before force-kill — the heartbeat header names
  the run's tmp and log paths; the watchdog copies them into the
  existing suite-failures shape, then kills. The in-suite trap
  remains for graceful paths; the external copy covers the path
  where no trap can run.
- STRUCTURE ASSERTION RELAXED [PCL-R2-009]: the JSONL assertion
  is: every selector-named section has >=1 start and a matching
  end, ends follow starts, and only sections in the DECLARED
  twice-consulted set appear twice. Exact-global-order is not
  claimed; per-section well-formedness is.
- BANNER SIMPLIFIED [PCL-R2-011]: line two is dropped. The banner
  is: witness state + duration class + the heartbeat/log paths.
  Family-skip decisions print where they are made, as they already
  do.

## Part 3, non-goals, and the structural rule: unchanged from
round 1 (runner surfaces relay the banner; validation asserts
every section heartbeats — RED otherwise; no daemons beyond the
suite-lifetime watchdog; no estimates database).

## Acceptance (mechanical; delta from round 1)

- Closure canary inverted (artifacts/ mutation reuses; any tracked
  byte voids).
- Consumer recheck: mutate the tree between check and use → fall
  back, never reuse.
- Setsid component survives group kill → census sweep proves the
  supervision shutdown got it.
- Watchdog pid-recycling guard: wrong pid+start → no kill.
- SIGKILL path: evidence present in suite-failures without any
  in-suite trap running.
- Long quiet-but-printing section (race gate) never killed;
  SIGSTOP'd suite killed within window.
- Expected-refusal adopt legs fail on the static scan in seconds.

## ROUND 3 (failsafe): seven disputes for Wido

Full texts: artifacts/agents/design-critic-20260827t194017z-6ca9.
Coordinator recommendations:
1+3. [R3-001 freeze semantics, R3-003 A-B-A race] ONE fix: the
   FROZEN EXPORT — arming exports the manifest's bytes to a
   private tmp tree and the gate runs THERE, never against the
   live tree. Freezing, execution locus, and A-B-A all resolve at
   once; nested copy-based validations receive the same export.
2. [R3-002 insider fabrication] Accept the single-user trust
   model as an explicit recorded risk: a same-user process can
   already replace bin/metasystem or the gate script itself;
   witness state is not a new privilege boundary. OS-level user
   separation is out of scope.
4. [R3-004 kill inventory] The watchdog's kill inventory = union
   of (a) supervision registry components under the suite root,
   (b) checkout-execution-guard member list, (c) the suite process
   group. All three inventories exist today.
5. [R3-005 Part 0 contradiction] Editorial: ONLY the static grep
   placeholder scan hoists; the earlier paragraph is corrected.
6. [R3-006 printing-but-wedged] Second liveness layer: a section
   exceeding a cap-multiple of its declared duration class is
   stalled even while printing (fixture-budget caps exist; K=3).
7. [R3-007 evidence copy blocks kill] Bounded: copy under a
   timeout and size cap, partial evidence + a loud note, then
   kill unconditionally.
