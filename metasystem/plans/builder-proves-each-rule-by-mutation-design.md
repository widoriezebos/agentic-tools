# builder-proves-each-rule-by-mutation

- Owner: m1e (goal 29 of plans/delivery-efficiency-plan.md). **Revision 2, 2026-09-14**, folded by the Codex gpt-6-astra design delegate from critique round 1 (records/misc/builder-proves-each-rule-by-mutation-critique-r1.md, verdict rework, seven material findings, all accepted) under seven seat rulings; the seat integrated the page and edits only this header block.
- Goal and current status: the builder returns one observed mutation claim per acceptance rule declared in the brief's Rules header; review-stage conformance joins the union of the chain's declared rules to the accepted claims and checks every named test against the final candidate tree; the critic reproduces a deterministic sample. Eleven builder units of at most 400 changed lines each (U1, U2, U3a, U3b, U4a, U4b, U5a, U5b, U5c, U6a, U6b), each with its own Boundary, Ceiling, Rules, Non-goals and the proof rule. Nothing built yet; every unit lands after brief-declares-the-round-boundary's units 1a, 1b and 3, which own the shared header parser and current-round source reader.
- In flight right now: revision 2 under critique round 2 (Codex gpt-5.6-sol, design critic), the last round inside this tier-2 goal's review budget; a clean round closes the design, a material round folds once more and then escalates under R-97-m1e.
- Decisions made (and who made them): Wido, 2026-09-14: the mutation battery moves from the read to the builder's return; units are at most 400 changed lines. The seat, 2026-09-14, on critique round 1: the sibling's header grammar (JSON arrays for Boundary and Rules, a bare decimal Ceiling) and its source reader are reused, not duplicated; review joins the whole chain, not the current round; one row per independently removable rule with its own observed failure line; the named test must live in the rule's package or script; the critic's sample is at least three rows or the square root of the row count rounded up, highest severity first then ascending id, and a sampled claim that does not reproduce is material; the U1 bootstrap (its mutation table travels in evidence because the schema does not exist before U1 lands) is the one accepted exception, read by the seat by hand and closed by U2's first acceptance row.
- Waiting on the human: nothing.
- Dead ends (do not retry without new evidence): comma-separated headers; create: markers inside Boundary; a Ceiling with units; a second composition snapshot of Rules; successorTaskDirection for header selection; current-round-only coverage; representative failure lines for several rules; fixture registration as behavioural proof.
- Next step: revision 2 lands with this line, so critique round 2 has a landed subject; on a clean round, and once the sibling's units 1a, 1b and 3 are landed, U1 (bootstrap the compatible return shape) is briefed to Codex gpt-5.6-sol with its own Boundary, Ceiling, Rules and proof rule, read by Opus, landed by the seat, U2 follows immediately to close the bootstrap, and the remaining units follow in the stated order.

The DONE sentence is the contract in `metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:8`. It requires mutation entries, builder instructions, review refusals that name missing rows or tests, critic spot checks, and four behavioral fixtures. The goal is approved and has a two-round review budget (`metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:3`, `:14`). This fold does not claim independent approval or reopen that budget.

Evidence level: read. Code was checked in this worktree at `5cc59746a539b5af08932a7b2558b3676e74a502`. The sibling page on local trunk still says Revision 1 (`metasystem/plans/brief-declares-the-round-boundary-design.md:3`). Its forthcoming changes are prerequisites specified by the seat, not claims about shipped code. All paths below are repository-relative. Commands run from `metasystem/` unless stated otherwise. New functions, tests, and messages below are proposed contracts.

## Facts that determine the design

| Current behavior | Evidence and consequence |
| --- | --- |
| The shipped implementer schema is frozen version 1 and closes its root object. | `metasystem/scripts/agents/schemas/implementer.schema.json:3-8`. Leave that file unchanged. |
| Version 2 derives identity fields from version 1. Materialization and completeness each invoke the derivation. | `metasystem/internal/returnschema/returnschema.go:34-74`, `:154-187`; `metasystem/internal/validate/returncomplete.go:161-214`. Both callers need the implementer overlay. |
| Launch materializes implementer version 2 before the builder runs. | `metasystem/scripts/agents/adapters/runtime-common.sh:103-110`. This causes U1's accepted bootstrap exception. |
| The provider-schema test requires every object to be closed and every property to be required. | `metasystem/internal/returnschema/returnschema_test.go:176-265`. Provider output requires `mutations`, including an empty array. |
| The schema validator supports type, enum, const, properties, required, additionalProperties, items, and pattern. | `metasystem/internal/validate/returncomplete.go:419-424`. Use this subset. Add no schema engine or dependency. |
| The brief parser currently owns mode and authority paths. Its create prefix only marks output references. | `metasystem/internal/dispatch/brief.go:41-104`, `:155-167`. Use the sibling's new header family; do not reinterpret create syntax as Boundary. |
| Composition selects raw caller brief bytes as task-direction, then appends recipe sources and continuations. | `metasystem/internal/dispatch/composition.go:237-283`, `:300-319`. Rules must come from that selected source. |
| Composition records source ranges, original byte counts, and both original and delivered digests. Oversized bodies become references. | `metasystem/internal/dispatch/composition.go:52-69`, `:366-401`. The source reader must distinguish original bytes from rendered section bytes. |
| Root and follow-up publication retain prompt and composition artifacts. Root publication also retains brief.md. | `metasystem/scripts/agents/dispatch.sh:1918-1929`, `:2883-2888`. Reuse these files. |
| The exhaustion reader returns the entire prompt when there is no task-direction reference. | `metasystem/internal/validate/conformance.go:930-960`. Do not reuse it for Rules. |
| Review resolves the implementer without requiring a critic, snapshots the candidate, checks boundaries, and writes success artifacts. | `metasystem/internal/validate/conformance.go:139-249`, `:355-428`. Mutation enforcement belongs before those writes. |
| Cumulative boundary inspection raises its round limit from chain job records, then reads numeric round directories and unions returns. | `metasystem/internal/validate/conformance.go:724-817`. It is not limited to the supplied round number. Match this chain convention explicitly. |
| Snapshot uses an isolated index, read-tree, and git add -A. Literal tree entries and blob reads are already available. | `metasystem/internal/gittree/gittree.go:246-276`, `:450-499`. Inspect the supplied reviewed tree, including uncommitted and unignored new files. |
| Nested project declarations strip the repository installation prefix exactly once. Delegate changes outside the project refuse. | `metasystem/internal/validate/conformance.go:276-293`, `:311-334`. Reuse projection; keep benchmark integration at the seat. |
| Acceptance normalizes the selected object and can rewrite a lawful boundary. Both repair prompts are separate owners. | `metasystem/internal/adapter/return.go:37-94`, `:191-225`; `metasystem/internal/validate/returncomplete.go:239-303`; `metasystem/internal/adapter/adjudicate.go:65-97`. Preserve evidence and prohibit invented observations in both prompts. |
| Requirements use capability names, while packet recipes can deliver a file source. | `metasystem/internal/capability/select.go:82-102`; `metasystem/scripts/agents/roles/implementer.requirements.json:2-15`; `metasystem/internal/dispatch/composition.go:275-283`. Add a proof string, not a capability named mutations. |

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
| location | `^[^\r\n]+(:[1-9][0-9]*\|#[^\r\n\s][^\r\n]*)$` |
| test.kind | Enum `go`, `fixture`. |
| test.path | `^[^\r\n]*\S[^\r\n]*$` |
| test.name | `^[^\r\n]*\S[^\r\n]*$` |
| failureLine | `^[^\r\n]*\S[^\r\n]*$` |

The location names a repository-relative file and either a positive one-based line in the restored candidate or a symbol after `#`. Split at the final `#` when present, otherwise at the final colon introducing decimal digits. Resolve its path at review as specified below. The critic checks the symbol or line and the claimed removal. A compile error, timeout, skip, or setup error is not an observed failure of the rule.

Keep the existing evidence object `{command, observed, level}`. Its owner is `metasystem/scripts/agents/schemas/implementer.schema.json:22-33`; the template explicitly preserves it at `metasystem/scripts/agents/templates/brief.md:34`. Record the passing baseline, isolated removal, failing run, restoration, and restored pass there. Each mutation entry contains one actual diagnostic for that row alone. Several rows may name one enclosing Go test, with separate subtests and separate removal runs. There is no representative diagnostic for a group of constraints.

Add `MutationTest`, `MutationEntry`, `MutationEntriesSchema`, `ImplementerVersionTwo`, and `CanonicalImplementerVersionTwo` in `metasystem/internal/returnschema/implementer.go`. The two structs mirror the JSON member names. `MutationEntriesSchema() map[string]any` owns the one fragment. Both overlays take and return `map[string]any` with an error, like VersionTwo. The provider overlay calls VersionTwo, installs the fragment, and requires mutations. The canonical overlay installs the identical fragment but omits mutations from the root required list.

`Materialize` selects the provider overlay only for implementer version 2. `returnChecker.checkReturn` selects the canonical overlay only for implementer version 2. Old omitted members pass without insertion. Present malformed members fail. Version 1 and other roles keep their contracts. Review, rather than acceptance, supplies the brief-dependent coverage check.

Expose the canonical form through `schema materialize --canonical`, accepted only with `--role implementer --version 2`. Other combinations exit 2 with `--canonical is only available for implementer version 2`. Add `MaterializeCanonicalImplementer(root, outputPath string) error` and share the existing encoding body between materializers. The CLI owner is `metasystem/cmd/metasystem/schema.go:13-46`.

The benchmark pins default materialized version 2 today, then validates historical returns against it (`benchmark/validate-kit.sh:84-102`; `benchmark/extractor.py:324-341`, `:514-538`). In U1 the seat regenerates `benchmark/schemas/evidence/implementer.schema.json` with the canonical materializer and changes only the implementer branch of the drift command and its regeneration hint. The pin remains optional at its root. Historical omission is required by `benchmark/extractor-fixtures.sh:260-274`. The extractor already supports optional declared properties and patterns (`benchmark/extractor.py:63-118`), so it needs no code change. Regenerate the pin again when U2 adds text constraints. No benchmark scoring change is part of this goal.

The builder supplies each exact benchmark patch in `metasystem/artifacts/reports/bprm-kit-compat.patch`; it does not edit outside the project. The seat applies it with the matching engine change in the same landing. Generated pin lines count toward the unit's 400 lines. Patch transport bytes do not count a second time as shipped source. The seat checks the concrete combined diff before landing. No cross-project engine dependency is added; the kit's dependency direction is explicit at `benchmark/validate-kit.sh:2-6`.

### Shared header grammar and current-round source

