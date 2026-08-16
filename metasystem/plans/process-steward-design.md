# The process steward (backlog item 21)

- Status: PAUSED on a sequencing truth (D87). r1 (plans/ps-critique-r1.md,
  7 findings) forced the rescope to a read-only aggregator; r2
  (plans/ps-critique-r2.md, 8 findings) established that the only
  currently-buildable invariant duplicates the Stop watchdog and
  every valuable invariant waits on owner-boundary instrumentation.
  Not converged — blocked on a dependency.
- Goal: process-steward (blocked; resequenced behind the janitor
  namespace-orphan verdict from disk-hygiene, D87)
- Next step: BLOCKED, not actionable now — do not pick this up as
  open work. It waits on a typed verdict about a currently-UNWATCHED
  invariant that no owner emits yet. The cheapest candidate — a
  "finished job left a live process" orphan verdict — turns out to
  need process-GROUP death detection (custody-list death is not
  group death; a reverted worktree-observer attempt proved this,
  plans/wt-code-critique-r1.md, D89), so it is NOT cheap and is
  itself unbuilt. The full first slice also needs the incident
  record, Stop-precedence, and attestation-freshness contracts named
  below. Resume only when a sound owner-boundary verdict exists;
  then build the aggregator + those contracts once, against it. Do
  NOT build the supervision-liveness duplicate.

## The human's mandate (2026-08-16, verbatim intent)

"Something that watches the process itself and signals issues... so
that the orchestration agent can act on them... more like a process
coach... add this to the backlog and name it." Named the PROCESS
STEWARD: it NOTICES process drift and SIGNALS it. r1's critique
proved the noticing must be narrow and records-based, and the
tending must NOT act — so r2 is an observer that signals, never a
second actor.

## The reshape r1's critique forced

1. **Aggregate, don't reinterpret (PSD-06).** Five existing owners
   already produce typed verdicts about their own domains
   (supervision liveness, run lifecycle, turn warnings, open-work,
   gate/critique conformance). The steward AGGREGATES those typed
   verdicts into one process view; it does NOT re-derive them from
   raw records — that would make it a sixth policy owner with
   threshold disagreements.
2. **Only what's observable ships (PSD-01).** Of the five candidate
   invariants, exactly ONE is fully checkable from existing records
   today (supervision liveness, via the canonical armed predicate).
   The others need new records at THEIR owning boundaries (a
   temp-namespace ownership record; a typed plan-work correlation;
   an ack-history/timestamp; a ship certification joining tree +
   gate + critique). r2 ships the aggregator over what owners
   already expose and lists the missing owner-boundary records as
   named prerequisites — it does not promise checks it cannot make.
3. **The act-allowlist is EMPTY (PSD-02).** `run ack` is holder-only
   authority; killing is destructive; dispatching a coach spends and
   is authority-bearing. The steward OBSERVES, persists a verdict,
   and SIGNALS. Any future remediation is a separately reviewed verb
   owned by the affected subsystem — never the steward's.

## The first slice: a read-only aggregator + one durable record

`steward scan --root` reads the TYPED verdicts the existing owners
already expose and writes ONE durable, atomic, typed STEWARD
INCIDENT RECORD (its own file under artifacts/agents/steward/,
atomic-published like every other record — NOT the flight recorder,
which is best-effort and may not own authority; NOT the run ledger,
which is a process ledger, per PSD-04). The record carries: scan
identity, completion time, per-invariant outcome, evidence
references, and an explicit UNKNOWN/DEGRADED outcome when an
authority record is missing or unreadable (PSD-05 — missing
evidence is never silently ok and never permission to act).

### The one shipped invariant, with its decision table (PSD-05)

**Supervision liveness.** Reuses the CANONICAL armed predicate
(internal/supervise/verifyarmed.go) — the steward introduces NO
fourth timing rule (PSD-01 flagged three existing intervals).

| Outcome | Predicate |
|---|---|
| ok | a session is active AND supervision is armed AND attesting fresh by the canonical predicate |
| unknown | the supervision state or attestation is unreadable |
| breach | a session is active AND the canonical predicate says not-armed-or-stale |

This is an IMMEDIATE binary invariant (a missing attestation is a
breach now, not a trend). Trend-based invariants (unacked runs
rising) are DEFERRED because ack history is not recorded (PSD-01);
they arrive when their owning boundary records the history.

### The owner-boundary prerequisites (named, not promised)

Each future invariant is a record at its owner, designed there
first (PSD-01, PSD-06):
- temp-namespace ownership + orphan observation (internal/run +
  the namespace the disk-hygiene goal introduces);
- typed plan-work correlation (expose the run's GoalId + a
  runtime-neutral background-work record; internal/report open-work);
- ack timestamp/transition (internal/run conclude/ack);
- ship certification joining shipped tree + green gate witness +
  critique/waiver (owned at the commit/gate boundary, gaterun +
  commit.sh — retrospective detection cannot reconstruct evidence
  never preserved, PSD-01).
The steward gains each check only when its owner emits the typed
verdict; it never scrapes.

## Who watches the steward (PSD-03)

The steward is NOT a third supervision component (that set is
schema-fixed at watcher+reaper). Instead the steward scan writes an
independent STEWARD-PASS ATTESTATION (its own freshness record).
The OUTSIDE observers are the ones r1's critique named: the Stop
hook and the next arming — the same independent boundaries that
already catch a fully-dead supervision fleet. The steward's own
staleness is thus one of the process facts the Stop hook surfaces,
closing the loop without circularity.

