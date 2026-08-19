# Goals

## Current goal: host-implementer-wall — The runner mechanically refuses a host turn that did implementer work: product-code diff without certified delegate attribution is a protocol error, not prose
- Origin: main
- Next step: Land slice 7 on AGREE (capped loop). Then PARK yielding to backlog-git-sync (D115); O13, O14, O15, O16 and rows O8/O10/O9/O11 resume as claimable work under the parallel backlog.

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

## Queued goal: custody-death-proof — Every terminal record write rides a real death proof: reaper, drain, lease sweep, protocol-error path, and fleet identity resolution
- Origin: main
- Next step: Design loop over the four shipped holes named in plans/acp-critique-r5.md finding 10 and D79; sealed-v1 (acp-transport) is the working prototype of the target contract.

## Queued goal: runtime-install-execution — The adoption/installation execution rewrite: plan owner, resume records, engine transport, crash-consistent completion
- Origin: main
- Next step: PARKED for Wido's ruling: implementation-first behind fixtures, a third prose budget, or descope. Seeds: plans/ric-critique-r1..r6.

## Queued goal: provision-genesis-authority — A virgin adopted target's goal-baseline genesis works again: authority admits genesis where no lease exists yet
- Origin: main
- Next step: Reproduce via benchmark/validate-kit.sh (BM-1 provisioning fails at adopt.sh:285); decide the authority rule for rootless genesis; gates the D82/D83 acceptance benchmarks.

## Queued goal: genesis-authority-design — Genesis authorization that cannot be laundered: classification root not caller-controlled, or a capability minted by an authenticated holder
- Origin: main
- Next step: Design over the 3 holes in plans/genesis-authority-review.md; classification-against-caller-root is unsound — the fix is architectural.

## Queued goal: small-change-lane — The 'change this little thing' case has a supported path: a dispatch lane cheap and fast enough that a certified one-line fix is not ceremony
- Origin: main
- Next step: Design when picked up: a micro-dispatch shape (single delegate round, tight brief, fast certification) that keeps every product byte delegate-certified under the host-implementer wall's no-exception rule (D100).

## Queued goal: source-comment-standard — Every source comment speaks the application's own language in plain English; the landed codebase is rewritten to that standard
- Origin: main
- Next step: Design per Wido's 2026-08-19 ruling: inventory landed offenders (round/slice references, history narration), rewrite in critiqued batches.

## Queued goal: flake-registry — Known flaky fixture legs are repo data with sighting counts and a bounded rerun protocol, not one agent's memory
- Origin: main
- Next step: Design the registry file and the one-solo-rerun rule; seed it with the fence job-cap-min ask leg (two sightings). Critique, then implement.

## Queued goal: executable-covenant — The verification covenant is runnable: one battery entrypoint with a verdict file, one critique-round driver any agent can invoke
- Origin: main
- Next step: Design: battery.sh wrapping go-gate + both fixture families with a codes file; critique-round.sh building, launching, archiving rounds. Critique, then implement.

## Queued goal: landing-tooling-fixes — Landing tools do what agents remember around: commit.sh survives multi-path pathspecs; landing pushes origin AND transport
- Origin: main
- Next step: Reproduce the pathspec mangling as a fixture, fix commit.sh; add the dual-remote push to the landing path. Critique, then implement.

## Queued goal: agent-ease-assessment — A hard look at the metasystem's complexity from the agent seat: it must be intuitive to use; accepted simplifications get executed
- Origin: main
- Next step: Wido's ruling 2026-08-19: assess ease of use across surfaces (CLI, scripts, skills, docs), name complexity hotspots and simplification opportunities, take the assessment through critique, then execute what is accepted.

## Queued goal: invariant-consolidation — Stacked defenses on hot artifacts consolidate behind one owner per invariant; first case: the contract's authenticated flow from origin to E0
- Origin: main
- Next step: Design per D114: inventory the contract's guard layers, name the single invariant and owner, fold redundant belts into it with their witnesses. Critique, then implement.

## Queued goal: goal-ledger-ergonomics — The goal ledger stops fighting lawful use: byte caps sized to purpose, and a verb to edit a queued goal in place
- Origin: main
- Next step: Design per D114: justify or retune the intent/next byte caps; add the missing edit verb under the same integrity chain. Critique, then implement.

## Queued goal: backlog-git-sync — Multiple machines work the backlog in parallel with git as the sync mechanism: claimed goals, merge-safe ledger mutations, surviving baselines
- Origin: main
- Next step: D115, picked next after the wall: design the claim verb, a merge-safe ledger format (the whole-file digest cannot merge), and baseline reconciliation across pushes. Critique, then implement.

## Queued goal: lease-acquire-atomicity — Lease acquisition and stale-lease cleanup are mutually exclusive: no second launcher can misclassify a mid-publication lease and mint two runners (KI-38)
- Origin: main
- Next step: Small change after the wall lands: one flock over classification-and-removal and marker-and-record publication, plus a two-process witness. Critique, then implement.

## Parked goal: disk-hygiene — Every byte the metasystem writes gets a declared lifecycle with janitor enforcement
- Origin: main
- Parked because: Yields the current slot to the human-approved acp-transport implementation (D81); design held at the budget-one boundary, r3 verdict will be folded when it lands
- Next step: Design per plans/backlog-notes.md item 19; headroom checks in suite and provision paths.

## Parked goal: process-steward — A standing role that watches the development PROCESS itself against expectations and signals deviations to the orchestrator, or acts
- Origin: main
- Parked because: Parked per D87/D94: no owner emits a typed process verdict yet; the near-checkable invariant mostly duplicates the watchdog. Resume DESIGN when any sound typed owner verdict exists; justify aggregator over direct delivery.
- Next step: Design per plans/backlog-notes.md item 21 when picked up: inventory the process invariants, decide signal-vs-act per class.

## Parked goal: acp-transport — ACP as the delegate transport, retiring the dangerous-mode waiver
- Origin: main
- Parked because: Yields the slot to host-implementer-wall per Wido's ruling (D99): the enforcement failure outranks the flip. Remaining here: snapshot identity surfaces + supervise_acp fixture, then the D82 pair on Wido's seals.
- Next step: READY, blocked on the human-sealed benchmark (D88): delegate path proven, provisioning unblocked, VM+Devin ready. Run bm-2d (run-cohort.sh, VM-only, stops for seal). Do not flip without it (D82).

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
