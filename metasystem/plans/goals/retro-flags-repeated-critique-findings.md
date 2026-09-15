# retro-flags-repeated-critique-findings

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: a missed recurrence costs a rework round, not data; novelty 2: receipts already carry read and design fields, matching finding ids across records is new; exposure 2: every critique and every retro; accumulation 2: repeated findings pile up unseen across fresh delegates and fresh sessions"
- Tier: 2
- Intent: Fresh design delegates and fresh sessions (goal seats-spend-tokens-in-bounded-sessions) lose the memory of earlier critique findings unless a record carries them. A critique's findings live on the design page's critique record and in records/misc; receipts carry read_tokens and read_calls (e700328e1) and design_tokens and design_calls (fb6839a44) but no finding ids, so the retro cannot see a finding that recurs across goals or across revisions. Wido's word 2026-09-15 (R-115-m1e): we should not regress the functionality because we get more efficient. DONE: every critique and code-read receipt carries the ids and normalized titles of its material findings; the retro reports any finding that appears in two or more critique records of different goals, or of one goal after a fold, naming the goals, the rounds and each disposition; proven by a fixture with two synthetic critique records that share a finding and one retro run that reports it, and one run over differing records that reports nothing.
- Origin: human
- Next step: Design by a fresh, bounded Claude Fable delegate briefed from scripts/agents/templates/design-brief.md (at most 30 calls, 2500 words): the receipt field shape, the matching rule, the retro report line, the witness. Context: skills/retro/SKILL.md, scripts/receipt.sh, the receipt fields of e700328e1 and fb6839a44, records/misc/seats-spend-tokens-in-bounded-sessions-critique-r1.md to r3 as the sample. Then one Codex critique round, then the build.
- OpenedAt: 2026-09-15T14:17:18Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-15T14:17:18Z TZ49S5E3XVPF0Z49AWG1C8XXKY-m1e-c6925449 open actor=human:Wido targets=retro-flags-repeated-critique-findings
Integrity: sha256=994c4ae3dcfad004dbfb865bdad8ccf6fa8d457967c6803def9a3e47b04fee38
