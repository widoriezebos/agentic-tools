# Dispositions: kill-shell plan, round 5

Job: design-critic-20260811t235059z-2b25 (codex gpt-5.6-sol, xhigh).
7 findings, 7 material, all accepted. Convergence: 9 → 5 → 11 → 10 → 7,
with severity narrowing to specification completeness.

| id | disposition |
| --- | --- |
| KS-R5-001 | accepted — ownership is the REGISTRY, not a glob: the budget's jurisdiction is exactly the scripts registered in shell-dispositions.json; metasystem-owned means registered, and an adopted project's own files are never registered. One implementable rule. |
| KS-R5-002 | accepted — the plan gains a Definition of Done: the program closes only when the registry carries zero debt entries and every registered script holds a verified thin-shim (or sequencer/custody) verdict. Phases finishing is not the program finishing. |
| KS-R5-003 | accepted — "argument relay" is defined: parsing that only maps CLI ergonomics onto verb argv (flags to flags, defaults from verbs, usage text from verbs) is relay; any flag whose value selects POLICY is a decision and moves. dispatch.sh's end state satisfies the definition rather than contradicting it. |
| KS-R5-004 | accepted — the coverage ratchet check is a Go verb in the audit family (audit coverage-ratchet), consulted by one go-gate shim line; the first new gate decision gets a Go owner like everything else. |
| KS-R5-005 | accepted — deletions leave tombstones: a port+delete entry stays in the registry forever, and the fence cross-checks tombstones against docs/migrations.md entries — two durable records that must agree, so a vanished script cannot vanish quietly. |
| KS-R5-006 | accepted — one registry file, two sections with schemas: scripts (path, verdict, debt deadline) and go-packages (import path, governing plan file); the audit verb validates both — the named plan file must exist, the package must exist, and an unreachable package without an entry fails. |
| KS-R5-007 | accepted — the construct enumeration returns to the fence text: per-file counts of if, while, case, and for, ratcheted like every other number. |
