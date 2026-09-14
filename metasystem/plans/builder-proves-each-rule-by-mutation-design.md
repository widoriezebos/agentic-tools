# builder-proves-each-rule-by-mutation

- Owner: m1e (goal 29 of plans/delivery-efficiency-plan.md). **Revision 3, 2026-09-14**, the final prose fold, by the Codex gpt-6-astra design delegate from critique round 2 (records/misc/builder-proves-each-rule-by-mutation-critique-r2.md, five material findings, all accepted; the five round-1 findings it kept open are folded here) under the seat's rulings and bound to the sibling's revision 3; the seat integrated the page and edits only this header block.
- Goal and current status: the builder returns one observed mutation claim per acceptance row declared in the brief's Rules header; review joins the chain's declarations to valid claims with explicit time semantics and checks their named tests in the final candidate; the critic reproduces a deterministic sample. Eleven builder units of at most 400 changed lines each, in this order after the sibling's 1a, 3a, 3b and 3c: U0, U3a, U3b, U1, U2, U4a, U4j, U5a, U5b, U5c, U4b. Nothing built yet.
- In flight right now: the two-round review budget is spent (seven material findings in round 1, five in round 2, all folded). Under R-97-m1e and D81 this page is the implementation spec; every standing finding is a named test or fixture leg in the unit that owns its rule, and each unit's independent code read carries the check from here. U0 waits for the sibling's admission units.
- Decisions made (and who made them): Wido, 2026-09-14: the mutation battery moves from the read to the builder's return; units are at most 400 changed lines. The seat, on round 1: the sibling's header grammar and reader are reused, review joins the whole chain, one row per removable rule, the named test lives in the rule's package or script, a deterministic critic sample, the U1 bootstrap named honestly. The seat, on round 2: this goal owns U0, which extends the sibling's admission parser, persisted record and review reader with Rules; the location pattern uses Go alternation; the join has explicit time semantics (an entry is valid only for an id declared at or before its round, a repair re-declares and re-proves its ids); the benchmark kit rules are rows with witnesses; and the bootstrap is stated truthfully: for U0, U3a, U3b, U1, U2, U4a, U4j, U5a, U5b and U5c the seat performs the row join by hand before landing and records it in that unit's landing message, and DONE is met when U4b lands. The seat, on the budget: no third prose round.
- Waiting on the human: nothing. The bootstrap sentence is carried into the goal record's Next step in Wido's name by the seat.
- Dead ends (do not retry without new evidence): comma-separated headers; create prefixes inside Boundary; units on Ceiling; Rules recovered from delivered or legacy prose; a separate Rules snapshot; future declarations validating past unknown claims; one diagnostic standing for several removals; fixture registration as behavioural proof; a location pattern with an escaped bar.
- Next step: revision 3 lands with this line as the implementation spec; once the sibling's units 1a, 3a, 3b and 3c are landed, U0 (extend the admitted bounds with Rules) is briefed to Codex gpt-5.6-sol with its own Boundary, Ceiling, Rules and proof rule, read by Opus, landed by the seat with the row join recorded by hand, and U3a, U3b, U1, U2, U4a, U4j, U5a, U5b, U5c and U4b follow in that order, each with its own read.

The DONE contract is `metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:8`. The approved goal has a two-round review budget (`metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:3`, `metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:14`). The seat carries this exact sentence into Next step, through the goal owner:

> Wido: For U0, U3a, U3b, U1, U2, U4a, U4j, U5a, U5b and U5c, the seat performs the row join by hand before landing and records it in that unit's landing message; DONE is met when the enforcing unit U4b lands with the full mutation check and all four behavioral fixtures proved.

This is a staged bootstrap, not an assertion that those earlier returns passed mutation conformance. U4b must pass its candidate engine's full public review check before it lands. No implementation unit follows U4b in this goal. The seat owns the goal-record update; this delegate changes no tracked record.

Evidence level: read. Current-code facts were re-checked in this worktree against `74fc3a27f2d45e44c67850bb12c503313254ab0d`. The binding sibling input is `artifacts/reports/sibling-bdrb-design-r3.md`, not the trunk plan. Its proposed APIs are dependencies, not shipped facts. All paths below are repository-relative. Commands run from `metasystem/` unless another directory is stated. New APIs, tests and diagnostics are proposed contracts. Changed-line allocations are estimates, not measured patches.

## Facts that determine the design

| Current behavior | Evidence and consequence |
| --- | --- |
| The shipped implementer schema is frozen version 1 and closes its root object. | `metasystem/scripts/agents/schemas/implementer.schema.json:3-8`. Leave that file unchanged. |
| Version 2 derives identity fields from version 1. Materialization and completeness each invoke the derivation. | `metasystem/internal/returnschema/returnschema.go:34-74`, `metasystem/internal/returnschema/returnschema.go:154-187`; `metasystem/internal/validate/returncomplete.go:161-214`. Both callers need the implementer overlay. |
| Launch materializes implementer version 2 before the builder runs. | `metasystem/scripts/agents/adapters/runtime-common.sh:103-110`. U0, U3a, U3b and U1 therefore use the launch-compatible evidence table. |
| The provider-schema test requires every object to be closed and every property to be required. | `metasystem/internal/returnschema/returnschema_test.go:176-265`. Provider output requires `mutations`, including an empty array. |
| The schema validator supports type, enum, const, properties, required, additionalProperties, items, and pattern. | `metasystem/internal/validate/returncomplete.go:419-424`. Use this subset. Add no schema engine or dependency. |
| The brief parser currently owns mode and authority paths. Its create prefix only marks output references. | `metasystem/internal/dispatch/brief.go:41-104`, `metasystem/internal/dispatch/brief.go:155-167`. Use the sibling's new header family; do not reinterpret create syntax as Boundary. |
| Composition selects raw caller brief bytes as task-direction, then appends recipe sources and continuations. | `metasystem/internal/dispatch/composition.go:237-283`, `metasystem/internal/dispatch/composition.go:300-319`. This is delivered task direction. Rules must instead come from admitted bytes through U0. |
| Composition records source ranges, original byte counts, and both original and delivered digests. Oversized bodies become references. | `metasystem/internal/dispatch/composition.go:52-69`, `metasystem/internal/dispatch/composition.go:366-401`. The sibling authenticates this packet separately from its retained admission record. |
| Root and follow-up publication retain prompt and composition artifacts. Root publication also retains brief.md. | `metasystem/scripts/agents/dispatch.sh:1918-1929`, `metasystem/scripts/agents/dispatch.sh:2883-2888`. The sibling adds admitted-brief.md and brief-bounds.json beside them; U0 extends that record. |
| The exhaustion reader returns the entire prompt when there is no task-direction reference. | `metasystem/internal/validate/conformance.go:930-960`. Do not reuse it for Rules. |
| Review resolves the implementer without requiring a critic, snapshots the candidate, checks boundaries, and writes success artifacts. | `metasystem/internal/validate/conformance.go:139-249`, `metasystem/internal/validate/conformance.go:355-428`. Mutation enforcement belongs before those writes. |
| Cumulative boundary inspection raises its round limit from chain job records, then reads numeric round directories and unions returns. | `metasystem/internal/validate/conformance.go:724-817`. It is not limited to the supplied round number. Match this chain convention explicitly. |
| Snapshot uses an isolated index, read-tree, and git add -A. Literal tree entries and blob reads are already available. | `metasystem/internal/gittree/gittree.go:246-276`, `metasystem/internal/gittree/gittree.go:450-499`. Inspect the supplied reviewed tree, including uncommitted and unignored new files. |
| Nested project declarations strip the repository installation prefix exactly once. Delegate changes outside the project refuse. | `metasystem/internal/validate/conformance.go:276-293`, `metasystem/internal/validate/conformance.go:311-334`. Reuse projection; keep benchmark integration at the seat. |
| Acceptance normalizes the selected object and can rewrite a lawful boundary. Both repair prompts are separate owners. | `metasystem/internal/adapter/return.go:37-94`, `metasystem/internal/adapter/return.go:191-225`; `metasystem/internal/validate/returncomplete.go:239-303`; `metasystem/internal/adapter/adjudicate.go:65-97`. Preserve evidence and prohibit invented observations in both prompts. |
| Requirements use capability names, while packet recipes can deliver a file source. | `metasystem/internal/capability/select.go:110-125`; `metasystem/scripts/agents/roles/implementer.requirements.json:2-15`; `metasystem/internal/dispatch/composition.go:275-283`. Add a proof string, not a capability named mutations. |

## Return member and acceptance rows

The new top-level member has this shape. The failure below is an illustration, not an observation.

```json
{"mutations":[{"ruleId":"R1","location":"metasystem/internal/validate/conformance_mutations.go#mutationViolations","test":{"kind":"go","path":"metasystem/internal/validate/conformance_mutations_test.go","name":"TestConformanceMutationMissingRow"},"failureLine":"conformance_mutations_test.go:91: missing R1 was admitted"}]}
```

`mutations` is an array. Each item is an object with exactly `ruleId`, `location`, `test`, and `failureLine`. `test` is an object with exactly `kind`, `path`, and `name`. Every nested member is required. Every object is closed. Every scalar is a string. Empty arrays are lawful. Null is not an array or an object.

Use the following patterns as schema regexes, with ordinary JSON escaping when serialized:

| Member | Constraint |
| --- | --- |
| ruleId | `^[A-Za-z][A-Za-z0-9_.-]*$` |
| test.kind | Enum `go`, `fixture`. |
| test.path | `^[^\r\n]*\S[^\r\n]*$` |
| test.name | `^[^\r\n]*\S[^\r\n]*$` |
| failureLine | `^[^\r\n]*\S[^\r\n]*$` |

The location pattern is written once here. Use Go alternation, with the unescaped bar between its two suffix forms. The literal pipe is excluded from this claim syntax. Ordinary JSON serialization supplies the escaping in materialized schemas.

```regex
^[^|\r\n]+(:[1-9][0-9]*|#[^|\r\n\s][^|\r\n]*)$
```

`TestMutationLocationForms` in U2 accepts `rule.go:12` and `rule.go#check`. It rejects the former misspelling `rule.go:12|#check`, zero lines, empty symbols and line breaks. Run those cases through the materialized fragment and canonical acceptance. Escaping the alternation bar must break both legal-form assertions. Removing the pattern must break a negative assertion. The schema validator uses Go regexp (`metasystem/internal/validate/returncomplete.go:474-479`, `metasystem/internal/validate/returncomplete.go:525-527`).

The location names a repository-relative file and either a positive one-based line in the restored candidate or a symbol after `#`. Split at the final `#` when present, otherwise at the final colon introducing decimal digits. Resolve its path at review as specified below. The critic checks the symbol or line and the claimed removal. A compile error, timeout, skip, or setup error is not an observed failure of the rule.

Keep the existing evidence object `{command, observed, level}`. Its owner is `metasystem/scripts/agents/schemas/implementer.schema.json:22-33`; the template explicitly preserves it at `metasystem/scripts/agents/templates/brief.md:34`. Record the passing baseline, isolated removal, failing run, restoration, and restored pass there. Each mutation entry contains one actual diagnostic for that row alone. Several rows may name one enclosing Go test, with separate subtests and separate removal runs. There is no representative diagnostic for a group of constraints.

Add `MutationTest`, `MutationEntry`, `MutationEntriesSchema`, `ImplementerVersionTwo`, and `CanonicalImplementerVersionTwo` in `metasystem/internal/returnschema/implementer.go`. The two structs mirror the JSON member names. `MutationEntriesSchema() map[string]any` owns the one fragment. Both overlays take and return `map[string]any` with an error, like VersionTwo. The provider overlay calls VersionTwo, installs the fragment, and requires mutations. The canonical overlay installs the identical fragment but omits mutations from the root required list.

`Materialize` selects the provider overlay only for implementer version 2. `returnChecker.checkReturn` selects the canonical overlay only for implementer version 2. Old omitted members pass without insertion. Present malformed members fail. Version 1 and other roles keep their contracts. Review, rather than acceptance, supplies the brief-dependent coverage check.

Expose the canonical form through `schema materialize --canonical`, accepted only with `--role implementer --version 2`. Other combinations exit 2 with `--canonical is only available for implementer version 2`. Add `MaterializeCanonicalImplementer(root, outputPath string) error` and share the existing encoding body between materializers. The CLI owner is `metasystem/cmd/metasystem/schema.go:13-46`.

The benchmark pins default materialized version 2 today, then validates historical returns against it (`benchmark/validate-kit.sh:84-102`; `benchmark/extractor.py:324-341`, `benchmark/extractor.py:514-538`). In U1 the seat regenerates `benchmark/schemas/evidence/implementer.schema.json` with the canonical materializer and changes only the implementer branch of the drift command and its regeneration hint. The mutations member remains optional at the pin's root. Historical omission is required by `benchmark/extractor-fixtures.sh:260-274`. The extractor already supports optional declared properties and patterns (`benchmark/extractor.py:63-118`), so it needs no code change. Regenerate the pin again when U2 adds text constraints. No benchmark scoring change is part of this goal.

The builder supplies the benchmark changes and their Go tests in `metasystem/artifacts/reports/bprm-kit-compat.patch`. The seat applies that patch with the matching engine change. Its exact allowed integration paths are `benchmark/validate-kit.sh`, `benchmark/schemas/evidence/implementer.schema.json` and the new `benchmark/bprm_mutation_test.go`. These are the seat's permitted integration targets. The dispatched Boundary contains only project paths, including the patch artifact. The sibling parser would refuse a benchmark path in a nested-project Boundary (`artifacts/reports/sibling-bdrb-design-r3.md:65-73`). The delegate's working tree only gains the patch artifact for this part. The seat supplies a disposable repository copy for the builder to apply and test that exact patch. It is evidence work, not an outside-project engine patch. The current project fence remains (`metasystem/internal/validate/conformance.go:311-334`).

The benchmark rows use the actual benchmark location and a named colocated Go test. The seat joins them against the full repository integration candidate during U1 and U2's manual window. They are not projected into the nested engine or renamed as an engine rule to evade association. Each unit is a fresh chain, so these pre-enforcement integration returns are not replayed through a later unrelated unit's gate. Generated pin lines and the new Go test count toward the 400-line combined ceiling. The patch transport is not counted a second time as shipped source. No benchmark scoring, extractor implementation, or engine dependency on the kit changes. The kit's dependency direction is stated at `benchmark/validate-kit.sh:2-6`.

`benchmark/bprm_mutation_test.go` contains four named Go tests. TestBPRMKitCanonicalCommand executes the derived-role loop extracted verbatim from validate-kit.sh in a disposable kit, using a command recorder at its engine path. It asserts that the implementer invocation includes --canonical and that its failure hint includes the canonical regeneration command. TestBPRMKitOtherRoles checks each other derived role still uses the ordinary v2 invocation. TestBPRMKitPin runs the newly built engine's canonical materializer and byte-compares its output with the applied pin. TestBPRMKitHistoricalOmission calls the existing extractor.schema_violations through Python against the applied pin and the historical implementer value from `benchmark/extractor-fixtures.sh:260-274`; it expects no violations and unchanged input. The production validator is at `benchmark/extractor.py:63-118`. All test logic and assertions are in Go. Do not add a Python test file or run a fixture bed in the builder.

