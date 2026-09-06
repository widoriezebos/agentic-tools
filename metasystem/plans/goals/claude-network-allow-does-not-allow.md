# claude-network-allow-does-not-allow

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: a builder that needs a module download fails and the record says it had network; novelty 2: the sandbox's allow shape is not yet known; exposure 2: every claude delegate; accumulation 1: nothing compounds"
- Tier: 2
- Intent: A claude delegate whose record says network allow gets a sandbox network block of empty allowedDomains and empty deniedDomains, which BuildClaudeSettings comments as ordinary egress; the live proof critic lpb-proof1 (2026-09-06, settings with that exact shape) had every outbound curl refused as 'deny network-outbound <host>:443 (user denied)'. The claude sandbox treats an empty allow list as no egress, so the record and the comment say allow while the shell is cut off: the KI-12 class the dispatch fixtures were written to prevent (a job recorded as networked and still cut off). Implementers are affected too (a go mod download in a claude builder fails silently). DONE means the recorded effective network for a claude delegate matches the sandbox: either the allow shape becomes a shape the sandbox honours as egress (proven by a probe curl succeeding) or the effective permission is recorded as deny for claude with the comment corrected, and a settings test pins the chosen shape.
- Origin: main
- Next step: Probe the allow shape first (allowedDomains ['*']? a wildcard entry? the CLI's documented default) with the live-probe recipe; then brief, build, land.
- OpenedAt: 2026-09-06T22:29:40Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T22:29:40Z T58VEDBYW6DDY901CQH946W21G-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-network-allow-does-not-allow
Integrity: sha256=36f20215c7d8579b999617baa5f18da95596d75e699a576afcf964f698d2b284
