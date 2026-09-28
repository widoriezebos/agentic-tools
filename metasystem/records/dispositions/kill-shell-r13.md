# Dispositions: kill-shell plan, round 13

Job: design-critic-20260812t005756z-9108 (codex gpt-5.6-sol, xhigh).
2 findings, 2 material, both accepted and fixed in shipped code in
the same fold.

| id | disposition |
| --- | --- |
| KS-R13-001 | accepted — the lease-succession fixture's go.mod presence gate was the sweep's last straggler; it now keys on the metasystem module line like every other template-only surface. |
| KS-R13-002 | accepted — the discriminator becomes three-state: metasystem module line → run; no metasystem Go source → skip (adopted); Go source present WITHOUT the module line → fail loudly as a damaged template. go-gate and the suite both refuse the false-green path now. |
