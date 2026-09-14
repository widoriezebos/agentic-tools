# Design critique: builder proves each rule by mutation, round 1

## Material findings

### BPRM-R1-001

Severity: critical

Material: yes

Claim: All five builder briefs use a Boundary and Ceiling grammar that the required predecessor will refuse. The design treats authority-path syntax as Boundary syntax. An implementer cannot start any unit as written after the predecessor lands.

Evidence: The design says, "None depends on the Boundary/Ceiling sibling" and "It changes no Boundary/Ceiling policy" (`artifacts/reports/bprm-design.md:290-292`). Each unit then uses a comma-delimited `Boundary:` value and a `Ceiling:` value with words (`artifacts/reports/bprm-design.md:304-310`, `artifacts/reports/bprm-design.md:341-347`, `artifacts/reports/bprm-design.md:376-382`, `artifacts/reports/bprm-design.md:411-417`, `artifacts/reports/bprm-design.md:446-452`). The predecessor requires Boundary to be a JSON array and Ceiling to be a bare unsigned decimal (`metasystem/plans/brief-declares-the-round-boundary-design.md:19-31`). It routes both initial and follow-up admission through the Go header parser (`metasystem/plans/brief-declares-the-round-boundary-design.md:48`; `metasystem/scripts/agents/dispatch.sh:858-864`, `metasystem/scripts/agents/dispatch.sh:1542`, `metasystem/scripts/agents/dispatch.sh:2643`). The cited `create:` logic only classifies a path mention as an authority output. It does not define the Boundary grammar (`metasystem/internal/dispatch/brief.go:121-171`). Rewrite all five headers in the predecessor's grammar. Remove the `create:` spelling from those JSON arrays.

### BPRM-R1-002

Severity: high

Material: yes

Claim: Current-round-only mutation checking can admit a final candidate whose earlier-round rules remain unproved or whose earlier named tests were later removed. The manual instruction to the seat is not chain closure.

Evidence: The design says, "Review uses the current round's `acceptanceRules`. It does not union prior round IDs or borrow another round's entries" and places responsibility on the seat to carry still-unproved rules (`artifacts/reports/bprm-design.md:121`). Review snapshots and judges the whole current candidate, not only the current round's edits (`metasystem/internal/validate/conformance.go:368-390`). Existing boundary enforcement reads every round return through the current round and unions their declarations (`metasystem/internal/validate/conformance.go:754-819`). Follow-up composition already carries the prior return when present (`metasystem/scripts/agents/dispatch.sh:2719-2725`). A round can therefore fail mutation review, receive a correction brief that names only a new or repaired row, and later pass while the old missing row is no longer checked. A later round can also delete an earlier named test. Preserve unresolved mutation obligations across the chain, or state and enforce an equivalent final-candidate closure rule.

### BPRM-R1-003

Severity: high

Material: yes

Claim: The promised legacy follow-up fallback can read Rules from an earlier brief. It can also turn one current header plus one earlier header into a duplicate-header refusal.

Evidence: The design says, "On later rounds use `successorTaskDirection`, which already handles verified staged task direction references" (`artifacts/reports/bprm-design.md:135`). For an inline task direction, the current function returns the entire composed prompt, not the task-direction source (`metasystem/internal/validate/conformance.go:930-960`). Composition puts the current task direction first and appends continuations later (`metasystem/internal/dispatch/composition.go:268-319`). Follow-ups append the original brief and may append the prior return (`metasystem/scripts/agents/dispatch.sh:2719-2725`). The predecessor design already records that this function cannot be reused unchanged for current-round headers (`metasystem/plans/brief-declares-the-round-boundary-design.md:67-79`). Use the exact current task-direction source or verified reference for legacy compositions. The new composition path does not lose an oversized current header because it reads raw `briefBytes` before reference planning (`metasystem/internal/dispatch/composition.go:237-268`, `metasystem/internal/dispatch/composition.go:395-401`).

### BPRM-R1-004

Severity: high

Material: yes

Claim: Four U5 rules have a named registration test, but no named builder-run test that exercises the behavior the rule claims. The unit therefore does not prove the four DONE fixtures by mutation before return.

