Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

The closing review of the fleet conversation channel design, tier 3 under
R-54-m1: after this the build starts. Read
metasystem/plans/fleet-slack-channel-design.md revision 2 and your own
round-1 findings as landed in
metasystem/records/misc/fleet-slack-channel-design-critique-r1.md
(FSC-R1-001 to 008 with the orchestrator's dispositions). Revision 2 folded seven as proposed and
AMENDED 004 (§12 of the design lists the dispositions). Scope is the
folds: whether each fold actually closes its finding as revision 2 states
it. The parts round 1 did not find against are agreed and build as
written; do not re-review them.

# The standard for a finding (R-60-m1, binding)

A finding is material only if it changes what gets built AND names the
artifact. This is the closing review: at its end every disputed point is
a named test obligation the build must satisfy, never a raise for another
review. Zero material findings is a closing answer if the reading
supports it.

# Read these folds against these questions

1. §5 lock and consumption (001): does the flock plus the consumption row
   written before attribution make two concurrent polls unable to attribute
   one step twice? Is the "same inbound ref" exception safe (it exists so a
   resumed phase is not judged a replay)?
2. §5 phase machine (002): with the op id allocated at MATCHED and the
   goal transaction engine treating a repeated op id as a no-op, does a
   crash after each of the four phases followed by a re-poll yield exactly
   one history operation and eventual closure? Is the rule "cursor
   persisted only after every envelope's disposition is durable" enough for
   the destination-wide cursor, given Poll's work budget in §7 (five
   dispositions per pass, the rest carried)? Name the gap if the budget
   can strand an envelope between a persisted cursor and an unjudged ref.
3. §5 grammar (003): does extending `ParseHistoryLine`'s authority
   validation to accept `AUTHENTICATED_CHANNEL_WORD` with four proof keys
   keep the strict token in the reason field where `RecordedNormApproval`
   scans, with no other grammar change?
4. §2 receive and cursor as acknowledgment (004, amended): given
   `Receive(threads, after)` returning envelopes with thread correlation
   and a destination-wide cursor persisted after disposition, is the
   adopted alert §2b checkpoint law satisfied for Slack now and for
   Telegram's getUpdates offset later without changing the interface? If
   an acknowledge operation is still required, name the concrete case the
   cursor rule cannot cover.
5. §3 spend (005): units per runtime from internal/usage, no dollars —
   does a durable source exist for what is now claimed?
6. §7 pass bound (006): one 15-second context for the whole channel phase
   plus the work budget, placed last in both tick drivers — does anything
   in the drivers still wait on it?
7. §5 consumers (007): `goal resume --approved-ref` and `goal
   set-obligation --approved-ref` validating an AUTHENTICATED_CHANNEL_WORD
   operation on the same goal; set-budget and enroll-terminal unchanged.
   Is the scope exactly the R-32-m1 set and is the horizon independence
   stated where a builder will read it?
8. §2 Credential (008): readiness only; the human check is §5's user id.

# Return

Code-critic schema. Findings first, each naming the section, the
artifact and the one-line change. Then one line: "agreed parts build as
written" or the disputed points as test obligations by name, added to
the design's §8 list.

# Constraints

Wall-clock budget: 30 minutes. Your sandbox is read-only; verify by
reading. R-31: no benchmarks. No redesign; the alert-channel sections are
decided.

# Gap Rule

stop and report a gap; never fill it silently.
