# Fable model alias — design

Goal `plans/goals/fable-model-alias.md`. Wido, verbatim (2026-09-03): "i want claude-fable-5 to be an alias for
claude-fable-5.1 to avoid running into DESSIGNM-BEARING", then "I want it to be the alias for the latest class 5 model,
is that possible?". Answer: yes, as a TRACKED POINTER moved only by a one-line configuration landing on his word.
Extends `plans/fable-5-1-rollover-design.md` (R-46-m0b); cites are relative to the metasystem root on branch
agent/fma-design-2 at 99bc045e. Provenance: authored whole by the Fable design delegate (job fma-design-2).

## 1. Where the alias is applied: inside `ResolveRoster`, before any pair is formed

The roster read is the only place a model id is BORN: `ResolveRoster` (`internal/dispatch/roster.go:114-232`) reads
`role.<role>.model.<rt>` / `role.default.model.<rt>` (`:157-181`) and lets `--model` replace the effective id
(`:182-185`). Every later consumer gets that string unchanged: `dispatch.sh:1274` copies `model` from the verb's JSON
and passes it to `compose-role-packet` (`:1437`; gate `composition.go:141`, refusal `REFUSED-HAZARD-CONFIGURATION` at
`:142`, the one Wido hit), to the cap key (`:1457`, `cap.go:36-41`), to `claim-launch` (`:1472`; fingerprint
`claim_fingerprint.go:110`, `canonicalModelKey` `claim.go:694`), to `build-record` (`:1538`; gate `build.go:310`,
`requestedModel` `build.go:409`); the adapter reads `requestedModel` from the record for the CLI `--model`
(`runtime-common.sh:84`, `claude.sh:126`); follow-ups re-read it from the latest record (`dispatch.sh:1762`). The
steward's revive path calls `ResolveRoster` directly (`cmd/metasystem/steward_verbs.go:394`). **Decision.** New `config.ResolveModelAlias(confPath, runtime, model) (canonical string, aliased bool, err)` in
`internal/config/model.go` beside `CanonicalModel`, applied by `ResolveRoster` to three values BEFORE `out` is built at
`roster.go:187`: `rosterModel` (`:157-168`), `requestedModel` (`:170-181`) and `p.ModelOverride` (`:182`). So `Model`,
`RosterModel`, `RequestedPair`, `RosterPair` and the tier comparison (`:196-231`) are all canonical. Yes, `--model
claude-fable-5` is aliased too: on a roster at `claude-fable-5-1` the pairs become equal, `Overridden` stays true
(`:189`), no escalation is asked (`:196`). `RosterResolution` gains one field, `AliasedFrom string json:"aliasedFrom"`
(empty when nothing fired, else the literal effective id). No other file rewrites ids. Why nowhere later: in
`dispatch.sh` after the verb, the steward revive path (`steward_verbs.go:394-410`) still stages the old id; at
`claim-launch` (`dispatch_verbs.go:233`) the composition gate (`composition.go:141`, run first at `:1437`) still
refuses; inside `runtimeProvesMaximalExecution` (`hazard.go:109-139`) the gate passes but `requestedModel`,
`canonicalModelKey`, the cap key, the composition `Model` (`composition.go:151`) and the CLI `--model` all carry the
old id, which R-46-m0b forbids; in the adapter, everything above it lies.

## 2. Where the table lives: a tracked key family, never code, never the API

**Decision: `runtime.<runtime>.model-alias.<source>=<target>` in `metasystem.conf`**, today one line:
`runtime.claude.model-alias.claude-fable-5=claude-fable-5-1`. A later 5.2 is one landing of this line on Wido's word.
The key matches `confKeyPattern` (`internal/config/resolve.go:15`). Explicit non-goal: the engine never asks the API
what "latest" is; the pointer is a static value in a committed file. Why not a hard-coded retired-id map: moving the
pointer would be a code build, not a landing; and a code map would silently change the meaning of every temp-root
fixture that spells `claude-fable-5` as an arbitrary string (§5), whereas a conf key is data a fixture conf lacks.
Read origin: TRACKED ONLY. `ResolveModelAlias` reads the family the way `budgetLawValue` reads budget keys
(`internal/config/budget.go:89-119`): an alias key in `.local` or the environment is refused by name with the same
"accepts only committed root configuration" wording, outside a fixture-authorized root (`budget.go:123-125`), because
a `.local` pointer would move the lane's model without a landing (R-46-m0b). The order does not say this in words; the
return reports it as a gap. `Validate` (`internal/config/validate.go:22`) gains a `modelAliasKey` regex beside
`maximalModelsKey` (`:555`) and checks per key: runtime in `metasystem.runtimes`; source and target non-empty after
trim; both equal to their own `CanonicalModel` (the fingerprint refuses a non-canonical key, `claim_fingerprint.go:291`);
target != source; target not itself a source (no chains); source absent from the tracked `runtime.<rt>.maximal-models`
(an id cannot be retired and admitted at once); plus the `.local`/env refusal, mirroring `validate.go:356-374`. The
target is not required in `maximal-models`: after aliasing it is the id the gate checks.

## 3. Cap rows: canonical id only, no fallback

