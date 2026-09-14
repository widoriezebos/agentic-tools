# Design critique, round 1

Reviewed design: `artifacts/reports/bdrb-design.md`

Reviewed commit: `91f097429f138cf51d427d2666fba4958899047e`

Evidence level: checked by reading. I did not run tests or fixture beds because this is a design review and no runtime code changed.

Material finding count: 8.

## Material findings

### BDRB-R1-01

Severity: high

Material: yes

Claim: The design says, "Project declarations with the existing `projectDeclaration`, then use this matcher" and later says, "Pass repository paths to the new check and project paths to the old one" (`artifacts/reports/bdrb-design.md:55`, `artifacts/reports/bdrb-design.md:131`).

Evidence: `projectDeclaration` removes `metasystem/` and returns a project-relative value in a nested installation (`metasystem/internal/validate/conformance.go:284-292`). The repository-level workspace returns repository-relative changed paths such as `metasystem/internal/a.go` (`metasystem/internal/validate/conformance.go:315-329`, `metasystem/internal/gittree/gittree.go:372-386`). The design never says to reject an outside-project path first and convert an inside-project changed path with `installationPath` before matching (`metasystem/internal/validate/conformance.go:86-95`).

An implementer can therefore compare `internal/a.go` with `metasystem/internal/a.go`. Every lawful nested-project change then appears outside Boundary. The special whole-project declaration `metasystem/` projects to the empty string, so an implementation that treats the empty directory prefix as match-all can also treat a sibling repository path as inside Boundary. The existing project fence still refuses the sibling, but the required typed Boundary violation would omit it. This contradicts the design's statement that an outside-installation path is outside every valid brief declaration. `TestNestedReviewStageSpeaksProjectSpace` records the current project-relative comparison contract (`metasystem/internal/validate/nested_conformance_test.go:38-101`), while `TestReviewRefusesSiblingChanges` records the separate repository fence (`metasystem/internal/validate/nested_conformance_test.go:138-179`). The design must settle the conversion and outside-project guard before implementation.

### BDRB-R1-02

Severity: high

Material: yes

Claim: The design says, "With only Boundary, run only the new path check. With only Ceiling, run only the new size check" and calls this "the literal optional-header contract" (`artifacts/reports/bdrb-design.md:25`).

Evidence: DONE specifies a brief carrying Boundary and Ceiling, then exempts a brief "without the headers" (`metasystem/plans/goals/brief-declares-the-round-boundary.md:8`). It does not admit either half by itself. Current preflight has only the required Working Mode grammar, so current code supplies no partial-header precedent (`metasystem/internal/dispatch/brief.go:37-55`).

This widens the goal. A Boundary-only brief has no size ceiling. A Ceiling-only brief has no seat-authored outer path permission. An implementer following the design will accept both states even though DONE only defines both present or both absent. The design needs an explicit contract ruling for partial presence and fixtures for the chosen outcome.

### BDRB-R1-03

Severity: high

Material: yes

Claim: The design says, "For each unit, run the named negative tests once with just the corresponding rule removed" (`artifacts/reports/bdrb-design.md:203`). Unit 5 permits changes only to two test files and says, "no production rules" (`artifacts/reports/bdrb-design.md:310-315`).

Evidence: The rules that Unit 5's tests exercise live in `reviewStage`, `projectDeclaration`, the project fence, and the immutable artifact path (`metasystem/internal/validate/conformance.go:276-293`, `metasystem/internal/validate/conformance.go:305-348`, `metasystem/internal/validate/conformance.go:351-428`). None of those production files is in Unit 5's Boundary. The design also says to stop before writing outside a unit Boundary (`artifacts/reports/bdrb-design.md:199`).

The Unit 5 builder cannot remove a corresponding production rule, observe the new test fail, and restore the rule without writing outside its Boundary. It must violate one instruction or omit the required mutation witness. The design needs a lawful mutation method or a different unit boundary.

### BDRB-R1-04

Severity: high

Material: yes

Claim: The design says, "for every rule you add, a test that fails when that rule alone is removed" and requires a focused mutation witness for each new rule (`artifacts/reports/bdrb-design.md:203-213`).

