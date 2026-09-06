Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal fast-gate-runs-the-refusal-register, tier 3, hazard DESIGN-BEARING, code critique of chain fgr-build1 round two)
Date: 2026-09-06

# Review brief: fast-gate-runs-the-refusal-register, round two

Round budget: three focused rounds for the goal's tier-3 box; this is
round two, after the fold of round one's two wording notes. The
orchestrator adjudicates every finding; you edit nothing.

Threat model and scope are those of round one
(metasystem/plans/fast-gate-runs-the-refusal-register-critique-r1-brief.md),
whose dispositions are recorded in
metasystem/records/misc/fast-gate-runs-the-refusal-register-critique-r1-dispositions.md.
The fold brief is
metasystem/plans/fast-gate-runs-the-refusal-register-fold-r2-brief.md.
The computed diff of the final work round is
metasystem/artifacts/agents/fgr-build1/rounds/2/diff.patch and its
reviewed tree is 91210fa7ca135468b4c3c2017c594650781ad09f; carry that
hash into your return exactly.

# Goal

Say whether the final change ships a defect, violates its brief, or
damages what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The fold changed exactly two lines of prose: the commit wrapper's
   static re-proof comment names the refusal register; the exclusion
   reason for the token `ledger-unreadable` calls it a digest sentinel
   the steward compares, not a refusal code. Nothing else moved since
   round one.
2. Round one's substance still holds: the fast-mode stage in
   metasystem/scripts/agents/go-gate.sh fails the gate when the refusal
   package fails, the full gate still covers the package once, the
   exclusion matches the one token exactly, the dead-exclusion check
   holds, and the whole is bash 3.2 clean.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory): `go test -count=1 ./internal/refusal`,
`bash -n ./scripts/agents/go-gate.sh`, `bash -n ./scripts/agents/commit.sh`.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
