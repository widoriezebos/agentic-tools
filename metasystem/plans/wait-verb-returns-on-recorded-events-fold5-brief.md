Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 6 of the build of member 1, on Wido's word this morning: fold the
three findings of the rostered read (WVB-40, WVB-41, WVB-42), then the
seat proves the round, a second rostered read follows, and the member
lands. The goal, the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md and the
return schema are those of round 1's brief; the workspace is round 2's
widened one. Build on the worktree as round 5 left it (it carries the
seat's fix: a matched event wins over a frontier change in the same
cycle); do not commit.

# The three findings and the seat's decisions

WVB-40 (medium). `metasystem wait --resume WAIT-ID --timeout D` on a row
that holds a saved result replays the result and registers nothing, so
the page's renewal rule cannot happen after a 124, a 130, a 6 or a 65.
Decision: a saved result whose exit is a source terminal (0, 1, 2, 3, 4)
is replayed and never renewed; a saved result whose exit is 124, 130, 6
or 65 is renewable: with an explicit `--timeout`, resume starts a
recorded renewal under the same key, with the saved selector, the
ORIGINAL cursor as the match floor, the saved last checked tip, a new
nonce, a new deadline and a fresh registration observation; the row
keeps the previous result in a `renewals` history (result, renewedAt).
Without `--timeout` the saved result is replayed as today. Test every
exit class both ways, including the reboot case of section 4 (a plain
resume after a changed boot returns 124; the renewal that follows
registers).

WVB-41 (medium). A landing wait never checks the landing destination
against the ledger branch it watches. Decision: at registration the
waiter resolves the code destination (the branch land.sh pushes to: the
checkout's configured landing branch, falling back to the checked-out
branch) and the ledger endpoint's branch; when they differ, registration
refuses with exit 4 and a named reason ("landings go to <code branch>;
this ledger endpoint watches <ledger branch>"), never watching HEAD or
another branch instead. In remote mode both are the configured branch
and the wait proceeds. Local-mode fixtures that want a landing wait
publish their landings to the dedicated local ledger ref, as the page
says; the wait-ledger bed leg is adjusted so its local-mode case either
expects the named refusal or lands on that ref. Test the refusal and the
remote-mode acceptance.

WVB-42 (medium). The actionable check runs the whole report scan every
ten-second cycle outside the wait's budget, through local Git processes
with no timeout. Decision: the check needs no Git. The open-work
signature is computed from the plan files' open steps alone (the same
digest the stop gate uses, without the ledger load), and the claimable
set comes from the observation's fetched tree, as round 5 already does.
Every read the check makes takes the wait's context, and the check
spawns no process; a test counts zero Git invocations per cycle and
asserts the check returns within the ten-second ceiling with an injected
slow reader. Registration's scan follows the same rule.

# Constraints, Expected Return, Acceptance Criteria, Gap Rule

As in round 1's brief. Wall clock: 60 minutes; a partial round returns
with its tests green for what exists and names what is left.