Evidence: The named tests do not cover every stated rule:

- No named test distinguishes exact case and column-zero header recognition, headers inside fences, or trimming from a broader parser. The current Working Mode test covers only one valid value, absence, duplication, and a placeholder (`metasystem/internal/dispatch/decisions_test.go:343-361`).
- No named test fails if review takes two snapshots. The current code takes separate project and repository snapshots (`metasystem/internal/validate/conformance.go:368-376`). `TestNestedReviewStageSpeaksProjectSpace` checks the resulting tree, not how many candidate snapshots supplied it (`metasystem/internal/validate/nested_conformance_test.go:74-101`). The proposed `TestReviewBriefUsesMergeBaseCandidate` mutates before review, not between snapshots.
- No named test fails if a headerless review still runs numstat. The proposed headerless leg observes only a successful result, so an unnecessary successful count remains invisible.
- No named test requires Boundary before Ceiling when both fail, an untruncated full path list, or the review-level `BRIEF_BOUNDS_UNREADABLE` and `BRIEF_LINES_UNREADABLE` wrappers. The listed tests check presence of both failures or primitive read/count failures, not those exact review contracts.
- No named test fails if the seat-ordering sentence is removed from `docs/orchestration.md` or `skills/code-critique/SKILL.md`. The current static template audit checks existing header presence and follow-up headings, not this ordering rule (`metasystem/scripts/validate-metasystem.sh:1614-1629`). `TestConformanceBriefCLIWithoutCritic` proves that the command can run without a critic. It does not prove that a seat runs it before dispatch.

These omissions change the required test suite. They also make the claimed remove-one proof impossible to audit from the named plan. The proof matrix must name a failing witness for each rule or narrow the rule claim.

### BDRB-R1-05

Severity: medium

Material: yes

Claim: The design says, "If task-direction is referenced, require one corresponding reference and read it through `dispatch.ReadVerifiedReference`" (`artifacts/reports/bdrb-design.md:67`).

Evidence: `ReadVerifiedReference` checks only that its own `OpenPath` stays inside the root, is a regular readable file, and matches the digest and byte count declared by that same reference (`metasystem/internal/dispatch/references.go:24-49`). It does not prove that `Path` names that `OpenPath`, or that the reference digest and byte count equal the task-direction source's `SourceDigest` and `SourceBytes`. The existing composition validator defines a corresponding reference by exactly that source-to-reference digest and byte-count join (`metasystem/internal/dispatch/build.go:1042-1077`). `TestJobRecordRejectsExpandedOrDishonestComposition` already proves that an unbound reference is dishonest (`metasystem/internal/dispatch/composition_test.go:1167-1221`). The proposed corrupt-source test names duplicate or missing references and changed referenced bytes, but not an unbound valid reference (`artifacts/reports/bdrb-design.md:274`).

"Corresponding" is underspecified. An implementer can select by slot alone and accept a different valid referenced body. That can apply the wrong round boundary. The design must require the existing source-reference binding facts and name the unbound-reference test.

### BDRB-R1-06

Severity: medium

Material: yes

Claim: The design says, "Documentation examples should indent their header lines when embedded in a real brief" and says only "a recognized Boundary line" becomes output-only authority (`artifacts/reports/bdrb-design.md:19`, `artifacts/reports/bdrb-design.md:57`).

Evidence: Authority extraction scans every line. It has no fence or quotation state. A path token is an input unless the line is an existing Workspace output line, follows a create prefix, or matches the special `diffBoundary` example exemption (`metasystem/internal/dispatch/brief.go:121-167`). An indented `Boundary:` line is deliberately not a recognized header, so its path tokens take the default input branch.

The prescribed way to show an inert Boundary example can therefore make preflight require every example path to exist in the committed tree. That breaks briefs whose examples use a new file or placeholder path. The design must say how authority extraction identifies an inert Boundary example, and it needs a named test for that exact form.

### BDRB-R1-07

Severity: medium

Material: yes

Claim: The design says JSON permits paths containing spaces, commas, tabs, quotes, or newlines, and also says, "A path also cited as an input elsewhere must still exist" (`artifacts/reports/bdrb-design.md:21`, `artifacts/reports/bdrb-design.md:57`).

