# The process steward (backlog item 21)

- Status: DRAFT r2 — critique r1 folded (plans/ps-critique-r1.md:
  7 findings, all structural; all folded, none refuted). The r1
  over-reached; r2 scopes to a read-only aggregator.
- Goal: process-steward
- Next step: Fold the critique verdict when run ps-critique-r2 concludes; implement only after convergence.

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

## Prototype plan

P1: the aggregator as pure Go reading the supervision-liveness typed
verdict via the canonical predicate, emitting the typed incident
record with ok/unknown/breach and evidence — fixtures for each
outcome including unreadable-evidence→unknown. P2: `steward scan`
verb + the steward-pass attestation + the Stop-hook input with
precedence/dedup. Owner-boundary records and the coach are named
follow-ups, each their own goal.

## Loop discipline

Critique at codex xhigh; the critique should attack: whether the
aggregator truly only reads typed owner verdicts (no raw
reinterpretation); whether the canonical armed predicate is the
right single source; whether the incident record's atomicity and
unknown-state handling are complete; whether the Stop-hook
precedence/dedup is fully specified against the existing
turn-verdict contract; and whether the owner-boundary prerequisites
are correctly assigned to their owners rather than smuggled back
into the steward.
