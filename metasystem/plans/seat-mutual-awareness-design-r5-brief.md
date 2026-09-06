Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-07

# Revision 5 of the seat-mutual-awareness design: three bounded folds

Goal seat-mutual-awareness (tier 3). Contract:
metasystem/plans/goals/seat-mutual-awareness.md. The design on main is
revision 4, metasystem/plans/seat-mutual-awareness-design.md (commit
7f8c13a6), written under Wido's ruling A (operational rollout; no
marker, no repair verb). The fourth review,
metasystem/records/misc/seat-mutual-awareness-critique-r4.md, returned
three material findings, all bounded; nothing else moved. Revision 5
folds exactly those three. Ruling A stands; do not reopen it. Do not
touch what converged.

# The folds

1. SMA-C-25 (reopened). The rollout rule in section 8 admits
   `metasystem supervise status`'s engineBuild field as a confirmation
   that a machine runs the new engine, but that field is the build
   stamp of the command process itself
   (metasystem/cmd/metasystem/supervise.go, around lines 105-159), not
   the enrolled steward or runner; up's re-arm outcome
   (metasystem/internal/up/up.go, around lines 561-605) is the state
   that proves the running reader. Rewrite the confirmation so every
   allowed form proves the running reader's generation: the re-arm
   outcome record of `metasystem up`, or the enrolled running
   steward's generation as the supervision status reads it from the
   enrolled runner; drop the command-stamp form. Add the focused
   fixture the critic's reopening trigger names: rebuilt bytes alone
   cannot satisfy the gate.
2. SMA-C-26. Section 10 makes all of SMA-F-SPOILED-TIP a slice-one
   gate, but its assertion "A's next tick republishes presence" needs
   presence publication, which slice two owns. Assign every assertion
   of every fixture at or after the slice that owns the behaviour it
   asserts: split SMA-F-SPOILED-TIP into the slice-one part (the
   validator refuses the spoiled tip; the human act removes it; the
   directory validates again) and the slice-two part (the next tick
   republishes presence). Do the same audit for every other fixture in
   section 9 against the slice order in section 10.
3. SMA-C-20 (reopened). Section 10 totals the remaining work at
   fifteen reservations and 1,800 job-minutes but asks Wido for twenty
   attempts and 2,400 minutes, because it added five earlier
   reservations that a fresh accounting revision would not count (goal
   set-budget rebinds the accounting revision; projection skips older
   jobs: metasystem/internal/dispatch/budget.go around lines 239-253
   and 344-360). Ask for exactly the declared box (fifteen attempts,
   1,800 minutes, and the elapsed, active-job and review-round members
   that fit it) or name each extra reservation and what it builds.
4. Dispositions table for round 4 at the end of the design: C-25,
   C-26, C-20 accepted, each naming the section that answers it.
   Self-grade updated honestly. Bump the title to revision 5.

# Constraints

Wall-clock budget: 35 minutes. DESIGN-BEARING reach; the deliverable is
the one file, edited in place. Cite only paths that exist under
metasystem/. Gap rule: if a fold needs a new verb, record or marker,
stop and say why with the smallest alternative written out.
