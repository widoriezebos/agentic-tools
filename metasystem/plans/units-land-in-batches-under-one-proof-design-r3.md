# Design: units land in batches under one proof (r3)

Goal: units-land-in-batches-under-one-proof (tier 3, priority 1, sequence 1). Revision 3, the last fold
round, 2026-09-16. Author: Claude Fable 5.1 design delegate, headless, seat m1e. Citations are origin/main
8c95ad54f read in worktree crit-blb, paths under metasystem/ unless absolute. Inputs, in
rank: Wido's answers after critique r2 (W1 lane 4 by hand as Go verbs, no scripts; W2 fully automatic,
agent commits named for the goal's approver since the attribution amendment, no human act from join to push; W3 the bash launchers land now, the next goal ports them to
Go); the r2 page; the Codex critique r2 (revise 12); the r1 answers (Q1 diagnostic runs exempt from the
lock; Q2 a dedicated landing checkout; Q3 best-effort quiet window); DONE (1)-(5); the r3 substrate
plans/proof-admission-fits-a-seat-proving-several-units-design.md; the seat constraints C1-C13. Rules: one
witness and one mutation per rule; injected clocks and probers (R-104-m1e, rulings.md:163); no narrowed
DONE (R-115-m1e, :174); no tmux (R-114-m1e, :173); no retry (R-103-m1e, :162); one engine run under the
mkdir lock, focused runs exempt (R-111-m1e, :170). Where r2 conflicts with the shape, the shape wins.

## Lane 4 by hand, and the verb for each step

Lane 4's record is the seat scratchpad landing-lane-4-report.md. Its landing was a human commit from the
pane (lane3-land.sh), so it never met the agent commit boundary; the automatic path must (W2). The SEAT
is the coordinator that built the unit; the OWNER is `landing batch owner`, one headless main per landing checkout
(D5).

| Step by hand | Verb | Runs as | Where |
|---|---|---|---|
| 1. take queued units whose read is LAND | `landing batch join --goal G --chain J` (D2, D3): the unit is its closed chain; the join runs the cheap gate itself | the seat | seat checkout; record in the landing root |
| 2. apply in join order on the trunk tip, refuse on conflict, record each prefix tree | inside `join` (D4): `git apply --3way` in a private worktree; `BATCH_JOIN_CONFLICT` names the files | the seat | landing root worktree |
| 3. run the fast gate on the stacked tree | `seal` step of the owner (D4): go-gate.sh --fast and the changed packages' tests on the tip | the owner | landing root worktree |
| 4. take the lock, arm the engine, plan the union under a claim, run one deep proof | `prove` step of the owner (D5, D6): lock, `test plan`, `test run --purpose delivery` under the owner's claim | the owner | landing checkout |
| 5. green: land each unit as its own commit in join order with its receipt row | `land` step of the owner (D8): commit.sh per unit as the lease-holding main, one `landing held`, one fast-forward push, re-arm, release | the owner | landing checkout |
| 6. red: failing group on the base alone; red on base is a trunk flake; else eject the owner, re-prove the rest | `diagnose` step of the owner (D7): base run, named set, serial fallback, eject, new tree | the owner | landing root |

No human act remains from step 1 to the push; the seat's only act is step 1.

## 0. Substrate and prerequisites

Consumed from the r3 substrate: U1a (attempt schema 3) so `batchMembers` is a schema-3 field, and U4
(candidate-scoped retry, attempt.go:826) so an earlier red under the authority goal demands no retry
decision for a new tree. Not consumed: U3a-U3c (`--authority`): the batch proof runs under the owner's
own claim, lane 4's shape (its proofs ran under the seat's claim of an unrelated goal, report lines 12
and 31), members are charged through `batchMembers` (D6). No r3 unit is landed at 8c95ad54f, so D9's refusal holds until U1a and U4 land.

## 1. The problem, with evidence

One 16 to 45 minute proof per landing under one lock; a 2 s trunk flake cost lane 4 a
53-group slot; stacks are hand-assembled from a markdown queue; m1b's race (rearm_on_landed.go:224-227
refused `TEST_POLICY_ENGINE_REQUIRED` on dirty receipts.log). New at r3: the hand lane's commits carry no
`Landing-Provenance` lines, so `wait --goal X --event landing` never returns on them (attention.go:988;
m1b 2026-09-16). And every current unit is built by the scratchpad launchers (a codex-rescue task id such
as `task-mu3h0tkn-wrazoe` in U2c-build.job), outside the job domain: no `artifacts/agents/jobs/<root>.json`
implementer record exists, so no agent commit can consume it today (observe.go:340-344, 398).

## 2. Decisions

