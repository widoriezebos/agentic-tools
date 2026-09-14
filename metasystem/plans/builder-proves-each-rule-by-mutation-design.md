# builder-proves-each-rule-by-mutation

- Owner: m1e (goal 29 of plans/delivery-efficiency-plan.md). **Revision 1, 2026-09-14**, designed by a Codex gpt-6-astra delegate at the seat's brief after Wido's word of the same day; the seat integrated the page unchanged and critiques it next.
- Goal and current status: a `mutations` array in the version-2 implementer return, one entry per acceptance row of the current brief (rule location, the test that fails when that rule alone is removed, the observed failure line); the brief declares its rows in a `Rules:` header frozen into the round's composition record; review-stage conformance refuses a missing, duplicate or unknown row and a named test absent from the candidate tree; the critic samples the builder's claims. Five builder units of at most 400 changed lines each. Nothing built yet.
- In flight right now: revision 1 under its first critique read (Codex gpt-5.6-sol, design critic); the fold and U1 follow.
- Decisions made (and who made them): Wido, 2026-09-14: the mutation battery moves from the read to the builder's return; units are at most 400 changed lines. The design delegate: acceptance rows are not mechanically extractable today, so the brief declares their ids in a `Rules:` header (the same header precedent as brief-declares-the-round-boundary); the schema proves shape and membership, never the observed causal failure, which stays with the builder's honesty and the critic's reproduction sample; no trusted mutation runner is added.
- Waiting on the human: nothing.
- Dead ends (do not retry without new evidence): none yet.
- Next step: revision 1 lands with this line, so the critique read has a landed subject; the read's material findings fold into revision 2, then U1 (the version-2 shape and compatible readers) is briefed to Codex gpt-5.6-sol with its own Boundary, Ceiling, Rules and the proof rule, read by Opus, landed by the seat, and U2 to U5 follow in order after brief-declares-the-round-boundary's unit 1, which owns the shared header precedent.

Design delegate return. Prepared on 2026-09-14 against commit `7e3d21210ee06e97779a22df36019f1480136597`. Evidence level: read. No implementation, test run, or mutation run is claimed here.

The contract is the DONE text in `metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:8`. Add a `mutations` array to the version-2 implementer return. Dispatch records the rule IDs from the current brief. Review joins those IDs to the returned entries and checks test names in the exact candidate tree. The builder runs the mutation battery before returning. The critic samples those claims.

All file names below are repository-relative, including the `metasystem/` prefix. Commands run from the `metasystem` directory unless stated otherwise. Proposed signatures, messages, and tests are specifications, not claims that they already exist.

## Facts that determine the design

| Current behavior | Evidence and consequence |
| --- | --- |
| The checked-in implementer schema is version 1. Its root is closed. Its required list contains identity, evidence, gaps, mode, risk, boundary, and summary fields. | `metasystem/scripts/agents/schemas/implementer.schema.json:3-39`. Leave this frozen file unchanged. |
| Version 2 adds `schemaVersion` and `claimed`. It makes observed session and model strings. It adds no mutation evidence. | `metasystem/internal/returnschema/returnschema.go:29-74`. Add an implementer-only version-2 overlay. |
| Runtime launch materializes version 2 for implementers and version 4 for critics. | `metasystem/scripts/agents/adapters/runtime-common.sh:103-110`. Keep these version choices. |
| The structured-output test requires every object property to be required and every object to be closed. | `metasystem/internal/returnschema/returnschema_test.go:176-265`. Newly materialized implementer schemas require `mutations`, even when its value is `[]`. |
| Canonical return validation regenerates a schema from the shipped version-1 file, then validates its shape and value. | `metasystem/internal/validate/returncomplete.go:161-214`. Update this path as well as materialization. Updating the provider schema alone would leave acceptance rejecting the new member. |
| The validator supports `type`, `enum`, `required`, `additionalProperties`, `items`, and `pattern`. It does not support `oneOf`, `minimum`, or `minLength`. | `metasystem/internal/validate/returncomplete.go:419-448`. Use the existing subset. No schema-validator expansion is needed. |
| `implementer.requirements.json` has an empty `required` array and runtime fallbacks and waivers. Capability selection interprets `required` as capability names. | `metasystem/scripts/agents/roles/implementer.requirements.json:2-15`; `metasystem/internal/capability/select.go:82-125`. Do not put `mutations` in that array. |
| Critic packet recipes carry the `artifactMember` instruction. Composition delivers it as its own source. Recipes also support additional file sources. | `metasystem/scripts/agents/role-packets.json:38-59`; `metasystem/internal/dispatch/composition.go:275-283`. Reuse the file-source mechanism to deliver the implementer's requirements sentence. No new packet mechanism is needed. |
| The implementer role enumerates its return members and asks for evidence marked `ran`, `read`, or `inferred`. The critic checks acceptance criteria and challenges named tests. Neither text requires an observed failure after removing a rule. | `metasystem/scripts/agents/roles/implementer.md:11-13`; `metasystem/skills/code-critique/SKILL.md:25-37`. Add the proof obligation to the builder and the reproduction spot check to the critic. |
| The brief template has a prose acceptance section. The brief parser extracts `Working Mode:` and repository paths, not acceptance rows. | `metasystem/scripts/agents/templates/brief.md:38-40`; `metasystem/internal/dispatch/brief.go:37-105,121-172`. Acceptance rows are not mechanically extractable today. Declare their IDs explicitly. |
| Composition reads the current task direction before adding role sources and continuations. It persists a per-round composition record. | `metasystem/internal/dispatch/composition.go:237-268,300-310,437-467`. Freeze the extracted IDs there, before prior briefs can enter the packet. |
| A follow-up may include both the original brief and the prior return. A large task direction may become a verified reference. | `metasystem/scripts/agents/dispatch.sh:2719-2725`; `metasystem/internal/dispatch/composition.go:395-401`; `metasystem/internal/validate/conformance.go:930-960`. Searching an assembled prompt indiscriminately could count an earlier round's rules. |
| Review snapshots the project, computes its diff, checks the cumulative boundary, then persists `review.json` and `diff.patch`. | `metasystem/internal/validate/conformance.go:355-428`. Insert the new check before persistence, beside the boundary call. |
| Boundary checking reads immutable per-round `return.json` files and unions their path declarations. | `metasystem/internal/validate/conformance.go:754-819`. Reuse its artifact location convention, not its union semantics. A mutation belongs to one round's acceptance row. |
| Conformance violations are strings returned in `[]string`. The prose waiver computes additions plus deletions and appends a specific diagnostic. | `metasystem/internal/validate/conformance.go:617-643`. Use stable mutation reason codes within the same string convention. The cited code does not define a separate typed violation struct. |
| Adapter acceptance normalizes a candidate, invokes job-mode completeness, and offers one reply-shape repair. The repair asks for existing findings and says not to repeat the work. | `metasystem/internal/adapter/adjudicate.go:48-78,208-246`. Preserve the acceptance/review split and forbid invented mutation evidence in both repair prompts. |
| Acceptance can normalize a bare existing boundary path and rewrite the canonical return. | `metasystem/internal/validate/returncomplete.go:239-303`. This permission does not extend to synthesizing mutation entries. |

## Return member and acceptance rows

Use this exact new top-level member. Other version-2 members stay as they are.

```json
{
  "mutations": [
    {
      "ruleId": "R1",
      "location": "metasystem/internal/validate/conformance_mutations.go#(*conformanceRun).mutationViolations",
      "test": {
        "kind": "go",
        "path": "metasystem/internal/validate/conformance_mutations_test.go",
        "name": "TestConformanceMutationMissingRow"
      },
      "failureLine": "conformance_mutations_test.go:91: missing R1 was admitted"
    }
  ]
}
```

The example failure line illustrates the wire format. It is not observed evidence. A builder must replace it with its actual test output.

Each entry has exactly `ruleId`, `location`, `test`, and `failureLine`. `test` has exactly `kind`, `path`, and `name`. All members are required. Both objects have `additionalProperties: false`. `mutations` is an array, never null. Empty is lawful at acceptance.

`ruleId` matches `^[A-Za-z][A-Za-z0-9_.-]*$`. It refers to one ID in the current brief's `Rules:` header. Require exactly one entry for each declared ID at review. Reject duplicate IDs and IDs absent from that header when the brief declares rows. One test may prove several rules, but each rule gets its own removal run and entry.

