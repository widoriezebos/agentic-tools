# Design critique: builder proves each rule by mutation, round 2

## Material findings

### BPRM-R2-001

Severity: critical

Material: yes

Claim: The specified location regex rejects both location forms that the design says are legal. An implementer who copies it will make every normal mutation entry invalid.

Evidence: The design specifies `^[^\r\n]+(:[1-9][0-9]*\|#[^\r\n\s][^\r\n]*)$` and then says a location is either `path:<positive line>` or `path#symbol` (`artifacts/reports/bprm-design-r2.md:46-57`). In Go regular expressions, `\|` is a literal pipe. Alternation is the unescaped `|`. The current validator compiles schema patterns with `regexp.Compile` and applies them with `regexp.MustCompile(...).MatchString` (`metasystem/internal/validate/returncomplete.go:474-479`, `metasystem/internal/validate/returncomplete.go:525-527`). The written pattern requires a suffix shaped like `:12|#symbol`. It rejects the design's own `...go#mutationViolations` example at line 41 and every plain `path:12` location. The benchmark reader also applies the pinned pattern with `re.fullmatch` (`benchmark/extractor.py:80-84`), so the bad expression would fail in both validators.

### BPRM-R2-002

Severity: critical

Material: yes

Claim: The required sibling revision provides neither the Rules parser nor the explicit-round shared reader this design consumes. It also specifies a different legacy follow-up fallback. The first BPRM unit cannot lawfully start after the named prerequisites land.

Evidence: The design says the sibling parser and reader are prerequisites, says sibling unit 1 "must expose Rules", and says sibling unit 3 owns a shared Boundary, Ceiling, and Rules reader that accepts an explicit round (`artifacts/reports/bprm-design-r2.md:5`, `artifacts/reports/bprm-design-r2.md:82-91`, `artifacts/reports/bprm-design-r2.md:238-240`). The actual sibling revision defines `BriefBounds` with only Boundary and Ceiling and only `ParseBriefBounds` and `ReadBriefBounds` (`metasystem/plans/brief-declares-the-round-boundary-design.md:42-53`). Its unit 1a owns only that paired bounds parser (`metasystem/plans/brief-declares-the-round-boundary-design.md:285-295`). Its unit 3 specifies `reviewBriefBounds()` with no round argument and a `dispatch.BriefBounds` result (`metasystem/plans/brief-declares-the-round-boundary-design.md:98-114`, `metasystem/plans/brief-declares-the-round-boundary-design.md:321-331`). The sibling contains no Rules contract. It also selects a legacy follow-up's own `prompt.md`, while BPRM requires the chain-root `brief.md` (`metasystem/plans/brief-declares-the-round-boundary-design.md:114`; `artifacts/reports/bprm-design-r2.md:89`). Current code has only Working Mode and authority parsing (`metasystem/internal/dispatch/brief.go:41-104`, `metasystem/internal/dispatch/brief.go:121-171`). Current composition makes `caller:brief` one source and appends prior brief and return as later sources (`metasystem/internal/dispatch/composition.go:268-319`; `metasystem/scripts/agents/dispatch.sh:2719-2725`). That is exactly why the missing shared source contract matters. BPRM's own unit boundaries exclude both `brief.go` and `brief_bounds_source.go`, so its implementer cannot repair the prerequisite without building something different from both designs.

### BPRM-R2-003

Severity: high

Material: yes

Claim: The design narrows the binding DONE contract for more than U1. U2, U3a, U3b, and U4a all carry Rules before mutation review is activated. Their missing rows are checked by the seat, not refused by conformance. U2 cannot retroactively make U1's return satisfy DONE.

