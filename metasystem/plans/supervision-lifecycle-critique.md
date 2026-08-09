# Independent critique: supervision-lifecycle.md

Requested by the human on 2026-08-09 after the sol chain exhausted
(13 -> 10 -> 13 material, round 3 unfolded). This round is by a
different model (Claude Fable 5), in a fresh context, with every code
claim re-verified against the working tree — file:line citations below
are to the current checkout, not to the plan's paraphrase of it.

## Verdict

The diagnosis is right, the reframe is right, and round 3 was right:
all thirteen R3 findings CONFIRM against the code, including the
worst one. The chain did not diverge because the critic was flailing;
it diverged because the design keeps patching inside three wrong
frames — identity DERIVED from repository properties, bounds scoped
PER CHECKOUT, and teardown performed BY THE DYING PROCESS. All three
have known exits, given below. The document is one rewrite away from
implementable, but D-1 as written must not be implemented at all.

## What the design gets right (re-verified)

- The causal chain is fact. `run_owner`'s loop is `while true` with no
  exit path (arm-supervision.sh:423-437; the three `break`s at 426,
  427, 430 leave only the inner `for`), and no repo-existence check
  exists anywhere in the function. The only stops are `--shutdown`
  and signals.
- Counting DEATHS, not launch failures, is the correct breaker input.
  The code confirms the incident shape: a component that heartbeats
  once passes `wait_for_first_heartbeat` (arm-supervision.sh:417-418)
  and its death is only ever seen as staleness on a later tick.
- Dropping the dead-man's switch is correct. A mechanism that can kill
  healthy supervision on a missed renewal is strictly worse than the
  leak.
- Indeterminacy-prefers-the-leak is the correct default, and the code
  already embodies it elsewhere (identity_alive treats unobservable
  argv as live, arm-supervision.sh:173-178).
- Teardown-not-return and kill-only-by-held-identity are the correct
  exit semantics (see SLC-F-001: the current code violates them).

## Round 3, adjudicated

- SLC-R3-001 CONFIRMED, critical, with proof. `fingerprint()`
  (process-census.py:908-943) hashes the supervision SCRIPTS
  (arm-supervision.sh, dispatch.sh, process-census.py, the adapters,
  watch-background-jobs.sh), the runtime signatures, and six config
  values. The supervised repo contributes exactly one thing: its
  resolved path string (`repositoryScope`). Editing any supervision
  script or config value in a live checkout changes the fingerprint,
  and E1 as written then terminates healthy supervision — the precise
  failure this design exists to prevent. The fingerprint's actual job
  is the opposite one: verify_armed compares it to force re-arm when
  supervision CODE goes stale (arm-supervision.sh:466-471). Reusing
  it as checkout identity inverts its purpose.
- SLC-R3-002 CONFIRMED and UNDERSTATED. Because only the path string
  enters the hash, a replacement repository at the same path produces
  an IDENTICAL fingerprint. The fingerprint contributes nothing to
  replacement detection; the inode carries the whole burden. (On
  APFS inode numbers are 64-bit and not reused, so the inode half is
  stronger than round 3 allowed — but see the fix, which removes
  derived identity entirely.)
- SLC-R3-003 CONFIRMED. The code's takeover gate requires exact
  pid+start identity proven DEAD (arm-supervision.sh:619-628). A
  hung, stopped, or unschedulable owner is alive under that test, so
  the plan's stated E2 scenario ("presumed dead ... resumed") cannot
  arise through the legal path. The case E2 actually insures against
  is a FALSE death observation (the census helper erring under load —
  plausible at load 134). Either name that honestly or drop E2; the
  fix below gives its protection for free.
- SLC-R3-004 CONFIRMED, critical. "Stale" in the code includes
  state-file read failure (arm-supervision.sh:426). D-2 counts every
  stale cycle; the Proof requires that chmod-000 state keeps the
  owner alive. As written, five unreadable cycles trip the breaker
  and the owner exits — the Proof and the breaker contradict. The
  breaker needs the same three-way split as E1: DEAD/STALE increment,
  UNKNOWN neither increments nor resets.
- SLC-R3-005 CONFIRMED. Counting per relaunch attempt under backoff
  gives up after ~25 minutes; counting per base-interval observation
  gives up after ~5. A 5x spread in the design's central constant is
  not an implementation detail. Decide: the counter counts BREAKER
  OBSERVATIONS at base interval; backoff gates relaunches only.