`location` is a single string in either form:

* `<repository-relative-path>:<positive-decimal-line>`, with a one-based line number in the restored candidate.
* `<repository-relative-path>#<symbol>`, for example a Go function, a JSON property, or a named fixture leg.

Its schema pattern is `^[^\r\n]+(:[1-9][0-9]*|#[^\r\n\s][^\r\n]*)$`. The schema checks this representation. It does not resolve the symbol or prove that the location is the rule's owner. The critic checks that claim. Symbols are preferable when line numbers are likely to move.

`test.kind` is the enum `go` or `fixture`. `test.path`, `test.name`, and `failureLine` use the pattern `^[^\r\n]*\S[^\r\n]*$`. `test.path` names the exact test file or script relative to the repository root. The candidate lookup below enforces its path and kind. `test.name` is an exact function or leg name, not a command or a regular expression. `failureLine` is one actual failure diagnostic from the mutated run. A compile error, timeout, skipped test, or unrelated setup failure does not prove the rule.

Use only the supported schema keywords. Keep `evidence` in its existing `{command, observed, level}` shape. That shape is specified at `metasystem/scripts/agents/schemas/implementer.schema.json:22-33` and explicitly protected by `metasystem/scripts/agents/templates/brief.md:34`. Record the runnable commands, baseline pass, isolated removal, observed failure, restoration, and restored pass there. The new array is the structured join key and failure claim.

Add these signatures in `metasystem/internal/returnschema/implementer.go`:

```go
type MutationTest struct {
    Kind string `json:"kind"`
    Path string `json:"path"`
    Name string `json:"name"`
}
type MutationEntry struct {
    RuleID      string       `json:"ruleId"`
    Location    string       `json:"location"`
    Test        MutationTest `json:"test"`
    FailureLine string       `json:"failureLine"`
}
func MutationEntriesSchema() map[string]any
func ImplementerVersionTwo(schema map[string]any) (map[string]any, error)
func CanonicalImplementerVersionTwo(schema map[string]any) (map[string]any, error)
func MaterializeCanonicalImplementer(root, outputPath string) error
// In returnschema.go, shared by the two public materializers:
func materialize(root, role string, version int, outputPath string, canonical bool) error
```

`ImplementerVersionTwo` calls `VersionTwo`, appends `mutations` to root `required`, and installs the fragment returned by `MutationEntriesSchema`. `Materialize` selects it only for role `implementer`, version 2. Other roles and version 1 remain unchanged.

`CanonicalImplementerVersionTwo` returns the same schema, with `mutations` retained in properties but omitted from the root required list. In `returnChecker.checkReturn`, use this canonical form for every implementer version-2 result. An omitted member is accepted without insertion. A present null or malformed member still fails its schema. New provider schemas require the field; historical canonical returns can omit it. An omitted field with declared rules is still refused by review, naming each missing row. The canonical form is for stored evidence readers and is never handed to a structured-output provider. Keep the existing structured-output invariant test on the default `Materialize` outputs.

There is a second actual schema consumer outside the engine. `benchmark/validate-kit.sh:84-92` compares its pinned implementer schema byte-for-byte against the default materializer. `benchmark/extractor.py:514-538` then loads each implementer return through that pinned schema; `load_json` applies it at `benchmark/extractor.py:324-341`. Leaving the pin unchanged would reject the new field. Making that pin require the field would invalidate historical returns, including the current no-member fixture at `benchmark/extractor-fixtures.sh:260-274`.

Add `--canonical` to `schema materialize`, accepted only with `--role implementer --version 2`. This branch calls `MaterializeCanonicalImplementer`. Reuse one private materialization body; keep `Materialize` as its existing provider-facing wrapper. The new branch must refuse other role/version combinations with exit 2 and `--canonical is only available for implementer version 2`. The current CLI owner is `metasystem/cmd/metasystem/schema.go:13-46`.

The seat regenerates `benchmark/schemas/evidence/implementer.schema.json` with `--canonical` and changes only the implementer branch of `benchmark/validate-kit.sh` to use that flag for its drift comparison and regeneration hint. The pinned canonical schema has the new property but does not require it. `benchmark/extractor.py` needs no code change: its existing optional-property and pattern validation already supports this at `benchmark/extractor.py:80-84,93-119`. Do not add Python implementation or alter benchmark scoring. Keep the old extractor fixture unchanged as a compatibility witness.

These two benchmark files are seat integration outputs of U1. The builder prepares their exact patch in `metasystem/artifacts/reports/bprm-kit-compat.patch`, alongside its Go change. The patch includes the generated schema and the small shell flag change. The seat reviews and applies it in the same integration commit as U1, after reviewing the builder's project diff. This separation is required by the current project fence: `refuseOutsideProjectChanges` rejects delegate edits outside the nested project at `metasystem/internal/validate/conformance.go:311-334`. Count the patch's actual benchmark additions and deletions toward U1's 400-line ceiling. Do not count the patch text again as shipped code. The builder never edits the benchmark files directly.

For the brief, use one small header:

```text
Working Mode: implement
Rules: R1, R2

| Rule id | Acceptance rule |
| --- | --- |
| R1 | A missing mutation entry refuses review and names its acceptance row. |
| R2 | An absent named test refuses review and names the test. |
```

Mechanically, an acceptance row is one ID declared by `Rules:`. The matching prose row explains its rule to the builder and critic. The engine does not attempt to infer IDs from Markdown or interpret the prose. The brief author must enumerate every added rule, with one independently removable rule per row. The critic still checks that the diff's rules are all covered by the brief.

Add `func BriefRules(text string) ([]string, error)` beside `BriefMode` in `metasystem/internal/dispatch/brief.go`. Scan lines beginning exactly `Rules:`. Trim the value and whitespace around comma-separated IDs. No header or an empty header means an empty list. Preserve order. More than one header, a repeated ID, an empty comma-delimited item, or an invalid ID is an error. Use the same ID grammar as `ruleId`. The exact errors are `Rules: header appears more than once`, `Rules: duplicate acceptance row "R1"`, and `Rules: invalid acceptance row "<value>"`. Do not treat `# Rules`, a bullet, or an indented example as this header.

In `ComposeRolePacket`, parse `briefBytes` only when `p.Role == "implementer"`. A parse error returns `CompositionRefusal{Code: "REFUSED-BRIEF-RULES", Source: p.Brief, Detail: err.Error()}` before output publication. Add `AcceptanceRules *[]string` with tag `json:"acceptanceRules,omitempty"` to `CompositionRecord`. For every newly composed implementer packet, write a non-nil pointer, including an empty slice. Other roles leave it nil. Thus a new no-row brief records `"acceptanceRules": []`, while an old composition has no member. This is derived brief data, owned by composition, not a second editable brief or a new artifact.

Review uses the current round's `acceptanceRules`. It does not union prior round IDs or borrow another round's entries. A correction brief must enumerate the rules it adds or repairs. It may reuse an ID in its own round. The seat carries any still-unproved rule into that correction brief. An earlier failed review is not certification. This goal adds no new chain-closure policy.

## Review check and test lookup

Add `metasystem/internal/validate/conformance_mutations.go` with:

```go
func (r *conformanceRun) mutationRules(roundDir string) ([]string, error)
func (r *conformanceRun) mutationViolations(reviewedTree string) []string
func mutationEntries(value any) ([]returnschema.MutationEntry, []string)
```

`mutationRules` reads `composition.json` in the current round. A present `acceptanceRules` must be a non-null array of unique valid ID strings. Validate each ID against the declared grammar. An unreadable or malformed composition or malformed member refuses. A present empty array is authoritative and does not fall back to an earlier brief.

For old compositions without this member, or absent compositions, use the retained root `brief.md` on round 1. On later rounds use `successorTaskDirection`, which already handles verified staged task direction references. Its current implementation is at `metasystem/internal/validate/conformance.go:914-960`. If the legacy source file itself does not exist, treat it as an old no-row brief. Other read errors refuse. Apply `BriefRules` to the selected legacy text. This fallback admits old records and supports direct fixture callers. New packets always use the frozen member, so they never scan their embedded prior briefs.

