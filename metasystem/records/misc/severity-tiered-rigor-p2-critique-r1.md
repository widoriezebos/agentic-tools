# Risk basis design (revision 4) — Fable critique, round 1

Critic: claude-fable-5-1 design-critic (job str-p2-design-cc1,
DESIGN-BEARING, reading only, 30 min) over
plans/severity-tiered-rigor-p2-design.md revision 4 at 8157e591. Ten
findings, nine material (two critical, four high, three medium), one
mechanical. Persisted by the m3 seat, 2026-09-03 22:20 local. This is
the ONE design round of this slice (Wido, R-60-m1; the brief said so
before dispatch): every finding is folded into revision 4.1 of the
design and becomes a named fixture obligation of the build brief. The
verbatim return is artifacts/agents/jobs/str-p2-design-cc1.json and its
round-1 return.json.

| finding | severity | disposition | fixture obligation |
| --- | --- | --- | --- |
| STR4-RISK-SHAPE-001 novelty and exposure scales re-imported the shape classifier | critical | accepted; scales re-scored by the answer (precedent examined; seats reached), path classes are evidence only | STR4-R1-SHAPE-FREE-DERIVATION: same file set scored 1,1,1,1 derives 1 and 3,1,1,1 derives 3 |
| STR4-ACCUMULATION-GATE-002 gateWidth had one consumer; battery not canonical | high | accepted; three consumers (tier-1 receipt, chain membership, composed brief) and one canonical command string | STR4-R1-FULL-WIDTH-CHAIN: tier-2 chain under width full lands only with a full-battery receipt; area receipt refused |
| STR4-POST-CLAIM-TRANSITION-003 raise after claim undefined | critical | accepted; one transaction, digest re-bound with `raise=<opid>`, claim re-bound at the new revision, running roots keep their tier | STR4-R1-RAISE-TRANSACTION: raise after claim; old root keeps goalTier, next dispatch reads the new tier, validate admits the re-bound digest only with the Misclassified line |
| STR4-DOWNGRADE-PATH-004 pair could lower via override edit or accumulation | high | accepted; any lowering of a score, tier or width is a human act | STR4-R1-FOUR-DOWNGRADES-REFUSED: four pair edits, four refusals |
| STR4-RISK-DIGEST-MIGRATION-005 nil-Risk digest undefined | high | accepted; nil Risk contributes no bytes, part one's digest unchanged | STR4-R1-NIL-RISK-DIGEST: a part-one approval validates unchanged in both modes |
| STR4-SWEEP-POPULATION-006 sweep skipped tiered riskless goals | medium | accepted; sweep selects every goal without a Risk record; lower derivations listed as human decisions | STR4-R1-SWEEP-BACKFILL: tiered riskless goal in the draft; lower derivation never lowered by confirm |
| STR4-MISCLASSIFICATION-EVIDENCE-007 evidence had no grammar | high | accepted; `root:`, `finding:`, `refusal:` kinds checked at edit time | STR4-R1-EVIDENCE-GRAMMAR: each kind accepted with an existing referent, refused with a missing one |
| STR4-COUNSELOR-COUNTABILITY-008 reader contract undefined | medium | accepted; register writer and reader kind move to slice 2b beside the accepted-risk writer; 2a writes the history line only | STR4-R1-MISCLASSIFICATION-KIND (slice 2b): strict reader opens both kinds |
| STR4-EXCEPTION-DIMENSIONS-009 counter missed elapsed and active jobs | medium | accepted; all five members | STR4-R1-FIVE-MEMBER-EXCEPTIONS: an over-box elapsed limit increments |
| STR4-MECHANICAL-010 stale HEAD cite, moved alias goal file | non-material | cites re-read at the reviewed base | none |

Not disputed: none. Nothing goes back to Wido from this round; the
scope, the box and the deadline are unchanged.