Evidence: The design says `TestMutationFixtureContract` checks that the four declarations remain, and explicitly says, "This is a fixture-registration check, not a claim that the legs ran" (`artifacts/reports/bprm-design.md:457-463`). It also forbids the builder from running either fixture bed (`artifacts/reports/bprm-design.md:475`). In the existing bed, behavioral proof comes from invoking conformance and checking exit status and exact diagnostic text (`metasystem/scripts/agents/conformance-fixtures.sh:153-170`). A real successful leg also invokes review and checks its artifacts (`metasystem/scripts/agents/conformance-fixtures.sh:175-186`). Keeping a `new_case` line while deleting or weakening its body would pass the proposed registration test. `U5-missing-row`, `U5-absent-test`, `U5-complete`, and `U5-no-rows` lack a named builder-run test that exercises their claimed behavior. Give each leg a runnable focused test, or let U5 run those four isolated fixture selectors and use their observed failures as its mutation proof.

### BPRM-R1-005

Severity: high

Material: yes

Claim: The acceptance inventory violates its own one-independently-removable-rule-per-row rule. It permits one mutation entry and one representative failure line to stand for several independent removals.

Evidence: The design says, "The brief author must enumerate every added rule, with one independently removable rule per row" (`artifacts/reports/bprm-design.md:115`). It later makes `U1-entry-fields` cover all four entry fields, `U1-test-fields` cover all three test fields, and `U1-test-text` cover both path and name constraints (`artifacts/reports/bprm-design.md:481-489`). It expressly allows one representative `failureLine` for several variants (`artifacts/reports/bprm-design.md:489`). The current schema demonstrates that required members are independent constraints (`metasystem/scripts/agents/schemas/implementer.schema.json:7-39`). The structured-output invariant also checks every property independently against `required` and object closure (`metasystem/internal/returnschema/returnschema_test.go:199-231`). The same bundling recurs in U4 for signature and mode alternatives. Split every independently removable constraint into its own row and entry. Otherwise an implementer will build a coarser proof contract than the design's rule and the goal's intent require.

### BPRM-R1-006

Severity: medium

Material: yes

Claim: The existence lookup can admit an unrelated test, and the critic instruction gives no minimum sample size or selection rule. Two critics can follow it and perform materially different proof.

Evidence: The design says, "This is deliberately an existence gate" and concedes that it cannot establish the observed causal failure (`artifacts/reports/bprm-design.md:176-187`). Its critic instruction says only, "Spot-check a sample" and expands the sample after evidence of a wider problem (`artifacts/reports/bprm-design.md:215-219`). The current critic skill says to challenge named tests and run focused checks, but sets no sample selection or size (`metasystem/skills/code-critique/SKILL.md:33-37`). The proposed Go lookup accepts a source declaration with the right name and signature. The fixture lookup accepts a selector line. Neither checks that the test exercises the claimed location. A builder can name an existing unrelated test and pass conformance. Define a deterministic minimum and selection rule, including treatment of platform-excluded Go tests and selector-looking shell data. This remains a spot check, but it stops "sample" from meaning one convenient row.

### BPRM-R1-007

Severity: medium

Material: yes

Claim: U1 is an explicit exception to the DONE return contract. Its per-row entries are embedded as JSON text inside one evidence string and cannot be joined or refused by conformance.

Evidence: The design says, "The U1 dispatch has a bootstrap exception in representation only" and directs U1 to place the table in `evidence[].observed` instead of top-level `mutations` (`artifacts/reports/bprm-design.md:298`). The DONE line requires the implementer return to carry a mutation entry per acceptance row and review to refuse missing entries (`metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:8`). The bootstrap cause is real: runtime launch materializes the schema before the builder changes it (`metasystem/scripts/agents/adapters/runtime-common.sh:103-110`), and the current closed implementer schema rejects an extra top-level field (`metasystem/scripts/agents/schemas/implementer.schema.json:5-8`). That does not make the exception representation-only. It removes machine enforcement for the first 18 rows. Obtain an explicit contract exception, or land a smaller schema bootstrap before dispatching the first mutation-governed unit.

## Non-material findings

### BPRM-R1-008

Severity: low

Material: no

Claim: The reader inventory misses several regression consumers. The omission does not break the proposed optional-canonical split.

Evidence: The design says its inventory covers executable consumers and lists tests and fixtures separately (`artifacts/reports/bprm-design.md:253-280`). It does not name the frozen version-1 field-roster check in `metasystem/scripts/validate-metasystem.sh:1661-1706`, the byte-identity version-2 tests in `metasystem/internal/validate/returncomplete_direct_test.go:31-121`, the version-2 adjudication fixture in `metasystem/internal/adapter/adjudicate_test.go:205-250`, or the version-2 telemetry fixture in `metasystem/scripts/agents/telemetry-census-fixtures.sh:48-60`. These remain valid because version 1 stays frozen and canonical version 2 permits omission. Add them to the inventory so an implementer knows they are compatibility witnesses.

### BPRM-R1-009

Severity: low

Material: no

