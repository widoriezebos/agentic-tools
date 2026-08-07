# Multi-main coexistence: two sessions, one repository, no interference

- Goal and current status: two main agents in one checkout can no longer interfere — streams carry mechanical owners, the working tree has one writer, the turn-end hook commands only work that is yours to start, and a delegate's identity claim can never overwrite what the adapter observed. Closes KI-21 and KI-22. Status: IN CRITIQUE, rounds 1-2 folded by rewrite (13 + 14 material findings). Scope judged at the round-2 fold: this stays ONE design — every finding lands on the coexistence mechanisms themselves, and the two candidates for splitting (return-schema versioning, adoption) resolved into two paragraphs each, not protocols.
- Next step: critique round 3 over the round-2 folds
- In flight right now: nothing — the critique chain is between rounds, orchestrator adjudicating (the supported IL-16 state)
- Waiting on: nothing. Devin integration is PARKED by the human's decision of 2026-08-07 until this design is implemented and proven.

## The evidence this stands on

On 2026-08-07 the human launched a second Claude session in this checkout.
The census and mains registry tracked both correctly — observation held.
Enforcement did not exist, and the peer session's transcript (read-only, at
`/Users/wido/Downloads/interference-transcript.txt`) proves three failures:

1. The turn-end hook COMMANDED the second session to start work the first
   session was actively running, because plans' "in flight" prose was stale
   and no stream records an owner. One human rejection prevented a
   double-dispatched stream and a supervision re-arm under a live `--wait`.
2. Both sessions share one git index and uncommitted state; either can
   scoop the other's half-written edits into its own commit. The 0b9ca1b
   pre-commit guard covers only new plan files.
3. KI-22, found by the peer from outside: every follow-up round of chain
   design-critic-20260807t063006z-8fb2 is stamped failed/protocol_error
   because the critic echoed the brief's "session af965b26" as its own
   sessionId. The validator caught it every round; nothing surfaced it; the
   findings were used anyway.

## Changes

**M-0: mainId, defined once and used everywhere.** The start hook mints
`main-<pidStartedAt>-<pid>-<rand6>` at announce time and records it in the
announcement file. It names one main PROCESS LIFETIME — no continuity across
restart is promised or needed: a restarted main is a new mainId whose claim
follows the dead-holder path below, and job records carry the dispatching
mainId so a successor can find its predecessor's work (MM-2-3, MM-2-7).
Authentication never takes a process's word for its mainId: mediated
operations walk the caller's ancestry toward init, and the FIRST classified
ancestor decides — an adapter's agent signature means delegate context,
refused; a pid-plus-start-time match against an announcement means that
main, authenticated, mainId read from the record; walk exhausted with no
announced ancestor means not a main. Delegates always meet their adapter's
signature before any main because the launcher sits between them — a rule
proven by a fixture that calls a mediated operation from inside a dispatched
fake delegate and is refused (MM-2-4).

**M-0a: humans are not gated.** A caller with NO announced ancestor and no
agent signature is the human's own shell or client, and every mediated
operation allows it untouched — the threat model is agent-versus-agent
interference; gating the human's own commits would be the harness
overreaching, and this design says so as a trust decision rather than an
oversight (MM-2-5).

**M-1: stream ownership resolves through the checkout lease.** Plans carry
`Stream-Owner: <mainId>`. The open-work check commands work owned by the
calling main or unowned; reports `OWNED-ELSEWHERE` for a holder that is
alive; treats a dead holder's streams as claimable, the claim writing a
takeover line into the plan.

**M-2: the checkout lease — alive-or-expired, never both.** The lease file
carries holder mainId, pid, pidStartedAt, claimedAt, renewedAt, ttlSec, a
monotonically increasing `generation`, a `state` of `active` or
`takeover-in-progress`, and takeover history. Liveness beats the clock:
expiry requires BOTH renewedAt older than ttlSec AND the holder process
dead by the census's pid-plus-start-time check — a live holder mid-long-turn
can never be expired out from under (MM-2-1), and a crashed holder can be
taken over the moment its death is provable, without waiting out any TTL.
ttlSec is owned by `metasystem.lease.ttl-sec`, default 900, bounds 300 to
3600; it only bounds how long an UNPROVABLE death (machine partition)
blocks takeover. Every mutation — claim, renew, takeover — runs under a
dedicated flock on the lease's lock sibling: re-read, validate expected
generation, decide expiry under the lock, write via temp-and-rename,
generation incremented. Linearization is the lock order; a renewal that
finds its generation superseded has lost to a takeover and the old holder
demotes itself on the spot (MM-2-2 — this is a new named mechanism, not the
dispatcher's job-status helper, and the proof covers stale-renewal-after-
takeover explicitly).

**M-2b: recovery is claim-first** — holder dead, supervision stale: claim
(instant on provable death), then arm as holder. No circular state.