For a kit removal, the builder changes only the corresponding hunk in the allowed patch artifact, applies that variant in the seat's disposable copy, and runs its named test. It restores the exact complete patch and proves the test passes there again. The returned location names the applied benchmark predicate and the evidence identifies the transport hunk too. A syntax check cannot stand in for these witnesses. From the disposable repository root, the focused test command is `go test -count=1 benchmark/bprm_mutation_test.go -run '^TestBPRMKit'`. The tests build the engine from that copy into t.TempDir with Go, preserving the supplied cache environment. They do not export METASYSTEM_BIN. The seat later runs the actual kit and extractor beds as specified under Seat verification.

### Shared header grammar and current-round source

Every unit has one physical line per header. Boundary and Ceiling retain the sibling's paired grammar. Rules is independent of that pair:

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance_mutations.go"]
Ceiling: 400
Rules: ["R1","R2"]
```

U0 belongs to this goal. Its direct sibling prerequisites are revision-3 **unit 1a** for the admission parser and in-memory value, **unit 3a** for the record codec and composition marker, **unit 3b** for ReadRoundBriefBounds, and **unit 3c** for admission persistence and publication. These owners are specified at `artifacts/reports/sibling-bdrb-design-r3.md:317-327`, `artifacts/reports/sibling-bdrb-design-r3.md:353-391`. Their own prerequisite order remains the sibling's. U0 requires no sibling Rules API and no reader of admitted prose. U4b separately requires the sibling's **unit 4c** for policy coexistence. No dependency is assigned to an undefined sibling unit 1, 3 or 4.

Extend dispatch.BriefBounds with `Rules []string`. Leave its Boundary and Ceiling fields and paired semantics intact. Absence of Rules yields nil in memory; `[]` yields a non-nil empty slice. Both declare no rows. Extend the sibling's existing single header scan inside ParseBriefBounds. Recognize exactly `Rules:` at column zero, including inside a fence. Indented and quoted examples are inert. Trim the raw value, never an id. Reject duplicate headers, blank values, null, non-arrays, non-string members, trailing JSON data, repeated ids and ids outside the ruleId pattern above. Preserve order. Check duplicate Boundary, Ceiling and Rules headers in that order, then the sibling's pairing and value checks, then Rules. Rules does not force an installation-prefix lookup when the bounds pair is absent. Skip live Rules and space/tab-indented Rules example lines during authority extraction, just as the sibling handles its header examples.

U0 adds exactly these functions:

```go
// internal/dispatch/brief.go, a value decoder called from the existing scan:
func ParseBriefRules(value string) ([]string, error)
func validateBriefRuleIDs(ids []string) error

// internal/validate/brief_bounds_source.go:
func ReadRoundBriefRules(root, rootJob, job, roundText string,
    jobComposition map[string]any) ([]string, error)
