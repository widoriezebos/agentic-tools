# Fable model alias — design, revision 2 (folds Sol critique round 1)

Goal `plans/goals/fable-model-alias.md`. Wido, verbatim (2026-09-03): "i want claude-fable-5 to be an alias for claude-fable-5.1 to
avoid running into DESSIGNM-BEARING", then "I want it to be the alias for the latest class 5 model, is that possible?". Answer: yes, as a
TRACKED POINTER moved only by a one-line configuration landing on his word. Extends `plans/fable-5-1-rollover-design.md` (R-46-m0b);
cites relative to the metasystem root at bc227e39. Provenance: Fable design delegate, jobs fma-design-2 and fma-design-2-r2. Confirmations
FMA-R1-ROLLOVER-SHAPE-SOUND, FMA-R1-OVERRIDE-FINGERPRINT-SOUND and FMA-R1-EXISTING-FIXTURES-SOUND: text kept.
Seat fold (m3, disclosed): Wido's word on the read origin, given after critique round 1, replaces the pending slots in §2 and §7; no other seat edit.
## 1. Where the alias is applied: `ResolveRoster` for fresh dispatch, the inherited id for follow-ups
Fresh dispatch: `ResolveRoster` (`internal/dispatch/roster.go:114-232`) reads `role.<role>.model.<rt>` / `role.default.model.<rt>`
(`:157-181`) and lets `--model` replace the effective id (`:182-185`). Every later consumer gets that string unchanged: `dispatch.sh:1274`
copies `model` into `compose-role-packet` (`:1437`; gate `composition.go:141`, refusal `REFUSED-HAZARD-CONFIGURATION` at `:142`, the one
Wido hit), the cap key (`:1457`, `cap.go:36-41`), `claim-launch` (`:1472`; `claim_fingerprint.go:110`, `canonicalModelKey` `claim.go:694`)
and `build-record` (`:1538`; gate `build.go:310`, `requestedModel` `build.go:409`); the adapter reads `requestedModel` for the CLI `--model`
(`runtime-common.sh:84`, `claude.sh:126`, `BuildClaudeCommand` `internal/adapter/claude.go:254-287`). The steward's revive path calls
`ResolveRoster` directly (`steward_verbs.go:394`). **Decision.** New `config.ResolveModelAlias(confPath, runtime, model) (canonical string, aliased bool, err)` in `internal/config/model.go`
beside `CanonicalModel`, applied by `ResolveRoster` to `rosterModel` (`:157-168`), `requestedModel` (`:170-181`) and `p.ModelOverride`
(`:182`) BEFORE `out` is built at `:187`, so `Model`, `RosterModel`, both pairs and the tier comparison (`:196-231`) are canonical.
`--model claude-fable-5` is aliased too: on a roster at `claude-fable-5-1` the pairs become equal, `Overridden` stays true (`:189`), no
escalation is asked (`:196`). `RosterResolution` gains `AliasedFrom` and `RosterAliasedFrom` (§4). Why nowhere later: in the shell, the steward path
still stages the old id; at `claim-launch` (`dispatch_verbs.go:233`) the composition gate (`:1437`, earlier) still refuses; inside
`runtimeProvesMaximalExecution` (`hazard.go:109-139`) the gate passes but every record field, the cap key, the composition `Model`
(`composition.go:151`) and the CLI `--model` carry the old id (R-46-m0b).
**Disposition FMA-R1-FOLLOWUP-BYPASS: accepted; second application point.** Follow-ups never call `ResolveRoster`: `dispatch.sh:1762`
reads `requestedModel` from the newest record and passes it unchanged to composition (`:1900-1901`), the cap key (`:1920`), `claim-launch`
(`:1935`, `:1966`) and `build-follow-record` (`:2005`), whose `BuildFollowRecord` copies the parent's `requestedModel` onto the child
(`build.go:524`, `:596`). Seam: a new internal subverb `job resolve-model-alias --conf <conf> --runtime <rt> --model <id>` (job table
`cmd/metasystem/main.go:110`; a thin relay of `ResolveModelAlias`) printing `{"model","aliasedFrom"}`; `dispatch.sh` calls it right after
`:1762` and overwrites `model` before `:1900`; `build-follow-record` gains `--aliased-from`, and `BuildFollowRecord` writes the child's
`requestedModel` from the canonical value, its own `aliasedFrom`, and `rosterAliasedFrom` null. Past records stay literal; closure
re-checks the recorded critic id (`hazard.go:293`, rollover design §5).
## 2. Where the table lives: a tracked key family, never code, never the API
**Decision: `runtime.<runtime>.model-alias.<source>=<target>` in `metasystem.conf`**, today one line:
`runtime.claude.model-alias.claude-fable-5=claude-fable-5-1`; a later 5.2 is one landing of this line on Wido's word. The key matches
`confKeyPattern` (`internal/config/resolve.go:15`). Non-goal: the engine never asks the API what "latest" is. Why not a hard-coded map:
moving the pointer would be a code build, and a code map would silently change the meaning of every temp-root fixture that spells
`claude-fable-5` as an arbitrary string (§5).
**Disposition FMA-R1-READ-ORIGIN-AUTHORITY: attribution accepted; now Wido's word (R-71-m3), verbatim: "Yes: committed-only — a local
alias would silently reinterpret every role and every explicit --model, which a local roster line doesn't."** So: TRACKED ONLY. `ResolveModelAlias` reads the family the way `budgetLawValue` reads budget
keys (`internal/config/budget.go:89-119`): an alias key in `.local` or the environment is refused by name with the "accepts only
committed root configuration" wording outside a fixture-authorized root (`budget.go:123-125`). `Validate` (`internal/config/validate.go:22`) gains a `modelAliasKey` regex beside `maximalModelsKey` (`:555`) and checks per key:
runtime in `metasystem.runtimes`; source and target non-empty after trim and canonical (`claim_fingerprint.go:291` refuses otherwise);
target != source; target not itself a source (no chains); source absent from the tracked `runtime.<rt>.maximal-models`; plus the
`.local`/env alias refusal mirroring `validate.go:356-374`. **Disposition FMA-R1-TARGET-NOT-ADMITTED: accepted; rule restored.** Every
target must be present in the tracked `runtime.<rt>.maximal-models` line (`validate.go:134-155` parses it), so an alias never resolves
to an id the maximal gate then refuses under its generic message; a `.local` `runtime.<rt>.maximal-models` overlay (read first by the
gate, `resolve.go:87-95`) that omits the target is named as a problem citing the `.local` line, as the alias overlay is.
## 3. Cap rows: canonical first, then the alias source, then general
**Disposition FMA-R1-CAP-ROW-FALLTHROUGH: accepted; ordered fallback.** `ResolveCap` (`cap.go:36-69`) gains a `source` parameter; its
`steps` table (`cap.go:39-43`) becomes canonical role-pair, canonical pair, then only when `source` is non-empty the source role-pair
(rule `config-role-pair-alias-source`) and source pair (rule `config-pair-alias-source`), then `dispatch.cap-min`, then built-in. A row
written for the dispatched model always wins, and a stale 30-minute family row no longer silently becomes the 120-minute general cap.
`job resolve-cap` (`dispatch_verbs.go:1473-1500`) and `claim-launch` (`:233-234`) gain `--alias-source`; `dispatch.sh` passes the literal
beside the canonical key at `:1457-1472` and `:1920-1935`. `RefuseUnsignedMissionCap` (`cap.go:75-89`) checks the same four keys. The
rule vocabulary is wire read by the benchmark kit (`cap.go:11-14`); the two new strings are additive. No validator line for stale old-id
rows is needed: they are consulted by rule and named in the cap resolution.
## 4. Records and reporting
**Disposition FMA-R1-ALIAS-PROVENANCE-SHAPE: accepted.** Two immutable provenance fields on every record, written by `BuildRecord`
(`build.go:365-430`, from new `--aliased-from` and `--roster-aliased-from` on `build-record`) and by `BuildFollowRecord` (§1):
`aliasedFrom` (literal of the effective input when its alias fired, else null) and `rosterAliasedFrom` (literal roster id when the roster
alias fired, else null). Combined case: roster on source A and `--model` on source B gives A and B; roster on A and `--model` on the
target gives A and null; no override gives A and A. Both join `immutableFields` (`internal/dispatch/record.go:60-75`) so the generic
patch refuses them (`record.go:523-524`). `requestedModel`, `effectiveModel` (written by the handshake, `handshake.go:107`) and
`canonicalModelKey` carry only the canonical id. Sweep: `grep -lE '"(aliasedFrom|rosterAliasedFrom)": "claude-fable-5"' artifacts/agents/jobs/*.json`.
Conf comment above the key: `# Family pointer (R-71-m3): the source id MEANS the target at resolution; moved only by landing this line on Wido's word.`
## 5. Tests
- `TestResolveRosterDecisions` (`roster_test.go:21`) gains: roster on a source (both pairs on the target, `RosterAliasedFrom` set);
  `--model` on a source (equal pairs, `Overridden` true, no escalation, `AliasedFrom` set); §4's combined cases; unaliased id unchanged.
