# Fable critique r1: manual work through public review

- Design: plans/designs/intent-manual-review.md, status draft, id 01M3EJ5W1JX95BA2NHD4G6Z1N7
- Resolved through `metasystem project design-of --goal verbs-match-intent` (m1e launcher): draft page listed first, parents 01M3EFDSFTKWEMSDCP1BB7TDGQ and 01M3EC3QT7M2TC36P7ZVNRF0RW accepted, verbs-match-intent.md superseded
- Reviewed bytes: working-tree page, git blob 4b80dcfbe269ad72f806d3924a4b4835b4b7e0d2, sha256 d5c739fce4904f2cc9abfcdbf82a93c50f52268104de16aebd2f43143b1d4525, on branch commit 55229e925 (page uncommitted)
- Source read from the same working tree, which carries uncommitted Opus edits; line numbers are working-tree lines, not commit lines
- Round 1 of 2. Read-only. No edits except this report.

## Verdict

REVISE. MATERIAL findings: 3 (IM-C1, IM-C2, IM-C3). Step 1 (diagnostic review) is buildable and safe from this page apart from one namespace decision (IM-C3). The required second slice (manual delivery) is honestly implementable, but the page sends the builder to a new retained request registry and a patch entry point that bypasses the commit core's index preflight; both would be built wrong (IM-C1, IM-C2). The corrections are deletions, not additions.

## Criterion answers

Step 1, "would an implementer build it DIFFERENT or WRONG": yes, on one point. `review changes` and `review diff` collide with the goal-id namespace in the existing dispatcher, and the page does not decide the collision (IM-C3). Everything else in step 1 names real owners: launch read admission with DiffFile, `read.diff` freezing, ChooseReadMode, Manager Status/Wait/Cancel, gittree Snapshot and NewDetachedWorktree. Step 1 "works and is safe without the findings": it works; safety holds because no Goal-Read, closure or commit is produced by the diagnostic subjects and the design forbids applying a patch to the source. The one proof gap is that the partition extraction touches the live build path and the table has no behavior-preservation row (N1, non-material).

Slice 2, "honestly implementable": yes, and with less than the page asks for. The existing chain is `goal branch commit` (raw verb) plus `goal branch push`, then public `review commit SHA --goal G`, then `land G`. Only the first two steps are non-public. The page's own words "no second job registry" and "SAME commit core" are right; two paragraphs contradict them.

## Findings

### IM-C1 MATERIAL (slice 2): the retained manual-submission request is a second registry the branch already provides

Page text: "Add a focused manual-submission request operation to internal/goal/branch. Its retained request lives beside existing branch operation/read state and freezes source/base/patch/brief, goal/work name, request identity, admitted destination and commit/publication result. It owns replay ... Record the computed subject and commit operation before publication so a lost commit or push response resolves the authentic existing result."

Source evidence (read):
- Manual commit already exists end to end as a raw verb: `goal branch commit --goal G --kind unit --unit NAME` (cmd/metasystem/goal_branch.go:905-1021) checks checkout lease, claim, writes the commit token itself (withGoalBranchCommitTokenAt, 1025-1046) and calls branch.CommitStaged. No builder, no trailer knowledge from the person.
- Public review of that commit already exists: `review commit SHA --goal G` (cmd/metasystem/intent_delivery.go:897-914) runs the branch read, closes, collects and publishes (intent_unit_review.go:375-431). It requires the commit on origin's goal/G (goal_branch.go:279-283), so a push precedes it.
- Public landing already ignores UnitRunner: landGoal (intent_delivery.go:1325) and handLandingSubject (1487-1515) read branch status and attestations only, and point an unread unit at `review commit`.
- Duplication is prevented by git, not by a request record: installCommitOnto checks out the new tip into the goal checkout (internal/goal/branch/commit.go:457-487), so a repeat finds no staged change; a crash before install leaves only a dangling scratch commit (buildCommitOnto, 382-405), so a repeat rebuilds the same tree with no branch duplicate. Lost push responses are already reconciled from recorded push transactions (internal/goal/branch/push.go:300-323, "reconciled"; pushWithRepository 337-348).
- Work-name namespace is already enforced by the range: amendUnit refuses "goal branch repeats build" and "has no build NAME to amend" by scanning Goal-Unit commits by unit name (commit.go:551-563). So "an existing model-built item is not silently replaced" needs no new store: a same-name unit in range refuses unless --after N.
- status/review visibility: NamedWork lists only named UnitRunner entries keyed by worktree (internal/launch/unit_named.go:476-518). A manual unit will never appear there and must not be faked into it. The honest owner read for "status sees manual work" is the branch range (InspectStatus units by name), which productionIntentBranchState already consumes (intent_delivery.go:283-315).

Smallest correction: delete the retained request operation. Replay = look up the work name in the goal branch range; same name and same tree means "already installed at SHA", same name and different tree means "changed bytes; submit with --after N", no name means new work. Lost commit response = repeat (index still staged, or checkout already at tip). Lost push response = branch.Push's own reconciliation. Keep only the frozen patch bytes plus brief under the launch owner's existing custody for the later read.