Evidence: The design calls U1 "one bootstrap exception", says "No further exception is authorized", and says U2 closes it (`artifacts/reports/bprm-design-r2.md:6`, `artifacts/reports/bprm-design-r2.md:269`, `artifacts/reports/bprm-design-r2.md:319`). It later concedes that the seat checks rows while review enforcement is being built (`artifacts/reports/bprm-design-r2.md:671`). U4a only builds the join and U4b activates it (`artifacts/reports/bprm-design-r2.md:451`, `artifacts/reports/bprm-design-r2.md:475-493`). The goal's DONE line requires each implementer return to carry an entry per acceptance row and review conformance to refuse missing entries (`metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:8`). Today launch freezes the old closed schema before U1 runs (`metasystem/scripts/agents/adapters/runtime-common.sh:103-110`; `metasystem/scripts/agents/schemas/implementer.schema.json:5-8`). That explains the bootstrap problem but does not amend DONE. The canonical checker that U1 adds is deliberately allowed to accept omitted mutations, so it cannot enforce Rules coverage. Current review has only cumulative diffBoundary enforcement before its success writes (`metasystem/internal/validate/conformance.go:351-425`). U2 through U4a therefore have top-level representation without the required review refusal. U2 also precedes U3b's mutation-specific repair wording. The current empty-delivery repair asks for a schema-shaped return but does not say not to invent mutation observations, and after-repair handling validates only the resulting return (`metasystem/internal/adapter/adjudicate.go:84-97`, `metasystem/internal/adapter/adjudicate.go:229-246`). A later replay does not change U1's stored return, and a manual read is not the contracted conformance refusal.

### BPRM-R2-004

Severity: high

Material: yes

Claim: The chain-wide join has contradictory time semantics. It cannot both accept an entry from any round against the final union and keep a historically unknown entry rejected after that id is declared later. It also cannot enforce the stated requirement that a repair re-declare its id in the repair round.

Evidence: The design defines `D` as the union of every round's Rules and says an entry may come from any round (`artifacts/reports/bprm-design-r2.md:103-108`). The same paragraph says historical unknown entries remain rejected and a later unique declared entry repairs coverage. If round 1 returns R9 before round 2 declares R9, R9 belongs to final `D`. The first rule accepts it. The historical-unknown rule rejects it. The design does not say whether declaration membership is tested against `S(r)`, declarations at or before r, or final `D`. It separately requires a correction to re-declare an id (`artifacts/reports/bprm-design-r2.md:93`, `artifacts/reports/bprm-design-r2.md:163`), but the specified join tests entries only against final `D`. A repair return can therefore add R1 without putting R1 in that round's Rules, and an old accepted R1 can cover changed repair code with no fresh entry. The current cumulative model flattens all round declarations into one map before checking the candidate (`metasystem/internal/validate/conformance.go:754-805`). Reusing that convention does not preserve the missing round relationship. U4a's unknown, cross-round, and rejected-repair witnesses can be implemented with incompatible answers while each appears faithful to the prose.

### BPRM-R2-005

Severity: high

Material: yes

Claim: The rule tables omit mandatory benchmark integration rules and give them no builder mutation witness. U1 and U2 can satisfy every listed row while breaking the benchmark's implementer evidence contract.

Evidence: The design requires the implementer drift command to use the canonical materializer, requires the pin to be byte-identical to that output, requires historical omission to remain accepted, and says those changes count toward the unit ceiling (`artifacts/reports/bprm-design-r2.md:67-69`). It also says the unit tables are complete (`artifacts/reports/bprm-design-r2.md:250`, `artifacts/reports/bprm-design-r2.md:667-669`). Neither U1's 29 ids nor U2's 38 ids includes a benchmark drift, byte-identity, or historical-omission rule (`artifacts/reports/bprm-design-r2.md:254-304`, `artifacts/reports/bprm-design-r2.md:306-365`). The kit currently runs default v2 materialization and byte-compares the result (`benchmark/validate-kit.sh:80-102`). The extractor validates stored returns through that pin (`benchmark/extractor.py:514-538`). Its historical implementer fixture omits mutations (`benchmark/extractor-fixtures.sh:260-274`). The configured extra suite cited by the design checks only mission-state evidence, not the implementer pin, drift command, or extractor fixture (`metasystem/metasystem.conf:102-105`; `benchmark/evidence-drift-fixtures.sh:35-69`). U1 asks the builder only to syntax-check `validate-kit.sh`, which cannot detect a wrong command or pin (`artifacts/reports/bprm-design-r2.md:303-304`). The missing rules are: canonical materialization in the implementer drift branch, canonical pin byte identity, and acceptance of the historical omitted member. Other-role drift preservation also has no named witness. U1 is already budgeted at 380 lines. Adding the missing rows and behavioral tests invalidates its stated headroom and may require the split that the design's stop rule permits.

