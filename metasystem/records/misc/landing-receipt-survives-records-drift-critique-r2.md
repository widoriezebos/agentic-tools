# receipt-survives-drift design critique — round 2 (revision 2)

Chain: revision 2 (landed f4abc699, sha256 b9795095aa99ab5272e5573737f19e5418382c0b761654b020bf736f96ef26c7) -> critic lrsrd-crit2 (design-critic, codex gpt-5.6-sol, xhigh, read-only; the harness observed gpt-5.6-sol and the return claimed nothing different). Reviewed at checkout 33126861. 8 findings, 7 material. The coordinator carried the return here verbatim because the critic's sandbox is read-only. One finding arrived with the id 'chain-unique' instead of an LRE number; it is recorded as LRE-08 below with its text unchanged.

## LRE-01 — critical, material=True

CLAIM: The rare register capture-and-restore case should not exist. Loud refusal leaves the exact local bytes in the worktree, and the shipped register-carriage landing can commit them on top of the local landing commit before the common advance path retries. Receipt and digest consumers require exact append-only bytes, but none requires the real checkout to be rebased while those bytes remain unstaged. The existing carriage path preserves them; re-emission is unnecessary and can break exact-prefix consumers. Steps F and G instead introduce an admitted data-loss path. The design should delete the capture files, link protocol, exported digest lock, rare-case fixtures, and two loss windows, and make register overlap a named refusal repaired by register carriage. This changes metasystem/plans/landing-receipt-survives-records-drift-design.md and what is implemented.

EVIDENCE: metasystem/internal/landing/observe.go:647-677 accepts record-only carriage relative to current HEAD, and Git history contains many such landings. metasystem/internal/retrodebt/debt.go:174-212 and metasystem/internal/narratordigest/digest.go:247-313 bind exact prefixes. Before step F, the design leaves the branch at landing commit L and the worktree bytes untouched, so refusal loses nothing.

## LRE-02 — critical, material=True

CLAIM: If the rare path remains, its two claimed loss windows are neither exhaustive nor bounded. A receipt writer can be descheduled for an arbitrary wall-clock interval after opening an inode and before writing, so two immediate agreeing reads can finish and cleanup can unlink the capture before that write occurs. During reset, the lockless O_CREATE writer can also create the temporarily absent path or open the destination while Git recreates it; the table covers only a writer that opened the old inode. Linking a complete pre-reset file does not control the file Git creates during reset. The protocol therefore cannot claim that only two instruction-width windows remain.

EVIDENCE: metasystem/internal/receipt/receipt.go:445-458 performs separate OpenFile, WriteString, Sync, and Close calls without a shared lock. The design itself says reset unlinks and recreates the register and admits a final-read dead-inode loss, but supplies no scheduling bound or probe for create/open activity during reset.

## LRE-03 — high, material=True

CLAIM: The manifest does not make crash resume sound. A crash after moving the register leaves it absent, so the next run’s classification refuses before reading the manifest. A crash after linking an exact pre-rebase blob can make status clean, so the next run takes the common path and ignores captured suffixes. A crash after reset changes HEAD from landing commit L to rebased commit N, so the next run keys a different directory and exits through the up-to-date fast path before restoration. Rename, link, unlink, and directory removal also lack directory-durability rules. The rare path would need a durable state machine consulted before its fast path and normal classification.

EVIDENCE: The design orders fast path B and classification D before manifest handling in F, keys the directory only by entry HEAD L, and moves HEAD before restore step G. Its crash paragraph covers only a subset of F and asserts that rerunning advance resumes, which this ordering contradicts.

## LRE-08 — critical, material=True

CLAIM: Landing commit L is not unique to one advance invocation. Two concurrent advance calls starting at the same HEAD share the manifest, counters, strip files, raw files, and cleanup directory under metasystem/artifacts/agents/landing/advance/L. No ownership claim distinguishes a crashed invocation from a live one. They can rename over one another’s captures, remove the shared directory, and both reset the branch without compare-and-swap protection. Advance must be serialized per checkout and verify that HEAD still equals L when moving the branch; naming storage after L is not concurrency control.

EVIDENCE: Steps F and G key all artifacts solely by L and treat any existing manifest as an interrupted run. metasystem/internal/lease/verbs.go:462-481 shows that the existing lease lock ends with commit.sh, before advance, and the new standalone verb specifies no replacement lock.

## LRE-04 — high, material=True

CLAIM: The common path has an unchecked stage-after-check race. Entry precondition A catches a stage made before advance begins, and classification D catches one made during the private rebase, but no lock or atomic comparison spans D through git reset --keep N. Git documents that --keep resets index entries. A stage on a path N did not change can therefore arrive after D, be reset away, and still let Advance report success. The design must hold the checkout mutation lock through the final check and reset, or specify an atomic HEAD-and-index comparison when moving the branch.

EVIDENCE: The installed git-reset manual says --keep resets index entries and only updates worktree paths changed between N and HEAD. metasystem/scripts/agents/land.sh continues after commit.sh has released its run-held lock. The design specifies two index snapshots but excludes no writer between the second snapshot and reset.

