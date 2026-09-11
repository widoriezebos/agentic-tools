Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry)
Date: 2026-09-11

# Fold brief: revision 5 of the carried-landing design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1d and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising

`metasystem/plans/human-carried-landing-carry-design.md`, revision 4, in
your worktree at commit 4d4dfa54a (sha256
5e4ad956cf52bad3274f69b478ff5fb850a73612a00ee4353edd566cdbbf2f87). Revise
it IN PLACE to revision 5: revision line updated, a revision record
naming each finding and what moved. The read is
`metasystem/artifacts/agents/hcl-context/critique-r2.md` (Sol,
hcl-crit2-20260911): nine findings, all material, two critical; the
coordinator's dispositions at its end are binding and are restated below
as decisions. This is the last design round before the build: what you
write is what gets built. Everything revision 4 settled outside these
nine stands. Read the code at HEAD as before.

## The decisions, one per finding

1. **HCL-C-33 (critical), a fleet-visible reservation.** Before the code
   push, the wrapper publishes the carry in flight to the ledger: `goal
   carrying` writes a `carrying` history row on the goal (the word's
   opid, the workspace tree, the seat, the intended commit's tree) as a
   goal transaction that every seat fetches, in addition to the local
   journal entry. The debt check (08) counts an open `carrying` row on
   any live goal as debt exactly as it counts an open `human-carried`
   obligation, so seat B's landing asks while A is between its push and
   its `carried` row. `goal carried` closes the `carrying` row (the same
   ApprovedRef); an abandoned or expired `carrying` row is closed by
   `goal recover` or by `goal carrying --abandon` (say the rule and the
   timeout). Fixture: the two-seat interleaving (A pushes, B fetches
   and lands before A's `carried` row: B asks with `carry-debt-unpaid`
   naming A's carrying row).
2. **HCL-C-21 (critical), the match rule, second form.** A code word
   `<name>` hits only when O.Code equals `<name>` AND the testing result
   R is sufficient (`R.Delivery.Sufficient` true: no missing group, no
   failing group, no uncovered obligation, no discrepancy;
   `unverified` never matches a code word). A group word `group:G` hits
   only when the ONLY insufficiency of R is G: missing united with
   failing equals `{G}`, no uncovered obligation, no discrepancy, and O
   passes or is one of the four receipt-refusal codes. Everything else
   asks. Fixtures: a verify error under a code word asks; an uncovered
   obligation under a code word asks; a discrepancy under a group word
   asks.
3. **HCL-C-30, supersede rechecks consumption.** Inside the Mutate
   callback, `--supersede <opid>` requires the target unconsumed by the
   COMPLETE predicate: no `carried` row, no superseding row, and no
   `Carry: <opid>` trailer reachable from the code tip since the target's
   anchor (the callback reads the code remote's tracking ref as the
   wrapper left it and says which ref it read). Fixture: the push-before-
   row race — the target's commit is on origin, its row not yet written;
   supersede refuses naming the commit.
4. **HCL-C-26, recovery replays the counselor hook.** Confirmed-carried
   recovery is a named path in recover.go: when the transaction trailer
   is on the ledger and the counselor line is missing, recovery rebuilds
   the `carried` request from the row and replays the AfterConfirmed
   hook (the counselor append) idempotently; it never calls the
   split-only effect. The ledger branch of `landing carry-status` names
   that command (`goal recover` with the entry, or `goal carried
   --repair-counselor --ref <opid>`; choose one) and its payload source
   (the `carried` row's fields). Fixture: crash between the ledger
   publication and the hook; recovery writes the line once.
5. **HCL-C-25, full-payload replay.** The `carried` row stores every
   field the obligation and the counselor line need: commit, project
   tree, workspace, past, battery, missing, failing, judge (mode, tree,
   digest), ledger tip, outcome, by. Replay compares all of them and
   refuses a mismatch naming the field. Fixture: one negative case per
   field (a table test).
6. **HCL-C-03, the blindness fence by owner.** The base judge refuses
   (asks) when the candidate changes any file under internal/landing,
   internal/goal, internal/proofrun, internal/testpolicy,
   internal/behaviorsurface, internal/config, internal/refusal, or
   metasystem.conf or testing.json — the owners of policy and wire
   formats — plus any new file the candidate adds under those
   directories. The table fixture changes one file per owner and asserts
   the ask.
7. **HCL-C-34, execution not ownership.** The groups that own
   internal/goal, internal/landing and cmd/metasystem run every test of
   those packages (or the page names each new test in the group's list;
   choose one and say why), and the contract fixture asserts that the
   plan for a candidate touching each new file executes each new
   fixture's test, not merely that a surface owns the path.
8. **HCL-C-07, accepted-risk replay.** Replay compares `--why` and every
   other immutable input of the carried accepted-risk record; a changed
   why is refused naming the field. Fixture: the changed-why case.
9. **HCL-C-32, one recoverable posture.** After a rebase conflict the
   wrapper runs `git rebase --abort` and then `git reset --soft
   <wip parent>` so HEAD is the wip commit's parent and the index holds
   the candidate; the algorithm text and HCL-05-REBASE-ASKS assert that
   one state (HEAD, index, working tree).

## What must not change in this fold

Everything revision 4 settled outside the nine: points 01 and 03 on the
old page; the two proofs; the word as a workspace tree; one named
refusal or group per word; the review deferred never deleted; the cap and
the debt as asks; one chain; the four seams; the format fence; the seat
binding and `--transfer`.

## Deliverable

The revised page in place, revision 5, with the revision record. Cite
lines at HEAD for everything you add. Wall-clock budget: 45 minutes.
Design only; you implement nothing and run no bed.
