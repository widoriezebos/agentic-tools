# no-code-carries-the-relayed-word-path

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: this deletes authority and identity code, so a wrong cut breaks arming or rearm, and dropping a record field that landed records already carry would make the whole ledger refuse to parse; novelty 2: the path is already retired by R-82-m1b so no behaviour has to be redesigned, but the read-old-write-none split has to be got right; exposure 3: 16 production Go files, 14 test files, one fixture script and two governance constants; accumulation 2: retired code left in the tree does not grow, but every seat that reads it has to rediscover that it is dead"
- Tier: 3
- Intent: Wido on 2026-09-20: "Remove the temporary word functionality. I need clean code." R-82-m1b retired the relayed-word path on 2026-09-07 with the horizon left at 2026-09-06 and NOT renewed, and that row ordered an enforcement sweep the same hour. The sweep stopped at the fixture legs (fixture-review-by-date-expired, landed 423e6ed6). Measured at 30f41710a, --temporary-human-word and its constants still occupy 16 production Go files, 14 test files and metasystem/scripts/agents/goal-cli-fixtures.sh, and the engine already ships its own off-ramp at metasystem/cmd/metasystem/goal_refusal.go:30.
- Origin: human
- Next step: MECHANICAL with one hard constraint, one chain. Delete the flag and its plumbing (metasystem/cmd/metasystem/goalsync_mutations.go:209 and :1057, metasystem/cmd/metasystem/goal_refusal.go:30 and :101), the validator and its proof seam (metasystem/internal/humanauthority/authority.go), the horizon and ruling constants (metasystem/internal/governance/types.go:103), the horizon-passed message (metasystem/internal/goal/file.go:302), the identity carry (metasystem/internal/steward/identity.go:66, metasystem/internal/steward/runner.go:344 and :858) and the test files that only cover the removed path; take the flag out of metasystem/scripts/agents/goal-cli-fixtures.sh. THE CONSTRAINT: stop WRITING the relayed-word record fields, keep READING them, because landed goal records already carry them and a closed grammar that forgets a landed field refuses the whole ledger, which cost two refusals on 2026-09-20 alone (Approved: unknown key "episode" and the - Risk: line in goal migrate). Then mark R-32-m1 swept in metasystem/memory/rulings.md under R-82-m1b, so no row points at deleted code.
- OpenedAt: 2026-09-20T07:22:42Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-20T07:29:21Z revision=2 opid=3FQNMDJV18HSMCDRMNAX1EG2XW-m1c-c6925449 authority=proven digest=cc123415c657a853d09549cbf1a3d3240f1f43826de3b7224302e2671b94d399 episode=2

History:
- 2026-09-20T07:22:42Z ZECX9M3YRTTZ56A9D80KGG5APS-m1c-c6925449 open actor=human:Wido targets=no-code-carries-the-relayed-word-path
- 2026-09-20T07:29:21Z 3FQNMDJV18HSMCDRMNAX1EG2XW-m1c-c6925449 approve actor=human:Wido targets=no-code-carries-the-relayed-word-path
Integrity: sha256=8cb6d816ef8c12c48e1d10ece0646c8f6d4a70e7cdae640eae20e88d1cacc7c2
