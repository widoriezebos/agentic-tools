# Complete exceptional delivery through human intent

Root-authored implementation correction to accepted designs/intent-workflows.md,
IW-4. Status: accepted with required owner fixtures; Fable rounds complete. Goal: verbs-match-intent. 26 September 2026.
This fixes a demonstrated missing composition, not a new authority policy.

## First use and smallest contract

`land G --exception CODE --reason TEXT --by NAME` computes the goal's existing
canonical landing candidate, records the person's exact exception through the
existing carry owner, stages that same product change into the repository's main
checkout, and executes the existing carried landing. `land G --using-exception ID`
continues the exact recorded subject without another exception. All authority,
one-exception, expiry, proof, reservation, consumption and recovery rules remain
with their current owners. No new public verb, certificate or general workflow.

## Facts checked against source

- intent_exception.go:119 calls land.sh without paths or --staged-only. Its
  stage_changes (land.sh:425) always refuses that fresh invocation.
- branch.prepareLanding (land.go:650..736) composes units/folds in a detached
  scratch checkout, verifies preimages/digests, returns CandidateOnly's PROJECTED
  workspace tree and deletes the scratch checkout. It stages nothing in main.
- landing.ProjectWorkspaceTree excludes shared goal/counselor/receipt/narrator
  state (registers.go:24..110). Checking that tree out as a full tree deletes state.
- The existing projection gives a smaller safe solution than exposing a full
  scratch tree: diff PROJECTED endpoint -> PROJECTED candidate. Both lack excluded
  paths, so this patch carries only product changes, never pending receipt rows.
  Apply it onto main's full index; the carried owner writes real coordination rows.
- lease.LockBounded(lease.LockPath(root), ...) is the existing checkout mutex
  (landing/advance.go:43). RunHeld alone does NOT lock HUMAN callers (verbs.go:539).
- gittree.Workspace.Diff/Apply preflight in a private index. Persistent plain
  git apply --index --binary is the existing exact application shape; --3way may
  leave conflict state and is not used on the user's destination.
- land.sh anchors root to its OWN pathname. goalBranchHolderRoot and
  linkedEnrollmentRoot already resolve the corresponding primary installation.
- ReadCarryStatus returns actual word/consumption/workspace. land.sh:1014..1059
  resumes local, origin and ledger consumption before staging. Its cleanup abandons
  reservations but deliberately retains staged changes/local commits.

## Composition (final order)

1. Resolve PRIMARY through the existing linked-checkout installation resolver and
   require its attached branch main. All artifacts, locks, drift/posture checks,
   receipt decisions and ReadCarryStatus use PRIMARY. Git whole-project operations
   resolve its repository TOP LEVEL. Canonical goal state stays endpoint-owned.
   Do not switch branches, discard local commits or select another project.
2. FIRST perform the code-origin and goal-ledger fetches land.sh owns, obtaining
   the accepted ledger tip. Missing remote/fetch failure records and stages nothing.
   Never create an origin alias. For --using-exception read the refreshed word/status
   now: local/origin/ledger consumption goes directly to the existing recovery path,
   without staging. Unproven/expired/superseded words keep the owner's decisions.
3. For fresh input, compose via the existing branch candidate owner. Its Endpoint
   is the live code endpoint AT COMPOSITION, not the goal's original branch base.
   Freeze only goal, endpoint commit, projected endpoint and projected candidate
   atomically in PRIMARY's existing intent-land/G artifacts, keyed by endpoint and
   workspace. Diff derives the patch; no duplicate patch storage or job registry.
   An existing word reuses its retained tuple; newer goal work never changes it.
   A channel word without that tuple may compose once, but only exact equality to
   its workspace permits adoption. Missing/ambiguous retained identities refuse
   without guessing; the original exact request can name its composed endpoint.
4. Validate the tuple against FETCHED code-origin main's projected tree. Product
   movement AFTER composition calls for a person's explicit public
   `land G --exception CODE --replace-exception ID --reason TEXT --by NAME`,
   which recomposes on the current endpoint and supersedes through the carry owner.
   Local main merely BEHIND that same fetched endpoint is a different case: if
   attached main, clean index and local HEAD ancestor of fetched main, call existing
   landing.Advance(PRIMARY, fetchedMain, ...) before staging, then reread posture.
   Advance owns LockPath(PRIMARY); do not hold that lock while calling it. Its
   no-clobber rules preserve append-only rows. No silent rebase of divergent local
   commits. This routine advancement is part of land intent; a behind checkout
   must not loop through --replace-exception. If either owner refuses, preserve
   its actual cause and source bytes, not a guessed stale-word explanation.
5. Preflight the projected-endpoint -> projected-candidate patch against the full
   local index with private-index Workspace.Apply. Record/rejoin the exact human
   carry through its existing owner, then bind the actual opid beside the exception
   message to the frozen tuple. Lost-response adoption matches the word workspace
   and unambiguous tuple. Explicit --replace-exception really invokes replacement;
   an old same-workspace word is not a completed replacement. A later error keeps
   the recorded word and returns `land G --using-exception ID`.
6. Acquire LockBounded(LockPath(PRIMARY), ...). Recheck main, fetched product base,
   no unmerged entries, index equal HEAD OR exactly the retained prepared product
   candidate, and no non-register dirty/untracked paths. All actual staged product
   paths must agree with the worktree. Preserve valid append-shaped register drift.
   Private-index preflight then plain git apply --index --binary at TOP LEVEL,
   once; an exact already-staged candidate is a no-op. No --3way, reset or clean.

   Complete the existing receipt-line obligation while under this same lock.
   ObserveReceiptLine names its actual tracked Ledger path. Exempt/pass needs no
   invented row. For missing receipt, first stage an existing valid append to that
   exact ledger and recheck (crash-after-append must not duplicate the row). If still
   missing, call receipt.Add with Root PRIMARY and explicit File equal to that
   decision's resolved ledger path, Type implement, Outcome reworked, Verify
   skipped, Goal G, and note naming exception CODE/opid and preparation for carried
   delivery. These are actual supported fields (receipt.go:170, defaults in
   receipt_verbs.go); no ordinary proof/verify=clean claim or premature shipped
   claim. Stage the ledger, then require ObserveReceiptLine to pass. Preserve all
   existing rows; never use branch appendReceiptRow's pending/proof placeholder.
   A failure reports actual stage and retains identity for retry. The receipt is
   excluded from the projected workspace, so it does not change the approved word.