D1. Batch record, one lock, append-only history, prober seam (C13, C12, C8). Records live at `<landing root>/artifacts/agents/landing-batches/<id>.json`, id a lowercase operation
ULID (identity.go:11-19); every mutation runs under one flock `artifacts/agents/locks/landing-batches.lock`
(`AcquireMutation`, attempt.go:255) with atomic replace. Schema 1:
`batchId`, `baseCommit`, `baseTree`, `tipTree`, `units[]`, `selectedGroups`, `state` (`open`, `sealed`,
`proving`, `diagnosing`, `landing`, `landed`, `held-trunk-red`, `dissolved`), append-only `history[]`
(`at`, `verb`, `from`, `to`, `actor`, `detail`), `seal`, `attempts[]`, `owner` (`ProcessIdentity`,
record.go:26-33, and `actor`), `openedAt`, `deadline`. Unit entry: `goalId`, `chain`, `claim` (machine,
lineage, revision from the goal file at join), `certifiedDigest`, `changedPaths`, `unitTree`,
`selectedGroups`, `fixtureGroups`, `gate` (D3 run ids), `joinedAt`, `state` (`joining`, `joined`,
`withdrawn-budget`, `ejected`, `landed`), `failure`. Every liveness decision (owner, sealer, launcher) calls `identity.AliveRef(batchSeams.prober,
ref)` where `batchSeams.prober` is an injected `identity.Prober` (internal/identity/identity.go:175-181)
defaulting to `KernelProber{}`; no batch file names `KernelProber` outside that default. Goal-side mark: label `batch-<id>` (32 characters, matching `^[a-z][a-z0-9-]{0,31}$`, goal.go:424),
written by the joiner through `goal edit --label` (goalsync_mutations.go:671), the claim holder's own act
(verbs.go:2704 refuses another's). Consumers: the commit boundary's delegation (D8) and the
one-live-batch-per-goal rule (D2). Collision: ids are minted, never chosen; a live `batch-` label refuses
a second join; a label of a `landed` or `dissolved` batch authorizes nothing and is replaced by the
next join. Ordering: the entry is written `joining` under the lock, then the label; it becomes `joined`
when the joiner's fetch shows the label commit on origin. Crash after the entry: the owner's reconcile finds no label on the base tree and drops the entry
(`join-incomplete`), the seat re-joins; a label with no live batch is replaced. Rejected: no mark (the
commit boundary would then trust a landing-root file for the holder's delegation).

D2. Unit identity is the implementation chain; the join takes no diff and no attestation (C1, C3). `landing batch join --goal G --chain J` reads the chain from the seat's job store:
`artifacts/agents/jobs/J.json` must be a root (`parentJob` absent), `role` implementer, `chainClosed`,
`goalId` G at `goalRevision` equal to G's live claim revision, `destructiveReach` DESIGN-BEARING or
DESTRUCTIVE-REACH, with `independentCritiqueJobRef` naming a closed code-critic root (observe.go:340-398;
recertification.go:340-357); the diff is the chain's certified output (`chainCertifiedOutput`,
observe.go:593), its digest the entry's `certifiedDigest`. The unit is (G, J). Under the lock the join scans every live record: J anywhere refuses `BATCH_UNIT_ELSEWHERE`; G in another batch refuses `BATCH_GOAL_ELSEWHERE`; G in this batch with another chain is accepted
(lane 4 landed U2c, U2d, U2e together). Aliasing is closed because the caller supplies no bytes: a rewritten patch is a different chain or
fails `bindCertifiedChange` at commit (observe.go:869). DONE (1)'s two attested results resolve here: the mutation witnesses are the implementer's certified return, the independent read the closed independent critic chain; both are what the commit boundary consumes (C6). Transport: the join copies the root, its members, the critic chain and their artifacts byte for byte into
the landing root's job store under the batch lock (`BATCH_CHAIN_CONFLICT` on differing bytes);
observeChain reads them there (observe.go:340). A unit built outside the job domain (section 1)
has no chain: `BATCH_JOIN_CHAIN_REQUIRED` names the prerequisite, Wido's next goal after
headless-launchers-live-in-the-repo, which ports the delegate launchers into Go verbs (W3); until then
such units land through the hand lane. No second evidence path is invented.

D3. The cheap gate is run by the join itself, outside the lock (C1, C9, DONE 1). In a private worktree at base plus the certified patch (`NewDetachedCommitWorktree`,
gittree/detached.go): (a) `go-gate.sh --fast` (gofmt, vet, staticcheck, refusal register, build; go-gate.sh:10-13);
(b) `go test -count=1` per changed package; (c) each fixture group of the unit as `test run --purpose
diagnostic --groups <G> --mode canary --tree <unitTree>` (test.go:154-156), exempt from the lock (Q1). Fixture groups: per changed bed (a script under scripts/agents that a section function lists) the owned map scripts/agents/fixture-bed-groups.tsv gives the group (W4b);
a changed shared harness file (fixture-bed-scenarios.sh, fixture-budget.sh, fixture-assert.sh) is
`covered-by-proof`. The whole group is a superset of the changed scenario, so DONE (1) is met; r2's
hunk-to-arm derivation is dropped (`ChangedLines` is a count, gittree/numstat.go:10-23; no hunk-range API
is owned). The entry records the run ids the verb wrote; a red step refuses `BATCH_JOIN_GATE_RED` naming
it; a step not run cannot be named, so a missing result refuses the join. DONE (1)'s five: (b), the
certified return, the critic chain, (a), (c).

D4. Assembly in join order; nothing is carried to the tip (C2, DONE 2). Join applies the patch with `git
apply --3way` onto `tipTree`; a conflict refuses `BATCH_JOIN_CONFLICT` naming every path, tip unchanged.
Seal re-runs (a) and (b) of D3 on the assembled tip (lane 4 step 3, 13.6 s), refusing `BATCH_SEAL_GATE_
RED`; fixture groups on the tip are executed by the proof itself, which plans the union on the tip and
reuses a group only when its execution identity, a digest over its declared inputs, is unchanged
(testing.go:150-153; test_result.go:35). An earlier unit changing a shared input of a later unit's group
changes that identity, so the group executes on the tip (W4c). Trunk moved at seal: fetch, re-apply in join order onto the new base, eject a conflicting unit
`trunk-conflict`, re-run the gate on the new tip. The proved tree is the tip's `CandidateTreeDigest()` (r3
U1a). Amendment (Wido, 2026-09-17): the assembled series is the branch `landing/<batch-id>`, committed in
join order from the base and pushed to origin under a lease before the proof, and the attempt names its tip
commit beside the tree; see the amendment paragraph before D8.

