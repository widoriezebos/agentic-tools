# delegate-proof-runs-inside-the-sandbox: design

Blocker of delegate-rounds-reuse-a-warm-gate (opened 2026-09-12 under
R-93-m1e). One critique read of this page, then the build behind the fixtures
in section 5, one code read, human commit.

Revised after the critique read (2026-09-12 evening). The first draft made the
engine run inside the sandbox: its git honoring the worktree's quarantine and
its candidate beds becoming private repositories. The critique showed that
this proves the wrong thing in the wrong place (section 1), so the mechanism
is now the opposite: the proof of a delegate round is made outside the
sandbox, by the orchestrator's enrolled engine, on the worktree as the round
left it, and recorded where the next round and the landing can reuse it.
The goal's title names the failure it was opened for; the record keeps it.

## 1. What fails, and why fixing it in place is wrong

A delegate that runs the engine-appended testing requirement
(`bin/metasystem test run --root . --goal <goal> --mode auto --purpose
delivery`) inside its job worktree fails before admission. The chain
implementer-5881d3816c94692315588ba0 (round 2, 2026-09-12) showed the first
wall; the critique read found three more behind it, and the last one decides
the design.

1. **The engine's git writes into the shared object store.** The delegate's
   own git writes into the worktree's quarantine (`GIT_OBJECT_DIRECTORY` set
   by the adapters) because the envelope grants no write to the shared
   `objects/`; the engine scrubs exactly those variables
   (internal/gittree `ScrubbedEnviron`), so `git write-tree` for the staged
   candidate hits the shared store and the sandbox refuses: `fatal:
   git-write-tree: error building trees`.
2. **The runner materializes beds as linked worktrees of the main
   repository** (`NewDetachedWorktree`, `.git/worktrees/<name>` under the
   main git dir), outside every root the envelope grants.
3. **A job worktree has no enrolled engine.** `test run` and `test plan`
   resolve the trusted policy engine from
   `<installation>/artifacts/agents/steward/identity.json` and the engine
   pins beside it; `artifacts/` is ignored, so the worktree carries none,
   and a delegate cannot enroll (its build stamp is dev-dirty). With 1 and 2
   fixed, `TEST_POLICY_ENGINE_REQUIRED` is the next wall (critique F1).
4. **An in-sandbox attempt would be recorded in the worktree's own control
   root** (`prepared.Installation` is the `--root`). The seat's installation
   never reads it: the landing could not reuse it, the goal's budget would
   not charge it, and doctrine already says delegate output never certifies
   (docs/orchestration.md, Trust and Certification). The proof would be made
   and then thrown away (critique F5).

So the place a round's proof belongs is the orchestrator's installation, and
the engine that makes it is the orchestrator's enrolled engine. That engine
can read the delegate's objects already: the main repository's
`objects/info/alternates` names every worktree quarantine, which is how
conformance and the follow-up rebase read a delegate worktree today, and the
snapshot itself is written from the seat's side. Nothing in the sandbox
needs to change, and both security boundaries (the scrub, the write roots)
stay exactly as they are.

## 2. The mechanism

### 2.1 `metasystem job prove-round --root <installation> --job <id>`

- Resolves the chain from any member id (`RootJobID`), reads the root
  record's `workspaceRoot`, `launchMode` and `goalId`, and the newest chain
  record's `round`.
- The tree proved is the worktree's snapshot, `Workspace.Snapshot("HEAD")`:
  HEAD plus every change in the working tree, tracked and untracked alike,
  the whole project. Delegates never commit (the follow-up rebase and the
  conformance review read the dirty worktree), so this is the round's work
  and exactly the tree conformance reviews. The verb refuses a
  shared-checkout job (its tree is the checkout's own index: the seat proves
  that as its own work), a job without a goal (a proof binds one),
  and a round whose newest record is not terminal (a round is proved after it
  has returned).
