# Dispositions: kill-shell plan, round 15

Job: design-critic-20260812t011612z-d70c (codex gpt-5.6-sol, xhigh).
1 finding, 1 material, accepted and fixed in shipped code — the
critic executed the predicate and proved it dead: main.go imports no
internal package (the verb table lives there, the handlers in
sibling files), so the round-14 discriminator always skipped.

| id | disposition |
| --- | --- |
| KS-R15-001 | accepted — the sentinel widens to the whole command directory: damaged-template means ANY cmd/metasystem source importing the metasystem's internal module path while go.mod does not declare it. Verified against the real tree before committing this time. |
