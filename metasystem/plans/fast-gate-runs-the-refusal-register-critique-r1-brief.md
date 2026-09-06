Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal fast-gate-runs-the-refusal-register, tier 3, hazard DESIGN-BEARING, code critique of chain fgr-build1)
Date: 2026-09-06

# Review brief: fast-gate-runs-the-refusal-register, round one

Round budget: three focused rounds for the goal's tier-3 box; this is
round one. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one seat landing through the fast gate on one machine, no
adversaries. In scope: a fast gate that still lets an unrowed refusal
token through, a stage that runs in the full gate twice, a stage whose
failure does not fail the gate, an exclusion that hides a real refusal,
a changed fast-mode contract (no environment switch, no witness), and
any bash-4 construct. Out of scope: hostile inputs and the full gate's
other stages.

Scope: the computed diff of implementer job fgr-build1 (round one)
against its base. The brief it implements is
metasystem/plans/fast-gate-runs-the-refusal-register-brief.md, landed
at e66ba132; it binds. The computed diff is
metasystem/artifacts/agents/fgr-build1/rounds/1/diff.patch and its
reviewed tree is 6c8811fbf09889126777756ecdd79485b30ccacb; carry that
hash into your return exactly.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The new stage in metasystem/scripts/agents/go-gate.sh: it runs only
   in fast mode, before the build, and a failure lands in the same red
   block the other static stages use so the gate exits nonzero; the
   passed line and the header name it; the full gate's race run still
   covers the package once.
2. The exclusion in metasystem/internal/refusal/register.go: the
   token `ledger-unreadable` is a digest sentinel (its definition at
   turnverdict.go line 291 and its two comparisons in the steward's
   alert episode), not a refusal; the exclusion's pattern matches it
   exactly and nothing else; the dead-exclusion check still holds.
3. Conformance: only the two files changed; nothing under plans.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory): `go test -count=1 ./internal/refusal`,
`bash -n ./scripts/agents/go-gate.sh`, `bash --version`. The gate itself
is run seat-side by the orchestrator.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