Claim: The five 400-line estimates are not established. The design has an adequate stop rule, so the estimates alone do not justify a blocking finding.

Evidence: The design estimates U1 and U2 at 390 changed lines, U3 at 330, U4 at 370, and U5 at 230 (`artifacts/reports/bprm-design.md:300-302`, `artifacts/reports/bprm-design.md:337-339`, `artifacts/reports/bprm-design.md:372-374`, `artifacts/reports/bprm-design.md:407-409`, `artifacts/reports/bprm-design.md:442-444`). The Rules lists contain 18, 24, 18, 20, and 11 IDs, not 18 to 24 in every unit (`artifacts/reports/bprm-design.md:309`, `artifacts/reports/bprm-design.md:346`, `artifacts/reports/bprm-design.md:381`, `artifacts/reports/bprm-design.md:416`, `artifacts/reports/bprm-design.md:451`). The existing structured-output test alone is about 90 lines and recursively checks every property (`metasystem/internal/returnschema/returnschema_test.go:170-267`). No computed patch proves the estimates. However, the design orders an implementer to return a gap before exceeding 400 (`artifacts/reports/bprm-design.md:290`). Keep that stop rule. Re-estimate after splitting the compound rows in BPRM-R1-005.

### BPRM-R1-010

Severity: low

Material: no

Claim: Two source citations are imprecise, but their underlying facts hold.

Evidence: The design says the return validator supports a listed keyword subset (`artifacts/reports/bprm-design.md:18`). The cited map also supports `const` (`metasystem/internal/validate/returncomplete.go:419-424`). The design says the review call site proves that Snapshot captures uncommitted and new unignored work (`artifacts/reports/bprm-design.md:170`). The call site only invokes Snapshot (`metasystem/internal/validate/conformance.go:368-390`). The actual capture is implemented by an isolated index and `git add -A` (`metasystem/internal/gittree/gittree.go:246-276`). Neither qualification changes the proposed implementation. The absolute installed-Go citation exists and supports the TestMain distinction, but it is a host path rather than repository evidence (`/opt/homebrew/opt/go/libexec/src/cmd/go/internal/load/test.go:753-766`).

## Contract and seam accounting

The version-1 and version-2 schema split is buildable as specified. The shipped version-1 file is closed and frozen (`metasystem/scripts/agents/schemas/implementer.schema.json:3-39`). Version 2 is regenerated from that file today (`metasystem/internal/returnschema/returnschema.go:29-74`, `metasystem/internal/validate/returncomplete.go:161-214`). A strict provider overlay can require `mutations`, while a canonical overlay can retain it as an optional property. The structured-output invariant only walks default materializations (`metasystem/internal/returnschema/returnschema_test.go:176-265`). It need not reject the separate canonical schema.

No existing compatibility witness must break under that exact split. `TestReturnCompleteNormalizesResolvableDiffBoundaryPaths` and its byte-identity case omit mutations (`metasystem/internal/validate/returncomplete_direct_test.go:31-121`). The static positive implementer return also omits it (`metasystem/scripts/validate-metasystem.sh:1940-2016`, `metasystem/scripts/validate-metasystem.sh:2365-2423`). The `implementer-v1-v2` fixture omits it today (`metasystem/scripts/agents/return-schema-fixtures.sh:99-175`). The telemetry fixture does too (`metasystem/scripts/agents/telemetry-census-fixtures.sh:48-60`). Canonical omission keeps all four valid. Changing the shipped version-1 schema or using the strict provider schema for stored returns would break them.

The benchmark seam is also coherent if both files land together. The kit currently requires byte identity with default materialized version 2 (`benchmark/validate-kit.sh:73-102`). The extractor validates every return against that pin (`benchmark/extractor.py:324-341`, `benchmark/extractor.py:514-538`). Its historical fixture has no mutations member (`benchmark/extractor-fixtures.sh:260-274`). Regenerating `benchmark/schemas/evidence/implementer.schema.json` from the proposed canonical materializer and changing only the implementer drift command preserves byte identity and historical admission. The acceptance repair has two prompt owners and validates again after ordinary repair (`metasystem/internal/adapter/adjudicate.go:65-97`, `metasystem/internal/adapter/adjudicate.go:208-246`). The design changes both prompts and asks neither adapter path to synthesize an entry. False model claims remain the sampling problem in BPRM-R1-006.