7. Release the staging lock before running PRIMARY's unchanged
   land.sh -m MESSAGE --goal G --carried ID --staged-only. That owner retains
   receipt checks, proof, fetch/reservation/commit, consumption and recovery.
   It rechecks the workspace and remains authoritative; no alternate publish path.
   A refusal keeps truthful stage/local-commit facts and a public same-word
   continuation. Existing diff --check refusal remains; no gate is weakened.

The staging helper belongs with landing, which owns workspace/carry/receipt
semantics. Candidate selection remains branch.PrepareLanding. There is no branch
source mutation, fabricated ordinary landing proof, PENDING placeholder receipt,
duplicate carry state machine or new testing policy. The actual receipt row is
staged with the product, through its existing owner.

## Proof required before IW-4 completes

- TestIntentCarriedGoalDeliveryGitAdapter: authentic reviewed goal branch -> public
  exception -> real carry word, actual staging and unmodified land.sh transaction,
  main contains payload, carried provenance and consumed word. The actual
  receipt-line step must pass with a truthful goal receipt, no PENDING/proof
  placeholder. Crash-after-append reuses the row rather than duplicating it. Existing fixture-only
  human proof/proof-token seams are permitted, no fake successful land callback.
- Same fixture starts from linked goal checkout and preserves main's existing
  append-only rows; no projected-tree deletions or pending receipt can land.
- TestIntentCarriedReplay: failure before push -> --using-exception resumes the same
  subject/word; moving source goal does not change it. Fetch discovers previously unknown
  origin consumption and stages nothing; missing or ambiguous retained base
  refuses without guessed replay. Explicit replacement really reaches the owner. local/origin/ledger recovery
  never stages. Channel word adopts only matching current composition.
- TestCarriedStagePreservesCheckout: unrelated index/dirty files, endpoint movement,
  conflicting patch, concurrent mutation lock, repeated exact staging, local
  main behind fetched origin (automatic safe advancement), and origin movement
  after composition (explicit replacement, no rewind). Refusals
  preserve bytes/index; no conflict state from --3way or destructive cleanup.

Full independent Sol critique remains mandatory. Maximum two Fable rounds for
this concrete correction; falling bounded findings may become named fixtures,
never permission to certify an unsafe or non-working implementation.

## Why the minimal retained base remains

A carry word records Workspace but no code endpoint (goal/verbs.go:4581 CarryWord;
file.go:446 HistoryLine). Suppose main moved product A -> B after word W approved
candidate C. Diff(project(B), W.Workspace=C) restores C's old version of B, quietly
undoing newer product work; there is no retained expected endpoint left to reject
that mutation. The canonical branch candidate cannot recover the old subject after
the goal branch changed. Therefore retain only the base/workspace tuple, deriving
patch bytes when needed. No new lifecycle registry or duplicate patch file.

## Final adjudication and implementation authority

Fable material trajectory4 ->2. Final IC-C6 correctly identifies an omitted
receipt producer that makes every real code landing fail. It calls that a
contract-shape failure; root verified it is resolved by the already-existing
receipt.Add + ObserveReceiptLine owners with supported truthful fields and an
explicit tracked file, without new policy/certification. IC-C7 is resolved by the
existing Advance owner plus distinct local/fetched predicates. IC-C8 is removed
by rewriting the order above. These concrete owner compositions are specified,
not delegated design choices. Under the user's explicit machinery bypass and
instruction to implement/iterate, proceed with named runtime fixtures and mandatory
independent Sol critique. No Fable-approved runtime or green carried delivery is
claimed. No third prose round is being substituted for implementation evidence.

## Missing proof recovery uses public intent

Actual carried delivery verifies retained test evidence; it does not produce it.
The existing public `test --goal G --repo PRIMARY` runs that owner against the real
staged index by default. When missing retained proof is the actual cause, the
partial result must name that public action and preserve the exact same-exception
land continuation for afterwards. A repeated land alone cannot fix absent proof.
Users never need to discover or supply a Git tree hash, an internal test verb or
a proof storage path. Other failures retain their own cause and recovery. The
real carried fixture must prove public test followed by the same exception land.

## Candidate construction and deliberate plan inclusion

Real replacement exposed an existing candidate-owner defect: its private scratch
commit executes the installation's delivery hooks. Candidate construction creates
an object to inspect; it is not a delivery commit. Keep its hooks isolated through
the existing private materialization mechanism. The actual land/commit path still
executes all wrapper, ledger, proof and delivery checks. The real replacement
fixture and a focused enrolled-hook candidate fixture must distinguish these paths.

The existing new-plan guard prevents accidental capture of another session's new
plan; it does not reserve every new plan for a separate human approval. Public
land explicitly selects the retained candidate, and the staging owner admits only
that exact product. At that boundary the adapter may acknowledge deliberate plan
inclusion to the unchanged commit owner. Do not duplicate the guard's pathname
regex in the adapter or acknowledge arbitrary unrelated current staging. Reusing
the existing acknowledgment leaves ledger and wrapper fences in force. This
decision is grounded in pre-commit-guard.sh's stated invariant and check order.
