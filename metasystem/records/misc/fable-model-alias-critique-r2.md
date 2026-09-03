# Fable model alias design — Sol critique, round 2

Critic: codex gpt-5.6-sol (job fma-critique-2-r2, follow-up on the same critic chain, DESIGN-BEARING, read-only) over plans/fable-model-alias-design.md revision 2 at 7e1dfe00. Six material findings (four high, two medium), four closures confirmed. Trajectory 7 -> 6 material; every round-2 finding is mechanical-grain (a flag, a named fixture, a bounded choice), none invariant-grade. Persisted verbatim by the m3 seat, 2026-09-03. Dispositions follow.

## FMA-R2-FOLLOWUP-CANONICAL-RELAY — high
CLAIM: Revision 2 does not close FMA-R1-FOLLOWUP-BYPASS because the canonical follow-up model has no specified relay into the child record, and the proposed fixture cannot detect the omission. The design says build-follow-record gains only --aliased-from and then says BuildFollowRecord writes requestedModel from the canonical value. The shipped function, however, has no model parameter: it reads parent requestedModel and copies it. Moreover, the third fixture follows the first newly aliased record, whose requestedModel is already fake-model, so leaving the shipped copy unchanged still passes. The design must name how the canonical model enters BuildFollowRecord, such as an explicit --model argument, and the fixture must start from a legacy parent whose requestedModel is literally fake-source. Otherwise a legacy source can still reach the child requestedModel and then the Claude --model argument.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:27-30 specifies only --aliased-from; metasystem/internal/dispatch/build.go:524 and :596 read and copy the parent model; metasystem/scripts/agents/dispatch.sh:2005-2015 currently passes no model; and metasystem/plans/fable-model-alias-design.md:79-83 makes the first fixture canonical before following it.
MATERIAL: True

## FMA-R2-TARGET-ADMISSION-ENV-SHADOW — high
CLAIM: Revision 2 closes the tracked and machine-local parts of FMA-R1-TARGET-NOT-ADMITTED, but leaves an environment-shadow hole. It requires the target in tracked maximal-models and names a machine-local overlay that omits it, while the actual gate reads an environment value before either file. An environment maximal-models value that omits claude-fable-5-1 therefore recreates the generic REFUSED-HAZARD-CONFIGURATION failure. The rule and tests must treat the environment like the machine-local overlay: name omission of the target, while allowing a dual source-and-target value needed for draining an old chain.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:44-47 and :69-73 mention only the machine-local maximal-models shadow. metasystem/internal/config/resolve.go:67-69 gives the environment first precedence, and metasystem/internal/dispatch/hazard.go:113-138 consumes that effective value literally.
MATERIAL: True

## FMA-R2-READ-ORIGIN-CLOSED — low
CLAIM: FMA-R1-READ-ORIGIN-AUTHORITY is closed and is not reopened. Revision 2 quotes Wido's committed-only answer, requires machine-local and environment alias keys to be refused, and records the ruling in R-71-m3.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:38-47 quotes the answer and defines committed-only resolution. metasystem/memory/rulings.md:122 records all three of Wido's statements verbatim.
MATERIAL: False

## FMA-R2-CONFIG-CAP-FALLBACK-CLOSED — low
CLAIM: The ordinary configuration-cap problem in FMA-R1-CAP-ROW-FALLTHROUGH is closed. Revision 2 specifies canonical role and pair rows first, alias-source role and pair rows second, and general defaults last, with distinct additive provenance rules and a test covering the 30-minute rather than 120-minute outcome.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:48-56 defines the ordered chain, and :74-75 asserts both the selected number and rule. The separate signed mission-cap omission is reported independently below.
MATERIAL: False