```

ParseBriefBounds(data []byte, installPrefix string) keeps its sibling signature. It calls ParseBriefRules for the one live value already selected by its scan. ParseBriefRules calls validateBriefRuleIDs after JSON decoding; the record codec calls that same id validator on decoded structured data. The sibling's byte-based admission helper still calls ParseBriefBounds once and gives the same admitted value and bytes to authority extraction and the writer. U0 extends these existing bodies, not their undisclosed internal function names. It does not add a second file-reading admission wrapper.

ReadRoundBriefRules calls the sibling's exact `ReadRoundBriefBounds(root, rootJob, job, roundText, jobComposition) (dispatch.BriefBounds, error)` once. It returns that result's Rules field and propagates its error. The sibling reader returns the extended BriefBounds from its validated structured record. It owns all identity, marker, digest, range, reference, regular-file and orphan-file checks. U0 changes neither its signature nor its bounds wrapper. There is no call to successorTaskDirection, no latest-round selection in this helper, and no new composition source member. The sibling's reader contract is `artifacts/reports/sibling-bdrb-design-r3.md:137-147`.

Extend the same `brief-bounds.json` codec with optional `rules`. Keep schemaVersion 1 and every existing required field. New admissions always write rules as an array, using [] for absent Rules. Old sibling records that omit rules remain lawful and declare no rows. Present null, non-array, malformed members, duplicate keys or ids, and invalid ids are unreadable evidence. Unknown keys remain forbidden. The new member does not affect the bounded boolean, which still describes only the Boundary/Ceiling pair. The paired null record can carry nonempty Rules.

The same record stays beside the round at `artifacts/agents/<rootJob>/rounds/<round>/brief-bounds.json`. Its admittedSha256 and admittedBytes still name the exact `admitted-brief.md`, including the Rules line. The sibling's admittedBrief.recordSha256 now covers the extended record bytes too. No rules digest, sidecar or second brief snapshot is added. Keep the writer's admission-time input and initial/follow-up publication sites from the sibling's unit 3c (`artifacts/reports/sibling-bdrb-design-r3.md:95-135`). Delivered caller:brief can be augmented after admission in current dispatch (`metasystem/scripts/agents/dispatch.sh:1655`, `metasystem/scripts/agents/dispatch.sh:1736-1742`, `metasystem/scripts/agents/dispatch.sh:2643`, `metasystem/scripts/agents/dispatch.sh:2697-2709`). It cannot override persisted Rules.

Admission syntax errors use `BRIEF_BOUNDS_INVALID: Rules: <detail>` through the existing sibling refusal type. Use `header occurs more than once`, `expected a JSON array of unique rule ids`, `repeated rule id <quoted id>` and `invalid rule id <quoted id>`. Review wraps a sibling read failure with MUTATION_BRIEF_INVALID and its round. The original BRIEF_BOUNDS_UNREADABLE detail stays visible. Do not fall back on any malformed or mismatching present evidence.

The legacy fallback is exactly the sibling's revision 3. With neither marker nor admitted files, validate any composition evidence that is present, then return no bounds and no Rules. An old record with no rules also declares no rows. Never parse root brief.md, a follow-up prompt, inline or referenced task direction, or the retained admitted copy for Rules during review. Thus a legacy follow-up whose Rules cannot be read declares no rows. Earlier nonlegacy declarations survive through the intentional chain union. The exhaustion reader retains its separate prompt fallback (`metasystem/internal/validate/conformance.go:925-954`).

An acceptance row is a Rules id with a matching prose row that names one independently removable rule and its severity. Use critical, high, medium or low. Missing severity ranks high for sampling. Severity is not a return member. A repair re-declares each repaired id and returns a fresh observation for each. Static conformance cannot infer an unmentioned code repair; the unit's code read must check that all changed rules were declared.

## Review check and test lookup

### Exact chain join

Add conformance_mutations.go in validate. `func (r *conformanceRun) mutationViolations(reviewedTree string) []string` owns the join. U4a collects source facts, U4j owns the temporal join, U5c attaches lookup, and U4b activates the complete check in reviewStage.

Use the resolved chain root and its numeric round directories. Match the current cumulative boundary limit: start at the supplied round and raise it to the highest recorded round in that chain (`metasystem/internal/validate/conformance.go:724-768`). Extract directory selection into one local helper. Preserve the boundary caller's ordering and error handling; mutation collection sorts a copy numerically and refuses directory read errors. Collect the job id and embedded composition for each selected round from the already loaded chain records. Pass that round's identity to ReadRoundBriefRules. Never pass the supplied job's composition for a different round. A marked round without a matching job record is unreadable evidence. An unmarked legacy round can pass nil composition to the sibling reader. Do not add another job-file read inside ReadRoundBriefRules.

Keep source-bearing rounds even if return.json is absent. A missing return contributes no entries. It does not erase the declarations. Retain round and array index for each entry. The current boundary loop's return-only aggregation is at `metasystem/internal/validate/conformance.go:768-805`; mutation collection must retain the extra relationship rather than flattening it away.

Let S(r) be the Rules read for round r. Let E(r) be its mutations, or empty for an absent return or member. Let D(r) be the union of S through r, and D the final union. Let q be the highest selected round. A valid entry is shape-valid, unique within its own return, declared by its entry round, and compliant with the re-declaration rule below. Candidate coverage additionally requires its named test to exist and its location to associate correctly. The exact join is numbered so each clause has its own named witness:

1. **Temporal admission.** An entry in round r is eligible only if its id is in D(r). It can never borrow a future declaration. TestMutationDeclaredByEntryRound proves same-round admission and rejects use of a future declaration when the entry is read.
2. **Unknown remains unknown.** An entry for an id first declared later is rejected as unknown at its original round. It never supplies coverage after that later declaration. TestMutationFutureDeclarationDoesNotReviveEntry uses an unknown R9 in round 1 and first declaration in round 2 with no new entry. Final coverage still lacks R9.
3. **Repair re-declaration.** Any later entry for an already declared id is a repair claim and must name that id in S(r). Otherwise reject it with MUTATION_REDECLARE. A builder must likewise re-declare every rule it repairs in code. TestMutationRepairRedeclaresID returns R1 in round 2 with only R2 in S(2) and observes the refusal. This check uses S(r), not final D.
4. **Repair freshness.** Repeating an id in S(r) requests a fresh valid entry for that id in E(r). An older valid entry cannot discharge that request. TestMutationRepairNeedsFreshEntry declares R1 again while returning no fresh R1 and observes refusal. A later round can repair this failed attempt by re-declaring R1 and providing a fresh entry. The current unmet freshness check is at each id's latest declaring round; historical missing attempts are not permanent poison. TestMutationLaterFreshRepairClosesAttempt proves that separate recovery rule. No successful old entry is removed.
5. **Final coverage.** C is the set of ids in D with at least one valid entry whose named test exists and associates in the final reviewedTree. Require C = D, plus the latest-declaration freshness requirement. An empty current S cannot erase D. TestMutationFinalTreeCoverage covers R1 and R2 from different rounds and refuses a missing earlier row. Report the earliest declaration for a wholly missing id; name the repair round for a freshness failure.
6. **Every valid claim remains checkable.** Inspect every valid entry in the final tree, even if another valid entry covers the same id. TestConformanceMutationEarlierTestDeleted supplies a newer R1 test but deletes the older valid R1 test; review still names the deleted test. Coverage alone is not enough to admit the candidate.
7. **Within-return uniqueness.** Duplicate ids invalidate all entries for that id in that return. TestMutationDuplicateEntry removes this guard. Repetition across rounds is lawful with clauses 3 and 4 satisfied. TestMutationRepeatedRuleAcrossRounds owns that distinct positive rule.
8. **Rejected claims can be repaired.** Unknown, undeclared repair and duplicate claims remain rejected with their original provenance. At q they cause their own refusal. In earlier rounds they give no coverage, but a later re-declared, unique valid entry can repair the deficit. TestMutationRejectedClaimRepair proves recovery and that the invalid old claim stays out of the lookup set. Do not apply this recovery to malformed JSON or entry shape.
9. **Shape before decoding.** Validate every present nonempty-chain mutations member with the shared MutationEntriesSchema before typed decoding. Present nonobject, unreadable or malformed returns refuse. They are never silently replaced by a later good entry. TestMutationMalformedEntry owns fragment validation; TestMutationReturnObject and TestMutationReturnReadFailure own the two independent return guards.
10. **No-row compatibility.** When D is empty, skip mutation return reading and lookup. Keep ordinary completeness and conformance. TestConformanceMutationNoRowsUnchanged proves headerless and explicit-empty cases preserve old return bytes. This exemption does not erase earlier declarations in a nonempty chain.

Source and shape errors are emitted in numeric round order. Missing ids follow earliest declaration and array order. Freshness failures name the latest declaration. Entry failures follow round and entry index. These rules are one join, not two competing uses of final D. The critic samples its valid entries and keeps rejected entries visible as rejected evidence.

In reviewStage, append mutation and sibling bounds policy failures before success writes. One policy must not mask another. On first refusal write neither success artifact. On a changed repeat retain earlier artifacts and report both the mutation refusal and the existing immutable-review refusal. Identical successful repeats reuse their bytes. Current persistence and repeat handling are at `metasystem/internal/validate/conformance.go:339-425`. Keep the three-field review.json shape (`metasystem/internal/validate/conformance.go:403-407`). Do not normalize or rewrite any return here.

Use these stable diagnostics, with quoted ids, paths and test names:

```text
conformance failure: MUTATION_MISSING: acceptance row "R1" declared in round 1 has no mutation entry in the chain
conformance failure: MUTATION_MISSING: repair row "R1" declared in round 2 has no fresh mutation entry in that round
conformance failure: MUTATION_REDECLARE: round 2 mutation entry for acceptance row "R1" requires Rules to re-declare that id
conformance failure: MUTATION_TEST_ABSENT: round 1 acceptance row "R1" names absent go test "TestMissing" in "example_test.go"
conformance failure: MUTATION_LOCATION_MISMATCH: round 1 acceptance row "R1" test "TestPresent" in "other/example_test.go" does not belong with location "rule.go#check"
conformance failure: MUTATION_DUPLICATE: round 2 acceptance row "R1" has more than one mutation entry in that return
conformance failure: MUTATION_UNKNOWN: round 1 mutation entry names acceptance row "R9" not declared at or before that round
conformance failure: MUTATION_INVALID: round 1 $.mutations[0].failureLine is required
conformance failure: MUTATION_BRIEF_INVALID: round 1 <shared reader diagnostic>
conformance failure: MUTATION_TEST_UNREADABLE: round 1 acceptance row "R1" cannot inspect test "TestPresent" in "example_test.go": <cause>
```

Use MUTATION_INVALID for a nonobject or unreadable present return, with its cause. Required-field detail reuses `metasystem/internal/validate/returncomplete.go:531-538`. Policy refusal exits 1; CLI usage remains exit 2 (`metasystem/cmd/metasystem/validate_verbs.go:276-299`, `metasystem/cmd/metasystem/validate_verbs.go:320-352`).

### Candidate lookup and location association

Add `mutation_test_lookup.go` and `mutation_go_lookup.go` in validate. Pass the full entry and the already computed reviewedTree. Add no snapshot, diff generator, test runner, compiler invocation, or execution of a returned command to conformance.

Both the location file and test path must be clean repository-relative slash paths. Reject an absolute path, NUL, backslash, line break, literal pipe, empty component, `.` component, or `..` component. Spaces are allowed. Apply projectDeclaration exactly once to each path. Read their exact entries from projectWorkspace using Entries and FileAt. Accept mode 100644 or 100755 only. A missing exact file, directory, symlink, or gitlink cannot satisfy the lookup. An absent test gets MUTATION_TEST_ABSENT; an absent or unrelated location gets MUTATION_LOCATION_MISMATCH. Git or source parsing errors get MUTATION_TEST_UNREADABLE. Never consult controller files, HEAD, or live workspace contents after the snapshot.

For a Go witness, the location file and test file must be in the same directory and thus the same package directory. When the location is Go, parse its package clause too; permit the test package to be that package or its conventional external `_test` package. Reject a different directory even if its package clause uses the same name. For a colocated JSON or instruction resource, the test's directory is its package ownership boundary. A small test-only package beside that resource is lawful. There is no cross-directory owner registry and no exemption for instruction files. These tests check instruction delivery or wording, not model obedience.

The test path must end in `_test.go`. Parse Go declarations with go/parser. Require an exact top-level function name, no receiver, no type parameters, no results, and exactly one non-variadic parameter of type `*testing.T`. Resolve the import of `testing`, including ordinary, alias, and dot imports. Require `Test` followed by the end of the name or a non-lowercase Unicode rune. A `TestMain(*testing.T)` satisfies this contract; `TestMain(*testing.M)` does not. Comments, strings, methods, helpers, benchmarks, and a subtest name alone do not count. Return the enclosing function name, and record the exact subtest command in evidence.

Source lookup is independent of build tags and operating-system suffixes. That is a membership result only. The builder must prove actual execution in a stated compatible environment. The critic must use that environment for a sampled row. Compilation without execution or a skipped test does not reproduce the claim. An unavailable environment leaves a material unproven finding; it never permits substituting a convenient sample row.

For a fixture witness, require a `.sh` test path identical to the location path. Its test.name is a literal leg name in that script. Support the two source forms already used by the beds (`metasystem/scripts/agents/conformance-fixtures.sh:175`; `metasystem/scripts/agents/return-schema-fixtures.sh:132`):

```sh
new_case bprm-missing-row
if [[ "$fixture_scenario" == implementer-v1-v2 ]]; then
```

Match the whole selector line after horizontal whitespace. Names contain only letters, digits, underscore, dot, and hyphen. Permit unquoted, single-quoted, or double-quoted literal names and an optional trailing comment. Do not accept a comment line, substring, negative comparison, or variable-expanded name. This remains a source-membership check, not a shell interpreter. Selector-looking heredoc or quoted data can imitate a source line; it does not establish a runnable leg. The critic must prove entry into the claimed leg inside that exact script and its rule-specific failure. A data-only selector, unreachable leg, missing selector dispatch, or failure from another leg is material non-reproduction. Do not claim regex matching excludes shell data, and do not add a shell parser dependency to hide that limit.

Schema and package association reduce false matches. They cannot prove causal failure. The builder's complete battery and the critic's fixed sample carry that proof.

## Role, requirements, brief, and critic wording

Add this sentence verbatim to the implementer role, the build template, and implementer follow-up guidance:

> for every rule you add, a test that fails when that rule alone is removed; run it before you return

Add the same string as top-level `mutationProof` in implementer.requirements.json. Leave required, optional, and waivers unchanged. Add `{"slot":"mutation-requirement","path":"scripts/agents/roles/implementer.requirements.json"}` to the implementer recipe's sources. This uses its current file delivery loop (`metasystem/internal/dispatch/composition.go:275-283`). U1 adds mutations to the role's member list when the schema lands. The instruction unit adds:

> For each id in this round's Rules JSON array, return one mutations entry with its ruleId, location, test kind, test path, test name, and observed failureLine. Pass the test, remove only that rule, observe its failure, restore the rule, and pass the test again. Keep every run in evidence. Re-declare every earlier id whose rule you repair and supply a fresh entry for it in that round. The chain retains earlier obligations and every earlier accepted test name. Return an empty mutations array when this round declares no rules. Put any unproved rule in gaps; never invent an observation.

The templates use a valid `Rules: []` example or an indented filled example. They explain that a dispatched brief replaces it with the exact id array and matching acceptance rows. They retain the sibling's Boundary and Ceiling guidance. Do not install live unfilled optional placeholders. A follow-up keeps the chain's return schema and re-declares repaired ids. Its current unchanged-schema contract is at `metasystem/scripts/agents/templates/follow-up.md:12-18`.

Append to both repair prompt writers:

> For an implementer return, copy only mutation entries and failure lines you already observed. Never invent a missing entry or turn a passing run into a mutation claim. Report an unproved rule in gaps. This reply-only repair does not authorize repeating the work.

Acceptance can complete delivery of a shape-valid return with gaps. Review can still refuse its missing rows. Keep ordinary post-repair validation, which already occurs at `metasystem/internal/adapter/adjudicate.go:208-246`.

In code-critique Layer 1, describe cumulative mutation coverage and test association alongside the sibling's limits. Keep the seat's pre-read conformance ordering. Add:

> Join all rounds' Rules and mutation entries using the numbered temporal join in this spec. Let n be the number of distinct required ids. For n greater than zero, sample min(n, max(3, ceil(sqrt(n)))) rows. Sort by highest severity first, then ascending rule id. Use the highest declared severity of a repeated id; a row without severity ranks high. For each selected id reproduce every accepted entry for that id, in round order. Record the full ordered row inventory, the selected ids, and the observations. In a disposable copy of the exact reviewed tree, pass the named test, remove only the claimed rule, observe the claimed failure, restore the rule, and pass again. Normalize only incidental output prefixes such as temporary paths and line numbers when comparing the failure. The named assertion and cause must match. A sampled claim that does not reproduce is a material finding. A skipped test or unavailable required environment remains unproven and material. A fixture sample must enter the named leg in the script named by location; selector-looking data is not execution. Restore the disposable copy after every attempt. Expand the sample only when observed failure warrants more investigation; the required initial sample never shrinks or swaps rows.

Examples: n=1 samples 1, n=2 samples 2, n=9 samples 3, n=10 samples 4, n=100 samples 10. For n=0 no mutation sample is needed; ordinary critique still applies. This is a critic instruction, not a new automatic mutation service. The existing independent critic and materiality contracts remain at `metasystem/skills/code-critique/SKILL.md:12-23`.

## Fixtures and Go proof

The four DONE legs run through the real public `validate conformance --stage review` verb, without a critic record. Use the existing bed's controller/worktree helpers and status-plus-message checks (`metasystem/scripts/agents/conformance-fixtures.sh:38-96`, `metasystem/scripts/agents/conformance-fixtures.sh:153-186`). The source test Go functions and the public CLI tests below exercise the same inputs and behavior. Removing a production check must break its behavioral test. Removing only a new_case declaration is never a mutation witness.

| Leg | Input and exact required behavior | Go witnesses |
| --- | --- | --- |
| bprm-missing-row | A persisted admitted round declares `Rules: ["R1","R2"]`. A valid v2 return carries only R1. Review exits 1 with `MUTATION_MISSING: acceptance row "R2" declared in round 1 has no mutation entry in the chain`. Neither success artifact exists. | TestConformanceMutationMissingRow in validate; TestConformanceMutationCLIMissingRow in cmd/metasystem. |
| bprm-absent-test | Declare R1 and return a well-shaped entry. Its same-package location exists. example_test.go contains TestPresent, while the entry names TestMissing. Review exits 1 naming R1, TestMissing, and example_test.go. Neither success artifact exists. | TestConformanceMutationAbsentTest; TestConformanceMutationCLIAbsentTest. |
| bprm-complete | Declare R1 and R2. R1 names rule.go and TestPresent in example_test.go in the same package. R2 names checks.sh and its executable present-leg. Declare every changed path. Review exits 0, emits reviewedTree, and writes both artifacts. Read them back and keep return bytes identical. | TestConformanceMutationComplete; TestConformanceMutationCLIComplete. |
| bprm-no-rows | Use the old brief without Rules and an old return without mutations. Review exits 0, creates its artifacts, and preserves return bytes. A companion with `Rules: []` also passes. A separate follow-up test proves current empty Rules does not erase earlier rules. | TestConformanceMutationNoRowsUnchanged; TestConformanceMutationCLINoRows. |

The test repositories install at their root, so example_test.go has no metasystem prefix. The existing fixture builder establishes this layout (`metasystem/scripts/agents/conformance-fixtures.sh:38-64`). U4j owns the missing-row and no-row legs. U5c owns the absent-test and complete legs. Their colocated Go tests drive the same production helper before public activation. U4b adds the four public CLI cases and executes all four legs through the candidate engine before landing. Before U4b, the new shell case functions are defined but have no call sites. U4b adds those calls with the public gate. No pending leg is reported as passed. Go source witnesses live in `metasystem/internal/validate/conformance_mutations_test.go` and call public Conformance, which implements the verb (`metasystem/internal/validate/conformance.go:196-205`). CLI witnesses live in `metasystem/cmd/metasystem/conformance_mutations_test.go` and call runValidateConformance with captured stdout, stderr, and exit status (`metasystem/cmd/metasystem/validate_verbs.go:280-352`). Each returned entry names its same-package source witness; the CLI test is additional public-surface proof.

Use real composed inline and referenced rounds in separate Go seam tests. Bed convenience must not replace production source selection with a fabricated acceptanceRules member. Extend the existing implementer-v1-v2 return-schema scenario with complete-entry preservation, missing failureLine refusal, and legacy v2 omission. Its normalization and completeness calls are at `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`.

Builders run Go tests and the fast gate. They never run fixture beds or export METASYSTEM_BIN for Go tests. The seat runs beds through its enrolled engine on the returned candidate. This follows the builder/seat split at `metasystem/docs/orchestration.md:172` and `metasystem/scripts/agents/templates/brief.md:14-20`. Test-only instruction packages are included explicitly in their unit commands. They do not prove model behavior. No fixture-registration test is required by this design.

## Seams and precedence

| Seam | Decision |
| --- | --- |
| Sibling admission | U0 extends unit 1a's parser and admission value. It adds no second header scan. |
| Sibling record and review | U0 extends unit 3a's codec, unit 3c's writer and unit 3b's reader result. ReadRoundBriefRules calls ReadRoundBriefBounds for the explicit round. Both consume the same admitted-byte digest and record hash. |
| Legacy rounds | The sibling's revision-3 no-prose fallback returns no Rules. A corrupt marked round refuses. |
| Whole candidate | Lookup uses the supplied project reviewedTree. The sibling's unit 4b owns its single-snapshot change, and unit 4c owns the policy collection extended by U4b here. |
| Acceptance | Shape and old omission belong to returnChecker. Temporal coverage and candidate membership belong to review. |
| Benchmark | U1 and U2 carry exact integration patches and builder observations. The seat checks their benchmark rows against the combined repository candidate during the named manual window. |
| Immutability | Review never rewrites earlier returns or success artifacts. |
| Merge and recertification | No added mutation policy. Stage routing stays at `metasystem/internal/validate/conformance.go:248-254`. |
| Future round-proof work | A later bed-failure record does not replace Rules, their admission record or this temporal join. There is no dependency on that goal. |

## Return readers and validators

This is the compatibility inventory for implementation. The executable owners were read in this worktree. Tests are compatibility witnesses, not new obligations to rewrite old evidence.

| Reader | Disposition and checked anchor |
| --- | --- |
| Provider materializer and schema CLI | Change via the shared overlays and explicit canonical flag. `metasystem/internal/returnschema/returnschema.go:154-187`; `metasystem/cmd/metasystem/schema.go:13-46`. |
| Role, job-file, and job completeness | Change their shared checker only. `metasystem/internal/validate/returncomplete.go:66-89`, `metasystem/internal/validate/returncomplete.go:141-228`. |
| Completeness CLI and shell wrapper | Inherit the checker. `metasystem/cmd/metasystem/validate_verbs.go:178-201`; `metasystem/scripts/assert-return-complete.sh:47-50`. |
| NormalizeReturn and boundary serialization | Preserve admitted new members and old omission. Add tests; do not add a second schema owner. `metasystem/internal/adapter/return.go:25-94`, `metasystem/internal/adapter/return.go:191-225`; `metasystem/internal/validate/returncomplete.go:239-303`, `metasystem/internal/validate/returncomplete.go:326-335`. |
| Adjudication | Keep normalization, shared completeness, and post-repair validation. Change both prompts. `metasystem/internal/adapter/adjudicate.go:48-97`, `metasystem/internal/adapter/adjudicate.go:208-246`. |
| Runtime and Devin acceptance | Runtime inherits materialization; both inherit completeness. `metasystem/scripts/agents/adapters/runtime-common.sh:103-110`, `metasystem/scripts/agents/adapters/runtime-common.sh:364-385`; `metasystem/internal/adapter/devincollect.go:220-240`. |
| Fake implementer producer | Keep its directly constructed version-2 omission and shared completeness as compatibility evidence. Fake does not source runtime-common.sh (`metasystem/scripts/agents/adapters/fake.sh:140-160`). Do not invent mutation failures. `metasystem/internal/adapter/fake.go:53-74`, `metasystem/internal/adapter/fake.go:109-112`. |
| Conformance | Add the separate chain join and lookup. Keep boundary declaration policy. `metasystem/internal/validate/conformance.go:214-223`, `metasystem/internal/validate/conformance.go:724-819`. |
| Follow-up overlap and design pairing | Inspect only their owned diffBoundary fields. `metasystem/internal/dispatch/followup_rebase.go:124-148`; `metasystem/internal/dispatch/review_reference.go:313-325`. |
| Continuation transport and artifact mirror | Carry whole returns and regular artifacts. Cap continuations omit the prior return, which makes chain artifact reads essential. `metasystem/scripts/agents/dispatch.sh:2719-2725`; `metasystem/internal/dispatch/composition.go:300-319`; `metasystem/internal/dispatch/mirror.go:120-139`. |
| Lost-return and supervisor recollection | Continue shared completeness. `metasystem/scripts/agents/dispatch.sh:1336-1349`; `metasystem/internal/supervise/reaper.go:234-260`; `metasystem/cmd/metasystem/supervise_component.go:325-326`; `metasystem/internal/steward/reap.go:106-107`. |
| Landed listings and adapter probes | Keep shared completeness and recursive string inspection. `metasystem/internal/mission/landed.go:101-155`; `metasystem/internal/adapter/selftestrun.go:99-125`, `metasystem/internal/adapter/selftestrun.go:217`, `metasystem/internal/adapter/selftestrun.go:237`, `metasystem/internal/adapter/selftestrun.go:335-342`; `metasystem/internal/adapter/devin.go:337-338`. |
| Role, templates, requirements, recipe, critic skill | Change their owned instruction text and source selection. `metasystem/scripts/agents/roles/implementer.md:11-17`; `metasystem/scripts/agents/templates/brief.md:32-40`; `metasystem/scripts/agents/templates/follow-up.md:12-18`; `metasystem/scripts/agents/roles/implementer.requirements.json:2-15`; `metasystem/scripts/agents/role-packets.json:38-59`; `metasystem/skills/code-critique/SKILL.md:25-37`. |
| Frozen roster test | Keep version 1's exact field set. `metasystem/scripts/validate-metasystem.sh:1661-1706`. |
| Direct completeness regression | Keep legacy v2 normalization and byte identity. `metasystem/internal/validate/returncomplete_direct_test.go:31-121`. |
| Adjudication regression | Keep v2 no-member candidates and successful repaired acceptance. `metasystem/internal/adapter/adjudicate_test.go:205-271`. |
| Telemetry fixture | Keep its version-1-shaped normalization and observed-identity checks. This leg does not call completeness. `metasystem/scripts/agents/telemetry-census-fixtures.sh:48-72`. |
| Static positive implementer fixtures | Keep their version-1 inputs. `metasystem/scripts/validate-metasystem.sh:1917`, `metasystem/scripts/validate-metasystem.sh:1946-1947`, `metasystem/scripts/validate-metasystem.sh:2388`, `metasystem/scripts/validate-metasystem.sh:2403`. |
| Return-schema and conformance beds | Add behavioral legs; retain historical omission inputs. `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`; `metasystem/scripts/agents/conformance-fixtures.sh:73-107`, `metasystem/scripts/agents/conformance-fixtures.sh:175-186`. |
| Benchmark pin, drift check, extractor, rubrics | Regenerate canonical pin and amend its drift command. Preserve extractor, historical fixture, and scoring. `benchmark/validate-kit.sh:84-102`; `benchmark/schemas/evidence/implementer.schema.json:4`, `benchmark/schemas/evidence/implementer.schema.json:111-125`; `benchmark/extractor.py:63-118`, `benchmark/extractor.py:324-341`, `benchmark/extractor.py:514-538`; `benchmark/extractor-fixtures.sh:260-274`; `benchmark/rubrics/evidence-honesty.md:15-22`; `benchmark/rubrics/brief-quality.md:15-22`. |

Other-role readers retain their own fields and contracts: finding registration (`metasystem/internal/dispatch/finding_register.go:137-175`), clean-read admission (`metasystem/internal/dispatch/read_admission.go:400-432`), human-carried critic validation (`metasystem/internal/dispatch/review_reference.go:17-48`), warden authorization (`metasystem/internal/validate/authorization.go:274-299`), recertification (`metasystem/internal/validate/recertification.go:398-425`), landing (`metasystem/internal/landing/observe.go:750-771`), and read-subject closure (`metasystem/internal/readsubject/closure.go:198-248`). They gain no mutations parser. Steward acceptance keeps shared completeness (`metasystem/internal/steward/reap.go:225-234`). Host and mission-runner acceptance use orchestrator contracts (`metasystem/internal/host/hostcollect.go:160-176`; `metasystem/internal/missionrunner/adjudicate.go:71-88`, `metasystem/internal/missionrunner/adjudicate.go:120-126`). The fingerprint fixture copies a completeness wrapper (`metasystem/scripts/agents/fingerprint-harness.sh:134`) and dispatches a design critic (`metasystem/scripts/agents/fingerprint-harness.sh:243-246`).

## Build units and builder briefs

Land **U0, U3a, U3b, U1, U2, U4a, U4j, U5a, U5b, U5c, U4b**, in that order. Each unit is a fresh implementation chain based on the preceding landing. U0 has only the four direct sibling prerequisites named above. U4b also waits for sibling unit 4c. The order puts repair honesty in place before a launcher can require mutations, and puts all schema and lookup prerequisites before the single enforcing call. Review cannot truthfully enforce DONE sooner: U1 only supplies representation, U2 supplies valid text constraints, and the join needs the source reader and both test-kind lookups. U4b is the first unit with those owners plus their public four-leg proof. There are no later instruction or fixture-only landings postponing DONE.

Every earlier unit is unenforced by mutation conformance. The seat's required manual record is explicit:

| Unit | Return representation at dispatch | Required landing record |
| --- | --- | --- |
| U0 | Launch-compatible evidence table | Seat joined every U0 Rules id to one location, named witness, isolated observed failure and restored pass before landing. |
| U3a | Launch-compatible evidence table | Seat performed the same row join for every U3a id before landing and recorded it in this unit's landing message. |
| U3b | Launch-compatible evidence table | Seat performed the same row join for every U3b id before landing and recorded it in this unit's landing message. |
| U1 | Launch-compatible evidence table, including kit rows | Seat performed the row join for every U1 id and the exact combined kit patch before landing and recorded it in this unit's landing message. |
| U2 | Top-level mutations, including kit rows | Seat performed the row join for every U2 id and the exact combined kit patch before landing and recorded it in this unit's landing message. |
| U4a | Top-level mutations | Seat performed the row join for every U4a id before landing and recorded it in this unit's landing message. |
| U4j | Top-level mutations | Seat performed the row join for every U4j id before landing and recorded it in this unit's landing message. |
| U5a | Top-level mutations | Seat performed the row join for every U5a id before landing and recorded it in this unit's landing message. |
| U5b | Top-level mutations | Seat performed the row join for every U5b id before landing and recorded it in this unit's landing message. |
| U5c | Top-level mutations | Seat performed the row join for every U5c id before landing and recorded it in this unit's landing message. |

For the first four units, place one normal-shaped mutation array in evidence[].observed headed `Mutation table for <unit>`, with level ran. Retain all commands and observations in evidence. The launch schema cannot carry the new member until U1 lands (`metasystem/scripts/agents/adapters/runtime-common.sh:103-110`). Starting with U2, use top-level mutations. Neither representation alone proves that review enforces rows. U4b's candidate engine must reject missing and absent-test companions and admit the complete and no-row companions before landing. Its landing record binds these results, the final tree, and every earlier manual record. Do not rewrite an earlier return.

The ceiling counts additions plus deletions in production, tests, instructions, shell and generated output, including seat integration. Each allocation below is at most 400. If the concrete patch will not fit, stop before exceeding Boundary or Ceiling and return a mechanical split that keeps each rule with its witness. The seat rebriefs that split and updates the manual-window list if needed. This does not reopen prose critique. Do not compress separate rules into one entry to fit a ceiling.

Every unit receives a code read. For every row below, run the named focused witness passing, remove only that rule, observe its specific assertion failure, restore it, then run it passing. A compile failure, skip, timeout or setup failure is not mutation proof. Keep removals inside Boundary. Kit variants change the permitted patch artifact and are exercised only on the stated integration targets in the disposable copy. Each witness cell explicitly names its function and row subtest; the return's test.name is the enclosing function, while evidence.command selects the subtest. Use the implemented location symbol or restored line, not an unresolved table fragment. Each row gets its own entry and actual failureLine.

Then run, in order, `go build ./...`, `go vet <unit packages>`, `go test -race -count=1 <unit packages>`, `bash -n <unit shell files>` when applicable, and `scripts/agents/go-gate.sh --fast`. Keep the supplied GOCACHE, GOTMPDIR and STATICCHECK_CACHE. Builders never commit, run fixture beds, or export METASYSTEM_BIN. This follows the current builder/seat split (`metasystem/internal/dispatch/build.go:1126`). The fast gate is an edit-loop check (`metasystem/scripts/agents/go-gate.sh:97-103`). The seat owns enrolled-engine and delivery proof.

The tables below together are the complete rule-to-witness table. Every Rules array equals its unit's table ids. Proposed output references use create outside Boundary when copied into an actual brief; existing authority extraction recognizes that classification (`metasystem/internal/dispatch/brief.go:155-167`). No JSON path contains create syntax. The former U6a/U6b proof-only work is assigned to the production owners U4j and U5c and the public enforcement owner U4b. No fixture-registration-only test counts as a witness.

### U0. Extend admitted bounds with Rules

Allocate 135 production, 205 colocated test and 50 command seam lines, total 390.

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go","metasystem/internal/dispatch/brief_rules_test.go","metasystem/internal/dispatch/brief_bounds_record.go","metasystem/internal/dispatch/brief_rules_record_test.go","metasystem/internal/validate/brief_bounds_source.go","metasystem/internal/validate/brief_rules_source_test.go","metasystem/cmd/metasystem/dispatch_brief_rules_test.go"]
Ceiling: 400
Rules: ["U0-column","U0-case","U0-fence","U0-duplicate-header","U0-blank","U0-null","U0-array","U0-string-member","U0-trailing-json","U0-duplicate-id","U0-id","U0-order","U0-empty","U0-absent","U0-independent","U0-precedence","U0-authority","U0-one-admission","U0-record-member","U0-record-legacy","U0-record-null","U0-record-type","U0-record-string","U0-record-ids","U0-record-repeated-id","U0-record-duplicate-key","U0-persist","U0-reader","U0-reader-round","U0-reader-error","U0-legacy"]
Non-goals: No new bounds dialect, file names, digest, composition field, prose fallback, review activation, shell publication owner, fixture execution or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Extend only the sibling owners described above. The admission decoder and record validator share id validation. The new review function is a small wrapper over ReadRoundBriefBounds. Preserve its existing reference and admitted-byte authentication; test their propagation rather than copying their implementation. The codec still rejects unknown keys.

Add TestDispatchToReviewAdmittedRules in the command package as additional seam proof. Use the sibling's same job brief-mode and compose-role-packet command entrypoints, then ReadRoundBriefRules. In separately named initial-inline, initial-referenced, follow-up-inline and follow-up-referenced cases, admit R1, change the caller file and deliver conflicting Rules R9, then read only R1. A recorded-headerless companion must still return no rows despite a delivered Rules line. Check the exact admitted sha256 and the extended record hash. The command seam observes integration; the table's colocated dispatch and validate witnesses own the individual removals.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U0-column | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-column` | Recognize an indented or quoted Rules line as live; the inert-example assertion must fail. |
| U0-case | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-case` | Accept a differently cased rules prefix as a declaration. |
| U0-fence | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-fence` | Ignore a column-zero Rules line inside a fence. |
| U0-duplicate-header | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-duplicate-header` | Accept a second live Rules header. |
| U0-blank | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-blank` | Treat a blank Rules value as absence. |
| U0-null | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-null` | Accept null as an empty array. |
| U0-array | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-array` | Accept a non-array JSON value. |
| U0-string-member | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-string-member` | Accept a non-string member. |
| U0-trailing-json | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-trailing-json` | Ignore JSON data after the array. |
| U0-duplicate-id | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-duplicate-id` | Accept the same id twice. |
| U0-id | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-id` | Accept an id outside the ruleId grammar. |
| U0-order | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-order` | Sort the decoded array; its authored order must survive. |
| U0-empty | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-empty` | Reject an explicit [] array; it must declare no rows and retain a non-nil empty in-memory slice. |
| U0-absent | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-absent` | Require Rules when absent; its nil value must declare no rows and keep paired bounds unchanged. |
| U0-independent | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-independent` | Require the bounds pair merely because Rules is present. |
| U0-precedence | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-precedence` | Report Rules value failure before the sibling paired-header failure. |
| U0-authority | high | `metasystem/internal/dispatch/brief.go#ParseBriefBounds` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesAdmission/U0-authority` | Extract a live Rules header or its space/tab-indented example as an input path. |
| U0-one-admission | high | `metasystem/internal/dispatch/brief.go#admission-value` | `metasystem/internal/dispatch/brief_rules_test.go#TestBriefRulesSingleAdmission/U0-one-admission` | Re-read the caller file instead of carrying the already parsed admission value; a changed caller must not change stored Rules. |
| U0-record-member | high | `metasystem/internal/dispatch/brief_bounds_record.go#rules-member` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesRecord/U0-record-member` | Drop rules from new serialization; a Rules-only admission must round-trip its ids. |
| U0-record-legacy | high | `metasystem/internal/dispatch/brief_bounds_record.go#optional-rules` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesLegacyRecord/U0-record-legacy` | Require rules in a pre-extension record; its old bounded pair must still read with no rows. |
| U0-record-null | high | `metasystem/internal/dispatch/brief_bounds_record.go#rules-decoding` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesRecordValidation/U0-record-null` | Allow present null rules. |
| U0-record-type | high | `metasystem/internal/dispatch/brief_bounds_record.go#rules-decoding` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesRecordValidation/U0-record-type` | Allow a non-array persisted rules value. |
| U0-record-string | high | `metasystem/internal/dispatch/brief_bounds_record.go#rules-decoding` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesRecordValidation/U0-record-string` | Allow a non-string member in a persisted rules array. |
| U0-record-ids | high | `metasystem/internal/dispatch/brief_bounds_record.go#rules-decoding` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesRecordValidation/U0-record-ids` | Skip id grammar validation for persisted rules. |
| U0-record-repeated-id | high | `metasystem/internal/dispatch/brief_bounds_record.go#rules-decoding` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesRecordValidation/U0-record-repeated-id` | Skip duplicate-id rejection for persisted rules. |
| U0-record-duplicate-key | high | `metasystem/internal/dispatch/brief_bounds_record.go#rules-decoding` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesRecordValidation/U0-record-duplicate-key` | Allow a repeated rules key in the closed codec. |
| U0-persist | high | `metasystem/internal/dispatch/brief_bounds_record.go#admission-writer` | `metasystem/internal/dispatch/brief_rules_record_test.go#TestBriefRulesPersist/U0-persist` | Omit the admitted Rules when handing the admission value to the existing record writer. |
| U0-reader | high | `metasystem/internal/validate/brief_bounds_source.go#ReadRoundBriefRules` | `metasystem/internal/validate/brief_rules_source_test.go#TestReadRoundBriefRules/U0-reader` | Read prompt headers instead of the sibling reader result; conflicting augmented and prior sources must not change Rules. |
| U0-reader-round | high | `metasystem/internal/validate/brief_bounds_source.go#ReadRoundBriefRules` | `metasystem/internal/validate/brief_rules_source_test.go#TestReadRoundBriefRulesExplicitRound/U0-reader-round` | Pass a different round or its composition; a two-round job must return only the requested round ids. |
| U0-reader-error | high | `metasystem/internal/validate/brief_bounds_source.go#ReadRoundBriefRules` | `metasystem/internal/validate/brief_rules_source_test.go#TestReadRoundBriefRulesFailure/U0-reader-error` | Swallow the sibling reader error on an admitted digest, record hash or composition mismatch. |
| U0-legacy | high | `metasystem/internal/validate/brief_bounds_source.go#ReadRoundBriefRules` | `metasystem/internal/validate/brief_rules_source_test.go#TestReadRoundBriefRulesLegacy/U0-legacy` | Parse a legacy follow-up prompt or root brief containing Rules; it must still declare no rows. |

