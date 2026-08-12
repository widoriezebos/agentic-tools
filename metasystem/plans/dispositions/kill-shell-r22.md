# Dispositions: kill-shell plan, round 22

Job: design-critic-20260812t022546z-123f (codex gpt-5.6-sol, xhigh).
4 findings, 2 material; all four accepted.

| id | disposition |
| --- | --- |
| KS-R22-001 | accepted — the three-state discriminator and toolchain-presence checks are PRE-BINARY decisions and stay in the bootstrap BY NECESSITY, exactly like compile-and-consult: no trustworthy Go binary can exist when they run. The Phase E split is restated: pre-binary guards live in the custody-shape bootstrap; post-binary policy (ordering, ratchets, classification) is verbs. |
| KS-R22-002 | accepted — the fence's evidence source is named: registry entries carry a CALLERS manifest recorded by the Phase 0 sweep, and the audit verb re-verifies each recorded caller still exists and still references the script; an entry with an empty callers list must be tombstoned or carried as debt. Reachability claims now have a durable, checkable source. |
| KS-R22-003 | accepted (non-material) — the tracked-file-allowlist wording updates to the registry-as-manifest rule it was superseded by. |
| KS-R22-004 | accepted (non-material) — the collision sentence now says what round 20 settled: a sentinel collision fails LOUDLY; nothing claims it never trips. |
