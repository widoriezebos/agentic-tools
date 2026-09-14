# Design critique, round 2

I reviewed revision 2 against the goal DONE line and the current checkout at `ef249e3a1eb3f26b82a1960fcd686097895c080e`. I found three material defects. Two reopen round-1 findings. One is new.

## Material findings

### BDRB-R2-01

Severity: high  
Material: yes

Claim: The design says, "Use the seat-authored task direction of the supplied implementer job. Current composition stores it as source `caller:brief` in slot `task-direction`" (`artifacts/reports/bdrb-design-r2.md:100`). Unit 3 then treats that source as the round's admitted brief and confines the implementation to two new validate files (`artifacts/reports/bdrb-design-r2.md:321-331`).

Evidence: The dispatcher admits the original brief at `metasystem/scripts/agents/dispatch.sh:1655`. It then replaces that file with an augmented copy for an implementer at `metasystem/scripts/agents/dispatch.sh:1736-1742`, can append serving-goal text at `metasystem/scripts/agents/dispatch.sh:1756-1764`, and appends the return-path form at `metasystem/scripts/agents/dispatch.sh:1780-1783`. The augmented file, not the admitted file, is passed to composition at `metasystem/scripts/agents/dispatch.sh:1817-1823` and copied to `brief.md` at `metasystem/scripts/agents/dispatch.sh:1918`. Follow-up admission has the same split. It checks `authority_message` at `metasystem/scripts/agents/dispatch.sh:2416` and `metasystem/scripts/agents/dispatch.sh:2643`, then augments `delivery_content` at `metasystem/scripts/agents/dispatch.sh:2697-2709` and composes that at `metasystem/scripts/agents/dispatch.sh:2771-2775`.

Composition reads exactly the file it is passed at `metasystem/internal/dispatch/composition.go:237-240` and labels those bytes `caller:brief` at `metasystem/internal/dispatch/composition.go:268`. The recorded source therefore proves the delivered, engine-augmented task-direction section. It does not prove the seat-authored bytes that admission checked.

The existing `section/dispatcher-adapter-and-mission-runner-fixtures` bed depends on this distinction. It requires the prompt to contain the exact payload brief at `metasystem/scripts/agents/dispatch-fixtures.sh:2182-2192`, requires serving-goal text inside that payload at `metasystem/scripts/agents/dispatch-fixtures.sh:2193-2225`, and requires a staged task direction to equal the composed job brief at `metasystem/scripts/agents/dispatch-fixtures.sh:2338-2343`.

An implementation as written will parse a different byte source from the one the seat authored and admitted. A generated section with a live `Boundary:` or `Ceiling:` line would silently change the round limit after admission. Fixing this needs an authenticated record of the admitted raw brief, or an explicit contract that binds the augmented delivered bytes. The former changes dispatcher persistence, source metadata, tests, and Unit 3's file boundary. The latter narrows the DONE contract. The current two-file Unit 3 cannot implement the stated source contract.

Rigor: severe. Local: true. Recoverable: true. Proof boundary crossed: true. Authority boundary crossed: true. Secrets boundary crossed: false. Irreversible data boundary crossed: false. External side-effect boundary crossed: false. Reopen while review still derives bounds from the augmented `caller:brief` body without a verified binding to the admitted seat-authored bytes.

### BDRB-R2-02

Severity: high  
Material: yes

Claim: The design says, "Use one path identity throughout this feature: the repository-relative string after JSON decoding" (`artifacts/reports/bdrb-design-r2.md:65`). It also says every non-directory declaration is a `path.Match` pattern and backslash is an escape (`artifacts/reports/bdrb-design-r2.md:70`). It then requires a structured authority citation to equal "a decoded concrete Boundary member" (`artifacts/reports/bdrb-design-r2.md:92`).

Evidence: These are not one identity. A JSON member that decodes to one backslash names a Git path with one backslash under the first rule. As a `path.Match` pattern, that backslash escapes the next byte and does not match that Git path. A pattern spelling that matches the literal backslash must contain another backslash after JSON decoding. That decoded member is no longer equal to the cited concrete path. Literal glob metacharacters have the same split between pattern spelling and concrete path identity.

