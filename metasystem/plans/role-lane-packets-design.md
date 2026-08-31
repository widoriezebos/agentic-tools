# Role-Lane Packets Design

- Goal: role-lane-packets (plans/goals/role-lane-packets.md, revision 2)
- Authority: R-25-m1 (the lane map), R-25b-m1 (carried whole), R-28-m1
  (tier/effort delegation inside the lanes, due 2026-09-30), R-23-m1
  (cross-family independence), R-31-m2 (Sol pin as a recorded R-28 act),
  Ruling O effort floors (unchanged by this design)
- Authored: 2026-08-31 by the Fable design lane (fresh-context delegate,
  job implementer-61d326ee25fbea6019b05a58)
- Status: proposed; goes to the Sol design-critique lane per R-25-m1

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
  `ResolveHazardConfiguration` refuses on anything but exact equality
  (`reflect.DeepEqual` per class plus a length check). This is the
  exact-equality fingerprint law the goal cites: engine and file change
  together or loading refuses.
- `internal/dispatch/hazard.go:150-157` — typed refusal reason constants
  in the `REFUSED-R22-M1-RULING-O-*` shape.
- `internal/dispatch/hazard.go:261-302` — independent-critique closure
  checks freshness, role, effort, and session distinctness, but NOT
  model-family distinctness from the work it examines.
- `internal/dispatch/composition.go:17,29-42,129-143` — the table path
  constant, the `rolePacketTable`/`rolePacketRecipe` structs (roles carry
  only `sources` today), and `ComposeRolePacket`, which already calls
  `ResolveHazardConfiguration` and `ValidateRuntimeHazardConfiguration`
  and refuses with typed `CompositionRefusal` codes before the packet or
  job record is written.
- `internal/dispatch/composition.go:280-294` — `readRolePacketTable`
  refuses any `SchemaVersion != 1`.
- `internal/dispatch/roster.go:114-232` — `ResolveRoster` resolves
  `role.<role>.runtime`, falling through to `role.default.runtime`, with
  mode-scoped overrides applied by `config.Get` (e.g.
  `mode.design.role.implementer.runtime`); overrides classify as
  escalation via `model.tier.*`, and nothing checks families.
- `scripts/agents/dispatch.sh:1240-1265` — the working mode is extracted
  from the brief's single mandatory `Working Mode:` header
  (`brief_mode`), a `--mode` flag may only confirm it, and the mode is
  passed to `job resolve-roster`. Composition (line 1426) runs before the
  claim/publish calls (lines 1461+), so a composition refusal precedes
  any reservation spend.
- `internal/dispatch/hazard.go:379-444` — `CanonicalLaunchRequest`
  carries role, runtime, canonical model key, and `inputHash` (the brief
  bytes), but no mode field; the mode is nevertheless integrity-bound
  because it is a header inside the fingerprinted brief bytes.
- `internal/config/model.go:15-18` — `CanonicalModel` lowercases and
  collapses non-alphanumerics ("gpt-5.6-sol" → "gpt-5-6-sol").
- `metasystem.conf` + `metasystem.conf.local` (this machine) — today's
  lane carriage: `role.default.runtime=codex`,
  `role.default.model.codex=gpt-5.6-sol`,
  `role.default.model.claude=claude-opus-5`,
  `role.code-critic.runtime=claude` +
  `role.code-critic.model.claude=claude-fable-5`,
  `mode.design.role.implementer.runtime=claude` +
  `mode.design.role.implementer.model.claude=claude-fable-5`,
  `runtime.claude.maximal-models=claude-fable-5`. No explicit warden,
  behavior-judge, or steward-continuation runtime keys exist.

## 3. Decisions

### D1 — A lane is a runtime family, never a model

The lane STRUCTURE R-25-m1 fixes and R-28-m1 reserves to Wido is which
FAMILY authors and which FAMILY examines. R-25-m1 itself records that
Fable-over-Opus within the claude family was the dispatch delegate's
choice under the delegated word, and R-31-m2 shows a within-lane pin
(Sol) landing as conf + ruling, not as structure. Therefore:

- A lane names a runtime identifier (`claude`, `codex`) and nothing
  else. Every shipped runtime binds exactly one model vendor family
  (claude→Anthropic, codex→OpenAI, devin→Cognition, fake→fixtures), so
  runtime identity IS family identity today.