## FMA-R2-MISSION-CAP-SOURCE-BYPASS — high
CLAIM: The new alias-source cap contract omits the signed mission-fence resolver, so a mission still resolves a cap without the source. The shell passes only the canonical model to mission fence authorize-cap; mission.AuthorizeCap checks only cap.min.<runtime>.<canonical-model> and otherwise falls to fence.job-cap-min. A signed cap.min.claude.claude-fable-5=30 row can therefore become a larger general mission cap after aliasing. Revision 2's plan to make RefuseUnsignedMissionCap inspect alias-source rows is also asymmetric while signed authority ignores those rows. The alias source must either be passed through the mission fence with canonical-pair, source-pair, general ordering, or signed mission rows must be explicitly excluded with a grounded reason. Tests must cover both signed source fallback and unsigned source-row refusal.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:49-55 extends resolve-cap, claim-launch, and RefuseUnsignedMissionCap but not mission fence authorization. metasystem/scripts/agents/dispatch.sh:1565-1574 sends only model_key to the mission fence. metasystem/internal/mission/fence.go:482-490 checks the canonical pair and then fence.job-cap-min.
MATERIAL: True

## FMA-R2-VALIDATOR-ALLOWANCES-UNTESTED — medium
CLAIM: FMA-R1-VALIDATOR-TEST-MATRIX is improved but not fully closed because two required allowed shapes have no discriminating positive test. The validator may accept direct fan-in, where two source identifiers share one target, and it must accept a machine-local maximal-models value containing both target and draining source. Section 5 tests only chain rejection and omission of the target. An implementer could reject duplicate targets or every local maximal-models overlay and still pass every named test, breaking the stated 5.2 rollover shape or R-46-m0b live-round safety. Add explicit acceptance fixtures for direct fan-in and for a dual local target-plus-source list.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:43 defines only a target-that-is-also-a-source as a chain, while :46-47 refuses a local overlay only when it omits the target. The tests at :69-73 contain rejection cases but no positive direct-fan-in or dual-overlay case. metasystem/plans/fable-5-1-rollover-design.md:35-48 requires the temporary dual local value to remain valid.
MATERIAL: True

## FMA-R2-CLAUDE-GATE-FIXTURE-OMITTED — medium
CLAIM: The promised Claude-runtime maximal-gate test was dropped from the actual design. The round-1 register says the fake-runtime short circuit is covered by a Go composition test on a Claude-runtime temporary root, but revision 2 section 5 names only roster tests, validator tests, cap tests, fake dispatcher fixtures, and a Claude command-builder test. Because the fake runtime returns true before reading maximal-models, every named dispatcher fixture can pass without exercising runtimeProvesMaximalExecution for a resolved Claude alias. The missing named composition fixture must be restored.
EVIDENCE: metasystem/records/misc/fable-model-alias-critique-r1.md:80 records the accepted Claude-runtime composition test. metasystem/internal/dispatch/hazard.go:109-116 shows the fake short circuit. metasystem/plans/fable-model-alias-design.md:66-87 contains no proposed Claude-runtime alias composition test.
MATERIAL: True

## FMA-R2-EFFECTIVE-MODEL-OBSERVATION-BYPASS — high
CLAIM: FMA-R1-END-TO-END-PROOF remains open for effectiveModel because Claude observations bypass both alias application points. Revision 2 says effectiveModel carries only the canonical identifier, but the Claude adapter writes the startup signal's model and later overwrites it from the result's modelUsage key without resolving aliases. The fake dispatcher fixture merely echoes requestedModel, and the command-builder test covers only outgoing argv. Thus all named tests can pass while a Claude result reporting claude-fable-5 writes that literal into effectiveModel. The design must define canonicalization of observed model identifiers, preserving the raw observation separately if required, and add a canned Claude-result fixture that supplies the alias source.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:63-64 claims effectiveModel is canonical and :76-85 relies on fake echo plus an argv unit test. metasystem/scripts/agents/adapters/claude.sh:153-177 accepts the signalled and result models; metasystem/scripts/agents/adapters/runtime-common.sh:158-168 writes the result model directly; and metasystem/internal/adapter/claude.go:116-125 returns raw modelUsage keys.
MATERIAL: True

## FMA-R2-PROVENANCE-SHAPE-CLOSED — low
CLAIM: FMA-R1-ALIAS-PROVENANCE-SHAPE is closed at the schema level. Revision 2 separates effective-input provenance as aliasedFrom from literal-roster provenance as rosterAliasedFrom, defines all combined override cases, makes both immutable, and correctly gives follow-ups no roster provenance. The separate missing canonical follow-up input is covered by FMA-R2-FOLLOWUP-CANONICAL-RELAY.
EVIDENCE: metasystem/plans/fable-model-alias-design.md:57-64 defines both fields, the combined cases, immutability, and canonical record identifiers.
MATERIAL: False

