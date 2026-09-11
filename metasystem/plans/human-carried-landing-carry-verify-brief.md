Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry, live proof of chain hcl-build3-20260911, final work round 6)
Date: 2026-09-11

# Goal

Goal human-carried-landing-carry
(metasystem/plans/goals/human-carried-landing-carry.md): a verified human
can carry one landing past one named refusal or one named testing group,
with the review deferred into an obligation on the goal, every use counted
and spoken, a cap on open carries per seat and no stacking on unpaid carry
debt. The chain has been built and read; this job is its live proof, the
evidence a DESTRUCTIVE-REACH chain needs to close. You are the verifier:
drive the changed behaviour through the real command surface and report
what you observed. Do not certify; do not infer from green tests.

# The behaviour to observe

With the engine built from the chain (below), in a scratch
contract-bearing repository with a bare origin and two clones (the land
bed's own recipe, metasystem/scripts/agents/land-fixtures.sh, the seed of
`make_leg` and `arm_receipt_runner`), the design
metasystem/plans/human-carried-landing-carry-design.md revision 6 names
these observable steps:

1. The word (HCL-WORD-04). `goal carry --fixture-human-authority` on a
   goal writes a `carry` history row naming one refusal code or one
   `group:<G>`, the workspace tree of the staged candidate
   (`landing workspace --tree`), the seat, and `authorityGeneration`;
   `landing carry-status --ref <opid>` reports `open`.
2. The carried landing (HCL-LANDING-05, HCL-TRANSACTION-06). A candidate
   whose landing refuses with exactly the named code lands with
   `land.sh --carried <opid>`: exit 0, origin's `main` moves, the commit
   carries the `Carry: <opid>` trailer and the page's other trailers, the
   goal gains the `carrying` row before the push and the `carried` row
   after it, the review obligation `human-carried` is open on the goal,
   the counselor `carried-landings` line is written once, and
   `landing carry-status` reports the terminal state.
3. One word, one defect (the match rule). The same word against a landing
   with a different refusal code, or with a second failing group, asks
   with `carry-refusal-mismatch` (exit 3), no commit, no row; a landing
   that does not refuse at all asks `unneeded`, and the word stays open.
4. The cap and the debt (HCL-COUNTER-08). A second `goal carry` on the
   same seat with a word open asks at the cap; clone two's carried
   landing while clone one's `carrying` row is open asks
   `carry-debt-unpaid` naming that row; the same when clone one's word
   is abandoned or expired before its `carried` row (the third debt arm,
   HCL-C-33).
5. Consumption and supersede. After step 2, `goal carry --supersede
   <opid>` of the consumed word refuses naming the commit; a superseded
   word's landing exits 3 with no commit, no row, no transport.
6. The obligation (HCL-OBLIGATION-07). `goal done` refuses while the
   carry word or the `human-carried` obligation is open; a discharge
   through `goal accept-risk` writes the carried accepted-risk record and
   `done` then passes.
7. The fence (HCL-IDENTITY-02). The trunk engine of ab513438b
   (metasystem/bin/metasystem) reading the fixture ledger after the
   format raise refuses to read the new row the way the page says, and
   `goal carry` on a ledger not yet raised asks for `--raise-format`.
8. The commit subject (HCL-CRITIC-09). `dispatch.sh --reviews commit:<sha>`
   for the code-critic role composes a brief that names the commit;
   the dispatch bed's scenario shows it.

# Facts

- The chain's engine, built from the reviewed round tree bcaa8063840666045a054b365e38b77df550d8e0,
  is at /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1d/1256ce4b-2cff-499e-9e81-ed1cf7deb77a/scratchpad/hcl-proof-engine. Use it as METASYSTEM_BIN and as the binary you
  invoke for every step; for the "before" contrast of step 7 use
  metasystem/bin/metasystem.
- The land bed's new legs (`carried-fresh`, `carried-chain-group`,
  `carried-forward`, `carried-crash`, `carried-asks`,
  `carried-record-failures`), the goal-cli bed's scenarios (`carry-word`,
  `carried-record`, `carried-discharge`) and the dispatch bed's commit
  scenario already drive every step above. Running those legs with the
  proof engine as the bed's source engine and reading their logs is an
  acceptable drive, provided you report each step's own observed output
  and exit code, never a bed's summary line.
- Do not touch this checkout's live ledger, claims or receipts.
  Everything lives under the temp directory and is removed when you are
  done. Fixture clones armed with `steward arm --repo` are ended with
  `steward disarm --repo`, never `stop --repo`.
- The delegate sandbox denies process enumeration and object-store writes
  in linked worktrees; if a step needs either, report exactly which step
  and what was denied and go on to the next step.

# Decisions (the orchestrator's; decided, not open)

D1. Evidence level for every step is `ran` with the exact command and the
observed output or exit code, captured from the verifying command's own
exit code, never from a wrapper. Each of the eight behaviours is one
evidence entry; the "before" contrast of step 7 is one more.

D2. You change nothing under the checkout.

D3. If any observation contradicts the expected behaviour, stop and report
it as the riskiest part; do not work around it.

# Constraints

Read-only on the repository. Return per the verifier schema. Wall-clock
budget: 60 minutes. Plain English in every human-visible field.