Every dispatched unit has one physical line per header:

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance_mutations.go"]
Ceiling: 400
Rules: ["R1","R2"]
```

Boundary and Ceiling mean exactly what the sibling specifies. Rules extends the same header family in `metasystem/internal/dispatch/brief.go`. The sibling's unit 1 owns that family and its admission calls. It must expose Rules as an optional JSON array of unique ids using the ruleId grammar. Absence and `[]` both declare no rules. A blank value, null, wrong JSON type, non-string member, duplicate header, repeated id, or invalid id refuses. Preserve array order. Recognize the exact case-sensitive prefix at column zero and trim the value. Indent examples embedded in actual briefs. There is no comma-list parser and no second pass over the assembled packet. These are integration requirements for the sibling fold; this goal does not implement another header parser.

The sibling's unit 3 owns the round-parameterized source reader in `metasystem/internal/validate/brief_bounds_source.go`. Boundary, Ceiling, and Rules consume its same result through the same parser. Use its landed names and result type. It must accept an explicit round, so chain collection can call it once for each round without changing which job a review is judging. These source rules bind that shared owner:

1. With composition, select exactly one `task-direction` source whose source is `caller:brief`. Validate the delivered byte range and DeliveredDigest against prompt.md. Do not scan another slot.
2. For an inline source, recover the original body using SourceBytes and the renderer's framing, then verify SourceDigest. The renderer adds a heading and newline framing (`metasystem/internal/dispatch/composition.go:366-390`); do not trim authored content to guess its digest.
3. For a referenced source, require exactly one reference joined by slot, `reference.Digest == source.SourceDigest`, and `reference.Bytes == source.SourceBytes`. Verify the reference with `dispatch.ReadVerifiedReference`. That function checks regular-file status, location, bytes, and digest (`metasystem/internal/dispatch/references.go:24-49`). Matching only the slot is insufficient. Do not parse the reference stub.
4. Without composition, use the retained chain-root brief.md as the shared legacy fallback. This is the only fallback, including for a legacy follow-up with no composition. Do not scan its prompt.md, which may embed older instructions. Missing root brief means no declarations. Other read failures or malformed present metadata refuse. This compatibility choice belongs to the sibling reader for all three headers. Normal composed follow-ups always use their own selected source.

Do not add `acceptanceRules` to CompositionRecord, another brief snapshot, or an artifact schema. Do not call or modify successorTaskDirection. The sibling's planned bounds reader is documented at `metasystem/plans/brief-declares-the-round-boundary-design.md:67-79`, `:270-282`; the seat's revision-2 source-join ruling supersedes its broader legacy prompt fallback.

An acceptance row is an id in Rules with a matching prose row. The row states one independently removable rule and its severity. Use critical, high, medium, or low. Missing severity defaults to high for sampling; it is not another schema member. A correction that repairs an earlier rule re-declares the same id. It returns a new observation for that id. A new id denotes a new obligation. A rule does not disappear because a later brief omits it.

## Review check and test lookup

### Exact chain join

Add `conformance_mutations.go` in the validate package. Its `mutationViolations(reviewedTree string) []string` owns the join and returns the existing string violation form. The chain is rooted at the root job already resolved by conformance. Use `artifacts/agents/<rootJob>/rounds/<round>/` for both source and return artifacts.

Match cumulativeBoundaryViolations' existing round selection: start at the supplied job's round, raise the limit to the highest round on its recorded chain, and inspect numeric round directories at or below that limit. Read in numeric order. The existing selection is at `metasystem/internal/validate/conformance.go:724-768`. Extract that directory selection into one local helper for both consumers, preserving boundary behavior. Keep the helper's existing lexicographic order for the boundary caller; sort a copy numerically for mutation collection. Return read errors so the mutation caller can refuse while the boundary caller retains its existing handling. Do not choose one latest return and discard the others. Include source-bearing rounds even when their return is absent, as after a cap. A missing return contributes no entries, not an empty Rules declaration. Directory or present-file read errors refuse the mutation check. Keep the boundary owner's existing compatibility handling unchanged.

Let `S(r)` be the Rules array from round r's shared brief reader. Let `E(r)` be its present mutation array, or empty if the return or member is absent. Let `D = union S(r)`. Let `E` be the concatenation of all E(r), retaining round and array index as provenance.

The exact join is:

- Every id in D must have at least one accepted entry in E, as defined below. An entry may come from any round's return. Report a missing id with its earliest declaring round. An empty current Rules array cannot erase earlier D.
- Every accepted entry's id must belong to D. In its own return an id may occur at most once. Repeating an id across different round returns is lawful. An unknown id or duplicate id in the supplied return refuses with its own diagnostic. Historical unknown entries and every historical entry for a duplicated id remain rejected claims and provide no coverage. A later unique declared entry can repair their missing coverage. This avoids making a rejected claim an irreversible chain poison. No previously accepted entry is discarded by this rule.
- Every present array is shape-validated using MutationEntriesSchema before typed decoding. Do not call ReturnCompleteJob from review because it can normalize a return. Malformed evidence is not silently dropped in favor of a later claim.
- Each shape-valid, declared, within-return-unique entry is an accepted entry for lookup. **Every test named by every such entry must exist in the final reviewedTree and satisfy the location association.** Never choose only the newest entry or stop after the first existing test. Earlier valid claims remain obligations even after repair. A repair must retain earlier named tests or repair their implementation so those names remain valid.
- When D is empty, skip the mutation check without reading or rewriting returns for this purpose. Ordinary return acceptance and existing conformance still apply. This preserves historical no-row admission. An undeclared present entry in a nonempty chain refuses; this does not retrofit evidence validation into old zero-row chains.

Three follow-up cases are mandatory: round 1 lacks R1 and round 2 has only R2, so final review still names missing R1; round 2 supplies R1 and R2, so both can be satisfied from that return; round 1 supplies R1 and round 2 deletes its test, so final review refuses that old test even if round 2 also supplies another R1 entry. An earlier failed review neither certifies nor cancels an obligation.

Call the check in reviewStage beside cumulativeBoundaryViolations and the sibling's bounds check, before persistence. Append applicable policy violations so one does not mask another. Fact-read failures still stop safely. On first refusal write neither success artifact. On a changed repeat, retain the old artifacts, report mutation violations and the existing immutable-review diagnostic, and exit 1. Identical successful repeats reuse the old bytes. Today's immutable artifact handling is at `metasystem/internal/validate/conformance.go:339-365`, `:392-425`. The successful artifact stays the existing three-field shape at `metasystem/internal/validate/conformance.go:403-407`. No return is repaired or rewritten by this check.

Use stable lines with quoted ids, paths, and test names:

```text
conformance failure: MUTATION_MISSING: acceptance row "R1" declared in round 1 has no mutation entry in the chain
conformance failure: MUTATION_TEST_ABSENT: round 1 acceptance row "R1" names absent go test "TestMissing" in "example_test.go"
conformance failure: MUTATION_LOCATION_MISMATCH: round 1 acceptance row "R1" test "TestPresent" in "other/example_test.go" does not belong with location "rule.go#check"
conformance failure: MUTATION_DUPLICATE: round 2 acceptance row "R1" has more than one mutation entry in that return
conformance failure: MUTATION_UNKNOWN: round 2 mutation entry names undeclared acceptance row "R9"
conformance failure: MUTATION_INVALID: round 1 $.mutations[0].failureLine is required
conformance failure: MUTATION_BRIEF_INVALID: round 1 <shared reader or parser diagnostic>
conformance failure: MUTATION_TEST_UNREADABLE: round 1 acceptance row "R1" cannot inspect test "TestPresent" in "example_test.go": <cause>
```

Use MUTATION_INVALID for nonobject or unreadable present returns, with the round and cause. Required-field detail reuses `metasystem/internal/validate/returncomplete.go:531-538`. Emit source/shape errors in numeric round order, missing ids by earliest declaration order, and entry failures by round and array index. Exit 1 for these refusals. CLI usage remains exit 2 (`metasystem/cmd/metasystem/validate_verbs.go:276-299`, `:320-352`).

### Candidate lookup and location association

Add `mutation_test_lookup.go` and `mutation_go_lookup.go` in validate. Pass the full entry and the already computed reviewedTree. Add no snapshot, diff generator, test runner, compiler invocation, or execution of a returned command to conformance.

Both the location file and test path must be clean repository-relative slash paths. Reject an absolute path, NUL, backslash, line break, empty component, `.` component, or `..` component. Spaces are allowed. Apply projectDeclaration exactly once to each path. Read their exact entries from projectWorkspace using Entries and FileAt. Accept mode 100644 or 100755 only. A missing exact file, directory, symlink, or gitlink cannot satisfy the lookup. An absent test gets MUTATION_TEST_ABSENT; an absent or unrelated location gets MUTATION_LOCATION_MISMATCH. Git or source parsing errors get MUTATION_TEST_UNREADABLE. Never consult controller files, HEAD, or live workspace contents after the snapshot.

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

Add the same string as top-level `mutationProof` in implementer.requirements.json. Leave required, optional, and waivers unchanged. Add `{"slot":"mutation-requirement","path":"scripts/agents/roles/implementer.requirements.json"}` to the implementer recipe's sources. This uses its current file delivery loop (`metasystem/internal/dispatch/composition.go:275-283`). U1 first adds mutations to the role's member list. The instruction unit adds:

> For each id in this round's Rules JSON array, return one mutations entry with its ruleId, location, test kind, test path, test name, and observed failureLine. Pass the test, remove only that rule, observe its failure, restore the rule, and pass the test again. Keep every run in evidence. Re-declare an earlier id when repairing its rule. The chain retains earlier obligations and every earlier accepted test name. Return an empty mutations array when this round declares no rules. Put any unproved rule in gaps; never invent an observation.

The templates use a valid `Rules: []` example or an indented filled example. They explain that a dispatched brief replaces it with the exact id array and matching acceptance rows. They retain the sibling's Boundary and Ceiling guidance. Do not install live unfilled optional placeholders. A follow-up keeps the chain's return schema and re-declares repaired ids. Its current unchanged-schema contract is at `metasystem/scripts/agents/templates/follow-up.md:12-18`.

Append to both repair prompt writers:

> For an implementer return, copy only mutation entries and failure lines you already observed. Never invent a missing entry or turn a passing run into a mutation claim. Report an unproved rule in gaps. This reply-only repair does not authorize repeating the work.

Acceptance can complete delivery of a shape-valid return with gaps. Review can still refuse its missing rows. Keep ordinary post-repair validation, which already occurs at `metasystem/internal/adapter/adjudicate.go:208-246`.

In code-critique Layer 1, describe cumulative mutation coverage and test association alongside the sibling's limits. Keep the seat's pre-read conformance ordering. Add:

> Join all rounds' Rules and mutation entries. Let n be the number of distinct required ids. For n greater than zero, sample min(n, max(3, ceil(sqrt(n)))) rows. Sort by highest severity first, then ascending rule id. Use the highest declared severity of a repeated id; a row without severity ranks high. For each selected id reproduce every accepted entry for that id, in round order. Record the full ordered row inventory, the selected ids, and the observations. In a disposable copy of the exact reviewed tree, pass the named test, remove only the claimed rule, observe the claimed failure, restore the rule, and pass again. Normalize only incidental output prefixes such as temporary paths and line numbers when comparing the failure. The named assertion and cause must match. A sampled claim that does not reproduce is a material finding. A skipped test or unavailable required environment remains unproven and material. A fixture sample must enter the named leg in the script named by location; selector-looking data is not execution. Restore the disposable copy after every attempt. Expand the sample only when observed failure warrants more investigation; the required initial sample never shrinks or swaps rows.

Examples: n=1 samples 1, n=2 samples 2, n=9 samples 3, n=10 samples 4, n=100 samples 10. For n=0 no mutation sample is needed; ordinary critique still applies. This is a critic instruction, not a new automatic mutation service. The existing independent critic and materiality contracts remain at `metasystem/skills/code-critique/SKILL.md:12-23`.

## Fixtures and Go proof

The four DONE legs run through the real public `validate conformance --stage review` verb, without a critic record. Use the existing bed's controller/worktree helpers and status-plus-message checks (`metasystem/scripts/agents/conformance-fixtures.sh:38-96`, `:153-186`). The source test Go functions and the public CLI tests below exercise the same inputs and behavior. Removing a production check must break its behavioral test. Removing only a new_case declaration is never a mutation witness.

| Leg | Input and exact required behavior | Go witnesses |
| --- | --- | --- |
| bprm-missing-row | Legacy root brief declares `Rules: ["R1","R2"]`. A valid v2 return carries only R1. Review exits 1 with `MUTATION_MISSING: acceptance row "R2" declared in round 1 has no mutation entry in the chain`. Neither success artifact exists. | TestConformanceMutationMissingRow in validate; TestConformanceMutationCLIMissingRow in cmd/metasystem. |
| bprm-absent-test | Declare R1 and return a well-shaped entry. Its same-package location exists. example_test.go contains TestPresent, while the entry names TestMissing. Review exits 1 naming R1, TestMissing, and example_test.go. Neither success artifact exists. | TestConformanceMutationAbsentTest; TestConformanceMutationCLIAbsentTest. |
| bprm-complete | Declare R1 and R2. R1 names rule.go and TestPresent in example_test.go in the same package. R2 names checks.sh and its executable present-leg. Declare every changed path. Review exits 0, emits reviewedTree, and writes both artifacts. Read them back and keep return bytes identical. | TestConformanceMutationComplete; TestConformanceMutationCLIComplete. |
| bprm-no-rows | Use the old brief without Rules and an old return without mutations. Review exits 0, creates its artifacts, and preserves return bytes. A companion with `Rules: []` also passes. A separate follow-up test proves current empty Rules does not erase earlier rules. | TestConformanceMutationNoRowsUnchanged; TestConformanceMutationCLINoRows. |

The test repositories install at their root, so example_test.go has no metasystem prefix. The existing fixture builder establishes this layout (`metasystem/scripts/agents/conformance-fixtures.sh:38-64`). Go source witnesses live in `metasystem/internal/validate/conformance_mutations_test.go` and call public Conformance, which implements the verb (`metasystem/internal/validate/conformance.go:196-205`). CLI witnesses live in `metasystem/cmd/metasystem/conformance_mutations_test.go` and call runValidateConformance with captured stdout, stderr, and exit status (`metasystem/cmd/metasystem/validate_verbs.go:280-352`). The latter follows the sibling's CLI-without-critic precedent (`metasystem/plans/brief-declares-the-round-boundary-design.md:328`). Each returned entry names its same-package source witness; the CLI test is additional public-surface proof.

Use real composed inline and referenced rounds in separate Go seam tests. Bed convenience must not replace production source selection with a fabricated acceptanceRules member. Extend the existing implementer-v1-v2 return-schema scenario with complete-entry preservation, missing failureLine refusal, and legacy v2 omission. Its normalization and completeness calls are at `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`.

Builders run Go tests and the fast gate. They never run fixture beds or export METASYSTEM_BIN for Go tests. The seat runs beds through its enrolled engine on the returned candidate. This follows the builder/seat split at `metasystem/docs/orchestration.md:172` and `metasystem/scripts/agents/templates/brief.md:14-20`. Test-only instruction packages are included explicitly in their unit commands. They do not prove model behavior. No fixture-registration test is required by this design.

## Seams and precedence

| Seam | Decision |
| --- | --- |
| Sibling header admission | Its unit 1 owns Boundary, Ceiling, and the Rules extension in the same family. This goal consumes its result. |
| Sibling source selection | Its unit 3 owns one explicit-round reader. Bounds use the supplied round; mutation collection calls that reader for every chain round. Header selection is shared; aggregation policy differs. |
| Whole candidate | All accepted entries are checked against the same project reviewedTree already used for diff.patch. The sibling owns its single-snapshot change. |
| Return acceptance | Shape and legacy compatibility belong to returnChecker. Coverage and candidate membership belong to review. No duplicate acceptance-time join. |
| Boundary and mutation refusals | Collect both before successful persistence. Neither declaration grants permission that another gate denies. |
| Immutability | Read existing evidence; refuse a changed repeat; never rewrite an earlier return or successful review. |
| Merge and recertification | No added mutation policy here. Existing stage routing remains (`metasystem/internal/validate/conformance.go:248-254`). |
| Future round-proof work | A bed-failure record can feed corrections. It does not replace the chain's Rules union or the same source reader. No new dependency on that goal. |

## Return readers and validators

This is the compatibility inventory for implementation. The executable owners were read in this worktree. Tests are compatibility witnesses, not new obligations to rewrite old evidence.

| Reader | Disposition and checked anchor |
| --- | --- |
| Provider materializer and schema CLI | Change via the shared overlays and explicit canonical flag. `metasystem/internal/returnschema/returnschema.go:154-187`; `metasystem/cmd/metasystem/schema.go:13-46`. |
| Role, job-file, and job completeness | Change their shared checker only. `metasystem/internal/validate/returncomplete.go:66-89`, `:141-228`. |
| Completeness CLI and shell wrapper | Inherit the checker. `metasystem/cmd/metasystem/validate_verbs.go:178-201`; `metasystem/scripts/assert-return-complete.sh:47-50`. |
| NormalizeReturn and boundary serialization | Preserve admitted new members and old omission. Add tests; do not add a second schema owner. `metasystem/internal/adapter/return.go:25-94`, `:191-225`; `metasystem/internal/validate/returncomplete.go:239-303`, `:326-335`. |
| Adjudication | Keep normalization, shared completeness, and post-repair validation. Change both prompts. `metasystem/internal/adapter/adjudicate.go:48-97`, `:208-246`. |
| Runtime, Devin, and fake acceptance | Inherit the materializer and completeness changes. `metasystem/scripts/agents/adapters/runtime-common.sh:103-110`, `:364-385`; `metasystem/internal/adapter/devincollect.go:220-240`; `metasystem/scripts/agents/adapters/fake.sh:140-149`, `:329-332`. |
| Fake implementer producer | Keep its version-2 omission as compatibility evidence; do not invent mutation failures. `metasystem/internal/adapter/fake.go:53-74`, `:109-112`. |
| Conformance | Add the separate chain join and lookup. Keep boundary declaration policy. `metasystem/internal/validate/conformance.go:214-223`, `:724-819`. |
| Follow-up overlap and design pairing | Inspect only their owned diffBoundary fields. `metasystem/internal/dispatch/followup_rebase.go:124-148`; `metasystem/internal/dispatch/review_reference.go:313-325`. |
| Continuation transport and artifact mirror | Carry whole returns and regular artifacts. Cap continuations omit the prior return, which makes chain artifact reads essential. `metasystem/scripts/agents/dispatch.sh:2719-2725`; `metasystem/internal/dispatch/composition.go:300-319`; `metasystem/internal/dispatch/mirror.go:120-139`. |
| Lost-return and supervisor recollection | Continue shared completeness. `metasystem/scripts/agents/dispatch.sh:1336-1349`; `metasystem/internal/supervise/reaper.go:234-260`; `metasystem/cmd/metasystem/supervise_component.go:325-326`; `metasystem/internal/steward/reap.go:106-107`. |
| Landed listings and adapter probes | Keep shared completeness and recursive string inspection. `metasystem/internal/mission/landed.go:101-155`; `metasystem/internal/adapter/selftestrun.go:99-125`, `:217`, `:237`, `:335-342`; `metasystem/internal/adapter/devin.go:337-338`. |
| Role, templates, requirements, recipe, critic skill | Change their owned instruction text and source selection. `metasystem/scripts/agents/roles/implementer.md:11-17`; `metasystem/scripts/agents/templates/brief.md:32-40`; `metasystem/scripts/agents/templates/follow-up.md:12-18`; `metasystem/scripts/agents/roles/implementer.requirements.json:2-15`; `metasystem/scripts/agents/role-packets.json:38-59`; `metasystem/skills/code-critique/SKILL.md:25-37`. |
| Frozen roster test | Keep version 1's exact field set. `metasystem/scripts/validate-metasystem.sh:1661-1706`. |
| Direct completeness regression | Keep legacy v2 normalization and byte identity. `metasystem/internal/validate/returncomplete_direct_test.go:31-121`. |
| Adjudication regression | Keep v2 no-member candidates and successful repaired acceptance. `metasystem/internal/adapter/adjudicate_test.go:205-271`. |
| Telemetry fixture | Keep its version-1-shaped normalization and observed-identity checks. This leg does not call completeness. `metasystem/scripts/agents/telemetry-census-fixtures.sh:48-72`. |
| Static positive implementer fixtures | Keep their version-1 inputs. `metasystem/scripts/validate-metasystem.sh:1917`, `:1946-1947`, `:2388`, `:2403`. |
| Return-schema and conformance beds | Add behavioral legs; retain historical omission inputs. `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`; `metasystem/scripts/agents/conformance-fixtures.sh:73-107`, `:175-186`. |
| Benchmark pin, drift check, extractor, rubrics | Regenerate canonical pin and amend its drift command. Preserve extractor, historical fixture, and scoring. `benchmark/validate-kit.sh:84-102`; `benchmark/schemas/evidence/implementer.schema.json:4`, `:111-125`; `benchmark/extractor.py:63-118`, `:324-341`, `:514-538`; `benchmark/extractor-fixtures.sh:260-274`; `benchmark/rubrics/evidence-honesty.md:15-22`; `benchmark/rubrics/brief-quality.md:15-22`. |

Other-role readers remain unchanged. Critic and warden consumers are at `metasystem/internal/dispatch/finding_register.go:80-82`, `:137-175`, `:413-429`, `:652-663`, `:1287-1293`; `metasystem/internal/dispatch/read_admission.go:400-432`; `metasystem/internal/dispatch/review_reference.go:17-48`; `metasystem/internal/validate/conformance.go:1169-1185`; `metasystem/internal/validate/authorization.go:274-299`; `metasystem/internal/validate/recertification.go:398-425`; `metasystem/internal/landing/observe.go:750-771`; and `metasystem/internal/readsubject/closure.go:198-248`. Host and continuation consumers use their own contracts at `metasystem/internal/steward/reap.go:225-234`, `metasystem/internal/host/hostcollect.go:174`, and `metasystem/internal/missionrunner/adjudicate.go:71-88`, `:120-126`. The fingerprint fixture copies a completeness wrapper at `metasystem/scripts/agents/fingerprint-harness.sh:134` but dispatches a design critic at `:243-246`; it is an indirect other-role regression.

## Build units and builder briefs

Land the sibling's unit 1, then its intervening prerequisites and unit 3, before any unit here. Confirm that its landed parser exposes Rules and its shared reader accepts an explicit round. This goal neither recreates nor privately forks those owners. Land this goal's units in the order printed below. Each is a fresh chain based on the preceding landing. U2 is the very next landing after U1.

Ceiling counts additions plus deletions, including tests, instructions, generated pins, and seat integration changes. Each unit is limited to 400 changed lines. Allocations are planning budgets, not measured patch sizes. If a concrete patch will not fit, stop and return a proposed split, such as U1a and U1b, before exceeding its Boundary or landing. Keep each atomic rule and its failing witness together. A split of the bootstrap must close its exception on the immediately following landing; it cannot create several exempt returns. The seat must rebrief such a split, not silently compress tests or bundle rows.

For every table row below, the test name denotes a proposed Go test in the stated file. Each shared test uses the row id as its subtest name. The return names the enclosing function. Location names a specific predicate or schema member, not a collection of predicates. Table fragments identify proposed targets; a return uses the actual implemented symbol or restored line number, never an unresolved table label. Every row gets one independent removal and one observed failureLine. Positive alternatives sharing one predicate are data variants; independent predicates are separate rows. No observation in this design is fabricated proof.

Temporary removals must stay within Boundary and be restored. Run the focused witness once passing, once with only its rule removed, and once restored. Then run, in order, `go build ./...`, `go vet <unit packages>`, `go test -race -count=1 <unit packages>`, `bash -n <touched scripts>` when applicable, and `scripts/agents/go-gate.sh --fast`. Preserve supplied GOCACHE, GOTMPDIR, and STATICCHECK_CACHE. No builder commits, fixture beds, or METASYSTEM_BIN exports. The seat applies and checks any benchmark patch in a disposable integration tree before landing.

A dispatch contains one unit header and its output table, with this page cited as authority. Table references to files absent in this worktree carry `create:` outside the Boundary array, so the existing authority classifier treats them as output references (`metasystem/internal/dispatch/brief.go:155-167`). The seat preserves those output markers when copying the table into the brief. Boundary contains only the unadorned JSON paths.

The tables below are the complete rule-to-witness table. Their ids are exactly the units' Rules arrays. U2 also re-declares every U1 id, with its original individual witness and a fresh observation. Later proof-only units re-declare the production ids they exercise; they do not invent a rule whose removal is merely deleting fixture registration.

### U1. Bootstrap the compatible return shape

About 110 production/CLI lines, 150 test lines, 100 generated/integration lines, and 20 instruction lines. Total allocation 380.

```text
Working Mode: implement
Boundary: ["metasystem/internal/returnschema/returnschema.go","metasystem/internal/returnschema/implementer.go","metasystem/internal/returnschema/implementer_test.go","metasystem/internal/validate/returncomplete.go","metasystem/internal/validate/return_mutations_test.go","metasystem/cmd/metasystem/schema.go","metasystem/cmd/metasystem/schema_mutations_test.go","metasystem/scripts/agents/roles/implementer.md","metasystem/scripts/agents/roles/mutation_contract_test.go","metasystem/artifacts/reports/bprm-kit-compat.patch"]
Ceiling: 400
Rules: ["U1-array","U1-entry-object","U1-test-object","U1-require-ruleId","U1-require-location","U1-require-test","U1-require-failureLine","U1-require-test-kind","U1-require-test-path","U1-require-test-name","U1-type-ruleId","U1-type-location","U1-type-failureLine","U1-type-test.kind","U1-type-test.path","U1-type-test.name","U1-close-entry","U1-close-test","U1-kind","U1-provider-required","U1-canonical-property","U1-canonical-optional","U1-provider-role","U1-provider-version","U1-checker-overlay","U1-cli-canonical","U1-cli-role","U1-cli-version","U1-role-members"]
Non-goals: No text-pattern constraints yet, header or source reader, review join, test lookup, repair prose, fixture execution, benchmark scoring, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Install the typed, required, closed fragment and kind enum first. U2 installs the separately removable text constraints. Materialize both provider and canonical outputs, wire completeness, expose the canonical CLI, update the role member list, and supply the matching benchmark patch. This is a preparatory shape landing, not final review enforcement.