Evidence: The current authority token grammar excludes spaces, commas, quotes, and control characters (`metasystem/internal/dispatch/brief.go:13-18`). Extraction operates on those tokens, not JSON-decoded Boundary members (`metasystem/internal/dispatch/brief.go:141-167`). A later input citation of `metasystem/internal/new file.go` is split into fragments and cannot make the exact Boundary member's input use win.

An implementation limited to marking the Boundary line output-only cannot satisfy the stated input-wins rule for the path forms the new header explicitly admits. It can fail open on the real path or falsely refuse a fragment. The design must define the shared path identity used by Boundary and authority extraction, then name a special-character input test.

### BDRB-R1-08

Severity: medium

Material: yes

Claim: The design says, "Glob characters in a trailing-slash declaration are literal directory-name characters; directory prefixes are not glob expansion" and also says, "Validate glob syntax with `path.Match`" (`artifacts/reports/bdrb-design.md:48`, `artifacts/reports/bdrb-design.md:51`).

Evidence: Current conformance gives every declaration exact map membership after prefix projection. It has no glob parser that resolves this conflict (`metasystem/internal/validate/conformance.go:798-810`).

For a trailing directory such as `metasystem/a[/`, the first rule makes `[` literal. An unconditional `path.Match` syntax check rejects it as malformed. The named parser and matcher tests do not state which outcome they require. An implementer must guess whether trailing directory declarations bypass glob validation. That changes accepted Boundary values and needs one explicit rule and test.

## Rigor rows for material findings

| findingId | rigorClass | facts | reopeningTrigger |
| --- | --- | --- | --- |
| BDRB-R1-01 | unproven | local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if the folded design still compares projected declarations with unprojected repository paths or leaves whole-project matching ambiguous. |
| BDRB-R1-02 | severe | local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if partial header presence remains admitted without a human-backed contract clause and fixtures. |
| BDRB-R1-03 | unproven | local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if Unit 5 still requires mutation outside its declared Boundary. |
| BDRB-R1-04 | severe | local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if any stated rule still lacks a named test that fails when only that rule is removed. |
| BDRB-R1-05 | severe | local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if referenced task direction is not joined to its source metadata by slot, digest, and byte count. |
| BDRB-R1-06 | unproven | local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if the recommended indented example can still become cited input authority. |
| BDRB-R1-07 | severe | local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if special-character Boundary paths and input citations still use different identities. |
| BDRB-R1-08 | unproven | local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false | Reopen if trailing directory declarations remain both literal and subject to glob syntax. |

## Non-material findings

### BDRB-R1-N01

Severity: low

Material: no

Claim: The reader inventory presents the task-direction readers as complete (`artifacts/reports/bdrb-design.md:157-182`).

Evidence: The search also finds the composition record reader in `metasystem/internal/dispatch/build.go:1099-1103`, plus the fake adapter's referenced task-direction discovery and tamper hook in `metasystem/scripts/agents/adapters/fake.sh:67` and `metasystem/scripts/agents/adapters/fake.sh:231-232`. The build reader is materially relevant to source-reference binding and is covered by BDRB-R1-05. The fake adapter sites do not interpret the new headers and need no production change, so their omission alone does not change what gets built.

### BDRB-R1-N02

Severity: low

Material: no

Claim: The design calls the proof "four legs" (`artifacts/reports/bdrb-design.md:184-195`).

Evidence: `brief-no-headers` also requires "a fresh companion case" for the undeclared-file refusal (`artifacts/reports/bdrb-design.md:193`). The bed's `new_case` helper creates one isolated controller and worktree per case (`metasystem/scripts/agents/conformance-fixtures.sh:38-64`). The design therefore names four behaviors but requires five fresh executions. This is harmless extra proof and does not change production behavior.

## Seam and existing-test audit