`mutationViolations` resolves the current round directory from `r.root`, `r.rootJob`, and `r.roundText`, exactly as `ConformanceWithOptions` does at `metasystem/internal/validate/conformance.go:214-217`. It then:

1. Loads the rule IDs. On error, returns the brief diagnostic below.
2. Returns no mutation violations immediately when there are no IDs. It does not read, normalize, or rewrite the return for this check. Existing boundary and identity behavior still applies.
3. Reads this round's canonical `return.json` as an object. Absence of `mutations` means no entries. A present value is validated through `mutationEntries`.
4. `mutationEntries` uses `returnChecker.validateSchemaShape` and `validateValue` against `returnschema.MutationEntriesSchema`, with path `$.mutations`. Only after successful shape validation does it decode the typed slice. It does not call `ReturnCompleteJob`, since that function can normalize a boundary and this review must not repair returns.
5. Checks the join. Emit missing rows in brief order. Emit duplicate or unknown rows in return order. For each entry that has a unique declared ID, check the named test. Do not deduplicate entries before diagnosing duplicates.

Add the call in `reviewStage` after `cumulativeBoundaryViolations(paths)` and append its results to the same `violations` slice. Keep the sibling goal's Boundary/Ceiling checker as a separate function and separate call. Neither suppresses the other's violations. Use the existing review refusal and immutable-review handling at `metasystem/internal/validate/conformance.go:392-400,413-419`. If a review artifact already exists and a rerun disagrees, the existing immutable-review refusal wins. A first failed review writes no successful review artifacts.

Exact new stderr lines, with `%q` quoting IDs and names:

```text
conformance failure: MUTATION_MISSING: round 1 acceptance row "R1" has no mutation entry
conformance failure: MUTATION_TEST_ABSENT: round 1 acceptance row "R1" names absent go test "TestMissing" in "metasystem/internal/validate/example_test.go"
conformance failure: MUTATION_TEST_ABSENT: round 1 acceptance row "R2" names absent fixture test "missing-leg" in "metasystem/scripts/agents/conformance-fixtures.sh"
conformance failure: MUTATION_DUPLICATE: round 1 acceptance row "R1" has more than one mutation entry
conformance failure: MUTATION_UNKNOWN: round 1 mutation entry names undeclared acceptance row "R9"
conformance failure: MUTATION_INVALID: round 1 <existing schema violation>
conformance failure: MUTATION_BRIEF_INVALID: round 1 <brief or composition error>
conformance failure: MUTATION_TEST_UNREADABLE: round 1 acceptance row "R1" cannot inspect test "TestExample" in "<path>": <cause>
```

For example, a missing failure field produces `conformance failure: MUTATION_INVALID: round 1 $.mutations[0].failureLine is required`. The existing required-field diagnostic comes from `metasystem/internal/validate/returncomplete.go:531-538`. A malformed return object uses `MUTATION_INVALID: round 1 return.json must be a JSON object`; JSON/read errors use `MUTATION_INVALID: round 1 return.json is unreadable: <cause>`. All mutation refusals exit 1.

Add `metasystem/internal/validate/mutation_test_lookup.go` with:

```go
func (r *conformanceRun) mutationTestExists(tree string, test returnschema.MutationTest) (bool, error)
func goMutationTestExists(source []byte, name string) (bool, error)
func fixtureMutationTestExists(source []byte, name string) bool
```

The candidate is the project-scoped `reviewedTree` from `reviewStage`, not HEAD and not the controller's filesystem. That snapshot already includes uncommitted and unignored new work through `Workspace.Snapshot` at `metasystem/internal/validate/conformance.go:368-390`. `Workspace.Entries` gives literal-path mode and object IDs, and `Workspace.FileAt` reads a blob from the specified tree at `metasystem/internal/gittree/gittree.go:450-499`.

Require a clean, nonempty, repository-relative slash path, with no absolute form, `.` or `..` components, backslash, or line break. Permit spaces and other ordinary filename characters. For a nested installation, strip `r.installPrefix + "/"` exactly once, using `projectDeclaration`. That conversion already owns this dialect at `metasystem/internal/validate/conformance.go:276-292`. An invalid or out-of-project test path fails as `MUTATION_TEST_ABSENT`, retaining the name and original path. Do not normalize the return.

Resolve that literal path with `r.projectWorkspace().Entries(tree, []string{projectPath})`. It must have mode `100644` or `100755`. An absent entry, directory, symlink, or gitlink does not name a test file. Read the blob through `FileAt` only after that check. Git or parsing errors produce `MUTATION_TEST_UNREADABLE`, never a successful lookup. Do not invoke a test command, shell, compiler, or returned command from conformance.

For `go`, require the named path to end in `_test.go`. Parse the blob with `go/parser.ParseFile` and inspect declarations. Accept an exact top-level `*ast.FuncDecl` named by `test.name`, with no receiver, no type parameters, no results, and exactly one parameter of type `*testing.T`. Resolve the local import name of import path `"testing"`, including an explicit alias or dot import. Apply Go's test-name rule: `Test` followed by end of name or a non-lowercase Unicode rune. A `TestMain` with `*testing.T` is an ordinary test and counts; a `TestMain` with `*testing.M` does not satisfy this signature. The installed Go loader makes that distinction at `/opt/homebrew/opt/go/libexec/src/cmd/go/internal/load/test.go:753-766`. A comment, string, method, ordinary helper, benchmark, and subtest name do not qualify. Name a subtest's enclosing Go test function in the return. Search only the supplied `_test.go` file, which makes duplicate names in separate packages unambiguous. Build tags and OS suffixes do not hide a source-level test from this existence check. Actual execution is the builder's proof.

For `fixture`, require `.sh` and an exact literal leg selector in that script. Support these two existing source forms:

```sh
new_case bprm-missing-row
if [[ "$fixture_scenario" == implementer-v1-v2 ]]; then
```

The first form is used by the conformance bed at `metasystem/scripts/agents/conformance-fixtures.sh:175`; the second by the return-schema bed at `metasystem/scripts/agents/return-schema-fixtures.sh:132`. Match a whole line after horizontal whitespace is trimmed. Accept an unquoted, single-quoted, or double-quoted literal name containing only letters, digits, `_`, `.`, and `-`. For `new_case`, require exactly that call and its one literal argument, with an optional trailing comment. For the scenario form require the shown positive equality and `then`, allowing horizontal whitespace variation and quoted literal names. Do not accept comments, name substrings, arbitrary mentions, negative comparisons, or variable-expanded names. This is a source membership check of the named selector, not a shell reachability proof. It does not parse heredoc context or prove execution; a selector-looking string inside data can still be a false claim, which the critic's reproduction check must expose. Do not add a shell parser or a new fixture declaration language for this goal.

This is deliberately an existence gate. It cannot certify that a failure was observed or caused by that rule. Those are builder obligations, independently sampled by the critic.

## Role, requirements, brief, and critic wording

Add this sentence verbatim to the implementer role and to the build brief template:

> for every rule you add, a test that fails when that rule alone is removed; run it before you return

Add the same exact string as a top-level `mutationProof` string in `metasystem/scripts/agents/roles/implementer.requirements.json`. Leave `required`, `optional`, and `waivers` unchanged. Select the requirements file in the implementer recipe in `metasystem/scripts/agents/role-packets.json` with this additional source:

```json
{"slot":"mutation-requirement","path":"scripts/agents/roles/implementer.requirements.json"}
```

The existing source loop delivers these bytes and records their digest. This gives the requirements sentence a real reader instead of leaving it as unused metadata. Runtime capability selection continues to read its existing keys. The recipe's `artifactMember` precedent demonstrates delivering a structured return instruction outside the frozen version-1 schema; it is not itself a schema declaration (`metasystem/internal/dispatch/composition.go:275-283`).

U1 adds `mutations` to the role's enumerated version-2 members. U2 follows it with:

> For each ID in this round's Rules header, return one mutations entry with ruleId, location, test kind, test path, test name, and the observed failureLine. First pass the test, remove only that rule, observe the test fail for that removal, restore the rule, and pass the test again. Keep each run in evidence. Return an empty mutations array when the brief declares no rows. Report an unproved rule in gaps; never invent an observation.