Verification packages: `./internal/dispatch ./internal/validate ./cmd/metasystem`. No shell syntax input.

### U3a. Deliver the builder obligation

Allocate 70 instruction and recipe lines and 230 focused test lines, total 300.

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/roles/implementer.md","metasystem/scripts/agents/roles/implementer.requirements.json","metasystem/scripts/agents/roles/mutation_contract_test.go","metasystem/scripts/agents/role-packets.json","metasystem/scripts/agents/mutation_packet_test.go","metasystem/scripts/agents/templates/brief.md","metasystem/scripts/agents/templates/follow-up.md","metasystem/scripts/agents/templates/mutation_contract_test.go"]
Ceiling: 400
Rules: ["U3a-role-proof","U3a-role-entries","U3a-redeclare","U3a-retain-tests","U3a-gap","U3a-requirement","U3a-delivery","U3a-brief-proof","U3a-brief-rules","U3a-followup-proof","U3a-followup-rules"]
Non-goals: No capability names, header parser, admission record, composition schema, review logic, fixture execution or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Use colocated Go test packages for the instruction resources. The recipe witness calls ComposeRolePacket and inspects delivered bytes and source metadata. Preserve the sibling's Boundary and Ceiling instructions and the requirements capability fields. Guards prove wording and delivery, not model obedience. This unit uses the launch-compatible evidence table in its own brief. The durable instruction describes the final contract; U1 updates the enumerated return-member list when that shape becomes available.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U3a-role-proof | high | `metasystem/scripts/agents/roles/implementer.md#mutation-proof` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions/U3a-role-proof` | Remove the exact proof sentence. |
| U3a-role-entries | high | `metasystem/scripts/agents/roles/implementer.md#mutation-return` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions/U3a-role-entries` | Remove the per-id entry instruction. |
| U3a-redeclare | high | `metasystem/scripts/agents/roles/implementer.md#mutation-repair` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions/U3a-redeclare` | Remove the instruction to re-declare repaired ids. |
| U3a-retain-tests | high | `metasystem/scripts/agents/roles/implementer.md#chain-obligations` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions/U3a-retain-tests` | Remove the instruction that earlier accepted test names remain obligations. |
| U3a-gap | high | `metasystem/scripts/agents/roles/implementer.md#unproved-rule` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions/U3a-gap` | Remove the instruction to report unproved rules in gaps. |
| U3a-requirement | high | `metasystem/scripts/agents/roles/implementer.requirements.json#mutationProof` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRequirement/U3a-requirement` | Remove the exact mutationProof value. |
| U3a-delivery | high | `metasystem/scripts/agents/role-packets.json#implementer.sources` | create: `metasystem/scripts/agents/mutation_packet_test.go#TestComposeMutationRequirement/U3a-delivery` | Remove the requirements source from the recipe; actual composed bytes must lose the required instruction. |
| U3a-brief-proof | high | `metasystem/scripts/agents/templates/brief.md#Constraints` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationBriefInstructions/U3a-brief-proof` | Remove the exact proof sentence from the build template. |
| U3a-brief-rules | high | `metasystem/scripts/agents/templates/brief.md#Acceptance-Criteria` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationBriefInstructions/U3a-brief-rules` | Remove the Rules JSON-array instruction. |
| U3a-followup-proof | high | `metasystem/scripts/agents/templates/follow-up.md#Unchanged-Return-Contract` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationFollowUpInstructions/U3a-followup-proof` | Remove the exact proof sentence from follow-up guidance. |
| U3a-followup-rules | high | `metasystem/scripts/agents/templates/follow-up.md#repaired-rules` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationFollowUpInstructions/U3a-followup-rules` | Remove the follow-up instruction to re-declare repaired ids. |

