Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Review brief: the third rostered read, granted by Wido, after the seat's fix of WVB-48

FINDING IDS: chain-unique; WVB-40 to WVB-51 are taken. WVB-48 is returned
under its OWN identifier with `material: false` if resolved, `material:
true` if it still stands; a new defect gets WVB-52 onward. Never report a
resolution as prose inside another finding.

Why this review exists: round 2 resolved WVB-40, WVB-41 and WVB-42 and
found WVB-48: a renewal read from the saved last checked tip instead of
the original cursor, so an event found by a registration that then ended
with a renewable exit was missed for good. The seat fixed it in the
chain's worktree, which the dispatcher snapshots for you: the renewal
observes from `renewalFloor(row)`, the original cursor for a ledger wait;
the failure exits of a registration and of a renewal save the tip from
before the observation when the observation already matched
(`tipAfterFailure`), so a matched event never sinks below the floor; a
subtest of TestWaitSavedResultRenewalClasses saves a ledger wait whose
registration matched and then failed, renews it, and asserts the renewal
reads from the original cursor and returns the event.

Round budget: 1 focused round, the last. A finding is material only if
an implementer must change the code or tests before this lands, and it
names the artifact it would change.

Contract: the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md
(section 2) as landed.

# Mandate

1. WVB-48: is it resolved as stated and tested? Return it under its
   identifier.
2. The fix's own reach: does reading a renewal from the original cursor
   reintroduce the observation-cost growth WVB-27 removed for the running
   wait (say whether a renewal's one read from the floor is bounded as the
   page's registration read is), and can `tipAfterFailure` ever save a tip
   older than the row already had?
3. Nothing else.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