- SLC-R3-006 CONFIRMED, critical, and the fix is already in the
  code's own idiom. The incident population was untagged: 149
  process-census.py helpers (spawned by the watcher each pass,
  watch-background-jobs.sh:175-215) and 54 dispatch `__lock-owner`
  helpers (dispatch.sh:2602-2609). None carry owner tags. But every
  component is launched via setsid (arm-supervision.sh:251-263), so
  each is a process-group leader and its helpers inherit its pgid —
  which is exactly the boundary stop_identity already kills by
  (`kill -TERM -- -pid`, arm-supervision.sh:230). Count MEMBERS OF
  THE RECORDED COMPONENT PROCESS GROUPS, not tagged processes, and
  the ceiling bounds what actually exploded.
- SLC-R3-007 CONFIRMED. "more than 12" admits a thirteenth. Trivial;
  fix the comparison or the Proof.
- SLC-R3-008 CONFIRMED and UNDERSTATED. The schema lacks component
  pid/pidStartedAt (tags alone cannot drive identity-verified kills)
  and lacks a reason field for the terminal records D-2 and D-5
  promise. Worse: launch_set mints NEW component tags every
  generation (arm-supervision.sh:384-388), so the single "armed"
  record is stale after the first self-heal. Either the owner appends
  a record per launch_set, or the janitor's contract is: kill the
  recorded owner by identity, then sweep survivors by owner-tag
  PREFIX (component tags embed the owner tag), which is what the
  manual cleanup actually did.
- SLC-R3-009 CONFIRMED. The reduction rule is cheap and must be
  written: ownerTag is unique per arming (repo+timestamp+pid,
  arm-supervision.sh:562); latest event per ownerTag wins; "armed"
  with no later terminal event is a live claim. Joins must not
  append (they mint no owner). Janitor runs may compact.
- SLC-R3-010 CONFIRMED, narrowed. Framing can be inherited from the
  flight recorder as the plan says; the genuinely missing rule is
  append failure: if the registry cannot be written, ARMING MUST
  FAIL. An unregistered owner is invisible to the only machine-wide
  view; proceeding unregistered recreates the incident's blindness.
- SLC-R3-011 CONFIRMED. The Proof promises a machine-wide number and
  no mechanism computes one. See fix (b): owners are born in exactly
  one place, so bound them there.
- SLC-R3-012 CONFIRMED. D-5's reporting lives in the driver's own
  cleanup path — the path that does not run when the driver is
  killed, which is the case under discussion. See fix (c).
- SLC-R3-013 CONFIRMED. Components launch before the state file
  publishes their identity (launch at arm-supervision.sh:389 and 395,
  publish at 399-413). An owner dying in that window leaves a
  component no record names. Publish intent (the minted tags) BEFORE
  the first launch_detached, and the tag-prefix sweep above covers
  the rest.

## New findings, this round

- SLC-F-001 (high). The existing exit path already violates D-1's
  own rule. `cleanup_owner` (trap on EXIT, arm-supervision.sh:358-366)
  calls stop_recorded_components, which kills whatever the CURRENT
  state file names (317-324) with no check that this owner wrote it.
  Any supersession followed by a late TERM to the old owner kills the
  successor's watcher and reaper. D-1 states the right rule
  ("never components named by a state file it no longer owns") but
  does not say the shipped trap breaks it, so an implementer who only
  adds exit conditions ships the successor-killing bug through the
  very trap those exits fire. Replacing the trap's kill with
  teardown-by-held-tags is a required implementation item, not an
  aspiration.
- SLC-F-002 (low). The Proof still references "the intent lease"
  (Live-supervision bullet) though D-3 deleted it. Stale text;
  it will confuse the implementer about what D-3 decided.
- SLC-F-003 (medium). The registry has three writer classes (armer
  appends "armed", owner appends "exited", janitor appends "reaped")
  and the plan specifies concurrency for none of them. One rule
  suffices — single O_APPEND write per framed line, no read-modify-
  write — but it must be stated in the registry contract, or someone
  will helpfully rewrite the file.

## The three "structural" problems are not structural

### (a) Identity: assign it, don't derive it

Three rounds tried to DERIVE checkout identity from repository
properties (path, fingerprint, inode) and each candidate is mutable,
reusable, or — per R3-001/002 — measures the wrong thing entirely.
The exit is to stop deriving: the owner already WRITES a file inside
the checkout that names it exactly — state.json, carrying its own
pid, pidStartedAt, and instanceTag (arm-supervision.sh:399-413).
That file is a self-issued identity token. Replace E1 and E2 with one
check the owner runs every base-interval cycle:

- state.json readable and its owner stanza names ME (pid+start+tag):
  purpose intact, I am current. Continue.
- readable but names ANOTHER owner: I have been superseded — this is
  E2, obtained for free, covering exactly the false-death-observation
  case. Tear down MY tags only; exit.
- definitively absent (ENOENT on the file or any parent, including
  the checkout root): purpose gone or deliberately revoked — this is
  E1 and subsumed E3. Tear down; exit.
- unreadable for any other reason: UNKNOWN. Log, continue.

No code edit changes this check (kills R3-001). A replacement
repository at the same path cannot contain a state.json naming this
owner's exact identity (kills R3-002 harder than the inode does).
Establishment-is-not-revocation is preserved: the check arms only
after this owner first publishes. Two consequences to accept and
write down: deleting state.json by hand on a live checkout now stops
supervision (that is revocation working, and the next arm
re-establishes), and a wholesale copy of a dead sandbox including
artifacts/ would be adopted at the recorded path (rare, and arguably
the owner doing its job). The fingerprint keeps its real job —
forcing re-arm on supervision code change — and leaves the identity
business entirely.

### (b) The machine-wide bound lives at arming

Owners are minted in exactly one place: arm_repository, holding the
lock. That is the chokepoint the incident needed. Before launching an
owner, reduce the registry (rule in R3-009) and refuse to arm when
live-armed owners >= K machine-wide, printing the claim list and
pointing at reap-orphans.sh. The incident's sixteen owners stop at K;
nothing continuous needs to run; the janitor at suite start keeps the
reduced set honest between refusals. Pick K in D-6 (8 is generous —
this machine has never legitimately needed more than a handful). The
per-checkout process ceiling then honestly stays per-checkout,
counted by pgid membership (R3-006), and the load-regression Proof
becomes assertable: at most K owners x ceiling processes, both
numbers written in D-6.

### (c) Durable teardown is recovery, not a promise

No design can make a killed process perform its own teardown; every
scheme that promises otherwise smuggles the assumption back in.
Reframe as write-ahead custody: provisioners record intent durably
BEFORE creating supervision, and recovery reduces the record on next
entry. Concretely: when a cohort driver or fixture arms a managed
checkout, it appends a custody record to the same registry — this
checkout is managed by custodian (pid, pidStartedAt), teardown
expected. The janitor then gains its missing rule for LIVE checkouts:
custodian recorded and provably dead -> reap that checkout's
supervision and report it (closes R3-012 without resurrecting the
dead-man's switch — this acts on proven death of a recorded identity,
never on a missed renewal). Checkouts armed by a human record no
custodian and are never auto-reaped, which is exactly D-3's "the
human's call" boundary, now enforced instead of assumed.

## What to do with the document

1. Rewrite D-1 around the self-issued identity above. Delete
   fingerprint and inode from the identity story. E2 folds into the
   names-another-owner branch.
2. Extract the registry into its own short contract doc — schema
   (add component pid/start and a reason field, R3-008), reduction
   rule (R3-009), append-failure-fails-arming (R3-010), writer rule
   (SLC-F-003), custody records (c). Findings R3-008..012 all land
   there; that is why the chain stopped converging — a load-bearing
   component was being specified in the margins of another design.
3. In D-2: define the breaker's three-way observation (R3-004), tie
   the counter to base-interval observations (R3-005), count pgid
   members (R3-006), fix the off-by-one (R3-007), add the arming-time
   machine-wide bound (b) with K in D-6.
4. Make the trap replacement an explicit implementation item
   (SLC-F-001).
5. Proof list: drop the intent-lease line (SLC-F-002); add the
   anti-R3-001 regression — edit a supervision script in a live
   armed checkout, and the owner MUST survive it; add the custody
   proof — kill a cohort driver mid-repetition, run the janitor, and
   the managed checkout's supervision is reaped and reported.
6. Implementation order: registry contract, then D-1' exits with
   held-tag teardown, then the breaker, then the janitor, then D-5
   loudness. The operational workaround (kill owners before
   components) stands throughout.

Nothing in round 3 was noise. Fold all thirteen; the eight criticals
are five distinct defects, and every one has a fix above that does
not reopen the others.
