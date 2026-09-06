Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal human-acts-derive-their-lineage, tier 2, hazard DESIGN-BEARING, code critique of chain hal-build1b round two)
Date: 2026-09-06

# Review brief: human-acts-derive-their-lineage, round two

Round budget: two review rounds for the goal's tier-2 box; this is
round two, the last. The orchestrator adjudicates every finding; you
edit nothing.

Threat model and scope as in round one
(metasystem/plans/human-acts-derive-their-lineage-critique-r1-brief.md).
Round one (hal-critic1) found two material defects: enroll-terminal
refusing by naming itself, and unproven verbs deriving a terminal
lineage from any shell that passes a name. Both were folded in round
two from metasystem/plans/human-acts-derive-their-lineage-fold-r2-brief.md:
the derivation now requires the terminal proof of the invoking
process, and enroll-terminal derives from the enrollment it publishes.
You review the WHOLE chain diff (rounds one and two together, the
computed diff of implementer job hal-build1b-r2 against the chain's
base). Reviewed tree (conformance, review stage, round two): af63ca0038eb8f11c776ddcd160818951b1b9ed7.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The proof gate in syncReq (metasystem/cmd/metasystem/goalsync_mutations.go):
   an agent shell passing --by in an enrolled checkout with no lineage
   is refused (which proof call, which outcome values pass, whether a
   temporary-word or fixture path can substitute for a real terminal
   and under what flag); the lineage's id and generation come from the
   enrollment the proof checked, not a re-read that could differ.
2. enroll-terminal: on an unenrolled checkout without a lineage it
   enrolls and records the new enrollment's lineage; on an enrolled
   checkout it re-enrolls the next generation and records that one;
   the enrollment error path is unchanged.
3. The six verbs that already prove: no double proof changes their
   outcome or their recorded proof file; discharge-review-obligation
   now passes its name.
4. Tests in metasystem/cmd/metasystem/goalsync_mutations_test.go: the
   refused-shell case and the derive case cannot pass vacuously; say
   how a passing proof is obtained in a test process and whether that
   route is reachable by an agent in production. The fixture leg in
   metasystem/scripts/agents/goal-cli-fixtures.sh: how it reaches a
   passing proof headless, and that its opid-suffix check is a real
   assertion.
5. Conformance: only the May-touch files; nothing under plans;
   internal/humanauthority and internal/goal untouched.

# Evidence you may run

From the reviewed worktree root (the metasystem directory):
`go test -count=1 -run 'Lineage|SyncReq|Enroll' ./cmd/metasystem`,
`go vet ./cmd/metasystem`, `gofmt -l ./cmd/metasystem`,
`bash -n scripts/agents/goal-cli-fixtures.sh`. Do not edit anything.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
