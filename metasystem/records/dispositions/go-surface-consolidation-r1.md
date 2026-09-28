# Go surface consolidation — round 1 dispositions

Critic: design-critic-20260812t063648z-0c07 (codex, gpt-5.6-sol).
7 findings, 7 material. All folded into records/misc/go-surface-consolidation.md.

| Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GSC-R1-001 | accepted | The draft claimed surface removal while mostly regrouping; reachability alone keeps the decomposition. | New section defines script-shaped operationally (bash-owned call sequences whose ordering carries an invariant); step 0 gains a sequence census producing the real coarsening list; regrouping is claimed as coherence work, not de-shell-ification. |
| GSC-R1-002 | accepted | record-create must precede shell-owned setup (dispatch.sh 984-1055; record.go 114-158); the cleanup trap needs observable pending-setup. A merged reserve either races or swallows plumbing. | `job reserve` withdrawn; reservation stays two-phase; coarsening only where no observable intermediate state another process relies on disappears. |
| GSC-R1-003 | accepted | adapter/host name pairs are behaviorally different operations with live callers on both sides; mission has colliding init/verify; gate check vs hooks check collide. | adapter+host merge withdrawn (two families stay); mission, proc, and util merges each require an exhaustive collision-resolving verb map as a step deliverable. |
| GSC-R1-004 | accepted | internal/supervise reaper (reaper.go 61-132) is a second verdict owner mutating records; naming only dispatch.sh as consumer leaves the ladder split. | One decision owner, two consumers wired in the same commit: the supervise reaper calls the function, dispatch.sh consults the verb. |
| GSC-R1-005 | accepted | "Organically" plus unconditional table deletion has no satisfiable completion condition. | Step 5: bounded in-tree old-name sweep to zero by the census rules, then delete the table; adopted repositories name scripts, never verbs. |
| GSC-R1-006 | accepted | Nothing executable reads shell-dispositions.json; adopt.sh's allowlist is the real exporter. A ship-list would be decorative; wiring it in would be new machinery. | Registry is deleted outright; adopt.sh's allowlist stays the single export contract. |
| GSC-R1-007 | accepted | Raw grep self-matches registrations; dynamic dispatch and wrapper functions hide callers; no classification rules were stated. | The executed census rules are now normative in the doc (corpus exclusion, literal-pair rule, variable-verb guard, loose-grep + implementation-trace + docs manual verification, tests never keep verbs alive); first slice recorded as c72f662. |