Add `Rules: <comma-separated acceptance row IDs; empty when this build adds no rules>` to `templates/brief.md`. Explain the ID table under Acceptance Criteria. Add the header and the proof sentence to `templates/follow-up.md`, scoped to implementer corrections. The follow-up currently preserves the original return contract at `metasystem/scripts/agents/templates/follow-up.md:12-18`; keep that preservation and require IDs for the correction's added or repaired rules.

Append this wording to both `writeRepairPrompt` and `writeDeliveryRepairPrompt`:

> For an implementer return, copy only mutation entries and failure lines you already observed. Never invent a missing entry or turn a passing run into a mutation claim. Report an unproved rule in gaps. This reply-only repair does not authorize repeating the work.

The delivery repair currently prints a schema and asks for the existing return at `metasystem/internal/adapter/adjudicate.go:84-97`. Adding the instruction to only the ordinary repair would leave this second path silent. The adapter must not populate entries, read tests to fabricate evidence, or rerun mutations. A repaired but unproved return may become schema-valid; review still refuses its missing rows. Completion of delivery is not review certification.

In the code critic's Layer 1, replace the claim that conformance checks only boundary coverage with an accurate statement that it also checks declared mutation rows and test existence. The current wording is at `metasystem/skills/code-critique/SKILL.md:29`. Add:

> Read the builder's mutation entries alongside the acceptance rows. Spot-check a sample by passing the named test, removing only the claimed rule, observing the claimed failure, restoring the rule, and passing the test again. A claim that does not reproduce is a material finding. Record the sampled row IDs and observed results. Run the sample in a disposable copy of the reviewed tree. Expand the sample when a failure gives evidence of a wider problem.

The builder owns the full battery. The critic need not reconstruct it on every read. The existing independent critic and materiality rules remain binding at `metasystem/skills/code-critique/SKILL.md:12,18-23`. A zero-row brief still receives the ordinary diff and defect review.

## Fixtures and Go proof

The four DONE fixtures are exact new `new_case` legs in the existing `metasystem/scripts/agents/conformance-fixtures.sh` bed. Its testing group is `section/conformance-fixtures`, declared at `metasystem/testing.json:74`. Use `new_case`, `write_implementer`, and `expect_failure`; their current owners are at `metasystem/scripts/agents/conformance-fixtures.sh:38-64,73-96,153-170`.

| Exact leg | Input and action | Required result |
| --- | --- | --- |
| `bprm-missing-row` | Current brief declares `Rules: R1, R2`. Its composition records both IDs. Supply a complete version-2 return with only R1's entry. Run review. | Exit 1 and exact line `conformance failure: MUTATION_MISSING: round 1 acceptance row "R2" has no mutation entry`. No successful `review.json` or `diff.patch`. |
| `bprm-absent-test` | Declare R1. Supply a well-shaped entry naming `TestMissing` in `example_test.go`. That candidate file contains `TestPresent`, but not `TestMissing`. Run review. | Exit 1 and exact line `conformance failure: MUTATION_TEST_ABSENT: round 1 acceptance row "R1" names absent go test "TestMissing" in "example_test.go"`. No successful review artifacts. |
| `bprm-complete` | Declare R1 and R2. Return one Go entry naming `TestPresent` in candidate `example_test.go` and one fixture entry naming `present-leg` in candidate `checks.sh`. The latter contains a literal `new_case present-leg`. Include every changed file in the boundary. Run review. | Exit 0, `reviewedTree=` output, and both review artifacts. Read back the review tree and compare return bytes before and after. |
| `bprm-no-rows` | Use the old brief without Rules and the existing legacy return without `mutations`. Run review. | Exit 0, both artifacts, and byte-identical return. Do not add `mutations: []` to make the old input pass. |

These fixture repositories have their installation at their repository root, as `new_case` constructs it at `metasystem/scripts/agents/conformance-fixtures.sh:38-64`. Their test paths therefore have no `metasystem/` prefix. Add an optional fixture-local setup helper for the three new version-2 cases. Do not rewrite the shared legacy return maker or retrofit every old case. Write both the brief and its matching composition member. The dispatch Go tests below prove that the real composer derives that member from the brief.

Also extend the `implementer-v1-v2` scenario in `metasystem/scripts/agents/return-schema-fixtures.sh`. Its group is `section/return-schema-fixtures`, declared at `metasystem/testing.json:78`. That scenario already drives normalization and role completeness at `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`. Add a complete mutation return that survives normalization, a missing failureLine refusal, and an explicit old version-2 no-member pass. These supplement the four DONE legs; they do not replace them.

Every newly enforced rule has a Go test listed in its unit below. Tests call the owning Go function or the public `Conformance` entrypoint. They do not launch a shell fixture bed. Where several assertions share a test function, name subtests after the mutation-table IDs, and return the enclosing function as `test.name`.

## Seams and precedence

| New rule | Existing function it layers on | Which behavior wins |
| --- | --- | --- |
| Version-2 mutation shape | `returnschema.Materialize`, `VersionTwo`, `returnChecker.checkReturn` | Provider schemas require the member. Canonical acceptance permits an old omitted member; any present member must validate. Version 1 and other roles stay unchanged. |
| Brief rule identity | `BriefMode` as header precedent; `ComposeRolePacket` as publication owner | Current task direction is authoritative. It is parsed before continuations. Malformed Rules refuses composition. No Rules is lawful. |
| Rule coverage | `reviewStage`, beside `cumulativeBoundaryViolations` | Both checks apply. No rows adds no review restriction. With rows, missing, duplicate, or unknown evidence refuses before new review artifacts. |
| Named test existence | `reviewStage`'s `reviewedTree`, `projectDeclaration`, `Workspace.Entries`, `Workspace.FileAt` | The named file in the candidate tree wins over controller files, HEAD, or later workspace edits. Source inspection does not claim test execution. |
| Shape repair without proof invention | `adjudicateValidate`, `writeRepairPrompt`, `writeDeliveryRepairPrompt`, `NormalizeReturn` | Acceptance can repair delivery shape and observed identity. It cannot supply mutation truth. Review may refuse a shape-valid repaired return. |
| Review immutability | `refuseExistingReview` | Its existing refusal wins if a previously persisted review would change. Mutation checks never rewrite historical reviews or returns. |
| Critic reproduction | Code critique Layer 1 and its materiality criterion | Non-reproducing claims are material even if schema and existence checks passed. The critic restores only its disposable copy. |
| Boundary and Ceiling sibling | Separate review-stage function calls | This design neither defines nor changes those headers, their parsing, nor their diff policy. The files can merge with a small shared call-site edit. |

`mergeStage`, recertification, and merge waiver policy are left alone. Their current paths are at `metasystem/internal/validate/conformance.go:442-580,584-643`. This goal's required enforcement point is review. Do not add mutation checks to the shared cumulative-boundary function merely because recertification calls it. Do not extend the scope into certification or chain policy without a separate requirement.

## Return readers and validators

This inventory covers executable implementer-return consumers and their entrypoints. It also distinguishes other-role readers found by the same search. Tests and fixture consumers are listed separately. Historical plans and records are evidence, not runtime readers.

