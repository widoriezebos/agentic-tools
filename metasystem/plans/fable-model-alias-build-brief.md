Working Mode: implement
Orchestrator Identity: m3+main-1788172645-85501-aa86ee (dispatch delegate under goal fable-model-alias)
Date: 2026-09-03

# Goal

Build the Fable model alias exactly as designed in
metasystem/plans/fable-model-alias-design.md (revision 2) as amended by
the dispositions in metasystem/records/misc/fable-model-alias-critique-r1.md
and metasystem/records/misc/fable-model-alias-critique-r2.md. Read all
three first, in that order; where the round-2 record amends the design,
the record wins. Wido's orders, verbatim: "i want claude-fable-5 to be an
alias for claude-fable-5.1 to avoid running into DESSIGNM-BEARING" and "I
want it to be the alias for the latest class 5 model, is that possible?";
his read-origin word is ruling R-71-m3 in metasystem/memory/rulings.md
("committed-only"). The design loop was closed by his word (R-72-m3)
with FIXTURES AS ARBITER: the six round-2 findings below are named
fixture obligations, each one a test that must exist and pass, and a
Fable code review follows this build.

# What to build (the design is the spec; this is the index)

1. `config.ResolveModelAlias` in metasystem/internal/config/model.go,
   tracked key family `runtime.<rt>.model-alias.<source>=<target>`, read
   committed-only the way `budgetLawValue` in
   metasystem/internal/config/budget.go reads budget keys (design §2).
2. Applied in `ResolveRoster` (metasystem/internal/dispatch/roster.go) to
   the roster id, the effective id and the `--model` override before the
   resolution is built; `RosterResolution` gains `AliasedFrom` and
   `RosterAliasedFrom` (design §1).
3. Follow-up seam: internal subverb `job resolve-model-alias --conf
   --runtime --model` printing `{"model","aliasedFrom"}`, called by
   metasystem/scripts/agents/dispatch.sh right after the newest record's
   `requestedModel` is read and before follow-up composition;
   `build-follow-record` gains `--model` AND `--aliased-from`;
   `BuildFollowRecord` (metasystem/internal/dispatch/build.go) takes the
   canonical model as a parameter (round-2 record, finding 1).
4. Validator rules in metasystem/internal/config/validate.go (design §2)
   including target admission against the tracked `maximal-models` line,
   with the `.local` overlay AND the environment value treated alike:
   named when the overlay omits the target, allowed when it lists the
   target plus a draining source (round-2 record, findings 2 and 4).
5. Cap chain (design §3): `ResolveCap` in
   metasystem/internal/dispatch/cap.go gains the source; order canonical
   role-pair, canonical pair, source role-pair
   (`config-role-pair-alias-source`), source pair
   (`config-pair-alias-source`), `dispatch.cap-min`, built-in;
   `job resolve-cap` and `claim-launch` in
   metasystem/cmd/metasystem/dispatch_verbs.go gain `--alias-source`;
   `RefuseUnsignedMissionCap` checks the same keys. Mission side
   (round-2 record, finding 3): `mission fence-authorize-cap`
   (metasystem/cmd/metasystem/mission.go) gains `--alias-source`,
   `mission.AuthorizeCap` in metasystem/internal/mission/fence.go orders
   canonical pair, source pair, general; dispatch.sh passes the source
   there too. The two rule strings are additive; the benchmark kit reads
   the vocabulary, so change no existing string.
6. Records (design §4): `aliasedFrom` and `rosterAliasedFrom` on
   `BuildRecord` (new `--aliased-from`, `--roster-aliased-from` on
   `build-record`) and on `BuildFollowRecord`; both join
   `immutableFields` in metasystem/internal/dispatch/record.go.
   `effectiveModel` is a raw OBSERVATION and is never rewritten
   (round-2 record, finding 6).
7. The conf line in metasystem/metasystem.conf, with the comment above it
   verbatim from design §4:
   `# Family pointer (R-71-m3): the source id MEANS the target at resolution; moved only by landing this line on Wido's word.`
   `runtime.claude.model-alias.claude-fable-5=claude-fable-5-1`

# Fixture obligations (the arbiter; each is a named test)

Every test in design §5, plus these six, one per round-2 finding:

- FMA-R2-FOLLOWUP-CANONICAL-RELAY: a dispatcher fixture in
  metasystem/scripts/agents/dispatch-fixtures.sh that follows up a
  CANNED legacy parent record whose `requestedModel` is literally the
  source (not a freshly aliased record) and asserts the child's
  `requestedModel` is the target, its `aliasedFrom` the source, and its
  `rosterAliasedFrom` null; plus a Go case on `BuildFollowRecord` that
  the written model is the parameter, not the parent's.
- FMA-R2-TARGET-ADMISSION-ENV-SHADOW: validator cases for the
  environment maximal-models value: named when it omits the target;
  accepted when it lists target plus draining source.
- FMA-R2-MISSION-CAP-SOURCE-BYPASS: in metasystem/internal/mission/fence_test.go,
  a signed source-pair row of 30 gives 30 (not the general mission cap)
  when the canonical pair has no row; an unsigned mission with only a
  source row refuses.
- FMA-R2-VALIDATOR-ALLOWANCES-UNTESTED: two positive validator cases:
  direct fan-in (two sources, one target) accepted; a `.local`
  maximal-models overlay listing target plus draining source accepted.
- FMA-R2-CLAUDE-GATE-FIXTURE-OMITTED: a Go composition test on a
  claude-runtime temp root (alias line, tracked maximal-models on the
  target, roster on the source): `ResolveRoster` then
  `ValidateRuntimeHazardConfiguration` passes; the same root without the
  alias line refuses with `REFUSED-HAZARD-CONFIGURATION`.
- FMA-R2-EFFECTIVE-MODEL-OBSERVATION-BYPASS: a canned runtime result
  reporting the SOURCE literal writes it into `effectiveModel` unchanged
  while `requestedModel` stays the target.

Existing fixtures that spell `claude-fable-5` in temp roots keep their
meaning (design §5, last bullet): no alias key there, the gate never
aliases. Change none of them.

# Gate

gofmt, go vet, go build; `GOFLAGS=-buildvcs=false go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./...` silent (the commit gate
runs it and refuses a chain on any line; metasystem/scripts/agents/go-gate.sh
`--fast` is the same check); go test -count=1 ./... green; one run of
metasystem/scripts/agents/dispatch-fixtures.sh in your sandbox (report
if the sandbox cannot run it, with the exact refusal). No benchmarks
(R-31), no sleeps (R-35). Leave the work in your working tree, stage
nothing, do not run the commit wrapper. diffBoundary: metasystem/internal/config,
metasystem/internal/dispatch, metasystem/internal/mission,
metasystem/internal/adapter, metasystem/cmd/metasystem,
metasystem/scripts/agents/dispatch.sh,
metasystem/scripts/agents/dispatch-fixtures.sh, metasystem/metasystem.conf,
and the new test files in those packages. Nothing under metasystem/plans,
metasystem/records or metasystem/memory. Paste the final gate lines and
the list of new test names in your return.

# Constraints

Wall-clock budget: 45 minutes. Version-2 implementer JSON. If a design
cite has drifted against the tree at your base, follow the tree and name
the drift in the return; do not redesign.

# Gap Rule

stop and report a gap; never fill it silently.
