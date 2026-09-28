# Dispositions: kill-shell plan, round 14

Job: design-critic-20260812t010648z-0bed (codex gpt-5.6-sol, xhigh).
1 finding, 1 material, accepted and fixed in shipped code in the
same fold.

| id | disposition |
| --- | --- |
| KS-R14-001 | accepted — the damage discriminator becomes collision-proof: damaged-template means a cmd/metasystem/main.go that IMPORTS the metasystem's own module path while go.mod does not declare it. No foreign project can legitimately import an unvendored module path, so an adopter's own file at that path never trips the gate. Shipped in go-gate.sh and validate-metasystem.sh. |