- The proof is the installation's own `test run --root <installation> --tree
  <tree> --goal <goal> --mode auto --purpose diagnostic`, in-process:
  admission, the candidate engine build, the worker and the attempt record
  are all the existing ones. Diagnostic, not delivery: a delivery attempt
  proves the seat's own staged index and nothing else (`prepareTesting`
  refuses any other candidate, because a delivery receipt is what lands),
  and a round's worktree snapshot is a foreign tree by construction. A
  diagnostic attempt runs or reuses every selected group and collects every
  failure, so one follow-up carries them all; the landing's delivery receipt
  on the seat's index reuses the passed groups by execution identity (the
  per-group composer consults the source attempt's terminal and identities,
  not its purpose; only the whole-attempt exact reuse is purpose-bound).
  Retained proof is reused by execution identity
  (retained-proof-reuse-crosses-claims-and-attempts), so a round that changed
  no group's inputs reuses every group from the previous round's attempt.
- The round directory gets `rounds/<n>/proof.json`: the tree, the goal, the
  purpose, the attempt id the run produced (the newest attempt for that tree
  and goal that did not exist before the run), whether it was sufficient,
  the exit status and the time. The attempt record itself holds the
  per-group evidence and reuse.

### 2.2 The brief says what the delegate can do

- The engine-appended "Required testing contract" (internal/dispatch/build.go
  `TestingRequirement`) told every implementer to run `metasystem test run`
  and `test verify` in its worktree, which no delegate can do (section 1),
  against docs/orchestration.md's own rule that a brief asks only for
  verification the delegate can perform. It now says: the proof of the round
  is the orchestrator's, made on the worktree snapshot with `job prove-round`
  and reused by identity; do not run `test run`, `test plan` or `test
  verify` in the worktree; run the focused tests and the fast gate for what
  you change with the provided cache; leave everything in the worktree and
  do not commit; report the commands you ran; a failed group comes back as a
  follow-up.
- The brief template's workspace paragraph and docs/orchestration.md's
  shared-testing sentence say the same in one sentence each.

### 2.3 What the delegate does not get

- No engine proof verbs inside the sandbox, no enrolled engine there, no
  write to the shared object store or the main git dir. The delegate keeps
  its warm chain cache (delegate-rounds-reuse-a-warm-gate slice A) for its
  own focused tests and gate.

## 3. What does not change

- The engine's git scrub, the envelope's write roots, the adapters'
  quarantine, `NewDetachedWorktree`: all as they were. The first draft's
  gittree change (the engine honoring the workspace's own quarantine, with
  its test) is parked as memory/backlog-notes.md proposal P-6 with the patch
  kept by the seat; it has no consumer once no delegate runs a snapshotting
  verb inside its worktree.
- The seat's own proof runs, the landing receipts, the cadence battery.
- Admission: `prove-round` is the seat's own `test run`, admitted and
  recorded like any other seat attempt (the same box rules apply: not
  beside a cadence attempt, no rebuild while it is live).

## 4. Risks

- A delegate cannot iterate against the risk-selected plan within a round;
  rounds are the iteration unit, and a red proof returns as a follow-up
  with the failed group's evidence. The delegate's in-round verification is
  what it was before the requirement was appended: focused tests and the
  fast gate.
- `prove-round` snapshots the worktree from the seat's side, so a delegate
  still writing would change the tree under the proof; the verb refuses a
  round whose newest record is not terminal, and the snapshot is taken once
  before admission.

## 5. Fixtures

1. cmd/metasystem `TestProveRoundProvesTheChainWorktreesCommittedTreeOnTheInstallation`:
   a project with a job worktree of the dispatcher's shape holding an
   uncommitted round and a two-round chain of records; `prove-round` on the
   round-2 id runs the installation's test run with `--root <installation>
   --tree <worktree snapshot> --goal goal-a --mode auto --purpose diagnostic`
   and writes `rounds/2/proof.json` with that tree and the run's exit. The
   engine's acceptance of a foreign tree under diagnostic purpose is not in
   this fixture (it stubs the run); the measured chain is that proof.
2. `TestProveRoundRefusesWhatItCannotProveAsARound`: an untracked change
   reaches the proved tree, shared-checkout job refused,
   goal-less job refused, a round still running refused, a record without a
   launch mode placed by its worktree path; no refusal starts a run. A
   `--retry-decision` file rides along to admission (the previous round's
   proof was red; proving the round that fixes it is a retry).
3. `TestNewestAttemptForTreeIsTheNewestMatchingOne`: the attempt linked into
   the round record is the newest one for that tree and goal that the run
   itself made.
4. The measured chain of delegate-rounds-reuse-a-warm-gate rerun (its slice
   C, proved per round with `job prove-round`) is the field proof and that
   goal's remaining DONE item.

## 6. Landing

Engine change in cmd/metasystem (the verb) and internal/gittree (git worktree
administration serialized per repository by a file lock: the second proof
attempt of this landing found two groups of one nested attempt racing on a
half-written `.git/worktrees` entry), the requirement text in internal/dispatch, the brief
template and one doctrine sentence; every seat rebuilds and re-arms (the
judge key changes with cmd/ and internal/).