| Reader or validator today | Disposition |
| --- | --- |
| `returnschema.Materialize`, `metasystem/internal/returnschema/returnschema.go:154-187`; CLI `metasystem/cmd/metasystem/schema.go:22-42` | **Changes** in the materializer through the implementer overlay. CLI **changes** to expose the explicit canonical implementer form to the benchmark reader. |
| `ReturnCompleteRole`, `ReturnCompleteJobFile`, `ReturnCompleteJob`, and `returnChecker.checkReturn`, `metasystem/internal/validate/returncomplete.go:66-89,141-155,161-228` | **Changes** in their shared checker. All entrypoints consume the same additive schema rule. |
| `checkDiffBoundary` and `writeNormalizedReturn`, `metasystem/internal/validate/returncomplete.go:239-303,326-335` | **Left alone** because they own path normalization. Verify that serialization retains every mutation entry. |
| Return CLI and shell wrapper, `metasystem/cmd/metasystem/validate_verbs.go:178-201`; `metasystem/scripts/assert-return-complete.sh:48-50` | **Left alone** because they call the changed owner. |
| Adapter `NormalizeReturn`, candidate scoring, and identity reconciliation, `metasystem/internal/adapter/return.go:25-27,37-94,191-200`; CLI `metasystem/cmd/metasystem/adapter_selftest_verbs.go:23-38` | **Left alone** because normalization clones the selected object and changes identity. Do not make mutation fields part of candidate scoring. Add preservation tests. |
| `adjudicateValidate` and `adjudicateTurnStage`, `metasystem/internal/adapter/adjudicate.go:48-60,208-246` | **Left alone** in control flow. They inherit shape validation. **Changes in U2** only to the two prompt writers at lines 65 and 84, to prohibit invented evidence. |
| Devin candidate collection, `metasystem/internal/adapter/devincollect.go:220-240` | **Left alone** because it normalizes and invokes `ReturnCompleteJobFile`. |
| Runtime schema and adjudication plumbing, `metasystem/scripts/agents/adapters/runtime-common.sh:103-110,364-368,383`; fake adapter validation at `metasystem/scripts/agents/adapters/fake.sh:149,332` | **Left alone** because these use the materializer or completeness owner. |
| Fake implementer return producer, `metasystem/internal/adapter/fake.go:109-112` | **Left alone** because its legacy no-member return remains valid. It must not manufacture failed mutation runs. New semantic fixtures supply explicit test data. |
| Review current-return existence and cumulative boundary reader, `metasystem/internal/validate/conformance.go:214-223,754-819` | **Changes** by adding the separate current-round mutation reader to `reviewStage`. Existing boundary parsing stays intact. |
| Follow-up path overlap reader, `metasystem/internal/dispatch/followup_rebase.go:97-151` | **Left alone** because it decodes only `diffBoundary` from raw JSON. |
| Design-critique pairing with a work return, `metasystem/internal/dispatch/review_reference.go:304-326` | **Left alone** because it compares only the terminal work return's `diffBoundary` with the design path. |
| Prior-return continuation transport, `metasystem/scripts/agents/dispatch.sh:2724-2725`; continuation source read at `metasystem/internal/dispatch/composition.go:300-310` | **Left alone** because it transports the whole return. Mutation data travels with it. It does not become the current Rules list. |
| Dispatch lost-return recollection, `metasystem/scripts/agents/dispatch.sh:1336-1349` | **Left alone** because it asks the role completeness owner. Recollection certifies delivery, not mutation coverage. |
| Supervisor recollection, `metasystem/internal/supervise/reaper.go:234-260`; bindings at `metasystem/cmd/metasystem/supervise_component.go:325-326` and `metasystem/internal/steward/reap.go:106-107` | **Left alone** for the same reason. No separate rule-coverage implementation. |
| Landed-return listing and validation, `metasystem/internal/mission/landed.go:101-121,131-155` | **Left alone** because its emitted rows use the shared job checker. It must not turn listing into review. |
| Round evidence mirroring, `metasystem/internal/dispatch/mirror.go:103-139` | **Left alone** because it copies every regular round artifact, including the extended return and composition. |
| Adapter selftests and permission evidence, `metasystem/internal/adapter/selftestrun.go:99-118,217,237,335-342`; Devin probe at `metasystem/internal/adapter/devin.go:337-338` | **Left alone** because selftests already use completeness and recursive string-value marker inspection. No schema-specific field enumeration is added. |
| Code critic skill, `metasystem/skills/code-critique/SKILL.md:25-37` | **Changes** because this human/model reader now samples mutation claims after mechanical review. |
| Implementer role, expected-return template, and correction template, `metasystem/scripts/agents/roles/implementer.md:11`; `metasystem/scripts/agents/templates/brief.md:32-40`; `metasystem/scripts/agents/templates/follow-up.md:12-18` | **Changes** because these describe and require the return. |
| Return-schema fixture scenario and conformance fixture bed, `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`; `metasystem/scripts/agents/conformance-fixtures.sh:73-107,175-186` | **Changes** with the named legs above. Preserve the old no-member cases. |
| Static return fixtures in `metasystem/scripts/validate-metasystem.sh:1986-2038,2389-2423`; fingerprint validation at `metasystem/scripts/agents/fingerprint-harness.sh:134` | **Left alone** because frozen version-1 and legacy version-2 acceptance remain supported. Their checks still call the shared validator. |

Readers for other roles are left alone: critic registration and closure (`metasystem/internal/dispatch/finding_register.go:80-82,137-175,652-663,1287`), critic clean-read admission (`metasystem/internal/dispatch/read_admission.go:400-432`), human-carried critic validation (`metasystem/internal/dispatch/review_reference.go:17-48`), critic merge checks (`metasystem/internal/validate/conformance.go:1170`), warden authorization (`metasystem/internal/validate/authorization.go:280-299`), recertification critic evidence (`metasystem/internal/validate/recertification.go:408`), landing critic evidence (`metasystem/internal/landing/observe.go:760`), and read-subject closure (`metasystem/internal/readsubject/closure.go:239`). They do not validate implementer mutation entries.

Steward continuation acceptance at `metasystem/internal/steward/reap.go:227-231` and orchestrator acceptance at `metasystem/internal/host/hostcollect.go:174` use their own roles. `metasystem/internal/missionrunner/adjudicate.go:71-88,120-126` validates a host/orchestrator return, not this implementer return. Leave these paths alone.

The benchmark pinned implementer schema and its drift comparison **change** as specified above (`benchmark/validate-kit.sh:84-92`; `benchmark/schemas/evidence/implementer.schema.json:4,111-125`). The extractor is **left alone because** the canonical schema lets its existing validator admit both old and extended returns (`benchmark/extractor.py:324-341,514-538`). Its old no-member fixture is **left alone because** compatibility must preserve it (`benchmark/extractor-fixtures.sh:260-274`). The rubric readers are **left alone because** their evidence and brief checks do not enumerate allowed return members (`benchmark/rubrics/evidence-honesty.md:15`; `benchmark/rubrics/brief-quality.md:19`). The shipped version-1 engine schema remains frozen. Benchmark scoring is unchanged.

## Build units and builder briefs

Land U1 first, then U2, U3, U4, and U5. Each unit depends only on already landed units. None depends on the Boundary/Ceiling sibling. Every ceiling counts additions plus deletions, including tests and instructions. Each unit must be at most 400 changed lines. The estimates below reserve space; they are not permission to exceed the ceiling. If the concrete diff exceeds it, return a gap for a smaller brief before landing.

In each Boundary list, `create:` marks a new output file; the following path is the exact authorized path. This uses the existing create-prefix recognition at `metasystem/internal/dispatch/brief.go:18,156-160`. Keep that marking on any repeated new-file reference in a dispatched brief, since an input use wins over an output use at `metasystem/internal/dispatch/brief.go:165-167`. It changes no Boundary/Ceiling policy.

These are implementation briefs. They authorize no commits by the builder. The seat integrates instruction changes and runs the fixture beds. The builder runs no fixture bed and never exports `METASYSTEM_BIN` for a Go run. This applies equally to baseline tests, mutation runs, race tests, and the fast gate. Preserve the adapter-provided cache environment. The existing brief tells the builder to use focused tests and the fast gate and leave work uncommitted at `metasystem/scripts/agents/templates/brief.md:14-20`.

For U2 onward, the returned `mutations` entry uses its Rule ID, the actual Location, the named Go test, and the actual observed `failureLine`. A table of proposed removals is not proof. First run the relevant test successfully. Apply one listed removal without editing its test. Run that test and retain its failure diagnostic. Restore the removal and rerun successfully before the next mutation. A skipped test or compile failure does not count. Temporary mutations must stay within the unit's Boundary and must all be restored before return.

The U1 dispatch has a bootstrap exception in representation only. Its launcher materializes the return schema before the builder changes code (`metasystem/scripts/agents/adapters/runtime-common.sh:103-110`). That old schema cannot accept a new top-level member. U1 therefore carries the entire mutation table as JSON text in one existing `evidence[].observed` string, headed `Mutation table for U1`, with `level: "ran"` and a real replayable command in `command`. That string contains an array with the same entry shape specified above and all final U1 row IDs. Its other evidence entries retain each individual run and result. U1 must not fabricate a top-level `mutations` field that its launch schema forbids. The seat reads this complete table, lands U1, and uses the rebuilt engine when dispatching U2. U2 onward carries the actual top-level array. No canonical return is rewritten to bridge this bootstrap.

