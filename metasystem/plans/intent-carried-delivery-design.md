# Complete exceptional delivery through human intent

Root-authored implementation correction to accepted designs/intent-workflows.md,
IW-4. Status: draft for Fable. Goal: verbs-match-intent. 26 September 2026.
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

## Composition

1. Resolve the corresponding primary installation with the existing linked-checkout
   resolver, and require its actual branch main. Do not switch branches, move HEAD,
   create another repository or silently target a different project. A non-main
   primary gives a public checkout correction, before recording an exception.
2. Use the existing branch candidate owner unchanged. Freeze a SUBJECT ARTIFACT
   before recording the carry: goal, endpoint commit, projected endpoint, projected
   candidate, binary patch and its digest. Put it under existing intent-land/G
   artifacts keyed by candidate workspace; write temp+rename, identical replay
   allowed, mismatched bytes refused. This is immutable input, not a job registry.
   Preflight the patch on the destination's full index without writes. Preserve
   existing admission/claim/read requirements for canonical branch composition.
3. Record or rejoin the exact carry word via existing owner. Retain its association
   to the frozen subject alongside the existing exception message. The pre-word
   workspace-keyed artifact closes a lost-response gap: read back the proven word's
   workspace and adopt only the matching artifact. A replacement is a new explicit
   human act, never automatic because code moved.
4. Read real carry status before installing. local/origin/ledger consumption goes
   straight to the existing recovery path, with NO staging. Superseded, expired,
   unproven or mismatched words keep the owner's refusal. An unconsumed word uses
   its frozen patch, never recomposes newer goal work. A word recorded through the
   channel may have no artifact: compose the CURRENT goal candidate once and retain
   it only if its projection equals the word. Otherwise refuse with the public
   replacement-exception act; never reinterpret the word.
5. Stage via a small existing landing-owner extension under LockBounded, shared for
   human and agent callers. Check actual main branch, expected product endpoint,
   no unmerged entries, index equal HEAD OR the exact already-prepared candidate,
   and no non-register dirty/untracked paths. Append-shaped shared registers are
   preserved under existing drift rules; the projected patch never touches them.
   A main tip that moved only in excluded coordination state can still qualify by
   matching projected endpoint; product movement refuses unchanged before staging.
   Recheck under lock, private-index preflight, then plain --index --binary once.
   Repeated staging is a no-op only when actual index projection equals the word
   AND the worktree agrees on every staged product path. No reset/clean rollback.
   Failure reports actual staged state, retaining the exact subject for retry.
6. Release the staging lock before calling the existing main installation's
   land.sh -m MESSAGE --goal G --carried ID --staged-only. Existing carried
   verification rechecks its workspace across reservation/fetch/commit; it stays
   authoritative. Do not hold a checkout lock across a nested owner that acquires
   it. No rewrite of the carried transaction. Failure returns a public
   `land G --using-exception ID` continuation and truthful retained stage/commit.

The standalone exact staging helper belongs with landing (the consumer of projected
workspace/carry status), while candidate selection remains branch.PrepareLanding.
No branch source mutation, fabricated ordinary landing proof, pending receipt in
main, new testing policy, or duplicate carry state machine.

## Proof required before IW-4 completes

- TestIntentCarriedGoalDeliveryGitAdapter: authentic reviewed goal branch -> public
  exception -> real carry word, actual staging and unmodified land.sh transaction,
  main contains payload, carried provenance and consumed word. Existing fixture-only
  human proof/proof-token seams are permitted, no fake successful land callback.
- Same fixture starts from linked goal checkout and preserves main's existing
  append-only rows; no projected-tree deletions or pending receipt can land.
- TestIntentCarriedReplay: failure before push -> --using-exception resumes the same
  subject/word; moving source goal does not change it. local/origin/ledger recovery
  never stages. Channel word adopts only matching current composition.
- TestCarriedStagePreservesCheckout: unrelated index/dirty files, endpoint movement,
  conflicting patch, concurrent mutation lock, repeated exact staging. Refusals
  preserve bytes/index; no conflict state from --3way or destructive cleanup.

Full independent Sol critique remains mandatory. Maximum two Fable rounds for
this concrete correction; falling bounded findings may become named fixtures,
never permission to certify an unsafe or non-working implementation.
