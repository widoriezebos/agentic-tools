Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Revision 7 of metasystem/plans/codex-handshake-design.md (revision 6 landed,
in your worktree). Sol's round-4 register is
metasystem/records/misc/codex-handshake-critique-r4.md: ONE material finding,
CHS-R4-FOLD-SCOPE-01 (high), decided at its foot. Fold it and stop. Edit in
place; diffBoundary is that one file. Keep it under eight minutes; read only
the lines named.

# The fold

In D2.5's shell paragraph (the sentence beginning "claude.sh:134 and :138
write `runtime_error handshake` before the fork"), replace the accepted
consequence with the decision: both callers become
`fail_pending launch_failed handshake` (verify the two lines in
metasystem/scripts/agents/adapters/claude.sh, lines 129-138, and the
dispatcher's own use of the class at metasystem/scripts/agents/dispatch.sh
line 1618 and metasystem/internal/missionrunner/patience.go line 47); the
pair is unlisted and never folds for any role, so a Claude critic that never
launched keeps failing as a launch, the follow-up gate refuses it as today,
and no synthetic finding is minted. Add the rule of record to the table's
rationale: a pair folds only when a runtime actually produced it; pre-launch
failures carry launch classes. Section 4: add claude.sh to the file table
(two-line class change). Section 5: one harness row in
metasystem/scripts/agents/adapter-deadline-fixtures.sh (ADPT-DL-009) under a
`{"role":"design-critic"}` record calling `fail_pending launch_failed
handshake <usage>` and asserting `error == launch_failed`, `phase ==
handshake`, no `handshakeExitStatus`. Mark the closure with the id. Section
7 stays "None" plus its facts.

Bump the header to revision 7 naming the round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 8 minutes. Design only; edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
