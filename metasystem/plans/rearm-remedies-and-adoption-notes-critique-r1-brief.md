Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal rearm-remedies-and-adoption-notes, tier 2, hazard DESIGN-BEARING, code critique of chain rra-build1)
Date: 2026-09-06

# Review brief: rearm-remedies-and-adoption-notes, round one

Round budget: two review rounds for the goal's tier-2 box; this is
round one. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one operator adopting checkouts and re-arming stewards on
their own machines, no adversaries. In scope: an adoption that leaves a
target half adopted or silently changes a landing ref the operator
set; a remedy sentence that sends the operator to the wrong repair; a
human arm that mints a generation without the human at the terminal,
or that fails to mint one when the human is there with changed bytes;
a machine re-arm whose rule changed by accident; a runner stopped when
it should not be; a Go test or fixture leg weakened or vacuous. Out of
scope: the resolver's message texts (fixed by the brief), the steward
verbs file, hostile inputs.

Scope: the computed diff of implementer job rra-build1 (round one)
against its base. The brief it implements is
metasystem/plans/rearm-remedies-and-adoption-notes-brief.md, landed at
bced1868; it binds. Reviewed tree (conformance, review stage): 1f448200dee8debc09a9f8ba045a2c390a5d0dad.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. Item five, the design-bearing one, in metasystem/internal/steward/runner.go:
   the live-runner check now takes the replace path when the plan is
   the human-terminal mint, replace is false, and the enrolled bytes
   differ from the recorded identity. Check what is compared (which
   field of the recorded identity against which bytes), what happens
   when the recorded identity is absent or unreadable, that the
   temporary-word form and the machine-rebuild plan keep the short
   circuit, that the unchanged-bytes case still reports already armed
   and names restart, that the minted generation is human-terminal
   with itself as witness, and that the old runner is actually stopped
   before the new one starts (no two runners).
2. Item three in metasystem/internal/up/up.go: the not-landed remedy
   branch is keyed on the resolver's messages in
   metasystem/internal/steward/rearm_resolver.go; check the key
   matches all four not-landed messages and no other, that the fetch
   names the configured remote and the checkout root, and that the
   no-ref and does-not-resolve branches are unchanged.
3. Item four in up.go: the before-mint classification by substring;
   check each substring actually occurs in the arm error it claims to
   classify (read runner.go's error strings), that an unrecognised
   error still gets the old sentence, and that a drift error still
   takes the drift path first.
4. Items one and two in metasystem/scripts/adopt.sh: a detached target
   continues, prints its note, skips the upstream and landing-ref
   steps and exits zero; a preset landing ref is read with the same
   scope the Go side uses and kept; the non-detached, unset-key case
   is byte-for-byte the old behaviour. Bash 3.2: no associative
   arrays, no `${var,,}`, no `mapfile`.
5. Tests: the new legs in metasystem/scripts/adopt-fixtures.sh and
   the Go tests in metasystem/internal/up/up_test.go and
   metasystem/internal/steward/rearm_test.go prove what they claim
   and cannot pass vacuously (a remedy test that only checks the
   error is non-empty proves nothing); no existing assertion loosened;
   metasystem/scripts/agents/supervision-fixtures.sh still asserts
   what it asserted unless a changed default sentence forced an edit.
6. Conformance: only files under the brief's May-touch list; nothing
   under plans; steward_verbs.go and the resolver messages untouched.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory): `go test -count=1 ./internal/up ./internal/steward`,
`go vet ./internal/up ./internal/steward`, `gofmt -l ./internal/up ./internal/steward`,
`bash -n scripts/adopt.sh scripts/adopt-fixtures.sh`. The adopt fixture
bed and the supervision bed are the orchestrator's to run seat-side.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