The current authority tokenizer cannot hide this conversion. It excludes backslash and the other special bytes at `metasystem/internal/dispatch/brief.go:13-18`, then records the token spelling it sees at `metasystem/internal/dispatch/brief.go:141-154`. The current cumulative declaration check uses exact project-path keys at `metasystem/internal/validate/conformance.go:798-810`. Neither owner supplies a pattern-to-concrete identity conversion.

Nested projection has another instance of the same problem. The design validates the whole non-directory declaration as a pattern before projection, but current projection requires the actual installation prefix literally and strips it once at `metasystem/internal/validate/conformance.go:284-293`. A supported installation prefix containing pattern syntax cannot be both the literal prefix required by projection and an escaped pattern spelling required by `path.Match`.

`TestInstallationPathPreservesGitIdentity/backslash` only tests prefix removal (`artifacts/reports/bdrb-design-r2.md:441`). `TestBriefAuthoritySpecialCharacterInput/backslash` only tests citation lookup (`artifacts/reports/bdrb-design-r2.md:425`). Neither proves that one declaration spelling survives JSON decoding, projection, authority equality, and matching against the same backslash path.

This reopens BDRB-R1-07. An implementer must define separate pattern spelling and concrete path identity, define a lossless conversion where one exists, or disallow ambiguous pattern bytes in exact-member authority. Those choices build different parsers and matchers.

Rigor: severe. Local: true. Recoverable: true. Proof boundary crossed: true. Authority boundary crossed: true. Secrets boundary crossed: false. Irreversible data boundary crossed: false. External side-effect boundary crossed: false. Reopen while decoded pattern spelling is also treated as decoded concrete path identity without a defined round-trip and an end-to-end witness.

### BDRB-R2-03

Severity: medium  
Material: yes

Claim: The design calls its table complete and says, "Each slash case below is a separately named subtest and a separate remove-one run when it encodes a distinct rule" (`artifacts/reports/bdrb-design-r2.md:409-411`). It also says every rule has a test that fails when that rule alone is removed in each unit brief, including Unit 3 at `artifacts/reports/bdrb-design-r2.md:328` and Unit 4a at `artifacts/reports/bdrb-design-r2.md:340`.

Evidence: The table still bundles independent checks behind names that do not identify each check.

- S4 requires a nonempty repository-relative Path, no parent component, an absolute OpenPath, and a matching Path suffix in the design text at `artifacts/reports/bdrb-design-r2.md:107`. Its witness row provides only `/path` for all of them at `artifacts/reports/bdrb-design-r2.md:436`. Current admission has separate checks for these facts at `metasystem/internal/dispatch/build.go:1050-1077`. Removing one fact does not have a separately named witness.
- The path dialect assigns distinct semantics to `?`, classes, and backslash escapes at `artifacts/reports/bdrb-design-r2.md:70`. P4 names only `/glob` and `/escaped-metacharacter`, with no named `?` or class case, at `artifacts/reports/bdrb-design-r2.md:442`. Current exact return matching supplies no fallback coverage for any pattern form at `metasystem/internal/validate/conformance.go:798-810`.
- A3 says only complete backtick spans and complete JSON strings are consumed at `artifacts/reports/bdrb-design-r2.md:90`, but its row has no incomplete-backtick or incomplete-JSON case at `artifacts/reports/bdrb-design-r2.md:425`. The current scanner processes line fragments independently at `metasystem/internal/dispatch/brief.go:128-160`, so an incomplete structured span can fall through to a materially different authority result.
- S1 has no case that distinguishes the admitted raw brief from the augmented composed body at `artifacts/reports/bdrb-design-r2.md:433`. The dispatcher distinction is established at `metasystem/scripts/agents/dispatch.sh:1655`, `metasystem/scripts/agents/dispatch.sh:1736-1742`, and `metasystem/scripts/agents/dispatch.sh:1817-1823`.