- **Disposition FMA-R1-VALIDATOR-TEST-MATRIX: accepted.** `TestValidateRejections` (`validate_test.go:75`) gains one case per rule:
  unrostered runtime, empty source, empty target, non-canonical source, non-canonical target, self-alias, chain, source in
  `maximal-models`, target absent from tracked `maximal-models`, `.local` `maximal-models` overlay omitting the target, overlay alias,
  environment alias. `TestResolveModelAliasOrigins` (`model_test.go`): overlay and environment refused outside a fixture-authorized root
  (`metasystem.runtimes=fake`), accepted inside one.
- `TestResolveCapAliasOrder` (`cap_test`): source role row 30 alone gives 30 with rule `config-role-pair-alias-source`, not 120; target
  pair row 200 plus source role row 30 gives 200 with rule `config-pair`; no source keeps the old chain.
- **Disposition FMA-R1-END-TO-END-PROOF: accepted.** `BuildRecord` cannot prove `canonicalModelKey` (the reservation writes it,
  `claim.go:694`), `effectiveModel` (the handshake) or the Claude argv (the adapter). Two dispatcher-level fixtures in
  `scripts/agents/dispatch-fixtures.sh` beside the happy-record field check (`:1286-1291`), on the fake runtime with
  `role.default.model.fake=fake-source`, tracked `runtime.fake.maximal-models=fake-model` and `runtime.fake.model-alias.fake-source=fake-model`
  (conf tailored at `:439-447`): one from the source-valued roster, one from `--model fake-source` on a roster at `fake-model`; each asserts
  `composition.model`, the cap resolution key, the reservation's `canonicalModelKey`, `requestedModel`, `effectiveModel` (the fake handshake
  echoes `requestedModel`, `adapters/fake.sh:193-194`, `handshake.go:107`) and both provenance fields: all `fake-model`, `fake-source` only in
  the provenance fields. A third fixture follows up the first and asserts the child's `requestedModel` and `aliasedFrom`. The fake runtime
  cannot prove the Claude argv: `TestBuildClaudeCommandArgv` (`internal/adapter/claudecommand_test.go:45`) gains a case that the `--model`
  argument is exactly the model passed in, and the adapter passes only `requestedModel`.