The schema tests inspect the materializer's externally consumed schema, including each scalar type and each required member. Canonical acceptance tests additionally feed complete and malformed returns through ReturnCompleteRole. Keep the frozen schema unchanged. The omitted legacy return must remain byte-identical.

U1 carries one JSON array headed `Mutation table for U1` in evidence[].observed, with level `ran`. It contains one normal-shaped entry per row below and its actual failureLine. Other evidence entries retain all runs. The seat manually joins every id, checks every observed failure, and reads the exact benchmark patch before landing. U1 does not insert a top-level member forbidden by its launch schema. Dispatch U2 using the rebuilt schema immediately after this landing.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U1-array | high | create: `metasystem/internal/returnschema/implementer.go#mutations.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the array type; the null-rejection assertion must fail. |
| U1-entry-object | high | create: `metasystem/internal/returnschema/implementer.go#mutations.items.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the item object type. |
| U1-test-object | high | create: `metasystem/internal/returnschema/implementer.go#test.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the test object type. |
| U1-require-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.ruleId` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only ruleId from entry.required. |
| U1-require-location | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.location` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only location from entry.required. |
| U1-require-test | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.test` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only test from entry.required. |
| U1-require-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.failureLine` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only failureLine from entry.required. |
| U1-require-test-kind | high | create: `metasystem/internal/returnschema/implementer.go#test.required.kind` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only kind from test.required. |
| U1-require-test-path | high | create: `metasystem/internal/returnschema/implementer.go#test.required.path` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only path from test.required. |
| U1-require-test-name | high | create: `metasystem/internal/returnschema/implementer.go#test.required.name` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only name from test.required. |
| U1-type-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#ruleId.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-location | high | create: `metasystem/internal/returnschema/implementer.go#location.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#failureLine.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-test.kind | high | create: `metasystem/internal/returnschema/implementer.go#test.kind.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-test.path | high | create: `metasystem/internal/returnschema/implementer.go#test.path.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-test.name | high | create: `metasystem/internal/returnschema/implementer.go#test.name.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-close-entry | high | create: `metasystem/internal/returnschema/implementer.go#entry.additionalProperties` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Permit an extra entry member. |
| U1-close-test | high | create: `metasystem/internal/returnschema/implementer.go#test.additionalProperties` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Permit an extra test member. |
| U1-kind | high | create: `metasystem/internal/returnschema/implementer.go#test.kind.enum` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the kind enum; an unknown kind must be refused. |
| U1-provider-required | high | create: `metasystem/internal/returnschema/implementer.go#ImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationProviderRequired` | Remove mutations from the provider root required list. |
| U1-canonical-property | high | create: `metasystem/internal/returnschema/implementer.go#CanonicalImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationCanonicalProperty` | Omit the canonical mutations property; an extended return must remain lawful. |
| U1-canonical-optional | high | create: `metasystem/internal/returnschema/implementer.go#CanonicalImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationCanonicalOmission` | Require mutations in canonical output; a historical omitted member must remain lawful. |
| U1-provider-role | high | `metasystem/internal/returnschema/returnschema.go#Materialize` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationOverlayScope` | Remove the implementer-only overlay guard. |
| U1-provider-version | high | `metasystem/internal/returnschema/returnschema.go#Materialize` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationOverlayScope` | Apply the overlay to version 1. |
| U1-checker-overlay | high | `metasystem/internal/validate/returncomplete.go#checkReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestReturnMutationShape` | Remove canonical implementer-v2 overlay selection. |
| U1-cli-canonical | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalImplementer` | Route --canonical through the provider materializer. |
| U1-cli-role | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalScope` | Remove only the --canonical role guard. |
| U1-cli-version | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalScope` | Remove only the --canonical version guard. |
| U1-role-members | high | `metasystem/scripts/agents/roles/implementer.md#version-2-members` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleMembers` | Remove mutations from the required return member sentence. |

