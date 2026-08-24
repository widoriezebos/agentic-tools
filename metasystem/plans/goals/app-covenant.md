# app-covenant

- State: claimed
- Intent: A versioned per-app covenant binds intent to proofs: requirements to executable specs, budgets to metrics, golden sets with provenance, and the battery that earns green — the interface between builder and app; content stays app-side (benchmark-specifics-stay-in-kit, generalized)
- Origin: human
- Next step: HALF-DAY EXTENSION IN PROGRESS (claimed m1, ruling adopted). THE SIX, with their fix map: (H1) internal/covenant/covenant.go Read — replace the Lstat-then-ReadFile pair with one os.OpenFile(O_NOFOLLOW) and read from the handle (the missionrunner covenantgate's canonical-omission probe keeps its own Lstat semantics — check both sites). (H2, RULED AS WRITTEN) covenant self-custody: (a) mission.VerifiedGuardrails / the wall's guardrail consumption treats covenant.json as a MEMBER OF EVERY GUARDRAIL CLASS BY CONSTRUCTION — appended after parse, not read from the contract, so no contract edit can drop it; (b) the issuance side refuses any host authorization DECLARING covenant.json (host-artifact refusal beside the existing guardrail-touch scan in internal/validate/authorization.go); (c) identity/battery changes escalate to the human tier — the warden/human-tier lane from custody slice two refuses a warden-lane record whose diff touches covenant identity or battery rows (locate the human-tier half landed in custody slice two and add covenant.json's identity+battery fields to its escalation predicate). Fixtures: a contract omitting covenant.json from its guardrail classes still refuses an unmarked covenant write; a host declaration naming covenant.json refuses at issuance; an identity change under a warden-only chain refuses naming the human tier. (M1) covenant battery metric AND command keys validate against the contract KEY grammar (reuse contract's key validator, not non-emptiness). (M2) ParseGuardrails on the covenant side gets the same protected-path predicate the contract side passes (the covenant reader currently passes a nil/permissive predicate — thread the real one). (M3) covenantPreflight binds the gate.threshold.* SET: any threshold key beyond the covenant's declared metrics refuses by name (today only the declared metric is checked present; extras change what earns green silently). (L1) the two stale sentences — grep the covenant package and covenantgate for the pre-round-three wording (the omission-refusal comment and the battery-binding doc line) and fix. Battery + gate + codex to AGREE + land; blown appetite = PARK, no third budget.
- OpenedAt: 2026-08-23T16:29:46Z
- Revision: 8
- Claimed: machine=m1 lineage=coordinator at=2026-08-24T09:50:24Z

History:
- 2026-08-23T16:29:46Z X2Y67SRMJJJ3T51WY03MAZSBDM-m1-bf243850 open actor=human:wido targets=app-covenant
- 2026-08-23T16:30:50Z EYH0NA8A9WYFZ2T4CAG3S89WZZ-m1-bf243850 edit actor=human:wido targets=app-covenant
- 2026-08-23T22:06:22Z ZTAF6JW3PFHSPMKN9QXYBSSXF6-m1-bf243850 claim actor=m1+coordinator targets=app-covenant
- 2026-08-24T00:33:28Z PVH3FJNS6GMBM1ENPM3F2Z6JKD-m1-bf243850 edit actor=m1+coordinator targets=app-covenant
- 2026-08-24T00:38:06Z 2Q13K1V9J86EB8FS54P8TQT7B0-m1-bf243850 release actor=m1+coordinator targets=app-covenant
- 2026-08-24T09:23:16Z VEEBVFE3GNTXGQXWJEQGN9TYVK-m1-bf243850 edit actor=m1+coordinator targets=app-covenant
- 2026-08-24T09:50:24Z 8NCQ7H0TT10T6B9J91PKSSAHDE-m1-bf243850 claim actor=m1+coordinator targets=app-covenant
- 2026-08-24T09:51:28Z 1HVQWMDMVBXQQ5JPZN17MBGWQH-m1-bf243850 edit actor=m1+coordinator targets=app-covenant
Integrity: sha256=b04ec842b1f70e45260180163e3f2a600bc8b1d62f1423a5ab1b6f882574920b
