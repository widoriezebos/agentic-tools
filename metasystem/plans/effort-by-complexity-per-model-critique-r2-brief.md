Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal effort-by-complexity-per-model, tier 2, hazard DESIGN-BEARING, code critique of chain ebc-build1 round three)
Date: 2026-09-07

# Review brief: effort-by-complexity-per-model, round two (the last)

Round budget: two review rounds for the goal's tier-2 box; this is
round two, the last. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one fleet whose dispatcher decides how hard a delegate
thinks, no adversaries. In scope: a configured effort that silently
weakens below the admission floor for a design-bearing or destructive
chain; a value the runtime does not understand reaching the CLI; a
record that says one effort while the adapter received another; a
follow-up round that silently drops to the floor; an unconfigured
repository that changes behaviour; a fixture or test that cannot
fail. Out of scope: the codex adapter's own flag, hostile configuration.

Scope: the computed diff of implementer job ebc-build1-r3 (the whole chain, three build rounds)
against its base. The brief it implements is
metasystem/plans/effort-by-complexity-per-model-brief.md; it binds.
Reviewed tree (conformance, review stage, round three, the whole chain diff of implementer job ebc-build1-r3): 6d5883ccc7ab7ae8e1418d7d28f2ed1b19212de6.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

0. The round-three fold: a follow-up on a pre-change parent record
   (no reasoningEffortSource) composes with the parent's effort and
   source record:pre-effort-keys, pinned by a test; the source field
   is immutable; the claude adapter clears a literal null. Round one's
   dispositions are in records/misc/effort-by-complexity-per-model-critique-r1-dispositions.md
   on main (F-1 to F-3 folded, F-4 to F-7 noted).

1. Resolution order in metasystem/internal/dispatch (mode, role,
   default) mirrors the model keys exactly; the class spelling in keys
   is lower case with the hyphen; no key set yields the floor and the
   source says so.
2. Vocabularies and floors in metasystem/internal/dispatch/hazard.go:
   codex minimal..xhigh, claude low..max, devin and fake only the
   empty value; the floor words medium/high/high; comparison is by
   position in the runtime's own list, never by string; an under-floor
   value refuses naming key, value and floor; the role packet table
   and ResolveHazardConfiguration agree with the floors; what became of
   the maximal-models proof and whether dropping or narrowing it opens
   a path for a runtime with no strongest-word proof.
3. The record and the argv: metasystem/internal/dispatch/build.go
   writes reasoningEffort and reasoningEffortSource from the resolved
   value; the caller-supplied --reasoning-effort must equal it; the
   follow-up round carries the parent's value; the claude argv carries
   `--effort <value>` (metasystem/internal/adapter/claude.go, the
   verb in metasystem/cmd/metasystem/adapter_runtime_verbs.go, the
   adapter metasystem/scripts/agents/adapters/claude.sh) and the codex
   argv is unchanged.
4. The shipped metasystem/metasystem.conf lines: Fable at high for
   design-bearing and destructive-reach, codex at xhigh, both at medium
   for mechanical; a comment naming vocabularies and floors.
5. Tests and fixtures: Go tests for floors, vocabularies, the claude
   flag; the dispatch fixture legs for shadowing, bad value,
   under-floor, no-switch runtime, recorded-equals-received; the old
   xhigh assertion on the fake implementer record updated to what the
   fixture configures; none vacuous.
6. Conformance: only the brief's May-touch files; nothing under plans.

# Evidence you may run

From the reviewed worktree root (the metasystem directory):
`go test -count=1 ./internal/dispatch ./internal/adapter ./cmd/metasystem`,
`go vet` and `gofmt -l` on the same, `bash -n` on the three scripts.
The dispatch fixture bed is the orchestrator's. Do not edit anything.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
