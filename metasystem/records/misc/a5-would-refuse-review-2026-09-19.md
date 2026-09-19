# A5 would-refuse review, 2026-09-19

This review uses base `94f27a5fa` and `origin/main` at
`7cfb5a04b242c0852a98e9b4f5fe198785d49192`. The requested log command
finds 53 trailers: 52 unpromoted verdicts and one already promoted
`missing-declaration` verdict. The earlier audit counted 53 unpromoted
verdicts. `origin/main` has moved since that audit, so one
`chain-full-gate-refused` trailer is no longer in the requested date window.

`TRUE` means the agent landing should have stopped. `MISFIRE` means the
current evaluator can stop a normal machinery landing. `PATH-GONE` means a
historical false-producing route was replaced before this base.

| Code | Count | Verdict | Evidence commits | Condition | Fix |
| --- | ---: | --- | --- | --- | --- |
| `chain-not-design-bearing` | 18 | **MISFIRE** | `7db34a272` changed `testing.json` after two named critics. `536c74bd4` changed the Stop renderer and tests after critic `brain-summary-crit1b`. `47e5f2f21` changed goal and report code after closing review `show-cc2b`. | `observeChain` rejects every root whose `destructiveReach` is `MECHANICAL`. It does this before reading independent-critic evidence. A reviewed mechanical chain therefore refuses even when its review is valid. | Replace the hazard-label test with a critique-evidence test. Require the tier-required independent critic, a clean canonical finding register, and closure bound to the final reviewed tree and patch. Use a critique-specific refusal such as `chain-not-critiqued`. Do not admit unreviewed mechanical chains. This is larger than this job's 180-line fix unit; the accepted small-change-lane design allocates 240 lines to its landing unit and depends on its earlier closure and lane units. |
| `chain-full-gate-refused` | 13 | **PATH-GONE** | `723231e66` changed one design page. `7211d7005` changed dispatcher Go and shell code. `ea8c3ead7` changed steward, adoption, and fixture code. | The old path required a literal full-battery receipt for a full-width chain. This base sets `testing.contract=testing.json`, so an ordinary chain now uses schema-2 evidence and reports `chain-test-receipt-refused`. The old code remains for unmigrated installations and legacy recertification, but that is not the route that made these trailers. | No base fix. Keep the legacy refusal until its compatibility path is removed. |
| `chain-open` | 12 | **TRUE** | `3f782639f` landed a seat-awareness design while its message said one design critique was still pending. `ab513438b` landed a design, briefs, and critique records while root `hcl-design6-20260911` was still open. `e68a86f9e` changed goal and metrics code without an authoritative closed root record. | The named root record did not carry JSON boolean `chainClosed: true`. A commit message is not closure evidence. The check is still reachable for any agent `--chain` landing on this base. | No evaluator fix. Close the chain before landing. |
| `chain-test-receipt-refused` | 3 | **PATH-GONE** | `79b0af799` changed proof-runner, fixture, and testing-contract paths with all 42 groups green. `92b31f774` changed four steward paths with all 42 groups green. Unrelated ledger movement invalidated both under the old exact-tree rule. `612b1e578` changed the receipt system and disclosed two insufficient groups, so that refusal was right. | The historical false route compared the whole tree and rejected receipts after unrelated ledger movement. `612b1e578` replaced it with workspace projections and selected-group identities. Current code still correctly rejects absent, malformed, insufficient, stale, or identity-mismatched schema-2 evidence. | No base fix. The false-producing exact-tree route is gone. |
| `chain-output-mismatch` | 3 | **PATH-GONE** | `c1525b90a` changed Stop-hook Go and shell paths after main changed the same files. `6325757a5` changed steward, adoption, and fixture paths after `75ac42e` changed two of them. Those were normal stale-base landings. `0b8d1bea8` changed only four critique-disposition files and omitted the certified goal-budget implementation, so that refusal was right. | The historical false route could not recertify a reviewed patch after harmless base movement on overlapping paths. `fe7212a93` replaced it with explicit no-overlap recertification. Current code still correctly rejects a candidate whose certified paths do not match its reviewed or recertified output. | No base fix. Use recertification after a base move. |
| `chain-has-uncarried-paths` | 2 | **TRUE** | `48d8bc39f` says it carried post-review guidance and touched guidance beside goal, channel, steward, and script paths. `fe7212a93` says it reseeded coverage-ratchet floors in the same commit. Neither provenance trailer declared register carriage. | The candidate contained paths outside the certified set. Only the append-only receipt ledger is automatically exempt. Other additions need certified output or `--direct-fix register-carriage`. | No evaluator fix. Certify the extra paths or declare register carriage. |
| `chain-recertification-test-command-refused` | 1 | **TRUE** | `ce59b313d` changed the human-carried landing system across Go, shell, documents, and manifests. Its message says the deep receipt was red on adoption and Stop-report groups and publication continued only on Wido's word. | The recertified agent landing lacked sufficient evidence bound to its merged tree and command. A human publication exception explains the landed commit but does not make the agent case safe. | No evaluator fix. Produce a sufficient bound recertification receipt. |
| `missing-declaration` | 1 | **TRUE** | `1b12f5349` changed the testing and landing system across many Go and shell paths. Its provenance says `none`; no chain, direct-fix, or attested declaration was supplied. | An agent landing must identify its proof lane. This code was already promoted, and the commit landed as a human publication exception. | No evaluator fix. Supply one valid declaration. |

## Landing paths on this base

The evaluator does not classify a commit from its Git author or its
`Landed-By` trailer. `scripts/agents/commit.sh` calls it for both caller
classes. The wrapper calls a commit an agent commit when the held lease has a
positive claim epoch. It calls the commit human when there is no claim epoch.
Only the wrapper applies C6: a human commit records a would-refuse trailer but
does not stop.

These paths create agent commits that promotion would refuse:

- `scripts/agents/land.sh` reaches `commit.sh`. A held agent lease makes its
  landing an agent commit.
- Batch owner and tick landing use `batch.CommitWithWrapper` for both legacy
  chain units and branch-build units. They set the goal approver as Git author
  and committer and add `Landed-By`, but they still run under the batch owner's
  positive claim epoch. The wrapper therefore treats them as agent commits.

These paths do not run the evaluator:

- `landing batch join` proves and records a unit but makes no landed commit.
- This base has no public `landing batch land` verb. Batch owner and tick do
  the normal landing. Their normal commit path runs the evaluator, but
  `RebuildLandingBranch` is a recovery path that writes commits with
  `git commit-tree` and does not run it.
- `goal land-ready` changes the ledger only. The current single-goal commit
  path is `goal branch land-prep`, followed by `goal branch land-push`.
  `land-prep` writes the landed commits with raw `git commit` in a detached
  worktree. `land-push` publishes them. Neither calls the evaluator. Those
  commits also use the goal approver as author and committer and add
  `Landed-By`; they bypass the evaluator rather than receiving C6 treatment.

## Stop decision

`chain-not-design-bearing` is a current misfire. A one-line admission of all
mechanical chains would remove a required review boundary. The proper repair
must derive the critique duty, validate the independent critic and its clean
register, and bind that evidence to the final conformance output. The accepted
`plans/small-change-lane-design.md` assigns 240 changed lines to unit U5b for
this landing boundary and places it after the lane admission and close units.
That exceeds the 180-line `a5-misfire-fixes` allowance. Under the task's stop
rule, this job stops after review. Promotion remains off and no code or tests
were changed.