## FMA-R2-FOLLOWUP-SUBVERB-OWNER-SOUND — low
CLAIM: The internal resolve-model-alias subverb is the appropriate owner after the recorded narrowing from no new CLI verb to no new operator verb. Follow-ups need the canonical value before composition; resolve-cap and claim-launch are later, build-follow-record is later still, and resolve-roster would incorrectly reread the current roster. A thin internal relay of the shared configuration helper is therefore smaller than overloading an existing downstream verb. This does not cure the missing relay into BuildFollowRecord.
EVIDENCE: metasystem/records/misc/fable-model-alias-critique-r1.md:76 records the seat's accepted narrowing. metasystem/scripts/agents/dispatch.sh:1762 reads the inherited model, while composition starts at :1900, cap resolution at :1920, claim-launch at :1932, and record construction at :2005.
MATERIAL: False

## Gaps reported by the critic

- The read-only constraint prohibited running the dispatcher or Claude command-line interface. The repository proves that arbitrary observed Claude model strings can enter effectiveModel, but the provider's actual spelling for a claude-fable-5-1 invocation remains unobserved.
- The benchmark-kit clone named as a consumer of cap-resolution rule strings is absent from this workspace. Repository code proves that the proposed strings are additive and preserve the record shape, but compatibility with an external closed enumeration cannot be independently verified here.

## Dispositions (m3 seat, dispatch delegate)

- FMA-R2-FOLLOWUP-CANONICAL-RELAY: ACCEPT. build-follow-record gains --model beside --aliased-from; BuildFollowRecord takes the canonical model as a parameter instead of copying the parent's; the follow-up fixture starts from a parent record whose requestedModel is literally the source (a canned legacy record), not from a freshly aliased one.
- FMA-R2-TARGET-ADMISSION-ENV-SHADOW: ACCEPT. The environment maximal-models value is treated like the machine-local overlay: named when it omits the alias target, allowed when it lists target and draining source; one validator case each.
- FMA-R2-MISSION-CAP-SOURCE-BYPASS: ACCEPT, symmetric. The seat first reached for an exclusion (signed authority is exact-pair) and withdrew it on reading the shell: the signed envelope is consulted only when an escalation is required, so a mission dispatch whose roster names the source pair never meets it and its signed source cap row would fall to the general mission cap. So the alias source is passed through the mission fence too (authorize-cap gains --alias-source; mission.AuthorizeCap orders canonical pair, source pair, general), and RefuseUnsignedMissionCap inspects the same two keys. Tests: a signed source-pair row of 30 gives 30, not the general mission cap; an unsigned mission with only a source row refuses.
- FMA-R2-VALIDATOR-ALLOWANCES-UNTESTED: ACCEPT. Two positive fixtures: direct fan-in (two sources, one target) accepted; a machine-local or environment maximal-models value listing target plus draining source accepted.
- FMA-R2-CLAUDE-GATE-FIXTURE-OMITTED: ACCEPT. The Go composition test on a claude-runtime temp root (alias line plus tracked maximal-models on the target, roster on the source) is restored by name: ResolveRoster then ValidateRuntimeHazardConfiguration passes, and the same root without the alias line refuses.
- FMA-R2-EFFECTIVE-MODEL-OBSERVATION-BYPASS: REFUTE the canonicalization, ACCEPT the claim correction. effectiveModel is an OBSERVATION: what the runtime said it ran. The alias applies to requests, never to observations; rewriting an observed id would hide a real fact (the CLI ran something other than what was asked). Section 4's claim that effectiveModel carries only the canonical id is withdrawn: requestedModel and canonicalModelKey are canonical by construction, effectiveModel is raw, and closure and the sweep read requestedModel and the provenance fields. Test: a canned Claude result reporting the source literal writes it into effectiveModel unchanged while requestedModel stays the target.
- FMA-R2-READ-ORIGIN-CLOSED, FMA-R2-CONFIG-CAP-FALLBACK-CLOSED, FMA-R2-PROVENANCE-SHAPE-CLOSED, FMA-R2-FOLLOWUP-SUBVERB-OWNER-SOUND: closures, no change.
