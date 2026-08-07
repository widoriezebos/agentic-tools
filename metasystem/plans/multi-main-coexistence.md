# One writer, safe readers: sessions sharing a repository without interference

- Goal and current status: sessions sharing a checkout cannot interfere because exactly ONE main holds the write role, enforced mechanically; every other session is a first-class read-only advisor with a paved one-command path to its own worktree when it wants to write. Closes KI-21 as experienced and KI-22. Status: RESCOPED at round 3 per IL-23 — material counts ran 13, 14, 13 with four criticals, and MM-3-8 named the truth: the mechanisms kept depending on the one-main rule, so the design now promises exactly that rule, enforced, instead of live two-writer coexistence it could not deliver. Rounds 1-3 folded.
- Next step: critique round 4 over the rescoped specification
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

## The rescope decision (IL-23, recorded)

Forty findings in three rounds kept landing on the same fault line: live
two-writer coexistence in one checkout is a distributed-systems protocol
that fights git's index, shared CLI state, and process-tree ambiguity. The
peer session that triggered this design already demonstrated the working
model — it stayed read-only, was useful, and asked the human before
touching anything. This design now enforces that model instead of fighting
for a harder one. Live takeover machinery (TTL dances, adoption scans,
takeover states) is DELETED, not deferred: takeover exists only on provable
death.

## Changes

**W-1: identity.** The start hook mints `main-<pidStartedAt>-<pid>-<rand6>`
at announce and records it in the announcement. Announce is idempotent,
keyed by pid plus start time: a duplicate start event returns the existing
mainId and never re-mints (MM-3-5). Process identity is pid PLUS start
time everywhere it is compared, which closes same-second pid reuse
(MM-3-11).

**W-2: caller classification, cooperative and self-first.** The session
start hook exports `METASYSTEM_MAIN_ID` into the main's environment; the
delegate launcher strips it and sets the delegate marker instead. A
mediated operation classifies its caller: the caller's own pid matching an
announcement decides FIRST, before any ancestor walk, resolving the
main-classifying-itself ambiguity (MM-3-7); then the environment markers;
then the ancestor walk as a cross-check — a marker that contradicts the
walk refuses. Callers with neither marker nor announced ancestry are the
human's own tools and pass untouched. This is stated plainly as
COOPERATIVE discipline (MM-3-3): it prevents accident, not malice — an
agent that strips its own identity to impersonate the human is outside
this trust model, exactly as the benchmark's declared trust model already
treats candidate-written records. Fail-open to humans is the deliberate
cost of never gating the human; the design says so.

**W-3: the write role is a lease with ONE takeover predicate: provable
death (MM-3-2).** The lease file carries holder mainId, pid,
pidStartedAt, claimedAt, renewedAt, generation. Renewal happens on turn
boundaries as hygiene, but expiry does not exist: the ONLY takeover
condition is the census's pid-plus-start-time death check on the holder.
A live holder can never lose the lease; a dead holder loses it instantly.
An uninspectable holder (permission-denied on the pid) is treated as
alive — refusing to take over is the safe error. All lease mutations run
under a dedicated flock on a lock sibling with generation compare — and
the lease directory is asserted local at claim time; a network-mounted
artifacts directory refuses with a plain message, which is the flock
portability contract stated executable (MM-3-12).

**W-4: non-holders are read-only advisors, and the paving is real
(MM-3-1, MM-3-8).** For a non-holder main: dispatch, commit, and arming
refuse, naming the holder and offering the one-command escape hatch —
`scripts/agents/second-session.sh`, which creates a git worktree under a
sibling directory with its own artifacts root and prints the cd command.
The open-work hook in a non-holder session reports OWNED-ELSEWHERE and
never commands. KI-21's incident closes because the second WRITING main
cannot exist in one checkout: the near-double-dispatch, the competing
arming, and the scooped-stage classes all require a second writer that
the lease now refuses. What remains possible — a peer editing files with
its own editor — is outside any harness's sight, stated as accepted, and
mitigated by the worktree path being easier than fighting.

