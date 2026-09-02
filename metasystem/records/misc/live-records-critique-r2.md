# Live-records design critique — round 2 (Sol)

Chain: revision 2 (landing-lock mechanism) -> critic
design-critic-639bcc09ed3f7b4ac648453b (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-02. Eight material findings (one round-1 fold
closed), four critical: the branch-ref advance breaks the caller's
index baseline; the lock lifetime across land.sh's exec boundaries is
unspecified and the stated mechanism unsafe; the writer-pause timeout
violates the no-softening byte law; a crash between commit-tree and
ref-advance recreates the stranded state; plus the conflict-trail
explanation, registry snapshot authority, starvation contract, and
lock-proof legs. SEAT VERDICT: the prose loop is not converging in
this domain (9 then 8 findings); like the alert producers, this needs
an executable spike (a throwaway lock+carry prototype driven through
the crash points) before revision 3. Parked cold-resumable at the
box's edge.

## R2-LR-001-FOLD-CLOSED — low, material=False

CLAIM: LR-001, the advisory-carriage fold, is adequately closed for paths already present in the live-record registry. Revision 2 requires a fresh append-only judgment before every land.sh push attempt and before commit.sh --push, and its proof plan includes a hand-crafted rewriting commit. That changes the original observational trailer into a hard boundary refusal without weakening the general observation mode. The separate first-registration snapshot defect is judged under LR-006.

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 363-376 specify the two hard push call sites, and lines 530-533 require both call sites to refuse a rewriting commit.

## R2-LR-002-CONFLICT-TRAIL — high, material=True

CLAIM: LR-002, the unexplained conflict-trail fold, remains open. The design first says the roughly fifteen collisions did not use the merge-driver path and calls dirty worktree reconciliation the dominant class, then admits that no event transcript exists and lists several incompatible possibilities. The retained Git record after union commit 95e0a644 contains 16 automatic rebase picks and no rebase continue, while 30 commits touched the digest. A future recurrence falsifier cannot establish the missing historical cause. This remains a false or at least unproved premise for attributing the observed

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 434-464 make both the categorical mechanism claim and the event-level disclaimer. The ran reflog count found starts=16, picks=16, continues=0, finishes=16 after 95e0a644.

## R2-LR-003-INDEX-BASELINE — critical, material=True

CLAIM: LR-003, the caller staging-contract fold, is not closed. A private GIT_INDEX_FILE isolates construction writes, but advancing the branch ref changes the baseline against which the shared index is interpreted. The shared index retains the old live-record blob, so immediately after the carry ref advance it contains a staged reversal of the carried append. Pathspec mode then refuses its newly nonempty index; staged-only mode presents the reversal to commit.sh as part of the caller's whole index. An implementer must invent a shared-index rebasing or sequencing protocol that the design explicitly s

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 284-300 advance the ref without touching the shared index. metasystem/scripts/agents/land.sh lines 216-219 refuse a populated pathspec index, while metasystem/scripts/agents/commit.sh lines 142-147 and 328-341 prove and commit the entire index.

## R2-LR-004-LOCK-LIFETIME — critical, material=True

CLAIM: LR-004, the carry-to-guard race fold, depends on a lock lifetime the design does not specify and whose stated exec mechanism is unsafe. A Go-opened lock descriptor is close-on-exec, so directly execing land.sh releases the lock unless that flag is deliberately cleared. Passing the descriptor through ExtraFiles instead makes it inheritable, and land.sh launches commit, Git, hooks, credential helpers, and transport subprocesses; any surviving descendant can retain the lock after the landing exits. Keeping the descriptor only in a waiting parent avoids descendant inheritance, but a killed parent

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 210-214 say the lock-holding verb execs the child, while lines 260-264 rely on an exported token. The installed Go source opens files with O_CLOEXEC, and flock(2) says locks survive only while at least one duplicate descriptor remains open.

## R2-LR-005-NO-SOFTEN-TIMEOUT — critical, material=True

CLAIM: LR-005, the no-dropped-bytes and concurrent-rebase fold, violates the binding no-softening law on the writer timeout path. The counselor has already rendered the exact payload before AppendPayload tries the shared gate. If a landing holds the gate for ten minutes, AppendPayload returns an error and the caller discards its in-memory rendered buffer. The next tick does not retry those bytes: it rebuilds from current records and necessarily writes a different generated-at timestamp. Revision 2's statement that the caller holds entries for an idempotent next-tick retry is therefore false, and its

EVIDENCE: metasystem/internal/steward/counselor_carriage.go lines 173-187 render into a local buffer and return on AppendPayload error without persisting it. metasystem/internal/counselor/render.go line 15 includes GeneratedAt in the payload. metasystem/plans/live-records-landing-design.md lines 231-245 claim retained idempotent retry, and lines 671-674 make inability to retain entries a reject condition.

## R2-LR-006-REGISTRY-SNAPSHOT — high, material=True

CLAIM: LR-006, the generic three-line adoption fold, leaves the governing registry snapshot contradictory. The registry is variously read from HEAD, from the evaluated commit's base tree, and from an unnamed snapshot during outgoing-commit enumeration. For a commit that adds a registry row and rewrites an existing file, the parent registry excludes the path while the final registry includes it. Reading the parent skips enforcement; reading the final tree catches the rewrite. The consistency check only verifies the three declarations and cannot resolve this choice, so the claimed protection against sm

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 197-200 say the engine reads HEAD and that this prevents smuggling; lines 354-362 say evaluator selection reads the base tree; lines 363-376 do not name the snapshot used while enumerating outgoing commits. metasystem/internal/landing/observe.go lines 445-485 show the cited existing policy loader uses the base HEAD tree.

## R2-LR-007-STARVATION — high, material=True

CLAIM: LR-007, the landing-wide mutex fold, serializes contenders only after one acquires the lock; it specifies no starvation contract. flock provides shared and exclusive exclusion but no fairness guarantee. Repeated nonblocking shared acquisitions can barge ahead of an exclusive waiter, and the landing's exclusive acquisition has no timeout or terminal outcome. In the other direction, land.sh's coverage, build, fetch, rebase, push, and transport commands have no wall-clock bounds, so a healthy landing can exceed the asserted ten-minute writer ceiling. The three push attempts bound count, not durat

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 219-245 specify polling shared acquisition and an unbounded exclusive acquisition. metasystem/scripts/agents/land.sh lines 288-317 invoke unbounded external commands. The local flock(2) manual states that acquisition may block but gives no FIFO or writer-preference guarantee.

## R2-LR-008-CRASHED-REF-INDEX — critical, material=True

CLAIM: LR-008, the crash-after-staging fold, recreates the stranded-index state after a different crash point. If the process dies after commit-tree and branch-ref advancement, the branch names the carry commit but the shared index still contains the old live-record blob. That is not merely an ordinary completed commit: it is a staged reversal relative to the new HEAD. The next pathspec landing refuses, while staged-only mode risks combining the reversal with caller content. The claim that no shared-index write means no stranded staged state confuses physical index bytes with their meaning relative t

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 315-322 classify post-ref-advance death as ordinary local Git state. The same document lines 284-300 deliberately leave the shared index unchanged while advancing the ref; metasystem/scripts/agents/land.sh lines 216-219 show the next pathspec landing's refusal.

## R2-LR-009-LOCK-PROOF — high, material=True

CLAIM: LR-009, the real-machinery proof fold, improves the first-round plan but still permits an incorrect lock implementation. The crash leg only says to kill the landing and assert that the flock is free; it does not distinguish killing the wrapper, the land.sh child, or a descendant that inherited the descriptor, nor does it assert that no orphan continues mutating after release. No leg holds a healthy landing beyond the writer ceiling, drives repeated shared acquisitions against an exclusive waiter, or proves that the exact pre-timeout counselor payload survives. An implementation can satisfy the

EVIDENCE: metasystem/plans/live-records-landing-design.md lines 516-521 test one normal shared writer, and lines 534-538 contain the ambiguous kill-and-free assertion. The ten listed legs contain no descriptor-inheritance, exclusive-starvation, timeout-payload, or orphan-mutation case.

## Critic-declared gaps (verbatim)

- The repository still has no individual transcripts for the asserted fifteen manual digest resolutions. The available m0b reflog contradicts a post-driver manual-rebase narrative, but it cannot establish what happened in another checkout or during unrecorded hand editing; the event-level classification remains a gap.
- The runtime was read-only, so no temporary repository was created to execute the shared-index ref-advance, process-kill, starvation, or ten-minute timeout interleavings. Those findings are based on the written control flow, Git index semantics, the shipped caller lifecycle, and operating-system lock documentation.
- The local lock documentation is for Linux. The design also targets macOS, but no live Darwin fairness or descriptor-inheritance experiment was available; the design cannot rely on stronger fairness there without separate evidence.