### U1. Version-2 shape and compatible readers

Estimated change: 390 lines, including the seat-applied benchmark patch. This unit lands usable schema support while old returns still work.

```text
Working Mode: implement
Boundary: metasystem/internal/returnschema/returnschema.go, create: metasystem/internal/returnschema/implementer.go, create: metasystem/internal/returnschema/implementer_test.go, metasystem/internal/validate/returncomplete.go, create: metasystem/internal/validate/return_mutations_test.go, create: metasystem/internal/adapter/mutation_return_test.go, metasystem/cmd/metasystem/schema.go, create: metasystem/cmd/metasystem/schema_mutations_test.go, metasystem/scripts/agents/roles/implementer.md, create: metasystem/artifacts/reports/bprm-kit-compat.patch
Ceiling: 400 changed lines, additions plus deletions
Non-goals: brief parsing, review coverage, test lookup, repair prompts, role proof prose, template changes, fixture beds, direct benchmark edits, schema version bumps, commits
Rules: U1-array, U1-entry-fields, U1-test-fields, U1-closed-entry, U1-closed-test, U1-kind, U1-id, U1-location, U1-failure, U1-test-text, U1-provider-required, U1-role-members, U1-legacy, U1-role, U1-version, U1-preserve, U1-canonical, U1-canonical-scope
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement the `returnschema` signatures above. Wire the strict form into `Materialize`, the optional canonical form into `checkReturn`, and the canonical flag into the CLI. Prepare the exact benchmark compatibility patch for the seat. Add `mutations` to the role's enumerated member list in this same unit so a new provider schema does not contradict the role text. U2 adds the proof prose. Keep normalization code and version-1 files unchanged.

| Rule ID | Location to mutate | Named Go test and what it proves | Removal that must fail |
| --- | --- | --- | --- |
| U1-SHAPE | `returnschema.ImplementerVersionTwo` and `MutationEntriesSchema` | `TestImplementerMutationSchema`, plus `TestReturnMutationShape`: the materialized schema requires the field; acceptance rejects null, wrong types, missing location/test/failure, blank or multiline failure, extra keys, and unknown test kind; both location forms and a complete entry pass. | Remove each independent required member, closed-object rule, enum, or pattern separately. The corresponding named subtest must fail. Use the exact expanded row IDs below and keep all member-removal variants in evidence. |
| U1-ROLE-MEMBERS | implementer role member list | `TestImplementerMutationRoleMembers`: the role enumerates the new member required by the generated schema. Put this test in `mutation_return_test.go`. | Remove `mutations` from the role member list. |
| U1-LEGACY | `returnChecker.checkReturn` overlay selection | `TestReturnMutationsLegacy`: version-1 and version-2 returns without the member still pass; validation leaves an old return byte-identical. | Force the overlay on an omitted member. |
| U1-ROLE | `returnschema.Materialize` role/version guard | `TestMutationOverlayImplementerOnly`: only implementer v2 gains the field; v1 bytes and other role schemas retain their existing field sets. | Remove the implementer or version guard, one at a time. |
| U1-PRESERVE | `NormalizeReturn` call boundary and boundary serialization | `TestNormalizeReturnPreservesMutationEvidence`: the selected object's mutation array survives normalization, identity reconciliation, and a lawful diffBoundary rewrite; a missing array stays missing. | Drop the array from the normalized result immediately before the lawful boundary rewrite in `checkDiffBoundary`, which is in this unit's Boundary; the test must drive that acceptance path. |
| U1-CANONICAL | `CanonicalImplementerVersionTwo` and the CLI canonical branch | `TestCanonicalMutationSchema`: old omission and a complete extended return pass the canonical schema; malformed present evidence fails; provider output still requires the member. `TestSchemaCanonicalImplementer`: the real CLI emits that form and refuses other roles and versions. | Require the optional member, drop its constraints, or remove each CLI scope guard separately. |

The group labels in the table organize the tests. The Rules header already contains the exact acceptance row IDs expanded below. Test each individual removal and return its observed evidence.

Final verification, in this order:

```sh
go build ./...
go vet ./internal/returnschema ./internal/validate ./internal/adapter ./cmd/metasystem
go test -race -count=1 ./internal/returnschema ./internal/validate ./internal/adapter ./cmd/metasystem
scripts/agents/go-gate.sh --fast
```

Before the fast gate, apply the benchmark patch only to a disposable copy and run `bash -n` on that copy's `benchmark/validate-kit.sh`. The direct builder tree contains no changed shell script. The seat also runs `bash -n ../benchmark/validate-kit.sh` after applying the patch. Do not run a fixture bed.

### U2. Declare current-round rows and deliver the builder instruction

Estimated change: 390 lines. This unit makes Rules mechanically available and makes the builder obligation explicit. U1 already admits the requested return.

```text
Working Mode: implement
Boundary: metasystem/internal/dispatch/brief.go, create: metasystem/internal/dispatch/brief_rules_test.go, metasystem/internal/dispatch/composition.go, create: metasystem/internal/dispatch/composition_rules_test.go, metasystem/scripts/agents/role-packets.json, metasystem/scripts/agents/roles/implementer.md, metasystem/scripts/agents/roles/implementer.requirements.json, metasystem/scripts/agents/templates/brief.md, metasystem/scripts/agents/templates/follow-up.md, metasystem/internal/adapter/adjudicate.go, create: metasystem/internal/adapter/mutation_repair_test.go
Ceiling: 400 changed lines, additions plus deletions
Non-goals: Boundary/Ceiling parsing, test lookup, conformance, fixture beds, capability changes, commits
Rules: U2-extract, U2-trim, U2-order, U2-anchor, U2-absent, U2-empty, U2-headers, U2-duplicate, U2-blank, U2-id, U2-snapshot, U2-staged, U2-empty-snapshot, U2-continuation, U2-before-output, U2-role, U2-source, U2-requirements, U2-role-text, U2-brief-text, U2-followup-text, U2-repair-shape, U2-repair-delivery, U2-repair-validation
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement `BriefRules`, the `CompositionRecord.AcceptanceRules` field, and the early composition call. Make the role, requirements, recipe, and template edits specified above. Add the no-invention instruction to both repair prompt writers. The acceptance-row expansion below is binding.

| Rule ID | Location to mutate | Named Go test and what it proves | Removal that must fail |
| --- | --- | --- | --- |
| U2-PARSE | `dispatch.BriefRules` | `TestBriefRules`: an exact header yields ordered trimmed IDs; Markdown headings and indented examples do not become rows. | Remove extraction, trimming, ordering, or anchored-header recognition separately. |
| U2-EMPTY | `dispatch.BriefRules` empty case | `TestBriefRulesEmpty`: no header and an empty header each yield no rows. | Reject either lawful empty form. |
| U2-INVALID | `dispatch.BriefRules` refusal branches | `TestBriefRulesRefusals`: duplicate headers, duplicate IDs, blank items, invalid IDs refuse with their named diagnostic. | Disable one refusal at a time. |
| U2-CURRENT | `dispatch.ComposeRolePacket` before source assembly | `TestComposeImplementerRules`: current IDs persist for inline and staged briefs; no-row implementer records `[]`; earlier continuation IDs do not enter the current list; malformed header refuses before outputs; other roles are unaffected. | Parse assembled text, omit snapshot assignment, turn empty into absent, bypass parser error, or remove role guard separately. |
| U2-REPAIR | `writeRepairPrompt`, `writeDeliveryRepairPrompt`, `adjudicateTurnStage` | `TestMutationRepairDoesNotInventEvidence`: both prompts prohibit invented observations; initial malformed evidence requests repair; a repaired observed entry survives unchanged; a remaining malformed entry ends as protocol error. Use `AdjudicateTurn`, following `metasystem/internal/adapter/adjudicate_test.go:205-250`. | Remove each prompt instruction separately; bypass after-repair validation separately. |
| U2-DELIVER | implementer recipe and instruction files | `TestComposeMutationRequirement`: real composition delivers the exact proof sentence from the requirements source and its source record; role and both templates carry the instruction and Rules guidance; capability `required` remains empty. | Remove the selected requirements source, its sentence, or each other required instruction independently. |