Verification packages: `./scripts/agents ./scripts/agents/roles ./scripts/agents/templates ./internal/dispatch`. No shell syntax input.

### U3b. Keep repair honest and fix the critic sample

Allocate 70 prompt and skill lines and 220 focused test lines, total 290.

```text
Working Mode: implement
Boundary: ["metasystem/internal/adapter/adjudicate.go","metasystem/internal/adapter/mutation_repair_test.go","metasystem/skills/code-critique/SKILL.md","metasystem/skills/code-critique/mutation_contract_test.go"]
Ceiling: 400
Rules: ["U3b-repair-shape","U3b-repair-delivery","U3b-sample-min","U3b-sample-root","U3b-sample-small","U3b-severity","U3b-id-order","U3b-each-entry","U3b-material","U3b-environment","U3b-fixture-execution","U3b-record","U3b-disposable","U3b-restore","U3b-severity-default","U3b-severity-repeat","U3b-no-swap","U3b-failure-cause"]
Non-goals: No automatic mutation runner, schema change, repair control-flow change, fixture execution, critic budget change or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Invoke both actual prompt writers and inspect their output. The skill test guards each independent sampling instruction. The seat later checks actual critic evidence. Keep ordinary post-repair validation. This unit lands before U1 so an empty-delivery or reply-shape repair cannot begin requiring new mutation entries without the honesty instruction.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U3b-repair-shape | high | `metasystem/internal/adapter/adjudicate.go#writeRepairPrompt` | create: `metasystem/internal/adapter/mutation_repair_test.go#TestMutationRepairInstructions/U3b-repair-shape` | Remove the exact mutation honesty paragraph from ordinary repair output. |
| U3b-repair-delivery | high | `metasystem/internal/adapter/adjudicate.go#writeDeliveryRepairPrompt` | create: `metasystem/internal/adapter/mutation_repair_test.go#TestMutationDeliveryRepairInstructions/U3b-repair-delivery` | Remove that paragraph from missing-delivery repair output. |
| U3b-sample-min | high | `metasystem/skills/code-critique/SKILL.md#minimum-sample` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-sample-min` | Remove the at-least-three component of the sample rule. |
| U3b-sample-root | high | `metasystem/skills/code-critique/SKILL.md#sample-growth` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-sample-root` | Remove the ceiling-square-root component. |
| U3b-sample-small | high | `metasystem/skills/code-critique/SKILL.md#small-inventory` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-sample-small` | Remove the cap at n, which samples every row when n is below three. |
| U3b-severity | high | `metasystem/skills/code-critique/SKILL.md#severity-order` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-severity` | Remove highest-severity-first ordering. |
| U3b-id-order | high | `metasystem/skills/code-critique/SKILL.md#id-order` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-id-order` | Remove ascending-id tie breaking. |
| U3b-each-entry | high | `metasystem/skills/code-critique/SKILL.md#repeated-id` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-each-entry` | Remove reproduction of every accepted entry for a sampled id. |
| U3b-material | high | `metasystem/skills/code-critique/SKILL.md#non-reproduction` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-material` | Remove material classification of a non-reproducing sample. |
| U3b-environment | high | `metasystem/skills/code-critique/SKILL.md#execution-environment` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-environment` | Permit a skip or unavailable environment to count as reproduction. |
| U3b-fixture-execution | high | `metasystem/skills/code-critique/SKILL.md#fixture-execution` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-fixture-execution` | Remove the requirement to enter the named leg in the location script. |
| U3b-record | high | `metasystem/skills/code-critique/SKILL.md#sample-record` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-record` | Remove the ordered inventory and sampled-observation record instruction. |
| U3b-disposable | high | `metasystem/skills/code-critique/SKILL.md#sample-copy` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-disposable` | Remove use of a disposable reviewed-tree copy. |
| U3b-restore | high | `metasystem/skills/code-critique/SKILL.md#sample-restoration` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-restore` | Remove restoration after each attempt. |
| U3b-severity-default | high | `metasystem/skills/code-critique/SKILL.md#default-severity` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-severity-default` | Remove the high default for a row without severity. |
| U3b-severity-repeat | high | `metasystem/skills/code-critique/SKILL.md#repeated-severity` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-severity-repeat` | Remove use of the highest declared severity across repeats of an id. |
| U3b-no-swap | high | `metasystem/skills/code-critique/SKILL.md#fixed-initial-sample` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-no-swap` | Permit replacing a selected row with a more convenient row. |
| U3b-failure-cause | high | `metasystem/skills/code-critique/SKILL.md#failure-cause` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract/U3b-failure-cause` | Remove the requirement that the reproduced assertion and cause match the claim. |

Verification packages: `./internal/adapter ./skills/code-critique`. No shell syntax input.

### U1. Bootstrap the compatible return shape

Allocate 100 production and CLI, 125 engine test, 70 generated pin, 80 kit integration and test, and 15 role lines, total 390.

```text
Working Mode: implement
Boundary: ["metasystem/internal/returnschema/returnschema.go","metasystem/internal/returnschema/implementer.go","metasystem/internal/returnschema/implementer_test.go","metasystem/internal/validate/returncomplete.go","metasystem/internal/validate/return_mutations_test.go","metasystem/cmd/metasystem/schema.go","metasystem/cmd/metasystem/schema_mutations_test.go","metasystem/scripts/agents/roles/implementer.md","metasystem/scripts/agents/roles/mutation_contract_test.go","metasystem/artifacts/reports/bprm-kit-compat.patch"]
Ceiling: 400
Rules: ["U1-array","U1-entry-object","U1-test-object","U1-require-ruleId","U1-require-location","U1-require-test","U1-require-failureLine","U1-require-test-kind","U1-require-test-path","U1-require-test-name","U1-type-ruleId","U1-type-location","U1-type-failureLine","U1-type-test.kind","U1-type-test.path","U1-type-test.name","U1-close-entry","U1-close-test","U1-kind","U1-provider-required","U1-canonical-property","U1-canonical-optional","U1-provider-role","U1-provider-version","U1-checker-overlay","U1-cli-canonical","U1-cli-role","U1-cli-version","U1-role-members","U1-kit-command","U1-kit-hint","U1-kit-pin","U1-kit-omission","U1-kit-other-roles"]
Non-goals: No text constraints yet, new headers, review join, lookup, benchmark scoring, direct benchmark edits in the delegate tree, fixture execution or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Install the typed, closed fragment and kind enum. Add the provider-required and canonical-optional overlays, wire completeness, expose --canonical and update the role's member list. U2 adds text constraints. Schema tests inspect the actual materialized schema; canonical tests call ReturnCompleteRole on complete, malformed and old omitted-member returns. Leave the v1 file unchanged.

Return the launch-compatible mutation table for every row, including all five kit rows. Supply the exact benchmark patch and its Go witnesses. The seat joins and reads the combined diff and each observation before landing. This lands representation only. It does not enforce Rules and it is not the only manually joined unit.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U1-array | high | create: `metasystem/internal/returnschema/implementer.go#mutations.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-array` | Remove the array type; the null-rejection assertion must fail. |
| U1-entry-object | high | create: `metasystem/internal/returnschema/implementer.go#mutations.items.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-entry-object` | Remove the item object type. |
| U1-test-object | high | create: `metasystem/internal/returnschema/implementer.go#test.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-test-object` | Remove the test object type. |
| U1-require-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.ruleId` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-require-ruleId` | Remove only ruleId from entry.required. |
| U1-require-location | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.location` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-require-location` | Remove only location from entry.required. |
| U1-require-test | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.test` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-require-test` | Remove only test from entry.required. |
| U1-require-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.failureLine` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-require-failureLine` | Remove only failureLine from entry.required. |
| U1-require-test-kind | high | create: `metasystem/internal/returnschema/implementer.go#test.required.kind` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-require-test-kind` | Remove only kind from test.required. |
| U1-require-test-path | high | create: `metasystem/internal/returnschema/implementer.go#test.required.path` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-require-test-path` | Remove only path from test.required. |
| U1-require-test-name | high | create: `metasystem/internal/returnschema/implementer.go#test.required.name` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-require-test-name` | Remove only name from test.required. |
| U1-type-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#ruleId.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-type-ruleId` | Remove only this scalar type from the provider schema. |
| U1-type-location | high | create: `metasystem/internal/returnschema/implementer.go#location.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-type-location` | Remove only this scalar type from the provider schema. |
| U1-type-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#failureLine.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-type-failureLine` | Remove only this scalar type from the provider schema. |
| U1-type-test.kind | high | create: `metasystem/internal/returnschema/implementer.go#test.kind.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-type-test.kind` | Remove only this scalar type from the provider schema. |
| U1-type-test.path | high | create: `metasystem/internal/returnschema/implementer.go#test.path.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-type-test.path` | Remove only this scalar type from the provider schema. |
| U1-type-test.name | high | create: `metasystem/internal/returnschema/implementer.go#test.name.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-type-test.name` | Remove only this scalar type from the provider schema. |
| U1-close-entry | high | create: `metasystem/internal/returnschema/implementer.go#entry.additionalProperties` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-close-entry` | Permit an extra entry member. |
| U1-close-test | high | create: `metasystem/internal/returnschema/implementer.go#test.additionalProperties` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-close-test` | Permit an extra test member. |
| U1-kind | high | create: `metasystem/internal/returnschema/implementer.go#test.kind.enum` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema/U1-kind` | Remove the kind enum; an unknown kind must be refused. |
| U1-provider-required | high | create: `metasystem/internal/returnschema/implementer.go#ImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationProviderRequired/U1-provider-required` | Remove mutations from the provider root required list. |
| U1-canonical-property | high | create: `metasystem/internal/returnschema/implementer.go#CanonicalImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationCanonicalProperty/U1-canonical-property` | Omit the canonical mutations property; an extended return must remain lawful. |
| U1-canonical-optional | high | create: `metasystem/internal/returnschema/implementer.go#CanonicalImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationCanonicalOmission/U1-canonical-optional` | Require mutations in canonical output; a historical omitted member must remain lawful. |
| U1-provider-role | high | `metasystem/internal/returnschema/returnschema.go#Materialize` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationOverlayScope/U1-provider-role` | Remove the implementer-only overlay guard. |
| U1-provider-version | high | `metasystem/internal/returnschema/returnschema.go#Materialize` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationOverlayScope/U1-provider-version` | Apply the overlay to version 1. |
| U1-checker-overlay | high | `metasystem/internal/validate/returncomplete.go#checkReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestReturnMutationShape/U1-checker-overlay` | Remove canonical implementer-v2 overlay selection. |
| U1-cli-canonical | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalImplementer/U1-cli-canonical` | Route --canonical through the provider materializer. |
| U1-cli-role | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalScope/U1-cli-role` | Remove only the --canonical role guard. |
| U1-cli-version | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalScope/U1-cli-version` | Remove only the --canonical version guard. |
| U1-role-members | high | `metasystem/scripts/agents/roles/implementer.md#version-2-members` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleMembers/U1-role-members` | Remove mutations from the required return member sentence. |
| U1-kit-command | high | `benchmark/validate-kit.sh#derived-implementer-command` | `benchmark/bprm_mutation_test.go#TestBPRMKitCanonicalCommand/U1-kit-command` | Remove --canonical from only the implementer drift invocation; the recorded argv assertion must fail. |
| U1-kit-hint | high | `benchmark/validate-kit.sh#implementer-regeneration-hint` | `benchmark/bprm_mutation_test.go#TestBPRMKitCanonicalCommand/U1-kit-hint` | Remove --canonical from only the implementer regeneration hint; the failed-comparison diagnostic assertion must fail. |
| U1-kit-pin | high | `benchmark/schemas/evidence/implementer.schema.json#canonical-pin` | `benchmark/bprm_mutation_test.go#TestBPRMKitPin/U1-kit-pin` | Change one pin byte; exact comparison with canonical materialization must fail. |
| U1-kit-omission | high | `benchmark/schemas/evidence/implementer.schema.json#required` | `benchmark/bprm_mutation_test.go#TestBPRMKitHistoricalOmission/U1-kit-omission` | Add mutations to the pin root required array; the actual extractor must reject the unchanged historical omitted member. |
| U1-kit-other-roles | high | `benchmark/validate-kit.sh#derived-other-role-command` | `benchmark/bprm_mutation_test.go#TestBPRMKitOtherRoles/U1-kit-other-roles` | Apply --canonical to another derived role; its recorded ordinary-v2 argv assertion must fail. |