## Non-material findings

### BPRM-R2-006

Severity: low

Material: no

Claim: Three current-state citations are stale or inaccurate. They do not independently change the requested implementation beyond BPRM-R2-002.

Evidence: The design says the sibling still says Revision 1 (`artifacts/reports/bprm-design-r2.md:13`), but the cited line says Revision 2 (`metasystem/plans/brief-declares-the-round-boundary-design.md:3`). It cites sibling lines 67-79 and 270-282 as the planned source reader, but those lines specify path matching and builder proof commands; the source reader is at lines 98-116 (`artifacts/reports/bprm-design-r2.md:91`; `metasystem/plans/brief-declares-the-round-boundary-design.md:67-79`; `metasystem/plans/brief-declares-the-round-boundary-design.md:270-282`). It cites sibling line 328 as a CLI-without-critic precedent, but that line is the unit 3 proof sentence (`artifacts/reports/bprm-design-r2.md:190`; `metasystem/plans/brief-declares-the-round-boundary-design.md:328`). The reader inventory also says fake acceptance inherits materialization (`artifacts/reports/bprm-design-r2.md:220`), while the fake adapter explicitly does not source `runtime-common.sh` and its Go producer constructs the version-2 object directly (`metasystem/scripts/agents/adapters/fake.sh:140-160`; `metasystem/internal/adapter/fake.go:53-112`). Fake does inherit completeness, so the intended compatibility disposition still holds.

## Round 1 closure check

| Round 1 finding | Round 2 result |
| --- | --- |
| BPRM-R1-001 | Open. JSON unit headers are fixed, but the promised sibling Rules parser does not exist in the sibling revision. See BPRM-R2-002. |
| BPRM-R1-002 | Partly folded, still open. The whole-chain union and final-tree lookup are present. The temporal acceptance and repair rules contradict that union. See BPRM-R2-004. |
| BPRM-R1-003 | Open. The design rejects the old prompt scan, but assigns its replacement to a sibling reader that returns only bounds and has the opposite legacy follow-up fallback. See BPRM-R2-002. |
| BPRM-R1-004 | Closed. U6a and U6b add public CLI Go witnesses, while the seat runs the four real fixture legs. Each leg reuses a production-rule mutation, not fixture registration. |
| BPRM-R1-005 | Partly folded, still open. Every listed id has its own table row and failure line. Mandatory benchmark integration rules are absent from the supposedly complete inventory. See BPRM-R2-005. |
| BPRM-R1-006 | Closed. The sample size, severity order, id tie-break, environment rule, fixture execution rule, and non-reproduction result are now explicit (`artifacts/reports/bprm-design-r2.md:142-153`, `artifacts/reports/bprm-design-r2.md:173-177`). |
| BPRM-R1-007 | Open. The exception is now admitted honestly, but the binding goal record was not changed. Several later units also precede enforcement. See BPRM-R2-003. |

## Schema, reader, and repair accounting

The provider and canonical schema split is otherwise coherent. The shipped version-1 file is closed (`metasystem/scripts/agents/schemas/implementer.schema.json:3-8`). Default version 2 is regenerated from it for provider output (`metasystem/internal/returnschema/returnschema.go:34-74`, `metasystem/internal/returnschema/returnschema.go:152-187`). Stored return validation regenerates from the same file (`metasystem/internal/validate/returncomplete.go:159-214`). A provider-only required member and a canonical optional member preserve the intended compatibility if the regex is corrected.

