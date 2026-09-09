# receipt-survives-drift design critique — round 1 (revision 1)

Chain: revision 1 (landed 5a316c26, sha256 d030a289924a02f330eb6ec4474c66e0a60ee39f69e27d1311536d030796ccb2) -> critic lrsrd-crit1 (design-critic, codex gpt-5.6-sol, xhigh, read-only; the harness observed gpt-5.6-sol and the return claimed nothing different). Reviewed at checkout 007d3f52. 5 findings, 5 material. The coordinator carried the return here verbatim because the critic's sandbox is read-only and it wrote no register file.

## LRD-01 — critical, material=True

CLAIM: Decision 3b cannot safely use git rebase --autostash for these live registers. During a successful rebase the temporary autostash is worktree-specific, but a failed reapplication is saved into the repository-wide stash list; the prescribed unqualified git stash pop can therefore apply and drop another session’s newest entry. The guard also depends on human Git output whose locale and version are not fixed. More seriously, an append that occurs after Git snapshots the autostash but before its reset is absent from both the stash and the new HEAD and can be overwritten silently; merge=union and the reapplication guard run too late to recover it. The design must replace autostash with private per-landing storage using the register paths already known by the drift verb, and it must define capture, rebase, and restoration against both writers’ concurrency protocols so every byte written before or during that interval survives. Its fixtures must exercise a concurrent register write and prove an unrelated sentinel stash remains untouched.

EVIDENCE: The reviewed design at lines 321-342 instructs autostash and recovery through the newest stash entry while claiming no bytes are lost. Both inspected worktrees share .git/refs/stash, although their temporary rebase autostash files differ. The Git binary records that a conflicted autostash is made into a stash entry. The two register writers do not share a lock with Git, and the narrator digest actually changed from clean to modified during this read-only review.

## LRD-02 — high, material=True

CLAIM: The merge=union pin does not establish the premise Decision 3b needs. It checks for a literal line, while Git uses the final effective attribute after all matching rules, and reapplication uses the post-rebase destination tree. A later overriding rule or an upstream removal of the attribute can therefore make the pin pass before rebase while the autostash reapplication lacks union behavior. If autostash remains, the design must test the effective attribute for both registers in the tree where restoration occurs and specify the refusal when it is absent; if LRD-01 removes autostash, this unsupported premise should be removed with it.

EVIDENCE: git check-attr currently returns merge=union for both registers, and metasystem/.gitattributes lines 1-2 supply those values. Decision 1 lines 185-196 instead specifies a text-file pin, while Decision 3b lines 335-342 makes the stronger claim that content conflicts cannot occur.

## LRD-03 — high, material=True

CLAIM: Schema version 2 lacks a cutover contract for a same-checkout two-engine path that exists today. A live pre-change bin/metasystem can create a schema-1 receipt for a staged candidate containing the new source, after which commit.sh builds a schema-2 proof engine from that candidate and uses it to read the receipt. The seat-boot check does not prevent this because it compares the live binary with committed HEAD, not staged source. This recreates the expensive post-battery refusal the goal is meant to remove. Decision 4 must choose and test a crossover rule, such as minting with the candidate-built engine, requiring an explicit candidate rebuild before receipt creation, or providing a narrowly proved compatibility read.

EVIDENCE: land.sh line 14 selects the live checkout binary and line 386 uses it to create receipts. commit.sh lines 297-319 build a separate engine from prospective source, and line 463 uses that engine for landing observe. dispatch.sh lines 248-269 inspect only commits between the engine stamp and HEAD. bin/metasystem is ignored rather than part of the candidate tree.

## LRD-04 — high, material=True

CLAIM: The --require-empty-index outcome table tolerates staged register changes. Because the staged rule only handles entries whose worktree column is blank, valid MM, TM, and AM register states fall through to the unconditional register-M tolerance; the named drift fixture even requires MM to be tolerated without distinguishing the flag. The post-commit check can consequently start transport with index changes that appeared after the proved commit. The table must classify every nonblank index column as staged drift when --require-empty-index is set, before applying the register exception; without the flag, the intended staged-candidate-plus-register-append case may remain tolerated. The tests must assert both modes.