D5. Size, start rule, best-effort quiet window, census, and the landing owner (C13, C6, Q3, DONE 3). 5.1 The union closes the batch at `deep-ceiling` (select.go:152-157); later joins refuse `BATCH_CLOSED`;
joins from `sealed` on refuse `BATCH_SEALED`. 5.2 Start when the lock is free (owner absent or dead by
the prober, testrun-lock.sh:37-41) and (two or more joined units, or the oldest `joinedAt` plus
`landing.batch-max-wait`, default 45m, has passed). Before the maximum wait the start also needs a quiet
sample: `!Loaded()` (attemptload.go:59-65) and `OverlapKnown && OverlappingHost == 0`; unknown is not
quiet. At the maximum wait the proof starts regardless (Q3), recording `quietWindow: expired`, the sample and
the launcher count. A red that load explains is a test defect fixed at its cause, never a hold, retry,
eject or raised bound. 5.3 Census: `isProofLauncherArgv` (attemptload.go:158-160) also recognises `test
run` and `landing batch owner`. 5.4 The owner: `landing batch owner` is a headless verb (never tmux, R-114-m1e), one per landing
checkout, spawned detached (`Setsid`, internal/supervise/proc.go:78) by a join that finds no live owner by
the prober. It announces itself as a main (`lease announce --owner-lineage <its main id>`, verbs.go:46, 190) and
exports `METASYSTEM_OWNER_LINEAGE` as that id; `RequireHolder` claims the unheld landing checkout for an
authenticated main (verbs.go:420-470; an unannounced headless caller is UNTRUSTED, classify.go:387-391),
so the owner holds lease and lineage for its whole life; a second owner exits on `ownedElsewhere`. Its loop takes an injected `Clock` and a wake channel
(SIGUSR1 from join and eject, a 30 s tick); each wake re-reads records, samples through `loadSeams`
(attemptload.go:36-39) and evaluates 5.2. Start: seal (D4), take the mkdir lock with owner line "batch pid start batch:<id>" (testrun-lock.sh:13-17),
claim the authority goal, prove (D6), then D7 or D8, release claim and lock. Its actor is `<landing
nickname>+<main id>` (commit.sh:582-585).

D6. Proof under the owner's claim; every member charged (C5, DONE 5). The authority goal `batch-landing-authority` is opened and approved at rollout (R-119-m1e), tier 1,
budget `attemptLimit` 12, `reservedJobMinutesLimit` 900, `elapsedLimit` 1d, `activeJobLimit` 1; the owner claims it at each batch start and releases it at the batch's end. The proof
is `test run --goal batch-landing-authority --tree <tip> --mode auto --purpose delivery`, admitted because
the caller is the holder main whose claim epoch matches the binding (proof_run.go:495-500). The attempt record gains `batchMembers[]`: `(goalId,
claimRevision)` of every sealed member (schema 3, r3 U1a). The projector counts an attempt for goal X when
`attempt.GoalID == X` (budget.go:517-521) or `batchMembers` names X at its current claim revision: the
attempt and its reserved, then observed, minutes in full. The batch proof charges every member; D7's base run has empty `batchMembers` (the authority goal only);
a serial proof names only its unit. Seal admission: every member shows, by `ProjectBudget`, headroom of two attempts and twice the reserved
minutes (the batch plus its own serial proof); a member without it is `withdrawn-budget`, proved alone later as a batch of one; the rest seal as a new
tree. Join pre-checks the same lens. Each re-assembly is a new candidate tree, so under r3 U4 no retry
decision is demanded. Rejected: head-only charging (r1); r3 `--authority`.

D7. Red: base first, then the named set, then serial order (C4, DONE 4). Named set N: sealed units whose `changedPaths` meet a failing group's `InputManifest`
(test.go:1233-1241) or a surface selecting it (select.go:118-127). Every diagnostic run is `test run
--purpose diagnostic --groups <failing> --mode canary --tree <T>`, exempt from the lock (Q1), bound to the
record before the next starts. Step 0: the failing groups on `baseTree`; any red is a trunk flake: `held-trunk-red`,
nobody ejected. Base green and N empty: `held-trunk-red` (DONE 4 literally). |N| = 1: eject it with the failure (attempt id, group, log path, failing names); the survivors re-apply
in join order as a new tree, proved as a new attempt. |N| >= 2: prove the unnamed units as a new tree (charged to them); green lands them (D8); then each
named unit in join order alone on the resulting tip (charged to it): green lands, red is ejected naming the landed partners. This is serial landing's order,
so a unit is ejected only when red against the trunk it lands on. Bound for N units and k = |N| named: at
most k + 2 proof runs and one base run; the batch and the unnamed re-prove are full proofs, the serial
ones reuse by identity. Hold: group, log path, end sample, one line in memory/flake-registry.md
(docs/flake-registry.md:17), members told by history line; reopen only on a new tree.

Amendment, the landing branch (Wido's ruling, 2026-09-17, recorded as rule 6.9 of
plans/goals-live-on-branches-design.md). The seal (D4) composes the batch on its own branch,
`landing/<batch-id>`, starting at the base: one agent commit per unit in join order, each with its receipt
row and the trailers D8 names, no merge commit; the tip's `CandidateTreeDigest()` stays the proved tree, and
the attempt and the record name the tip commit as well, so the candidate is a commit any enrolled host can
fetch. The branch is pushed with a lease, created against an absent ref and replaced against the tip last
pushed; a ref moved behind the lease refuses `BATCH_LANDING_BRANCH_MOVED` and writes nothing. D7's ejection
rebuilds the branch from the survivors in join order under that lease: the ejected unit's commit is not
re-applied, which is the ruling's reason, and the new tip is the new candidate; trunk moved at seal or
between green and push rebuilds it the same way from the new base. D8 then lands by one fast-forward of the
endpoint to the landing tip under the lease of the base, the whole batch or nothing, and deletes the branch
against its tip; a landing given up (`dissolve`, `held-trunk-red` resolved by a trunk fix) deletes it the
same way, and a `landing/` branch whose id names no live batch is listed by the sweep of
goals-live-on-branches unit 9 and deleted on the word. Per-unit commits, trailers and receipts (D8, BA13,
BA14b) are unchanged: the commits that reach the endpoint are the branch's. Witness: BA12b (ejection
rebuilds the branch, the ejected commit absent from the new tip), BA14a (a moved endpoint refuses, a moved
landing tip refuses, the fast-forward lands all or nothing and the branch is deleted), BA13 (the attempt
names the tip commit).