**W-5: death cleanup is the existing reaper, idempotent (MM-3-4, MM-3-6).**
On claiming a dead holder's lease, the new holder runs the existing reaper
sweep before its first dispatch — enforced by dispatch itself: a lease
whose `reapedAfterClaim` stamp is absent refuses dispatch until the sweep
runs. The sweep is already idempotent; a crash mid-cleanup re-runs it. A
dispatch that passed authorization under the dead holder writes a record
whose mainId is the dead holder's; the sweep classifies it like any other
orphan — no fencing window, because there is no live takeover to fence.

**W-6: identity provenance and schema v2 (KI-22), corrected.** The
per-field, per-runtime table: sessionId — claude observed at handshake,
codex observed as thread id, devin OBSERVED (the shipped adapter requires
it; round 3 corrected my table, MM-3-9); model.effective — claude observed
from result telemetry (V-1), codex and devin `unobserved` until their
adapters observe it. ONE literal, `unobserved`, for every unobserved state
(MM-3-10). Return schemas bump to v2 with required schemaVersion; presence
discriminates; claimed values live in `claimed` and never overwrite.

**W-7: protocol errors, keyed and cursored** — unchanged from round 2's
fold: add-if-absent set under the record lock at the adapter lifecycle
point, per-main cursor initialized at announce (idempotent announce keeps
it stable, MM-3-5), advanced after emit, inherited counts printed once at
claim.

**W-8: the config filter has a semantic authority (MM-3-13).** Each
adapter's filter file pairs every excluded key with a one-line
justification citing the CLI's documentation or an observed churn record,
and the file's owner is the adapter — changes ride the loop like any
instruction-bearing file. Unknown keys hash; out-of-range CLI versions
hash everything and warn. The filter's claim stays narrow: it removes
false churn; real behavior changes rightly churn and the refusal names
the changed keys. Per-main config isolation remains recorded future work;
with W-4, a read-only peer that changes shared behavior config is a human
action surfaced by name, not a silent breakage.

## Proof

- Announce twice with one pid+start: one mainId, stable cursor.
- Classification: the announced main's own pid classifies as itself before
  any walk; a dispatched fake delegate refuses; a bare shell passes; a
  marker contradicting the ancestor walk refuses.
- Lease: claim on a live holder refuses regardless of clock; claim on a
  provably dead holder succeeds immediately; permission-denied treats as
  alive and refuses; two contenders on a dead holder — one generation
  winner; network-mounted lease directory refuses with the plain message.
- Non-holder posture: dispatch, commit, arming refused naming the holder
  and printing the second-session command; the created worktree has its
  own artifacts root and its session claims ITS lease independently;
  open-work in the non-holder reports OWNED-ELSEWHERE and commands nothing.
- Post-claim gate: dispatch refuses until reapedAfterClaim stamps; the
  sweep run twice is a no-op the second time.
- Returns and provenance: v1-without-field validates v1; v2 validates v2;
  devin sessionId observed; every unobserved field holds the one literal.
- Protocol errors: double validation, one entry; announce-then-error
  reports exactly once; claim prints inherited counts once.
- Filter: every excluded key carries its justification line or the kit
  check fails; bookkeeping churn keeps identity; a model change breaks it
  and the refusal names the key.

## What is deliberately not changed, and what was deleted

Deleted, not deferred: live-holder TTL expiry, takeover-in-progress
states, adoption scans, and foreign-edit detection — each was a round-3
critical or its direct cause, and each existed only to serve live
two-writer coexistence, which this design no longer promises. The census
and mains registry stay observation-only for everything but dispatch,
commit, and arming. Read-only peers stay legitimate and unauthenticated
reads stay free. The one-main-per-checkout rule stops being an interim
apology and becomes the enforced design.