## Delivery (PSD-04)

The durable steward incident record is the authority. Delivery is
an accelerator: the Stop hook (which already does digest-based
exactly-once turn-verdict delivery) gains the steward verdict as
one MORE input, with explicit precedence and dedup against the
existing watchdog/turn-verdict signals — never a second
independent warning source that contradicts them. The no-hook
fallback still exposes the durable record (the accelerator ruling:
correctness survives on records alone).

## The coach (PSD-07), deferred to a later slice

The coach is a ROSTERED ROLE dispatched through the existing
orchestration path (not an adapter — only its model binding is
adapter config), with NO repository permissions, producing
UNTRUSTED advice the orchestrator adjudicates. Automatic invocation
needs an explicit caller, cost budget, cadence, and dedup — so it
is a LATER slice, after the read-only aggregator proves the
verdict record. First slice signals via the record + Stop hook
only.

## The r2 verdict: this goal is blocked on owner instrumentation (D87)

The r2 critique (plans/ps-critique-r2.md, 8 structural findings,
codex xhigh) accepted the rescope's direction but reached a
load-bearing conclusion the design must honor rather than fold away:

**The one currently-checkable invariant duplicates the Stop
watchdog, and every genuinely-unwatched invariant needs a typed
verdict its owner does not yet emit.**

- Supervision liveness is ALREADY surfaced end-of-turn by
  `internal/supervise.WatchdogReport` (stale/failed census,
  untracked processes, stale fingerprint, dead recorded identities,
  each with re-arm advice) through the same Stop hook the steward
  would use. A steward re-check is a duplicate signal, not new
  coverage (r2 #36).
- Worse, even that duplicate is NOT a small slice: `ArmedNow` is a
  Boolean that collapses missing/unreadable/stale/dead into `false`,
  so the steward cannot even distinguish `unknown` from `breach`
  without a NEW supervise-owned typed liveness verdict (r2 #1) — and
  it would still need an incident lifecycle, a scan-cadence/
  attestation protocol, a turn-verdict state-machine extension, and
  arming integration (r2 #4–#7).
- The invariants that would be genuine NEW value — orphaned temp
  namespaces, unleashed plan promises, uncertified ships — each
  require instrumentation at an owner that does not exist yet
  (r2 #8): the janitor's namespace-orphan verdict (a disk-hygiene
  slice), `Run.GoalId` populated by the public launch/register
  verbs plus a plan-work decision owner, and a real ship-
  certification domain owner joining tree + green gate + critique.

Building a duplicate of the watchdog to add complexity for no new
coverage is the opposite of the clean system the program is for.
So the steward does not build now. D87: process-steward is
RESEQUENCED behind the cheapest owner-boundary that emits a typed
verdict about a currently-unwatched invariant. The natural first
one is the **janitor namespace-orphan verdict** from the
disk-hygiene goal — when that owner emits a typed
"finished run left a process/namespace behind" verdict, the steward
becomes a thin read-only aggregator that surfaces it (a fact the
watchdog does NOT cover), and the incident-record / attestation /
Stop-precedence contracts below get built once, against a verdict
that pays for them.

## The target design once unblocked (unchanged from r2's shape)

When the first genuinely-new owner verdict exists, the first slice
is the read-only aggregator + one durable typed incident record
described above, with these r2 contracts made concrete rather than
asserted:

- a supervise-owned (or janitor-owned) TYPED verdict carrying
  outcome + reason + evidence identity — the steward reads it, never
  re-derives it (r2 #1, #8);
- ONE named supervision-health owner contract, so the steward does
  not invent a freshness window that disagrees with dispatch, the
  watchdog, and the run-pass reader (r2 #2);
- a decision table with a SINGLE fail-unknown outcome for missing/
  unreadable evidence (no "unknown here, breach there") and a named,
  kernel-revalidated source for "a session is active" (r2 #3);
- an incident record with a STABLE incident identity + semantic
  digest, compare-and-swap ordering, reopen/resolve transitions, a
  rule for whether `unknown` preserves an open breach, and evidence
  refs bound to their generation/digest — the run-record disciplines,
  not bare atomic rename (r2 #4);
- a fully specified Stop-hook composition: input fields, a
  precedence table, an all-clear veto, no blocking authority for a
  steward-only finding while the act-allowlist is empty, and
  acknowledgement-safe dedup/clear/retry that composes into the ONE
  authoritative turn display instead of racing the watchdog digest
  (r2 #5, #6);
- a named freshness owner for the steward-pass attestation with a
  defined caller, cadence, future-clock rule, first-attestation
  bootstrap, and publication ordering (r2 #7);
- the coach as a rostered role whose model binding is `metasystem.conf`
  roster config (NOT adapter config), on the `none` permission preset
  (repository reads, no writes), producing untrusted advice (r2 #9).

## Loop discipline

The design loop pauses here, not on convergence but on a sequencing
truth: two rounds established that the buildable slice duplicates
existing machinery and the valuable slices wait on instrumentation
owned by other goals. Resuming the loop (r3+) on the current premise
would re-derive the same duplication finding. The loop resumes when
an owner boundary ships its first typed verdict about an unwatched
invariant — at which point the critique attacks the concrete
contracts above (incident lifecycle, Stop precedence, attestation
freshness), not the premise.
