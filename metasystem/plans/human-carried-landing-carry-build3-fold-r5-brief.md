Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry, fifth round on chain hcl-build3-20260911)
Date: 2026-09-11

# Fold: one leg, the coordinator's own error in the last fold

The host ran round 4's three host-only legs: `carried-prefixed` passes,
`carried-second` passes, `carried-crash-local` fails at the rerun with the
workspace ask: "word workspace=88f023cc… candidate workspace=4f357661…;
run goal carry … --supersede <word>" and "local recovery did not finish".
The cause is the previous fold's decision 2, which told you to move origin
before the rerun with an unrelated CODE commit. A code move under a word
changes the workspace projection, so the wrapper correctly asks for a
superseding word (design 05 step 4, HCL-04-SECOND-WORD-LANDS, a recorded
follow-up). The recovery the leg proves, HCL-19-RECOVERY-KEEPS-REBASED-COMMIT,
keeps the rebased commit when origin moved by a LEDGER commit, which the
projection excludes. Change nothing else in this round.

# The one decision

In `carried-crash-local`, between the kill and the rerun, origin moves by
a ledger commit, not a code commit: the peer seat runs one goal verb
against its own goal (for example `goal edit --id fx-b --next "…"` with
its lineage, as the two-seat legs do) and that transaction lands on
origin's main, which is the fixture's ledger branch. Keep every check of
round 4: the pushed commit's parent is the moved origin tip; its author
date equals the crashed local commit's; the `Carry:` trailer and the
`Landing-Provenance` line equal the crashed commit's; the crashed sha is
on no branch; the expected rebased tree; exactly one rendered `carrying`
row (open) and one `carried` row (landed); one counselor line. If the
rebase onto a ledger move changes anything the checks compare, say so
under `deviations` with the exact line, and do not weaken the check.

# Proof before you return

`bash -n scripts/agents/land-fixtures.sh`; `git diff --check --cached`.
The leg needs the host; say so and return. Return within 20 minutes.

# Return

`diffBoundary` and `files` are repository-root paths; under `evidence`
the commands with their observed result; under `deviations` anything the
decision could not hold, with the line.