- Model names never appear in the lane encoding. Tier selection within a
  family stays where it lives now — `role.<role>.model.<runtime>` and
  `role.default.model.<runtime>` conf keys — under the R-28-m1
  delegation until its expiry, with pins like R-31-m2 recorded there.
- Benign variation follows for free: a new model within the lawful
  family (claude-fable-5.1, a future Sol point release) changes no lane
  byte and cannot refuse on lane grounds. The separate
  `runtime.<runtime>.maximal-models` effort gate (hazard.go:109-139)
  continues to govern whether that model may carry xhigh classes; that
  is Ruling O's surface, untouched here.
- CanonicalModel consequence: because lanes name no models,
  canonicalization cannot corrupt a lane match. The one binding rule for
  any future extension: if a lane ever needs a model-prefix restriction
  (a hypothetical multi-vendor runtime), the prefix MUST be expressed
  and compared in `CanonicalModel` output form. No such restriction
  ships now; a multi-vendor runtime does not exist to guard.

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
    "lane": { "authority": "R-25-m1", "family": "codex" },
    "sources": [ ... ]
  },
  "code-critic": {
    "lane": { "authority": "R-25-m1", "family": "claude" },
    "sources": [ ... ]
  },
  "warden": {
    "lane": { "authority": "R-25-m1", "family": "claude" },
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
  that mode only. Modes absent from the map inherit `family`. The only
  mode entry on day one is the implementer's `design` → `claude`,
  because R-25-m1 distinguishes exactly DESIGN authorship from
  IMPLEMENTATION. A future mode-scoped family deviation (e.g. a devin
  refactor lane, as the conf comments contemplate) is a structure change
  and takes the engine-plus-file path on Wido's word; until then the
  conservative reading of "IMPLEMENTATION is codex+Sol" holds for every
  non-design mode.
- `authority: "roster"` — R-25-m1 does not fix this role's family; the
  conf roster decides it exactly as today, and the lane entry exists
  only so requirement "every rostered role carries its lane" is
  satisfied honestly: the lane states that its family is
  roster-delegated rather than pretending a law that was never given.
  `family` and `modes` are absent (and refused if present).
- Warden carries the claude examination lane. This is the one derivation
  in this design rather than a literal quote: hazard.go:274 accepts a
  warden job as the independent critique of an implementation chain, and
  R-25-m1's law for IMPLEMENTATION CRITIQUE is claude+Fable; leaving
  warden on the codex default would let a Ruling-O-gated examination
  land same-family through the warden door. See the self-grade for the
  rejection condition.

### D3 — Engine mirror and the exact-equality fingerprint law

Following `requiredConfigurationByHazard` exactly, the engine gains:

```go
type RoleLane struct {
    Authority string            `json:"authority"`
    Family    string            `json:"family,omitempty"`
    Modes     map[string]string `json:"modes,omitempty"`
}

var requiredLaneByRole = map[string]RoleLane{
    "implementer":   {Authority: "R-25-m1", Family: "codex", Modes: map[string]string{"design": "claude"}},
    "design-critic": {Authority: "R-25-m1", Family: "codex"},
    "code-critic":   {Authority: "R-25-m1", Family: "claude"},
    "warden":        {Authority: "R-25-m1", Family: "claude"},
    "behavior-judge":       {Authority: "roster"},
    "investigator":         {Authority: "roster"},
    "steward-continuation": {Authority: "roster"},
    "verifier":             {Authority: "roster"},
}
```

`ResolveRoleLanes(root)` (new, in `internal/dispatch`, beside
`ResolveHazardConfiguration`) reads the packet table and refuses unless:

1. `SchemaVersion == 2` — this is what makes the coupling mutual. An old
   engine refuses the new file at `readRolePacketTable`'s existing
   version check; the new engine refuses a schemaVersion-1 file. JSON
   decoding ignores unknown fields, so WITHOUT the version bump an old
   engine would silently accept a lane-bearing file; the bump is
   therefore mandatory, not cosmetic. `readRolePacketTable`'s check
   moves from `!= 1` to `!= 2`, and `ResolveHazardConfiguration` is
   untouched otherwise.
2. `len(table.Roles) == len(requiredLaneByRole)` and every required role
   is present with `reflect.DeepEqual(table.Roles[role].Lane,
   requiredLaneByRole[role])`. Missing lane, extra role, extra mode
   entry, or differing family all refuse. Consequence, stated
   deliberately: adding a rostered role now requires editing the file
   AND the engine table together — the fingerprint law applied to
   lanes, consistent with how `TestRolePacketTableCoversEveryDispatchableRole`
   already couples the roles map to the dispatchable set.
3. Structural disjointness self-check (see D6): for every hazard class
   with `IndependentCritiqueRequired`, the builder lane's family differs
   from each examining lane's family — `implementer@design` ≠
   `design-critic`, and `implementer@default` ≠ `code-critic` and ≠
   `warden`. This runs over the engine's own constant so that even a
   joint engine-plus-file edit that made both surfaces equal-but-
   same-family still refuses unless this named invariant is also
   deleted — which is not quiet.

### D4 — Refusal at launch: where, with what reason, before what spend

The lane check is `ValidateRoleLane(root, role, mode, runtime) error`.
For a roster-authority lane it always passes. For an R-25-m1 lane it
resolves the lawful family (mode entry if present, else default) and
refuses when the resolved runtime differs. It fires at:

1. **Roster resolution** (`ResolveRoster`, roster.go): after the runtime
   is resolved and before model resolution. `ResolveRoster` already
   receives the mode (dispatch.sh:1263) and the conf path; it gains the
   packet-table root. This is the earliest point and catches conf drift
   and `--runtime` overrides alike — a lane crossing is a refusal, never
   an escalation question for the tier ladder.
2. **Brief assembly** (`ComposeRolePacket`, composition.go): beside the
   existing `ResolveHazardConfiguration` call. The engine re-derives the
   mode itself from the task-direction bytes it already reads
   (`p.Brief`), using the same grammar dispatch.sh's `brief_mode` uses —
   exactly one filled `Working Mode:` header, refusing on zero or many —
   so the check does not trust the shell. The derived mode is recorded
   as a new `Mode` field in `CompositionRecord`, making the lane a job
   was admitted under part of packet provenance. The brief bytes are
   already covered by `inputHash` inside the launch fingerprint
   (hazard.go:392), so the mode is integrity-bound without a
   `LaunchFingerprintVersion` bump.
3. **Claim/admission re-validation** (build.go:310, claim.go:80,
   claim.go:227, beside `ValidateRuntimeHazardConfiguration`): these
   paths hold a `CanonicalLaunchRequest`, which has no mode, so they
   enforce the mode-independent projection — the runtime must be in the
   role's lawful family SET (default family ∪ all mode families). For
   every role but the implementer that set has one element, so the check
   is exact; for the implementer it admits {codex, claude}, and the
   exact (role, mode) binding is carried by points 1 and 2, which run
   before reservation spend in the dispatch flow (dispatch.sh: resolve
   1263 → compose 1426 → claim 1461+). Requirement "refused before any
   budget spends" is satisfied at points 1–2; point 3 is defense in
   depth for non-shell callers.

Typed reasons, following the existing shapes:

- Composition and roster refusals: `CompositionRefusal` code
  `REFUSED-ROLE-LANE`, `Source` naming the role, `Detail` naming role,
  mode, resolved runtime, lawful family, and — when a conf key or
  override produced the runtime — that key or flag, so a human reads
  which surface to reconcile.
- Table/fingerprint mismatches surface through `ResolveRoleLanes`'s
  error, wrapped at composition as `REFUSED-ROLE-LANE-TABLE` (parallel
  to `REFUSED-HAZARD-CONFIGURATION`).
- Closure-time same-family refusal (slice 2, D6):
  `REFUSED-R25-M1-SAME-FAMILY-EXAMINATION`, an `OpError` reason constant
  in the hazard.go:150 block's style.

What never refuses: a lawful-family launch with any model (D1), a
roster-authority role on any rostered runtime, and `runtime=main` roles,
which are not dispatchable at all (roster.go:133) and carry
roster-authority lanes.

### D5 — Conf versus packet: precedence, conflict, migration

The question the brief demands an explicit answer to. Precedence, in
order:

- **P1.** For an R-25-m1 lane, the packet family is law. No conf key,
  environment variable, or CLI flag outranks it.
- **P2.** An EXPLICIT role-specific runtime key that contradicts the
  lane — `role.<role>.runtime` or `mode.<mode>.role.<role>.runtime`
  resolving to a runtime outside the lawful family — REFUSES with
  `REFUSED-ROLE-LANE` naming the key. It is never silently overridden
  and never silently wins: a contradicting conf records a drifted
  intent, and per R-25b the seat's pen does not quietly resolve it; a
  human reconciles the conf or, on Wido's word, the lane.
- **P3.** When no role-specific runtime key exists and resolution would
  fall through to the generic `role.default.runtime`, the lane SUPPLIES
  the runtime instead of the generic default. The generic default is a
  fallback, not an intent, and the lane outranks a fallback. Model
  resolution then proceeds through the existing
  `role.<role>.model.<family>` / `role.default.model.<family>` keys
  unchanged — this is exactly where the R-28-m1 tier delegation
  continues to live.
- **P4.** `--runtime`/`--model` overrides crossing the lane refuse
  identically at P2's refusal. The escalation ladder ranks cost within
  lawful configurations; it cannot buy a lane crossing.
- **P5.** Roster-authority roles resolve exactly as today; no behavior
  change.

Migration, day one, audited against this machine's live conf:

- Every existing key AGREES with the lanes: implementer default codex
  (committed `role.implementer.runtime=codex`), design-critic codex,
  code-critic claude (`role.code-critic.runtime=claude`), implementer
  design mode claude (`mode.design.role.implementer.runtime=claude`).
  Zero conflicts, zero refusals, zero conf deletions. The runtime keys
  for lane-fixed roles become redundant-but-checked: kept, verified,
  refused only on contradiction.
- The one behavior change is the warden: with no explicit warden key it
  today falls through to codex and under P3 will resolve claude, model
  `role.default.model.claude=claude-opus-5`. Because claude-opus-5 is
  not in `runtime.claude.maximal-models`, a warden carrying a
  Ruling-O-gated critique would then fail the existing effort-proof
  gate. The remedy is one conf.local line —
  `role.warden.model.claude=claude-fable-5` — which is precisely an
  R-28-m1 tier choice: the dispatch delegate records it with reasoning
  where it lands. It is deliberately NOT part of the implementation
  slice (conf edits are out of the goal's diff and conf.local is
  machine-local); the design names it so no seat discovers it as a
  surprise refusal.
- Longer term (out of scope, named for honesty): conftailor guidance for
  adopting projects, so tailored confs stop emitting runtime keys for
  lane-fixed roles.

### D6 — Cross-family independence stays derivable and unquiet

Three layers, so no single edit can quietly seat one family on both
sides of a Ruling-O-gated examination:

1. The lane constants themselves (D2/D3): builder and examiner families
   are disjoint by construction for every dispatched path.
2. The load-time disjointness self-check in `ResolveRoleLanes` (D3.3):
   any future lane change — even a lawful joint engine-plus-file edit —
   that collapses builder and examiner into one family refuses until
   the invariant itself is loudly removed.
3. Closure-time (slice 2): `validateIndependentCritiqueReference` gains
   a family check — the critic job's runtime must differ from the final
   work job's runtime — refusing with
   `REFUSED-R25-M1-SAME-FAMILY-EXAMINATION`. This closes the residual
   channel layers 1–2 cannot see: evidence jobs assembled outside the
   standard dispatch flow, and any roster-authority role that ever
   stands in as an examiner. It runs only where that function already
   runs, i.e. exactly the classes Ruling O gates.

### D7 — What this design does not touch

The `destructiveReach` table and Ruling O floors (bytes identical, law
identical). `ResolveHazardConfiguration`'s class checks. The launch
fingerprint version. Effort-tier and model-selection conf keys. The
brief templates. Any adapter. And per R-25b-m1: the implementation brief
carries this design whole; a deviation returns through the design lane
or rises to Wido, never the carrying seat's pen.

## 4. Provability — the tests that prove it

Extend the hazard-configuration family in
`internal/dispatch/composition_test.go` (the
`TestHazardConfiguration*` group) and `TestResolveRosterDecisions` in
`roster_test.go`:

1. `TestRoleLaneRefusesWrongFamilyAtComposition` — a wrong-lane fixture:
   implementer brief with `Working Mode: design` dispatched on codex →
   `REFUSED-ROLE-LANE`, no packet bytes written (mirrors
   `TestComposeRolePacketRefusesCallerSourceOutsideRecipeBeforeWriting`).
2. `TestRoleLaneAcceptsLawfulFamilyAndAnyModelWithin` — a right-lane
   fixture launches: code-critic on claude with a model name absent from
   every conf list passes the lane check (benign variation), and
   implementer on codex in default mode composes end to end.
3. `TestRoleLaneRefusesTamperedPacketTable` — a tampered packet file
   refuses: design-critic lane flipped to claude; a role's lane deleted;
   an extra role added; `schemaVersion` left at 1 — each refuses with
   the exact-equality error (mirrors
   `TestHazardConfigurationRefusesAWeakenedMinimum`).
4. `TestResolveRosterRefusesExplicitConfContradictingLane` —
   `mode.design.role.implementer.runtime=codex` in the fixture conf →
   typed refusal naming that key (P2), and a `--runtime` override across
   the lane refuses identically (P4).
5. `TestResolveRosterLaneOutranksGenericDefault` — warden with only
   `role.default.runtime=codex` resolves claude (P3).
6. `TestRoleLaneMirrorRefusesSameFamilyExamination` — the load-time
   disjointness self-check fires on a doctored constant/table pair
   (D6.2).
7. Slice 2: `TestHazardClosureRefusesSameFamilyIndependentCritique` —
   extends `TestHazardDutiesGateChainCompletion`'s fixture set with a
   critic job whose runtime equals the final work job's runtime →
   `REFUSED-R25-M1-SAME-FAMILY-EXAMINATION`.

## 5. Slice plan (R-17: first slice ≤ 4 hours)

**Slice 1 — the lane law lands (≤ 4h, the goal's appetite).** The
implementer builds exactly: (a) `role-packets.json` schemaVersion 2 with
the eight `lane` objects of D2; (b) the `RoleLane` type,
`requiredLaneByRole` mirror, `ResolveRoleLanes` with the version bump,
exact-equality and disjointness checks (D3); (c) `ValidateRoleLane`
wired into `ResolveRoster` with precedence P1–P5 and into
`ComposeRolePacket` with engine-side `Working Mode` derivation and the
new `CompositionRecord.Mode` field (D4.1–D4.2); (d) the typed refusal
codes `REFUSED-ROLE-LANE` and `REFUSED-ROLE-LANE-TABLE`; (e) tests 1–6.
This alone discharges requirements 1–5 and layers 1–2 of requirement 6,
and every refusal precedes reservation spend.

**Slice 2 — closure and claim-path depth (separate, after slice 1
lands).** The claim/build mode-independent projection checks (D4.3), the
closure-time same-family refusal (D6.3), and test 7.

**Waits, explicitly:** the warden model tier conf.local line (an R-28
recorded act by the seat, not repository code); conftailor emission
changes for adopters; any model-prefix lane extension (no multi-vendor
runtime exists); any new mode lane (Wido's word plus the engine-and-file
edit).

## 6. Self-grade (R-24-m1)

- **Confidence:** high on the encoding, mirror, and refusal mechanics —
  they are byte-for-byte patterned on the shipped, tested hazard-table
  law (`ResolveHazardConfiguration` and its test family), and the
  day-one conf audit was performed against this machine's live conf, not
  assumed. Medium on the precedence rule P3 (lane outranks only the
  generic default), which changes warden's resolved runtime on day one.
- **Weakest claim:** the warden's fixed claude lane. It is a derivation
  — hazard closure accepts warden as implementation critique
  (hazard.go:274) and R-25-m1 assigns implementation critique to claude
  — not Wido's literal word, and it is the only lane row with a
  day-one behavioral consequence.
- **Reject this design if:** Wido states the warden sits outside the
  R-25-m1 lane map. In that case warden's row becomes
  `{"authority": "roster"}` and the closure-time disjointness check
  (D6.3) moves from slice 2 into slice 1, because it then carries
  requirement 6's warden channel alone. Independently, reject D4.2 if
  the `brief_mode` header grammar proves not mechanically identical
  between the shell and a Go re-implementation — the mode derivation
  must be one grammar or the two admission points can disagree.
