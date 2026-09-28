# Dispositions: kill-shell plan, round 16

Job: design-critic-20260812t012304z-c65b (codex gpt-5.6-sol, xhigh).
1 finding, 1 material (medium), accepted and fixed in shipped code
with both branches executed before committing: the sentinel now
requires Go IMPORT syntax in Go files — a quoted internal module
path at an import position — so a foreign project mentioning the
path in comments, docs, or constants never trips it, while the real
tree still matches.

| id | disposition |
| --- | --- |
| KS-R16-001 | accepted — import-syntax match, Go files only; positive branch proven against the real tree, negative branch proven against a comment-only mention. |