Verification packages: `./internal/returnschema ./internal/validate ./cmd/metasystem ./scripts/agents/roles`. Shell syntax: run `bash -n benchmark/validate-kit.sh` in the disposable integration copy. The role file is prose.

### U2. Validate each text claim and preserve compatibility

Allocate 30 constraint, 210 acceptance and preservation test, 65 fixture and 50 pin and kit lines, total 355.

```text
Working Mode: implement
Boundary: ["metasystem/internal/returnschema/implementer.go","metasystem/internal/returnschema/implementer_test.go","metasystem/internal/validate/returncomplete.go","metasystem/internal/validate/return_mutations_test.go","metasystem/internal/adapter/return.go","metasystem/internal/adapter/mutation_return_test.go","metasystem/scripts/agents/return-schema-fixtures.sh","metasystem/artifacts/reports/bprm-kit-compat.patch"]
Ceiling: 400
Rules: ["U2-pattern-ruleId","U2-pattern-location","U2-pattern-test.path","U2-pattern-test.name","U2-pattern-failureLine","U2-normalize","U2-boundary-preserve","U2-omit-unchanged","U2-kit-command","U2-kit-hint","U2-kit-pin","U2-kit-omission","U2-kit-other-roles"]
Non-goals: No second bootstrap contract, new source owner, review activation, lookup, benchmark scoring, direct benchmark edits in the delegate tree, fixture execution or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Install the five text-pattern applications from Return member. TestMutationLocationForms accepts both legal suffixes and rejects the former misspelling. All other scalar constraints have distinct row subtests. Add normalization and boundary-serialization preservation witnesses and the three return-schema scenario assertions. Refresh the canonical benchmark pin and independently repeat all five kit removals against the U2 integration candidate. A copied failure line is not a fresh observation.

This is the first top-level mutations return. The seat still joins it manually, including its kit rows. U2 does not rewrite U1 or claim to close the enforcement window. Canonical omission preserves old returns; it cannot enforce declared Rules.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U2-pattern-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#ruleId.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints/U2-pattern-ruleId` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-pattern-location | high | create: `metasystem/internal/returnschema/implementer.go#location.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationLocationForms/U2-pattern-location` | Remove the location pattern; the malformed-form assertion must fail. Separately replace its alternation with a literal bar; both legal-form assertions must fail. |
| U2-pattern-test.path | high | create: `metasystem/internal/returnschema/implementer.go#test.path.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints/U2-pattern-test.path` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-pattern-test.name | high | create: `metasystem/internal/returnschema/implementer.go#test.name.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints/U2-pattern-test.name` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-pattern-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#failureLine.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints/U2-pattern-failureLine` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-normalize | high | `metasystem/internal/adapter/return.go#NormalizeReturn` | create: `metasystem/internal/adapter/mutation_return_test.go#TestNormalizeMutationEvidence/U2-normalize` | Drop mutations from the selected normalized object; the complete evidence must survive. |
| U2-boundary-preserve | high | `metasystem/internal/validate/returncomplete.go#writeNormalizedReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestBoundaryRewritePreservesMutations/U2-boundary-preserve` | Drop mutations only at boundary serialization; the same entries must survive a lawful bare-path repair. |
| U2-omit-unchanged | high | `metasystem/internal/validate/returncomplete.go#checkReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestLegacyMutationOmissionUnchanged/U2-omit-unchanged` | Insert an empty mutations field into an otherwise valid old return; byte identity must fail. |
| U2-kit-command | high | `benchmark/validate-kit.sh#derived-implementer-command` | `benchmark/bprm_mutation_test.go#TestBPRMKitCanonicalCommand/U2-kit-command` | Remove --canonical from only the implementer drift invocation; the recorded argv assertion must fail. |
| U2-kit-hint | high | `benchmark/validate-kit.sh#implementer-regeneration-hint` | `benchmark/bprm_mutation_test.go#TestBPRMKitCanonicalCommand/U2-kit-hint` | Remove --canonical from only the implementer regeneration hint; the failed-comparison diagnostic assertion must fail. |
| U2-kit-pin | high | `benchmark/schemas/evidence/implementer.schema.json#canonical-pin` | `benchmark/bprm_mutation_test.go#TestBPRMKitPin/U2-kit-pin` | Change one pin byte; exact comparison with canonical materialization must fail. |
| U2-kit-omission | high | `benchmark/schemas/evidence/implementer.schema.json#required` | `benchmark/bprm_mutation_test.go#TestBPRMKitHistoricalOmission/U2-kit-omission` | Add mutations to the pin root required array; the actual extractor must reject the unchanged historical omitted member. |
| U2-kit-other-roles | high | `benchmark/validate-kit.sh#derived-other-role-command` | `benchmark/bprm_mutation_test.go#TestBPRMKitOtherRoles/U2-kit-other-roles` | Apply --canonical to another derived role; its recorded ordinary-v2 argv assertion must fail. |

Verification packages: `./internal/returnschema ./internal/validate ./internal/adapter`. Shell syntax input: `scripts/agents/return-schema-fixtures.sh; also benchmark/validate-kit.sh in the disposable integration copy`.

### U4a. Collect declared rows from their admitted rounds

Allocate 140 production and moved lines and 210 test lines, total 350.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance.go","metasystem/internal/validate/conformance_rounds.go","metasystem/internal/validate/conformance_rounds_test.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go"]
Ceiling: 400
Rules: ["U4a-rounds","U4a-root","U4a-round-read","U4a-source","U4a-source-error","U4a-returnless","U4a-identity","U4a-numeric-order"]
Non-goals: No source parser copy, temporal join, acceptance normalization, candidate lookup, review activation, fixture execution or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Extract the directory selection from cumulativeBoundaryViolations, leaving boundary policy and compatibility handling unchanged. Carry each selected round's job and composition into ReadRoundBriefRules. Include returnless rounds. This unit collects declarations before return parsing so the following join can preserve the zero-row fast path. A real composition fixture must include admitted record markers and hashes, not just a fabricated Rules member. The shared helper gets a numeric-order mutation test while the boundary caller retains its old order.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U4a-rounds | high | create: `metasystem/internal/validate/conformance_rounds.go#chainRoundDirectories` | create: `metasystem/internal/validate/conformance_rounds_test.go#TestMutationChainRoundDirectories/U4a-rounds` | Inspect only the supplied round; higher recorded chain rounds must remain in the inventory. |
| U4a-root | high | create: `metasystem/internal/validate/conformance_rounds.go#chainRoundDirectories` | create: `metasystem/internal/validate/conformance_rounds_test.go#TestMutationChainRootIsolation/U4a-root` | Remove chain-root filtering; another chain must not supply obligations or evidence. |
| U4a-round-read | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationRoundFacts` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRoundReadFailure/U4a-round-read` | Treat an unreadable round directory as an empty chain. |
| U4a-source | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationRules` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationChainRuleSource/U4a-source` | Replace ReadRoundBriefRules with root prose; admitted inline and referenced rounds with conflicting delivered Rules must fail. |
| U4a-source-error | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationRules` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationBriefFailure/U4a-source-error` | Swallow a shared source or parser error. |
| U4a-returnless | high | create: `metasystem/internal/validate/conformance_mutations.go#collect-round-declarations` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReturnlessRound/U4a-returnless` | Skip a round brief because its return is absent; its rule must remain required. |
| U4a-identity | high | `metasystem/internal/validate/conformance_rounds.go#round-job-composition` | `metasystem/internal/validate/conformance_rounds_test.go#TestMutationRoundJobIdentity/U4a-identity` | Reuse the supplied job or its composition for another selected round; per-round persisted record binding must fail. |
| U4a-numeric-order | high | `metasystem/internal/validate/conformance_rounds.go#numeric-mutation-rounds` | `metasystem/internal/validate/conformance_rounds_test.go#TestMutationNumericRoundOrder/U4a-numeric-order` | Keep lexicographic order for mutation rounds; rounds 2 and 10 must be joined in numeric order. |

Verification packages: `./internal/validate`. No shell syntax input.

### U4j. Join rows with explicit time semantics

Allocate 120 join, 195 focused test and 75 fixture-function lines, total 390.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go","metasystem/scripts/agents/conformance-fixtures.sh"]
Ceiling: 400
Rules: ["U4j-union","U4j-any-round","U4j-shape","U4j-object","U4j-return-read","U4j-missing","U4j-duplicate","U4j-cross-round-repeat","U4j-rejected-repair","U4j-temporal","U4j-unknown-stays","U4j-redeclare","U4j-fresh","U4j-fresh-recovery","U4j-final-coverage","U4j-no-rows","U4j-missing-public-rule"]
Non-goals: No second header reader, candidate-file implementation, public activation, evidence rewriting, fixture execution by the builder or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement the numbered temporal join over U4a's round facts. Keep its valid-entry inventory distinct from final coverage. The join exposes its candidate-lookup input as a package-local function parameter so U5c can attach the real owner; focused join tests supply explicit found, absent and error results. This is an internal algorithm seam, never a permissive public gate. Tests assert that rejected entries never reach lookup. U5c must replace that test input with real reviewed-tree lookup before U4b activates review.

Define bprm_missing_row_case and bprm_no_rows_case in the existing conformance bed. Each function contains its named new_case leg, persisted admission setup and public CLI assertions from Fixtures and Go proof. U4b adds their invocation sites with the public gate. Before that, the builder proves the production join directly through the named colocated Go tests. These functions have pending public proof, never a reported successful bed run. No fixture name is a substitute for a mutation witness.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U4j-union | high | create: `metasystem/internal/validate/conformance_mutations.go#required-rule-union` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationEarlierMissingRule/U4j-union` | Replace the union with the current Rules array; an earlier missing rule must still refuse. |
| U4j-any-round | high | create: `metasystem/internal/validate/conformance_mutations.go#entry-collection` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationCrossRoundEntry/U4j-any-round` | Read only the current return; earlier and later properly declared entries must both contribute. |
| U4j-shape | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationEntries` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationMalformedEntry/U4j-shape` | Bypass the one shared schema fragment before typed decoding. |
| U4j-object | high | create: `metasystem/internal/validate/conformance_mutations.go#return-object` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReturnObject/U4j-object` | Treat a present nonobject return as empty evidence. |
| U4j-return-read | high | create: `metasystem/internal/validate/conformance_mutations.go#return-read` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReturnReadFailure/U4j-return-read` | Treat a corrupt or unreadable present return as missing. |
| U4j-missing | high | create: `metasystem/internal/validate/conformance_mutations.go#missing-id` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationMissingCoverage/U4j-missing` | Remove the coverage requirement; absent member and one absent id are data cases of this rule. |
| U4j-duplicate | high | create: `metasystem/internal/validate/conformance_mutations.go#duplicate-in-return` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationDuplicateEntry/U4j-duplicate` | Remove within-return duplicate rejection. |
| U4j-cross-round-repeat | high | create: `metasystem/internal/validate/conformance_mutations.go#duplicate-in-return` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRepeatedRuleAcrossRounds/U4j-cross-round-repeat` | Reject an id repeated in a different round; repaired claims must remain lawful. |
| U4j-rejected-repair | high | create: `metasystem/internal/validate/conformance_mutations.go#accepted-entry-collection` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRejectedClaimRepair/U4j-rejected-repair` | Let a rejected historical duplicate, undeclared repair or unknown claim permanently refuse a later unique complete join. |
| U4j-temporal | high | `metasystem/internal/validate/conformance_mutations.go#declared-by-entry-round` | `metasystem/internal/validate/conformance_mutations_test.go#TestMutationDeclaredByEntryRound/U4j-temporal` | Test membership in final D instead of D(r); a future declaration must not admit an early entry. |
| U4j-unknown-stays | high | `metasystem/internal/validate/conformance_mutations.go#unknown-id` | `metasystem/internal/validate/conformance_mutations_test.go#TestMutationFutureDeclarationDoesNotReviveEntry/U4j-unknown-stays` | Revive an earlier unknown entry when its id is later declared; R9 must still lack coverage. |
| U4j-redeclare | high | `metasystem/internal/validate/conformance_mutations.go#repair-declaration` | `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRepairRedeclaresID/U4j-redeclare` | Skip S(r) membership for a repair entry; returning R1 under only Rules R2 must refuse. |
| U4j-fresh | high | `metasystem/internal/validate/conformance_mutations.go#repair-freshness` | `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRepairNeedsFreshEntry/U4j-fresh` | Let an old valid entry satisfy a repeated declaration without a new E(r) entry. |
| U4j-fresh-recovery | high | `metasystem/internal/validate/conformance_mutations.go#latest-declaration` | `metasystem/internal/validate/conformance_mutations_test.go#TestMutationLaterFreshRepairClosesAttempt/U4j-fresh-recovery` | Keep an earlier missing repair attempt permanently open after a later re-declaration and fresh valid entry. |
| U4j-final-coverage | high | `metasystem/internal/validate/conformance_mutations.go#final-coverage` | `metasystem/internal/validate/conformance_mutations_test.go#TestMutationFinalTreeCoverage/U4j-final-coverage` | Compute coverage from only the latest return or from an entry that fails final-tree lookup; the exact C = D assertion must fail. |
| U4j-no-rows | high | `metasystem/internal/validate/conformance_mutations.go#empty-required-set` | `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationNoRowsUnchanged/U4j-no-rows` | Read or demand mutation evidence with empty D; legacy bytes must remain unchanged. |
| U4j-missing-public-rule | high | `metasystem/internal/validate/conformance_mutations.go#missing-id` | `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationMissingRow/U4j-missing-public-rule` | Remove missing-row rejection; the production helper must name R2 and public proof in U4b must refuse. |

Verification packages: `./internal/validate`. Shell syntax input: `scripts/agents/conformance-fixtures.sh`.

### U5a. Bind files and locations to the candidate

