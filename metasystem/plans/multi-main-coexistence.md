# Multi-main coexistence: two sessions, one repository, no interference

- Goal and current status: two main agents in one checkout can no longer interfere — streams carry mechanical owners, the working tree has one writer, the turn-end hook commands only work that is yours to start, and a delegate's identity claim can never overwrite what the adapter observed. Closes KI-21 and KI-22. Status: DRAFT, awaiting critique.
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

**M-1: streams have mechanical owners.** Every plan gains one line the
tools own: `Stream-Owner: <session-uuid>` (the runtime session identifier
the harness already records in the mains registry). The turn-end open-work
check resolves ownership before commanding anything:

- Owned by this session, or unowned: commanded exactly as today.
- Owned by ANOTHER session that is alive per the mains registry and a
  liveness check: reported as `OWNED-ELSEWHERE <plan>: session <id>` —
  informational, never a command, never a block.
- Owned by a session that is dead: claimable. The claim writes a takeover
  line into the plan (old owner, new owner, timestamp) so succession is in
  the record, then proceeds as owned.

The orchestrator sets the owner line when it opens a stream; a plan without
the line is unowned legacy and the first session to touch it claims it.

**M-2: the working tree has one writer.** A claim file,
`artifacts/agents/mains/worktree-owner.json`, holds the owning session's
uuid and pid, written by the first main's start hook and renewed on its
turns. Enforcement, both fail-closed:

- Dispatch refuses a job dispatch from a session that is not the working
  tree owner while the recorded owner is alive, naming the owner and the
  two remedies: work read-only, or take over explicitly with a new
  `--claim-worktree` acknowledgment that requires the recorded owner dead
  or the human's typed approval on a TTY (the F-4 escalation pattern,
  reused).
- The pre-commit guard extends beyond new plan files: it refuses any
  commit from a non-owner session while the owner lives, same override.
- A second main can always read, inspect, and answer questions — exactly
  what the peer session did well. It cannot dispatch, commit, or arm.
- Supervision arming by a non-owner is refused the same way; the peer
  almost re-armed under the owner's live `--wait`.

**M-3: liveness, not prose.** The open-work check already derives in-flight
from job records; the stale "In flight right now" prose that misled the
peer stops being load-bearing entirely: the hook's ownership and in-flight
answers come from the mains registry, the claim file, and job records. The
prose lines stay for humans and are labeled as narrative by the plan
template.

**M-4 (KI-22a): the adapter's observation always wins.** Return
normalization sets `sessionId` and `model.effective` from what the adapter
observed at handshake and result time; a delegate-claimed value that
disagrees is recorded in a `claimed` sub-field for the record, never in the
canonical field, and cannot fail the identity check that burned six rounds.
The brief template's orchestrator line drops the word "session"
(`Orchestrator: af965b26` reads fine and stops inducing the echo).

**M-5 (KI-22b): detected protocol errors surface where decisions happen.**
When a follow-up is dispatched onto a chain whose newest round ended in
protocol_error, dispatch prints one plain line naming the round and the
violation before proceeding, and the chain root record accumulates a
`protocolErrors` counter. The turn-end report includes any chain whose
counter grew this turn. Detection that nobody sees is the failure mode this
whole day kept finding; this is the general fix for the identity-mismatch
instance.

## Proof

- Fixture: two fake mains registered; the open-work check commands the
  owner, reports OWNED-ELSEWHERE to the peer, and permits takeover only
  after the owner is dead — with the takeover line written.
- Fixture: dispatch and commit from a non-owner refused while the owner
  lives, naming the owner; permitted after death-plus-claim.
- Fixture: arming by a non-owner refused.
- Fixture: a return claiming the orchestrator's session id normalizes to
  the adapter-observed id, the claim lands in `claimed`, the round is not
  a protocol error, and a synthetic mismatch in the adapter-observed value
  itself still fails.
- Fixture: a follow-up onto a protocol_error round prints the notice and
  bumps the counter; the turn report names the chain.

## What is deliberately not changed

The mains registry and census stay observation-only for anything that is
not dispatch, commit, or arming — a read-only peer is legitimate and
useful, as the transcript itself demonstrated. And the one-main-per-
checkout INTERIM rule stays in force until this design is implemented and
proven; this design is what makes the rule enforceable instead of polite.