### IM-C2 MATERIAL (slice 2): CommitPatch cannot take the patch "instead of reading it from the index" without breaking the core's preflight

Page text: "Add CommitPatch as a narrow entry to the SAME commit core used by CommitStaged, taking the frozen binary patch instead of reading it from the index."

Source evidence (read): the core reads the index four times, not once. commitPreparedState takes paths from facts.Staged for validateCommitPaths (commit.go:327-345); commitStagedOnto takes the patch from facts.Patch, which is `git diff --cached --binary --full-index` (commit.go:489-507, commit_repository.go:92-93); installCommitOnto's checkoutInstallPreflight compares the checkout's index tree with the new tip and refuses unstaged collisions (commit.go:407-443, 457-460); amendUnit reads facts.Index as the wanted tree (commit.go:565-570). Injecting a Patch function that returns frozen bytes (CommitStagedWithInputs, commit.go:350-355, allows it) while Staged and Index still describe an unrelated goal-checkout index yields a commit whose tree is the patch but whose install preflight and path validation were judged against a different index. That is built WRONG, silently.

Smallest correction: no CommitPatch. Apply the frozen patch into the goal checkout's index with `git apply --cached --binary` (or --index when the checkout is the destination), then call CommitStaged unchanged. The scratch 3-way apply in buildCommitOnto still runs on top. If the page wants a name for this, it is "stage frozen patch", an effect in the CLI's goal-worktree step (prepareGoalWorktree exists: cmd/metasystem/intent_worktree.go:121-140, claim and lease verified first), not a new core entry.

### IM-C3 MATERIAL (step 1): `review changes` and `review diff` are goal ids to the current dispatcher

Source evidence (read): runIntentReview routes any single argument not in {design, job, unit, commit} to runIntentReviewGoal as a goal id (intent_delivery.go:496-498); goal ids allow lowercase words such as `changes` and `diff` (validIntentJobID, 404-415). Today `metasystem review changes --brief F` is "review goal changes" and is refused because review G rejects --brief (intent_selection.go:330-335); `review diff PATCH` falls to "no subject kind". The page adds both words as subject kinds under "no new top-level verb" but never says they are reserved. One builder will reserve them; another will try the goal first and fall back to the diagnostic path, so a goal later named `changes` changes the meaning of a diagnostic command. Different builds, one wrong.

Smallest correction: one sentence in Public subjects: "changes and diff are reserved subject words of review; a goal with either id is reviewed as `review G --work NAME` only through status's continuation, and `review changes`/`review diff` never resolve a goal." Plus a help-text test in TestIntentManualDiffReview.

## Non-material notes

- N1 Extraction proof gap. The read sequence lives inside UnitRunner as step state: appendReadSteps (internal/launch/unit_run.go:451-479), serialised split reads sharing declared outputs (527-556), compaction reruns fanning whole to package to file (615-661), and the counting rule (429-449, 663). The page's "per-request entry retains the read sequence's child IDs" is that step list without a build step. Add one proof row: existing unit-run read tests unchanged after extraction, so the extraction is provably behavior-preserving for builds. Recommended, not blocking.
- N2 Reader checkout. Existing reads launch with WorkingDirectory set to the worktree (unit_run.go:542). gittree.Snapshot plus NewDetachedWorktree are the right primitives (gittree.go:343-352; detached.go:93-125), but NewDetachedWorktree lives under the system temp directory and is named for landing receipts. Declared outputs must be copied into launch custody before Close; the page implies this but should say it, because temp is swept by other seats.
- N3 "contains no local secret config" is stronger than a worktree gives: `.git/config` is shared by every linked worktree. Ignored files and conf.local are absent, which is what matters. Reword.
- N4 source = destination. When the caller stands in the goal checkout, CommitStaged's own preflight and adoption checks (commit.go:361-376, 407-443) already give the documented review G effects. The page's claim holds; no owner change needed.
- N5 Amendment. amendUnit locates the target by unit name, applies the patch on the old commit and walks the suffix dropping that target's Read commit (commit.go:530-600, suffix loop partially read). The page's "removes only this work's obsolete attestation" matches what I read.
- N6 Diagnostic feedback stays diagnostic. Nothing in the launch read path writes a Goal-Read or closure; that is done only by goal branch read/commit-read (goal_branch.go:968-996). Step 1 is safe on this axis as written.

## Unexamined

- Whether a goal-associated diagnostic read (`--goal G`) is subject to pack or goal budget admission in Manager.admit/CheckPack (admit.go:57) and what a refusal looks like publicly.
- unitReadPacket and readDiff packet contents for split modes; `stop review REF` semantics over a multi-child sequence (cancel one child or all).
- push.go reconciliation before line 300; attest.go prospectiveReadPatch (read-kind commits) beyond confirming it is a separate owner.
- The concurrent Opus working-tree changes to intent_*.go; my citations are to that tree, not to 55229e925.
- No runtime execution: no builds, tests, goals, agents or config were touched.

## Tool budget

22 tool calls before this write; within 12 minutes.