**Decision: `ResolveCap` is unchanged** (`cap.go:36-69`); it receives the canonical key of the aliased model
(`dispatch.sh:1457`), so `cap.min.code-critic.claude.claude-fable-5=30` is never consulted. Why no fallback: the chain
is role-pair, pair, general (`cap.go:39-43`); a fallback at the role-pair step would let an old-id role row (30) beat a
pair row the operator wrote FOR the dispatched model (say `cap.min.claude.claude-fable-5-1=200`), silently giving the
role a smaller cap than its operator wrote. Canonical-only descends only to rows written for the dispatched model or
for everyone; the seat adds rows on the new id (rollover design §2). `RefuseUnsignedMissionCap` (`cap.go:75-89`) is
keyed the same way. Whether the validator should name a stale old-id row is a gap in the return.

## 4. Records and reporting

`build-record` gains `--aliased-from`, fed from the verb's `aliasedFrom` in `dispatch.sh`; `BuildRecord`
(`build.go:254`) writes `"aliasedFrom": nullableString(p.AliasedFrom)` beside `requestedModel` (`build.go:409`).
`requestedModel`, `effectiveModel` and `canonicalModelKey` carry only the canonical id. Follow-up records inherit the
canonical id (`dispatch.sh:1762`) and carry no `aliasedFrom`; the alias fires only where a roster is resolved. Sweep:
`grep -l '"aliasedFrom": "claude-fable-5"' artifacts/agents/jobs/*.json` per seat. Conf comment above the key:
`# Family pointer (R-71-m3): the source id MEANS the target at roster resolution; moved only by landing this line on
Wido's word; never read from .local or the environment.`

## 5. Tests

- `TestResolveRosterDecisions` (`roster_test.go:21`) gains three cases: "alias rewrites the roster id" (alias line plus
  `role.default.model.claude=claude-fable-5`; `Model`, `RosterModel`, both pairs on `claude-fable-5-1`, `AliasedFrom`
  `claude-fable-5`); "alias rewrites --model" (roster on `claude-fable-5-1`, override `claude-fable-5`; equal pairs,
  `Overridden` true, `EscalationRequired` false); "unaliased id passes through" (no alias key; `AliasedFrom` empty).
- `TestHazardConfigurationAcceptsAliasedRoster` (`composition_test.go`): temp root, tracked
  `maximal-models=claude-fable-5-1` plus the alias line, roster on `claude-fable-5`; `ResolveRoster` then
  `ValidateRuntimeHazardConfiguration` passes; `BuildRecord` shows `requestedModel` `claude-fable-5-1`, `aliasedFrom`
  `claude-fable-5`.
- `TestValidateRejections` (`validate_test.go:75`) gains: self-alias, chained alias, empty target, non-canonical
  source, source listed in `maximal-models`, alias key in `.local` (refused by name).
- `TestResolveCapIgnoresAliasSourceRow` (`cap_test`): `.local` holds only the old-id role row; rule `config-general`.
- Existing fixtures on `claude-fable-5` (`composition_test.go:275,278,441,447`, `decisions_test.go:449-488`,
  `claim_test.go:75`, `validate_test.go:44-104`, `delegate_reroute_test.go:485,497`) keep their meaning: each runs in a
  temp root whose conf has no alias key, and the gate itself never aliases. No renames.

## 6. Non-goals

No new CLI verb (`--aliased-from` is an internal flag on an existing verb); no change to R-25-m1 lane structure; the
`maximal-models` line keeps its meaning and value; no API discovery of "latest"; no migration of any seat's
`conf.local`. Codex and devin: the family is runtime-generic and `ResolveRoster` is runtime-agnostic, so aliasing them
costs nothing extra, but no codex or devin line lands under this goal. Live-round safety (rollover design §5) holds:
closure re-checks the critic's RECORDED `requestedModel` (`hazard.go:293`), never rewritten for past jobs and canonical
for new ones; `maximal-models` is untouched, so nothing in flight is refused.
## 7. The ruling row: append after R-71-m2 (`memory/rulings.md:120`; R-70-m3 and R-70-m2 coexist, so the id is number plus machine)
```text
| R-71-m3 | 2026-09-03 | CLAUDE-FABLE-5 IS THE FAMILY POINTER TO THE LATEST FABLE 5.x (Wido, verbatim: "i want claude-fable-5 to be an alias for claude-fable-5.1 to avoid running into DESSIGNM-BEARING", refined minutes later: "I want it to be the alias for the latest class 5 model, is that possible?"): the tracked line runtime.claude.model-alias.claude-fable-5=claude-fable-5-1 makes the family id resolve to the canonical id inside ResolveRoster, once, before any gate, cap key, fingerprint, record field or CLI argument sees it; the pointer moves only by a landing of that one line on his word, never by a code build and never by the engine asking the API what is latest; the alias key is read from the committed file only. R-46-m0b stands: claude-fable-5 still never reaches the Claude CLI; cap rows are read on the canonical id only | Given 2026-09-03 afternoon; the refusal it answers was REFUSED-HAZARD-CONFIGURATION on a seat whose conf.local still named claude-fable-5 against the literal gate in hazard.go | Wido | |
```
