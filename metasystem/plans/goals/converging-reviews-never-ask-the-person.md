# converging-reviews-never-ask-the-person

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Instruction text in two review skills; no engine code; the engine-level criterion stays with critique-stops-on-convergence"
- Tier: 1
- Intent: A review loop that is converging never stops to ask the person; a reviewer such as Astra settles what a reviewer can settle (Wido 2026-10-03: 'I don't understand why I'm asked if we have a reviewer like Astra'; m1h asked him verify/accept/stop after design rounds of 8 then 4 material findings, all fixed). Two rules in the review skills cause it and are changed now, ahead of the engine work in critique-stops-on-convergence: (1) skills/design-critique/SKILL.md 'Declare the Failsafe Round at Loop Start' lets a seat pick an early round (m1h picked 2) at which it stops and asks; a declared round becomes advice, and a loop whose material count is falling continues to the goal's cap; (2) 'Stop on divergence: ... half or more of them sit in what the last fold changed, step back and go to the human' (design-critique and code-critique SKILL.md, Round Budget sections): stepping back means fixing by subtraction and continuing, the person hears of a loop only at the cap. And: a fold no critic has read yet is confirmed by one more round automatically, never by a question. The person is asked only for choices only a person can make (owner preferences, risk acceptance he wants to own), each with its impact in plain English (R-143-m1e).
- Origin: main
- Next step: Edit the two skills (and docs/orchestration.md or internal/protocol/templates/review-brief.md if they carry the same rules); keep every phrase internal/mission/design_round_rule_test.go requires; run internal/mission, internal/audit, internal/adopt and go test ./cmd/metasystem -run 'TestAuditMessages|Skill' before handing in; tell every seat once it lands.
- OpenedAt: 2026-10-03T07:13:17Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-03T07:13:17Z 2E2H3QSECV8SHB2F1TM2JET50A-m1e-718ba0eb open actor=human:Wido targets=converging-reviews-never-ask-the-person
Integrity: sha256=006d71c3da48a7d97cf713eb2de06ca4a831814165903973df04a05ec96d094d
