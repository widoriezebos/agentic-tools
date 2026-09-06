Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal rearm-remedies-and-adoption-notes, tier 2, hazard DESIGN-BEARING, code critique of chain rra-build1 round two)
Date: 2026-09-06

# Review brief: rearm-remedies-and-adoption-notes, round two

Round budget: two review rounds for the goal's tier-2 box; this is
round two, the last. The orchestrator adjudicates every finding; you
edit nothing.

Threat model and scope as in round one
(metasystem/plans/rearm-remedies-and-adoption-notes-critique-r1-brief.md).
Round one (rra-critic1) returned two material findings, both folded in
round two from metasystem/plans/rearm-remedies-and-adoption-notes-fold-r2-brief.md;
the dispositions are in
metasystem/records/misc/rearm-remedies-and-adoption-notes-critique-r1-dispositions.md.
You review the WHOLE chain diff (rounds one and two together, the
computed diff of implementer job rra-build1-r2 against the chain's
base), not only the fold. Reviewed tree (conformance, review stage, round two): eeb990d9d7a17401495551d1a1004dbee3be8ea9.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The fold of F-1: the arm-lock remedy in metasystem/internal/up/up.go
   names the lock file and repairs that can apply (permissions, space,
   the open-file limit); no sentence in up.go or its test names a lock
   holder; the two test cases assert the new sentence in full.
2. The fold of F-2: in metasystem/scripts/adopt.sh the preset-key read
   now precedes the detached test; a preset key prints the kept note
   only, on a branch or detached; a detached target without a key
   prints the detached note; a branch without a key keeps the old
   upstream logic byte for byte; `set -euo pipefail` survives every
   branch (a failing `git config --get` inside an `if` is fine, one
   outside is not). The third leg in metasystem/scripts/adopt-fixtures.sh
   pins detached-plus-preset and cannot pass vacuously.
3. The fold of F-5: the fallback case pins the unchanged sentence.
4. Round one's untouched parts still hold: the human-arm replace path
   in metasystem/internal/steward/runner.go and its test in
   metasystem/internal/steward/rearm_test.go, the not-landed remedy.
5. Conformance: only the six files of round one; nothing under plans;
   runner.go, rearm_test.go, the resolver and the steward verbs
   unchanged by round two.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory): `go test -count=1 ./internal/up`,
`go vet ./internal/up`, `gofmt -l ./internal/up`,
`bash -n scripts/adopt.sh scripts/adopt-fixtures.sh`,
`grep -n 'holding the steward arm lock' internal/up/up.go internal/up/up_test.go`
(expected: nothing). The adopt fixture bed is the orchestrator's to run.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
