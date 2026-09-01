Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal native-spend-cap-retirement)
Date: 2026-09-01

# Goal

Build the certified spend-cap retirement design
(plans/spend-cap-retirement-design.md revision 5, landed, critique-clean —
its Option-B build specification section is your exact contract, and it
leaves no judgment calls).

# Workspace

The delegate worktree. Touch exactly what the design's build specification
names: internal/adapter/claude.go (the ClaudeBudget function and the one
conditional append in BuildClaudeCommand) and its tests
(internal/adapter/claudecommand_test.go and any test the design names).
Nothing else.

# The change, per the design

1. ClaudeBudget: no default budget — when METASYSTEM_CLAUDE_MAX_BUDGET_USD
   is unset, no --max-budget-usd flag is emitted at all; a set value is
   validated exactly as today, malformed values still return the
   invalid_native_budget protocol error. The doc comment is the design's
   specified system-language text, verbatim.
2. BuildClaudeCommand: the budget-flag append becomes conditional on a
   budget being present.
3. Tests: update per the design — the default assertion, the argv byte-order
   pins for both the flag-present and flag-absent shapes, and the env
   override cases.

# Verification you can perform

- go test ./internal/adapter/ — level ran, observed PASS.
- go vet ./internal/adapter/ — level ran, observed clean.

# Constraints

Wall-clock budget: 15 minutes. The design's reject condition binds: if this
diff must touch anything beyond internal/adapter/claude.go and its tests,
STOP and report the gap.

# Expected Return

Version-2 implementer JSON; diffBoundary listing exactly the touched files.

# Gap Rule

stop and report a gap; never fill it silently.
