Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger, live proof of chain dcwl-build3-20260911, final work round 3)
Date: 2026-09-10

# Goal

Goal delivery-candidate-is-the-workspace-not-the-ledger
(metasystem/plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md):
a landing receipt must survive a goal-ledger publish by another seat.
The chain has been built and read; this job is its live proof, the
evidence a DESTRUCTIVE-REACH chain needs to close. A first live proof
(dcwl-verify1-20260911) of the same candidate without the six closing-read
fixes observed steps 1, 2, 3, 5 and 6 as designed; this proof covers the
final tree, and adds step 7. You are the verifier:
drive the changed behaviour through the real command surface and report
what you observed. Do not certify; do not infer from green tests.

# The behaviour to observe

With the engine built from the chain (below), in a scratch contract-bearing
repository with a bare origin and two clones (the land bed's own recipe,
metasystem/scripts/agents/land-fixtures.sh, sections the design names in
8.1):
1. Clone one stages a change and takes a receipt with
   `landing test-receipt --root . --tree $(git write-tree) --mode auto --goal fx`;
   the receipt carries a `workspace` object whose `tree` equals
   `landing workspace --root . --tree <receipt tree>`.
2. Clone two runs one goal verb and pushes (tip A becomes B, only
   `plans/goals/` differs). Clone one lands with
   `land.sh --chain <chain> --test-receipt <receipt> --staged-only --skip-transport`:
   exit 0, origin's `main` equals clone one's HEAD, `HEAD^1` is B. Before
   the chain (the trunk engine, `metasystem/bin/metasystem`) the same
   landing refuses with "names tree X but the staged candidate is Y".
3. Clone two instead pushes a new file under `records/misc/`; clone one's
   landing still lands, and `land.out` shows site 8 reached `test verify`.
4. Clone two instead appends a line to a declared input that is NOT an
   engine path: the bed's own `scripts/application-input.txt` (the land
   bed's `input-move-refuses` leg does exactly this; `scripts/agents/**`
   is an engine path and would refuse earlier through the engine binding,
   as the first live proof found). Clone one's landing refuses,
   `land.out` carries `proof-input-moved-after-receipt`, the group id and
   that path, and origin's `main` is still B.
5. `landing workspace --tree` prints the same id for two trees that differ
   only under `plans/goals` and `records/counselor`, and different ids
   when `internal/` differs.
6. The engine of 6bc19ba1c (the bed builds it in the `receipt-cutover`
   leg) refuses a receipt carrying `workspace` with
   `chain-test-receipt-refused`, and observes its own receipt for the same
   tree at the same path.

7. `landing test-receipt --root . --mode auto --goal fx --cap-min 1`
   WITHOUT `--tree`, in clone one before any move, publishes a receipt at
   `artifacts/agents/landing/receipts/<index tree>.json` (the closing
   read's DCW-19, fixed in this chain).

# Facts

- The chain's engine, built from this chain's reviewed round-1 tree 3eeab385ca4d182a41a1b3c2d59071d0d26f0713, is
  at /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1d/1256ce4b-2cff-499e-9e81-ed1cf7deb77a/scratchpad/dcwl-proof-engine-3. Use it as METASYSTEM_BIN and as the binary you invoke
  for every step; for the "before" contrast in step 2 use
  metasystem/bin/metasystem.
- The land bed already builds every piece of this drive: read
  metasystem/scripts/agents/land-fixtures.sh for the seed, the peer clone
  and the five legs, and reuse its helpers rather than inventing a bed.
  Running the bed's legs `ledger-move-lands`, `records-move-lands`,
  `input-move-refuses` and `receipt-cutover` with the proof engine as the
  bed's source engine, and reading their logs, is an acceptable drive
  for steps 1 to 4 and 6, provided you report each step's own observed
  output, not the bed's summary line.
- Do not touch this checkout's live ledger, claims or receipts. Everything
  lives under the temp directory and is removed when you are done.
- The delegate sandbox denies process enumeration; if a step needs it,
  report exactly which step and what was denied and stop there.

# Decisions (the orchestrator's; decided, not open)

D1. Evidence level for every step is `ran` with the exact command and the
observed output or exit code, captured from the verifying command's own
exit code, never from a wrapper. Each of the seven behaviours is one
evidence entry; the "before" contrast of step 2 is one more.

D2. You change nothing under the checkout.

D3. If any observation contradicts the expected behaviour, stop and report
it as the riskiest part; do not work around it.

# Constraints

Read-only on the repository. Return per the verifier schema. Wall-clock
budget: 60 minutes. Plain English in every human-visible field.