| Seam | Checked result | Existing test or bed that constrains it |
| --- | --- | --- |
| `reviewStage` ordering | The current stage takes snapshots, rejects sibling paths, computes project diff and paths, applies the cumulative declaration check, then writes artifacts. The design correctly places new checks before writes. Headerless ordering must remain exact. | `TestConformanceReviewAndCritiqueMerge`, `TestConformanceReviewRefusesToOverwriteRoundEvidence`, `TestConformanceReviewIdenticalRerunIsIdempotent`, `TestReviewRefusesSiblingChanges`, and `section/conformance-fixtures`. |
| `projectDeclaration` reuse | Prefix validation and one-strip behavior are real. The design's changed-path space is not settled, as BDRB-R1-01 explains. | `TestNestedDeclarationDialectIsMandatory`, `TestNestedReviewStageSpeaksProjectSpace`, and `TestConformanceTopLevelBoundarySurvivesReturnValidation`. |
| One candidate snapshot | `Snapshot` captures committed, staged, unstaged, deleted, and untracked unignored content through an isolated index. `TreeOf` can peel the nested project subtree from the repository tree. Replacing the two current snapshots is sound if the same tree identifier supplies paths, numstat, patch, and `reviewedTree`. The proof gap is BDRB-R1-04. | `TestNestedReviewStageSpeaksProjectSpace`, `TestReviewRefusesSiblingChanges`, `TestReviewStageWritesOnlyThreeFields`, and the `internal/gittree` snapshot tests. |
| Existing cumulative `diffBoundary` check | The helper unions concrete return declarations and checks exact project-relative paths. Keeping it is correct. It must continue to run separately from the new outer Boundary. | `TestConformanceReviewAndCritiqueMerge`, `TestNestedDeclarationDialectIsMandatory`, `TestConformanceTopLevelBoundarySurvivesReturnValidation`, and recertification closure tests. |
| Boundary as output-only authority | Current input use wins over current output declarations for extractable paths. The new rule is incomplete for indented examples and special-character paths, as BDRB-R1-06 and BDRB-R1-07 explain. | `TestBriefAuthorityExemptsDeclaredOutputsUnlessTheyAreAlsoInputs`, `TestBriefAuthorityTreatsMetasystemDiffBoundaryExampleAsPrefixOnly`, and the proposed two new authority tests. |
| Current round source range | Composition records the task direction first, with delivered byte ranges and digests. Inline range selection is sound. Referenced selection needs the existing source-reference join from BDRB-R1-05. Legacy root and follow-up paths match current evidence readers. | `TestComposeRolePacketFollowsClosedRecipeAndRecordsEveryRange`, `TestComposeRolePacketReferencesABriefOverMaxDirectiveBytes`, `TestJobRecordRejectsExpandedOrDishonestComposition`, `TestVerifyReferencesRefusesAChangedFileByName`, and `TestExhaustionDisciplineReadsReferencedSuccessorTaskDirection`. |

No existing named test or bed must break under the intended conditional behavior. Headerless cases are the compatibility guard. The findings above identify places where the design does not determine an implementation that can preserve those guards.

## DONE contract audit

| DONE clause | Result |
| --- | --- |
| Boundary names repository paths, with directories and globs | Specified, but nested matching is not implementable as written because declaration and changed-path spaces differ. See BDRB-R1-01. The trailing-directory grammar is also contradictory. See BDRB-R1-08. |
| Ceiling is additions plus deletions | Satisfied. It uses merge-base to one repository snapshot and numstat, with binary rows excluded as requested. |
| Both headers are extracted in `internal/dispatch/brief.go` beside Working Mode | Satisfied for the both-present and both-absent forms. The design widens the contract to partial presence without authority. See BDRB-R1-02. |
| Review refuses outside paths or a count over Ceiling | Intended, with concrete error types and pre-artifact placement. Nested path comparison remains wrong or ambiguous. |
| Refusal names every offending path and the count | Intended for ordinary UTF-8 paths. Boundary and Ceiling errors carry paths and the total count. The nested whole-project case can omit a Boundary offender unless BDRB-R1-01 is fixed. |
| The seat checks the typed violation before dispatching a read | The design adds types, CLI output, and seat instructions. It deliberately does not add a dispatch gate. That matches the actor named in DONE, but its ordering rule has no remove-one witness. See BDRB-R1-04. Current late failure is proved by `TestCritiqueSubjectDiffRefusalsNameReviewedRoundAndPath` (`metasystem/internal/dispatch/severity_tiered_rigor_test.go:198-223`). |
| Headerless briefs are admitted unchanged | Satisfied for readable current or legacy evidence. The design keeps the old checks and no default ceiling. Present but unreadable composition refuses because absence cannot be proved. |
| Four fixture outcomes | All four named outcomes are present. The headerless leg adds a fifth fresh case to prove the existing cumulative check, which is harmless. |

