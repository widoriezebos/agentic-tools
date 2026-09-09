Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Review brief: second independent critique of the abandoned-goal design

FINDING IDS: chain-unique, continue the register's series at GAW-08.
Never F-n.

## What you are reading

`metasystem/plans/goal-abandoned-with-a-reason-design.md`, revision 2,
landed at commit 7ae27b7e, sha256
55b979c7b06a8e08afadf0b066af85edc38140eaf318911d518ed7b4dc7e1d83, 1014
lines. Confirm the sha before reading; if it differs, stop and say so.
Revision 2 folds the seven findings of your predecessor's read, which is at
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r1.md`
with the coordinator's disposition, under the fold brief
`metasystem/plans/goal-abandoned-with-a-reason-design-fold-r1-brief.md`.
The revision record at the end of the page names each finding and what
moved. The authoring job reported three gaps of its own: no gate was run
(a page only); the claim that an older engine accepts the new root history
line was read from the parser, not executed against an old binary; and the
meeting with the parked landing-receipt chain is stated from that chain's
design page, not its code.

This is a fresh read of the whole page, not a diff review. A finding that
the fold merely relabelled a defect counts as material. Read the code at
7ae27b7e; every line number the page cites was read at cab73164 or 96c6098b
and the tree has not moved on those files since, but verify, do not trust.

## First on your mandate

1. Section 4a, the closure of the straggler landing. The page's argument
   is: the observation binds the claim revision and `commit.sh` stamps it
   as a Goal-Revision trailer; a new verb `landing held` re-reads the goal
   at the pushed commit's parent before every push; a plain push accepts
   only above origin's tip, so the pair "check the parent, push only above
   that parent" closes the window. Attack this. Is the parent the commit
   the check reads on every land.sh route, including the recertified branch
   and the retry loop (`metasystem/scripts/agents/land.sh` 560 to 591)? Does
   `metasystem/scripts/agents/sync-transport.sh` or any second remote
   reopen it? Does the human-commit exemption (a `+human` actor gets a
   warning and exit 0) hand an agent a path, given how the Machine trailer
   is produced in `metasystem/scripts/agents/commit.sh`? The register
   carriage and exact-revert call sites bind revision 0, "whatever the base
   tree holds": is there a register-carriage landing an abandoned goal's
   straggler could take? And does the page's "no Goal-Revision trailer from
   a non-human actor refuses" wedge every commit made by a seat whose
   engine predates the trailer, and if so is that the rollout section's
   problem or a new one?

2. Section 4a, the transition. It takes the goal-revision lock dispatch
   holds (`metasystem/internal/goalrevision/lock.go`) for every claimed goal
   in the abandoned set, then re-reads the tip inside the transaction. The
   page admits the lock is checkout-local and says "across checkouts, the
   state is the guarantee". Read that paragraph against
   `metasystem/internal/dispatch/admission.go` and say whether a dispatch
   on the claimant's checkout can be admitted against the claim between the
   human's read on another checkout and the publish, and what happens to
   that job.

3. Section 6, reopen. The enrolled proof now comes through
   `proveGoalHumanAuthority` (`metasystem/cmd/metasystem/goalsync_mutations.go`
   around line 599). For a record with a frozen fence the page calls
   `VerifyStopBatchComplete` (`metasystem/internal/goal/stop.go` line 249)
   and says the fenced reopen "runs where resume would have run, on the
   claimant's checkout". Say what that means for a human at a different
   machine, whether the page's rejection of the keep-the-fence alternative
   holds, and whether a reopen can ever produce a queued record that still
   carries stop authority.

4. Section 10, rollout. The gate is a new root History line, verb word
   `engine-floor`, minted by a new human verb, plus a refusal when any
   armed checkout visible on this machine is below the floor. Read the root
   history parser (`metasystem/internal/goal/root.go`) and
   `metasystem/internal/goal/file.go` around lines 1303 to 1421 and say
   whether an engine at cab73164 accepts a history line with that verb word
   and those keys, or refuses the tree. This is the designer's own stated
   gap; close it by reading, or say the reading cannot settle it and what
   would.

5. The census and the third map (sections 3 and 5). Rule 1 of your
   predecessor's read was an incomplete census. Redo it: every reader of
   `TreeGoals.Done` and every place that decides a goal is archived, at
   7ae27b7e, against the page's table. Name any reader the table still misses.

6. The fold itself. For each of GAW-01 to GAW-07, say closed, narrowed or
   relabelled, in one line each, with the page section that does it.

## Then the rest

Read every other section as you would a first revision. The grammar,
validation rules, prune, recovery and the fixtures all changed in the
fold; the specimens in section 12 must exercise what the page now claims.

## What you may not do

Do not propose a different design; find where this one is wrong,
underspecified where the builder would have to guess, or unprovable by its
own fixtures. Your sandbox is read-only; write the register in your return
and the coordinator carries it. Wall-clock budget: 60 minutes.
