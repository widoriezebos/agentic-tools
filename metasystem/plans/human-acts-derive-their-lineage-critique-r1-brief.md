Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal human-acts-derive-their-lineage, tier 2, hazard DESIGN-BEARING, code critique of chain hal-build1b)
Date: 2026-09-06

# Review brief: human-acts-derive-their-lineage, round one

Round budget: two review rounds for the goal's tier-2 box; this is
round one. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one operator at an enrolled terminal and agent seats
sharing a ledger, no adversaries. In scope: a human act recorded under
a lineage that misattributes it (another seat's, another terminal's,
a stale generation); an agent call that slips through the human path
by passing --by; a derived lineage that collides with a seat lineage
or breaks the opid or a history line; a refusal that sends the person
to the wrong repair; a test or fixture leg that cannot fail. Out of
scope: the enrollment mechanism itself (internal/humanauthority is
read-only for this chain), hostile inputs.

Scope: the computed diff of implementer job hal-build1b (round one)
against its base. The brief it implements is
metasystem/plans/human-acts-derive-their-lineage-brief.md, landed at
82a175f8; it binds. Reviewed tree (conformance, review stage):
32d552665ef1c6b072ad2d27ccd195029194509e.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. syncReq in metasystem/cmd/metasystem/goalsync_mutations.go: the
   derivation runs only when both --lineage and the variable are empty
   AND --by is set; an agent call keeps the old sentence byte for
   byte; the new refusal names enroll-terminal; the sanitised terminal
   id keeps enough identity (a colon becomes a hyphen; two different
   terminals must not collapse into one lineage).
2. Attribution: is `--by` alone enough to take the human path, or can
   an agent shell pass --by Wido and receive a terminal lineage
   without the terminal proof? Say what the human-only verbs check
   afterwards (humanauthority.ProveOrTemporaryGoalAuthority, called
   after syncReq in the same verbs) and whether the derived lineage
   ever reaches the ledger without that proof.
3. The enrollment read: what ReadEnrollment returns on a stale or
   half-written enrollment, and whether the generation in the lineage
   is the enrollment generation the proof later checks.
4. Tests: TestSyncReqLineage in metasystem/cmd/metasystem/goalsync_mutations_test.go
   writes an enrollment through humanauthority.Enroll; check the three
   cases cannot pass vacuously and that the test does not depend on
   the seat's real terminal. The fixture leg in
   metasystem/scripts/agents/goal-cli-fixtures.sh writes the on-disk
   enrollment shape by hand because the bed is headless; check it
   writes the same shape ReadEnrollment reads, that it unsets
   METASYSTEM_OWNER_LINEAGE for the act, and that its assertion on
   the recorded opid or lineage is real.
5. Conformance: only the three May-touch files; nothing under plans;
   internal/humanauthority and internal/goal untouched.

# Evidence you may run

From the reviewed worktree root (the metasystem directory):
`go test -count=1 -run 'Lineage|SyncReq' ./cmd/metasystem`,
`go vet ./cmd/metasystem`, `gofmt -l ./cmd/metasystem`,
`bash -n scripts/agents/goal-cli-fixtures.sh`. The full goal-cli
fixture bed is the orchestrator's to run. Do not edit anything.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