This reopens BDRB-R1-04. The missing witnesses let an implementation omit a Path/OpenPath guard, simplify the matcher, mishandle an incomplete span, or bind the wrong brief while still satisfying every named test in the table. The owning unit tests and, for the raw-source case, the unit boundary must change.

Rigor: severe. Local: true. Recoverable: true. Proof boundary crossed: true. Authority boundary crossed: true. Secrets boundary crossed: false. Irreversible data boundary crossed: false. External side-effect boundary crossed: false. Reopen until every independently removable rule has a distinct named case and the source case crosses the actual dispatch-to-review seam.

## Non-material findings

### BDRB-R2-N01

Severity: low  
Material: no

Claim: The design presents a reader and writer inventory at `artifacts/reports/bdrb-design-r2.md:216-241`.

Evidence: No production reader of `diffBoundary` is missing. The inventory covers review and recertification at `metasystem/internal/validate/conformance.go:724-819`, `metasystem/internal/validate/recertification.go:809`, and `metasystem/internal/validate/recertification.go:1182`; follow-up overlap at `metasystem/internal/dispatch/followup_rebase.go:123-149`; review pairing at `metasystem/internal/dispatch/review_reference.go:313-325`; return normalization at `metasystem/internal/validate/returncomplete.go:239-302`; role and schema readers at `metasystem/scripts/agents/roles/implementer.md:3-13`, `metasystem/scripts/agents/schemas/implementer.schema.json:7`, and `metasystem/scripts/agents/schemas/implementer.schema.json:38`; and fake output at `metasystem/internal/adapter/fake.go:109-112`.

The Working Mode emitter inventory is not exhaustive. Test briefs also appear at `metasystem/scripts/agents/supervision-fixtures.sh:1752`, `metasystem/scripts/agents/supervision-fixtures.sh:2048`, `metasystem/scripts/agents/supervision-fixtures.sh:3024`, `metasystem/scripts/agents/dispatch-fixtures.sh:493`, `metasystem/scripts/agents/dispatch-fixtures.sh:1010`, `metasystem/scripts/agents/dispatch-fixtures.sh:1463`, and `metasystem/scripts/agents/checkout-execution-guard-fixtures.sh:102`. `metasystem/internal/dispatch/mirror.go:117-118` also copies the root brief without interpreting its headers. All remain valid because paired bounds are optional and headerless briefs retain current behavior. No additional implementation follows from these omissions.

### BDRB-R2-N02

Severity: low  
Material: no

Claim: The design says there are nine separately landing units and that each is capped at 400 changed lines (`artifacts/reports/bdrb-design-r2.md:4`).

Evidence: The stated allocations are 335, 350, 330, 370, 365, 370, 390, 320, and 345 changed lines for Units 1a, 1b, 2, 3, 4a, 4b, 4c, 6a, and 6b. Each arithmetic total is at most 400. The plan contains seven fixture executions, not only the four DONE legs. The extra three are a headerless guard and two partial-pair refusals. The estimates are not runtime proof, but `artifacts/reports/bdrb-design-r2.md:499` tells a builder to split before exceeding the cap. This does not change the build by itself. BDRB-R2-01 separately makes Unit 3's present file boundary invalid.

## Round-1 fold audit