Verification packages: `./internal/returnschema ./internal/validate ./cmd/metasystem ./scripts/agents/roles`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
Check the seat patch with `bash -n benchmark/validate-kit.sh` in a disposable repository copy. Run no kit or fixture bed in the builder worktree.

### U2. Close the bootstrap and validate each text claim

About 30 constraint lines, 210 acceptance and preservation test lines, 65 fixture lines, and 35 regenerated pin lines. Total allocation 340.

```text
Working Mode: implement
Boundary: ["metasystem/internal/returnschema/returnschema.go","metasystem/internal/returnschema/implementer.go","metasystem/internal/returnschema/implementer_test.go","metasystem/internal/validate/returncomplete.go","metasystem/internal/validate/return_mutations_test.go","metasystem/cmd/metasystem/schema.go","metasystem/cmd/metasystem/schema_mutations_test.go","metasystem/scripts/agents/roles/implementer.md","metasystem/scripts/agents/roles/mutation_contract_test.go","metasystem/artifacts/reports/bprm-kit-compat.patch","metasystem/internal/adapter/return.go","metasystem/internal/adapter/mutation_return_test.go","metasystem/scripts/agents/return-schema-fixtures.sh"]
Ceiling: 400
Rules: ["U2-bootstrap","U2-pattern-ruleId","U2-pattern-location","U2-pattern-test.path","U2-pattern-test.name","U2-pattern-failureLine","U2-normalize","U2-boundary-preserve","U2-omit-unchanged","U1-array","U1-entry-object","U1-test-object","U1-require-ruleId","U1-require-location","U1-require-test","U1-require-failureLine","U1-require-test-kind","U1-require-test-path","U1-require-test-name","U1-type-ruleId","U1-type-location","U1-type-failureLine","U1-type-test.kind","U1-type-test.path","U1-type-test.name","U1-close-entry","U1-close-test","U1-kind","U1-provider-required","U1-canonical-property","U1-canonical-optional","U1-provider-role","U1-provider-version","U1-checker-overlay","U1-cli-canonical","U1-cli-role","U1-cli-version","U1-role-members"]
Non-goals: No new bootstrap exception, header/source implementation, review policy, test lookup, benchmark scoring, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

**First acceptance row: U2-bootstrap re-validates every U1 row in the new top-level mutations shape.** Re-declare all U1 ids as shown in Rules. Run each U1 removal separately against U2's candidate. Carry a fresh top-level entry and actual failureLine for every U1 id, plus each U2 id. The bootstrap test passes the complete table through canonical acceptance and requires the malformed companion to refuse. It does not replace any individual U1 witness. The seat reads the returned top-level table before landing U2, closing U1's one accepted exception on this next landing. Do not rewrite U1's old canonical return.

Install all five text-pattern applications. The location regex's two legal alternatives are data variants of one predicate. Add normalization and boundary-serialization preservation witnesses. These mutations may temporarily touch the existing serialization owners named in Boundary; restore them before return. Add the three return-schema bed assertions described above, with Go acceptance counterparts. Refresh the canonical benchmark pin. U1 witness rows repeated below keep their original meanings and need new observations, not copied failure text.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U2-bootstrap | high | `metasystem/internal/validate/returncomplete.go#checkReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestMutationBootstrapTopLevel` | Bypass the canonical overlay selection; the complete top-level U1 replay table must be accepted and a malformed replay must be rejected. |
| U2-pattern-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#ruleId.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-pattern-location | high | create: `metasystem/internal/returnschema/implementer.go#location.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-pattern-test.path | high | create: `metasystem/internal/returnschema/implementer.go#test.path.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-pattern-test.name | high | create: `metasystem/internal/returnschema/implementer.go#test.name.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-pattern-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#failureLine.pattern` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationTextConstraints` | Remove only this pattern; its invalid-scalar rejection assertion must fail. |
| U2-normalize | high | `metasystem/internal/adapter/return.go#NormalizeReturn` | create: `metasystem/internal/adapter/mutation_return_test.go#TestNormalizeMutationEvidence` | Drop mutations from the selected normalized object; the complete evidence must survive. |
| U2-boundary-preserve | high | `metasystem/internal/validate/returncomplete.go#writeNormalizedReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestBoundaryRewritePreservesMutations` | Drop mutations only at boundary serialization; the same entries must survive a lawful bare-path repair. |
| U2-omit-unchanged | high | `metasystem/internal/validate/returncomplete.go#checkReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestLegacyMutationOmissionUnchanged` | Insert an empty mutations field into an otherwise valid old return; byte identity must fail. |
| U1-array | high | create: `metasystem/internal/returnschema/implementer.go#mutations.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the array type; the null-rejection assertion must fail. |
| U1-entry-object | high | create: `metasystem/internal/returnschema/implementer.go#mutations.items.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the item object type. |
| U1-test-object | high | create: `metasystem/internal/returnschema/implementer.go#test.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the test object type. |
| U1-require-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.ruleId` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only ruleId from entry.required. |
| U1-require-location | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.location` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only location from entry.required. |
| U1-require-test | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.test` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only test from entry.required. |
| U1-require-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#entry.required.failureLine` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only failureLine from entry.required. |
| U1-require-test-kind | high | create: `metasystem/internal/returnschema/implementer.go#test.required.kind` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only kind from test.required. |
| U1-require-test-path | high | create: `metasystem/internal/returnschema/implementer.go#test.required.path` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only path from test.required. |
| U1-require-test-name | high | create: `metasystem/internal/returnschema/implementer.go#test.required.name` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only name from test.required. |
| U1-type-ruleId | high | create: `metasystem/internal/returnschema/implementer.go#ruleId.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-location | high | create: `metasystem/internal/returnschema/implementer.go#location.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-failureLine | high | create: `metasystem/internal/returnschema/implementer.go#failureLine.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-test.kind | high | create: `metasystem/internal/returnschema/implementer.go#test.kind.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-test.path | high | create: `metasystem/internal/returnschema/implementer.go#test.path.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-type-test.name | high | create: `metasystem/internal/returnschema/implementer.go#test.name.type` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove only this scalar type from the provider schema. |
| U1-close-entry | high | create: `metasystem/internal/returnschema/implementer.go#entry.additionalProperties` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Permit an extra entry member. |
| U1-close-test | high | create: `metasystem/internal/returnschema/implementer.go#test.additionalProperties` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Permit an extra test member. |
| U1-kind | high | create: `metasystem/internal/returnschema/implementer.go#test.kind.enum` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationSchema` | Remove the kind enum; an unknown kind must be refused. |
| U1-provider-required | high | create: `metasystem/internal/returnschema/implementer.go#ImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationProviderRequired` | Remove mutations from the provider root required list. |
| U1-canonical-property | high | create: `metasystem/internal/returnschema/implementer.go#CanonicalImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationCanonicalProperty` | Omit the canonical mutations property; an extended return must remain lawful. |
| U1-canonical-optional | high | create: `metasystem/internal/returnschema/implementer.go#CanonicalImplementerVersionTwo` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationCanonicalOmission` | Require mutations in canonical output; a historical omitted member must remain lawful. |
| U1-provider-role | high | `metasystem/internal/returnschema/returnschema.go#Materialize` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationOverlayScope` | Remove the implementer-only overlay guard. |
| U1-provider-version | high | `metasystem/internal/returnschema/returnschema.go#Materialize` | create: `metasystem/internal/returnschema/implementer_test.go#TestMutationOverlayScope` | Apply the overlay to version 1. |
| U1-checker-overlay | high | `metasystem/internal/validate/returncomplete.go#checkReturn` | create: `metasystem/internal/validate/return_mutations_test.go#TestReturnMutationShape` | Remove canonical implementer-v2 overlay selection. |
| U1-cli-canonical | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalImplementer` | Route --canonical through the provider materializer. |
| U1-cli-role | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalScope` | Remove only the --canonical role guard. |
| U1-cli-version | high | `metasystem/cmd/metasystem/schema.go#runSchemaMaterialize` | create: `metasystem/cmd/metasystem/schema_mutations_test.go#TestSchemaCanonicalScope` | Remove only the --canonical version guard. |
| U1-role-members | high | `metasystem/scripts/agents/roles/implementer.md#version-2-members` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleMembers` | Remove mutations from the required return member sentence. |

Verification packages: `./internal/returnschema ./internal/validate ./internal/adapter ./cmd/metasystem ./scripts/agents/roles`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
Shell syntax input: `scripts/agents/return-schema-fixtures.sh`. The seat runs its bed; the builder does not.

### U3a. Deliver the builder obligation

About 70 instruction/recipe lines and 230 focused contract/composition test lines. Total allocation 300.

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/roles/implementer.md","metasystem/scripts/agents/roles/implementer.requirements.json","metasystem/scripts/agents/roles/mutation_contract_test.go","metasystem/scripts/agents/role-packets.json","metasystem/scripts/agents/mutation_packet_test.go","metasystem/scripts/agents/templates/brief.md","metasystem/scripts/agents/templates/follow-up.md","metasystem/scripts/agents/templates/mutation_contract_test.go"]
Ceiling: 400
Rules: ["U3a-role-proof","U3a-role-entries","U3a-redeclare","U3a-retain-tests","U3a-gap","U3a-requirement","U3a-delivery","U3a-brief-proof","U3a-brief-rules","U3a-followup-proof","U3a-followup-rules"]
Non-goals: No capability names, header parser, source reader, composition schema, conformance logic, fixture beds, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Use colocated Go test-only packages for instruction resources. These satisfy the same-directory location rule without inventing resource-to-package aliases. The recipe witness calls ComposeRolePacket and reads its delivered bytes and selected source record. Also keep required/optional/waivers unchanged and preserve the sibling's bound instructions. Text guards prove the stated instructions are delivered; they make no claim about a model's obedience.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U3a-role-proof | high | `metasystem/scripts/agents/roles/implementer.md#mutation-proof` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions` | Remove the exact proof sentence. |
| U3a-role-entries | high | `metasystem/scripts/agents/roles/implementer.md#mutation-return` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions` | Remove the per-id entry instruction. |
| U3a-redeclare | high | `metasystem/scripts/agents/roles/implementer.md#mutation-repair` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions` | Remove the instruction to re-declare repaired ids. |
| U3a-retain-tests | high | `metasystem/scripts/agents/roles/implementer.md#chain-obligations` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions` | Remove the instruction that earlier accepted test names remain obligations. |
| U3a-gap | high | `metasystem/scripts/agents/roles/implementer.md#unproved-rule` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRoleInstructions` | Remove the instruction to report unproved rules in gaps. |
| U3a-requirement | high | `metasystem/scripts/agents/roles/implementer.requirements.json#mutationProof` | create: `metasystem/scripts/agents/roles/mutation_contract_test.go#TestMutationRequirement` | Remove the exact mutationProof value. |
| U3a-delivery | high | `metasystem/scripts/agents/role-packets.json#implementer.sources` | create: `metasystem/scripts/agents/mutation_packet_test.go#TestComposeMutationRequirement` | Remove the requirements source from the recipe; actual composed bytes must lose the required instruction. |
| U3a-brief-proof | high | `metasystem/scripts/agents/templates/brief.md#Constraints` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationBriefInstructions` | Remove the exact proof sentence from the build template. |
| U3a-brief-rules | high | `metasystem/scripts/agents/templates/brief.md#Acceptance-Criteria` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationBriefInstructions` | Remove the Rules JSON-array instruction. |
| U3a-followup-proof | high | `metasystem/scripts/agents/templates/follow-up.md#Unchanged-Return-Contract` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationFollowUpInstructions` | Remove the exact proof sentence from follow-up guidance. |
| U3a-followup-rules | high | `metasystem/scripts/agents/templates/follow-up.md#repaired-rules` | create: `metasystem/scripts/agents/templates/mutation_contract_test.go#TestMutationFollowUpInstructions` | Remove the follow-up instruction to re-declare repaired ids. |

