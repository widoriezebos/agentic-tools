# Bounded unit checks

- Owner: Astra, branch `chain-c1b`, direct build requested by Wido.
- Status: declaration lifecycle implemented; bounded execution is unavailable and refused before admission writes a baseline, proposal or snapshot. Returning to legacy permits the same build to proceed; carry flags unavailable units and continues. The default remains legacy.
- Contract: `proof.check-kind` accepts `legacy` or `bounded`; `proof.check-budget` is positive execution minutes, default six. Both travel with admission, frozen rounds and inverse restoration. Default legacy declarations keep their old serialized form for carried-gate digests.
- Correction 1: the capability decision remains in `UnitCheck.RequireSupportedKind`; admission invokes it before saving. The owner states nobody ran the original declaration build, so no persisted-record migration is needed.
- Review first: `internal/launch/unit_declaration.go` and the resume comparison in `internal/processchange/check.go`. Resume must decode an act separately from the caller's frozen declaration.
- Next build: connect prerequisites and nomination-first/changed-package feedback to these settings; implement the execution budget, process-tree settlement, admission-wait recording, ordinary-round continuation and status. Publication and carry must obtain their own sufficient gate when feedback is incomplete.
- Still owed: builder and reader nomination transport, builder instructions, bounded mixed-version record safety, all bounded execution/continuation fixtures, and the two additional carry route fixtures from the first independent critique. No lane changes or default switch belong in this slice.
- External brief and report: `lane-test-20261009/briefs/chain-C1-slice2-brief.md` and `lane-test-20261009/panes/chain-c1b-report.txt` in the shared evidence directory. The report owns exact verification results and timing.
- Waiting: no user decision is needed for this partial build. Independent review, the remaining implementation and the owner's complete proof remain separate work; no push is authorized.
