Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal native-spend-cap-retirement)
Date: 2026-09-01

# Goal

Closing critique of plans/spend-cap-retirement-design.md revision 4 (landed,
in your worktree): the clean-kill Option B promoted to specification by
Wido's recorded ruling R-43-m0b. Judge: (1) the promotion's completeness —
no backstop residue left in normative text, the dissolved finding
dispositioned honestly; (2) the build specification's exactness — the
ClaudeBudget no-default change, the BuildClaudeCommand conditional append,
every named test change including the argv byte-order pins when the flag is
absent, the mission-host export unchanged; (3) one flagged reading to rule
on: the design's reject condition says "the ClaudeBudget function, its
tests, and the doc comment" while the specification touches two functions in
internal/adapter/claude.go — the orchestrator's stated reading is that the
condition means the adapter budget policy code in that one file plus its
tests; judge whether that reading is sound or the condition must be reworded
by a further fold. A clean return (the reading ruled sound counts as clean)
closes the design phase; the build dispatches against this landing.

# Constraints

Wall-clock budget: 20 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
