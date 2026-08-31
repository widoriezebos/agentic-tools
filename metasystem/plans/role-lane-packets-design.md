# Role-Lane Packets Design

- Goal: role-lane-packets (plans/goals/role-lane-packets.md, revision 2)
- Authority: R-25-m1 (the lane map), R-25b-m1 (carried whole), R-28-m1
  (tier/effort delegation inside the lanes, due 2026-09-30) as narrowed
  by R-31-m2 (Sol implementation-lane pin) and R-34-m2 (no model
  substitution on any lane for any reason without Wido's explicit
  per-case approval), R-23-m1 (cross-family independence), Ruling O
  effort floors (unchanged by this design)
- Authored: 2026-08-31 by the Fable design lane (fresh-context delegate,
  job implementer-61d326ee25fbea6019b05a58)
- Status: revised against critique round 1
  (design-critic-38f304b59d2bb62c9e1711ab); all five findings
  (RLP-R1-001 through RLP-R1-005) folded — none refuted, none deferred;
  awaiting re-critique

## 0. Critique round 1 dispositions

- RLP-R1-001 (critical) ACCEPTED: model substitution is now refused at
  launch; the "any model within the family never refuses" permission is
  withdrawn. See D1 (revised).
- RLP-R1-002 (critical) ACCEPTED: examiner-to-reviewed-work-mode
  binding is now encoded and enforced at admission, before spend, in
  slice 1. See D6a (new).
- RLP-R1-003 (high) ACCEPTED: follow-up rounds inherit the mode from
  the parent job's record instead of requiring a Working Mode header in
  the correction message. See D4.2 (revised).
- RLP-R1-004 (high) ACCEPTED: the warden's model key ships inside
  slice 1's own commit, so governed warden launches work on day one.
  See D5 (revised).
- RLP-R1-005 (high) ACCEPTED: lane-table validation gains a
  generic-JSON presence pass (exact member sets, refusing unknown,
  present-empty, and present-null members) before the typed
  exact-equality mirror, on the shipped
  `configurationObligationsMatchObject` pattern. See D3 (revised).

## 1. Problem

R-25-m1 fixes who sits in which seat — DESIGN authored by claude+Fable,
DESIGN CRITIQUE by codex+Sol, IMPLEMENTATION by codex+Sol, IMPLEMENTATION
CRITIQUE by claude+Fable — but today those lanes are carried by hand in
every launch: they live in `metasystem.conf`/`metasystem.conf.local` keys
that any conf edit can silently reroute, and nothing refuses a launch that
puts a role on the wrong lane. The ruling names `role-packets.json`, the
closed-packet recipe file, as the enforcing surface. This design decides
how lanes enter that file, what the engine expects in return, and how a
wrong-lane launch is refused before any budget spends.

## 2. Facts traced

Every mechanism this design builds on was read at these locations:

- `scripts/agents/role-packets.json` — schemaVersion 1; a
  `destructiveReach` table of three classes and a `roles` map of eight
  recipes (behavior-judge, code-critic, design-critic, implementer,
  investigator, steward-continuation, verifier, warden), each only
  `sources`.
- `internal/dispatch/hazard.go:36-76` — `requiredConfigurationByHazard`
  is the engine-owned mirror of the file's `destructiveReach` table;
  `ResolveHazardConfiguration` refuses on anything but exact equality.
  This is the exact-equality fingerprint law the goal cites.
- `internal/dispatch/hazard.go:141-148` —
  `configurationObligationsMatchObject` validates a generic JSON object
  by exact member count plus per-member equality: the shipped precedent
  for presence-strict validation that `reflect.DeepEqual` over typed
  structs cannot provide (RLP-R1-005).
- `internal/dispatch/hazard.go:150-157` — typed refusal reason constants
  in the `REFUSED-R22-M1-RULING-O-*` shape.
- `internal/dispatch/hazard.go:216-225, 261-302` — final-work selection
  excludes all critic roles, and independent-critique closure accepts
  code-critic, design-critic, or warden with no relationship to the
  reviewed work's mode and no family-distinctness check (RLP-R1-002).
- `internal/dispatch/composition.go:17,29-42,129-143` — the table path
  constant, the `rolePacketTable`/`rolePacketRecipe` structs, and
  `ComposeRolePacket`, which refuses with typed `CompositionRefusal`
  codes before any packet bytes are written.
- `internal/dispatch/composition.go:280-294` — `readRolePacketTable`
  decodes with plain `json.Unmarshal` into typed structs (unknown
  members silently dropped) and refuses any `SchemaVersion != 1`.