Verification packages: `./scripts/agents ./scripts/agents/roles ./scripts/agents/templates ./internal/dispatch`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
No shell syntax input for this unit.

### U3b. Keep repair honest and fix the critic sample

About 70 prompt/skill lines and 220 output-contract test lines. Total allocation 290.

```text
Working Mode: implement
Boundary: ["metasystem/internal/adapter/adjudicate.go","metasystem/internal/adapter/mutation_repair_test.go","metasystem/skills/code-critique/SKILL.md","metasystem/skills/code-critique/mutation_contract_test.go"]
Ceiling: 400
Rules: ["U3b-repair-shape","U3b-repair-delivery","U3b-sample-min","U3b-sample-root","U3b-sample-small","U3b-severity","U3b-id-order","U3b-each-entry","U3b-material","U3b-environment","U3b-fixture-execution","U3b-record","U3b-disposable","U3b-restore","U3b-severity-default","U3b-severity-repeat","U3b-no-swap","U3b-failure-cause"]
Non-goals: No automatic mutation runner, schema changes, repair control-flow changes, fixture execution, critic budget changes, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Tests invoke both actual prompt writers and inspect their emitted instructions. The skill test protects each independently removable sampling instruction. It does not pretend to run a critic. The seat later proves compliance by reading actual critic sample evidence. Preserve post-repair validation and the existing independent critic contract.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U3b-repair-shape | high | `metasystem/internal/adapter/adjudicate.go#writeRepairPrompt` | create: `metasystem/internal/adapter/mutation_repair_test.go#TestMutationRepairInstructions` | Remove the exact mutation honesty paragraph from ordinary repair output. |
| U3b-repair-delivery | high | `metasystem/internal/adapter/adjudicate.go#writeDeliveryRepairPrompt` | create: `metasystem/internal/adapter/mutation_repair_test.go#TestMutationDeliveryRepairInstructions` | Remove that paragraph from missing-delivery repair output. |
| U3b-sample-min | high | `metasystem/skills/code-critique/SKILL.md#minimum-sample` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove the at-least-three component of the sample rule. |
| U3b-sample-root | high | `metasystem/skills/code-critique/SKILL.md#sample-growth` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove the ceiling-square-root component. |
| U3b-sample-small | high | `metasystem/skills/code-critique/SKILL.md#small-inventory` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove the cap at n, which samples every row when n is below three. |
| U3b-severity | high | `metasystem/skills/code-critique/SKILL.md#severity-order` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove highest-severity-first ordering. |
| U3b-id-order | high | `metasystem/skills/code-critique/SKILL.md#id-order` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove ascending-id tie breaking. |
| U3b-each-entry | high | `metasystem/skills/code-critique/SKILL.md#repeated-id` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove reproduction of every accepted entry for a sampled id. |
| U3b-material | high | `metasystem/skills/code-critique/SKILL.md#non-reproduction` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove material classification of a non-reproducing sample. |
| U3b-environment | high | `metasystem/skills/code-critique/SKILL.md#execution-environment` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Permit a skip or unavailable environment to count as reproduction. |
| U3b-fixture-execution | high | `metasystem/skills/code-critique/SKILL.md#fixture-execution` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove the requirement to enter the named leg in the location script. |
| U3b-record | high | `metasystem/skills/code-critique/SKILL.md#sample-record` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove the ordered inventory and sampled-observation record instruction. |
| U3b-disposable | high | `metasystem/skills/code-critique/SKILL.md#sample-copy` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove use of a disposable reviewed-tree copy. |
| U3b-restore | high | `metasystem/skills/code-critique/SKILL.md#sample-restoration` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove restoration after each attempt. |
| U3b-severity-default | high | `metasystem/skills/code-critique/SKILL.md#default-severity` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove the high default for a row without severity. |
| U3b-severity-repeat | high | `metasystem/skills/code-critique/SKILL.md#repeated-severity` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove use of the highest declared severity across repeats of an id. |
| U3b-no-swap | high | `metasystem/skills/code-critique/SKILL.md#fixed-initial-sample` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Permit replacing a selected row with a more convenient row. |
| U3b-failure-cause | high | `metasystem/skills/code-critique/SKILL.md#failure-cause` | create: `metasystem/skills/code-critique/mutation_contract_test.go#TestMutationCriticContract` | Remove the requirement that the reproduced assertion and cause match the claim. |

