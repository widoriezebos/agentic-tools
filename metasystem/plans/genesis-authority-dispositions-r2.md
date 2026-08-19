# Genesis authority design — critique round 2 dispositions

Critic: design-critic, codex gpt-5.6-sol, job `genesis-authority-critique-2`
round 2 (`artifacts/agents/genesis-authority-critique-2/rounds/2/return.json`),
attacking the round-1 rewrite (seed/reconcile split). Verdict line: 6
material. Body read in full; all six dispositioned.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GA-R2-01 | accepted | The claim "reconcile-genesis never admits a caller the target cannot name" was an overclaim: `Classify` returns HUMAN when the walk cannot start or finds nothing (`classify.go:302-323`; `classify_test.go:47-55`), and HUMAN is admitted (`authority.go:29`). In a process-table-denied sandbox a delegate is therefore HUMAN-shaped for reconcile-genesis. The same reading admits it for `goal open` and every holder-only verb in the same sandbox, so this verb is not the weakest link and the design does not widen it. The mechanism fix — an indeterminate class for a truncated walk — changes every verb's behaviour in exactly the sandboxes the kit gate is required to run in, which is a classifier decision of its own, not a fold. | §2 R2 states the residual plainly and names the follow-up; §4 gains the row. No mechanism change. |
| GA-R2-02 | accepted | Correct: HEAD membership proves "the live branch carries a ledger", not "never had one"; the adopted pair is untracked until the first commit (`adopt.sh:247-255` copies, never commits). The guard's purpose is narrower than the draft said: rm-then-seed must not launder a rewrite of a TRACKED ledger into a merge; an untracked deleted pair held nothing git or seed could recover. | §2 R1 restates what the guard protects and what it does not claim; the obligation wording follows. |
| GA-R2-03 | accepted | Seed's own ledger-then-baseline crash (`goalverbs.go:191-204`) would have left a state only HUMAN/MAIN could repair, stranding the agent-ancestry flows after an ordinary interrupt. | R1 gains the "completed" arm: ledger present, no baseline, and the ledger is exactly a seed skeleton (goal-free, origin `adoption`, digest == ScanDigest(R)) → write the baseline. Only bytes that say what seed would say now are completed. Test per arm. |
| GA-R2-04 | accepted | "Skip seed if a ledger exists" conflated a healthy pair, a half-seed, and a pre-existing hand-written ledger, and dropped today's reconcile path for the last. | R1: baseline present → no-op; R3: adopt.sh runs seed unconditionally and falls back to `goal reconcile` when seed refuses a ledger-without-baseline that is not a skeleton — today's behaviour for that case, decided by the target's own classification. |
| GA-R2-05 | accepted | `refuseMissionSeat` guards every mutation (`goalverbs.go:236-242,560-564`); a seed during an active mission would create two wall-protected files (`missionrunner/wall.go:63-87`). | R1: seed runs the mission-seat refusal first, under the lock. Obligation GA-6. |
| GA-R2-06 | accepted | The guard had no outcome contract for non-repository, unborn HEAD, nested prefix, or a failing git. | R1 names each arm: not-a-repository → allow; unborn HEAD → allow; `ls-tree --name-only HEAD -- plans/goals.md` in R (prefix-relative by git's own rule) non-empty → refuse, empty → allow; any other failure → refuse with the error. Tests enumerate the arms including a nested-prefix fixture. The probe execs git directly from `internal/goal`; the wall program's tree package is not reused (different owner, and seed needs one question, not a tree projection). |

Round closed by join: 6 findings, 6 dispositions.

## Re-expression note (after the Mac session's scope addendum)

The round-2 amendments were first written onto a `goal seed` verb (the
round-1 rewrite). The Mac session then reserved adopt.sh's ledger seeding
and the ledger format for its parallel-backlog rewrite, so the design was
re-expressed on the authority admission path alone. Where each amendment
now lives: GA-R2-01 → §2 R6 residual (unchanged); GA-R2-02 and GA-R2-06 →
the HEAD guard inside `AdoptionShaped` (§2 R3), same arms and scope
statement; GA-R2-03 → moot (reconcile writes one file; adopt's skeleton
re-run reconciles on the same terms, §2 R5); GA-R2-04 → moot (adopt.sh's
existing-ledger handling is untouched); GA-R2-05 → already true for
reconcile (`refuseMissionSeat`, `goalverbs.go:560-564`). The seed verb's
doctrine point is carried to the report as a recommendation for the
seeding rewrite.