## LRE-05 — medium, material=True

CLAIM: The implementation boundary for Advance is underspecified. The design promises that every Git call uses gittree’s scrubbed, bounded wrapper, but that wrapper is package-private. The implementation map adds only Workspace.Status and NewDetachedCommitWorktree. Advance also requires ancestor probing, rebase and abort in the detached worktree, reset --keep, and, if the rare path remains, update-index --refresh. An implementer must either bypass the promised timeout and environment boundary with os/exec or invent an unreviewed exported interface. The design must name the owned gittree operations and typed outcomes.

EVIDENCE: metasystem/internal/gittree/gittree.go:104-154 and metasystem/internal/gittree/snapshotscope.go:41 keep git, gitTop, gitAt, and gitProbe unexported. The package’s exported method inventory has no rebase, abort, reset-keep, refresh, or ancestor operation, while implementation-map steps 1-8 name none.

## LRE-07 — medium, material=True

CLAIM: The implementation map omits the new public advance refusal codes from the refusal register. The design introduces advance-not-on-branch, advance-index-not-empty, advance-rebase-conflict, advance-unstaged-drift, advance-register-removed, advance-register-contended, advance-capture-corrupt, and advance-restore-mismatch, but does not update metasystem/internal/refusal/register.go or name each repair route. The fast gate will not expose this omission because none matches the automatic collector’s recognized suffixes. The design must classify these codes or explicitly exclude them with reasons.

EVIDENCE: metasystem/internal/refusal/register.go contains rows for current landing refusals. metasystem/internal/refusal/register_test.go:16-47 automatically collects upper-snake tokens and hyphenated tokens ending only in refused, unreadable, malformed, or unavailable. Implementation-map steps 1-8 omit the refusal package.

## LRE-06 — low, material=False

CLAIM: The explanation of the post-commit check is wrong after the design’s own Advance replacement. metasystem/scripts/agents/sync-transport.sh reads refs only, as the revision now says. But the check also no longer protects a real-worktree rebase because the new rebase is private, and it never protected the proof-to-commit equation because commit.sh compares the landed tree directly with the proved tree. What it still does is reject index changes and non-register or non-append worktree changes observed immediately after commit. The prescribed implementation is otherwise unambiguous, so correcting this rationale would not change what gets built.

EVIDENCE: metasystem/scripts/agents/sync-transport.sh:31-35 fetches and pushes refs. metasystem/scripts/agents/commit.sh:539-578 verifies the committed tree and rolls back a mismatch. The proposed Advance rebases only its detached worktree.

## Gaps the critic named

- The requested register metasystem/records/misc/landing-receipt-survives-records-drift-critique-r2.md could not be written because the critic filesystem is read-only. The complete register is contained in this return, and the target remains absent.
- No fixture bed or scratch repository mutation was run, as required by the brief. Fixture conclusions are based on source, installed Git documentation, and the design’s recorded probes.
- The link-a-complete-file step was not itself probed. The additional reset-window risk is inferred from the lockless O_CREATE writer and the design’s own premise that Git unlinks and recreates the register.
- The launcher exposed no session identifier, so sessionId is reported as unobserved and no differing session is claimed.

## What the critic verified