The design narrows no stated successful case on purpose. It widens DONE by admitting single-header briefs. Its nested path-space ambiguity can also narrow lawful bounded briefs in practice.

## Unit-size and remove-one audit

| Unit | Planned changed lines | At most 400 on the plan | Remove-one status |
| --- | ---: | --- | --- |
| 1 | About 380 | Yes, with 20 lines of margin | Incomplete for case, column, fenced-example, and special-character authority rules. |
| 2 | About 300 | Yes, with 100 lines of margin | The named tests cover arithmetic, binary rows, renames, odd filenames, supplied trees, Git failure, nesting, and diff drivers. |
| 3 | About 360 | Yes, with about 40 lines of margin if the register row is included in production allocation | Missing the source-reference binding witness from BDRB-R1-05. |
| 4 | About 375 | Yes, with 25 lines of margin if registration and rewiring fit the production allocation | Missing exact diagnostic ordering, full-list, unreadable-wrapper, and no-extra-numstat witnesses. |
| 5 | At most 360 | Yes, with 40 lines of margin | Its mutation instruction conflicts with its test-only Boundary. See BDRB-R1-03. It also lacks a witness that detects two snapshots. |
| 6 | About 320 | Yes, with 80 lines of margin | The cap paragraph has a named test. The seat-ordering and template instruction changes have none. |

No arithmetic estimate exceeds 400. These are allocations, not measurements. The hard stop-and-split sentence prevents an oversized final unit, but Units 1 through 3 cannot yet rely on the new engine check because enforcement lands in Unit 4.

## Reader inventory and overlap

The independent searches covered `diffBoundary`, `Working Mode`, `brief_mode`, `brief_authority`, and `task-direction` under `metasystem/internal`, `metasystem/cmd`, `metasystem/scripts`, and `metasystem/skills`.

All production readers of `diffBoundary` are accounted for: cumulative conformance, return normalization, follow-up rebase overlap, design-critique pairing, fake implementer production, the implementer schema and role, and shell return-schema checks. All production Working Mode readers and emitters are accounted for. The missing task-direction reader and fake adapter sites are recorded in BDRB-R1-N01.

`member-size-gate` does not collide with this implementation order. It acts before build, while this design checks actual output after build. A gap remains for their future composition: one Boundary directory or glob can authorize many actual files, while the member-size goal promises to bound a member's file count before build (`metasystem/plans/goals/member-size-gate.md:8`). The current concrete `diffBoundary` check still catches undeclared actual files after build (`metasystem/internal/validate/conformance.go:754-817`), but neither design says how a pre-build file-count gate counts a Boundary pattern. This is not a reason to change this goal's required glob support. The later member-size design must own the expansion or require a separate concrete inventory.

`cross-cutting-change-inventories-its-readers` shares `internal/dispatch/brief.go` and the brief grammar, but there is no current semantic collision. Boundary is output permission. Future `Readers:` and `Layers on:` declarations are input and seam authority. This design names the layers it preserves, including `projectDeclaration`, `cumulativeBoundaryViolations`, the project fence, protected paths, composition records, and the tier-1 limit. The later goal should reuse one header scanner and keep only Boundary output-only. It must not infer reader inventory from Boundary globs (`metasystem/plans/goals/cross-cutting-change-inventories-its-readers.md:8-10`).

## File and line claim audit

Every cited location in the design was opened in this worktree. "Holds" means the cited code supports the sentence. "Partial" names the material exception above.

