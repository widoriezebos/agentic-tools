# Dispositions: kill-shell plan, round 9

Job: design-critic-20260812t002358z-8c99 (codex gpt-5.6-sol, xhigh).
6 findings, 6 material (one critical), all accepted.

| id | disposition |
| --- | --- |
| KS-R9-001 | accepted (critical) — the engine delivery path for adopted targets is stated: the BINARY is a committed artifact of the adopted payload (as adoption already ships it), because targets receive no Go source and can never rebuild; a fresh CI clone therefore carries its engine. The bootstrap freshness rule is TEMPLATE-ONLY; the adopted update path is re-adoption. |
| KS-R9-002 | accepted — the publication protocol is named: internal/dispatch/ownerlock.go's claim/release over a dedicated bin publication lock directory, then a staged atomic rename — the same primitive the chain and lifecycle locks trust, not a phrase. |
| KS-R9-003 | accepted — bootstraps ARE the custody shape: adopt.sh and go-gate.sh launch a child (the toolchain), wait, consult verbs, and finish — launch-wait-consult custody by its own definition. Their registry verdicts say custody; no fourth shape exists. |
| KS-R9-004 | accepted — function-grain debt gets representation and completion: go-packages entries carry an optional symbols list, each symbol with its own deadline; the definition of done includes zero expired Go debt, template-only like all Go-section enforcement. |
| KS-R9-005 | accepted — export conditions project by RELATIVE PATH identity: the registry stores the relative path, adoption installs at the same relative path when the condition holds, and validation judges optional scripts only when present. |
| KS-R9-006 | accepted — corrected against the native-only rule: Go never launches the toolchain. The gate SEQUENCE stays in the bootstrap (custody shape); the gate's POLICY — check ordering, skip rules, failure classification, the coverage ratchet — is Go verbs the bootstrap consults between steps. r6/KS-R6-007's wording is superseded to this split. |
