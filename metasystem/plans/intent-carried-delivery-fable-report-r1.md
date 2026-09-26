# Fable report r1: intent-carried-delivery-design.md

Reviewed at 1355e65c218ac0711cd21b9950153a1c1505190c (branch
codex/intent-workflows-20260926, dirty working tree; the design page itself is
uncommitted). Parent: plans/designs/intent-workflows.md IW-4 (row at line 359,
TestIntentQueueOnly at 379). Criterion applied verbatim: would an implementer
build step 1 DIFFERENT or WRONG because of the finding; does step 1 WORK and is
it SAFE without it. Round 1 of at most two. 18 tool calls, source read, no build.

## Facts on the page verified against source (read, not recited)

- intent_exception.go:119 calls land.sh with `-m --goal --carried` only; land.sh
  accepts that (line 189 exempts `--carried`), then `stage_changes` refuses at
  land.sh:427 ("name pathspecs or choose --staged-only"). Page cites 425; same
  function. The script's own comment (land.sh:523) expects the caller to have
  staged before `--carried`, so stage-then-`--staged-only` is the script's intent.
- land.sh:12 anchors `root` to its own pathname and `cd`s there; `verify_checks`
  (land.sh:384) exits 3 off main. Today the verb runs the goal worktree's copy, so
  step 1's primary-installation resolution is a real first-use blocker fix.
  `linkedEnrollmentRoot` (engine_refusal.go:46) and `goalBranchHolderRoot`
  (goal_branch.go:783) exist and map a linked worktree path to the main checkout.
- `prepareLanding` CandidateOnly returns `Candidate: projected` (land.go:728-733),
  a projected TREE id; `rev-parse X^{tree}` on it is identity, so the carry word's
  workspace already equals the projected candidate. `WorkspaceExclusions`
  (registers.go:24-72) removes receipt ledger, narrator digest, plans/goals*,
  records/counselor, records/goals. Projected-to-projected diff therefore carries
  no coordination rows. Confirmed.
- `Workspace.Diff` (gittree.go:459) is deterministic `--binary --no-renames`;
  `Workspace.Apply` (gittree.go:500) is `apply --cached` in an isolated index at
  the repository top level, no worktree writes. Confirmed as stated.
- `RunHeld` (lease/verbs.go:533-550): `ClassHuman` runs the child with NO lock
  and no gate; others take `leasePaths(root).Lock`, which is `LockPath(root)`
  (lease.go:69). So step 6's "release before land.sh" is required for agent
  callers and harmless for humans. Confirmed.
- `Advance` (advance.go:43-74) takes `LockBounded(LockPath(root))` and refuses
  `advance-index-not-empty` when index != HEAD. A staged patch is therefore not
  destroyed by the agent path; it blocks it (see IC-C1).
- `artifacts/` is gitignored (metasystem/.gitignore:1); the subject artifact
  cannot dirty the primary.

## Findings

### IC-C1  material, SAFETY  Step 4 reads carry status without the fetches it depends on

Source: `ReadCarryStatus(root, carried, goalID, ledgerTip, now)` (landing/carried.go:34)
refuses unless `ledgerTip` equals the LOCAL accepted-ledger projection (line 43),
and reads origin consumption against the locally known `refs/remotes/origin/main`
(line 59-63). land.sh gets "actual" only because `run_carried_landing` runs
`fetch_origin` then `goal_fetch_for_carry` (`goal fetch --root`, land.sh:766-777,
1008-1013) BEFORE `read_carry_status`. The page's fact "ReadCarryStatus returns
actual word/consumption/workspace" omits both preconditions, and step 4 says
"read real carry status" with no fetch.

Failure: a word consumed on origin by another seat, or by a crashed run whose push
landed, reads `consumption=none` locally. Step 5 stages the frozen patch. land.sh
then fetches, sees `origin:*` or `ledger:*`, takes the recovery branch, prints
"already landed", exits 0. The verb reports "goal G landed under exception". The
product patch stays staged in main's index: the design forbids reset, `Advance`
refuses `advance-index-not-empty`, so every agent landing on that machine blocks
until a person clears an index nobody reported. The `local:*` branch is worse: its
`git rebase` (land.sh:1043) refuses on a non-empty index and `carry_ask`s a
misleading "rebase conflict".

Tests: DIFFERENT (implementer must obtain a tip; either invents a local one or
hits `carry-ledger-moved`). WORKS single-machine; NOT SAFE across seats or after
a crash-after-push.

Smallest amendment: in step 4, "run the same two fetch steps land.sh runs
(`fetch origin`; `goal fetch --root <primary>` for the ledger tip) and pass that
tip to ReadCarryStatus; only then classify consumption; no staging before both."
Add to TestIntentCarriedReplay: "origin consumption discovered by fetch stages
nothing and returns the recovery result."

