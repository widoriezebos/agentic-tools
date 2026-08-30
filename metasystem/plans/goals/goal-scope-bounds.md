# goal-scope-bounds

- State: queued
- Intent: Goals get mechanical size bounds AND a split mechanism (Wido 2026-08-30): a big goal is welcome at intake, but before slicing it evolves into an arc of small goals - the scope norm triggers, the split verb is the remedy; concurrency is designed at two axes, scope (this goal: planning parallelism via arcs) and load (machine-concurrency-governor: execution parallelism via slots)
- Origin: human
- Next step: Appetite: 3 slices of 4h. Slice 1 (design, Fable lane): (a) the scope norm - a configured ceiling (metasystem.budget.goal-norm-job-minutes, proposal 1440m = six slices) checked at set-budget and claim; over the norm refuses with the typed remedy: split; (b) the SPLIT mechanism - 'goal split --id parent' takes drafted member definitions and atomically opens the members bound to an arc named for the parent, records dependency edges between members, and concludes the parent as decomposed-with-pointer; members INHERIT the parent's origin and authority envelope (a split never mints new authority and never expands scope - the members' combined intent stays within the parent's), so a human-origin parent splits without a fresh human word; the split DRAFT is design work in the Fable lane triggered when the norm refuses, ratified by the parent's origin tier (human-origin: Wido reads the split; main-origin: the coordinator ratifies). Slice 2 (implement, Sol lane): the norm refusal plus the split verb in the goal machinery with fixture proof - split atomicity, arc binding, dependency edges, origin inheritance, parent conclusion. Slice 3 (implement, Sol lane): claim-side integration - members claimable independently by any machine within their recorded dependencies (a member with an unmet dependency refuses claim with the typed dependency named), making arcs the standard parallel-execution shape. R-2's open-gate stays as the human front door for genuinely new big authority; this adds the mechanical floor and the evolution path behind it. Disjoint from the steward seam; claimable by either machine
- OpenedAt: 2026-08-30T16:08:07Z
- Revision: 2

History:
- 2026-08-30T16:08:07Z ZBBC230KPCRGNH7WXR6CE9FNXM-m1-bf243850 open actor=m1+coordinator targets=goal-scope-bounds
- 2026-08-30T16:09:26Z B8VEFZ7EP7TPFG2V8MCTWJWJ0E-m1-bf243850 edit actor=m1+coordinator targets=goal-scope-bounds
Integrity: sha256=546819cb99cb26917837c3c11db7dba7e06df0eff20448be07e07e4b6500b9db