| Round-1 finding | Result | Checked fold |
| --- | --- | --- |
| BDRB-R1-01 | Closed for the current ordinary nested installation | Revision 2 guards siblings, projects declarations once, and matches project paths at `artifacts/reports/bdrb-design-r2.md:74-80`. P1-P3 and E3 cover the normal root and nested cases. The pattern-spelling problem in BDRB-R2-02 is a distinct remaining edge in that projection. |
| BDRB-R1-02 | Closed | Both-or-neither is normative at `artifacts/reports/bdrb-design-r2.md:29-40`. H2 and H8 name both missing-header refusals. The fixture plan contains both partial-pair legs. |
| BDRB-R1-03 | Closed | The old test-only Unit 5 is gone. Snapshot integration, enforcement, and public-bed tests now sit with production-owning Units 4b, 4c, and 6b at `artifacts/reports/bdrb-design-r2.md:345-407`. |
| BDRB-R1-04 | Reopened | The table adds many missing witnesses, but S4, P4, A3, and the raw-source seam still lack one independently named case per removable rule. See BDRB-R2-03. |
| BDRB-R1-05 | Closed | S4 and S5 now require source/reference slot, digest, bytes, Path/OpenPath, and verified file facts at `artifacts/reports/bdrb-design-r2.md:106-110` and `artifacts/reports/bdrb-design-r2.md:436-437`. The remaining problem is witness granularity, not a restatement of the old slot-only join. |
| BDRB-R1-06 | Closed | The exact leading-space or tab exemption is defined at `artifacts/reports/bdrb-design-r2.md:88`; A2 separately covers spaces, tabs, headerless text, and a citation on another line. |
| BDRB-R1-07 | Reopened | Structured citation decoding is specified, but the design still makes one decoded string serve as both pattern spelling and concrete path identity. See BDRB-R2-02. |
| BDRB-R1-08 | Closed | Trailing-slash declarations now take a literal directory branch and bypass `path.Match` at `artifacts/reports/bdrb-design-r2.md:67-70`. H6 names both the admitted literal directory and refused malformed non-directory form. |

Six of eight material round-1 findings are genuinely closed. Two are material again.

## Seam and existing-test audit

| Seam | Result | Existing test or bed at risk |
| --- | --- | --- |
| reviewStage ordering | Holds if implemented exactly as specified. Current review captures two snapshots, checks the project fence, runs the cumulative check, then writes evidence at `metasystem/internal/validate/conformance.go:355-428`. The design's headerless ordering preserves the outside-project-first and immutable-evidence behavior. | `TestReviewRefusesSiblingChanges`, `TestConformanceReviewAndCritiqueMerge`, `TestConformanceReviewRefusesToOverwriteRoundEvidence`, `TestConformanceReviewIdenticalRerunIsIdempotent`, `TestReviewStageWritesOnlyThreeFields`, and `section/conformance-fixtures`. |
| projectDeclaration reuse | Holds for ordinary root and nested paths. Current code requires the literal prefix and strips it once at `metasystem/internal/validate/conformance.go:276-293`. The sibling guard must run before installationPath. | `TestNestedDeclarationDialectIsMandatory`, `TestNestedReviewStageSpeaksProjectSpace`, and `TestReviewRefusesSiblingChanges`. Pattern syntax in the installation prefix remains unresolved by BDRB-R2-02. |
| installationPath reuse | Holds only after the promised literal rewrite. Its sole current caller classifies waivers at `metasystem/internal/validate/conformance.go:97-107`. | `TestNestedWaiverProtectsProjectPlans` must remain green. |
| one snapshot | Holds. `TreeOf` can derive a subtree from a supplied tree at `metasystem/internal/gittree/gittree.go:199-228`. `Snapshot` captures the candidate through an isolated index at `metasystem/internal/gittree/gittree.go:236-276`. | `TestReviewBriefSingleSnapshot`, `TestReviewBriefSnapshotMembership`, `TestNestedReviewStageSpeaksProjectSpace`, and `TestReviewStageWritesOnlyThreeFields`. |
| existing cumulative diffBoundary check | Holds. The current exact declaration check is at `metasystem/internal/validate/conformance.go:724-819`; recertification calls the same policy at `metasystem/internal/validate/recertification.go:809` and `metasystem/internal/validate/recertification.go:1182`. | `TestReviewBriefDeclarationIsNotPermission`, existing conformance tests, follow-up overlap tests, and recertification tests. |
| Boundary lines are output-only for authority | Holds for ordinary members and indented examples if the new lexer is correct. Current input use wins at `metasystem/internal/dispatch/brief.go:155-167`. | Existing authority tests plus `TestBriefAuthorityBoundaryIsOutput`, `TestBriefAuthorityIndentedBoundsExampleIsInert`, and `TestBriefAuthorityBoundaryInputStillRequired`. BDRB-R2-02 blocks the claimed special-character identity. |
| current-round composition source range | Range and delivered digest facts hold at `metasystem/internal/dispatch/composition.go:366-390`. The selected body's provenance does not. It is the augmented body passed at `metasystem/scripts/agents/dispatch.sh:1817-1823` or `metasystem/scripts/agents/dispatch.sh:2771-2775`. | `section/dispatcher-adapter-and-mission-runner-fixtures` would break if an implementation silently changed existing composed payload semantics. See BDRB-R2-01. |