EVIDENCE: Decision 3a lines 288-298 orders Y-space handling before unconditional Y-M register tolerance. The installed porcelain-v1 table permits MM, TM, AM, RM, and CM; DM is not a valid ordinary state, and all documented unmerged pairs already reach the final refusal rule. Decision 3’s fixture list at lines 468-472 currently says MM register is tolerated without qualifying the mode.

## LRD-05 — medium, material=True

CLAIM: The shell refusal canary cannot exercise the refusal it claims. Immediately after the existing matching-receipt landing, the index is clean; appending only payload.txt and attempting another staged-only landing reaches “the caller-selected staging set is empty” before the drift verb. HEAD remaining unchanged can therefore pass for the wrong reason, while the required “unstaged changes remain after staging” text is unreachable from the specified setup. The fixture must stage a fresh non-register candidate, mint the matching receipt for that candidate, then introduce a separate unstaged payload edit and assert the exact drift refusal before any commit.

EVIDENCE: Decision 3’s shell fixture lines 491-496 introduces only the payload edit after the prior landing. land.sh lines 320-323 keep the empty-staging-set check first, before its current or proposed drift check.

## Gaps the critic named

- The requested register could not be written to metasystem/records/misc/landing-receipt-survives-records-drift-critique-r1.md because the review role and filesystem sandbox are read-only. This schema return contains the complete register, and the target file remains absent.
- No fixture bed or scratch mutation was run, as required by the brief. Fixture conclusions are based on code and installed Git documentation, not runtime bed evidence.
- The launcher exposed no session identifier, so sessionId is reported as unobserved and no differing session is claimed.

## What the critic verified

- (ran) git rev-parse HEAD; sha256sum metasystem/plans/landing-receipt-survives-records-drift-design.md; git merge-base --is-ancestor 5a316c26 HEAD => The synchronized checkout is commit 007d3f52617c0dd2f8704d510423e9b3d8dd4d9b. Commit 5a316c26 is its ancestor, and the reviewed page matches SHA-256 d030a289924a02f330eb6ec4474c66e0a60ee39f69e27d1311536d030796ccb2.
- (read) Read metasystem/AGENTS.md, metasystem/wow.md, development/project-rules-local.md, .agents/skills/design-critique/SKILL.md, the complete desi => The binding materiality test is whether an implementer would build something different or wrong. Review-only work must not modify repository files.
- (ran) git rev-parse --path-format=absolute --git-path refs/stash; git -C metasystem/artifacts/agents/worktrees/lrsrd-design3 rev-parse --path-form => Both worktrees resolve refs/stash and its reflog to the same common repository path. Their temporary rebase autostash files are worktree-specific. The installed Git manual says an autostash is temporary but is saved to the stash list on quit, while the Git binary contains the conflict path messages “Applying autostash resulted in conflicts” and “Autostash exists; creating a new stash entry.”
- (read) Read metasystem/internal/narratordigest/digest.go:91-169, metasystem/internal/receipt/receipt.go:439-458, and the autostash reset behavior r => The narrator digest rewrites the whole file atomically under its own flock. The receipts log appends through an open file descriptor without that flock. Git does not participate in either writer protocol when it snapshots and resets an autostashed worktree.
- (ran) git status --short at the beginning and end of the review => The checkout was initially clean. Without any critic write, records/narrator-digest.log became modified during the review, directly confirming that a register writer can run inside an operation’s critical window.
- (ran) git check-attr merge -- metasystem/memory/receipts.log metasystem/records/narrator-digest.log; read metasystem/.gitattributes and Decision 1 => Both paths currently resolve to merge=union, and metasystem/.gitattributes contains literal merge=union lines for them. The proposed pin only says to find such a line in the file; it does not test the effective attribute after all matching rules or after the destination tree changes during rebase.
- (read) Read metasystem/scripts/agents/land.sh:12-15,361-388; metasystem/scripts/agents/commit.sh:287-319,443-463; and metasystem/scripts/agents/dis => The live checkout binary creates a receipt, but commit.sh builds a separate engine from the staged prospective source and uses that engine for landing observe, which reads the receipt. The dispatch skew guard compares the live engine only with committed HEAD, so it cannot detect staged receipt-schema changes.
- (read) Read the porcelain-v1 status table in the installed git-status manual and Decision 3a’s ordered rules. => Valid ordinary states ending in worktree M include MM, TM, AM, RM, and CM; DM is not an ordinary porcelain-v1 state. Decision 3a tolerates those register states even with --require-empty-index because its staged rule only handles Y equal to space. All seven documented unmerged states reach the refusal rule. Untracked ?? correctly refuses, while ignored artifacts are not emitted because the command does not request ignored files.
- (inferred) Read metasystem/internal/gittree/gittree.go:279-311, metasystem/internal/landing/receipt.go:260-310, and Decision 4’s exact-tree rules. => FilterTree deterministically removes fixed paths and writes the Git tree of everything remaining. The design’s recorded scratch probe says an absent removal path exits successfully. Filtered projections necessarily coincide for candidates that differ only in excluded registers, but the receipt as a whole does not collide because its exact tree field and both exact index bindings must still equal the candidate.
- (read) Search all Go call sites of behaviorsurface.Policy.Classify and ClassifyChanges, then read metasystem/internal/behaviorsurface/policy.go:241 => The only production Classify caller in the tree is the behavior-surface classify command; ClassifyChanges has no production caller. Adding the digest to coordinationPaths intentionally changes LANDING membership and the command’s explanatory class, without an unidentified in-tree consumer requiring another design decision.
- (read) Read metasystem/internal/landing/receipt_test.go:16-90, metasystem/internal/landing/observe_test.go:22-72, metasystem/scripts/agents/land.sh => The existing Go paths can produce the named receipt errors. The shell refusal setup cannot reach its intended non-register-drift refusal because, after the preceding successful landing, it has no staged candidate and land.sh checks for an empty staging set before checking worktree drift.

