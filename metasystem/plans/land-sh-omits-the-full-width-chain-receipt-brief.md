Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal land-sh-omits-the-full-width-chain-receipt, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

A reviewed implementer chain whose goal has accumulation 2 or higher
carries `gateWidth: full` on its root job record, and the landing
evaluator (the width==full branch in
metasystem/internal/landing/observe.go, around line 147) refuses it
with chain-full-gate-refused unless a test receipt for the full battery
command is supplied through `--test-receipt`. The landing script
metasystem/scripts/agents/land.sh accepts `--test-receipt` nowhere: it
creates and forwards a receipt only in its tier-1 lane
(`create_test_receipt`, run when `--tests` is given, and `--tests` is
refused outside `--direct-fix tier-1` at lines 114 to 121), so
`land.sh --chain <root>` cannot land a full-width chain at all. On
2026-09-06 two such chains (rgr-build1 and shr-build1, and a third,
bcd-build1) had to be landed by calling
metasystem/scripts/agents/commit.sh directly with `--chain`, `--goal`
and `--test-receipt`, followed by a manual fetch, rebase and push, which
skips land.sh's verify, clean-tree and push-retry steps. The
human-carried-landing design in
metasystem/plans/human-carried-landing-design.md says the receipt step
covers "the full battery under width full (land.sh's receipt step)".

When you are done, `land.sh --chain <root>` lands a full-width chain
through land.sh with a receipt the caller made beforehand, refuses
early and plainly when the root is full-width and no receipt was given,
and a landing fixture pins it.

# The design

1. `land.sh` accepts `--test-receipt <path>` together with `--chain`
   (and only with `--chain`; with `--direct-fix tier-1` the receipt is
   still created by `--tests`, and giving both is a usage error). The
   path is forwarded to commit.sh unchanged in `commit_changes` (line
   285 already forwards `landing_test_receipt` when set; set it from
   the flag). Usage text names the new flag.
2. Early refusal: before the verify step, when `--chain` is given,
   land.sh reads the chain root's job record
   (`artifacts/agents/jobs/<root>.json`, field `gateWidth`) and, when
   it is `full` and no `--test-receipt` was given, refuses with:
   "land refused: chain <root> is full-width (its goal's accumulation
   is 2 or more); make the full battery receipt for the candidate tree
   first (metasystem landing test-receipt --root . --tree <subtree>
   --command \"<the full battery command>\") and pass it with
   --test-receipt" and exit 2. The full battery command is the constant
   in metasystem/internal/landing/tierone.go; do not duplicate its
   text in land.sh by hand: read it from the engine if a verb prints
   it, otherwise name the source file in the message and keep the
   literal in one place (say which you did and why).
3. The receipt's tree is the metasystem subtree of the staged candidate
   (the same `git rev-parse <write-tree>:<prefix>` land.sh's
   `create_test_receipt` already computes); when `--test-receipt` is
   given, land.sh checks before committing that the receipt file's
   `tree` equals that subtree and refuses with "land refused: the
   receipt at <path> names tree <a> but the staged candidate is <b>;
   make the receipt against this exact candidate" otherwise. This is
   the check that today surfaces only as the evaluator's late
   chain-output or receipt refusal.
4. A landing fixture: in metasystem/scripts/agents/land-fixtures.sh add
   one scenario, `full-width-chain`, shaped like the existing
   `tier-one` scenario: a chain root record with `gateWidth: full`, a
   receipt made against the exact candidate subtree with the full
   battery command (use a trivially true command through the same
   verb only if the fixture bed cannot run the real one; say which),
   and three legs: land.sh refuses early without a receipt with the
   message above; refuses a receipt whose tree differs; lands with the
   matching receipt and the commit carries the pass-bar-a provenance.
   Register the scenario in the bed's list and its success line.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/scripts/agents/land.sh
May touch: metasystem/scripts/agents/land-fixtures.sh
May touch: metasystem/scripts/agents/commit.sh, only if forwarding
needs a change there (say what).
Must not touch: the landing evaluator in metasystem/internal/landing,
anything under plans, any other file.

# Constraints

- Bash 3.2 clean. Do not weaken any existing landing fixture leg.
- Do not run the whole land bed if your sandbox cannot make temporary
  Git repositories; run the commands below and say so. The orchestrator
  runs the bed seat-side.
- One round, at most 90 minutes of wall clock. Hazard DESIGN-BEARING:
  the landing lane changes; an independent critique follows.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/land.sh` and `bash -n ./scripts/agents/land-fixtures.sh` (expected: exit 0)
- `bash ./scripts/agents/land.sh 2>&1 | head -3` (expected: the usage line naming --test-receipt)
- `grep -n 'test-receipt' ./scripts/agents/land.sh` (expected: the flag, the early refusal, the tree check, the forwarding)
- `grep -n 'full-width-chain' ./scripts/agents/land-fixtures.sh` (expected: the list entry and the scenario)
- `bash ./scripts/agents/land-fixtures.sh` if your sandbox allows it (expected: the land fixtures pass with the new leg counted)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. A full-width chain lands through `land.sh --chain <root> --test-receipt <path> --staged-only` with pass bar a.
2. Without a receipt, land.sh refuses before any commit with the named message; with a mismatched receipt it refuses naming both trees.
3. The tier-1 lane is unchanged; giving --tests and --test-receipt together is a usage error.
4. The new fixture scenario pins all three legs and the bed's count line includes it.

# Gap Rule

stop and report a gap; never fill it silently.
