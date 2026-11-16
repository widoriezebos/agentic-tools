# A landing reads its paths from the repository top

- Kind: design
- Id: a-landing-reads-its-paths-from-the-repository-top
- Status: draft
- Goals: a-landing-reads-its-paths-from-the-repository-top
- Critique: closed at round 1 on 0 material findings (Astra, chain design-critic-c3459cf0d6b286d6fba198db; one low wording note folded into D1; the engine's synthetic binding finding, caused by the seat moving its checkout during the round, accepted as risk by Wido through a1 at 15:07)

Drafted 2026-11-16 by the m1e seat steward from finding 638 (evidence directory /Users/wido/agentic-tools-evidence/lane-test-20261009/, findings.md). Code read at origin/main 7be6390cf5; paths are under metasystem/.

## Problem

A person's exception landing (`work land G --exception CODE ...`) and a hand landing compose the goal's units on main in a scratch worktree and, before applying each unit, check that every file the unit changes still holds the content the unit was built on (internal/goal/branch/land.go:250 verifyUnitPreimagesWith, :280 verifyFoldPreimagesWith). Both read the composed tree through treeEntry (land.go:234), which runs `git -C REPO ls-tree -z TREE -- PATH`.

REPO is the installation root the caller passes as `--root`: cmd/metasystem/intent_exception.go:419 and intent_delivery.go:2317 pass `root.Path()`, and goal_branch.go:482 uses it unchanged as the request's Repo. In this repository the installation is the `metasystem/` subdirectory. `git ls-tree` resolves PATH relative to the current directory unless `--full-tree` is given, so `git -C .../metasystem ls-tree TREE -- metasystem/internal/goal/root.go` looks up `metasystem/metasystem/...`, prints nothing, and treeEntry answers "absent". Every existing file a unit modifies then reads as "changed", and the landing refuses GOAL_UNIT_REREAD "build U no longer applies to main: ... changed there; run work rebase", whatever main holds.

Observed 2026-11-16 03:04 and 11:59 on the-deputy-acts-under-a-recorded-delegation: both attempts (old branch, then rebased branch) were refused naming the same seven files of U1a1; a by-hand replay of the same composition, reading paths from the top, passes all 13 units on today's main. The refusal's remedy (`work rebase`) cannot help, so the person-approved exception cannot land the goal. treeEntryAt (internal/goal/branch/attest.go:703, through attest_repository.go:126 TreeEntry) has the same form. read.go:1113 already reads with `--full-tree`.

## Step 1

D1. treeEntry reads PATH from the repository top: `git -C REPO ls-tree --full-tree -z TREE -- PATH`. Its PATH comes from diff-tree raw entries and is always repository-relative, so the change alters no result for a caller whose REPO is the top level, and makes a subdirectory REPO give the same answer. treeEntryAt (attest.go:703) is NOT changed: its callers strip the installation prefix (attest.go:825) and query full source commits with installation-relative paths (:835-837), relying on ls-tree's directory-relative lookup. Revision 2 had included it; round 1 of U1 showed the error (TestChangeDigestGitAdapter/distant_base_change and TestRebaseGitAdapter/generated went red), and correction 1 reverts it.

D2. One test on the real layout, with real git. Why no test caught this: TestPrepareLandingGitAdapter (internal/goal/branch/land_adapter_test.go:15) already sets Repo to the `metasystem/` subdirectory, but its fixture (land_test.go:22 newLandFixture) has units that only add new files (one.go, two.go, three.go), and an added file passes the check whether ls-tree finds the path or not; the preimage tests in land_policy_test.go double the entry seam, so ls-tree never runs. The new case uses the same fixture with one unit that modifies a file present at the base; PrepareLanding with Repo set to the `metasystem/` subdirectory composes it. It is red on today's code with GOAL_UNIT_REREAD naming the modified file, and green after D1. A second case keeps the refusal honest: main changes that file after the unit's base, and the landing from the subdirectory still refuses naming exactly that file.

Deferred: whether other `git -C REPO ... -- PATH` reads in the landing path share the form (rebase_conflict.go:335 already prefixes; intent_design_gate.go:191 relativizes); a census of them belongs to the-unit-route-runs-on-the-real-layout's successor, not to this step.

## Units

| Unit | Production lines | Total lines | Last | Content |
|---|---|---|---|---|
| U1 | 4 | 140 | yes | D1 and D2: `--full-tree` in treeEntry (not treeEntryAt, see D1); the real-layout landing test, red before and green after, with the honest-refusal case |

## Implementation obligations

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| LP1 | HIGH | D1 | The landing's preimage reads find a repository-relative path whether REPO is the top level or the installation subdirectory | Landing composition (internal/goal/branch land.go treeEntry) | land.go:235 carries --full-tree; attest.go:704 unchanged | TestLandingFromTheInstallationSubdirectoryComposesItsUnits | The deputy's exception landing no longer refuses GOAL_UNIT_REREAD | MISSING | Build U1 |
| LP2 | HIGH | D2 | A unit whose base main did change is still refused from the subdirectory, naming exactly that file | Landing composition | verifyUnitPreimagesWith unchanged | TestLandingFromTheInstallationSubdirectoryRefusesAMovedFile | none beyond the test | MISSING | Build U1 |

## Acceptance

1. The D2 test fails on origin/main 7be6390cf5 and passes with U1.
2. After deploy, the exception landing of the-deputy-acts-under-a-recorded-delegation, run again by a person with the approved command, no longer refuses GOAL_UNIT_REREAD.

## Dispositions (critique design-critic-c3459cf0d6b286d6fba198db)

Written by metasystem design review when critique design-critic-c3459cf0d6b286d6fba198db closed: every answered round's decisions, as the author made them.

| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- | --- | --- |
| 1 | LP-PATH-WORDING | Non-gating wording issue: D1 calls every path repository-relative, but attestation reads also use paths relative to a projected installation tree. The prescribed --full-tree command handles both correctly. Test 1: correcting the explanation would not change implementation. Test 2: step 1 works and remains safe without this correction. | noted | Wording only: --full-tree reads PATH from the given tree's root, so it serves both the landing's repository-relative paths and the attestation reads' projected-installation paths (attest.go:825, :835-837; Astra's own probe). It changes nothing that is built. | D1 now says PATH is relative to the root of TREE: repository-relative for raw entries, installation-relative for projected attestation trees. |