- `internal/dispatch/roster.go:114-232` — `ResolveRoster` resolves
  runtime and model from role-specific keys falling through to
  `role.default.*`, with mode-scoped overrides; a `--model` override is
  classified through the cost-escalation ladder, not refused
  (RLP-R1-001).
- `scripts/agents/dispatch.sh:1222-1227` — a code-critic or warden
  dispatch requires `--reviews` naming a job whose record has role
  `implementer`; the reviewed job's working mode is never checked
  (RLP-R1-002).
- `scripts/agents/dispatch.sh:1240-1265` — fresh dispatch extracts the
  mode from the brief's single mandatory `Working Mode:` header and
  passes it to roster resolution; composition (line 1426) precedes the
  claim/publish calls (1461+), so a composition refusal precedes any
  reservation spend.
- `scripts/agents/dispatch.sh:1875-1893` and
  `scripts/agents/templates/follow-up.md` — follow-up composition
  passes the correction message as the composition brief, and the
  follow-up template has no `Working Mode:` header (RLP-R1-003).
- `internal/dispatch/hazard.go:379-444` — `CanonicalLaunchRequest`
  carries role, runtime, canonical model key, and `inputHash` (the
  brief bytes), but no mode field.
- `internal/config/model.go:15-18` — `CanonicalModel` lowercases and
  collapses non-alphanumerics ("gpt-5.6-sol" → "gpt-5-6-sol"). The
  model-equality check in D1 compares in this form.
- `metasystem.conf` + `metasystem.conf.local` (this machine) — today's
  lane carriage: `role.default.runtime=codex`,
  `role.default.model.codex=gpt-5.6-sol`,
  `role.default.model.claude=claude-opus-5`,
  `role.code-critic.runtime=claude` +
  `role.code-critic.model.claude=claude-fable-5`,
  `mode.design.role.implementer.runtime=claude` +
  `mode.design.role.implementer.model.claude=claude-fable-5`,
  `runtime.claude.maximal-models=claude-fable-5`. No explicit warden,
  behavior-judge, or steward-continuation runtime or model keys exist.