Allocate 145 production and 230 test lines, total 375.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/mutation_test_lookup.go","metasystem/internal/validate/mutation_test_lookup_test.go"]
Ceiling: 400
Rules: ["U5a-absolute","U5a-nul","U5a-backslash","U5a-linebreak","U5a-empty-component","U5a-dot","U5a-parent","U5a-prefix","U5a-tree","U5a-controller","U5a-frozen","U5a-exact-entry","U5a-regular-only","U5a-mode-644","U5a-mode-755","U5a-location","U5a-package-dir","U5a-package-name","U5a-external-package","U5a-fixture-script","U5a-pipe"]
Non-goals: No new snapshot, returned-command execution, source reader, declaration matcher, public activation, fixture execution or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement the path, exact-tree read and package/script association helpers taking the full entry and reviewedTree. No extra snapshot is allowed. Spaces remain lawful. Keep each path predicate independently tested at its helper so another later failure cannot mask removal. The same applies to modes and association. The following units add declaration recognition.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U5a-absolute | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-absolute` | Permit an absolute path. |
| U5a-nul | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-nul` | Permit NUL in a path. |
| U5a-backslash | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-backslash` | Permit a backslash in a path. |
| U5a-linebreak | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-linebreak` | Permit a line break in a path. |
| U5a-empty-component | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-empty-component` | Permit an empty slash component. |
| U5a-dot | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-dot` | Permit a dot component. |
| U5a-parent | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-parent` | Permit a parent component. |
| U5a-prefix | high | create: `metasystem/internal/validate/mutation_test_lookup.go#project-paths` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPathProjection/U5a-prefix` | Remove the existing one-time projectDeclaration conversion; nested paths must resolve correctly. |
| U5a-tree | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationCandidateFiles/U5a-tree` | Read HEAD instead of the supplied tree; an uncommitted test must count and a deletion must not. |
| U5a-controller | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationControllerIsNotCandidate/U5a-controller` | Read the controller file for a missing candidate path. |
| U5a-frozen | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFrozenTree/U5a-frozen` | Read live workspace bytes after tree capture. |
| U5a-exact-entry | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationExactEntry/U5a-exact-entry` | Treat a descendant entry as proof that the requested directory is a test file. |
| U5a-regular-only | high | create: `metasystem/internal/validate/mutation_test_lookup.go#regular-file-mode` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFileModes/U5a-regular-only` | Remove the regular-mode whitelist; symlink and gitlink data cases must refuse. |
| U5a-mode-644 | high | create: `metasystem/internal/validate/mutation_test_lookup.go#regular-file-mode` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFileModes/U5a-mode-644` | Remove 100644 from the accepted modes. |
| U5a-mode-755 | high | create: `metasystem/internal/validate/mutation_test_lookup.go#regular-file-mode` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFileModes/U5a-mode-755` | Remove 100755 from the accepted modes. |
| U5a-location | high | create: `metasystem/internal/validate/mutation_test_lookup.go#location-file` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationLocationFile/U5a-location` | Skip existence checking for the location file. |
| U5a-package-dir | high | create: `metasystem/internal/validate/mutation_test_lookup.go#go-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationSamePackage/U5a-package-dir` | Accept an unrelated test in another directory with the same package name. |
| U5a-package-name | high | create: `metasystem/internal/validate/mutation_test_lookup.go#go-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPackageClause/U5a-package-name` | Accept an unrelated Go package clause in the same directory. |
| U5a-external-package | high | create: `metasystem/internal/validate/mutation_test_lookup.go#go-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationExternalTestPackage/U5a-external-package` | Reject the conventional external test package for the rule package. |
| U5a-fixture-script | high | create: `metasystem/internal/validate/mutation_test_lookup.go#fixture-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationSameScript/U5a-fixture-script` | Permit a fixture witness from a script other than the location path. |
| U5a-pipe | medium | `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths/U5a-pipe` | Permit the reserved literal pipe in a path; the claim syntax must reject it. |

Verification packages: `./internal/validate`. No shell syntax input.

### U5b. Recognize actual Go test declarations

Allocate 100 AST and import lines and 210 test lines, total 310.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/mutation_go_lookup.go","metasystem/internal/validate/mutation_go_lookup_test.go"]
Ceiling: 400
Rules: ["U5b-suffix","U5b-name","U5b-test-prefix","U5b-name-case","U5b-declaration","U5b-receiver","U5b-type-parameters","U5b-results","U5b-arity","U5b-pointer","U5b-type-T","U5b-import","U5b-alias","U5b-dot","U5b-platform","U5b-parse-error"]
Non-goals: No test execution in conformance, compiler, package loader, shell matcher, public activation, fixture execution or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Use go/parser and go/ast for the stated declaration contract. Arity counts parameters, not AST field nodes. Parseable invalid signatures fail membership. Malformed source is an inspection error. Do not compile or run tests in conformance. Platform-independent membership does not replace the builder and critic execution evidence.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U5b-suffix | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestDeclaration/U5b-suffix` | Remove the _test.go suffix requirement. |
| U5b-name | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestDeclaration/U5b-name` | Accept a different declared function from the exact returned name. |
| U5b-test-prefix | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestName/U5b-test-prefix` | Remove the Test prefix requirement. |
| U5b-name-case | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestName/U5b-name-case` | Remove the non-lowercase next-rune rule. |
| U5b-declaration | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestDeclaration/U5b-declaration` | Accept a comment or string mention as a declaration. |
| U5b-receiver | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature/U5b-receiver` | Remove only the receiver rejection. |
| U5b-type-parameters | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature/U5b-type-parameters` | Remove only the type-parameter rejection. |
| U5b-results | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature/U5b-results` | Remove only the result-list rejection. |
| U5b-arity | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature/U5b-arity` | Remove only the exactly-one-parameter check. |
| U5b-pointer | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature/U5b-pointer` | Accept testing.T by value or a variadic parameter instead of the required pointer AST node. |
| U5b-type-T | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature/U5b-type-T` | Accept *testing.M, including TestMain, instead of *testing.T. |
| U5b-import | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestImports/U5b-import` | Trust the spelling testing without resolving its import path. |
| U5b-alias | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestImports/U5b-alias` | Remove explicit import-alias support. |
| U5b-dot | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestImports/U5b-dot` | Remove dot-import support. |
| U5b-platform | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestPlatformSource/U5b-platform` | Filter out a build-tagged or OS-suffixed source before declaration inspection. |
| U5b-parse-error | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestParseFailure/U5b-parse-error` | Treat a parser error as successful membership. |

Verification packages: `./internal/validate`. No shell syntax input.

### U5c. Complete real lookup and its fixture owners

Allocate 110 production, 205 test and 75 fixture-function lines, total 390.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/mutation_fixture_lookup.go","metasystem/internal/validate/mutation_fixture_lookup_test.go","metasystem/internal/validate/mutation_test_lookup.go","metasystem/internal/validate/mutation_test_lookup_test.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go","metasystem/scripts/agents/conformance-fixtures.sh"]
Ceiling: 400
Rules: ["U5c-fixture-suffix","U5c-case","U5c-scenario","U5c-whole-line","U5c-literal","U5c-positive","U5c-quote-single","U5c-quote-double","U5c-lookup-call","U5c-absent","U5c-error","U5c-association","U5c-all-entries","U5c-complete"]
Non-goals: No shell interpreter, returned-command execution, trusted mutation runner, second source owner, public activation, fixture execution by the builder or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Compose U5a's candidate reads, U5b's Go matcher and the fixture matcher. Attach them to U4j's join for every valid entry. All production mutationViolations calls now use real lookup; only focused join tests inject a test lookup. Run the named tests directly on that complete check in a real candidate tree. Include same-id older-test deletion, both test kinds, parser errors and a selector-looking heredoc that is only a source-membership result.

Define bprm_absent_test_case and bprm_complete_case in the same bed. Their new_case names and assertions match Fixtures and Go proof. U4b adds their invocation sites. Their builder witnesses remove the actual validate predicates and observe the same-package Go failure. TestConformanceMutationEarlierTestDeleted implements numbered join clause 6. There is no partial public lookup phase.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U5c-fixture-suffix | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-fixture-suffix` | Remove the .sh suffix requirement. |
| U5c-case | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-case` | Remove the literal new_case source form. |
| U5c-scenario | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-scenario` | Remove the positive fixture_scenario source form. |
| U5c-whole-line | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-whole-line` | Permit a substring or a commented selector line. |
| U5c-literal | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-literal` | Permit a variable-expanded name as a literal selector. |
| U5c-positive | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-positive` | Permit a negative scenario comparison. |
| U5c-quote-single | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-quote-single` | Remove single-quoted literal-name support. |
| U5c-quote-double | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership/U5c-quote-double` | Remove double-quoted literal-name support. |
| U5c-lookup-call | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationViolations` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationAbsentTest/U5c-lookup-call` | Remove the named-test lookup loop. |
| U5c-absent | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-result` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationAbsentTest/U5c-absent` | Ignore a false membership result; the joined check must name the absent test; U4b proves the public refusal. |
| U5c-error | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-result` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationUnreadableTest/U5c-error` | Convert a Git or parser inspection error into a successful joined check. |
| U5c-association | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-result` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationLocationMismatch/U5c-association` | Ignore a failed package or script association. |
| U5c-all-entries | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-all-accepted-entries` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationEarlierTestDeleted/U5c-all-entries` | Check only one or the newest entry per id; deleting an earlier named test must refuse. |
| U5c-complete | high | create: `metasystem/internal/validate/conformance_mutations.go#successful-join` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationComplete/U5c-complete` | Refuse a completely covered chain whose Go and fixture witnesses both exist. |

Verification packages: `./internal/validate`. Shell syntax input: `scripts/agents/conformance-fixtures.sh`.

### U4b. Enforce the complete join and prove DONE

Allocate 55 review and bed-call lines, 165 validate test lines and 170 CLI test lines, total 390.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go","metasystem/cmd/metasystem/validate_verbs.go","metasystem/cmd/metasystem/conformance_mutations_test.go","metasystem/scripts/agents/conformance-fixtures.sh"]
Ceiling: 400
Rules: ["U4b-call","U4b-empty-followup","U4b-before-artifacts","U4b-bounds","U4b-immutable","U4b-repeat","U4b-named-repeat","U4b-cli-missing","U4b-cli-absent","U4b-cli-complete","U4b-cli-no-rows","U4b-four-legs"]
Non-goals: No new source or test matcher, weakened earlier checks, test-runner/group change, merge or recertification policy, return rewriting, fixture execution by the builder or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

This is the enforcing unit and the final landing. Add the complete mutationViolations call beside the sibling unit 4c's bounds policy collection. Keep all applicable diagnostics before persistence, including on changed repeats. There is no flag, placeholder lookup or bypass. Add the four bed-function calls and four public CLI Go cases. Share a case constructor and assertion table to keep their production proof inside the ceiling. The CLI rows mutate only the owned CLI status/output boundary; the validate rows mutate only review or mutation policy. Do not mutate a dependency outside Boundary.

Use real controller/worktree fixtures and the current admitted records. The CLI cases capture stdout, stderr and exit status, require both artifact outcomes and check return byte identity. Run without a critic record. Also prove bound-plus-mutation coexistence, malformed admitted evidence propagation and unchanged repeated review. The seat executes the candidate engine's four actual bed legs and the supplemental return-schema and kit proof before landing. TestConformanceMutationFourOutcomes drives all four inputs through public Conformance. The corresponding individual helper witnesses remain in U4j and U5c; this integration witness never replaces their observations.

The landing message carries the exact Wido bootstrap sentence and the joined earlier manual records, then names the final tree and public proof. Only this landing meets DONE. Failed fixture proof keeps U4b unlanded and returns a bounded correction to its owner. It does not request another prose round.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U4b-call | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationReviewGate/U4b-call` | Remove the mutationViolations call from reviewStage. |
| U4b-empty-followup | high | create: `metasystem/internal/validate/conformance_mutations.go#required-rule-union` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationEmptyFollowUp/U4b-empty-followup` | Let an empty follow-up reset earlier obligations. |
| U4b-before-artifacts | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRefusalHasNoArtifacts/U4b-before-artifacts` | Move successful persistence ahead of mutation validation. |
| U4b-bounds | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationAndBoundsRefusals/U4b-bounds` | Short-circuit one policy violation list when the other also fails. |
| U4b-immutable | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReviewImmutable/U4b-immutable` | Overwrite previous successful review evidence on a changed repeat. |
| U4b-repeat | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReviewRepeat/U4b-repeat` | Refuse an identical successful invocation instead of reusing its evidence. |
| U4b-named-repeat | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationChangedReviewNamesFailure/U4b-named-repeat` | Suppress the mutation diagnostic when immutable review evidence also causes refusal. |
| U4b-cli-missing | high | `metasystem/cmd/metasystem/validate_verbs.go#runValidateConformance` | `metasystem/cmd/metasystem/conformance_mutations_test.go#TestConformanceMutationCLIMissingRow/U4b-cli-missing` | Suppress the mutation-refusal exit status at the CLI; R2 must be named and both success artifacts absent. |
| U4b-cli-absent | high | `metasystem/cmd/metasystem/validate_verbs.go#runValidateConformance` | `metasystem/cmd/metasystem/conformance_mutations_test.go#TestConformanceMutationCLIAbsentTest/U4b-cli-absent` | Suppress the absent-test diagnostic at the CLI; R1, TestMissing and its path must be named. |
| U4b-cli-complete | high | `metasystem/cmd/metasystem/validate_verbs.go#runValidateConformance` | `metasystem/cmd/metasystem/conformance_mutations_test.go#TestConformanceMutationCLIComplete/U4b-cli-complete` | Suppress successful conformance output at the CLI; reviewedTree and both artifacts must be present and the return unchanged. |
| U4b-cli-no-rows | high | `metasystem/cmd/metasystem/validate_verbs.go#runValidateConformance` | `metasystem/cmd/metasystem/conformance_mutations_test.go#TestConformanceMutationCLINoRows/U4b-cli-no-rows` | Replace the successful CLI status with refusal for the legacy input; empty and omitted Rules must both pass unchanged. |
| U4b-four-legs | high | `metasystem/internal/validate/conformance.go#reviewStage` | `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationFourOutcomes/U4b-four-legs` | Remove the full mutation gate; the missing-row and absent-test legs must lose their expected refusals while complete and no-row inputs remain accepted. |

Verification packages: `./internal/validate ./cmd/metasystem`. Shell syntax input: `scripts/agents/conformance-fixtures.sh`.

## Acceptance-row accounting

Each independently removable requirement has a separate row. Required entry fields, required test fields, scalar types, object closures, each pattern application, signature predicates and path predicates remain separate. The five kit rows name the command, regeneration hint, exact pin, historical omission and other-role behavior independently in both U1 and U2. A fresh U2 run is required even when the test code is reused. The temporal clauses name separate declaration-time, historical-unknown, re-declaration, freshness, recovery and final-coverage witnesses. Every table witness has an explicit row subtest.

