# The process steward (backlog item 21)

- Status: PARKED (D87 as corrected by D94). Two critique rounds
  (plans/ps-critique-r1.md, 7 findings; plans/ps-critique-r2.md, 8
  findings) plus the Opus-window special review
  (plans/opus-window-review-steward.md, 3 findings) shaped this
  record. The park stands; the original fold's contradictions are
  normalized out — this document is ONE voice: what is true today,
  why the goal waits, and the contracts a resumed design must
  settle.
- Goal: process-steward (parked)
- Next step: none.

## The human's mandate (2026-08-16, verbatim intent)

"Something that watches the process itself and signals issues... so
that the orchestration agent can act on them... more like a process
coach... add this to the backlog and name it." Named the PROCESS
STEWARD: it NOTICES process drift and SIGNALS it. The critiques
proved the noticing must be narrow and owner-fed, and the tending
must not act — an observer that signals, never a second actor.

## What is true today (all three reviews agree)

1. **No owner emits a typed process-health verdict yet.** Every
   existing signal is either a Boolean that collapses
   missing/unreadable/stale/dead (supervise.ArmedNow), prose lines
   for a human (WatchdogReport), or raw records whose
   reinterpretation would create a second policy owner (ps-r2 #1,
   #6). The steward's first prerequisite is a typed verdict at an
   OWNER boundary — outcome + reason + evidence identity — which is
   new instrumentation, not something owners already expose.
2. **The watchdog LARGELY overlaps the liveness invariant but is
   not equivalent** (the window's D87 overclaimed "no new
   coverage"; D94 corrects it). WatchdogReport: census freshness up
   to min(2×interval,180s), no heartbeat/generation/cap/tag
   verification, no session precondition, prose not tri-state,
   discarded on blocking turns, not persisted, Stop-only. ArmedNow
   verifies more but answers only true/false. A supervise-owned
   typed verdict would add real predicate and outcome coverage —
   the r2 verdict stands anyway because that verdict does not
   exist and building it is the owner's work, not a scanner's.
3. **The empty act-allowlist is settled** (ps-r1): the steward
   observes, persists, signals. run ack is holder-only, kill is
   destructive, coach dispatch spends — none are steward acts.
4. **The valuable invariants each wait on owner instrumentation**
   (ps-r2 #8, sharpened by later work): a process-orphan verdict
   needs process-GROUP death proof, not custody-list death (D89
   proved custody death insufficient — so this is NOT cheap);
   plan-work correlation needs a decision owner that does not
   exist (internal/report scans facts, internal/goal owns the turn
   decision, Run.GoalId is unpopulated by the public verbs); ship
   certification needs a real domain owner (gaterun owns live
   markers, not outcomes; commit.sh enforces lease authority, not
   certification). Each is designed at ITS boundary first.

## Why the goal is parked

The one nearly-checkable invariant would duplicate most of an
existing warning while still requiring a new supervise-owned typed
verdict, an incident lifecycle, a scan-cadence/attestation
protocol, a turn-verdict state-machine extension, and arming
integration — a large build for marginal new coverage. The
genuinely new invariants wait on owners that do not emit their
verdicts yet. Building a scanner that reinterprets raw records
instead would create a competing policy owner, the exact ps-r2 #6
failure.

RESUME CONDITION (corrected by the special review): the park lifts
when a sound typed OWNER verdict about a currently-unwatched
invariant exists — whichever owner lands one first; the janitor
orphan verdict is A candidate, not THE dependency (D89 showed it
needs group-death proof, so it is not cheap). Meeting the condition
authorizes RESUMING THE DESIGN, not building: the resumed round
must first show why that verdict should pass through a steward
aggregator at all, rather than being delivered directly by its
owner or the existing turn verdict.

## The contracts a resumed design must settle (from ps-r2, kept)

- A supervise-owned (or other owner's) TYPED verdict — outcome +
  reason + evidence identity — the steward reads and never
  re-derives (r2 #1).
- ONE named supervision-health owner contract, so no fourth
  freshness window disagreeing with dispatch, the watchdog, and
  the run-pass reader (r2 #2).
- A decision table whose EVERY missing/unreadable-evidence path is
  the same fail-unknown outcome (no "unknown here, breach there"),
  with a named, kernel-revalidated source for "a session is
  active" (r2 #3).
- An incident record with stable identity + semantic digest,
  compare-and-swap ordering, reopen/resolve transitions, a rule
  for whether unknown preserves an open breach, evidence refs
  bound to generation/digest, AND the durable-write contract's
  committed-but-durability-unknown outcome with its anchor — the
  run-record disciplines plus the atomicfile contract, not bare
  atomic rename (r2 #4, restored by D94; the window's fold had
  dropped the durability-unknown outcome).
- A fully specified Stop-hook composition: input fields, a
  precedence table, an all-clear veto, no blocking authority for a
  steward-only finding, acknowledgement-safe dedup/clear/retry
  composed into the ONE authoritative turn display (r2 #5, #6).
- A named freshness owner for the steward-pass attestation with
  caller, cadence, future-clock rule, first-attestation bootstrap,
  and publication ordering (r2 #7).
- The coach as a rostered role: model binding in metasystem.conf
  roster config (not adapter config), the `none` permission preset
  (repository reads, no writes), untrusted advice adjudicated by
  the orchestrator, explicit caller + cost budget + cadence +
  dedup before any automatic invocation (r2 #9).

## Who watches the steward (kept from r2/r1)

Not a third supervision component (that set is schema-fixed at
watcher+reaper). The steward scan writes an independent
steward-pass attestation; the outside observers are the Stop hook
and the next arming — the boundaries that already catch a dead
supervision fleet. The attestation's freshness owner is one of the
contracts above; without it, "who watches" remains topology, not
operation.

## Loop discipline

The loop resumes on the resume condition, against the contracts
above, attacking first whether the aggregator earns its existence
over direct owner delivery. Critique at codex xhigh, as ever.