Amendment, attribution (Wido's ruling, 2026-09-17, recorded as rule 6.10 of
plans/goals-live-on-branches-design.md: when a human in the loop granted the authority, the commits name
that human). W2's "agent commits" keeps its meaning, the commit.sh boundary with its chain, landing
observe and the receipt; it no longer implies a machine identity. D8 runs each unit's commit.sh with
author and committer set to the approver of that unit's goal, the ledger's `Approved.By`, whose git
identity the landing checkout's configuration names in `goal.human.<name>` as `Name <email>`; the ambient
`user.name` and `user.email` are never used. The join (D2) refuses a member whose approver has no
identity there, `BATCH_JOIN_AUTHOR_UNBOUND`, so the seal never writes a commit it cannot attribute. The
owner rides in a `Landed-By: <seat>` trailer beside R15's trailers. No human act is added, and nothing
reads identity to tell a human commit from an agent one. The fast-forward keeps the commit objects, so
the endpoint shows each unit's approver. Witness: W15 (author and committer equal the approver's
configured identity, `Landed-By` names the owner, an ambient `user.email` naming another person changes
nothing) and the join refusal of an unbound approver.

D8. Green lands as a series of agent commits by commit.sh, each authored and committed as its goal's approver (amendment, attribution), one atomic push (C6, C7, DONE 5). Where: inside the owner's lock hold after green, so no engine run is between its fetch and its
fast-forward while the series reaches origin (m1b's race). Per unit in join order, in the landing checkout at the batch base: apply the certified patch (index and
worktree equal the prefix tree), append its receipt row (`scripts/receipt.sh add`, receiptline.go:142),
then run `scripts/agents/commit.sh --chain <J> --goal <G> --test-receipt <tip receipt> -F <message>` as
the owner's child: the caller is the lease-holding main with claim epoch and lineage (commit.sh:31-57), the
brain fence runs (commit.sh:9-29), `landing observe` consumes the chain as today (commit.sh:592-604;
observe.go:324-470) and the `Goal-Item`, `Landing-Provenance` and verdict trailers are written
(commit.sh:829-832), so `matchLandingMessage` (attention.go:967-1002) matches and a registered `wait
--goal G --event landing` returns (attention.go:831-840). The message is the chain's return message plus one batch paragraph; the tip receipt is `landing
test-receipt --tree <tip> --mode auto` (landing_verbs.go:167-231). Two boundary changes in Go, with
the witness that neither weakens the per-commit rule: (i) receipt.go series rule: in `readTestReceipt` (receipt.go:666-694), when the receipt does not
cover the candidate by identity, the candidate is accepted iff the installation root holds a batch in
state `landing` whose `seal.tipTree` projects to the receipt tree and whose `units[i].unitTree` equals the
candidate, and, intrinsically, every changed path of the candidate holds the base blob or the tip blob;
the posture check then compares index and worktree to the candidate (testing.go:226-246). No intermediate commit is ever a trunk tip: the push is one ref update. (ii) observe.go delegation:
`heldGoal` (observe.go:1093-1107) accepts an actor other than the holder iff the goal file at the base
carries `batch-<id>` and the installation root's batch `<id>` is in `landing` with a unit (G, J) whose
`claim` equals the goal's current claim and whose `owner.actor` is the actor; the `goal-revision-moved` check is unchanged. Rejected (C7): per-prefix receipts composed by identity. `fast-static-build` and `context-standard`
declare `metasystem/cmd/**`, `metasystem/internal/**` and `metasystem/scripts/**` as inputs
(testing.json:33, 55), so any two Go units give such groups different identities at prefix and tip; a
prefix receipt would re-execute the proof per unit, which is serial landing. After the last commit: `landing held --base origin/main --commit HEAD` once (main.go:142), one
fast-forward `git push origin HEAD:refs/heads/main`. Trunk moved between green and push: fetch; when `ChangedPaths(baseTree, origin tree)` meets no selected
group's `InputManifest` (lane 4's CONDITION 2, report line 42), rebase the series in a detached worktree (advance.go:83-92), re-run `landing held`, and `test verify --tree <new tip> --purpose delivery`
composes by identity (test.go:1069-1117); when an input is met, `test run` on the new tip under the same hold executes the missing groups (a new attempt, charged as D6); red there is D7. A
push rejection re-fetches and repeats at most three times (land.sh:1188-1199), then the batch returns to
`open` on the new base. Crash in `landing`: origin holds the series tip: `landed`; otherwise the local
series is rebuilt from the record. Re-arm at the new tip, release the lock. The register-preserving fast-forward (`FastForwardPreservingRegisters` behind `landedRearmFastForward`,
rearm_on_landed.go:222-227; ledger-preserve.sh:15-27 in Go) is B6: a dirty append-shaped register's local
lines are held, the tracked file restored, `merge --ff-only` run and the lines re-appended; a non-append
shape refuses `TEST_POLICY_ENGINE_REQUIRED` naming it.

D9. Staged rollout refusal (C10). Before the first join: the landing checkout /Users/wido/LocalStorage/GitHub/agentic-tools-landing
exists, enrolled with nickname `m1l`, its engine built and armed, and `batch-landing-authority` opened and
approved. Every batch verb runs `batchSubstrate()`, which refuses `BATCH_UNAVAILABLE` naming each missing
capability: r3 U1a (`proofrun.CandidateAttemptSchemaVersion`), r3 U4, member charging
(`budget.chargesMember`), the series receipt rule, the delegation, the owner verb and the series landing
step (`batchLandSeries`), each a Go symbol in a capability table, so "B7 present but B11 absent" refuses.
The hand lane lands until the table is complete.

D10. `landing batch status`: per batch the state and reason, units with gate and claim states, the lock
owner and queue, the owner's liveness by the prober, the last sample, time to the maximum wait.

## 3. Rules and witnesses

Each witness fails on origin/main and again under its named mutation; the builder proves each in the
return. Scenarios live in scripts/agents/land-fixtures.sh.

R1. The join runs its own gate and names only runs it made. W1a `TestBatchJoinRunsItsOwnGate`: the seam `batchGateExec` records one execution per step at the unit
tree and the entry names those run ids; W1b `TestBatchJoinRefusesRedStep`: a scripted red in (b) refuses
`BATCH_JOIN_GATE_RED` naming the package; W1c `TestBatchJoinIgnoresSuppliedResults`: a run record placed
under the landing root before the join is not named and shortens nothing. Mutations: read a step's result
from a file; accept exit 1; skip step (c).

R2. The unit is its closed chain. W2a `TestBatchJoinRefusesOpenChain`: `chainClosed` false, critic ref missing, role not implementer,
`goalRevision` behind the claim, each its own code (`BATCH_JOIN_CHAIN_UNCLOSED`, `_UNREAD`,
`_NOT_IMPLEMENTATION`, `_REVISION_MOVED`). W2b `TestBatchJoinDerivesDiffFromChain`: no `--diff` flag
exists; `certifiedDigest` equals `chainCertifiedOutput`'s digest. W2c
`TestBatchJoinRequiresJobDomainChain`: a codex-rescue task id refuses `BATCH_JOIN_CHAIN_REQUIRED` naming
the launcher port. W2d `TestBatchChainTransportIsByteExact`: copies equal the seat's; a differing existing id refuses
`BATCH_CHAIN_CONFLICT`. Mutations: accept an open chain; read the diff
from a flag; overwrite on conflict.

R3. One membership per chain and per goal. W3 `TestBatchUnitJoinsOneBatch`: chain J live in batch A, joined to B under another name: `BATCH_UNIT_ELSEWHERE`; goal G live in A with J1, joining B with J2:
`BATCH_GOAL_ELSEWHERE`; J1 and J2 of G in A: accepted. Mutation: index by goal only; by chain only.

R4. Fixture groups by the owned map; nothing carried to the tip. W4a `TestFixtureGroupsForChangedBeds`: a
changed bed selects its group; a harness file gives `covered-by-proof`; a Go-only unit selects none.
W4b `TestFixtureBedGroupMapMatchesSections`: the tsv equals validate-metasystem.sh:1051-1056 and
validate-section-selector.sh:8-30. W4c, scenario `batch-seal-executes-changed-inputs`: unit E changes a
helper in group G's inputs, a later unit U changes G's bed; the tip plan marks G `execute`; with E
absent, G is `reused` from U's join-time run. Mutations: map a bed to a sibling
group; carry the join-time run id into the tip plan.

R5. One lock, append-only history, label ordering, prober. W5a `TestBatchJoinsSerializeUnderTheLock`: two
joins with a barrier; each tip is the previous plus that unit. W5b `TestBatchLabelMatchesGrammar`: `ValidateLabels` (goal.go:427) accepts the label. W5c
`TestBatchCrashBetweenEntryAndLabel`, real verbs, both orders: (1) seam `batchAfterEntryWrite` aborts before the label: the owner's reconcile drops the entry,
the re-join succeeds; (2) a label naming a `landed` batch: the next join replaces it. W5d
`TestBatchHistoryIsAppendOnly`: eight transitions, eight entries. W5e `TestBatchSealFreezesMembership`.
W5f `TestBatchLivenessUsesInjectedProber`: a fake prober scripts alive, dead and unknown for the owner
and the sealer without real pids: dead re-spawns, unknown does not, alive spawns nothing. Mutations: write without the lock; history through `--next`; seal after the lock; `KernelProber{}` in the
seam (the fake pids read dead or unknown, W5f red).

R6. A conflicting join names every file. W6 `TestBatchJoinConflictNamesFiles`. Mutation: first path only.

R7. Seal re-runs the fast gate on the tip. W7 `TestBatchSealRegates`: the seam records (a) and (b) on the
tip tree; a scripted red refuses `BATCH_SEAL_GATE_RED`; trunk moved: re-apply, one `trunk-conflict`
ejection named. Mutation: seal on the recorded unit results.

R8. The union closes at the deep ceiling. W8 `TestBatchSelectionUnionClosesAtCeiling`. Mutation: counts.

R9. Start rule, clock, wake, owner. W9a `TestBatchStartRule` (pure): lock held: no; free, one unit before the wait: no; at the wait: yes; two
units, quiet: yes; two units, unknown census: no; at the wait under load with two launchers: yes,
`expired`. W9b
`TestBatchOwnerLaunchesAtMaximumWait`: fake clock; `batchLaunchProof` empty until the deadline, then one
launch with the sample. W9c `TestBatchOwnerWakesOnSignal`. W9d `TestBatchJoinSpawnsOneOwner`: two joins,
one owner by the fake prober; a dead one replaced. W9e `TestBatchOwnerHoldsTheLease`, scenario: the built owner announces, `lease require-holder` reports it
holder, a second owner exits `ownedElsewhere`. W9f `TestBatchOwnerWiringBound` (tenfold bound). Mutations: drop the
maximum-wait branch; unknown as quiet; wall clock; ignore the channel; spawn unconditionally; skip the
announcement.

R10. The census counts every top-level proof entry. W10a `TestProofLauncherArgv`: `test run` and `landing
batch owner` are launchers, a shell mentioning them is not; W10b `TestCensusCountsRunningBatchProof`: an
unreadable table gives known=false. Mutation: keep the `proof-run launch` clause only.

R11. Every member is charged; headroom reserved at seal. W11a `TestBatchAttemptChargesEveryMember`: goals X, Y, Z each show one more attempt and the full minutes
after one batch attempt. W11b
`TestBatchSealWithdrawsMemberWithoutHeadroom`: Y with one attempt left is `withdrawn-budget`, X and Z
seal as a new tree. W11c `TestBatchBaseRunChargesNoMember`: empty `batchMembers`. W11d `TestBatchSerialProofChargesItsUnit`: one member. W11e `TestBatchMemberAtLimitAtEachStage`: Y at its limit before the red batch (withdrawn at seal), before
the base run (it still runs), before its serial proof (refused `BATCH_MEMBER_BUDGET_REFUSED`, left `open`,
not ejected). Mutations: charge the authority goal only; skip the seal reservation;
charge the base run to members.

R12. Red maps by inputs, base first, serial order. `TestBatchOwnerSearch` table, each row asserting run trees, count and eject set: W12a single
owner (N = 2, k = 1): batch red, base green, eject, one re-prove: 3 proof runs, 1 base. W12b pair (N = 3,
k = 2): batch, base, the unnamed unit alone (lands), A alone (red, ejected naming the landed partner), B
alone (lands): 5 runs. W12c three-unit-only red (N = 3, all named; singles and pairs green): batch, base,
A lands, B lands on tip+A, C red on tip+A+B, ejected naming A and B: 5 runs. W12d trunk red: batch, base
red: hold, nobody ejected. W12e no unit named (N = 2): batch, base green: hold. Mutations:
skip the base run; eject every named unit at once; match by file-name suffix.

R13. Eject re-assembles exactly; reuse follows identity. W13 `TestBatchEjectAndReassemble`: the surviving
patches applied in join order in a fresh worktree equal the record's new tip; the next prove is a new
attempt with no retry decision; a group whose inputs held the ejected file is planned `execute`, an
untouched one `reused`. Mutations: drop a survivor; reorder two; reuse by group id.

R14. A held batch reopens only on a new tree. W14 `TestBatchReopenNeedsNewTree`: reopen on the held tree
refuses `BATCH_REOPEN_SAME_TREE`; a trunk commit meeting the group's inputs reopens. Mutation: reopen on
any call.

R15. Each commit is an agent commit through commit.sh with its chain, by the lease-holding owner. W15, scenario `batch-lands-by-agent-commit`: the built owner lands one unit; the commit carries the guard
marker, `Machine: m1l+main-...`, `Goal-Item`, `Landing-Provenance: chain=<J>`, `Landed-By: <seat>` and a pass verdict, with author and committer equal to the goal approver's `goal.human.<name>` identity. W15b `TestBatchDelegationRules`: no label, a label naming a batch not in `landing`, or an owner actor
differing from the record: `goal-item-not-held`; a claim re-made after dispatch: `goal-revision-moved`;
all present: pass. (W15b is superseded by BLB-R3-001 in brief B: observe and held pass unmodified and nothing
delegates, so the test of that name is gone; `TestBatchLandingRequiresGreenProofActor` keeps the landing-side
rule that only the green proof's actor lands.)
Mutations: call `git commit` directly; accept the label without the batch state; drop the revision check.

R16. The series receipt rule and the atomic push. W16, scenario `batch-two-units-disjoint-groups`: unit A changes internal/testpolicy (`policy-canary`),
unit B a bed (its section group); commit A's candidate is the prefix and `landing observe` passes by the
series rule; with the batch not in `landing` it refuses `chain-test-receipt-refused`; a candidate holding
a blob neither base nor tip refuses; one push
moves origin by two commits; a registered `wait --goal <A's goal> --event landing` returns
`commit:<sha>:chain=<J>`. W16b `batch-land-trunk-moved`: goal-verb commits only: rebase, `test verify`
sufficient, one push; a commit inside a selected group's inputs: `test run` under the same hold. W16c `batch-land-resumes`: a crash seam after the first commit; the rerun rebuilds and pushes
once. Mutations: push after each commit; accept any candidate under a `landing` batch; skip `landing held`.

R17. The rollout refuses fail-closed. W17 `TestBatchVerbsRefuseWithoutSubstrate`: each table entry missing in turn, the series step included;
every verb refuses `BATCH_UNAVAILABLE` naming it and launches nothing; W17b on the built engine. Mutation: proceed
with one capability missing.

R18. The re-arm fast-forward preserves append-only lines. W18a `TestFastForwardPreservesRegisters`: two
unpushed receipt lines survive a fast-forward that appended others, in order; W18b: a register edited
mid-file refuses `TEST_POLICY_ENGINE_REQUIRED` naming it, bytes unchanged; W18c scenario
`rearm-preserves-ledger`. Mutations: skip the re-append; treat every dirty register as append-shaped.

R19. Refusal rows travel with their first emitting unit. Each unit's proof runs go-gate.sh --fast, whose refusal register stage is `TestHCL03EveryCodeRowed`
(register_test.go:20-48; the walk at 152-182 covers internal/landing and cmd/metasystem). Mutation: move
a row to a later unit (its gate is red).

## 4. Units

Changed lines are additions plus deletions, tests included, at most 300 per unit; basis about 25 lines
per function, 30 per Go test, 40 per scenario. Landing order is row order; B1 is first. Files are under
internal/landing unless a path says otherwise; every unit adds the rows for the codes it first emits
(R19).

| Unit | Files | Lines | Basis | DONE | Witness | After |
|---|---|---|---|---|---|---|
| B1 record, store, lock, history, label, prober seam | batch.go, batch_store.go; internal/refusal/register.go | 290 | types 60, store and flock 60, transitions 40, label and reconcile 40, rows 10, 3 tests 80 | D1 | W5b, W5c, W5d, W5f | none |
| B2 chain reader, unit identity, uniqueness, transport | batch_chain.go, batch_join.go | 280 | chain read 60, transport 50, uniqueness 30, rows 10, 4 tests 130 | D2 | W2a-d, W3 | B1 |
| B3 join gate: fast gate, packages, fixture groups, bed map | batch_gate.go; scripts/agents/fixture-bed-groups.tsv, land-fixtures.sh | 290 | exec and records 80, package derivation 30, bed map 40, rows 10, 3 tests 90, scenario 40 | D3 | W1a-c, W4a, W4b | B2 |
| B4 join assembly, conflict, ceiling, seal gate | batch_join.go, batch_seal.go | 270 | apply 50, conflict 30, ceiling 30, seal re-gate 50, rows 10, 4 tests 100 | D4, D5.1 | W5a, W5e, W6, W7, W8 | B3 |
| B5 census, start rule, clock, wake, owner verb | internal/proofrun/attemptload.go, batch_owner.go; cmd/metasystem/landing_batch_verbs.go; internal/config | 300 | argv rule 10, owner loop and announce 130, spawn 30, config 20, 6 tests 110 | D5.2-5.4 | W9a-f, W10 | B4 |
| B6 register-preserving fast-forward | fastforward.go; cmd/metasystem/rearm_on_landed.go; scripts/agents/supervision-fixtures.sh | 250 | helper 90, seam 10, 2 tests 70, scenario 40, refusal text 40 | D8 race fix | W18a-c | B1 |
| B7 member charging, `batchMembers`, seal admission | internal/proofrun/attempt.go, internal/dispatch/budget.go, admission.go; batch_seal.go | 240 | field 30, projector 40, admission 50, 5 tests 120 | D6 | W11a-e | B4; r3 U1a |
| B8 proof launch under the authority claim, rollout table | batch_prove.go; cmd/metasystem/test.go | 260 | claim and launch 70, plan relay 30, capability table 50, rows 10, 3 tests 100 | D6, D9 | W4c, W17, W17b | B7; r3 U4 |
| B9 red: base run, named set, serial fallback, eject, hold, reopen | batch_red.go, batch_reopen.go | 290 | search 90, eject 50, hold and registry 30, reopen 20, table test and 2 tests 100 | D7 | W12a-e, W13, W14 | B8 |
| B10 series receipt rule and delegation | receipt.go, observe.go | 260 | series rule 70, intrinsic check 40, delegation 50, 4 tests 100 | D8 (i), (ii) | W15b, W16 (observe half) | B1 |
| B11 series landing step: commits, held, push, rebase, resume | batch_land.go; cmd/metasystem/landing_batch_verbs.go; scripts/agents/land-fixtures.sh | 290 | series build 80, rebase and verify 60, push and resume 40, 3 scenarios 110 | D8 | W15, W16, W16b, W16c | B9, B10, B6 |
| B12 status verb, docs | cmd/metasystem/landing_batch_verbs.go, docs/project-rules.md, docs/glossary.md | 160 | status 70, docs 40, 2 tests 50 | D10 | status rows | B11 |

Total about 3,180 changed lines over twelve units. Builds are Codex gpt-5.6-sol in a worktree, reads
Opus, critique by another model; B6 may land any time after B1.

## 5. Moved effects

Old owner, new owner, count and decision per moved write; the old owner's code is the backticked path.

| Effect | From | To | Code |
|---|---|---|---|
| Queue entry write, one per unit; retired | the hand lane's markdown queue | `landing batch join`, a record entry under the batch lock | `metasystem/scripts/agents/land.sh:1091` (stages caller paths, reads no queue) |
| Testrun lock take and release for a landing proof, one per proof; moved | the seat sourcing hact testrun-lock.sh by hand | the owner, owner line "batch pid start batch:<id>" | the run it wraps: `metasystem/cmd/metasystem/test.go:753` |
| Delivery proof launch, one per batch; wrapped, verb unchanged | a seat running `test run --purpose delivery` under its own claim | the owner under its claim of `batch-landing-authority` | `metasystem/cmd/metasystem/test.go:753` |
| Attempt and minute charge, one record, n member charges; changed count | the claim's goal only | the authority goal plus every member through `batchMembers` | `metasystem/internal/dispatch/budget.go:519`, `metasystem/internal/proofrun/attempt.go:373` |
| Launcher census identity, one predicate; widened | `proof-run launch` argv only | plus `test run` and `landing batch owner` | `metasystem/internal/proofrun/attemptload.go:158` |
| Bed-to-group map, one file; new owned copy proved equal | the suite script's section functions | scripts/agents/fixture-bed-groups.tsv checked by W4b | `metasystem/scripts/validate-metasystem.sh:1051`, `metasystem/scripts/agents/validate-section-selector.sh:8` |
| Flake registry line, one per held batch; moved | the seat by hand under rule 4 | the owner's hold path | `metasystem/docs/flake-registry.md:17` |
| Goal-side batch mark, one label add per join, replaced at the next join; new derived write | no goal-side record of a stack today | `goal edit --label` from the joiner (the claim holder) | `metasystem/cmd/metasystem/goalsync_mutations.go:671` |
| Chain records, one closed chain per unit; new copy | the seat's job store only | the landing root's job store, byte-exact, under the batch lock | `metasystem/internal/landing/observe.go:340` (reads the landing root) |
| Checkout lease of the landing checkout, one holder; new holder | no holder (the checkout does not exist) | the owner, an announced main | `metasystem/internal/lease/verbs.go:420` |
| Unit commit, one per unit; wrapped, commit.sh retained | land.sh `commit_changes` | the owner's land step calling commit.sh per unit | `metasystem/scripts/agents/land.sh:464`, `metasystem/scripts/agents/commit.sh:1` |
| Receipt row write, one per unit; wrapped | the seat running `scripts/receipt.sh add` by hand | the land step, before each commit | `metasystem/scripts/receipt.sh`, `metasystem/internal/landing/receiptline.go:142` |
| Goal-held rule at the commit boundary, one check per commit; extended | holder actor only | holder actor, or the batch owner under a live `batch-` label | `metasystem/internal/landing/observe.go:1093` |
| Receipt coverage rule, one check per commit; extended | exact, workspace-equal or identity-covered candidate | plus a prefix of a `landing` batch whose tip the receipt proves | `metasystem/internal/landing/receipt.go:666` |
| Rebase onto origin, one per landing; reused by call | `landing advance` on one commit | the land step with the same helper over the series | `metasystem/internal/landing/advance.go:83` |
| Proof verification after rebase, one per landing; wrapped and extended | land.sh `verify_current_testing_proof` | the land step: identity reuse, else a re-prove under the lock | `metasystem/scripts/agents/land.sh:717`, `metasystem/cmd/metasystem/test.go:1069` |
| Push to origin, one per landing, three tries; moved | land.sh `push_origin` | the land step, one ref update for the series | `metasystem/scripts/agents/land.sh:650` |
| Re-arm fast-forward, one per re-arming run; replaced | plain `merge --ff-only` in the seam | `landing.FastForwardPreservingRegisters` | `metasystem/cmd/metasystem/rearm_on_landed.go:224` |
| Attempt schema validation, one field; extended | schema 3's tuple validation (r3 U1a) | the same validator with `batchMembers` | `metasystem/internal/proofrun/attempt.go:373` |
| Refusal rows, twenty-one; extended, one unit each | the register's current rows | the register with each unit's codes | `metasystem/internal/refusal/register.go:13` |

## 6. Rollout facts

The landing checkout of D9 exists before B1's first use: only its attempts back batch receipts
(testing.go:129-185) and its job store backs observeChain. `batch-landing-authority` is opened and
approved in Wido's name (R-119-m1e) before B8. Until D9's table is complete, joins may be made (B1-B5 are queue and gate) but the hand lane lands. When R-111-m1e's cap
expires, the owner's lock step is a no-op behind the engine's admission cap; 5.2's quiet sample
stays.

## 7. Fold of critique r2

| Finding | Status | Section | Witness |
|---|---|---|---|
| BLB-R2-001 forgeable gate | FIXED | D2, D3 | W1a-c, W2c |
| BLB-R2-002 drift to the tip | FIXED | D4 | W4c, W7 |
| BLB-R2-003 aliasing | FIXED | D2 | W3, W2b |
| BLB-R2-004 partial minimal red set | FIXED by replacement (C4): minimal red set withdrawn for serial order | D7 | W12c |
| BLB-R2-005 budgets during diagnosis | FIXED | D6 | W11b, W11e |
| BLB-R2-006 refused series command | FIXED | D5.4, D8 | W15, W15b, W9e |
| BLB-R2-007 one receipt per series | FIXED (series boundary chosen; per-prefix rejected) | D8 (i) | W16 |
| BLB-R2-008 invalid label | FIXED | D1, D8 (ii) | W5b, W5c, W15b |
| BLB-R2-009 hunk API | FIXED (whole group, C9 option 1) | D3 | W4a, W4b |
| BLB-R2-010 rollout | FIXED | D9, section 6 | W17 |
| BLB-R2-011 unrowed codes | FIXED | R19, section 4 | each unit's gate under R19 |
| BLB-R2-012 prober | FIXED | D1, D5.4 | W5f, W9d |

Fixed 12, refuted 0, deferred 0. C13 kept: lock and history (W5a, W5d), census (W10), Q3 (W9a),
clock and wake (W9b, W9c), fast-forward (W18), moved effects (section 5).

## 8. Out of scope

The r3 substrate units; the launcher port (Wido's next goal); `--authority` proofs; the hact drivers and tmux; the seat context cap; doctrine beyond one sentence in docs/project-rules.md.

## 9. Assumptions

A2: `git apply --3way` applies wholly or names every conflicted path. A3: a group's execution identity
is unchanged when its declared inputs are unchanged (r3 A2). A4: the landing root's engine is the trusted
landed engine at proof time; `prove` runs `test plan` first. A7:
a goal write is an atomic replace. A8: goal-verb commits touch only ledger paths, which no group's inputs
name (testing.json:30-55) and the delivery workspace excludes (registers.go:39-45), so they never change a
proved identity (lane 4's CONDITION 2). A9: a closed chain in the seat's job store is the trust class
today's agent commit consumes (observe.go:340-398); the batch adds no new trust.

## Escalation

None. DONE (1) is D3's three runs plus the chain's two closures; (2) D4; (3) D5; (4) D7 in the seat's
serial-order form; (5) D5.2 and D8. Two facts for the seat: scratchpad-built units cannot enter a batch until the launcher port lands (D2);
`batch-landing-authority` needs its open and approval before B8 (D6).
