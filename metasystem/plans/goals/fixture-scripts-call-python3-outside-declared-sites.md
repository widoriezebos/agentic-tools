# fixture-scripts-call-python3-outside-declared-sites

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="The full validation is red on every machine until it is fixed; nothing is destroyed; six line-level replacements."
- Tier: 3
- Intent: The dependency ratchet (scripts/agents/dependency-ratchet.sh) allows python3 at exactly three declared fixture sites (validate-metasystem.sh, preflight-commands.sh, dispatch-fixtures.sh) and refuses any other script that invokes it, in the full validation's shell-and-dependency-audits section. Two fixture scripts landed after the ratchet call python3 undeclared: scripts/agents/channel-fixtures.sh (five heredoc python3 calls reading question JSON fields, landed 2026-09-03 and 09-04) and scripts/agents/supervision-hook-fixtures.sh (one call reading the hooks JSON, landed 09-02). The governed validation on m1c (2026-09-06 15:19Z) is red on that section alone besides the Go gate. DONE means those scripts read their JSON through the engine's json verbs (bin/metasystem json get) instead of python3, or the ratchet's declared list names them with a stated reason if the engine cannot express the read, the audit section is green, and no other undeclared python3 use remains under scripts.
- Origin: main
- Next step: Read the six python3 sites and what each extracts; replace each with bin/metasystem json get --file --field (the engine verb the hook already uses), keep the fixture assertions byte-identical, run the two fixture scripts seat-side and the dependency ratchet; one small chain.
- OpenedAt: 2026-09-06T15:20:55Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T15:20:55Z AD3DY0S3Q42EKXGZN1385QHXJV-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=fixture-scripts-call-python3-outside-declared-sites
Integrity: sha256=e91f16f52fd676500525622c98a9ef2d3d9a2752b5319837c1bbdbee1b6652ea
