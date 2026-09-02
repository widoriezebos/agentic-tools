# Dispositions, round 2: two-bars caller-class design critique

Design revision 2 (job implementer-178d269e0852ac7a8e897657-r2). Critic chain two-bars-cc-crit-3, round 2 (job two-bars-cc-crit-3-r2, gpt-5.6-sol). Round 1 is in plans/two-bars-caller-class-dispositions.md; one table per file because the mechanical join reads exactly one dispositions table.

## Round 2 — 3 material findings, verdictMaterialCount=3 (job two-bars-cc-crit-3-r2)

Fold verification: the critic confirmed all three round-1 findings folded.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| TBCC-R2-LAND-SIDE-DOOR | accepted | Verified: land.sh:244-252 invokes commit.sh without --push, then :265-316 fetch, rebase and push the CURRENT branch to origin (branch from `git symbolic-ref --short HEAD`), so from a job worktree a worker verdict skips the landing bar and land.sh still publishes the agent branch. The design's "a worker commit never lands" premise is not mechanically true while the landing driver runs in a worktree. | Revision 3: land.sh refuses to run inside a job worktree (the same geometry rule as the worker path, step 1), naming the lawful path (land from the main checkout with --chain); the design's non-goals drop land.sh accordingly and a fixture proves the refusal. |
| TBCC-R2-NONRUNNING-WORKER | accepted | Verified: internal/dispatch/record.go:38-52 defines pending statuses apart from running and TerminalStatus covers only completed, failed, cancelled, timeout; custody.go:10-15 and :40-42 permit custody registration while a job is pending. `!TerminalStatus` therefore admits pending, empty and unknown statuses as "the running job". | Revision 3: step 4 requires `J.status == "running"` exactly (the record's own running literal, cited), and the refusal table test adds pending, empty and unknown statuses. |
| TBCC-R2-AMBIGUOUS-MACHINE-LINEAGE | accepted | Verified: commit.sh:355-366 passes the caller's message through and appends the wrapper's trailers, so a message already carrying a Machine trailer yields two; RAN: 42 commits since 00:00 today carry more than one Machine trailer, the hand-written one first. A reader taking the first trailer reads the borrowed identity. | Revision 3: the wrapper refuses a commit whose message already carries any wrapper-owned trailer (Machine, Landing-Provenance, Landing-Provenance-Verdict) with a named sentence, before minting the token; a leg proves it; the fleet note that hand-typed Machine trailers now refuse rides the landing message. |

Trajectory 3 -> 3, all three mechanical-grain (a bounded refusal, a status literal, a trailer rule). Round 3 is the declared failsafe; if material findings remain after it and are all mechanical-grain, the principled exit applies: fold them as fixture obligations and build with a mandatory code critique.