The structured-output test should not break. It walks default provider materializations and requires every property to be required and every object closed (`metasystem/internal/returnschema/returnschema_test.go:176-265`). The separate canonical form is not provider output. `TestReturnCompleteNormalizesResolvableDiffBoundaryPaths` and its unchanged-byte case omit mutations (`metasystem/internal/validate/returncomplete_direct_test.go:31-121`). `TestAdjudicateTurnRepairsBareImplementerDiffBoundaryAtAcceptance` also uses a version-2 return without mutations (`metasystem/internal/adapter/adjudicate_test.go:205-271`). The frozen field-roster and static positive inputs remain version 1 (`metasystem/scripts/validate-metasystem.sh:1661-1706`, `metasystem/scripts/validate-metasystem.sh:1911-1947`, `metasystem/scripts/validate-metasystem.sh:2378-2407`). The `implementer-v1-v2` bed currently omits mutations and goes through normalization and completeness (`metasystem/scripts/agents/return-schema-fixtures.sh:132-175`). Canonical omission keeps these fixtures lawful. Provider validation would not.

The executable reader inventory is complete enough. Normalization clones the selected object and preserves admitted extra members (`metasystem/internal/adapter/return.go:55-66`, `metasystem/internal/adapter/return.go:191-224`). Devin, reapers, landed listings, and the shell wrapper delegate validity to the shared completeness checker (`metasystem/internal/adapter/devincollect.go:220-240`; `metasystem/internal/supervise/reaper.go:234-260`; `metasystem/internal/steward/reap.go:225-234`; `metasystem/internal/mission/landed.go:131-155`; `metasystem/scripts/assert-return-complete.sh:47-50`). Follow-up rebase reads only diffBoundary, and mirroring copies whole artifacts (`metasystem/internal/dispatch/followup_rebase.go:123-148`; `metasystem/internal/dispatch/mirror.go:117-139`). Critic, warden, authorization, recertification, landing, closure, host, and mission-runner readers consume other-role fields or their own named field (`metasystem/internal/dispatch/finding_register.go:137-175`; `metasystem/internal/dispatch/read_admission.go:400-432`; `metasystem/internal/dispatch/review_reference.go:17-48`; `metasystem/internal/validate/authorization.go:274-299`; `metasystem/internal/validate/recertification.go:398-425`; `metasystem/internal/landing/observe.go:745-771`; `metasystem/internal/readsubject/closure.go:198-248`; `metasystem/internal/host/hostcollect.go:160-176`; `metasystem/internal/missionrunner/adjudicate.go:71-126`). I found no omitted executable implementer-return reader that needs a second mutations parser.

Acceptance normalization does not invent members. The design correctly changes both repair prompt owners and retains post-repair validation (`metasystem/internal/adapter/adjudicate.go:48-97`, `metasystem/internal/adapter/adjudicate.go:208-246`). The sequencing defect before U3b is part of BPRM-R2-003.

## Rules and 400-line accounting

The eleven Rules arrays and their tables match exactly:

| Unit | Rules ids | Table rows | Planned changed lines |
| --- | ---: | ---: | ---: |
| U1 | 29 | 29 | 380 |
| U2 | 38 | 38 | 340 |
| U3a | 11 | 11 | 300 |
| U3b | 18 | 18 | 290 |
| U4a | 16 | 16 | 380 |
| U4b | 9 | 9 | 300 |
| U5a | 20 | 20 | 360 |
| U5b | 16 | 16 | 310 |
| U5c | 14 | 14 | 330 |
| U6a | 2 | 2 | 350 |
| U6b | 2 | 2 | 340 |

Every listed row names a Go witness and a remove-one failure. Shared enclosing tests use row-specific subtests, which is compatible with the return naming the enclosing function (`artifacts/reports/bprm-design-r2.md:242-250`). U5b-pointer uses by-value and variadic inputs as two negative cases of the one required pointer-node predicate (`artifacts/reports/bprm-design-r2.md:559-572`). I do not count that as a missing row. The missing rules are the benchmark rules named in BPRM-R2-005.

The revision no longer has 18 to 24 ids per unit. It has 2 to 38. U2's 38 includes a replay of U1's 29 existing witnesses, so the header count alone does not require 38 new test implementations. No patch exists, so none of the eleven size estimates is proved. U1 and U4a leave only 20 estimated lines. The concrete-diff stop and rebrief rule at `artifacts/reports/bprm-design-r2.md:242` prevents an over-ceiling landing. The estimates alone are therefore non-material, but U1 must be re-budgeted after BPRM-R2-005 is folded.

