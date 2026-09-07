Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal effort-by-complexity-per-model, tier 2, hazard DESIGN-BEARING)
Date: 2026-09-07

# Goal

Wido's word (2026-09-06): the reasoning effort a delegate runs with is
configuration, per hazard class, per model, in the MODEL'S OWN WORDS,
beside the model keys the configuration already carries. Today
metasystem/internal/dispatch/hazard.go fixes it per hazard class for
every runtime (requiredConfigurationByHazard: medium for MECHANICAL,
xhigh for DESIGN-BEARING and DESTRUCTIVE-REACH), the composition
copies that value into the job record (metasystem/internal/dispatch/build.go,
the reasoningEffort field and the equality check that refuses any
other value), metasystem/scripts/agents/dispatch.sh reads it from
composition.json and passes --reasoning-effort to the adapter, the
codex adapter sends it as model_reasoning_effort
(metasystem/internal/adapter/codex.go), and the claude adapter sends
nothing at all although the CLI takes `--effort <level>` (low, medium,
high, xhigh, max). Wido's first line for this feature: Fable runs at
high, not xhigh.

When you are done: (1) the effort comes from configuration keys
resolved per role, runtime and hazard class in the same order as the
model keys; (2) a value is validated against the named runtime's own
vocabulary and a bad value refuses at dispatch naming the key and the
accepted words; (3) hazard.go keeps only an admission floor per class,
a configured value weaker than the floor refuses (never silently
raised), stronger is allowed; (4) the job record shows the effort sent
and the key it came from; (5) the claude adapter sends `--effort`; (6)
fixtures pin it; (7) the shipped configuration carries Fable at high.

# The design

1. Keys, resolved in the model keys' order (read how
   metasystem/internal/dispatch/roster.go resolves
   mode.<mode>.role.<role>.model.<runtime>, role.<role>.model.<runtime>,
   role.default.model.<runtime> and mirror it):
   `mode.<mode>.role.<role>.effort.<runtime>.<class>`,
   `role.<role>.effort.<runtime>.<class>`,
   `role.default.effort.<runtime>.<class>`, where <class> is the
   hazard class in lower case (mechanical, design-bearing,
   destructive-reach). No key set means the floor value for that
   runtime and class (so nothing changes for an unconfigured
   repository).
2. Vocabularies, in the engine, one per runtime, ordered weakest to
   strongest: codex minimal, low, medium, high, xhigh; claude low,
   medium, high, xhigh, max; devin and fake have no effort switch and
   accept only the empty value (their no-op). Validation names the key
   and the accepted list on refusal.
3. Floors in hazard.go, per class, as words that exist in both
   vocabularies: MECHANICAL medium; DESIGN-BEARING high;
   DESTRUCTIVE-REACH high. Weaker than the floor refuses at dispatch
   naming the key, the value and the floor. The role packet table
   (metasystem/scripts/agents/role-packets.json, destructiveReach) and
   the ResolveHazardConfiguration comparison follow the floor words;
   the builderReasoningEffort and independentCritiqueReasoningEffort
   in ConfigurationObligations become the floor. The existing
   maximal-models proof (runtimeProvesMaximalExecution) no longer keys
   on the word xhigh: keep it as "a runtime's top word requires the
   runtime.<runtime>.maximal-models proof" only if a configured value
   is that runtime's strongest word; otherwise drop the check and say
   so in the return.
4. Resolution happens in the composition (Go, where the record is
   built): the record gains `reasoningEffort` (the value sent, as
   today) and `reasoningEffortSource` (the key it came from, or
   "floor:<class>"); the equality check in build.go compares the
   caller-supplied --reasoning-effort against the resolved value, and
   the follow-up round composition carries the parent's resolved
   value, not the floor.
5. The claude adapter: BuildClaudeCommand gains the effort and appends
   `--effort <value>` when non-empty; the claude-command verb in
   metasystem/cmd/metasystem/adapter_runtime_verbs.go takes
   --reasoning-effort like the codex verb; metasystem/scripts/agents/adapters/claude.sh
   passes it. The codex path is unchanged.
6. The shipped metasystem/metasystem.conf gains, beside the role model
   lines: role.default.effort.codex.mechanical=medium,
   role.default.effort.codex.design-bearing=xhigh,
   role.default.effort.codex.destructive-reach=xhigh,
   role.default.effort.claude.mechanical=medium,
   role.default.effort.claude.design-bearing=high,
   role.default.effort.claude.destructive-reach=high, with a comment
   naming the vocabularies and the floors.
7. Fixtures in metasystem/scripts/agents/dispatch-fixtures.sh (the
   fake runtime is the no-switch runtime): a role line shadows the
   default; a bad value refuses naming the key and the accepted list;
   under-floor refuses naming the floor; the no-switch runtime accepts
   only the empty value; the recorded effort and source equal what the
   adapter received (the fake adapter records its argv; for codex and
   claude a Go test on the command builders pins the flag). Update the
   existing assertion that expects xhigh on the fake implementer
   record to the value the fixture configures.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/dispatch/hazard.go
May touch: metasystem/internal/dispatch/build.go
May touch: metasystem/internal/dispatch/composition.go
May touch: metasystem/internal/dispatch/roster.go
May touch: metasystem/internal/dispatch (tests)
May touch: metasystem/internal/adapter/claude.go
May touch: metasystem/internal/adapter/claudecommand_test.go
May touch: metasystem/cmd/metasystem/adapter_runtime_verbs.go
May touch: metasystem/scripts/agents/adapters/claude.sh
May touch: metasystem/scripts/agents/dispatch.sh (the effort read and pass only)
May touch: metasystem/scripts/agents/dispatch-fixtures.sh
May touch: metasystem/scripts/agents/role-packets.json (the destructiveReach effort words only)
May touch: metasystem/metasystem.conf (the new effort lines and their comment only)
Must not touch: the codex adapter's flag, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 120 minutes
  of wall clock. Hazard DESIGN-BEARING: admission floors and a new
  adapter flag; one Fable critique follows.
- The dispatch fixture bed needs host state your sandbox lacks: run
  the Go tests and bash -n, and say the bed is the orchestrator's.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/dispatch ./internal/adapter ./cmd/metasystem` (expected: ok; say which tests pin the floors, the vocabularies and the claude flag)
- `go vet ./internal/dispatch ./internal/adapter ./cmd/metasystem` and `gofmt -l` on the same (expected: clean)
- `bash -n ./scripts/agents/dispatch.sh ./scripts/agents/adapters/claude.sh ./scripts/agents/dispatch-fixtures.sh` (expected: clean)
- `git diff --stat` (expected: only files under May touch)

# Acceptance Criteria

1. A DESIGN-BEARING claude dispatch with the shipped configuration
   records reasoningEffort high from role.default.effort.claude.design-bearing
   and the claude argv carries --effort high; a codex one records
   xhigh from its key and the codex argv is unchanged.
2. A bad word, an under-floor word and a non-empty word for the
   no-switch runtime refuse at dispatch naming the key.
3. The fixtures and Go tests pin all of it; the bed is green on m1.

# Gap Rule

stop and report a gap; never fill it silently.
