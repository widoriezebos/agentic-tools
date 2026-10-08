# Task: revise the design machinery-measures-its-own-process after critique round 1

Working Mode: Design
Revise plans/designs/machinery-measures-its-own-process.md in place (keep Status: draft). Round 1 (Opus) found 9 material findings; fold each with a concrete change and add a "Round 1 findings and changes" table. Size cap binds: each unit at most 250 production lines, at most 5 units; recut or move to the follow-up brief (plans/process-changes-cover-declarations-and-interventions-design-brief.md) with a stated reason where needed.

1. The unit's check is a recorded process input now, not deferred: `work build --check` (cmd/metasystem/intent_work.go:806, kept as the plan's Proof :901) chosen per unit by the agent; a check whose argv differs from the goal's previous unit or from the resolved proof.cheap/proof.full argv becomes a ProcessAct (added or widened by argv comparison) that U3 attributes and U4 counts. proof.full/proof.cheap are committed-only (internal/config/defaults.go:79, :81) and landing.*/review.stop are already direct-person-only (intent_work.go:2439): trim U2's settings scope accordingly. Deferring the scaffold check block and nested-check timing stays legitimate.
2. A helm or attorney grant act counts as the agent's: ProcessAct records Proof.Helm (internal/humanauthority/authority.go:916, :937); for U4 and U5 a grant act is the agent's own; under process.change=person it is held unless it carries direct-person proof. This is the 10-07 case: the seat agent acted under a general power of attorney.
3. U3: when a consumed matching process act exists, the cause is process-change with any external evidence attached, and the revert is printed first; never unclassified because load is also present.
4. The suite-minutes band's input: a unit check counts as the full suite when its argv equals the resolved committed proof.full (ProofCommand has Name/Dir/Argv/Env only, internal/launch/unit_plan.go:44); no command-text guesses.
5. Name the home for goal-episode process and U4 stops (beside ProcessAct) and its one reader, within U3's lines; per-round stops stay in the round directory (internal/launch/unit_stop.go:209-259); no generic register (review-chain-stops-and-records.md:33).
6. U2 must be buildable first and within 250: move the question adapter into U2; move undo/`settings unset` into U3 (the unit that prints the revert); recount.
7. Key U4's window by target checkout and actor lineage, not by the optional --goal; an agent proposal naming no goal is held.
8. U1 adds the measures appended to the brief at 23:30 (merge conflicts at integration, suite minutes against the unloaded baseline, flake reruns per gate, builds and suites refused or queued for load, fix units per unit), or routes each to the follow-up with a reason; the plan's load-aware lever names this goal as its measure (plan :417).
9. Freeze the estimate denominator at the unit's first admission in the goal; only a person's recorded `design` correction changes it later.

Decided by m1e for Wido (record under "Decided by m1e for Wido", 2026-10-08, reversible): the bands default to twice the frozen estimate and the goal's own baseline, citing Wido's 10-03 "estimates and tracking" and 10-07 "why are we progressing this slow"; process.change defaults to person (NO HAL 9000 read both ways: the machine never rewrites its own process without a person).

Do not touch code, memory/, records/ or the ledger. Never open any metasystem.conf.local. Return: units with production-line estimates and what changed per finding.