| Design line | Cited current location | Result |
| ---: | --- | --- |
| 5, 25 | `metasystem/plans/goals/brief-declares-the-round-boundary.md:8` | Holds as the DONE source. The design's partial-header reading is not stated there. |
| 7 | `metasystem/internal/validate/conformance.go:232-241`, `metasystem/internal/validate/conformance.go:368-390` | Holds. Review derives merge-base from target HEAD and currently snapshots and compares the worktree. |
| 19, 25, 159 | `metasystem/internal/dispatch/brief.go:41-55`, `metasystem/internal/dispatch/brief.go:47-53`, `metasystem/internal/dispatch/brief.go:52-55` | Holds. Working Mode uses exact column-zero prefix scanning and is mandatory. |
| 40, 161 | `metasystem/cmd/metasystem/dispatch_verbs.go:1993-2027`, `metasystem/cmd/metasystem/dispatch_verbs.go:2014-2027` | Holds. Authority and mode errors use `recordExit`, and mode alone is printed on success. |
| 40, 162 | `metasystem/scripts/agents/dispatch.sh:1542` | Holds. The current suffix mentions only Working Mode. |
| 44, 170 | `metasystem/internal/validate/conformance.go:276-293` and `metasystem/internal/validate/conformance.go:284-293` | Holds. Nested declarations require one literal prefix and are stripped once. |
| 53, 133 | `metasystem/internal/validate/conformance.go:798-810` and `metasystem/internal/validate/conformance.go:798-803` | Holds. Current return declarations use exact map membership and retain wrong-prefix refusal. |
| 55 | `metasystem/internal/validate/conformance.go:242-246` | Holds. Review derives the installation prefix before the stage. |
| 57, 160 | `metasystem/internal/dispatch/brief.go:62-104`, `metasystem/internal/dispatch/brief.go:83-104`, `metasystem/internal/dispatch/brief.go:121-171`, `metasystem/internal/dispatch/brief.go:155-167` | Holds for current extractable paths. The new indented and special-character cases are unresolved. |
| 61, 163 | `metasystem/internal/dispatch/composition.go:268`, `metasystem/internal/dispatch/composition.go:380-390` | Holds. Task direction is the first caller brief source and gets byte ranges and digests. |
| 61 | `metasystem/scripts/agents/dispatch.sh:2771-2777`, `metasystem/scripts/agents/dispatch.sh:1924-1929`, `metasystem/scripts/agents/dispatch.sh:2883-2888` | Holds. Initial and follow-up prompts and composition records are persisted. |
| 66 | `metasystem/internal/dispatch/composition.go:368-390` | Holds. A delivered section includes its heading, body, terminating newline, range, and digest. |
| 67 | `metasystem/internal/dispatch/references.go:22-49` | Partial. It checks root containment, regular-file status, bytes, and digest, but not `Path` binding or source-to-reference binding. See BDRB-R1-05. |
| 68 | `metasystem/scripts/agents/dispatch.sh:1918`, `metasystem/internal/validate/conformance.go:925-934` | Holds. Initial dispatch stores `brief.md`; the legacy successor fallback reads `prompt.md`. |
| 69, 164 | `metasystem/internal/validate/conformance.go:914-960`, including `metasystem/internal/validate/conformance.go:942-954` | Holds. The unchanged successor reader returns all inline prompt bytes and switches only for a task-direction reference. |
| 73, 170 | `metasystem/internal/validate/conformance.go:724-819`, including `metasystem/internal/validate/conformance.go:754-817` | Holds. The cumulative helper reads concrete declarations across rounds and checks actual project paths. |
| 73, 174 | `metasystem/internal/dispatch/capcontinuation.go:22-28`, `metasystem/internal/dispatch/capcontinuation.go:26`, `metasystem/internal/dispatch/capcontinuation.go:89` | Holds. The paragraph requires the whole chain's concrete return declaration. |
| 77 | `metasystem/internal/validate/conformance.go:232-241` | Holds. `boundaryBase`, not `baseSha`, is the merge-base with current target HEAD. |
| 77 | `metasystem/internal/gittree/gittree.go:199-228`, `metasystem/internal/gittree/gittree.go:246-255` | Holds. `TreeOf` peels the project subtree and `Snapshot` uses an isolated index. |
| 79 | `metasystem/internal/gittree/gittree.go:246-254`, `metasystem/internal/validate/conformance.go:368-376` | Holds. Snapshot semantics include the stated candidate content; review currently makes two snapshots. |
| 94 | `metasystem/internal/gittree/gittree.go:357-376` | Holds. Diff and ChangedPaths disable rename detection and external diff behavior. |
| 96, 182 | `metasystem/internal/validate/conformance.go:617-637`, `metasystem/internal/validate/conformance.go:617-631`, `metasystem/internal/validate/conformance.go:632-637` | Holds. The prose waiver sums additions and deletions and separately rejects binary or uncountable rows. |
| 96, 182 | `metasystem/internal/landing/observe.go:1286-1290` | Holds. Tier 1 requires at most 3 files and 40 changed lines. |
| 127 | `metasystem/internal/validate/conformance.go:392`, `metasystem/internal/validate/conformance.go:421-425` | Holds. The cumulative check precedes both artifact writes. |
| 129 | `metasystem/internal/validate/conformance.go:377-392` | Holds. The project fence currently returns before the cumulative declaration check. |
| 131 | `metasystem/internal/validate/conformance.go:700-717` | Holds. Plans and the agent control plane remain independently protected. The new path-space instruction is still defective. |
| 135 | `metasystem/internal/validate/conformance.go:403-419`, `metasystem/internal/validate/conformance.go:403-407`, `metasystem/internal/validate/conformance.go:413-419`, `metasystem/internal/validate/conformance.go:361-365`, `metasystem/internal/validate/conformance.go:393-400` | Holds. The successful shape has three fields, identical bytes are reused, and existing evidence masks ordinary later failure. |
| 135 | `metasystem/internal/validate/conformance_review_shape_test.go:10-32` | Holds. The test enforces exactly the three successful fields. |
| 137 | `metasystem/internal/refusal/register.go:69`, `metasystem/internal/refusal/register_test.go:71-86` | Holds. Brief authority is a Question refusal; Agent rows need a remedy or known defect. |
| 147, 172 | `metasystem/cmd/metasystem/validate_verbs.go:276-352`, `metasystem/cmd/metasystem/validate_verbs.go:276-299`, `metasystem/cmd/metasystem/validate_verbs.go:280-352`, and `metasystem/cmd/metasystem/validate_verbs.go:343-352` | Holds. Review is an existing stage with exit codes 0, 1, and 2 and ordinary output relay. |
| 147 | `metasystem/internal/validate/conformance.go:227-230` | Holds. Invocation from the implementer workspace refuses. |
| 149 | `metasystem/internal/validate/conformance.go:139-193`, `metasystem/internal/validate/conformance.go:214-249` | Holds. Review resolves implementer facts and calls `reviewStage` without a critic record. |
| 149, 186 | `metasystem/scripts/agents/conformance-fixtures.sh:38-96`, `metasystem/scripts/agents/conformance-fixtures.sh:73-96`, `metasystem/scripts/agents/conformance-fixtures.sh:153-179`, `metasystem/scripts/agents/conformance-fixtures.sh:153-169`, and `metasystem/scripts/agents/conformance-fixtures.sh:175-179` | Holds. The bed has the named helpers and runs review before any critic is written. |
| 186 | `metasystem/testing.json:74` | Holds. `section/conformance-fixtures` is the existing registered group. |
| 151 | `metasystem/docs/orchestration.md:36-42`, `metasystem/skills/code-critique/SKILL.md:27-31` | Holds. Current orchestration goes directly from implementation to critic, while the skill tells the critic to run conformance. |
| 153 | `metasystem/internal/dispatch/read_subject_compute.go:24-26`, `metasystem/internal/dispatch/read_subject_compute.go:134-146` | Holds. Missing review evidence can produce no read subject rather than an initial dispatch error. |
| 162 | `metasystem/scripts/agents/dispatch.sh:858-864`, `metasystem/scripts/agents/dispatch.sh:1655`, `metasystem/scripts/agents/dispatch.sh:2643` | Holds. Initial and follow-up admission both reach the Go brief-mode verb. |
| 165 | `metasystem/internal/validate/conformance.go:827` | Holds. Mission stream reads the root brief for a different policy. |
| 166 | `metasystem/internal/adapter/fake.go:111`, `metasystem/internal/adapter/fake.go:137-167` | Holds. Fake return production emits an empty declaration; fake mode reading scans prompt or staged task direction. |
| 167 | `metasystem/scripts/agents/templates/brief.md:1`, `metasystem/scripts/agents/templates/brief.md:36`, `metasystem/scripts/agents/templates/follow-up.md:1-18` | Holds. These are the current mode and return-path instruction sites. |
| 168 | `metasystem/scripts/agents/fingerprint-harness.sh:187`, `metasystem/scripts/agents/adapters/fake.sh:449-453`, `metasystem/internal/adapter/selftestrun.go:135`, `metasystem/internal/steward/stage.go:102`, `metasystem/internal/steward/stage.go:155` | Holds. These emit or rewrite only Working Mode and remain lawful if new headers are optional. |
| 169 | `metasystem/scripts/validate-metasystem.sh:1618`, `metasystem/scripts/validate-metasystem.sh:1967-2010` | Holds. Current mandatory template headers and return schema checks do not mention the new headers. |
| 171 | `metasystem/internal/validate/conformance.go:355-428` | Holds. This is the full current review stage and the right integration owner. |
| 173 | `metasystem/internal/validate/recertification.go:809`, `metasystem/internal/validate/recertification.go:1182` | Holds. Recertification reuses only the cumulative concrete declaration helper. |
| 175 | `metasystem/internal/dispatch/followup_rebase.go:123-149` | Holds. Rebase overlap reads concrete return declarations. |
| 176 | `metasystem/internal/dispatch/review_reference.go:313-325` | Holds. Design pairing requires one exact returned design path. |
| 177 | `metasystem/internal/validate/returncomplete.go:239-302` | Holds. Return completion validates and may normalize concrete declaration spelling. |
| 178 | `metasystem/internal/adapter/adjudicate_test.go:205-260` | Holds. Acceptance repair handles a bare concrete return declaration. |
| 179 | `metasystem/scripts/agents/roles/implementer.md:3-13`, `metasystem/scripts/agents/schemas/implementer.schema.json:7`, `metasystem/scripts/agents/schemas/implementer.schema.json:38` | Holds. The role and schema require concrete `diffBoundary`; the schema need not change. |
| 180 | `metasystem/scripts/agents/role-packets.json:40-43`, `metasystem/scripts/agents/role-packets.json:54-59` | Holds. The recipes already select the code-critique skill and implementer orchestration guidance. |
| 181 | `metasystem/skills/code-critique/SKILL.md:29-31` | Holds. Current Layer 1 owns the computed diff and semantic conformance after dispatch. |
| 195 | `metasystem/scripts/validate-metasystem.sh:1129-1131` | Holds. The testing section launches the existing conformance bed through the engine. |
| 201 | `metasystem/internal/dispatch/build.go:1126`, `metasystem/docs/orchestration.md:172`, `metasystem/scripts/agents/go-gate.sh:97-99` | Holds. The seat owns shared proof and fast mode is an edit-loop check. |
| 318 | `metasystem/internal/validate/conformance_test.go:48-85` | Holds. `newConformanceFixture` supplies a real controller and worktree fixture. |
| 342 | `metasystem/internal/dispatch/capcontinuation_test.go:55-76` | Holds. The named test asserts the current cumulative declaration paragraph. |
| 354 | `metasystem/testing.json:95` | Holds. The dispatch and mission-runner fixture group already exists. |
| 367 | `metasystem/internal/validate/conformance.go:70-78`, `metasystem/internal/gittree/gittree.go:121-141` | Holds. Both Git execution paths use bounded execution and scrubbed environment where claimed. |

Proposed receipt for the integrating seat: `DESIGN-CRITIQUE brief-declares-the-round-boundary round 1: found eight material design defects in nested path matching, header pairing, mutation proof, source binding, authority extraction, and path grammar; evidence=read; runtime-proof=not-run`.

Verdict: fold these findings first.