Against DONE, the design covers role and brief wording (`artifacts/reports/bprm-design.md:189-207`), missing-row and absent-test refusals (`artifacts/reports/bprm-design.md:139-160`), unchanged no-row admission (`artifacts/reports/bprm-design.md:140`, `artifacts/reports/bprm-design.md:230`), and the four named fixtures (`artifacts/reports/bprm-design.md:221-234`). It narrows the per-row return requirement for U1, as BPRM-R1-007 explains. It narrows one-rule-per-entry proof through compound rows, as BPRM-R1-005 explains. It widens review beyond DONE by refusing duplicate rows, unknown rows, malformed entries, unreadable tests, and invalid test paths (`artifacts/reports/bprm-design.md:139-160`). Those widenings are consistent with a closed evidence contract. The critic spot check is present but underspecified, as BPRM-R1-006 explains.

The named-test lookup is not a second diff owner. It consumes the `reviewedTree` already computed by review (`metasystem/internal/validate/conformance.go:368-390`) and reads literal entries and blobs from that tree (`metasystem/internal/gittree/gittree.go:450-499`). Its source-membership result is weaker than causal proof, but it does not recompute the diff.

The Boundary/Ceiling sibling collides directly, as BPRM-R1-001 and BPRM-R1-003 explain. The queued round-proof goal has no implementation or design to collide with yet. Its contract will record bed failures beside a round and gate follow-ups that ignore that file (`metasystem/plans/goals/round-proof-feeds-the-next-brief.md:6-10`). That record is not a substitute for carrying unresolved mutation rows. It records failing bed assertions, not all earlier mutation claims. This leaves the chain gap in BPRM-R1-002. A later integration must also keep Rules parsing limited to the current task direction and must not parse a referenced findings file as a Rules source.

## File and line claim audit

Every cited location in the design was opened. The results are grouped by owner. Repeated citations are listed once.

