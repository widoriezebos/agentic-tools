# Multi-main coexistence: two sessions, one repository, no interference

- Goal and current status: two main agents in one checkout can no longer interfere — streams carry mechanical owners, the working tree has one writer, the turn-end hook commands only work that is yours to start, and a delegate's identity claim can never overwrite what the adapter observed. Closes KI-21 and KI-22. Status: IN CRITIQUE, round 1 folded (13 material findings, MM-1-1..13) by full rewrite of the Changes section — no amendment layering, per the round-5 lesson of the validity chain.
- Next step: design critique by the design-critic role (gpt-5.6-sol), worktree synced per KI-20's interim rule; dispatch waits for the currently running V-1/V-4 gate chain to finish so the suite is not raced.
- In flight right now: nothing in this stream; the V-1/V-4 gates are finishing in the sibling stream
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

**M-1: stream ownership is a lease, not an identity.** Durable ownership
cannot ride a runtime session UUID that dies at every restart (MM-1-2).
Every plan gains a tool-owned `Stream-Owner` line holding a claim into the
lease system below; the turn-end open-work check resolves it: owned by the
calling main — commanded; owned by a live lease-holder — reported as
`OWNED-ELSEWHERE`, never commanded; lease expired — claimable, and the
claim writes a takeover line (old holder, new holder, timestamp) into the
plan. Two peers claiming simultaneously are serialized by the same
compare-and-swap one-shot-patch mechanism dispatch already uses for record
updates — the loser's swap fails and it re-reads (MM-1-3).

**M-2: one lease over the checkout's MEDIATED mutations — and an honest
boundary.** The harness cannot prevent a second session from editing files;
no hook sees an editor write (MM-1-1). What it can and now does serialize,
fail-closed, is every mutation it mediates: dispatch, commit, and
supervision arming require the checkout lease. The lease file
`artifacts/agents/mains/worktree-lease.json` carries holder mainId, pid,
pidStartedAt, claimedAt, renewedAt, ttlSec, and a takeover history
(MM-1-5); the holder's stop/start hooks renew it each turn; acquisition and
renewal go through the same compare-and-swap (MM-1-3). Expiry is the
takeover path: a holder that stops renewing is dead or idle within one TTL,
and a peer then claims cleanly — restart-and-resume costs at most one TTL
of downtime instead of orphaning streams (MM-1-2). The residual risk —
a peer editing shared files outside any mediation — is DETECTED, not
prevented: each session's stop hook fingerprints `git status` and reports a
working tree that changed outside the session's own recorded operations.
Prevention would require sandboxing the human's own sessions; the design
declines and says so.

**M-2a: callers are authenticated by process ancestry (MM-1-4).** Every
mediated operation resolves the calling process's ancestor to an announced
main via the census's existing find-ancestor, and checks that main against
the lease. Delegates run detached in their own sessions, so their ancestry
never reaches a main and they can never act as one; a caller with no
announced ancestor is refused.

**M-2b: recovery is claim-first and never circular (MM-1-6).** When the
holder is dead and supervision is stale, the order is: wait out or
human-override the lease (the typed-approval TTY path F-4 established),
claim it, then arm as the holder. Arming checks the lease, not history —
so the revived repository has exactly one legitimate armer and there is no
state where nobody may arm.

**M-2c: takeover adopts surviving work (MM-1-7).** Claiming an expired
lease runs the existing reaper over the previous holder's running jobs:
dead processes are classified as today; live delegates are adopted —
recorded in the takeover history with adoptedFrom — and their chains
follow up via the documented resume-or-embed fallback. A new holder never
races a survivor it does not know about.

**M-3: in-flight truth comes from records, and jobs name their stream
(MM-1-8).** Dispatch gains `--stream <plan>`; the job record carries it;
the open-work check derives per-stream in-flight state from job records
alone. Plans' prose lines stay for humans and stop being load-bearing.
Legacy stream-less jobs count globally, as today.

**M-4: identity fields state their provenance, per runtime, honestly
(MM-1-10).** The canonical sessionId and model.effective come from adapter
observation where the runtime provides one — claude and codex both do
(handshake signal; result telemetry per V-1) — and are recorded as the
literal `unobserved` where it does not, rather than pretending. A
delegate-claimed value never overwrites observation: it lands in an
optional `claimed` object, added in a versioned bump of the return schemas
with a migration window during which the validator accepts both versions
(MM-1-11); the claim-versus-observation mismatch that burned six rounds
becomes a recorded fact, not a protocol error.

**M-5: protocol errors surface at detection, where the human already
looks.** The counter on the chain root increments when
assert-return-complete detects the violation — not at the next follow-up,
which may never come (MM-1-12). The surface is the one line the human
demonstrably reads: the turn-end status message this week built, which
gains `PROTOCOL ERRORS: <chain> grew to N` whenever a counter grew since
the session's last turn, tracked by a per-session cursor (MM-1-13).

**M-6: the identity hash covers configuration, not CLI bookkeeping
(absorbs KI-19; closes the read-only-peer channel, MM-1-9).** A read-only
peer still churns shared CLI state simply by existing; the fix is that the
identity hash stops caring: each adapter declares the self-written
sections of its config (codex: notice and TUI state; claude: its session
bookkeeping) and the hash covers the filtered remainder. A fixture proves
a notice-flag touch keeps identity while a model or permission change
breaks it. With M-6, a read-only peer's presence changes nothing any gate
reads, and KI-21's second tooth closes with its first.

## Proof

- Fixture: two fake mains registered; the open-work check commands the
  owner, reports OWNED-ELSEWHERE to the peer, and permits takeover only
  after the owner is dead — with the takeover line written.
- Fixture: dispatch and commit from a non-owner refused while the owner
  lives, naming the owner; permitted after death-plus-claim.
- Fixture: arming by a non-owner refused.
- Fixture: a return claiming the orchestrator's session id normalizes to
  the adapter-observed id, the claim lands in `claimed` under the bumped
  schema (both schema versions accepted during the window), the round is
  not a protocol error, and a synthetic mismatch in the adapter-observed
  value itself still fails.
- Fixture: two simultaneous claims on one expired lease — exactly one
  wins the compare-and-swap; the loser reads the winner's claim.
- Fixture: holder dead, supervision stale — claim then arm succeeds;
  arm before claim refuses; no circular state.
- Fixture: takeover with a surviving live delegate — adopted, recorded,
  chain follow-up works.
- Fixture: a protocol error on a terminal round with no follow-up still
  increments the counter and appears in the next turn-end line.
- Fixture: the filtered identity hash — CLI bookkeeping churn keeps
  identity; a model or permission change breaks it (M-6).
- Fixture: a follow-up onto a protocol_error round prints the notice and
  bumps the counter; the turn report names the chain.

## What is deliberately not changed

The mains registry and census stay observation-only for anything that is
not dispatch, commit, or arming — a read-only peer is legitimate and
useful, as the transcript itself demonstrated. And the one-main-per-
checkout INTERIM rule stays in force until this design is implemented and
proven; this design is what makes the rule enforceable instead of polite.