## Header, lookup, DONE, and overlap accounting

On a correctly implemented composed path, earlier Rules do not leak from continuations into the current round. Composition records the current raw brief as `caller:brief` before it appends continuations (`metasystem/internal/dispatch/composition.go:237-283`, `metasystem/internal/dispatch/composition.go:300-319`). An oversized brief also does not lose its header. Reference planning retains the raw byte digest and count, and `ReadVerifiedReference` checks the regular file, bytes, and digest (`metasystem/internal/dispatch/composition.go:350-401`; `metasystem/internal/dispatch/references.go:24-49`). Earlier ids remain required only through the intentional chain union. The unresolved sibling fallback can still parse a composed legacy follow-up prompt containing earlier material. That collision is BPRM-R2-002.

A builder can name an existing same-package Go test or same-script fixture leg that does not exercise the rule. The design admits that source membership is not causal proof (`artifacts/reports/bprm-design-r2.md:142-153`). For every sampled id, the critic must pass the named test, remove only the claimed rule, match the assertion and cause, restore, and pass again (`artifacts/reports/bprm-design-r2.md:175`). An unrelated sampled test therefore becomes a material non-reproduction. An unrelated unsampled test can survive, which is the residual risk of the DONE contract's spot check. The lookup is not a second diff owner. It consumes the already computed reviewedTree and uses literal tree entries and blobs (`metasystem/internal/validate/conformance.go:368-390`; `metasystem/internal/gittree/gittree.go:450-499`).

Against DONE, final role and brief wording is covered by U3a. Final missing-row and absent-test refusals are covered by U4b and U5c. The deterministic critic spot check is covered by U3b. Headerless and `Rules: []` compatibility is covered by U4b and U6b. The four public fixture legs are covered by U6a and U6b (`artifacts/reports/bprm-design-r2.md:155-194`, `artifacts/reports/bprm-design-r2.md:618-662`). The design narrows DONE through the early unenforced units in BPRM-R2-003. It makes the required location impossible through BPRM-R2-001. It leaves repair freshness undefined through BPRM-R2-004. It widens DONE with duplicate, unknown, malformed-entry, unreadable-test, and location-association refusals. Those widenings are consistent with a closed evidence contract once their temporal rule is fixed.

The Boundary and Ceiling sibling collides as stated in BPRM-R2-002. The queued round-proof goal exposes the chain gap in BPRM-R2-004. That goal requires the engine to compose a findings record while the seat's follow-up brief supplies only judgment, not a restatement of failures (`metasystem/plans/goals/round-proof-feeds-the-next-brief.md:8-10`). BPRM requires repaired ids to be re-declared only in the caller brief and says the findings record does not replace Rules (`artifacts/reports/bprm-design-r2.md:93`, `artifacts/reports/bprm-design-r2.md:200-207`). No owner converts a recorded failed row into the repair round's Rules. Under the written final-union join, the old entry can still cover the repaired code. The two goals therefore leave a proof-freshness gap rather than a source-parser collision.

## File and line claim audit

Every current-code location cited by revision 2 was opened. Repeated locations are grouped here.