## Coordinator disposition (m1b, 2026-09-09)

All five accepted; every one changes what gets built, so the design loop's
stop criterion is not met by revision 1. This is fold-read cycle 1.

- LRD-01 (critical): Decision 3b is withdrawn. The autostash's failed
  re-apply goes to the repository-wide stash list that every worktree and
  agent session on this machine shares, so the page's own recovery, an
  unqualified git stash pop, can take another session's entry; and an
  append between git's snapshot and its reset survives in neither the stash
  nor HEAD. The critic observed the digest change from clean to modified
  during its read-only review, which is the race made visible. Revision 2
  replaces the stash with private per-landing storage of the register
  bytes, defined against both writers' protocols: the digest rewrites its
  whole file atomically under its own flock (internal/narratordigest/
  digest.go), the receipts log appends through an open descriptor with no
  flock (internal/receipt/receipt.go). A fixture must exercise a concurrent
  register write and prove an unrelated sentinel stash entry is untouched.
- LRD-02 (high): falls with 3b if no merge of register bytes remains; if any
  does, the pin must test the effective attribute in the tree where
  restoration happens, not a literal line, and name the refusal when absent.
- LRD-03 (high): Decision 4 needs a cutover rule for the same-checkout
  two-engine path that exists today: land.sh mints the receipt with the live
  binary while commit.sh builds a separate engine from the staged source
  and reads it with that. The critic named three shapes; the design lane
  chooses, under the constraint that the crossover landing cannot hit a
  post-battery refusal, which is the defect this goal exists to remove.
- LRD-04 (high): under --require-empty-index every non-blank index column
  is staged drift before the register exception applies; MM, TM, AM, RM, CM
  on a register are drift in that mode and tolerated without it; the tests
  assert both modes.
- LRD-05 (medium): the shell refusal canary must stage a fresh non-register
  candidate, mint its receipt, then introduce a separate unstaged payload
  edit, and assert the exact drift refusal before any commit; as written it
  reaches the empty-staging-set check first and can pass for the wrong
  reason.

Next: fold brief to the design lane for revision 2 in place; then a fresh
read whose reviews is revision 2. At cycle 2 the coordinator says aloud that
this loop has no natural exit; before cycle 3 the land-or-fold call is
Wido's.
