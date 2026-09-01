Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal native-spend-cap-retirement)
Date: 2026-09-01

# Goal

Two-layer implementation critique of the spend-cap retirement build (job
implementer-d1947930c9b516cb64dffdb8, reviewedTree 19be6f3c, diff.patch in
its round evidence): first conformance against the certified design
(plans/spend-cap-retirement-design.md revision 5, its Option-B build
specification is the contract), then adversarial defect review of the diff.

# Attack surface

- The moved validation: a SET but malformed override must still return
  invalid_native_budget; an UNSET budget must skip validation entirely and
  emit no flag. Check every caller of ClaudeBudget and BuildClaudeCommand
  for an assumption that budget is non-empty.
- The argv order pins: the flag-absent shape must match what the tests now
  assert, byte for byte, and the flag-present shape must be unchanged from
  the shipped order.
- The comment: system language only, per the repository comment law.
- Anything the diff touches beyond the design's reject condition
  (internal/adapter/claude.go and its tests) is a finding.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