- (ran) git rev-parse HEAD; git merge-base --is-ancestor f4abc699 HEAD; shasum -a 256 metasystem/plans/landing-receipt-survives-records-drift-design => The synchronized checkout is commit 331268618d437b4c4e373e5c0a088d114ff10cce. Revision-2 commit f4abc699 is its ancestor, and the reviewed design matches SHA-256 b9795095aa99ab5272e5573737f19e5418382c0b761654b020bf736f96ef26c7.
- (read) Read the complete metasystem/plans/landing-receipt-survives-records-drift-design.md and metasystem/records/misc/landing-receipt-survives-rec => Revision 2 replaces the shared stash with a private rebase, a common reset path, and a rare capture-and-restore protocol. All five first-round findings have recorded dispositions.
- (read) Read the installed Git 2.50.1 git-reset and git-status manuals => Git documents that reset --keep resets index entries and updates only working-tree files that differ between the target commit and HEAD, aborting when such a file has local changes. The porcelain-v1 table supports the design’s MM, AM, TM, deletion, type-change, untracked, and seven unmerged-state classifications.
- (read) Read metasystem/internal/narratordigest/digest.go, metasystem/internal/receipt/receipt.go, metasystem/internal/retrodebt/debt.go, metasystem => The digest atomically replaces its whole file under a flock and uses exact prefix cursors. Receipt writers perform separate lockless OpenFile, WriteString, Sync, and Close calls. Receipt consumers include cadence checks, delivery health, watch, metrics, and retro-debt records that bind exact byte prefixes.
- (ran) git log --all --format='%H %s %(trailers:key=Landing-Provenance,valueonly)' -- metasystem/memory/receipts.log metasystem/records/narrator-di => Repository history contains many direct-fix class=register-carriage commits for these logs, confirming an existing path that can preserve their exact bytes after a rare-overlap refusal.
- (read) Read metasystem/internal/landing/observe.go and metasystem/scripts/agents/land.sh => A register-carriage candidate is evaluated relative to current HEAD and may be committed on top of the already-created local landing commit. The ordinary landing performs its post-commit check, fetch, advance, and push after commit.sh has returned.
- (read) Read metasystem/internal/lease/verbs.go and metasystem/scripts/agents/commit.sh => The checkout lease lock is held only while the commit wrapper child runs. It does not span the later post-commit check or the proposed advance operation, and the standalone advance verb has no specified lock.
- (read) Read metasystem/internal/landing/receipt.go and the revision-2 schema rules => Raw candidate equality does imply equality after deterministic path filtering. Version 1 can be checked against raw bindings and version 2 against filtered bindings. Because both registers are tracked in real candidates, the two mixed forms differ and the proposed pins detect them.
- (read) Read metasystem/scripts/agents/land.sh step order and the crossover procedure => Without METASYSTEM_BIN, the stale engine reaches the unknown landing-drift verb before the tier-1 receipt command and therefore before its battery. On a chain landing the battery has already minted the supplied receipt; the unknown verb fails before commit, not before that battery, and the receipt remains available for retry.
- (read) Read metasystem/scripts/agents/sync-transport.sh and metasystem/scripts/agents/commit.sh tree postcondition => Transport fetches and pushes refs without reading the worktree. commit.sh itself proves the committed tree equals the proved index tree. After the proposed private rebase, the post-commit drift check protects only the immediate checkout posture, not transport, the private rebase, or the proof-to-commit equation.
- (read) Read metasystem/scripts/agents/land-fixtures.sh and metasystem/scripts/agents/fixture-bed-scenarios.sh => The revised refusal setup has a nonempty staged candidate and therefore reaches the intended unstaged-drift check. The combined passing canary is red on the untouched implementation because current stage_changes rejects the dirty registers; its stash identity and two-register status assertions can detect the named regressions after implementation.
- (read) Inspect exported methods in metasystem/internal/gittree and read metasystem/internal/refusal/register.go plus register_test.go => gittree exposes no rebase, abort, reset-keep, refresh, or generic bounded Git operation to package landing. The refusal register’s automatic collector would not recognize any proposed advance code because those codes lack its recognized suffixes.
- (ran) Attempt to add metasystem/records/misc/landing-receipt-survives-records-drift-critique-r2.md with apply_patch => The write was rejected because the critic filesystem is read-only. The target file remains absent.
- (ran) git status --short at the end of the review => A background process modified metasystem/records/narrator-digest.log during this read-only review. The critic made no repository change, and the observed modification was preserved.

## Coordinator disposition (m1b, 2026-09-09) — fold-read cycle 2

Said aloud, as the rule requires: this loop has no natural exit. Two reads
have each found material defects in a page that answered the previous read.
The exit condition set here, which Wido may veto: revision 3 goes to the
build, not to a fourth design read; the chain's closing code read verifies
the four specifications below; that keeps the goal at its
reviewRoundLimit of 3 (two design reads and one code read).

What makes this fold safe to take without a third design read is that it is
a deletion plus four bounded specifications, not new mechanism.

- LRE-01 (critical): ACCEPTED, and it is what the orchestrator recommended
  to Wido before this read. The rare case (Decision 3e steps F and G) is
  deleted. A register in the advance overlap refuses loudly with a named
  code; the bytes stay in the worktree; the shipped register-carriage
  landing commits them on top of the local landing commit; the common
  advance path then succeeds. Re-emission is not only unnecessary, it
  would break the exact-prefix consumers the critic names.
- LRE-02 (critical) and LRE-03 (high): fall with the rare case. Nothing
  survives of the capture files, the link protocol, the exported digest
  lock, the loss windows or the manifest resume.
- LRE-08 (critical, the 'chain-unique' finding): ACCEPTED in the half that
  survives the deletion. Advance must be serialized per checkout and must
  verify HEAD still equals L when moving the branch. The storage-collision
  half is gone with the capture directory.
- LRE-04 (high): ACCEPTED. The common path needs the checkout mutation lock
  held from the final index check through the reset, or an atomic
  HEAD-and-index comparison; today the lease lock ends with commit.sh
  (internal/lease/verbs.go:462-481).
- LRE-05 (medium): ACCEPTED. gittree's git wrapper is package-private; the
  page must name the owned gittree operations and typed outcomes advance
  needs (ancestor probe, rebase and abort in the detached worktree,
  reset --keep) instead of leaving the implementer to bypass the bounded
  wrapper.
- LRE-07 (medium): ACCEPTED. The advance refusal codes join the refusal
  register with repair routes, or are excluded with reasons.
- LRE-06 (low, not material): ACCEPTED as a rationale correction.

Next: fold brief to the Fable lane for revision 3 in place; then the build
brief on the implementation lane, the closing code read, and the
receipt-bound landing.