### IC-C2  material, SAFETY  The lock and state root are not named; the current code uses the goal worktree's

Source: `LockPath(root)` is a per-directory file after `resolveRoot` (abs +
EvalSymlinks, lease/verbs.go:30); nothing maps a linked worktree to its primary.
`Advance` locks `LockPath(primary)`. The existing verb uses
`inv.layout.InstallationRoot` (goal worktree) for the message artifact and
`recordedException`, and `inv.stateRoot` for `goal carry` (intent_exception.go:63,
77, 109). Step 5 says "under LockBounded" without a root.

Failure: implementer locks `LockPath(goalWorktreeRoot)`. `Advance` in the primary
passes its index==HEAD check under its own lock, rebases privately, then
`reset --keep`s main while the human path runs `git apply --index` there: two
writers on one index and worktree, no mutex between them.

Tests: DIFFERENT (which root). Step 1 WORKS when nothing else runs; NOT SAFE
against the existing agent path it claims to share a lock with.

Smallest amendment: step 5, "LockBounded(LockPath(PRIMARY)) where PRIMARY is the
step-1 result; drift, posture, apply, ReadCarryStatus and the artifact directory
all take PRIMARY; goal-ledger reads keep the existing state root."

### IC-C3  material, SIMPLIFICATION  The subject artifact duplicates what the carry word already freezes

Source: the word stores `Workspace` (projected candidate tree id), `Past`, `By`,
`Expires`, `Source` (carried.go:49-52); `Diff` is byte-deterministic for a given
tree pair (gittree.go:455-467); step 5 already requires project(main HEAD tree) ==
projected endpoint before staging. Under that check,
`Diff(project(HEAD^{tree}), word.Workspace)` is exactly the frozen patch. The page's
own channel-word rule (step 4, "compose the CURRENT candidate once and retain it
only if its projection equals the word") is already the general path and needs no
artifact. `recordedException` already re-reads the word by projected tree, so the
"lost-response gap" in step 3 is closed by the word.

Tests: DIFFERENT (implementer builds a write-once store, temp+rename, mismatch
refusal, workspace-keyed adoption, replay rules). WORKS and SAFE without it; the
artifact is additive mechanism, not a defect. Reported because the brief asks
whether any mechanism is unnecessary at first use.

Smallest amendment: delete the artifact from steps 2-4. Step 2: compose via the
existing owner, record the word (existing). Step 5: `patch = Diff(project(HEAD
tree), word.Workspace)` in PRIMARY; if the workspace tree object is unreachable
there (separate object store), recompose once and require projection equality,
i.e. the existing channel-word rule. Keep the digest in the failure report only.
Caveat the implementer must check: linked worktrees share objects; a clone with
alternates does not write back to the primary.

### IC-C4  material, low  Endpoint-movement refusal names no public repair

Source: step 5 "product movement refuses unchanged before staging". This is the
right rule (land.sh:823-833 would otherwise demand a supersede), but the parent
requires every public refusal to map to an applicable public repair (intent-
workflows.md, "Every existing public return ... is audited and mapped"; IW-4
runtime proof "Human/agent refusal"). Not a first-use blocker: a fresh goal on
current main passes.

Smallest amendment: one sentence naming the existing public act that advances
the goal's endpoint, or stating it is an outside-system dependency; assert the
continuation text in TestCarriedStagePreservesCheckout "endpoint movement".

### IC-C5  minor  Persistent apply must run where Apply runs

Source: `Workspace.Apply` deliberately runs `apply --cached` at the top level
because path interpretation is cwd-relative (gittree.go:512-518). Step 5's "plain
--index --binary once" says nothing about cwd, and the primary installation root
is a subdirectory of the repository.

Smallest amendment: "at the repository top level, as Workspace.Apply does."

## Answers to the brief's challenge

Projected endpoint -> projected candidate patch, immutable subject before carry,
staging under the existing lock, then the existing carried script: authority is
preserved (word owner, land.sh verification, RunHeld gating all untouched); crash
replay is preserved by the word plus land.sh's consumption branches, PROVIDED
IC-C1's fetches precede any staging; shared state is preserved PROVIDED IC-C2
names the primary's lock. The "immutable subject before carry" element is the one
unnecessary mechanism (IC-C3). No stated helper semantics is false; one is
incomplete (ReadCarryStatus preconditions).

## Unexamined

commit.sh carried trailers; `goal carry` proof and expiry internals;
`FilterPrefixes` object persistence; the `landing drift` register classifier;
`sync-transport.sh` on a dirty index; channel `answer` words; `landCandidate`'s
EndpointTip source; conf.local (not read); no build or test run; no fixture code.