- Existing fixtures on `claude-fable-5` (`composition_test.go:275,278,441,447`, `decisions_test.go:449-488`, `claim_test.go:75`,
  `validate_test.go:44-104`, `delegate_reroute_test.go:485,497`) keep their meaning: temp roots with no alias key; the gate never aliases.
## 6. Non-goals
No new OPERATOR verb; revision 1's "no new CLI verb" is narrowed by §1's internal `job resolve-model-alias` subverb (reported as a gap).
No change to R-25-m1 lane structure; the `maximal-models` line keeps its meaning and value; no API discovery of "latest"; no migration of
any seat's `conf.local`. Codex and devin: the family and `ResolveRoster` are runtime-agnostic, so aliasing them costs nothing extra, but
no codex or devin line lands. Live-round safety (rollover design §5) holds: closure re-checks the critic's RECORDED `requestedModel`
(`hazard.go:293`), never rewritten for past jobs and canonical for new ones; `maximal-models` is untouched.
## 7. The ruling row: R-71-m3, minted in `memory/rulings.md` by the seat with all three of Wido's statements verbatim
The row records both orders and his read-origin word, the tracked line, the two application points (roster resolution, follow-up
inheritance), the pointer moving only by a landing on his word, the refusal of an alias key in `.local` or the environment, and that
R-46-m0b stands. Its text is the ledger's, not this design's; the build brief cites the row, not this section.
