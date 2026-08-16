# Goals

## Current goal: acp-transport — ACP as the delegate transport, retiring the dangerous-mode waiver
- Origin: main
- Next step: Prototype against devin acp per plans/backlog-notes.md item 18; design loop before build.

## Queued goal: disk-hygiene — Every byte the metasystem writes gets a declared lifecycle with janitor enforcement
- Origin: main
- Next step: Design per plans/backlog-notes.md item 19; headroom checks in suite and provision paths.

## Queued goal: narrator — A plain-English story of what happened, why it happened, and who did it
- Origin: main
- Next step: Design per plans/backlog-notes.md item 20 when picked up; distillation over new capture.

## Queued goal: two-bars-for-changes — Design changes take the loop, mechanical defects take a direct fix, declared and audited
- Origin: main
- Next step: Design per plans/backlog-notes.md item 2; the never-direct-fix list is the core.

## Queued goal: fixtures-as-arbiter — A principled critique exit: falling trajectory folds to named fixtures plus code critique
- Origin: main
- Next step: Generalize the one-instance rule per plans/backlog-notes.md item 4 into the critique skills.

## Queued goal: qualified-name-sweep — Agent-facing prose speaks qualified names where a project owns the bare words
- Origin: main
- Next step: Sweep templates, roles, skills, docs per plans/backlog-notes.md item 5; add the audit check.

## Queued goal: critique-stop-rule — A chain closes on zero unrefuted material findings with evidence-carrying refutations
- Origin: main
- Next step: Land the sharpening per plans/backlog-notes.md item 7 in the critique skills and join script.

## Queued goal: ki-23-acknowledged-process — The acknowledged-process mechanism for KI-23
- Origin: main
- Next step: Design per plans/backlog-notes.md item 8 now that the one-writer implementation landed.

## Queued goal: mission-completion-protocol — The mission-completion protocol with its seven carried findings
- Origin: main
- Next step: Resume plans/mission-completion-protocol.md per plans/backlog-notes.md item 9.

## Queued goal: kill-python-fixtures — Finish the two-languages end state: no python3 in fixtures, the suite, or preflight
- Origin: main
- Next step: Sweep the ~150 python3 heredocs in scripts/ into engine verbs and shell; the doctrine is backlog-notes item 13's end state.

## Parked goal: runtime-install-execution — The adoption/installation execution rewrite: plan owner, resume records, engine transport, crash-consistent completion
- Origin: main
- Parked because: Two six-round budgets show this subdomain diverges under prose critique (structural 12/11/5/5/5/8); the approach choice is Wido's: implementation-first behind fixtures, a third budget, or descope
- Next step: PARKED for Wido's ruling: implementation-first behind fixtures, a third prose budget, or descope. Seeds: plans/ric-critique-r1..r6.

## Done goal: monitor-facility — Tracked long-running work with terminal-state watching as metasystem behavior
- Origin: main
- Concluded: Shipped: run records with custody proofs, the waiter contract, verdict integration, the locked sweep. The mandatory code critique found 11 defects (4 critical); all folded with tests. Both hosts green at 0d76eb1.

## Done goal: agnosticism-audit — The core never names an agent runtime; every runtime surface is adapter-declared
- Origin: main
- Concluded: Phase A shipped: registry, capability tables, fail-closed residual waivers, all consumers. The mandatory critique found 14 defects, all folded. Both hosts green 91ff675. Phase B split per D74.

## Done goal: runtime-integration-contracts — The contested runtime-integration contracts: adoption/registration/installation, fixture authorization, conf schema, enforcement transport
- Origin: main
- Concluded: B1 shipped: fixtureauth capability system, declared vectors and enforcement compare, registration rows as data, hook contract. The mandatory critique found 16 defects, all folded. Both hosts green fb6b7df. B2 parked (D76).

## Done goal: runtime-file-placement — Runtime code lives in its runtime's file within the adapter seam
- Origin: main
- Concluded: Every runtime symbol home (the original stray was D25 copy-drift, fixed by D61). The parser-based placement check pins the convention; its critique-driven rebuild caught two more strays. Both hosts green 2f42ec7.
