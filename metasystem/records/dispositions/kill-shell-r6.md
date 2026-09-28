# Dispositions: kill-shell plan, round 6

Job: design-critic-20260812t000010z-7c4b (codex gpt-5.6-sol, xhigh).
7 findings, 7 material, all accepted.

| id | disposition |
| --- | --- |
| KS-R6-001 | accepted — go-packages registry validation is TEMPLATE-ONLY, gated exactly like the go gate itself (a checkout without go.mod skips it); adopted targets validate the scripts section alone. |
| KS-R6-002 | accepted — the registry becomes a CLOSED WORLD: every tracked shell file in the payload globs must carry an entry, and an unregistered tracked script fails the fence outright. Registration is ownership, and the fence enforces registration. |
| KS-R6-003 | accepted — tombstones get their own registry section: scripts holds live files only, deletion MOVES the entry to tombstones, and the definition of done quantifies over the scripts section. No forever-failing absent file, no silent shape exemption. |
| KS-R6-004 | accepted — the scripts schema gains shape (exec, custody, sequencer) and verified (date); done = every live entry verified with a shape and zero debt. Planned versus completed is now representable. |
| KS-R6-005 | accepted — recorded supersession: plans/go-production-grade.md Phase 0c's 'add the check to go-gate.sh' is superseded on ownership by this plan's audit coverage-ratchet verb; the note rides here because that plan is under the human's own live critique, and the conflict is flagged for him rather than edited into his file. |
| KS-R6-006 | accepted — the relay definition restores the round-5 disposition in full: defaults come from verbs, usage text comes from verbs; only flag-to-argv mapping stays in shell. |
| KS-R6-007 | accepted — go-gate's own policy (the no-module skip, foreign-gate refusal wording, check ordering, failure ramps) joins Phase E as an `audit gate` verb; the bootstrap keeps only compile-and-consult. 'Near-minimal' was a grade, not an exemption. |
