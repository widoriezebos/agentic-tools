Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal landing-receipt-races-the-narrator-digest, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

The full-battery landing receipt (`metasystem landing test-receipt`,
CreateTestReceipt in metasystem/internal/landing/receipt.go) runs its
command in the live checkout and refuses afterwards with "the real index
or working tree moved while the command ran" whenever anything under
the tree changed meanwhile. The command takes about twenty minutes (the
fast gate, the dispatch fixtures, the goal-cli fixtures), and on a live
seat the narrator appends to records/narrator-digest.log on every ledger
or landing movement anywhere in the fleet, so on 2026-09-06 the first
receipt for chain rgr-build1 failed after every fixture had passed, and
every later receipt that day was made by hand in a detached worktree of
the same candidate (git worktree add --detach at HEAD, the round diff
applied with --index, the verb run there, the receipt file copied into
the live root's receipts directory), which the landing evaluator accepts
because a receipt binds only tree, command, exit status and time. That
by-hand recipe also protects against the second failure of the day:
any landing of a brief or record between making the receipt and
committing the chain changes the candidate's metasystem subtree and the
commit refuses "candidate subtree moved".

When you are done, the verb makes the receipt against an isolated copy
of the exact candidate tree itself, so neither the narrator nor an
unrelated landing can invalidate it, and a fixture pins a digest append
during the command as still green.

# The design

1. CreateTestReceipt keeps its contract: `--root`, a full tree object
   id that must equal the real index tree AND the working-tree
   projection at the start (the posture check before the run stays as
   the caller's promise that the receipt names what will be committed),
   `--command`, the receipt written to the live root's receipts path,
   the binding fields recorded.
2. The command no longer runs in the live root. The verb creates a
   temporary detached worktree of the repository at the candidate:
   `git worktree add --detach <tmp> HEAD` followed by reading the
   candidate's index into it (the staged tree the caller named; apply
   it with `git read-tree` plus `git checkout-index -a -f` from the
   live index, or an equivalent that reproduces the staged tree
   exactly, and verify with `git write-tree` that the worktree's tree
   equals the named tree before running anything). The command runs
   with that worktree's metasystem directory as its working directory,
   with the same environment the live run had (GOTOOLCHAIN included),
   and its output goes to the same place as today.
3. After the command, the verb checks the worktree's tree is still the
   named tree (the command must not have changed the candidate) and
   writes the receipt into the LIVE root's receipts directory with the
   binding fields naming the tree before and after in the worktree.
   The live root's index and working tree are not compared after the
   run; they may move freely during the twenty minutes.
4. The worktree is removed and pruned on every exit path, including a
   failing command and a signal; its path lives under the system
   temporary directory, never under the repository.
5. Fixture: in the landing fixtures (metasystem/scripts/agents/land-fixtures.sh,
   or the Go tests in metasystem/internal/landing/receipt_test.go if
   that is where the receipt is pinned today; say which), one leg makes
   a receipt with a command that appends a line to
   records/narrator-digest.log in the live root and sleeps briefly, and
   asserts the receipt is green and names the candidate; one leg runs
   a command that modifies a tracked file inside the worktree and
   asserts the receipt is refused with "the candidate changed while the
   command ran"; the existing receipt tests keep passing.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/landing/receipt.go
May touch: metasystem/internal/landing/receipt_test.go
May touch: metasystem/internal/gittree (only if a worktree helper
belongs there; say what)
May touch: metasystem/scripts/agents/land-fixtures.sh (only for the
fixture legs if they are shell-shaped)
Must not touch: the landing evaluator's reading of receipts
(readTestReceipt and observe.go), anything under plans, land.sh.

# Constraints

- Go tests first: `go test -count=1 ./internal/landing` must pass
  before you touch shell.
- The receipt's file format does not change; the evaluator must accept
  a receipt made this way without any change on its side (it compares
  tree, command, exit status, time and the binding trees).
- Bash 3.2 clean for any shell you add. Do not run the full battery in
  your sandbox; the orchestrator runs the real verb seat-side.
- One round, at most 120 minutes of wall clock. Hazard DESIGN-BEARING:
  the receipt is what certifies a full-width landing; an independent
  critique follows.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/landing` (expected: ok)
- `go test -race -count=1 -run 'Receipt' ./internal/landing` (expected: ok)
- `go vet ./internal/landing` and `gofmt -l ./internal/landing` (expected: clean, no output)
- `bash scripts/agents/go-build.sh` (expected: a rebuilt engine line)
- a direct probe: stage any one-line change, take the staged tree's metasystem subtree, run `bin/metasystem landing test-receipt --root . --tree <subtree> --command 'true'` and, in another shell during it or right before, append a line to records/narrator-digest.log; expected: the receipt is green and the live root still shows the appended line
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. A receipt made while the live root's narrator digest changes is
   green and names the candidate.
2. A command that changes the candidate inside the worktree is refused
   with a sentence naming that.
3. The temporary worktree is gone after every run, green or red.
4. `readTestReceipt` and the landing evaluator are unchanged and accept
   the new receipts.

# Gap Rule

stop and report a gap; never fill it silently.