| Design citations checked | Result |
| --- | --- |
| `metasystem/plans/goals/builder-proves-each-rule-by-mutation.md:8` | Holds. This is the DONE contract used above. |
| `metasystem/scripts/agents/schemas/implementer.schema.json:3-39`, `:22-33`; `metasystem/internal/returnschema/returnschema.go:29-74`, `:154-187`; `metasystem/internal/returnschema/returnschema_test.go:176-265`; `metasystem/scripts/agents/adapters/runtime-common.sh:103-110,364-368,383`; `metasystem/cmd/metasystem/schema.go:13-46` | Holds. Version 1 is closed. Version 2 is derived. Runtime uses version 2 for implementers. Default materializations are checked for required closed objects. See BPRM-R1-010 for the omitted `const` keyword. |
| `metasystem/internal/validate/returncomplete.go:66-89,141-155,161-228`, `:239-303`, `:326-335`, `:419-448`, `:531-538`; `metasystem/cmd/metasystem/validate_verbs.go:178-201`; `metasystem/scripts/assert-return-complete.sh:48-50` | Holds. All public completeness entries share `checkReturn`. Boundary normalization rewrites the whole map, so an extra admitted member is preserved. Required-field diagnostics have the claimed form. |
| `metasystem/scripts/agents/roles/implementer.requirements.json:2-15`; `metasystem/internal/capability/select.go:82-125` | Holds. `required` contains capability names and is empty for the implementer. |
| `metasystem/scripts/agents/role-packets.json:38-59`; `metasystem/internal/dispatch/composition.go:237-268`, `:275-283`, `:300-310`, `:395-401`, `:437-467` | Holds. The current brief is read before source assembly and reference planning. Requirements can be delivered as a source. Continuations follow the task direction. Composition is persisted. |
| `metasystem/internal/dispatch/brief.go:18`, `:37-105`, `:121-172`, `:156-160`, `:165-167` | The current code facts hold. The design's use of authority `create:` syntax as future Boundary syntax does not hold. See BPRM-R1-001. |
| `metasystem/scripts/agents/dispatch.sh:1336-1349`, `:2719-2725`; `metasystem/internal/validate/conformance.go:914-960` | Lost-return recollection and prior-return transport claims hold. The claim that the successor reader yields only an inline current task direction does not hold. See BPRM-R1-003. |
| `metasystem/internal/validate/conformance.go:214-223`, `:276-292`, `:311-334`, `:355-428`, `:442-580`, `:584-643`, `:617-643`, `:754-819`; `metasystem/internal/validate/conformance_test.go:48-85` | Holds. Review uses the project snapshot, cumulative boundary owner, and immutable artifacts. The fixture constructor makes real controller and worktree repositories. Current-round-only mutation semantics are still a design gap. See BPRM-R1-002. |
| `metasystem/internal/gittree/gittree.go:450-499`; `/opt/homebrew/opt/go/libexec/src/cmd/go/internal/load/test.go:753-766` | Holds. Entries uses literal paths and FileAt reads the tree blob. The installed loader distinguishes TestMain signatures. The absolute citation is nonportable. |
| `metasystem/scripts/agents/conformance-fixtures.sh:38-64`, `:73-107`, `:153-175`, `:175-186`; `metasystem/scripts/agents/return-schema-fixtures.sh:132-175`; `metasystem/testing.json:74`, `:78` | Holds. The named helpers, selectors, scenarios, and test groups exist. The proposed U5 Go test proves registration only. See BPRM-R1-004. |
| `metasystem/scripts/agents/roles/implementer.md:11-13`; `metasystem/scripts/agents/templates/brief.md:14-20`, `:32-40`; `metasystem/scripts/agents/templates/follow-up.md:12-18`; `metasystem/skills/code-critique/SKILL.md:12`, `:18-37` | Holds. Current text names passing evidence and return members but no mutation run. The critic has no sampling rule. |
| `metasystem/internal/adapter/adjudicate.go:48-78`, `:84-97`, `:208-246`; `metasystem/internal/adapter/adjudicate_test.go:205-250` | Holds. There are ordinary and empty-delivery repair prompts. Ordinary repair is revalidated. The cited test is a usable adjudication precedent. |
| `metasystem/internal/adapter/return.go:25-27`, `:37-94`, `:191-200`; `metasystem/cmd/metasystem/adapter_selftest_verbs.go:23-38`; `metasystem/internal/adapter/devincollect.go:220-240`; `metasystem/internal/adapter/fake.go:109-112`; `metasystem/scripts/agents/adapters/fake.sh:149,332` | Holds. Normalization clones the selected object and preserves unknown admitted members. Candidate scoring need not own mutations. Devin and fake paths use shared completeness. Fake implementer returns contain no mutation field. |
| `metasystem/internal/dispatch/followup_rebase.go:97-151`; `metasystem/internal/dispatch/review_reference.go:17-48`, `:304-326`; `metasystem/internal/dispatch/mirror.go:103-139` | Holds. These readers either inspect only their owned fields or copy the whole artifact. They need no mutation parser. |
| `metasystem/internal/supervise/reaper.go:234-260`; `metasystem/cmd/metasystem/supervise_component.go:325-326`; `metasystem/internal/steward/reap.go:106-107`, `:227-231`; `metasystem/internal/mission/landed.go:101-121`, `:131-155` | Holds. These paths delegate return validity to the shared completeness checker. |
| `metasystem/internal/adapter/selftestrun.go:99-118,217,237,335-342`; `metasystem/internal/adapter/devin.go:337-338`; `metasystem/scripts/agents/fingerprint-harness.sh:134` | Holds. These paths rely on shared validation or recursive marker inspection. They do not enumerate implementer mutation fields. |
| `metasystem/internal/dispatch/finding_register.go:80-82,137-175,652-663,1287`; `metasystem/internal/dispatch/read_admission.go:400-432`; `metasystem/internal/validate/conformance.go:1170`; `metasystem/internal/validate/authorization.go:280-299`; `metasystem/internal/validate/recertification.go:408`; `metasystem/internal/landing/observe.go:760`; `metasystem/internal/readsubject/closure.go:239` | Holds. These are critic or warden readers. They do not validate implementer mutation entries. |
| `metasystem/internal/host/hostcollect.go:174`; `metasystem/internal/missionrunner/adjudicate.go:71-88,120-126` | Holds. These validate host or orchestrator returns, not implementer returns. |
| `metasystem/scripts/validate-metasystem.sh:1986-2038`, `:2389-2423` | Holds for the cited invocations. The positive version-2 implementer object is constructed at `metasystem/scripts/validate-metasystem.sh:1940-1947` and omits mutations. Canonical omission keeps it valid. |
| `benchmark/validate-kit.sh:2-6`, `:84-92`; `benchmark/schemas/evidence/implementer.schema.json:4,111-125`; `benchmark/extractor.py:80-84`, `:93-119`, `:324-341`, `:514-538`; `benchmark/extractor-fixtures.sh:260-274`; `benchmark/rubrics/evidence-honesty.md:15`; `benchmark/rubrics/brief-quality.md:19` | Holds. The pin is byte-compared, the extractor validates through it, the fixture omits mutations, and the rubrics do not enumerate return members. The proposed canonical pin is compatible. |
| `metasystem/metasystem.conf:105`; `metasystem/cmd/metasystem/test.go:148-183`; `metasystem/docs/project-rules.md:17`; `metasystem/docs/design/design-obligation-gate.md:15-17` | Holds. The extra suite and test flags exist. Diagnostic selection does not replace delivery proof. |

Verdict: rework.
