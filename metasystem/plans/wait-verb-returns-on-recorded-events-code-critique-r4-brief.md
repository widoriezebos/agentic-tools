Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Review brief: the last round of the fresh root, after the seat's fix of WVB-52

FINDING IDS: chain-unique; WVB-40 to WVB-54 are taken. WVB-52 is returned
under its OWN identifier with `material: false` if resolved, `material:
true` if it still stands; a new defect gets WVB-55 onward. Never report a
resolution as prose inside another finding.

Why this review exists: the previous round resolved WVB-48 and found
WVB-52: reading every renewal from the original cursor brought back the
read-cost growth WVB-27 removed, so a human-act wait on a busy ledger
could never be renewed. The seat fixed it in the chain's worktree, which
the dispatcher snapshots for you. A renewal reads from the row's last
checked tip again, as the running wait does (`renewSavedWait`). The last
checked tip a new row records is the cursor before the observation when
the registration's first observation already matched
(`floorAfterObservation`), and the failure exits of a registration and
of a renewal keep that floor when their observation matched
(`tipAfterFailure`); a pending observation advances the tip as before.
So a matched event whose result was never published stays above the
floor, and a wait that advanced past many accepted changes renews with
one read from where it stopped. Three subtests of
TestWaitSavedResultRenewalClasses cover it: the renewal after a matched
registration that failed reads from the kept floor and returns the
event; the renewal of a wait that had advanced to a later tip reads once
from that tip and never from the original cursor; a registration whose
first observation matched and whose delivery then failed saves the
cursor before the event as the last checked tip.

Round budget: 1 focused round, the last of this root. A finding is
material only if an implementer must change the code or tests before
this lands, and it names the artifact it would change.

Contract: the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md
(section 2) as landed.

# Mandate

1. WVB-52: is it resolved as stated and tested? Return it under its
   identifier.
2. The fix's own reach: is there a path on which a matched, unpublished
   event sinks below the floor a renewal reads from (a running wait's
   matched observation whose result write fails, a renewal whose first
   observation matched and whose delivery then failed, a registration
   whose first observation matched), and does any path still read from
   the original cursor after the first registration?
3. Nothing else.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