## Fixture and unit audit

The four DONE fixture legs are all named:

1. `brief-boundary-unlisted` refuses and writes neither success artifact.
2. `brief-ceiling-exceeded` refuses and writes neither success artifact.
3. `brief-both-admitted` succeeds.
4. `brief-no-headers` succeeds unchanged.

The design adds `brief-no-headers-undeclared`, `brief-missing-ceiling`, and `brief-missing-boundary`. That makes seven executions. `metasystem/testing.json:75` is the existing conformance group. `metasystem/scripts/agents/conformance-fixtures.sh:38-96` and `metasystem/scripts/agents/conformance-fixtures.sh:153-169` supply the named fixture helpers.

The design has six base unit numbers, but split Units 1, 4, and 6 produce nine landing units. Their stated totals are all within 400. Unit 4c has the least headroom at 390. The stop-and-split instruction at `artifacts/reports/bdrb-design-r2.md:499` is necessary. The current Unit 3 boundary is not sufficient for the raw-authority source contract, as BDRB-R2-01 explains.

The rule-to-witness table is not complete. The independently removable rules without a distinct named witness are:

- each separate Path/OpenPath fact in S4;
- `?` matching and character-class matching in P4;
- incomplete backtick and incomplete JSON span handling in A3;
- admitted raw brief versus augmented composed body in S1.

All other table rows name a witness that can observe the stated rule on paper. Runtime and mutation proof remain pending.

## DONE contract audit

| DONE clause | Result |
| --- | --- |
| Brief carries `Boundary:` and `Ceiling:` | Satisfied. The design defines a paired, optional header contract and deterministic partial-pair refusal. |
| Extraction lives in `internal/dispatch/brief.go` beside Working Mode | Satisfied by Units 1a and 1b. |
| Review refuses every outside path and an excessive text-line count | Satisfied on paper by P1-P5, V1-V3, and E1-E7, subject to the pattern-identity defect in BDRB-R2-02. |
| Typed violation names every offending path and the count | Satisfied. Boundary carries all outside repository paths and the total count. Ceiling carries all changed repository paths and the total count. Diagnostics are untruncated JSON. |
| Seat checks before dispatching a read | Satisfied as a seat procedure. W1 puts the command and 0/1/2 handling in both instruction owners. The design expressly does not add an automatic critic-launch gate. DONE says the seat checks, so this is not a contract narrowing. |
| Headerless briefs are admitted unchanged | Satisfied. The bounded branch is absent and no numstat call occurs. Existing fence, cumulative declaration, and immutable-evidence ordering stay in force. |
| Four required fixtures | Satisfied. All four are named with status and artifact assertions. Three additional guard executions widen proof only. |

The pair-or-neither rule resolves the goal's unspecified partial-pair input. It does not narrow a stated DONE case. The design widens proof with strict syntax, special-character paths, three guard executions, and explicit seat instructions. BDRB-R2-01 nevertheless uses an engine-augmented source where DONE requires the authored brief. BDRB-R2-02 leaves some allowed path bytes without a coherent identity.

## Goal overlap audit

`member-size-gate` owns a concrete member file-count check before build at `metasystem/plans/goals/member-size-gate.md:8`. This design owns outer path permission and cumulative text-line count at review. A Boundary directory or glob is not a concrete member inventory. There is no mechanism collision. There is a deliberate interval between dispatch and review where this design does not count files. The later goal owns that interval. Unit 6a currently lists eight files, so its future member-size treatment may require a split without changing this design's semantics.