Final verification:

```sh
go build ./...
go vet ./internal/dispatch ./internal/adapter
go test -race -count=1 ./internal/dispatch ./internal/adapter
scripts/agents/go-gate.sh --fast
```

No shell script changed. No fixture bed runs.

### U3. Refuse missing or mismatched evidence at review

Estimated change: 330 lines. This unit enforces the row join. Test existence arrives in U4, without making U3 depend on an unlanded helper.

```text
Working Mode: implement
Boundary: metasystem/internal/validate/conformance.go, create: metasystem/internal/validate/conformance_mutations.go, create: metasystem/internal/validate/conformance_mutations_test.go
Ceiling: 400 changed lines, additions plus deletions
Non-goals: named-test lookup, Boundary/Ceiling logic, merge/recertification policy, fixture beds, rewriting returns, commits
Rules: U3-member, U3-row, U3-shape, U3-object, U3-duplicate, U3-unknown, U3-current, U3-snapshot-invalid, U3-legacy-root, U3-legacy-followup, U3-reference, U3-missing-legacy, U3-read-error, U3-empty, U3-empty-current, U3-call, U3-before-artifacts, U3-immutable
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement `mutationRules`, `mutationEntries`, and `mutationViolations`, except the named-test loop. Wire the result into `reviewStage`. Use `newConformanceFixture` and its public `Conformance` call. The existing fixture constructs real controller and worktree repositories at `metasystem/internal/validate/conformance_test.go:48-85`. The acceptance-row expansion below is binding.

| Rule ID | Location to mutate | Named Go test and what it proves | Removal that must fail |
| --- | --- | --- | --- |
| U3-MISSING | `(*conformanceRun).mutationViolations` coverage join | `TestConformanceMutationMissingRow`: both a missing member and one missing row refuse, naming the row. | Skip coverage checking, or treat a nonmatching entry as coverage. |
| U3-SHAPE | `mutationEntries` | `TestConformanceMutationMalformed`: malformed data, incomplete entries, and nonobject returns refuse through review, without relying on earlier acceptance. | Bypass fragment validation or accept a nonobject return. |
| U3-JOIN | duplicate and unknown-ID branches | `TestConformanceMutationJoin`: duplicate and undeclared entries refuse with exact codes; a unique complete join passes. | Disable each branch separately. |
| U3-SOURCE | `(*conformanceRun).mutationRules` | `TestConformanceMutationRuleSource`: current composition wins over root/continuation Rules; malformed snapshot refuses; legacy round-1 and verified follow-up sources work; missing legacy source means no rows; other read errors refuse. | Replace current snapshot with root data, ignore a reference mismatch, or swallow each malformed-source error separately. |
| U3-EMPTY | zero-row early return | `TestConformanceMutationNoRowsUnchanged`: legacy no-member input passes byte-identically; explicit current empty rows do not inherit old rows. | Require entries for empty rows or fall back from an explicit empty list. |
| U3-REVIEW | `reviewStage` call and persistence order | `TestConformanceMutationReviewArtifacts`: a first refusal creates neither success artifact; complete join emits them; a changed repeat preserves existing immutable artifacts. | Remove the new call, persist before checking, or bypass existing-review refusal separately. |

Final verification:

```sh
go build ./...
go vet ./internal/validate
go test -race -count=1 ./internal/validate
scripts/agents/go-gate.sh --fast
```

No touched scripts. No fixture bed runs.

### U4. Check named tests in the candidate snapshot

Estimated change: 370 lines. This unit completes the review-stage contract by adding test lookup to U3.

```text
Working Mode: implement
Boundary: create: metasystem/internal/validate/conformance_mutations.go, create: metasystem/internal/validate/mutation_test_lookup.go, create: metasystem/internal/validate/mutation_test_lookup_test.go
Ceiling: 400 changed lines, additions plus deletions
Non-goals: test execution by conformance, shell parsing framework, new dependencies, fixture beds, symbol resolution for rule locations, commits
Rules: U4-candidate, U4-controller, U4-deleted, U4-frozen, U4-prefix, U4-clean-path, U4-mode, U4-suffix, U4-go-name, U4-go-decl, U4-go-signature, U4-go-import, U4-go-platform, U4-fixture-case, U4-fixture-scenario, U4-fixture-literal, U4-fixture-exact, U4-error, U4-absent, U4-call
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Implement the three lookup signatures and the loop in `mutationViolations`. Use the precise source-membership definitions above. The acceptance-row expansion below is binding.

| Rule ID | Location to mutate | Named Go test and what it proves | Removal that must fail |
| --- | --- | --- | --- |
| U4-TREE | `(*conformanceRun).mutationTestExists` snapshot read | `TestMutationTestUsesCandidateTree`: a new uncommitted candidate test counts; a controller-only test does not; deletion counts as absence; a post-snapshot filesystem change cannot alter lookup. | Read HEAD, the controller, or live workspace bytes instead of the supplied tree, separately. |
| U4-PATH | path conversion and entry-mode checks | `TestMutationTestPaths`: exact nested path strips once; spaces are accepted; traversal, sibling paths, symlinks, directories, and gitlinks refuse. | Remove each containment/mode check or strip the prefix twice. |
| U4-GO | `goMutationTestExists` | `TestGoMutationTestExists`: the named top-level test passes with ordinary, aliased, and dot-imported testing; absent names, comments, methods, helpers, invalid signatures, TestMain with *testing.M, benchmarks, and subtest-only names do not; platform-tagged source counts. | Remove each predicate separately, or replace AST inspection with substring search. |
| U4-FIXTURE | `fixtureMutationTestExists` | `TestFixtureMutationTestExists`: exact literal `new_case` and positive scenario selectors count, including quoting and spacing; comments, prefixes, negative comparisons, and variable names do not. | Disable either supported form or relax each false-positive guard separately. |
| U4-ERROR | lookup error mapping | `TestMutationTestInspectionFailure`: unreadable Git objects and malformed Go source refuse as unreadable; absence refuses as absent. | Convert an inspection error or absence into success, separately. |
| U4-WIRE | `mutationViolations` named-test loop | `TestConformanceMutationAbsentTest`: public review names the missing test and row; changing only the name to an existing test admits the same complete return. | Remove the lookup call or ignore its false result. |

Final verification:

```sh
go build ./...
go vet ./internal/validate
go test -race -count=1 ./internal/validate
scripts/agents/go-gate.sh --fast
```

No touched scripts. No fixture bed runs.

### U5. Put the contract in the fixture beds and critic

Estimated change: 230 lines. This unit adds the four engine-driven witnesses and changes the critic's instruction after the mechanical checks exist.

