# Dispositions: kill-shell plan, round 19

Job: design-critic-20260812t014943z-06c1 (codex gpt-5.6-sol, xhigh).
1 finding, 1 material, accepted — by ending the generating cause
rather than shaving the next edge.

## The generating-cause resolution

Rounds 12-19 were one seam: I was reimplementing a Go lexer in awk,
and every round found the next lexical edge (presence vs identity,
main.go's empty imports, string position, data lists, comments,
pipefail, mid-line block comments). The discriminator now lexes
NOTHING: damaged-template means the engine's own source files
(internal/missionrunner/stoploss.go and internal/mission/ledger.go)
present while go.mod does not declare the metasystem module. Adopted
targets receive no Go source; a foreign collision needs both exact
paths and still fails loudly rather than silently. Both branches
executed; there is no lexer left to find edges in.

| id | disposition |
| --- | --- |
| KS-R19-001 | accepted — resolved by the presence-of-engine-source discriminator above; the awk scanner is deleted. |
