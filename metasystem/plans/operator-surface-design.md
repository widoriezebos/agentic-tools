# The Operator Surface — the claw-back design (v3, 2026-08-28, after critique round 2)

Working Mode: design

Owner: m1 coordinator, in direct discussion with Wido; codex as
co-designer from round 2 onward (Wido's order: "you and codex
design your way out"). ROUND 1: 20 material + 3 not — all folded
into v2 (dispositions below). ROUND 2: 15 material (14 shape) —
codex then DRAFTED the resolutions to its own findings under the
four standing rulings, and the coordinator adjudicated the draft
ACCEPTED IN FULL: the authoritative resolution text lives in
plans/operator-surface-v3-resolutions.md, one section per finding,
and SUPERSEDES this document's corresponding v2 sections (notably:
the bootstrap epochs and retrospective seal; terminal-rooted
human-authority proof; the rebase-before-acceptance landing
protocol with CAS push; engine-of-record provenance; the
stop-capability model; recovery bundles with rehearsal; the
D121-respecting operator-owned Ring 3; generation-bound success
evidence; receipt-in-candidate; the 25-slice bootstrap-honest
partition; the DRAFT→OBSERVE→LIMITED→ENFORCED rule lifecycle; the
four-field budget tuple with reserved-job-minutes; the eight-name
surface routing table; evidence-based dual-run retirement; the
total health/watch boundary table).

WIDO'S STANDING RULINGS: A — human authority by process ancestry,
never a string; B — six daily verbs + mission and run lifecycle
families; C — the authority boundary is his INTERACTIVE TERMINAL,
no agent-signature process between invoker and terminal; D — the
ch.15 bootstrap exception for S0a..S4 under his recorded chat
word, retroactively re-judged by the first battery after S4.

RULINGS E-H (Wido, on the v3 open items): E — protected custody =
git refs for acceptance/provenance + the durable evidence root for
recovery bundles; F — D121 STANDS, restated in his words: the
metasystem starts its own supervision when it initializes (`up` at
session start is the primary recovery), and an OS-level scheduler
entry is strictly OPTIONAL, operator-owned, recovery-only; G —
phone alerts use the delivery-receipt + agent-free-terminal
acknowledgment shape (`health acknowledge-alert`), not session
push; H — the four-field budget tuple is BLESSED (elapsed,
attempts, reserved delegate-minutes, concurrent jobs — all
mandatory on claim; wall-time alone missed sixteen parallel passes
under one clock).

Round budget: 3; failsafe 3; ROUND 3 (the failsafe) attacks v3 =
this document + the resolutions file together, under Rulings A-H.

## THE GOAL

This design serves the vision recorded in docs/paper — a system
that governs the production of software through revisable intent,
evidence over trust, durable records, enforced rules that refuse,
budgets that stop, and human authority at value decisions — and it
restores the one property without which none of that vision can
operate: A SURFACE ITS OWN OPERATOR CAN HOLD. The breakdown was
not a failure of the vision; it was unmetered mechanism accreting
past operability. The claw-back keeps everything that demonstrably
works (the goal ledger, the landing gates, the census and reaper
machinery, the metrics family, the custody internals on the wip
branch), deletes or internalizes what made the system inoperable,
and re-earns trust in the machinery slice by slice under the
current controls and Wido's word.

THE BOOTSTRAP POSTURE (binding until revoked): the current state
cannot be relied on to run its own rescue. Until each protection
demonstrably works again, this work uses only the minimal trusted
core — direct delegation (plain codex execution, coordinator-
monitored), the existing landing gate, the coordinator's own
verification by execution, and Wido's authenticated acceptance. No
slice depends on machinery a later slice builds (round-1 findings
02/07/08 caught three violations; the slice plan below is
re-partitioned to honor this for real).

The breakdown evidence base, the two laws (NOTHING IS PROSE; ALL
COMPLEXITY SHIELDED BEHIND MINIMAL EASY VERBS), and the Verb
Contract (MANDATORY-IF-IMPORTANT, IDEMPOTENT, FORGIVING,
SELF-VERIFYING, SELF-EVIDENT) carry forward from v1 unchanged in
meaning; the mechanisms below are the corrected execution.