Verification packages: `./internal/adapter ./skills/code-critique`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
No shell syntax input for this unit.

### U4a. Collect the whole chain and join its rows

About 200 production and moved lines plus 180 test lines. Total allocation 380.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance.go","metasystem/internal/validate/conformance_rounds.go","metasystem/internal/validate/conformance_rounds_test.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go"]
Ceiling: 400
Rules: ["U4a-rounds","U4a-root","U4a-round-read","U4a-source","U4a-source-error","U4a-union","U4a-returnless","U4a-any-round","U4a-shape","U4a-object","U4a-return-read","U4a-missing","U4a-duplicate","U4a-cross-round-repeat","U4a-unknown","U4a-rejected-repair"]
Non-goals: No duplicate brief parser or reader, acceptance normalization, lookup, artifact wiring, merge policy, fixture beds, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Extract only the existing round-directory selection from cumulativeBoundaryViolations. Its declarations and diagnostics keep their owner. Mutation collection uses the same chain root and round limit, with its own read-error handling. The shared sibling source reader is consumed, not edited. This unit supplies a tested join; U4b activates it at the review boundary. Source seam tests compose real packets with conflicting prior-brief and prior-return headers.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U4a-rounds | high | create: `metasystem/internal/validate/conformance_rounds.go#chainRoundDirectories` | create: `metasystem/internal/validate/conformance_rounds_test.go#TestMutationChainRoundDirectories` | Inspect only the supplied round; higher recorded chain rounds must remain in the inventory. |
| U4a-root | high | create: `metasystem/internal/validate/conformance_rounds.go#chainRoundDirectories` | create: `metasystem/internal/validate/conformance_rounds_test.go#TestMutationChainRootIsolation` | Remove chain-root filtering; another chain must not supply obligations or evidence. |
| U4a-round-read | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationViolations` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRoundReadFailure` | Treat an unreadable round directory as an empty chain. |
| U4a-source | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationRules` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationChainRuleSource` | Replace the shared explicit-round source reader with root text; inline and verified-reference round tests must fail. |
| U4a-source-error | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationRules` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationBriefFailure` | Swallow a shared source or parser error. |
| U4a-union | high | create: `metasystem/internal/validate/conformance_mutations.go#required-rule-union` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationEarlierMissingRule` | Replace the union with the current Rules array; an earlier missing rule must still refuse. |
| U4a-returnless | high | create: `metasystem/internal/validate/conformance_mutations.go#required-rule-union` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReturnlessRound` | Skip a round brief because its return is absent; its rule must remain required. |
| U4a-any-round | high | create: `metasystem/internal/validate/conformance_mutations.go#entry-collection` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationCrossRoundEntry` | Read only the current return; earlier and later entries must both be able to cover an id. |
| U4a-shape | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationEntries` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationMalformedEntry` | Bypass the one shared schema fragment before typed decoding. |
| U4a-object | high | create: `metasystem/internal/validate/conformance_mutations.go#return-object` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReturnObject` | Treat a present nonobject return as empty evidence. |
| U4a-return-read | high | create: `metasystem/internal/validate/conformance_mutations.go#return-read` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReturnReadFailure` | Treat a corrupt or unreadable present return as missing. |
| U4a-missing | high | create: `metasystem/internal/validate/conformance_mutations.go#missing-id` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationMissingCoverage` | Remove the coverage requirement; absent member and one absent id are data cases of this rule. |
| U4a-duplicate | high | create: `metasystem/internal/validate/conformance_mutations.go#duplicate-in-return` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationDuplicateEntry` | Remove within-return duplicate rejection. |
| U4a-cross-round-repeat | high | create: `metasystem/internal/validate/conformance_mutations.go#duplicate-in-return` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRepeatedRuleAcrossRounds` | Reject an id repeated in a different round; repaired claims must remain lawful. |
| U4a-unknown | high | create: `metasystem/internal/validate/conformance_mutations.go#unknown-id` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationUnknownEntry` | Remove the check that an entry id is in the chain union. |
| U4a-rejected-repair | high | create: `metasystem/internal/validate/conformance_mutations.go#accepted-entry-collection` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRejectedClaimRepair` | Let a rejected historical duplicate or unknown claim permanently refuse a later unique complete join. |

Verification packages: `./internal/validate`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
No shell syntax input for this unit.

### U4b. Enforce the join before review artifacts

About 60 review-wiring lines and 240 behavioral test lines. Total allocation 300.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go"]
Ceiling: 400
Rules: ["U4b-missing","U4b-call","U4b-no-rows","U4b-empty-followup","U4b-before-artifacts","U4b-bounds","U4b-immutable","U4b-repeat","U4b-named-repeat"]
Non-goals: No new source owner, lookup implementation, merge/recertification checks, return rewriting, fixture beds, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Activate the chain check. Use the real controller/worktree fixture and public Conformance. Prove inline current source, referenced current source, source corruption, missing legacy input, root legacy fallback, and explicit empty current Rules through that shared call. The sibling owns the individual range and digest predicates and their mutation rows; this unit proves integration without copying them. U4b also waits for the sibling's unit 4 review enforcement, so its coexistence witness can exercise both checks. Reuse that landed review policy collection.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U4b-missing | high | create: `metasystem/internal/validate/conformance_mutations.go#missing-id` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationMissingRow` | Remove the missing-id branch; the public review verb must refuse and name the row. |
| U4b-call | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationReviewGate` | Remove the mutationViolations call from reviewStage. |
| U4b-no-rows | high | create: `metasystem/internal/validate/conformance_mutations.go#empty-required-set` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationNoRowsUnchanged` | Require a mutations member when the chain union is empty; public review must preserve the old return bytes. |
| U4b-empty-followup | high | create: `metasystem/internal/validate/conformance_mutations.go#required-rule-union` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationEmptyFollowUp` | Let an empty follow-up reset earlier obligations. |
| U4b-before-artifacts | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationRefusalHasNoArtifacts` | Move successful persistence ahead of mutation validation. |
| U4b-bounds | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationAndBoundsRefusals` | Short-circuit one policy violation list when the other also fails. |
| U4b-immutable | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReviewImmutable` | Overwrite previous successful review evidence on a changed repeat. |
| U4b-repeat | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationReviewRepeat` | Refuse an identical successful invocation instead of reusing its evidence. |
| U4b-named-repeat | high | `metasystem/internal/validate/conformance.go#reviewStage` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestMutationChangedReviewNamesFailure` | Suppress the mutation diagnostic when immutable review evidence also causes refusal. |