- `memory/rulings.md` — R-31-m2 pins the Sol implementation lane to
  gpt-5.6-sol; R-34-m2 (verbatim: "no model substitution on any lane
  for any reason without his explicit per-case approval") hardens it
  program-wide. R-34-m2 postdates this worktree's branch point and was
  read from the integration branch; the dispatching orchestrator
  verified both rows before this revision.

## 3. Decisions

### D1 — A lane names a runtime family; the model is pinned, not free

(Revised for RLP-R1-001.) The lane STRUCTURE R-25-m1 fixes is which
FAMILY authors and which FAMILY examines, and the encoding keeps
families as runtime identifiers (`claude`, `codex`): every shipped
runtime binds exactly one model vendor family. But the round-1 claim
that any same-family model launches freely is WITHDRAWN: R-31-m2 pins
the Sol implementation lane, and R-34-m2 prohibits model substitution
on ANY lane without Wido's explicit per-case approval. The R-28-m1
tier delegation survives only as the right to propose and record a
model choice in the roster — never as a per-launch liberty. Therefore:

- The roster-recorded model IS the lane model. At launch, the effective
  model must equal the roster-resolved model for the (role, mode)
  pair, compared in `CanonicalModel` form so spelling variants cannot
  dodge the check. A `--model` override that differs refuses with
  `REFUSED-ROLE-LANE-MODEL`; the cost-escalation ladder no longer
  classifies model substitution at all (it retains runtime-level
  ranking for roster-authority roles only).
- An approved substitution is not a launch-time act: it lands as a conf
  edit to the `role.<role>.model.<runtime>` key carrying Wido's
  recorded per-case approval (the R-31-m2 shape: conf value plus
  ruling row). The engine enforces the boundary it can see — no launch
  deviates from the recorded roster — and the recording of the
  approval itself remains a human-governed act on the conf surface.
- Model names still do not enter `role-packets.json`. Rationale,
  restated against the finding: the packet+engine pair encodes what
  changes only with the lane map itself (Wido's structural word), while
  R-34-m2 approvals are per-case acts that land on the conf surface
  where R-31-m2 already lives. The enforcement gap the finding named is
  closed at launch by the equality check above, not by freezing every
  approved substitution into an engine release.
- "Benign variation must not refuse" is now scoped honestly: a new
  model within the lawful family enters through a recorded,
  Wido-approved conf edit and then launches without any lane refusal
  and without any engine or packet-file change. What no longer exists
  is a refusal-free per-launch substitution path.
- CanonicalModel consequence: lane FAMILY matching still names no
  models; the model-equality check is the one model comparison and is
  performed on canonical keys. Any future model-prefix extension must
  also compare in canonical form.

### D2 — Encoding: a `lane` object on every role, schemaVersion 2

`role-packets.json` moves to `"schemaVersion": 2`. Every entry in
`roles` gains a required `lane` object beside `sources`:

```json
"roles": {
  "implementer": {
    "lane": { "authority": "R-25-m1", "family": "codex",
              "modes": { "design": "claude" } },
    "sources": [ ... unchanged ... ]
  },
  "design-critic": {
    "lane": { "authority": "R-25-m1", "family": "codex",
              "examines": "design" },
    "sources": [ ... ]
  },
  "code-critic": {
    "lane": { "authority": "R-25-m1", "family": "claude",
              "examines": "implementation" },
    "sources": [ ... ]
  },
  "warden": {
    "lane": { "authority": "R-25-m1", "family": "claude",
              "examines": "implementation" },
    "sources": [ ... ]
  },
  "behavior-judge":        { "lane": { "authority": "roster" }, "sources": [ ... ] },
  "investigator":          { "lane": { "authority": "roster" }, "sources": [ ... ] },
  "steward-continuation":  { "lane": { "authority": "roster" }, "sources": [ ... ] },
  "verifier":              { "lane": { "authority": "roster" }, "sources": [ ... ] }
}
```

Semantics:

- `authority: "R-25-m1"` — the family is law. `family` is the default
  lane; `modes` maps a working mode to a different lawful family for
  that mode only; modes absent from the map inherit `family`. The only
  mode entry on day one is the implementer's `design` → `claude`.
- `examines` (new, for RLP-R1-002) — the work-mode class this examiner
  lane is lawful for: `"design"` means the reviewed implementer job's
  recorded mode must be `design`; `"implementation"` means it must NOT
  be `design` (every non-design implementer mode is implementation
  work under R-25-m1's map). Present exactly on the three examiner
  lanes; refused elsewhere. This is what makes "Sol builds, Claude
  critiques; Fable designs, Sol critiques" mechanical rather than
  assumed: a code-critic or warden pointed at a design-mode
  (Claude-authored) job refuses at admission, before spend.
- `authority: "roster"` — R-25-m1 does not fix this role's family; the
  conf roster decides it exactly as today. `family`, `modes`, and
  `examines` are absent (and their presence refuses — see D3).
- The warden's fixed claude lane is now SCOPED, answering the round-1
  weakest claim and the critique's line-3 verdict: claude is law for
  the warden only as an implementation examiner, and the `examines`
  binding guarantees a warden never examines design-mode work at all.

### D3 — Engine mirror, presence-strict validation, exact equality

(Revised for RLP-R1-005.) Following `requiredConfigurationByHazard`,
the engine gains a `RoleLane` type (`Authority`, `Family`, `Modes`,
`Examines`) and a `requiredLaneByRole` constant mirroring section D2's
table exactly. `ResolveRoleLanes(root)` refuses unless ALL of:

1. `SchemaVersion == 2`. This makes the coupling mutual: the old engine
   refuses the new file at its existing version check, and the new
   engine refuses a schemaVersion-1 file. Without the bump, JSON
   decoding would let an old engine silently accept a lane-bearing
   file.
2. **Presence pass (generic JSON).** The `roles` value is re-decoded as
   `map[string]any` and validated member-exactly, on the
   `configurationObligationsMatchObject` pattern (hazard.go:141-148):
   each role object has exactly the members `{"lane","sources"}`; each
   R-25-m1 lane object has exactly the members its row requires
   (`{"authority","family","modes"}` for the implementer,
   `{"authority","family","examines"}` for the three examiners); each
   roster lane has exactly `{"authority"}`. Any unknown member, any
   forbidden-but-present member, a present-empty string, or a
   present-null value refuses. This closes the typed-decoder blindness
   the finding proved: `family:""` versus family-omitted, `modes:null`
   versus modes-omitted, and silently-dropped unknown members are all
   distinct and all refused here, before the typed comparison runs.
3. **Typed pass.** `len(table.Roles) == len(requiredLaneByRole)` and
   every required role present with `reflect.DeepEqual` against the
   mirror. Missing lane, extra role, extra mode entry, or differing
   family or examines value refuses. Adding a rostered role now
   requires editing file AND engine together — the fingerprint law
   applied to lanes.
4. **Disjointness self-check.** For every hazard class with
   `IndependentCritiqueRequired`: the design-mode builder family
   (implementer@design) differs from the design examiner's family, and
   the implementation builder family (implementer@default) differs
   from every `examines:"implementation"` lane's family. Runs over the
   engine's own constant so even a joint engine-plus-file edit that
   collapsed builder and examiner into one family refuses until this
   named invariant is loudly deleted.

### D4 — Refusal at launch: where, with what reason, before what spend

The lane check is `ValidateRoleLane(root, role, mode, runtime,
canonicalModel, rosterCanonicalModel) error`, refusing on family
mismatch (`REFUSED-ROLE-LANE`) or model deviation from the roster
resolution (`REFUSED-ROLE-LANE-MODEL`, D1). It fires at:

1. **Roster resolution** (`ResolveRoster`): after runtime and model
   resolution, using the mode dispatch.sh already passes. Catches conf
   drift and `--runtime`/`--model` overrides alike; a lane crossing or
   model substitution is a refusal, never an escalation question.
2. **Brief assembly** (`ComposeRolePacket`), revised for RLP-R1-003.
   Mode derivation is round-aware:
   - Round 1 (fresh dispatch): the engine derives the mode from the
     task-direction bytes it already reads — exactly one filled
     `Working Mode:` header, refusing on zero or many, the same
     grammar as dispatch.sh's `brief_mode`.
   - Rounds > 1 (follow-up, including the fresh-context continuation
     fallback): the composition brief is the correction message and
     the shipped follow-up template intentionally has no mode header,
     so requiring one would refuse every correction round. Instead the
     mode is INHERITED from the parent job record's recorded `mode`
     field (added in this slice; see below). A correction message that
     does carry a `Working Mode:` header must match the inherited mode
     or refuse — a follow-up may never move a job between lanes.
   The derived or inherited mode is recorded in
   `CompositionRecord.Mode` AND in the job record as a new `mode`
   field, which is what makes inheritance and examiner binding (D6a)
   mechanical. Round-1 mode integrity is already covered by the brief
   bytes inside the fingerprinted `inputHash`; the job-record field is
   the queryable projection of the same fact.
3. **Claim/admission re-validation** (build.go:310, claim.go:80,
   claim.go:227): these paths hold a `CanonicalLaunchRequest`, which
   has no mode, so they enforce the mode-independent projection — the
   runtime must be in the role's lawful family set — as slice-2
   defense in depth. The exact (role, mode) and model checks live at
   points 1–2, which precede reservation spend in the dispatch flow
   (resolve 1263 → compose 1426 → claim 1461+).

Typed reasons: `REFUSED-ROLE-LANE` (family), `REFUSED-ROLE-LANE-MODEL`
(model deviation), `REFUSED-ROLE-LANE-TABLE` (table/fingerprint
mismatch, wrapping `ResolveRoleLanes` errors at composition),
`REFUSED-ROLE-LANE-EXAMINER` (D6a), and at closure (slice 2)
`REFUSED-R25-M1-SAME-FAMILY-EXAMINATION` in the hazard.go:150 constant
style. Each refusal detail names role, mode, resolved runtime and
model, the lawful value, and the conf key or flag that produced the
unlawful one.

What never refuses: a launch on the roster-recorded model of a lawful
family, a roster-authority role on any rostered runtime, and follow-up
rounds continuing a lawfully admitted job.

### D5 — Conf versus packet: precedence, conflict, migration

Precedence, in order:

- **P1.** For an R-25-m1 lane, the packet family is law. No conf key,
  environment variable, or CLI flag outranks it.
- **P2.** An EXPLICIT role-specific runtime key
  (`role.<role>.runtime` or `mode.<mode>.role.<role>.runtime`)
  contradicting the lane refuses with `REFUSED-ROLE-LANE` naming the
  key. Never silently overridden, never silently winning: a human
  reconciles the conf or, on Wido's word, the lane (R-25b).
- **P3.** With no role-specific runtime key, the lane SUPPLIES the
  runtime; the generic `role.default.runtime` is a fallback and the
  lane outranks a fallback. Model resolution then proceeds through the
  existing model keys — the surface where R-28/R-31/R-34-recorded
  choices live — and the resolved model becomes the pinned launch
  model (D1).
- **P4.** `--runtime` overrides crossing the lane refuse at P2's
  refusal; `--model` overrides deviating from the roster resolution
  refuse at `REFUSED-ROLE-LANE-MODEL` on EVERY lane, R-25-fixed or
  roster-authority, per R-34-m2's "any lane" wording.
- **P5.** Roster-authority roles otherwise resolve exactly as today.

Migration, day one (revised for RLP-R1-004), audited against this
machine's merged conf:

- Every existing key AGREES with the lanes: implementer default codex,
  design-critic codex, code-critic claude with claude-fable-5,
  implementer design-mode claude with claude-fable-5. Zero conflicts
  and zero deletions among existing keys.
- The warden is the one resolution change (codex default → claude via
  P3), and round 1's deferral of its model key is withdrawn: under
  `role.default.model.claude=claude-opus-5` every Ruling-O-gated
  warden composition would refuse the maximal-model proof
  (hazard.go:91-105 against `runtime.claude.maximal-models=
  claude-fable-5`), which made slice 1 knowingly non-operational.
  **Slice 1's own commit therefore adds
  `role.warden.model.claude=claude-fable-5` to `metasystem.conf`.**
  This names no new model choice needing an R-34-m2 approval: Fable is
  the model R-25-m1 itself names for the implementation-critique seat
  the warden occupies. With it, the day-one claim becomes true as
  stated: zero conflicts, zero refusals, one committed key addition.
- Longer term (out of scope, named for honesty): conftailor emission
  guidance for adopting projects.

### D6 — Cross-family independence: derivable, unquiet, launch-sound

1. The lane constants (D2/D3): builder and examiner families are
   disjoint by construction.
2. The load-time disjointness self-check (D3.4).
3. **D6a (new, slice 1, for RLP-R1-002) — examiner binding at
   admission.** When a critic dispatch names `--reviews`, admission
   loads the reviewed job record and refuses
   `REFUSED-ROLE-LANE-EXAMINER` unless the examiner lane's `examines`
   class matches the reviewed job's recorded `mode` (design-critic ↔
   mode design; code-critic and warden ↔ any non-design mode). This
   runs in the engine beside the existing role-of-reviewed-job check
   that dispatch.sh:1222-1227 performs in shell today, and it fires
   before composition and reservation — the wrong-family examination
   the finding demonstrated (a claude warden or code-critic examining
   a claude-authored design) can no longer spend a cent. Reviewed jobs
   admitted before this slice have no `mode` field; a missing field
   refuses, which is the fail-closed reading (pre-slice jobs are
   re-examined only through new admissions).
4. Closure-time (slice 2, defense in depth behind D6a):
   `validateIndependentCritiqueReference` gains the same
   examines-to-mode check plus a family check — the critic job's
   runtime must differ from the final work job's runtime — refusing
   `REFUSED-R25-M1-SAME-FAMILY-EXAMINATION`. This closes the residual
   channel for evidence assembled outside the standard dispatch flow.

### D7 — What this design does not touch

The `destructiveReach` table and Ruling O floors.
`ResolveHazardConfiguration`'s class checks. The launch fingerprint
version. The brief and follow-up templates (RLP-R1-003 is solved by
inheritance, not by template edits). Any adapter. And per R-25b-m1:
the implementation brief carries this design whole; a deviation
returns through the design lane or rises to Wido.

## 4. Provability — the tests that prove it

Extend the `TestHazardConfiguration*` family in
`internal/dispatch/composition_test.go` and `TestResolveRosterDecisions`
in `roster_test.go`:

1. `TestRoleLaneRefusesWrongFamilyAtComposition` — implementer brief
   with `Working Mode: design` on codex → `REFUSED-ROLE-LANE`, no
   packet bytes written.
2. `TestRoleLaneAcceptsRosterModelAndRefusesSubstitution` — the
   right-lane fixture launches on the roster-recorded model; a
   `--model` override differing from the roster resolution (same
   family) refuses `REFUSED-ROLE-LANE-MODEL`; a spelling variant of
   the same model (canonicalization) does NOT refuse.
3. `TestRoleLaneRefusesTamperedPacketTable` — lane flipped, lane
   deleted, extra role, schemaVersion 1, AND the presence-pass
   fixtures the round-1 list omitted: `"family": ""` on a roster row,
   `"modes": null`, an unknown lane member, `examines` on a
   non-examiner row — each refuses.
4. `TestResolveRosterRefusesExplicitConfContradictingLane` — a
   contradicting mode-scoped runtime key refuses naming the key; a
   cross-lane `--runtime` override refuses identically.
5. `TestResolveRosterLaneOutranksGenericDefault` — warden with only
   `role.default.runtime=codex` resolves claude, and with the slice's
   conf line resolves model claude-fable-5, passing the maximal gate.
6. `TestRoleLaneMirrorRefusesSameFamilyExamination` — the load-time
   disjointness self-check fires on a doctored constant/table pair.
7. `TestFollowUpInheritsModeAndRefusesLaneMove` — a follow-up round
   composes without any `Working Mode:` header (the shipped template
   shape) inheriting the parent record's mode; a correction message
   carrying a different mode header refuses.
8. `TestExaminerBindingRefusesWrongModeReview` — a code-critic or
   warden dispatch reviewing a design-mode implementer job refuses
   `REFUSED-ROLE-LANE-EXAMINER` before composition; a design-critic
   reviewing implementation-mode work refuses symmetrically; a
   reviewed record with no `mode` field refuses (fail-closed).
9. Slice 2: `TestHazardClosureRefusesSameFamilyIndependentCritique`
   and the closure-side examines-to-mode check, extending
   `TestHazardDutiesGateChainCompletion`.

## 5. Slice plan (R-17: first slice ≤ 4 hours)

Task-level estimate for slice 1 (answering the line-5 verdict that the
four-hour claim was unsupported):

| Task | Estimate |
| --- | --- |
| (a) `role-packets.json` schemaVersion 2 with lane objects incl. `examines` | 20m |
| (b) `RoleLane` mirror, presence pass, typed pass, disjointness check | 50m |
| (c) `ValidateRoleLane` (family + model equality) wired into `ResolveRoster`, P1–P5 | 40m |
| (d) Composition mode derivation with follow-up inheritance; `mode` in `CompositionRecord` and job record | 45m |
| (e) Examiner binding at admission (D6a) | 30m |
| (f) `role.warden.model.claude=claude-fable-5` conf line + migration audit | 10m |
| (g) Tests 1–8 (many share fixtures with existing hazard tests) | 45m |
| Total | 4h00m |

**Slice 1 — the lane law lands (4h, at the goal's appetite ceiling).**
Tasks (a)–(g) above, all typed refusal codes of D4, nothing else. This
discharges requirements 1–5 and requirement 6's dispatched paths, with
every refusal preceding reservation spend, working correction rounds,
and an operational warden — the three independent-deployment failures
the critique proved are each closed inside this slice. The estimate
sits exactly at the ceiling: any overrun is a stop-and-report to the
orchestrator, never a silent scope cut (R-17, R-25b).

**Slice 2 — closure and claim-path depth.** The claim/build
mode-independent projection checks (D4.3), the closure-time
same-family and examines-to-mode refusals (D6.4), and test group 9.

**Waits, explicitly:** conftailor emission changes for adopters; any
model-prefix lane extension (no multi-vendor runtime exists); any new
mode lane (Wido's word plus the engine-and-file edit). Nothing a
governed slice-1 launch needs is deferred any longer.

## 6. Self-grade (R-24-m1, refreshed for revision 1)

- **Confidence:** high on the folds for RLP-R1-002, -003, -004, and
  -005 — each closes at a named deterministic boundary the critique
  itself located, using shipped patterns
  (`configurationObligationsMatchObject`, the job-record join that
  `--reviews` admission already performs in shell). Medium-high on the
  RLP-R1-001 fold, which turns on where an R-34-m2 approval is allowed
  to land.
- **Weakest claim:** that the conf model key is the lawful landing
  surface for a Wido-approved substitution — i.e., that the engine
  discharges R-34-m2 by refusing every launch-time deviation from the
  roster while the approval itself stays a human-governed conf-plus-
  ruling act the engine does not verify. A second, smaller soft spot:
  follow-up mode inheritance trusts the parent job record's `mode`
  field, whose write path is slice 1's own code.
- **Reject this design if:** Wido rules that R-34-m2 approvals must be
  machine-verified at launch (an approval-reference artifact checked
  by the engine, the R-14 envelope shape) rather than recorded on the
  conf surface — then D1 needs an approval-reference channel and this
  revision is insufficient. Independently, reject the D6a fail-closed
  rule if Wido wants pre-slice implementer jobs (no recorded mode) to
  remain reviewable without re-admission; the design would then need
  an explicit legacy-mode derivation instead of refusal.