## HUMAN AUTHORITY IS PROCESS-PROVEN (RULING A; folds R1-01, R1-10)

`--by wido` was a string any agent could type: the override
authority and the appeal route were self-granting, and Wido's
slice acceptance was unrecorded prose. Corrected:

- A HUMAN-AUTHORITY INVOCATION is valid only when the invoking
  process provably descends from a session the human announced —
  the generalization of the existing `proc acknowledge` pattern,
  which already classifies the real parent and refuses agent
  invokers. Agent ancestry → refusal by name. No passwords, no
  nonce ceremony (Wido's choice): the process tree is the proof.
- ACCEPTANCE IS A RECORD, NOT A CHAT LINE: Wido accepts a
  safeguard slice via a human-authority verb that writes an
  acceptance record binding HIS identity proof to the EXACT
  candidate tree hash. `land` refuses a safeguard-slice landing
  without a standing acceptance for the exact tree it is about to
  commit. The chat remains where discussion happens; the record is
  what the machine obeys.
- THE JUDGE IS NOT THE CANDIDATE (R1-01's second half, ch. 15):
  the landing gate for safeguard slices runs the ENGINE OF RECORD
  — the binary built from the base commit, not from the candidate
  — and battery controllers come from the base. The wip-branch
  snapshot pattern is the named, rehearsed recovery route: code,
  records, and authority state restorable from the last accepted
  landing.

## THE SURFACE (RULING B; folds R1-17, R1-18)

SIX DAILY VERBS — `up`, `health`, `goal`, `delegate`, `watch`,
`land` — PLUS TWO LIFECYCLE FAMILIES that are operator-facing but
not daily and are documented on their own page: `mission`
(unattended missions: start, status, answer, taint resolution) and
`run` (supervised arbitrary long commands — the only lawful home
for batteries and non-agent runs; it stays because deleting it
would orphan supervised runs, R1-18). Everything else the binary
exposes is INTERNAL: reachable by plumbing and fixtures, absent
from top-level help and operator docs.

## THE SIX VERBS, corrected

1. `metasystem up`
   Arms everything idempotently: supervision owner, watcher,
   steward runner, session announcement, lease — with dead-owner
   takeover (absorbing the arming-dead-owner-takeover backlog
   item). Session start runs it via the hook. SECOND-SESSION LAW
   (R1-20): the lease has one holder; a second session's `up`
   returns the typed outcome `advisor` — armed for reading, no
   write authority, the holder named, a worktree suggested for
   parallel work. No displacement, no guessing.

2. `metasystem health`
   The roles-alive verdict, typed per role: alive | dead |
   unknown. TOTALITY (R1-14): every state has an owner and a
   behavior — dead → the named remedy verb + the owner (tick
   auto-restart / coordinator / Wido by alert); unknown → held one
   interval, then escalates to Wido if it persists (unknown is
   never silently tolerated); aggregate exit codes 0 healthy,
   1 unhealthy, 2 unknown-present. Repeated failure of one role
   within a window escalates past auto-restart to Wido.
   CHECKS ARE BOOTSTRAP-HONEST (R1-02): at slice-one time health
   checks what EXISTS — the prose-prefix appetite until slice two
   replaces it, the current arming scripts as remedies until `up`
   exists — and each later slice updates the check and the remedy
   it owns. The check list: steward runner; supervision owner;
   repo watcher; census freshness; narrator freshness; session
   main; hook freshness (R1-03: the hook writes an invocation
   stamp; health flags a stale or silently-failing hook — the
   common failure domain gets its own check); claimed-goal
   appetite; dead-process non-terminal jobs; snapshot ages.

3. `metasystem goal …`
   a. STRUCTURED APPETITE (R1-11, R1-12 lifecycle made total):
      `goal open` takes `--appetite` (mandatory for agent actors;
      open --claim keeps its atomicity because the appetite rides
      the same command); `goal set-appetite` covers existing
      goals; legacy claimed goals without the field are flagged
      unhealthy by health until set (grace, not wedge). Breach
      computes from claim age against the field and feeds the
      TYPED bands that already exist (BREACH-ESCALATE /
      BREACH-STOP, internal/goal/project.go) — the fixture proves
      an overdue structured claim drives unhealthy + heartbeat +
      notification on the next tick.
   b. THE BUDGET ACTUALLY STOPS (R1-04, ch. 11): BREACH-ESCALATE
      notifies and marks the chat line. BREACH-STOP is enforced:
      the engine PARKS the claim in a typed state carrying a
      typed stop reason, the tick winds down the goal's live
      delegate jobs through the existing cancel transactions, and
      the goal records its stop-state (what works, what does not,
      the open uncertainty — typed fields, prose only for the
      human summary). The coordinator switches items; only a
      human-authority act un-parks past a stop.
   c. RULE RECORDS ARE TYPED AND GOVERNED (R1-09, ch. 12): a
      small authoritative store (one engine-owned file) holds each
      enforced rule: id, scope, adoption evidence, must-stop and
      must-permit test references, refusal evidence shape, owner,
      review-by, withdrawal route, and TRIAL MODE — every new rule
      runs observe-only (logging would-refuse) for its trial
      before it may refuse. The 4h ceiling, claim-requires-
      appetite, and mandatory-fields enter as the first three
      records, in observe-only first like everything else.

4. `metasystem delegate`
   `--role <role> --brief <file> --goal <id|none-explicit>`; also
   `--follow-up <job> --brief <file>` and `--cancel <job>`
   (R1-13: corrections, cancellation, and chain accounting are
   lifecycle, not luxuries; closure stays internal and automatic).
   THE TYPED OUTCOMES STAY TYPED (R1-06, reversing v1's collapse —
   the critic caught v1 violating "nothing is prose"): the
   full outcome enum from the custody machine (won, in-progress,
   bound, replayed-<status>, reconciling, and the named refusals)
   is the verb's JSON output; the human-facing headline groups
   them (started / already running / refused) but the machine
   reads the enum, never a because-string. The operation identity
   derives from (goal, role, brief hash) with `--op` for explicit
   retry binding; the fingerprint law from the custody design
   applies. GOAL REVISION BINDING (R1-19): delegate records the
   goal's revision at launch; land refuses a candidate whose
   goal revision moved unless a fresh human acceptance names the
   new revision.

5. `metasystem watch --job <id>`
   Follow to terminal; on failure show the record, log tail, and
   remedy. TOTAL UNCERTAINTY BEHAVIOR (R1-14): a bounded default
   timeout; on a non-terminal record with a dead/indeterminate
   process it reports the zombie verdict instead of waiting
   forever; exit codes distinguish terminal-ok, terminal-failed,
   timeout, and zombie-suspected.

6. `metasystem land`
   `--message <file> --goal <id> --built-by <who>` plus a
   MANDATORY candidate selection: `--staged-only` or explicit
   pathspecs (R1-05 — sweeping the tree by default is how
   unrelated bytes get landed). ATOMIC BY OPERATION IDENTITY
   (R1-05): land journals an operation id with phases
   (gate → commit → push origin → push transport → receipt);
   a rerun completes remaining phases idempotently; a crash
   between phases is completed by the rerun, never repeated.
   The receipt's important fields are land flags (type, outcome
   mandatory; the rest defaulted harmlessly). Safeguard slices
   additionally require the standing human acceptance record for
   the exact tree (above).

## THE HEARTBEAT AND ALERTS, corrected (folds R1-03)

Three rings, no common silent failure:
- RING 1, every turn, in the chat: the turn display carries the
  one-line health verdict. The hook that renders it writes its own
  invocation stamp; health checks that stamp, so a dead or
  silently-failing hook is ITSELF a named unhealthy role — the
  line's absence is no longer the only signal.
- RING 2, every tick: the steward tick computes health, narrates,
  and alerts on unhealthy via the platform notifier. MUTUAL
  WATCHING lands with slice one using processes that exist today:
  the supervision watcher checks the steward runner's freshness
  and re-arms it; the tick checks the watcher; neither corpse can
  go unnoticed because its peer names it (R1-02: the tick cannot
  restart itself — the WATCHER holds that duty).
- RING 3, boot-level: a minimal system scheduler entry (launchd/
  cron) runs `up --if-down` hourly — the reboot/total-loss
  recovery the v1 ledger wrongly deleted (R1-03). It is the
  outermost ring and its absence is a health finding.
- ALERTS TO WIDO: desktop banner via the notifier now; the
  session pushes to his iPhone on unhealthy with alert-episode
  dedup (one push per episode, cleared on healthy); his next chat
  message is the acknowledgment; telegram/slack later through the
  same notify seam.

## THE REMOVAL LEDGER, corrected

As v1, with these reversals and corrections (R1-13/15/16/18/21):
- KEEP the receipt family (correction, stats, check, retro) — the
  retro learning loop consumes it; `land` absorbs only `add` for
  landings. The retro skill keeps its instruments.
- KEEP `proc acknowledge` as a HUMAN-AUTHORITY verb under Ruling A
  — the human-reserved census judgment stays human; health's
  untracked-process alert names it as the remedy.
- KEEP the `run` family (lifecycle surface) and `mission` (per
  Ruling B).
- KEEP the claim-launch typed outcome machine intact (R1-06).
- Path corrections: scripts/receipt.sh and
  scripts/watch-background-jobs.sh (not scripts/agents/...).
- Everything else in the v1 ledger stands: critique-round.sh
  DELETED into delegate; arming/dispatch/land scripts
  INTERNALIZED; the engine's other families INTERNAL.

## THE SLICES, re-partitioned (folds R1-02, R1-07, R1-08)

Bootstrap-ordered: every slice depends only on what exists when it
builds. Each ≤4h with contents listed so the ceiling is checkable;
each judged by the engine of record, accepted by Wido's recorded
act, landed alone. A slice that exhausts 4h stops, records its
typed stop-state, and raises.

- S0a — TREE ISOLATION (≤2h): the dirty custody tree moves cleanly
  to the wip branch (which already snapshots it); the primary tree
  returns to landed main. Landings stop needing stash
  choreography; custody work continues from the branch when its
  turn comes. (R1-08's missing preparation slice.)
- S0b — WIP TRIAGE (≤4h, document only): keep/adapt/delete over
  the custody branch with reasons, reviewed and accepted by Wido
  BEFORE any consumer builds (R1-07). Health's liveness checks and
  delegate's internals both consume its verdicts.
- S1a — HEALTH CORE (≤4h): the verb (checks that exist today,
  including hook freshness), tick integration, narration, desktop
  alert, aggregate exit codes, unknown-escalation. Fixtures kill
  each role.
- S1b — RINGS (≤4h): the chat line via the hook + invocation
  stamp; mutual watching (watcher re-arms steward, tick checks
  watcher); the boot-level `up --if-down` entry; iPhone push with
  episode dedup.
- S2 — APPETITE THAT STOPS (≤4h): the structured field + open/
  set-appetite lifecycle + typed bands wiring + BREACH-STOP
  enforcement (park, wind-down, typed stop-state) + the rule-
  record store with the first three rules in observe-only.
- S3 — UP (≤4h): idempotent arming, dead-owner takeover, advisor
  outcome, session-start hook call; health's remedies switch to
  name it.
- S4 — HUMAN AUTHORITY (≤4h): the ancestry-proof seam for
  --by/acceptance/acknowledge; the acceptance record; land's
  acceptance check for safeguard slices.
- S5+ — DELEGATE (split per the custody design's own partition,
  consuming the S0b triage; typed outcomes, follow-up, cancel,
  revision binding), then WATCH, then LAND (atomic phases,
  receipt absorption). Each its own ≤4h slice, contents fixed at
  S0b's close.

Acceptance-gate, dual-running of old and new protections,
habituation retro test, stop-states, and reopen-observations carry
forward from v1 unchanged.

## Round 1 dispositions (r1-output.md)

| Finding | Disposition | Fold |
| --- | --- | --- |
| OSD-R1-01 | accepted | Recorded acceptance bound to tree hash; engine-of-record judges; rehearsed recovery named |
| OSD-R1-02 | accepted | Bootstrap-honest checks; mutual watching via the existing watcher; slices re-partitioned |
| OSD-R1-03 | accepted | Hook freshness check; three rings; boot-level resumer restored; push episode semantics |
| OSD-R1-04 | accepted | BREACH-STOP enforced: park + wind-down + typed stop-state |
| OSD-R1-05 | accepted | land: mandatory candidate selection; operation-id phases; receipt fields as flags |
| OSD-R1-06 | accepted | v1's collapse REVERSED: typed outcome enum stays; headline grouping only for humans |
| OSD-R1-07 | accepted | Triage is S0b, before all consumers |
| OSD-R1-08 | accepted | S0a isolation slice; health split; delegate split per custody partition; per-slice contents listed |
| OSD-R1-09 | accepted | Typed rule-record store; observe-only trials; must-stop/must-permit refs |
| OSD-R1-10 | accepted, RULED A | Process-ancestry proof for all human authority |
| OSD-R1-11 | accepted | Structured appetite wired to the typed bands; fixture named |
| OSD-R1-12 | accepted | open --appetite keeps open-claim atomic; legacy grace via health |
| OSD-R1-13 | accepted | delegate gains --follow-up and --cancel; closure internal |
| OSD-R1-14 | accepted | health/watch totality: unknown owned, exit codes, timeouts, zombie verdicts |
| OSD-R1-15 | accepted | Receipt family KEPT; land absorbs only add |
| OSD-R1-16 | accepted | proc acknowledge kept as human-authority verb |
| OSD-R1-17 | accepted, RULED B | Six daily + two lifecycle families |
| OSD-R1-18 | accepted | run family kept as lifecycle surface |
| OSD-R1-19 | accepted | Revision binding at delegate; land refuses stale revision without fresh acceptance |
| OSD-R1-20 | accepted | up's advisor outcome for second sessions |
| OSD-R1-21 | accepted (not-material) | Ledger paths corrected |
| OSD-R1-22 | noted | Goal statement confirmed against the paper |
| OSD-R1-23 | noted | Preservation/removal judgments confirmed grounded |


## ROUND 3 (FAILSAFE): 11 material — BUDGET EXHAUSTED, DESIGN OPEN, WORK HELD

Trajectory 20→15→11. The declared three-round budget is spent with
the open set enumerated: OSD-R3-01..11 (verbatim in
artifacts/agents/critiques/operator-surface/r3-output.md). Nine are
shape-level; the sharpest three are RULING INTERACTIONS only the
human can settle: R3-01 (the first accepted base is circular —
who accepts the base that judges the first acceptance), R3-02
(Ruling F's session-start `up` runs UNDER an agent and therefore
cannot satisfy Ruling C's agent-free terminal proof — enrollment
and arming must be split into a rare human act and an ambient
session act, if the human so rules), R3-03 (two-machine acceptance
custody under Ruling E needs a per-operation vs append-only ref
decision). The remainder: lock-order integration (04), dual-store
reconciliation (05), stop-vs-dual-run authority (06), quiescence
protocol (07), S0a prerequisites (08), retry-identity revision
(09, fixture-expressible), slice-table calendar honesty (10),
hook lastSuccess (11, fixture-expressible).

STANDING ORDER (Wido, 2026-08-28, recorded here as binding design
law): NO IMPLEMENTATION STARTS WITHOUT HIS EXPLICIT APPROVAL,
under any circumstance — a closed design is never a start signal.
CONTINUATION RULED (Wido: "agreed, do as proposed"):

- RULING I (resolves R3-02): ENROLLMENT AND ARMING ARE SPLIT.
  Enrollment is a rare HUMAN act performed once from Wido's
  agent-free terminal — it records the terminal root identity that
  Ruling C proofs walk to. Session-start `up` is AMBIENT: any
  session arms watchers, announces itself, and acquires the
  working lease WITHOUT claiming or requiring human authority; it
  consults the standing enrollment, never creates one. Human-
  authority verbs alone demand the agent-free ancestry.
- RULING J (resolves R3-01): THE GENESIS BASE IS A HUMAN ACT.
  Wido accepts the current landed tip as the first accepted base
  by one recorded acceptance performed from his enrolled terminal
  (the ch.15 bootstrap's "named human authority establishes the
  first bounded rules", literally). Every later acceptance chains
  from it; the bootstrap retrospective judges S0a..S4 against
  this genesis basis, resolving the circularity.
- RULING K (resolves R3-03): ACCEPTANCE CUSTODY IS PER-OPERATION
  IMMUTABLE REFS — one protected ref per landing operation,
  append-only by construction, no shared-ref race; a second
  machine's concurrent acceptance simply exists beside the first,
  and the branch CAS remains the single serialization point for
  what actually lands. Retry identity stays with the operation id;
  a lost CAS mints a NEW operation (new receipt, new acceptance)
  as the resolutions file already prescribes.

The remaining eight findings (R3-04..11) were resolved by the
co-design pattern: codex drafted, the coordinator adjudicated
ACCEPTED IN FULL — the v4 resolutions live in PART TWO of
plans/operator-surface-v3-resolutions.md and supersede overlapping
earlier text. Headlines: one ranked lock chain (chain →
goal-revision → cap → lifecycle → occupancy → record); the budget
journal DELETED (job reservations are the sole spending facts);
BREACH-STOP shadowed during dual-run per the transition records;
the total quiescence protocol pausing launches never
design/adjudication; S0a prerequisites honest under Rulings D/J;
revision in retry identity; hook lastSuccess semantics; and THE
CALENDAR-HONEST MANIFEST: 40 landings, ~158 clean execution hours
(~20 working days) plus ≥14 elapsed soak days across the T1-T4
authority transfers — the human buys this knowingly or descopes
it. One new human ruling open: the iPhone delivery-receipt
transport provider. The successor critique budget (rounds 4-6)
judges this v4; the no-implementation standing order remains in
force throughout.


## THE LEAN MANDATE (Wido, 2026-08-28 evening — supersedes the 40-landing manifest)

His ruling, verbatim intent: optimize where time is spent; BATTERY
OUT (not interested — gates never run it; consistent with his
standing battery-cadence override); TESTS YES, FAST (the
per-landing gate runs gofmt, vet, and focused tests bounded at
minutes, never nine-minute suites — a slow suite runs only when
its own subsystem changes); BENCHMARK OUT; land the functionality
tested-fast; the 158-hour from-scratch program is REFUSED; the
functionality itself is WANTED, much faster.

What the lean recount cuts and why it is honest:
- The four T-transfer ceremonies with 7-day soaks each →
  REPLACED by parity fixtures (the old path's must-stop and
  must-permit cases run against the new path in the same landing)
  plus ONE program-wide soak week during which old paths merely
  remain callable. The comparison-record machinery is CUT.
- Recovery bundles S4d-S4h (five landings) → ONE landing: the
  git-snapshot recovery we practiced live this week, documented,
  plus one destructive rehearsal. The full bundle manifest was
  ch.15 gold-plating for a two-operator shop.
- Custody S5a-S5g priced as fresh builds → APPLY the wip branch's
  verified work through the triage (three landings, not seven).
- The governed-rule store S8a/S8b → DEFERRED post-program; the
  three blessed invariants land as plain enforced code with the
  blessing recorded.
- Phone-transport provisioning → DEFERRED; desktop banner + chat
  line now.
- Batteries/benchmarks → out of every gate, per the ruling.

THE LEAN MANIFEST (~17 landings, ≤4h ceiling each, honest
estimate at demonstrated pace ~2h average — three to five days of
continuous operation, with the human's acts as the only sync
points: enrollment, genesis acceptance, per-slice approvals):
L1 S0a isolation (2h) · L2 triage-and-apply plan · L3 health core
· L4 rings (chat line, mutual watching, alerts) · L5 budget schema
+ revision binding · L6 breach-stop enforcement · L7 up · L8
enrollment + authority proof · L9 genesis acceptance + landing
acceptance check · L10-L12 custody applied from wip (identity/
claim/occupancy; group custody + progress; call-site migration) ·
L13 delegate verb · L14 watch · L15 land verb (phases + receipt)
· L16 recovery documented + rehearsed · L17 old-path deletions
after the soak week. Fast-test law binds every landing's gate.