Verification packages: `./internal/validate`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
No shell syntax input for this unit.

### U5a. Bind test files and locations to the candidate

About 140 lookup/path lines and 220 focused test lines. Total allocation 360.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/mutation_test_lookup.go","metasystem/internal/validate/mutation_test_lookup_test.go"]
Ceiling: 400
Rules: ["U5a-absolute","U5a-nul","U5a-backslash","U5a-linebreak","U5a-empty-component","U5a-dot","U5a-parent","U5a-prefix","U5a-tree","U5a-controller","U5a-frozen","U5a-exact-entry","U5a-regular-only","U5a-mode-644","U5a-mode-755","U5a-location","U5a-package-dir","U5a-package-name","U5a-external-package","U5a-fixture-script"]
Non-goals: No new snapshot, shell or Go execution, source-reader change, test declaration matcher, review activation, fixture beds, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement candidate file reads and package/script association as helpers taking the full entry and reviewedTree. Spaces and ordinary filenames remain legal. The seven path restrictions have individual rows; modes and package alternatives also have their own removable acceptance branches. The helpers become consumers of declaration matchers in the following units. Do not create a permissive temporary public review gate.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U5a-absolute | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths` | Permit an absolute path. |
| U5a-nul | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths` | Permit NUL in a path. |
| U5a-backslash | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths` | Permit a backslash in a path. |
| U5a-linebreak | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths` | Permit a line break in a path. |
| U5a-empty-component | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths` | Permit an empty slash component. |
| U5a-dot | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths` | Permit a dot component. |
| U5a-parent | medium | create: `metasystem/internal/validate/mutation_test_lookup.go#cleanMutationPath` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPaths` | Permit a parent component. |
| U5a-prefix | high | create: `metasystem/internal/validate/mutation_test_lookup.go#project-paths` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPathProjection` | Remove the existing one-time projectDeclaration conversion; nested paths must resolve correctly. |
| U5a-tree | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationCandidateFiles` | Read HEAD instead of the supplied tree; an uncommitted test must count and a deletion must not. |
| U5a-controller | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationControllerIsNotCandidate` | Read the controller file for a missing candidate path. |
| U5a-frozen | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFrozenTree` | Read live workspace bytes after tree capture. |
| U5a-exact-entry | high | create: `metasystem/internal/validate/mutation_test_lookup.go#candidate-files` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationExactEntry` | Treat a descendant entry as proof that the requested directory is a test file. |
| U5a-regular-only | high | create: `metasystem/internal/validate/mutation_test_lookup.go#regular-file-mode` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFileModes` | Remove the regular-mode whitelist; symlink and gitlink data cases must refuse. |
| U5a-mode-644 | high | create: `metasystem/internal/validate/mutation_test_lookup.go#regular-file-mode` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFileModes` | Remove 100644 from the accepted modes. |
| U5a-mode-755 | high | create: `metasystem/internal/validate/mutation_test_lookup.go#regular-file-mode` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationFileModes` | Remove 100755 from the accepted modes. |
| U5a-location | high | create: `metasystem/internal/validate/mutation_test_lookup.go#location-file` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationLocationFile` | Skip existence checking for the location file. |
| U5a-package-dir | high | create: `metasystem/internal/validate/mutation_test_lookup.go#go-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationSamePackage` | Accept an unrelated test in another directory with the same package name. |
| U5a-package-name | high | create: `metasystem/internal/validate/mutation_test_lookup.go#go-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationPackageClause` | Accept an unrelated Go package clause in the same directory. |
| U5a-external-package | high | create: `metasystem/internal/validate/mutation_test_lookup.go#go-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationExternalTestPackage` | Reject the conventional external test package for the rule package. |
| U5a-fixture-script | high | create: `metasystem/internal/validate/mutation_test_lookup.go#fixture-location-association` | create: `metasystem/internal/validate/mutation_test_lookup_test.go#TestMutationSameScript` | Permit a fixture witness from a script other than the location path. |

Verification packages: `./internal/validate`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
No shell syntax input for this unit.

### U5b. Recognize actual Go test declarations

About 100 AST/import lines and 210 test lines. Total allocation 310.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/mutation_go_lookup.go","metasystem/internal/validate/mutation_go_lookup_test.go"]
Ceiling: 400
Rules: ["U5b-suffix","U5b-name","U5b-test-prefix","U5b-name-case","U5b-declaration","U5b-receiver","U5b-type-parameters","U5b-results","U5b-arity","U5b-pointer","U5b-type-T","U5b-import","U5b-alias","U5b-dot","U5b-platform","U5b-parse-error"]
Non-goals: No test execution, compiler invocation, package loader, shell matching, platform proof substitution, fixture beds, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement the exact declaration contract with standard-library go/parser and go/ast. Each independent signature predicate has its own row. Arity counts parameters, not merely AST field nodes. Parseable but invalid test signatures must fail membership; malformed source must return an inspection error. Platform-independent source membership does not waive builder or critic execution proof.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U5b-suffix | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestDeclaration` | Remove the _test.go suffix requirement. |
| U5b-name | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestDeclaration` | Accept a different declared function from the exact returned name. |
| U5b-test-prefix | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestName` | Remove the Test prefix requirement. |
| U5b-name-case | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestName` | Remove the non-lowercase next-rune rule. |
| U5b-declaration | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestDeclaration` | Accept a comment or string mention as a declaration. |
| U5b-receiver | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature` | Remove only the receiver rejection. |
| U5b-type-parameters | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature` | Remove only the type-parameter rejection. |
| U5b-results | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature` | Remove only the result-list rejection. |
| U5b-arity | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature` | Remove only the exactly-one-parameter check. |
| U5b-pointer | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature` | Accept testing.T by value or a variadic parameter instead of the required pointer AST node. |
| U5b-type-T | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestSignature` | Accept *testing.M, including TestMain, instead of *testing.T. |
| U5b-import | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestImports` | Trust the spelling testing without resolving its import path. |
| U5b-alias | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestImports` | Remove explicit import-alias support. |
| U5b-dot | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestImports` | Remove dot-import support. |
| U5b-platform | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestPlatformSource` | Filter out a build-tagged or OS-suffixed source before declaration inspection. |
| U5b-parse-error | high | create: `metasystem/internal/validate/mutation_go_lookup.go#goMutationTestExists` | create: `metasystem/internal/validate/mutation_go_lookup_test.go#TestGoMutationTestParseFailure` | Treat a parser error as successful membership. |

Verification packages: `./internal/validate`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
No shell syntax input for this unit.

### U5c. Recognize fixture claims and complete review lookup

About 100 matching/wiring lines and 230 test lines. Total allocation 330.

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/mutation_fixture_lookup.go","metasystem/internal/validate/mutation_fixture_lookup_test.go","metasystem/internal/validate/mutation_test_lookup.go","metasystem/internal/validate/mutation_test_lookup_test.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go"]
Ceiling: 400
Rules: ["U5c-fixture-suffix","U5c-case","U5c-scenario","U5c-whole-line","U5c-literal","U5c-positive","U5c-quote-single","U5c-quote-double","U5c-lookup-call","U5c-absent","U5c-error","U5c-association","U5c-all-entries","U5c-complete"]
Non-goals: No shell parser or execution, trusted mutation runner, source reader, new artifacts, fixture beds, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Compose the candidate-file, location-association, Go, and fixture helpers, then activate lookup for every accepted entry. The scalar returned names are never executed. Include a selector-looking heredoc data case and document that source matching alone cannot prove execution; the critic instruction is its reproduction gate. Include a same-id two-round case where the old test is deleted but the new test exists. It must still refuse the old name.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U5c-fixture-suffix | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Remove the .sh suffix requirement. |
| U5c-case | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Remove the literal new_case source form. |
| U5c-scenario | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Remove the positive fixture_scenario source form. |
| U5c-whole-line | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Permit a substring or a commented selector line. |
| U5c-literal | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Permit a variable-expanded name as a literal selector. |
| U5c-positive | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Permit a negative scenario comparison. |
| U5c-quote-single | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Remove single-quoted literal-name support. |
| U5c-quote-double | high | create: `metasystem/internal/validate/mutation_fixture_lookup.go#fixtureMutationTestExists` | create: `metasystem/internal/validate/mutation_fixture_lookup_test.go#TestFixtureMutationMembership` | Remove double-quoted literal-name support. |
| U5c-lookup-call | high | create: `metasystem/internal/validate/conformance_mutations.go#mutationViolations` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationAbsentTest` | Remove the named-test lookup loop. |
| U5c-absent | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-result` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationAbsentTest` | Ignore a false membership result; public review must name the absent test. |
| U5c-error | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-result` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationUnreadableTest` | Convert a Git or parser inspection error into successful review. |
| U5c-association | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-result` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationLocationMismatch` | Ignore a failed package or script association. |
| U5c-all-entries | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-all-accepted-entries` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationEarlierTestDeleted` | Check only one or the newest entry per id; deleting an earlier named test must refuse. |
| U5c-complete | high | create: `metasystem/internal/validate/conformance_mutations.go#successful-join` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationComplete` | Refuse a completely covered chain whose Go and fixture witnesses both exist. |

Verification packages: `./internal/validate`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
No shell syntax input for this unit.

### U6a. Add the two refusal fixtures and public CLI witnesses

About 110 bed lines and 240 Go/CLI test lines. Total allocation 350.

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/conformance-fixtures.sh","metasystem/cmd/metasystem/conformance_mutations_test.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go"]
Ceiling: 400
Rules: ["U4b-missing","U5c-absent"]
Non-goals: No new production policy, registration-only witnesses, testing.json changes, fixture execution by the builder, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Add bprm-missing-row and bprm-absent-test. Add their public CLI tests, including exit status, exact diagnostic, and absence of both success artifacts. Re-run each named validate witness with its corresponding production rule removed, then restore it. The rule location is the production predicate in validate, not the fixture declaration. The returned test stays in the same package. The CLI witnesses prove the same behavior through runValidateConformance, without a critic. The seat must execute both bed legs through the engine on this candidate.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U4b-missing | high | create: `metasystem/internal/validate/conformance_mutations.go#missing-id` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationMissingRow` | Remove the missing-id branch; the public review verb must refuse and name the row. |
| U5c-absent | high | create: `metasystem/internal/validate/conformance_mutations.go#lookup-result` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationAbsentTest` | Ignore a false membership result; public review must name the absent test. |

Verification packages: `./internal/validate ./cmd/metasystem`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
Shell syntax input: `scripts/agents/conformance-fixtures.sh`. The seat runs its bed; the builder does not.

### U6b. Add the two admission fixtures and public CLI witnesses