`cross-cutting-change-inventories-its-readers` adds `Readers:` and `Layers on:` declarations at `metasystem/plans/goals/cross-cutting-change-inventories-its-readers.md:8-10`. This design does not add those headers and does not claim Boundary can replace them. Its manual inventory is a design-time seam audit. There is no collision. BDRB-R2-01 is the gap that a complete future reader and writer inventory should expose: dispatch augments the task direction before the composition reader records it.

## Current-code citation audit

Every current-code citation in revision 2 was opened. Proposed test names are not current-code claims.

| Design line | Current-code claim | Result |
| --- | --- | --- |
| 13 | DONE is at the goal line | Holds at `metasystem/plans/goals/brief-declares-the-round-boundary.md:8`. The cited fact-pass commit is older than this checkout, but the cited owners remain at the stated locations. |
| 15, 124 | Review base is the merge-base | Holds at `metasystem/internal/validate/conformance.go:232-241`. |
| 27 | Working Mode uses column-zero recognition and trims the value | Holds at `metasystem/internal/dispatch/brief.go:47-53`. |
| 59 | Combined CLI admission checks authority before mode; mode-only keeps its refusal | Holds at `metasystem/cmd/metasystem/dispatch_verbs.go:2014-2027` and `metasystem/internal/dispatch/brief.go:41-55`. The initial shell route computes mode earlier at `metasystem/scripts/agents/dispatch.sh:1542`, but the sentence is limited to the CLI route. |
| 61 | Initial shell suffix names only Working Mode | Holds at `metasystem/scripts/agents/dispatch.sh:1542`. |
| 76, 471 | projectDeclaration requires the literal install prefix and strips once | Holds at `metasystem/internal/validate/conformance.go:284-293`. |
| 82, 226 | installationPath currently normalizes backslashes and slashes; its caller is waiver classification | Holds at `metasystem/internal/validate/conformance.go:88-107`. |
| 84, 478 | Cumulative return matching is exact map membership | Holds at `metasystem/internal/validate/conformance.go:798-810`. |
| 86 | Existing input use wins over output use | Holds at `metasystem/internal/dispatch/brief.go:155-167`. |
| 88, 476 | Authority extraction scans lines without fence state | Holds at `metasystem/internal/dispatch/brief.go:128-160`. |
| 92, 477 | Old tokens and eligibility exclude special path bytes | Holds at `metasystem/internal/dispatch/brief.go:13-24` and `metasystem/internal/dispatch/brief.go:141-154`. The claimed replacement identity is internally inconsistent. See BDRB-R2-02. |
| 94 | Artifact and committed-tree lookups are existing owners | Holds at `metasystem/internal/dispatch/brief.go:83-104`. |
| 100, 223 | Composition records task-direction as `caller:brief`, its delivered range, and digest | Partly holds at `metasystem/internal/dispatch/composition.go:268` and `metasystem/internal/dispatch/composition.go:358-390`. It records the bytes passed by dispatch, not necessarily the seat-authored bytes. See BDRB-R2-01. |
| 100, 114 | Initial and follow-up prompt/composition artifacts are persisted | Holds at `metasystem/scripts/agents/dispatch.sh:1924-1929` and `metasystem/scripts/agents/dispatch.sh:2883-2888`. The root `brief.md` copy at `metasystem/scripts/agents/dispatch.sh:1918` is the augmented file. |
| 107, 224, 475 | Build admission checks source/reference binding facts | Holds at `metasystem/internal/dispatch/build.go:1042-1077` and `metasystem/internal/dispatch/build.go:1099-1103`. The separate facts are why S4 needs separate witnesses. |
| 108, 475 | ReadVerifiedReference checks containment, file type, bytes, and digest, but not source join | Holds at `metasystem/internal/dispatch/references.go:24-49`. |
| 109 | Composition's task-direction envelope and byte range are as described | Holds at `metasystem/internal/dispatch/composition.go:366-390` and `metasystem/internal/dispatch/composition.go:555-562`. |
| 112 | Existing SHA-256 helper is reusable | Holds at `metasystem/internal/validate/recertification.go:115-118`. |
| 114 | Root brief persistence and successor legacy prompt fallback exist | Holds at `metasystem/scripts/agents/dispatch.sh:1918` and `metasystem/internal/validate/conformance.go:925-934`. The persisted root brief is augmented, which undermines the authored-source claim. |
| 116, 225 | Exhaustion's inline branch returns the whole prompt and successor/mission readers are separate | Holds at `metasystem/internal/validate/conformance.go:827-839` and `metasystem/internal/validate/conformance.go:914-960`. |
| 120, 230 | Return declarations are cumulative and cap continuation requires predecessor paths | Holds at `metasystem/internal/validate/conformance.go:754-817`, `metasystem/internal/dispatch/capcontinuation.go:22-28`, and `metasystem/internal/dispatch/capcontinuation.go:89`. |
| 128 | TreeOf peels a subtree and Snapshot uses an isolated index with add -A | Holds at `metasystem/internal/gittree/gittree.go:199-228` and `metasystem/internal/gittree/gittree.go:236-276`. |
| 130 | Review currently calls Snapshot twice | Holds at `metasystem/internal/validate/conformance.go:368-376`. |
| 145, 499 | Git runner pins configuration, scrubs steering variables, and has bounded execution | Holds at `metasystem/internal/gittree/gittree.go:90-99` and `metasystem/internal/gittree/gittree.go:121-141`. |
| 149 | Existing path diff uses no renames | Holds at `metasystem/internal/gittree/gittree.go:362-376`. |
| 151, 239 | Prose waiver and Tier 1 limits are separate policies | Holds at `metasystem/internal/validate/conformance.go:617-637` and `metasystem/internal/landing/observe.go:1286-1290`. |
| 182, 226, 227 | Project fence, protected/cumulative policy, and review locations are current | Holds at `metasystem/internal/validate/conformance.go:88-107`, `metasystem/internal/validate/conformance.go:276-428`, `metasystem/internal/validate/conformance.go:700-725`, and `metasystem/internal/validate/conformance.go:798-817`. |
| 184 | Outside-project refusal precedes the cumulative check | Holds at `metasystem/internal/validate/conformance.go:377-400`. |
| 190 | Success shape, idempotent reuse, and masking closure are current | Holds at `metasystem/internal/validate/conformance.go:361-365`, `metasystem/internal/validate/conformance.go:403-419`, and `metasystem/internal/validate/conformance_review_shape_test.go:10-32`. |
| 192, 457 | Existing refusal precedent and Agent-row invariant are current | Holds at `metasystem/internal/refusal/register.go:80`, `metasystem/internal/refusal/register_test.go:20-40`, and `metasystem/internal/refusal/register_test.go:71-86`. |
| 202, 238 | Validate CLI relays 0/1/2 and refuses the implementer checkout | Holds at `metasystem/cmd/metasystem/validate_verbs.go:276-352` and `metasystem/internal/validate/conformance.go:227-230`. |
| 204, 245, 355 | Review can run without a critic and the fixture helpers use isolated worktrees | Holds at `metasystem/internal/validate/conformance.go:139-193`, `metasystem/internal/validate/conformance.go:214-249`, `metasystem/scripts/agents/conformance-fixtures.sh:38-96`, `metasystem/scripts/agents/conformance-fixtures.sh:153-179`, and `metasystem/internal/validate/conformance_test.go:48-85`. |
| 210 | Current orchestration and critic instructions place conformance at the named seam | Holds at `metasystem/docs/orchestration.md:36-42` and `metasystem/skills/code-critique/SKILL.md:25-31`. |
| 214 | Missing artifacts can yield an absent read subject | Holds at `metasystem/internal/dispatch/read_subject_compute.go:24-26` and `metasystem/internal/dispatch/read_subject_compute.go:134-146`. |
| 220-222 | Brief readers, CLI relay, and shell preflights are correctly located | Holds at `metasystem/internal/dispatch/brief.go:41-104`, `metasystem/internal/dispatch/brief.go:121-171`, `metasystem/cmd/metasystem/dispatch_verbs.go:1993-2027`, and `metasystem/scripts/agents/dispatch.sh:858-864`, `metasystem/scripts/agents/dispatch.sh:1542`, `metasystem/scripts/agents/dispatch.sh:1655`, `metasystem/scripts/agents/dispatch.sh:2643`. |
| 227-229 | All named concrete diffBoundary readers are current | Holds at `metasystem/internal/validate/conformance.go:724-819`, `metasystem/internal/validate/recertification.go:809`, `metasystem/internal/validate/recertification.go:1182`, `metasystem/internal/dispatch/followup_rebase.go:123-149`, `metasystem/internal/dispatch/review_reference.go:313-325`, and `metasystem/internal/validate/returncomplete.go:239-302`. |
| 231-233 | Role, schema, templates, and packet selection are current | Holds at `metasystem/scripts/agents/roles/implementer.md:3-13`, `metasystem/scripts/agents/schemas/implementer.schema.json:7`, `metasystem/scripts/agents/schemas/implementer.schema.json:38`, `metasystem/scripts/agents/templates/brief.md:1-36`, `metasystem/scripts/agents/templates/follow-up.md:1-18`, and `metasystem/scripts/agents/role-packets.json:40-43`, `metasystem/scripts/agents/role-packets.json:54-59`. |
| 234-236, 479 | Fake adapter, fake reference hooks, and named mode emitters are current | Holds at `metasystem/internal/adapter/fake.go:109-112`, `metasystem/internal/adapter/fake.go:137-167`, `metasystem/scripts/agents/adapters/fake.sh:62-67`, `metasystem/scripts/agents/adapters/fake.sh:231-236`, `metasystem/scripts/agents/adapters/fake.sh:449-453`, `metasystem/scripts/agents/fingerprint-harness.sh:187`, `metasystem/internal/adapter/selftestrun.go:135`, `metasystem/internal/steward/stage.go:102`, and `metasystem/internal/steward/stage.go:155`. The emitter list is not exhaustive, but the omissions are non-material. |
| 237 | Static header and return audits are at the named locations | Holds at `metasystem/scripts/validate-metasystem.sh:1614-1630` and `metasystem/scripts/validate-metasystem.sh:1967-2010`. |
| 241 | Later goals own member size and cross-cutting reader declarations | Holds at `metasystem/plans/goals/member-size-gate.md:8` and `metasystem/plans/goals/cross-cutting-change-inventories-its-readers.md:8-10`. |
| 245, 407 | Existing testing groups are registered at the named lines | Holds at `metasystem/testing.json:75` for conformance and `metasystem/testing.json:96` for dispatcher, adapter, and mission runner. |
| 261 | The existing conformance section runs through the engine | Holds at `metasystem/scripts/validate-metasystem.sh:1129-1131`. |
| 269 | Builder/seat proof and cache rules are already stated | Holds at `metasystem/internal/dispatch/build.go:1126` and `metasystem/docs/orchestration.md:172`. |
| 283 | The fast gate is an edit-loop check | Holds at `metasystem/scripts/agents/go-gate.sh:97-103`. |
| 331 | Reduced exhaustion fixture composition writes references only | Holds at `metasystem/internal/validate/conformance_test.go:118-123`. |
| 391 | Cap continuation paragraph test exists | Holds at `metasystem/internal/dispatch/capcontinuation_test.go:55-76`. |
| 441, 448 | Nested waiver and nested review guards exist | Holds at `metasystem/internal/validate/nested_conformance_test.go:229` and `metasystem/internal/validate/nested_conformance_test.go:44-101`. |
| 471 | Existing changed paths are repository-relative before project projection | Holds at `metasystem/internal/gittree/gittree.go:372-386`. |
| 480 | Existing fixture suite has an isolated guard case | Holds at `metasystem/scripts/agents/conformance-fixtures.sh:38-64`. |

No implementation test, fixture bed, or mutation run was executed. This was a read-only design critique.

Proposed receipt for the integrating seat: `DESIGN_CRITIQUE brief-declares-the-round-boundary round 2: three material findings; raw brief provenance, pattern/concrete path identity, and incomplete remove-one witnesses; evidence=read; runtime-proof=not-applicable`.

fold these findings first