```text
Working Mode: implement
Boundary: metasystem/scripts/agents/conformance-fixtures.sh, metasystem/scripts/agents/return-schema-fixtures.sh, metasystem/skills/code-critique/SKILL.md, create: metasystem/internal/validate/mutation_contract_test.go
Ceiling: 400 changed lines, additions plus deletions
Non-goals: production validator changes, testing.json changes, running fixture beds, recreating the builder battery in the critic, commits
Rules: U5-missing-row, U5-absent-test, U5-complete, U5-no-rows, U5-schema-preserve, U5-schema-invalid, U5-schema-legacy, U5-sample, U5-material, U5-record, U5-disposable
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Add the four exact conformance legs and the three return-schema checks above. Add the critic text. Keep Bash limited to fixture setup and invoking the engine. Do not move schema or matching logic into Bash. The acceptance-row expansion below is binding.

| Rule ID | Location to mutate | Named Go test and what it proves | Removal that must fail |
| --- | --- | --- | --- |
| U5-LEGS | the four `new_case` declarations | `TestMutationFixtureContract`: the four required named legs remain in the existing script and the existing testing group still targets that bed. This is a fixture-registration check, not a claim that the legs ran. | Remove each required declaration independently. |
| U5-SCHEMA | return-schema scenario's three added checks | `TestMutationReturnFixtureContract`: the normalize-preservation, malformed-entry, and legacy-pass checks remain in `implementer-v1-v2`. Give each check an explicit literal diagnostic so the contract test can identify its assertion. | Remove each required assertion block independently. |
| U5-CRITIC | code critique Layer 1 mutation paragraph | `TestMutationCriticContract`: the critic is instructed to sample, classify non-reproduction as material, record results, and restore a disposable tree. | Remove each required instruction independently. |

The registration and instruction tests are limited contract checks. The seat's engine runs prove fixture behavior. The production Go tests in U1, U3, and U4 prove the corresponding semantics without a builder running a bed.

Final verification:

```sh
go build ./...
go vet ./internal/validate
go test -race -count=1 ./internal/validate
bash -n scripts/agents/conformance-fixtures.sh scripts/agents/return-schema-fixtures.sh
scripts/agents/go-gate.sh --fast
```

The builder does not run either fixture bed and never exports `METASYSTEM_BIN` for any Go run.

## Acceptance-row expansion for the builder briefs

The headers above use these exact row IDs. Each ID is one acceptance row and needs one return entry, using the U1 bootstrap representation when necessary. Its test function is the one assigned to its parent group in the table. Its `location` and `failureLine` must name the particular constraint and observed failure. The acceptance text below is the mutation-table expansion; copy it with each brief. Do not ask the builder to invent the decomposition.

| Unit | Exact final Rules value | Acceptance rows and individual removals |
| --- | --- | --- |
| U1 | `U1-array, U1-entry-fields, U1-test-fields, U1-closed-entry, U1-closed-test, U1-kind, U1-id, U1-location, U1-failure, U1-test-text, U1-provider-required, U1-role-members, U1-legacy, U1-role, U1-version, U1-preserve, U1-canonical, U1-canonical-scope` | Array type; all four entry fields; all three test fields; closed entry; closed test; kind enum; ID pattern; location pattern; nonblank single-line failure; nonblank single-line test path/name; provider-required root field; matching role member list; old omission; role guard; version guard; evidence preservation; optional canonical schema with identical mutation constraints; canonical CLI limited to implementer version 2. For member lists and shared string schemas, delete each declared member or pattern use in a separate run and record the corresponding subtest diagnostic in evidence. |
| U2 | `U2-extract, U2-trim, U2-order, U2-anchor, U2-absent, U2-empty, U2-headers, U2-duplicate, U2-blank, U2-id, U2-snapshot, U2-staged, U2-empty-snapshot, U2-continuation, U2-before-output, U2-role, U2-source, U2-requirements, U2-role-text, U2-brief-text, U2-followup-text, U2-repair-shape, U2-repair-delivery, U2-repair-validation` | Extract IDs; trim tokens; preserve order; exact header anchor; absent header; empty header; duplicate headers; duplicate IDs; blank item; invalid ID; current snapshot; staged-current snapshot; explicit empty snapshot; continuation isolation; refusal before publication; implementer-only parsing; source selection; requirements sentence; role contract; brief contract; follow-up contract; ordinary repair instruction; delivery repair instruction; validation after repair. |
| U3 | `U3-member, U3-row, U3-shape, U3-object, U3-duplicate, U3-unknown, U3-current, U3-snapshot-invalid, U3-legacy-root, U3-legacy-followup, U3-reference, U3-missing-legacy, U3-read-error, U3-empty, U3-empty-current, U3-call, U3-before-artifacts, U3-immutable` | Missing member; missing row; fragment shape; return object type; duplicate entry; unknown ID; current snapshot precedence; malformed snapshot; legacy root source; legacy follow-up source; reference mismatch; missing legacy source compatibility; other read failures; no-row compatibility; explicit empty current list; review call; check before artifact persistence; immutable existing review. |
| U4 | `U4-candidate, U4-controller, U4-deleted, U4-frozen, U4-prefix, U4-clean-path, U4-mode, U4-suffix, U4-go-name, U4-go-decl, U4-go-signature, U4-go-import, U4-go-platform, U4-fixture-case, U4-fixture-scenario, U4-fixture-literal, U4-fixture-exact, U4-error, U4-absent, U4-call` | Candidate uncommitted test; controller exclusion; deletion; frozen tree; one prefix strip; clean contained path; regular blob mode; kind/file suffix; Go name rule; top-level declaration; Go test signature; testing import resolution; platform-independent source membership; new_case form; positive scenario form; literal quoting; whole selector matching; inspection error; absence; public review call. For signature and mode alternatives, mutate each check independently and retain all failures in evidence. |
| U5 | `U5-missing-row, U5-absent-test, U5-complete, U5-no-rows, U5-schema-preserve, U5-schema-invalid, U5-schema-legacy, U5-sample, U5-material, U5-record, U5-disposable` | Each of the four exact conformance legs; each of the three return-schema assertions; critic sampling; material non-reproduction; sampled-result recording; disposable-copy restoration. |

Some rows have several data variants, such as each required JSON field. The one rule is that the declared set is complete. Prove every member-removal variant and retain every observed line in `evidence`; `failureLine` in the row is one representative actual diagnostic. If implementation introduces an independent new rule beyond this inventory, it is a brief gap, not permission to bury it inside a broad row.

## Seat verification and remaining proof

After U1, the seat applies and reads back its exact benchmark compatibility patch. It materializes the canonical implementer schema into a temporary artifact and uses `cmp` against `../benchmark/schemas/evidence/implementer.schema.json`; they must be byte-identical. It also confirms that only the implementer branch of the kit drift loop requests `--canonical`. The existing engine extra-suite route runs `../benchmark/evidence-drift-fixtures.sh`, configured at `metasystem/metasystem.conf:105`; include `section/project-extra-suites` in the seat's diagnostic selection for this reader change. The full kit gate remains outside the engine by its own dependency rule at `benchmark/validate-kit.sh:2-6`. This goal adds no engine dependency on the kit. After U5, the seat runs both required existing beds through the engine. A diagnostic engine invocation, from `metasystem`, is:

```sh
bin/metasystem test run --root . --goal builder-proves-each-rule-by-mutation --mode canary --purpose diagnostic --groups section/return-schema-fixtures,section/conformance-fixtures,section/project-extra-suites --json
```

These flags and diagnostic group selection are implemented at `metasystem/cmd/metasystem/test.go:148-183`. The seat follows with the goal's selected delivery proof, using `test plan`, `test run`, and `test verify` as required by `metasystem/docs/project-rules.md:17` and `metasystem/docs/design/design-obligation-gate.md:15-17`. A diagnostic run does not replace that delivery receipt. No builder exports a fixture binary into its Go environment.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| BPRM-SHAPE | HIGH | Return member | Typed mutation evidence with legacy omission support | returnschema and returnChecker | U1 targets | U1 tests and mutations | return-schema bed | PARTIAL | Build U1 |
| BPRM-ROWS | HIGH | Acceptance rows | Current brief IDs become round evidence before continuation assembly | dispatch composition | U2 targets | U2 tests and mutations | composed implementer packet | PARTIAL | Build U2 |
| BPRM-JOIN | HIGH | Review check | Missing, duplicate, and unknown rows refuse without return repair | conformance review | U3 targets | U3 tests and mutations | bprm-missing-row and bprm-no-rows | PARTIAL | Build U3 |
| BPRM-TEST | HIGH | Test lookup | Named Go or fixture test exists in the reviewed candidate | conformance lookup | U4 targets | U4 tests and mutations | bprm-absent-test and bprm-complete | PARTIAL | Build U4 |
| BPRM-HONESTY | HIGH | Builder and critic wording | Builder observes failure before return; repair invents none; critic samples reproduction | implementer and independent critic | U2, U5 targets | prompt and contract tests | builder mutation returns and critic sample evidence | PARTIAL | Seat reads actual evidence |

The design contract is specified, with owners, refusal behavior, tests, fixture legs, and ordered build briefs. The implementation and runtime proof are pending. Source checks were read-only. No independent design critique is claimed. The seat can send this artifact through its design review before implementation.

There are no questions reserved for Wido in this design. The material limitation is explicit: schema and source membership cannot prove an observed causal failure. A missing brief row or a fabricated observation still needs the critic's conformance review and reproduction sample. The goal moves the full battery to the builder; it does not add a trusted mutation runner.

Proposed receipt for the seat's integration commit: `design builder-proves-each-rule-by-mutation: specified per-row mutation evidence, candidate-tree existence checks, unchanged no-row acceptance, honest repair, four conformance fixtures, and five units capped at 400 changed lines; source-grounded, implementation proof pending`.
