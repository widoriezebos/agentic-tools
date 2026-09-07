# rounds-prove-with-the-smallest-run

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="severity 2: no wrong code ships, but the verification cost per round is multiplied for no gain, which is how a two-hour fix becomes a day; novelty 1: the rule already exists, it is not mechanized; exposure 3: every chain on every seat; accumulation 3: this is the SECOND time the same class has cost a full day and it recurs whenever a seat is under pressure, which is exactly when it costs most"
- Tier: 3
- Intent: THE BATTERY MISTAKE AGAIN, in Wido's words 2026-09-07: 'this feels a lot like you are repeating the battery mistake again: run full very expensive test suite just to prove a small change.' R-21-m1 already retired the battery orchestration for manufacturing its own defect class, and its lesson was that expensive validation belongs at weight-triggered milestones, not per change; the canary ruling says cheap first test before full validation, applied by default. On goal metasystem-stop-verb 2026-09-06/07 the orchestrator ran all five process-owning fixture beds and the eleven-package matrix on nearly every one of twenty trees, about forty minutes each, where the single bed a change touched would have taken eight, and where the single SCENARIO would have taken one. Nine of those rounds fixed one to three lines. The mechanism to run one scenario existed all along (scripts/agents/<bed>.sh --fixture-bed-child <scenario> <capability-file>, capability minted by harness_fixture_bed_mint_capability) and the orchestrator found it at round seventeen by reading the harness. DONE means the smallest proving run is named in the round's brief and reported in its return, the full acceptance is demanded only at the close, running one scenario is a documented one-liner rather than a rediscovery, and the standing rule is written where a seat under pressure will hit it rather than in a ruling it has to remember
- Origin: main
- Next step: three pieces, all small: (1) document the single-scenario invocation in each bed script's header and in docs/orchestration.md; (2) add a smallest-proving-run line to the brief template beside the verification section, so every brief states what would fail on THIS change; (3) state the standing rule beside the canary and battery rulings: the round proves its own change with the smallest run that can fail on it, the orchestrator proves the slice once at the close, and an expensive suite per change is the retired battery pattern under a new name. Sibling: delegate-sandbox-cannot-run-the-beds removes the round trip; this one removes the waste inside it
- OpenedAt: 2026-09-07T20:10:31Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T20:10:31Z H84AJYWT1J928X9TJC98PC1S87-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=rounds-prove-with-the-smallest-run
Integrity: sha256=3ee241ccf3f549dfcacc873747b323c48ffd9d0a4b20242ed33eb030617f4a8a