**M-2c: takeover is a state, not a moment (MM-2-7).** The claim writes
`state=takeover-in-progress`; while it stands, dispatch refuses everyone
including the new holder. The adoption scan then walks the predecessor
mainId's non-terminal jobs using the existing per-job record locks: a
compare-and-swap of running to adopted records the new mainId; a swap lost
to the delegate's own terminal write is accepted by re-read — terminal
needs no adoption. Scan done, `state=active`. Classification and adoption
sit inside the per-job lock; the takeover state spans the whole scan.

**M-3: jobs name their stream, with a defined argument (MM-2-14).**
Dispatch accepts `--stream <plan>`; the value normalizes to a repository-
relative path that must exist under plans/, and when the flag is absent
dispatch derives it from the brief's plan reference. The reserved value
`none` (used by adapter selftests and ad-hoc jobs) is exempt from
derivation. Jobs with stream `none` or legacy records count as in-flight
for EVERY plan — conservative, current behavior, no new suppression class.

**M-4: identity provenance is a table, not a sentence (MM-2-13).**
Per field and runtime: sessionId — claude observed at handshake, codex
observed as the thread id, devin unobserved; model.effective — claude
observed from result telemetry (V-1, a CLAUDE mechanism; the earlier codex
citation was wrong), codex currently an unobserved requested-echo and
therefore recorded as `unreported` until its adapter gains result
telemetry (a small implementation item of this design), devin unobserved.
Unobserved fields hold the literal `unobserved`; a delegate-claimed value
lands only in the `claimed` object. Return schemas bump to version 2 with
a REQUIRED `schemaVersion: 2` field; the discriminator is presence — a
return without the field validates against the frozen v1 schema, one with
it against v2; the v1 path retires once all adapters emit v2 (MM-2-12 —
executable with the existing one-schema validator, no oneOf).

**M-5: protocol errors are a keyed set, surfaced once (MM-2-8, MM-2-9).**
The chain root records `protocolErrors` as a set keyed by round and
violation hash, written add-if-absent under the record lock at the adapter
lifecycle point that persists the failed round; repeated validation runs
are harmless and follow-ups only print. The turn-end line reports growth
against a per-main cursor file initialized AT ANNOUNCE to the then-current
set sizes and advanced only after the status line is emitted; a takeover
prints the predecessor's outstanding counts once at claim, so inherited
errors are seen exactly once rather than missed or replayed.

**M-6: the identity hash filter is data, versioned, fail-closed (MM-2-10)
— and its claim is narrowed to what it delivers (MM-2-11).** Each adapter
ships an exact path list (`<runtime>-config-filter.v<N>.txt`, instruction-
bearing, never waivable) naming the CLI's self-written bookkeeping keys for
a declared CLI version range; only enumerated keys are excluded, unknown
and new keys are hashed, and a CLI version outside the declared range
hashes everything and warns — churn over blindness. What M-6 removes is
FALSE churn. A peer that changes real shared behavior — saving a new
default model, as the transcript's peer did — rightly churns identity, and
the refusal now names the changed keys so the cause is visible in one
line. Full per-main configuration isolation would be the complete fix and
is recorded as future work, not smuggled in; until then the one-main
interim rule and OWNED-ELSEWHERE reporting are the standing mitigation.

**M-7: foreign-edit detection is withdrawn (MM-2-6).** The designed
evidence cannot attribute an unmediated edit to a session — a fingerprint
without attribution either accuses every legitimate edit or misses every
foreign one. The stop hook keeps a purely informational dirty-tree line;
the residual risk of unmediated peer edits is mitigated by the interim
rule and M-2's serialization of everything mediated, and it is named here
as accepted rather than papered over.

## Proof

- Two fake mains: open-work commands the holder, reports OWNED-ELSEWHERE
  to the live peer, permits claim only on provable death or lease expiry,
  and writes the takeover line.
- Lease under flock: two simultaneous claims — one winner by generation;
  a stale renewal after takeover fails and the old holder demotes; a live
  holder past its TTL is NOT expirable; a dead holder is claimable before
  its TTL lapses.
- Ancestry: a mediated call from inside a dispatched fake delegate refuses;
  from an announced main authenticates; from a bare human shell passes
  untouched (M-0a).
- Takeover state: dispatch refuses during takeover-in-progress; adoption's
  lost compare-and-swap against a delegate's terminal write is accepted by
  re-read; adopted jobs carry the new mainId.
- Returns: a v1 return without schemaVersion validates against v1; a v2
  return with claimed object validates against v2; the claimed value never
  overwrites an observed field; unobserved fields hold the literal.
- Protocol errors: the same violation validated twice yields one set entry;
  a terminal round with no follow-up still appears in the next turn line;
  a fresh main's first turn reports only post-announce growth; a takeover
  prints inherited counts once.
- Filter: bookkeeping churn keeps identity; a model change breaks it AND
  the refusal names the changed key; an out-of-range CLI version hashes
  everything with a warning.

## What is deliberately not changed

The mains registry and census stay observation-only for anything that is
not dispatch, commit, or arming — a read-only peer is legitimate and
useful, as the transcript itself demonstrated. And the one-main-per-
checkout INTERIM rule stays in force until this design is implemented and
proven; this design is what makes the rule enforceable instead of polite.
