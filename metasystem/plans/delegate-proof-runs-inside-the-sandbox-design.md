# delegate-proof-runs-inside-the-sandbox: design

Blocker of delegate-rounds-reuse-a-warm-gate (opened 2026-09-12 under
R-93-m1e). One critique read of this page, then the build behind the fixture
in section 5, one code read, human commit.

## 1. What fails, and where

A delegate that runs the engine-appended testing requirement
(`bin/metasystem test run --root . --goal <goal> --mode auto --purpose
delivery`) inside its job worktree fails before admission. The chain
implementer-5881d3816c94692315588ba0 (round 2, 2026-09-12) showed the first
failure; reading the code shows a second one behind it.

1. **The engine's git writes into the shared object store.** The delegate's
   own git commands write loose objects into the worktree's quarantine
   (`<gitdir>/objects-quarantine`, `GIT_OBJECT_DIRECTORY` set by the
   adapters, `GIT_ALTERNATE_OBJECT_DIRECTORIES` pointing at the shared
   store) because the envelope grants no write to the shared `objects/`
   (internal/dispatch/envelope.go `worktreeGitWriteRoots`, issue 5). The
   engine's git invocations scrub exactly those two variables
   (internal/gittree `steeringEnv`, `ScrubbedEnviron`), so `git write-tree`
   for the staged candidate (`Workspace.StagedTree`, `Snapshot`) writes the
   shared store and the sandbox refuses: `fatal: git-write-tree: error
   building trees`.
2. **The runner materializes beds as linked worktrees of the main
   repository.** `NewDetachedWorktree` (internal/gittree/detached.go) runs
   `git worktree add --detach <tmp> HEAD` from the repository top, which
   writes the administrative entry `.git/worktrees/<name>` in the main git
   dir, outside every root the envelope grants; three sites in
   proofrun/test_build.go (identity preparation, the per-group beds, the
   revalidation) use it. Even with the object store fixed, the run fails
   here.

The scrub and the missing grants are security boundaries: an inherited
`GIT_OBJECT_DIRECTORY` must not steer the seat's engine into a fixture's
store, and a delegate must never write the shared store or main's refs.
The fix keeps both.

## 2. The mechanism

### 2.1 The engine honors the workspace's own quarantine, and only that

- `gittree.Workspace` learns `quarantine(w.Dir)`: the path
  `<absolute-git-dir>/objects-quarantine` when that directory exists for the
  workspace's git dir, else empty. When the process environment carries
  `GIT_OBJECT_DIRECTORY` whose resolved value equals that path, every git
  invocation of that workspace re-adds `GIT_OBJECT_DIRECTORY=<quarantine>`
  and `GIT_ALTERNATE_OBJECT_DIRECTORIES=<common objects dir>` after the
  scrub. Nothing else from the inherited steering survives. The value is
  derived from the workspace, never trusted from the environment: an
  inherited variable that names any other directory is still scrubbed.
- The same rule reaches the engine's other git callers that write objects
  inside a delegate: `landingGit` (internal/landing) reads only; the judge
  key reads only; `NewDetachedWorktree` is replaced (2.2). So the change is
  in one place, `gittree`, and the read-only callers stay as they are.
- Effect: `write-tree`, `commit-tree`, `read-tree` and `hash-object` from the
  engine inside a delegate worktree land in the quarantine, which the
  envelope grants and the engine already links through alternates for
  conformance and merge.

### 2.2 Beds are private repositories, not linked worktrees, inside a delegate

- Inside a quarantined workspace (2.1's condition), `NewDetachedWorktree`
  materializes the candidate as a private repository under the temporary
  directory: `git init -q` there, `objects/info/alternates` naming the
  shared store and the quarantine (reads), `git read-tree <candidate tree>`
  and `git checkout-index -a` into the working tree, and
  `git commit-tree <tree>` plus `update-ref HEAD` so the bed has a real HEAD
  (the build stamp and every script that asks git for a commit keep
  working). The private repository's own writes stay under the temporary
  directory, which both sandboxes grant (the claude scratch, /tmp under
  codex). No `.git/worktrees` entry is written anywhere. `Close` removes the
  directory.
- Outside a delegate (the seat's engine, fixtures), `NewDetachedWorktree`
  stays a linked worktree exactly as today: same code path, same evidence
  identity. The candidate engine build identity is derived from the tree,
  not the bed's commit id, so a private-repository bed proves the same
  bytes; the design critic checks this claim against
  `candidateEngineBuildIdentity`.
- The nested prefix case (a candidate that is the `metasystem/` subtree of
  a repository) keeps `NewDetachedWorktree`'s existing prefix handling: the
  private repository holds the whole project tree with the workspace
  subtree replaced by the candidate, as the linked worktree does today.

### 2.3 What the delegate does not get

- No write to the shared object store, main's refs, or the main git dir.
- No steering of the engine's git by any inherited variable other than the
  workspace's own quarantine path.
- No change to admission: the hook-delegate path (cmd/metasystem/proof_run.go)
  keeps authenticating the delegate and binding its goal.

## 3. What does not change

- The seat's own proof runs, the landing receipts, the cadence battery: none
  of them run in a quarantined workspace, so 2.1 and 2.2 never fire there.
- The adapters' quarantine environment and the envelope's roots.

## 4. Risks

- A private-repository bed differs from a linked worktree in one visible
  way: `git worktree list` in the main repository does not show it, and its
  HEAD commit is a fresh commit-tree object. Anything that read the bed's
  commit id as the candidate's identity would differ; the design claims
  nothing does (section 2.2), and the critic checks it.
- The quarantine grows with every engine run's objects; it is the
  delegate's quarantine already, removed with the worktree.

## 5. Fixtures

1. gittree: `TestQuarantinedWorkspaceHonorsOnlyItsOwnObjectDirectory`: a
   repository with a linked worktree carrying `objects-quarantine`; with
   `GIT_OBJECT_DIRECTORY` naming it, `StagedTree` writes its tree object into
   the quarantine and nothing into the shared store; with the variable
   naming another directory, it is scrubbed and the shared store receives the
   object; a workspace without a quarantine ignores the variable.
2. gittree: `TestDetachedWorktreeInsideAQuarantineIsAPrivateRepository`: the
   materialized bed has the candidate's files, a HEAD, no `.git/worktrees`
   entry in the main repository, and `Close` removes it.
3. dispatch-fixtures, fake runtime: a worktree job whose fake round runs
   `bin/metasystem test plan` and `test run` (the corpus-style command
   contract) inside the worktree completes with an attempt id recorded; the
   shared `objects/` directory's inode set is unchanged before and after.
4. The measured chain of delegate-rounds-reuse-a-warm-gate rerun (its
   slice C) is the field proof.

## 6. Landing

Engine change in `internal/gittree` and its three callers; every seat
rebuilds and re-arms (the judge key changes with `internal/`).
