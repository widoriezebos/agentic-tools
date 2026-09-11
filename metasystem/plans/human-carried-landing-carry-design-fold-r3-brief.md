Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry)
Date: 2026-09-11

# Fold brief: revision 6 of the carried-landing design, the last fold

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1d and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising

`metasystem/plans/human-carried-landing-carry-design.md`, revision 5, in
your worktree at commit 316c99b2d (sha256
138f6bb7311dec3c648fb49124234ef7255d9ff25441605b37a25995d4403a04). Revise
it IN PLACE to revision 6: revision line updated, a revision record
naming each finding and what moved. The read is
`metasystem/artifacts/agents/hcl-context/critique-r3.md` (Sol,
hcl-crit3-20260911): four findings, all material, one critical, all
re-opened from earlier reads; the coordinator's dispositions at its end
are binding. The goal's three design reads are spent, so this fold gets
no fourth read: the page you leave is built as it stands, and the
implementation's closing read verifies these four as named checks. Make
each change exact and small; change nothing else. Read the code at HEAD.

## The four decisions

1. **HCL-C-33 (critical), the third debt arm.** The debt predicate has
   three arms and the third is: every proven carry word (terminal or
   channel) on any live or done goal that has a reachable `Carry: <opid>`
   trailer on the code tip since its anchor AND no `carried` row —
   regardless of whether the word is consumed, expired, abandoned or
   superseded. Such a word is a landed-but-unrecorded carry, and it is
   debt until `goal carried` (or recovery) writes its row. Restate the
   open-word definition so it is used only for the cap, never for this
   arm. Fixture: two seats, A pushes and its reservation is (a) abandoned
   and (b) expired before its `carried` row; B's landing asks with
   `carry-debt-unpaid` naming A's word in both cases.
2. **HCL-C-25, the target in the replay.** `goal carried`'s AlreadyApplied
   compares the intent's goal target with the existing row's target
   before any field comparison; a row on another goal with the same
   ApprovedRef is a mismatch, refused naming the goal. The mismatch table
   gains the target case.
3. **HCL-C-03, governance in the fence.** The owner fence includes
   `internal/governance/**` and every compiled package that owns a ledger
   authority wire the base judge reads (say which: at least
   internal/governance, internal/goal, internal/humanauthority); the
   table fixture changes one file in each.
4. **HCL-C-34, execution by owner.** Either split the cmd/metasystem carry
   fixtures by owning surface (the channel command tests under the
   human-authority-and-channel surface's group, the goalsync command tests
   under dispatch-goal-mission's, the landing command tests under
   proof-and-landing's), or attach `carry-landing-standard` and its
   obligation to every surface that owns a command file holding those
   fixtures. Choose one, say why, and make the selection fixture assert
   execution of each fixture group for a candidate touching each owner's
   file.

## What must not change in this fold

Everything else on the page.

## Deliverable

The revised page in place, revision 6, with the revision record. Cite
lines at HEAD for everything you add. Wall-clock budget: 30 minutes.
Design only; you implement nothing and run no bed.
