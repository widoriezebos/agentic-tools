# Brief: briefs-carry-their-rules B2, correction 1

Working Mode: Implement
B2 is uncommitted in this worktree. Its read found one material defect (a probe ran ReadReviewBriefAdmission with real git over all 184 plans/*brief*.md: 165 refused on this change against 67 on base, including the B2 brief itself); fix exactly this.

internal/dispatch/brief_citation.go:15 (briefCitationPath) and internal/dispatch/brief.go:689 (briefAuthorityPathEligible now returns briefCitationPath) dropped the directory allowlist, so any prose token with a `/` (`and/or`, `read/write`), any bare name ending in .go/.md, and any mention of metasystem.conf.local must exist on the base, and an agent's brief is refused with a remedy that would delete a required line ("Never open any metasystem.conf.local"). Count something as a code citation only when it has a line suffix, or is in inline code or a link, or has a path separator and its first segment exists in the base tree; a mention of metasystem.conf.local without a line suffix is skipped (never opened), never refused. Add those prose cases to TestWorkBriefResolvesAndChecksBaseCitations, and one case running the admission over a brief shaped like plans/lane-policies-and-helm-u1-settings.md (accepted). Mutation: the allowlist-free rule -> red.
Also (not material, cheap): delete treeHolds (brief.go:702, no callers); avoid a full ls-tree -r per absent candidate (one tree listing per admission).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