Shared fixture constructors and enclosing test functions reduce repetition. They do not permit a representative failure for a group. The actual return uses its implemented location and enclosing test function; its evidence selects the named row subtest. If code adds an independent rule not listed here, report a brief gap. The seat adds its id and witness within the unit ceiling or rebriefs a mechanical split. No silent bundling is allowed.

The seat retains the four launch-compatible tables and all later top-level returns. For each pre-enforcement unit, its landing message records the row join listed above. U4b's proof binds those records to the completed feature; it never certifies a historical return as having passed a gate that did not exist. The goal record's Next step carries the same Wido sentence. That ledger write belongs to the seat, not this prose-only delegate.

## Seat verification and remaining proof

After U1 and U2, apply the exact returned kit patch to the disposable integration candidate. Run the five independent builder witnesses, materialize canonical implementer v2 into a temporary file, compare it byte-for-byte with the pin, and inspect the other-role command recordings. Retain the unchanged historical input and the extractor's empty violation list. The seat also executes the real kit and historical extractor bed. From the full repository integration root, after building that candidate's engine, the exact commands are:

```sh
bash benchmark/validate-kit.sh
bash benchmark/extractor-fixtures.sh
```

The first exercises the production drift loop; the second proves the historical return through the full extractor fixture. They supplement the builder's focused Go witnesses. Stop on a failed command, retain its output, and return the failure to the owning unit. A syntax check is not kit proof.

The configured extra suite is `../benchmark/evidence-drift-fixtures.sh` (`metasystem/metasystem.conf:102-105`). It checks mission-state evidence, not the implementer pin (`benchmark/evidence-drift-fixtures.sh:35-69`). It therefore remains a separate regression run. From `metasystem/`, the exact enrolled-engine selection is:

```sh
bin/metasystem test run --root . --goal builder-proves-each-rule-by-mutation --mode canary --purpose diagnostic --groups section/project-extra-suites --json
```

Before U4b lands, run all four DONE legs and the supplemental return-schema scenario through the candidate engine. From `metasystem/`, use:

```sh
bin/metasystem test run --root . --goal builder-proves-each-rule-by-mutation --mode canary --purpose diagnostic --groups section/return-schema-fixtures,section/conformance-fixtures --json
```

These existing groups are declared at `metasystem/testing.json:75`, `metasystem/testing.json:79` and `metasystem/testing.json:84`. Selection flags are at `metasystem/cmd/metasystem/test.go:148-183`. The public conformance command runs from the target checkout; the implementer checkout is refused (`metasystem/internal/validate/conformance.go:227-230`). Inspect exact statuses, row/test diagnostics, both review artifacts and return byte identity. Keep explicit proof for the instruction-resource Go packages too.

The seat then consumes the goal's risk-selected delivery proof with test plan, test run and test verify. Diagnostic evidence alone is not delivery authorization. The completion contract is `metasystem/docs/project-rules.md:17` and `metasystem/docs/design/design-obligation-gate.md:15-17`. Every unit also has its implementation code read. Read the actual builder observations and the critic's fixed sample record. Schema validity and source membership do not establish causal failure.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| BPRM-SOURCE | HIGH | Shared header grammar | Rules extend the one admission and authenticated record; explicit round; legacy has no rows | dispatch parser/codec/writer and validate sibling reader extension | U0, on sibling 1a/3a/3b/3c | TestBriefRulesAdmission, TestBriefRulesRecord, TestReadRoundBriefRules and independent row subtests | TestDispatchToReviewAdmittedRules initial/follow-up inline/reference cases | PARTIAL | Build U0 and retain its manual join |
| BPRM-SHAPE | HIGH | Return member | Required provider mutations; optional historical canonical member; legal location forms | returnschema and returnChecker | U1 and U2 | Every schema row; TestMutationLocationForms; preservation tests | Complete, malformed and old return-schema scenario | PARTIAL | Build U1/U2 and retain manual joins |
| BPRM-KIT | HIGH | Benchmark integration | Canonical drift and hint, byte-identical pin, old omission, other roles unchanged | benchmark, integrated by seat | U1/U2 kit patches | Five rows per unit in TestBPRMKit functions | validate-kit.sh, extractor-fixtures.sh and separate exact extra-suite selection | PARTIAL | Apply and prove each exact combined candidate |
| BPRM-CHAIN | HIGH | Numbered join | Declarations bind entry time; repairs re-declare and refresh; final union survives | validate mutation join | U4a and U4j | One named test per temporal clause and guard | Missing-row and no-row legs in U4b | PARTIAL | Build collection and join with own code reads |
| BPRM-TEST | HIGH | Candidate lookup | Every valid claim names an associated test in the final tree | validate file, Go and fixture lookup | U5a/U5b/U5c | Each path/signature/selector row; TestConformanceMutationEarlierTestDeleted | Absent-test and complete legs in U4b | PARTIAL | Build actual lookup before activation |
| BPRM-HONESTY | HIGH | Role and critic wording | Complete builder battery, honest repairs, deterministic critic reproduction | role, requirements, templates, repair prompts and critic skill | U3a/U3b | Colocated wording, actual composition and actual prompt output tests | Builder returns and critic sample observations | PARTIAL | Land before U1 and retain manual joins |
| BPRM-ENFORCEMENT | HIGH | Review persistence | Full check before success artifacts; bounds coexist; immutable repeat | reviewStage and public CLI | U4b after sibling 4c | TestConformanceMutationFourOutcomes and four named CLI tests; coexistence/repeat cases | All four DONE legs and review artifact readback | PARTIAL | Run candidate engine and land U4b only on proof |
| BPRM-BOOTSTRAP | HIGH | Wido sentence | Every unenforced unit has a recorded manual join; DONE at enforcing landing | seat | Ten unit landing messages and goal Next step | Each owning unit's named witness table; U4b public gate test | Final tree and retained bootstrap joins bound in U4b landing message | PARTIAL | Seat records each join and the exact sentence |

## Critique record

Round 1 reported seven material and three non-material findings. Round 2 reported five material and one non-material finding. It kept BPRM-R1-001, 002, 003, 005 and 007 open. All five are explicitly closed as prose decisions below and remain named implementation obligations. The seat's rulings settle the contract choices. No disposition claims passing runtime proof or an independent third approval.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BPRM-R1-001 | accepted, reopened, folded | The current create marker classifies authority references (`metasystem/internal/dispatch/brief.go:155-167`). The sibling revision 3 owns admission, not Rules. | U0 extends sibling units 1a/3a/3b/3c. TestBriefRulesAdmission and TestDispatchToReviewAdmittedRules prove the extension. Every unit has JSON arrays and a decimal Ceiling. Closed with BPRM-R2-002. |
| BPRM-R1-002 | accepted, reopened, folded | Current review judges a candidate tree and aggregates chain returns (`metasystem/internal/validate/conformance.go:368-392`, `metasystem/internal/validate/conformance.go:724-817`). The old design flattened time. | U4j's numbered clauses and TestMutationDeclaredByEntryRound, TestMutationFutureDeclarationDoesNotReviveEntry, TestMutationRepairRedeclaresID, TestMutationRepairNeedsFreshEntry and TestMutationFinalTreeCoverage settle time. U5c retains TestConformanceMutationEarlierTestDeleted. Closed with BPRM-R2-004. |
| BPRM-R1-003 | accepted, reopened, folded | successorTaskDirection can return the entire prompt (`metasystem/internal/validate/conformance.go:942-954`). The final sibling uses an admission record instead (`artifacts/reports/sibling-bdrb-design-r3.md:137-147`). | U0's exact ReadRoundBriefRules wrapper calls ReadRoundBriefBounds. TestReadRoundBriefRules, TestReadRoundBriefRulesExplicitRound, TestReadRoundBriefRulesFailure and TestReadRoundBriefRulesLegacy prove it. No prose fallback. Closed with BPRM-R2-002. |
| BPRM-R1-004 | accepted, retained | The bed's behavioral owner invokes review and inspects status and artifacts (`metasystem/scripts/agents/conformance-fixtures.sh:153-186`). | U4j and U5c own the four leg bodies and production mutation witnesses. U4b owns public activation, all four CLI tests and TestConformanceMutationFourOutcomes. The seat runs every leg before that enforcing landing. |
| BPRM-R1-005 | accepted, reopened, folded | Independent schema requirements are checked separately (`metasystem/internal/returnschema/returnschema_test.go:199-231`). Missing kit rules were still outside the inventory. | Each table cell now names its row subtest. U1 and U2 each have five kit rows. TestBPRMKitCanonicalCommand, TestBPRMKitOtherRoles, TestBPRMKitPin and TestBPRMKitHistoricalOmission are builder witnesses with separate observations. Closed with BPRM-R2-005. |
| BPRM-R1-006 | accepted, retained | Membership cannot prove causal execution. The current independent critic attacks named tests (`metasystem/skills/code-critique/SKILL.md:12`, `metasystem/skills/code-critique/SKILL.md:25-37`). | U3b retains the exact sample size, severity/id order, every-entry rule, environment and shell execution limits, and material non-reproduction. Each clause has its own TestMutationCriticContract row subtest. |
| BPRM-R1-007 | accepted, reopened, folded | Launch materializes a closed old schema before U1 edits it (`metasystem/scripts/agents/adapters/runtime-common.sh:103-110`; `metasystem/scripts/agents/schemas/implementer.schema.json:5-8`). More than U1 precedes enforcement. | The header and bootstrap table name all ten unenforced units and their manual landing joins. The exact Wido sentence is queued for the goal owner. U4b's full gate and TestConformanceMutationFourOutcomes close the bootstrap at DONE, without rewriting old returns. Closed with BPRM-R2-003. |
| BPRM-R1-008 | noted, non-material | Frozen fields, direct-byte tests, adjudication and telemetry remain compatibility consumers (`metasystem/scripts/validate-metasystem.sh:1661-1706`; `metasystem/internal/validate/returncomplete_direct_test.go:31-121`; `metasystem/internal/adapter/adjudicate_test.go:205-271`; `metasystem/scripts/agents/telemetry-census-fixtures.sh:48-72`). | Retained in the reader inventory. No new behavior follows from this finding. |
| BPRM-R1-009 | noted, non-material | No implementation diff exists; prose allocations are estimates. | All unit allocations stay at or below 400 and the concrete-diff stop remains. Added kit proof and temporal work have explicit allocations. |
| BPRM-R1-010 | noted, non-material | const exists in the supported subset (`metasystem/internal/validate/returncomplete.go:421`). Snapshot uses an isolated index and add -A (`metasystem/internal/gittree/gittree.go:255-276`). | Those facts retain current anchors. There is no host-installed-Go citation. |
| BPRM-R2-001 | accepted, folded | The validator compiles and applies Go patterns (`metasystem/internal/validate/returncomplete.go:474-479`, `metasystem/internal/validate/returncomplete.go:525-527`). The old escaped bar was wrong. | One location pattern uses alternation. U2-pattern-location names TestMutationLocationForms with both legal forms and the former misspelling. Its own corrected schema is also regenerated into the canonical kit pin. |
| BPRM-R2-002 | accepted, folded | The final sibling assigns parser, record codec, reader and persistence to 1a, 3a, 3b and 3c (`artifacts/reports/sibling-bdrb-design-r3.md:317-391`). It defines no Rules parser. | This goal owns U0. Its exact signatures extend the existing admission scan and record and call the exact sibling ReadRoundBriefBounds. Persisted Rules share admittedSha256 and recordSha256. The U0 admission, record, reader, legacy and actual command seam tests discharge this and reopened R1-001/R1-003. |
| BPRM-R2-003 | accepted, folded | The existing review success path has only boundary enforcement (`metasystem/internal/validate/conformance.go:392-425`). Shape support is not review enforcement. | Every pre-enforcement unit has its own manual join and landing record. Honest repair lands before the provider requires mutations. U4b is the first complete public gate and the final unit. The goal owner carries the exact Wido sentence. U4b's four-outcome and CLI tests close this and reopened R1-007. |
| BPRM-R2-004 | accepted, folded | The old union loses entry time (`metasystem/internal/validate/conformance.go:754-805`). The seat specifies entry-time validity and fresh repairs. | Numbered join clauses 1-10 each name tests. U4j owns temporal admission, unknown retention, re-declaration, freshness, recoverability and final coverage. U5c owns all-entry final-tree lookup. This closes reopened R1-002. |
| BPRM-R2-005 | accepted, folded | The drift loop compares materialized v2 bytes (`benchmark/validate-kit.sh:84-102`), and stored implementer evidence uses the pin (`benchmark/extractor.py:514-538`). The configured extra suite only checks mission state (`benchmark/evidence-drift-fixtures.sh:35-69`). | Five independent U1 and U2 kit rows have builder-run Go witnesses, including exact command and hint, byte identity, historical omission and other roles. Seat verification names the actual kit/extractor commands and the exact separate extra-suite command. The combined ceiling includes this proof. This closes reopened R1-005. |
| BPRM-R2-006 | noted, non-material | Fake directly constructs a v2 return and does not source runtime-common.sh (`metasystem/internal/adapter/fake.go:53-112`; `metasystem/scripts/agents/adapters/fake.sh:140-160`). | The inventory states that distinction. Sibling citations all name the supplied final revision 3. No separate behavior change follows from the citation corrections. |

All twelve material finding ids have dispositions and named implementation witnesses. All four non-material ids are retained. The five reopened round-1 items are joined to their round-2 closures. Runtime obligations remain PARTIAL until their owning unit supplies actual proof and receives its code read. The prose loop ends here under R-97-m1e (`metasystem/memory/rulings.md:156`) and D81 (`metasystem/docs/reviews/2026-08-13-delegated-decisions.md:715-721`). There is no third critique round.

Design verification: read both revisions, both critique bodies, the goal, the supplied final sibling and the relevant current source, readers, validators and fixture owners. Checked the header and table joins, unit ownership, proposed signatures, allocation totals and disposition ids. A Python regex consistency check accepted both legal location examples and rejected the former misspelling; the named Go tests remain implementation obligations. This is the requested new replacement page. No code or tracked file was edited by this delegate and no commit was made.

No implementation test, fixture bed, mutation experiment or model reproduction ran for this prose deliverable. Bad admitted evidence and unreadable tests refuse; missing or stale claims name their rule or test; a failed sampled reproduction remains material. The residual risk is explicit: mechanical source membership cannot establish causality, and the critic samples rather than repeating every builder run. Concrete unit sizes and runtime proof remain for implementation. No open question belongs to Wido.

Proposed receipt for the integrating seat: `DESIGN builder-proves-each-rule-by-mutation revision 3: folded five material round-2 findings and all five reopened round-1 findings; U0 extends sibling revision-3 admission record and reader; temporal join, atomic kit witnesses, explicit manual bootstrap and full enforcing final unit; prose loop closed under R-97-m1e and D81; evidence=read; runtime-proof=pending`.