About 110 bed lines and 230 Go/CLI test lines. Total allocation 340.

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/conformance-fixtures.sh","metasystem/cmd/metasystem/conformance_mutations_test.go","metasystem/internal/validate/conformance_mutations.go","metasystem/internal/validate/conformance_mutations_test.go"]
Ceiling: 400
Rules: ["U5c-complete","U4b-no-rows"]
Non-goals: No new production policy, registration-only witnesses, test-runner or group changes, fixture execution by the builder, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Add bprm-complete and bprm-no-rows. Add their public CLI tests and read back reviewedTree, diff.patch, review.json, and unchanged return bytes. The complete case uses both a same-package Go test and a fixture leg in its location script. Mutation-test admission by removing the successful join path, and no-row compatibility by removing its empty-set exemption. Do not mutate the legacy return into the new shape. The seat runs all four DONE legs and the supplemental return-schema scenario through the engine before claiming completion.

| Rule id | Severity | Location to mutate | Named Go witness | One removal that must produce an observed failure |
| --- | --- | --- | --- | --- |
| U5c-complete | high | create: `metasystem/internal/validate/conformance_mutations.go#successful-join` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationComplete` | Refuse a completely covered chain whose Go and fixture witnesses both exist. |
| U4b-no-rows | high | create: `metasystem/internal/validate/conformance_mutations.go#empty-required-set` | create: `metasystem/internal/validate/conformance_mutations_test.go#TestConformanceMutationNoRowsUnchanged` | Require a mutations member when the chain union is empty; public review must preserve the old return bytes. |

Verification packages: `./internal/validate ./cmd/metasystem`. Use the ordered build, vet, race test, syntax check, and fast-gate sequence above.
Shell syntax input: `scripts/agents/conformance-fixtures.sh`. The seat runs its bed; the builder does not.


## Acceptance-row accounting

The unit tables replace revision 1's compound expansion. Required ruleId, location, test, and failureLine are four rows. Required kind, path, and name are three rows. Every scalar type is separate. Each path/name pattern application is separate. Receiver, type parameters, results, arity, pointer form, and testing.T are separate signature predicates. The mode whitelist and its two positive alternatives have separate rows. Each builder returns one observation for every listed removal.

A shared named test is allowed because the returned ruleId and its subtest identify one removal. It never licenses one mutation run to stand for another. If implementation adds another independent predicate, the builder reports a brief gap. The seat adds an explicit row and witness or splits the unit before proceeding. It does not hide the predicate in an existing row to meet the line ceiling.

U1's evidence table and U2's top-level replay table are both retained. U2's first row is a closure obligation, and its explicit U1 rows remain individual mutation obligations. The first row cannot replace them. Unit chains after U2 always use top-level mutations. The seat checks their declared rows before landing while review enforcement is being built; this staged implementation does not add another schema bootstrap exception. Once U4b and U5c land, review supplies the chain and candidate checks for subsequent returns.

## Seat verification and remaining proof

After U1 and U2, the seat applies the exact benchmark patch to the candidate integration tree, regenerates the canonical schema into a temporary artifact, and compares its bytes with the pin. It checks that other roles' drift commands are unchanged and that the historical extractor fixture is still admitted. The configured project extra suite is `../benchmark/evidence-drift-fixtures.sh` (`metasystem/metasystem.conf:105`). Run that selection for this reader change. Do not add the kit to the engine's dependency graph.

After the refusal and admission fixture units, the seat runs the four DONE legs and the return-schema scenario through the enrolled engine. Existing group ids are `section/conformance-fixtures` and `section/return-schema-fixtures` (`metasystem/testing.json:75`, `:79`). No new bed, runner, group id, or testing policy is required. The public review verb runs from the target checkout; invoking it from the implementer workspace is already refused (`metasystem/internal/validate/conformance.go:227-230`). The seat reads exit status, exact diagnostics, review artifacts, and return byte identity before dispatching the critic.

A diagnostic selection from the installation is:

```sh
bin/metasystem test run --root . --goal builder-proves-each-rule-by-mutation --mode canary --purpose diagnostic --groups section/return-schema-fixtures,section/conformance-fixtures,section/project-extra-suites --json
```

These selection flags belong to `metasystem/cmd/metasystem/test.go:148-183`. The seat then uses the goal's actual `test plan`, `test run`, and `test verify` delivery selection. Diagnostic evidence alone is not delivery authorization. That shared completion contract is at `metasystem/docs/project-rules.md:17` and `metasystem/docs/design/design-obligation-gate.md:15-17`. Retain explicit results for the colocated instruction test packages as well as the engine selections. Read actual builder mutation entries and the critic's ordered sample evidence. Green schema and membership checks cannot replace those observations.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| BPRM-SHAPE | HIGH | Return member | Typed v2 mutations, optional historical canonical omission | returnschema and returnChecker | U1 and U2 overlays | Individual schema and preservation witnesses | Return-schema bed and canonical benchmark pin | PARTIAL | Build U1, then close bootstrap in U2 |
| BPRM-SOURCE | HIGH | Shared source | All headers read one authenticated authored source per round | Sibling dispatch parser and validate brief reader | Sibling units 1 and 3 | Sibling predicate tests plus U4a source integration | Real inline and referenced compositions | PARTIAL | Land prerequisites before U1 |
| BPRM-CHAIN | HIGH | Exact chain join | Every chain id covered; no earlier obligation erased | validate mutationViolations | U4a and U4b | Missing, returnless, repaired, empty-follow-up, duplicate and unknown witnesses | bprm-missing-row and bprm-no-rows | PARTIAL | Build cumulative enforcement |
| BPRM-TEST | HIGH | Candidate lookup | Every accepted test exists in final tree and belongs with its location | validate candidate and declaration lookup | U5a through U5c | File, package, script, signature, and earlier-test deletion witnesses | bprm-absent-test and bprm-complete | PARTIAL | Build lookup and all-entry loop |
| BPRM-BOUNDARY | HIGH | Review persistence | Collect applicable refusals before immutable success artifacts | reviewStage with sibling bounds owner | U4b | Refusal, repeated-review, and bounds coexistence tests | Four public CLI tests without a critic | PARTIAL | Land sibling enforcement before U4b |
| BPRM-HONESTY | HIGH | Builder and critic instructions | Full builder observations and deterministic critic reproduction | Implementer, repair prompts, independent critic | U3a and U3b instruction outputs | Colocated instruction and emitted-prompt guards | Real mutation returns and recorded critic sample | PARTIAL | Read actual observations before landing |
| BPRM-BOOTSTRAP | HIGH | Accepted exception | U1 manual table is re-proved in top-level form on next landing | Seat and U2 builder | Canonical overlay; no permanent bypass | U2-bootstrap plus every U1 row separately | U1 evidence table and U2 top-level return | PARTIAL | Close on U2 landing |

## Critique record

Round 1 verdict was rework: seven material findings. All seven are accepted below. The three non-material findings are recorded and do not control the next round. This is a disposition join, not a claim that an independent critic has approved revision 2.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BPRM-R1-001 | accepted | Seat ruling; create syntax belongs to authority classification (`metasystem/internal/dispatch/brief.go:155-167`). | Every unit has JSON Boundary and Rules, decimal Ceiling, and explicit dependency on sibling units 1 and 3; no duplicate header owner. |
| BPRM-R1-002 | accepted | Review judges a candidate tree and cumulative declarations span the chain (`metasystem/internal/validate/conformance.go:368-390`, `:724-817`). | Exact union/join over all selected chain rounds; entries may cross rounds; every accepted test is checked in the final tree; repair re-declares its id. |
| BPRM-R1-003 | accepted | successorTaskDirection returns the whole inline prompt (`metasystem/internal/validate/conformance.go:942-954`). | One sibling-owned source-range reader, verified slot/digest/byte-count reference join, and explicit root-brief legacy fallback. |
| BPRM-R1-004 | accepted | Existing legs prove status and diagnostics by invoking review (`metasystem/scripts/agents/conformance-fixtures.sh:153-186`). | Four engine-driven behavioral legs, four public CLI Go tests, and same-package behavioral mutation witnesses; no registration-only witness. |
| BPRM-R1-005 | accepted | Schema members and object closure are separate constraints (`metasystem/internal/returnschema/returnschema_test.go:199-231`). | Complete atomic row tables, one removal and failureLine per row, split units, and a hard concrete-diff stop before 400 lines. |
| BPRM-R1-006 | accepted | Source membership cannot establish causal failure; the seat defines the required association and sample. | Same package or same script; sample min(n,max(3,ceil(sqrt(n)))), severity then id; all sampled claims reproduce or become material, including platform and shell-data cases. |
| BPRM-R1-007 | accepted | Launch freezes the old closed schema before U1 runs (`metasystem/scripts/agents/adapters/runtime-common.sh:103-110`; `metasystem/scripts/agents/schemas/implementer.schema.json:5-8`). | One accepted exception in Decisions; seat reads U1 by hand; U2's first row re-validates each U1 row in top-level form and closes it on the next landing. |
| BPRM-R1-008 | noted, non-material | Added omitted compatibility readers. Telemetry is a v1-shaped normalization witness, not a v2 completeness test (`metasystem/scripts/agents/telemetry-census-fixtures.sh:48-72`). | Inventory includes frozen roster, direct byte identity, adjudication, and telemetry; static v1 and fingerprint labels are corrected too. |
| BPRM-R1-009 | noted, non-material | No implementation patch exists, so allocations are not measured sizes. | Re-estimated split units while accepting finding 005; retain the concrete 400-line stop and require a rebriefed split if needed. |
| BPRM-R1-010 | noted, non-material | const is supported (`metasystem/internal/validate/returncomplete.go:421`); actual snapshot capture uses isolated index and add -A (`metasystem/internal/gittree/gittree.go:255-276`). | Facts cite those owners. The Go declaration contract is a proposal, with no host-specific installed-Go citation. |

Every material id in the critique has one disposition. No material finding is refuted. The seat's rulings determine the changed contracts; the code pass supports their ownership and integration points. A further critique uses the existing chain and its recorded budget, not a fresh chain.

This design does not claim runtime success. Implementation, actual mutation observations, public CLI tests, fixture execution, benchmark compatibility execution, and independent code critique remain pending. Failure to read evidence refuses the new check; failure to reproduce a sampled claim remains material. The residual limit is explicit: source association and shape are mechanically checked, while causal truth is sampled. There is no new question for Wido.

Proposed receipt for the seat: `design builder-proves-each-rule-by-mutation r2: accepted all seven material findings; shared sibling headers and source reader; cumulative mutation join and final-tree test association; atomic witnesses and deterministic critic sample; explicit U1 exception closed by U2; implementation and runtime proof pending`.