| Design citations checked | Result |
| --- | --- |
| Goal and sibling citations | The goal status, DONE, and budget citations hold (`metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:3-14`). The sibling revision and reader citations do not hold as claimed. See BPRM-R2-002 and BPRM-R2-006. |
| Implementer schema and schema generation | Hold (`metasystem/scripts/agents/schemas/implementer.schema.json:3-39`; `metasystem/internal/returnschema/returnschema.go:34-74`, `metasystem/internal/returnschema/returnschema.go:152-187`; `metasystem/internal/returnschema/returnschema_test.go:176-265`; `metasystem/cmd/metasystem/schema.go:13-46`). Version 1 is frozen and closed. Default version 2 is derived. The provider linter requires closed, fully required objects. |
| Canonical return validation | Holds, subject to the bad proposed regex (`metasystem/internal/validate/returncomplete.go:66-89`, `metasystem/internal/validate/returncomplete.go:141-228`, `metasystem/internal/validate/returncomplete.go:239-303`, `metasystem/internal/validate/returncomplete.go:326-335`, `metasystem/internal/validate/returncomplete.go:419-424`, `metasystem/internal/validate/returncomplete.go:531-538`; `metasystem/cmd/metasystem/validate_verbs.go:178-201`; `metasystem/scripts/assert-return-complete.sh:47-50`). The public paths share `checkReturn`, and boundary rewriting serializes the whole map. |
| Brief parsing and authority | Holds for current code (`metasystem/internal/dispatch/brief.go:41-104`, `metasystem/internal/dispatch/brief.go:121-171`). Working Mode and authority are owned there. `create:` is only output classification. No current Rules parser exists. |
| Composition and retained artifacts | Hold (`metasystem/internal/dispatch/composition.go:52-69`, `metasystem/internal/dispatch/composition.go:237-319`, `metasystem/internal/dispatch/composition.go:366-401`; `metasystem/scripts/agents/dispatch.sh:1918-1929`, `metasystem/scripts/agents/dispatch.sh:2719-2725`, `metasystem/scripts/agents/dispatch.sh:2883-2888`; `metasystem/internal/dispatch/references.go:24-49`). Current brief source identity, continuation order, reference verification, and persisted root/follow-up artifacts have the claimed shapes. |
| Existing successor reader | Holds (`metasystem/internal/validate/conformance.go:930-960`). Without a task-direction reference it returns the whole prompt, so it is unsafe for Rules. |
| Review, boundaries, and stage routing | Hold (`metasystem/internal/validate/conformance.go:139-254`, `metasystem/internal/validate/conformance.go:276-334`, `metasystem/internal/validate/conformance.go:339-428`, `metasystem/internal/validate/conformance.go:724-819`). Review snapshots before persistence, cumulative boundary raises the round limit and unions returns, project projection is reused, and merge or recertification routes separately. The temporal mutation semantics are not supplied by this code. |
| Git candidate reads | Hold (`metasystem/internal/gittree/gittree.go:246-276`, `metasystem/internal/gittree/gittree.go:450-499`). Snapshot uses an isolated index and `git add -A`. Entries uses literal paths. FileAt reads the selected tree blob. |
| Acceptance and repair | Hold for the stated current behavior (`metasystem/internal/adapter/return.go:25-94`, `metasystem/internal/adapter/return.go:184-225`; `metasystem/internal/adapter/adjudicate.go:48-97`, `metasystem/internal/adapter/adjudicate.go:208-246`; `metasystem/internal/validate/returncomplete_direct_test.go:31-121`; `metasystem/internal/adapter/adjudicate_test.go:205-271`). Normalization preserves members, the two prompt owners are separate, and repaired returns are validated again. The early sequencing issue remains. |
| Capability, role, templates, and packet recipe | Hold (`metasystem/internal/capability/select.go:82-110`; `metasystem/scripts/agents/roles/implementer.requirements.json:2-15`; `metasystem/scripts/agents/roles/implementer.md:11-17`; `metasystem/scripts/agents/templates/brief.md:14-40`; `metasystem/scripts/agents/templates/follow-up.md:12-18`; `metasystem/scripts/agents/role-packets.json:38-59`; `metasystem/skills/code-critique/SKILL.md:12-37`). Requirements names are capabilities. Recipes can deliver a file. Current role and critic text lacks the new mutation contract. |
| Fixture and public CLI anchors | Hold (`metasystem/scripts/agents/conformance-fixtures.sh:38-107`, `metasystem/scripts/agents/conformance-fixtures.sh:153-186`; `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`; `metasystem/cmd/metasystem/validate_verbs.go:276-352`; `metasystem/testing.json:75-79`). The current legs use the public review verb, status and diagnostic checks, and persisted artifacts. The fixture selector source forms exist at the cited lines. |
| Runtime and compatibility consumers | Hold except fake materialization (`metasystem/scripts/agents/adapters/runtime-common.sh:103-110`, `metasystem/scripts/agents/adapters/runtime-common.sh:364-385`; `metasystem/internal/adapter/devincollect.go:220-240`; `metasystem/internal/adapter/fake.go:53-112`; `metasystem/scripts/agents/adapters/fake.sh:140-160`, `metasystem/scripts/agents/adapters/fake.sh:329-332`; `metasystem/internal/adapter/selftestrun.go:99-125`, `metasystem/internal/adapter/selftestrun.go:217-237`, `metasystem/internal/adapter/selftestrun.go:335-342`; `metasystem/internal/adapter/devin.go:337-338`). See BPRM-R2-006 for fake. |
| Other return and artifact readers | Hold (`metasystem/internal/dispatch/followup_rebase.go:124-148`; `metasystem/internal/dispatch/review_reference.go:17-48`, `metasystem/internal/dispatch/review_reference.go:313-325`; `metasystem/internal/dispatch/mirror.go:120-139`; `metasystem/internal/supervise/reaper.go:234-260`; `metasystem/cmd/metasystem/supervise_component.go:325-326`; `metasystem/internal/steward/reap.go:106-107`, `metasystem/internal/steward/reap.go:225-234`; `metasystem/internal/mission/landed.go:101-155`; `metasystem/scripts/agents/dispatch.sh:1336-1349`). They either use shared completeness, read only an owned field, or copy whole artifacts. |
| Critic, warden, host, and mission readers | Hold (`metasystem/internal/dispatch/finding_register.go:80-82`, `metasystem/internal/dispatch/finding_register.go:137-175`, `metasystem/internal/dispatch/finding_register.go:413-429`, `metasystem/internal/dispatch/finding_register.go:652-663`, `metasystem/internal/dispatch/finding_register.go:1287-1293`; `metasystem/internal/dispatch/read_admission.go:400-432`; `metasystem/internal/validate/conformance.go:1169-1185`; `metasystem/internal/validate/authorization.go:274-299`; `metasystem/internal/validate/recertification.go:398-425`; `metasystem/internal/landing/observe.go:750-771`; `metasystem/internal/readsubject/closure.go:198-248`; `metasystem/internal/host/hostcollect.go:174`; `metasystem/internal/missionrunner/adjudicate.go:71-126`). These consume critic, warden, or orchestrator contracts rather than implementer mutation members. |
| Static and telemetry fixtures | Hold (`metasystem/scripts/validate-metasystem.sh:1661-1706`, `metasystem/scripts/validate-metasystem.sh:1917`, `metasystem/scripts/validate-metasystem.sh:1946-1947`, `metasystem/scripts/validate-metasystem.sh:2388-2407`; `metasystem/scripts/agents/telemetry-census-fixtures.sh:48-72`; `metasystem/scripts/agents/fingerprint-harness.sh:131-143`, `metasystem/scripts/agents/fingerprint-harness.sh:243-246`). The field roster is version 1. Static implementer inputs omit mutations. Telemetry normalizes without completeness. Fingerprint dispatches a design critic. |
| Benchmark citations | Hold as current facts (`benchmark/validate-kit.sh:2-6`, `benchmark/validate-kit.sh:80-102`; `benchmark/schemas/evidence/implementer.schema.json:4`, `benchmark/schemas/evidence/implementer.schema.json:111-125`; `benchmark/extractor.py:63-124`, `benchmark/extractor.py:324-341`, `benchmark/extractor.py:514-538`; `benchmark/extractor-fixtures.sh:260-274`; `benchmark/rubrics/evidence-honesty.md:15-22`; `benchmark/rubrics/brief-quality.md:15-22`). Their proposed proof coverage does not hold. See BPRM-R2-005. |
| Test selection and delivery guidance | Hold (`metasystem/metasystem.conf:102-105`; `metasystem/cmd/metasystem/test.go:148-183`; `metasystem/docs/orchestration.md:172`; `metasystem/docs/project-rules.md:17`; `metasystem/docs/design/design-obligation-gate.md:15-17`). The selected extra suite exists, but it is not an implementer-schema suite. |

Verdict: fold these findings first.
