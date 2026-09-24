# Defect analysis earns the fix

- Kind: design
- Id: 01M3A2YHDMJY93Q1PFHVN3XE6P
- Status: accepted
- Goals: defect-analysis-gate

Design for defect-analysis-gate, against repository commit 0acef6eb6. This page is the umbrella context for the three members in section 6.
Drafted by a Codex design round for the m1b seat on 2026-09-13 (chain implementer-873925bde8faae7cf849fd18) on the seat's brief; the rostered Codex design read is the next act, then the members are opened and built on the roster lane.
The contract is the intent and next step in `metasystem/plans/goals/defect-analysis-gate.md:6` and the ladder in `metasystem/docs/paper/12-learning-systems.md:17`.
The chosen boundary is dispatch admission. A goal may hold an investigation before its analysis survives. It cannot authorize a fix dispatch until then.

## 1. What a defect-fix item is today and where one can enter

There is no typed defect-fix item today. `GoalFile` has intent, risk, budget and history, but no defect or analysis member (`metasystem/internal/goal/file.go:23`).
The seat's opening rule says “defect” but checks a claimed blocker relationship, not a diagnosis (`metasystem/internal/goal/verbs.go:635`, `metasystem/internal/goal/verbs.go:777`).
These are the creation and conversion paths. None carries mechanically challenged defect evidence.

| Path | Existing boundary and evidence carried |
| --- | --- |
| Synced public open | `runGoalOpen` enters the shared router at `metasystem/cmd/metasystem/goal.go:167` and `metasystem/cmd/metasystem/goal.go:121`. `metasystem/cmd/metasystem/goalsync_mutations.go:830` calls `OpenRisked` at line 874. It carries intent, origin, next step, risk and basis, tier, budget, labels and an optional blocker. |
| Compatibility library open | `Open` and `OpenTiered` reach `openRequest` with no risk or blocker (`metasystem/internal/goal/verbs.go:616`, `metasystem/internal/goal/verbs.go:622`, `metasystem/internal/goal/verbs.go:675`). They are fixture conveniences, not the synced command's path. |
| Legacy public open | Unconverted roots fall through at `metasystem/cmd/metasystem/goalsync_mutations.go:813` to `Store.Open` (`metasystem/internal/goal/goalverbs.go:329`). They carry intent, origin and next step. Legacy Current may hold free-text Evidence (`metasystem/internal/goal/goal.go:49`). |
| Edit or set-next | `metasystem/cmd/metasystem/goalsync_mutations.go:2043` and `metasystem/cmd/metasystem/goalsync_mutations.go:927` reach `Edit` (`metasystem/internal/goal/verbs.go:2244`). Intent, risk and next step can change. The evidence argument concerns risk classification; its accepted grammar is at `metasystem/internal/dispatch/admission.go:17`. |
| Synced reconcile | `metasystem/internal/goal/reconcilemap.go:116` maps hand-created files to open. `metasystem/internal/goal/reconcilepub.go:242` constructs a goal separately; line 369 applies edits. It copies intent, origin, next step, blockers and labels, without reproduction evidence. |
| Legacy genesis and adoption | `metasystem/cmd/metasystem/goal.go:254` reaches `metasystem/internal/goal/goalverbs.go:567`; genesis baselines legacy records at line 597. Adoption invokes it at `metasystem/scripts/adopt.sh:435`. Its evidence is authority and ledger shape. |
| Migration and manifest additions | `metasystem/cmd/metasystem/goalsync_verbs.go:373` calls migration. `metasystem/internal/goal/migrate.go:297` synthesizes goals; line 322 preserves old evidence as legacy prose; line 379 admits manifest additions. |
| Recovery replay | `metasystem/internal/goal/recover.go:266` reconstructs open through `openRequest`; lines 387 and 427 reconstruct edits. Journaled risk may be absent. Recovery has no separate analysis test. |
| Reopen | `metasystem/internal/goal/verbs.go:2177` restores an archived goal to queued and clears approval and budget. Legacy reopen is separate (`metasystem/internal/goal/goalverbs.go:505`). Neither re-examines a diagnosis. |
| Split | `metasystem/internal/goal/split.go:62` reads member drafts; line 264 constructs children with their own intent and inherited context. No risk or reproduction is copied. |
| Retired open-and-claim | `metasystem/internal/goal/verbs.go:2570` refuses it; its recovery mutation also refuses at line 2589. It is not a successful intake path. |

The dispatch paths are broader than the public delegate command.

| Path | Existing boundary and evidence carried |
| --- | --- |
| Fresh goal-bound delegate | `metasystem/cmd/metasystem/delegate.go:62` authorizes the shell driver. `metasystem/scripts/agents/dispatch.sh:1591` resolves the goal; lines 1799 and 1800 run revision and lineage admission before reservation. |
| Explicit goal-free delegate | `none-explicit` is normalized away at `metasystem/cmd/metasystem/delegate.go:304`. Revision admission then returns early (`metasystem/scripts/agents/dispatch.sh:711`). Lineage admission still runs. |
| Follow-up, resume and fresh-context continuation | `metasystem/cmd/metasystem/delegate.go:252` enters the same driver. `metasystem/scripts/agents/dispatch.sh:2440` inherits the parent's role and context; line 2451 inherits its goal. Lines 2681 and 2682 repeat admission for the new reservation. |
| Repeated operation | `PREFLIGHT-MATCHED` skips a second admission at `metasystem/scripts/agents/dispatch.sh:1797` and `metasystem/scripts/agents/dispatch.sh:2679`. This reuses an existing reservation; it must not authorize different inputs. |
| Mission, steward and adapter self-test | Mission context enters the driver at `metasystem/scripts/agents/dispatch.sh:892` and line 1553. Steward and self-test route through `metasystem/cmd/metasystem/delegate.go:236` and line 242. They reach the same reservation machinery, with goal-free work still possible. |
| Go launch owner | `ClaimLaunch` at `metasystem/internal/dispatch/claim.go:221` checks fingerprints, hazard and provenance. It does not call either goal admission evaluator. Lines 558 through 568 mark and publish a reservation. The command is capability-protected (`metasystem/cmd/metasystem/dispatch_verbs.go:240`). |
| Direct record creation | `metasystem/cmd/metasystem/dispatch_verbs.go:168` reaches `metasystem/internal/dispatch/record.go:233`. It can create pending setup data. That record alone is not proof that a provider launched or that goal admission ran. |
| Governed proof execution | `metasystem/cmd/metasystem/proof_run.go:536` calls revision admission as an implementer; line 571 repeats after extension. An existing reservation owner skips that call. These are proof processes, so a fix-only check must not prevent collecting its own evidence. |

`EvaluateGoalAdmission` is a seat-wide budget walk, with early returns for old ledgers and empty lineages (`metasystem/internal/dispatch/admission.go:92`). It has no proposed work subject.
`EvaluateGoalRevisionAdmissionForDispatch` checks the accepted claim revision, risk, hazard and budgets (`metasystem/internal/dispatch/admission.go:189`). Missing risk may only mark at line 220.
Reservations carry goal revision, tier, width, role, review subject, input identity and runtime provenance (`metasystem/internal/dispatch/claim.go:718`). Neither evaluator joins an analysis.
The risk answers are separate integers from one through three. Tier is the worse of severity and novelty; accumulation can widen testing (`metasystem/internal/goal/file.go:102`, `metasystem/internal/goal/file.go:133`).

## 2. The mechanism

**The record.** Extend `GoalFile` in `metasystem/internal/goal/file.go` with `WorkKind` and `DefectAnalysis`. The analysis lives inside the existing per-goal record in `metasystem/plans/goals`, and follows that record into its archive.
`WorkKind` is `other`, `defect-fix`, or the read-only legacy value `unknown`. A defect fix promises to remove an observed violation of existing behavior. A feature, investigation-only goal or refactor declares `other`.
New opens require an explicit kind. A seat open with origin main and a blocker requires `defect-fix`. Legacy imports and compatibility calls without a kind produce `unknown`, never an inferred exemption.
`goal edit --work-kind <kind> --basis <reason>` classifies an unknown goal. Changing an established defect fix to other requires the existing human unapprove, intent-edit and approve path; it clears the analysis.
Reconcile, migration, recovery and split preserve the kind explicitly. Split children inherit defect-fix when their parent is a defect fix, but need their own analysis attachment. Reopen retains history and clears challenge validity.
No keyword search in an identifier, label, brief or intent decides this law.

Serialize `DefectAnalysis` as one strict canonical JSON value after `DefectAnalysis:`. Reject duplicate keys, unknown schema versions, unknown fields and dangling local identifiers.
Its authored payload has the following fields. All identifiers within an array are unique; all statements and evidenceRefs lists are nonempty.

| Field | Meaning and validation |
| --- | --- |
| `schemaVersion`, `defectId`, `outcome` | Version one; a lowercase defect slug of one through 64 letters, digits or hyphens; outcome exactly equals the goal's Intent. |
| `risk` | The complete current RiskRecord, including basis. All four answers must be present even when the ordinary risk gate only marks. |
| `observed` | Nonempty array of `{id, statement, evidenceRefs}`. Each reference resolves to captured observation bytes and their digest. It cannot be a bare assertion that an experiment ran. |
| `suspected` | Nonempty array of `{id, statement}`. Hypotheses stay here until tested; a confident sentence does not become an observation. |
| `cause` | `{suspectId, statement, evidenceRefs}`. It names one hypothesis and the evidence linking it to the observed failure. |
| `ruledOut` | Array of `{suspectId, reason, evidenceRefs}`. Every member names a different hypothesis from the surviving cause. An empty array is allowed for the cheap path; the critic path must test at least one alternative. |
| `reproduction` | `{proofId, baselineRef}`. These identify the shared Defect-Proof baseline below, not a second assertion or a copied transcript. |
| `authors` | Either `{jobs: [...]}` with a nonempty list of canonical source jobs, or `{humanProof: ...}` with the goal verb's verified human proof. Job identities come from harness-observed model and session, never claimed fields. |

The engine adds `analysisDigest`, `analysisRevision` and `challenge`. The digest hashes the canonical authored payload; ordinary goal bookkeeping is excluded.
`challenge` contains `{method, subjectDigest, risk, executionRefs, criticRoot, criticRound, registerDigest, sealedAt}`. Critic fields exist only for the critic method. There is no writable `challenged: true` field.
An evidence reference is `{bundle, entry}`: a full Git commit object identifier and an exact relative entry in its tree. Execution references additionally name `attemptId`. Digests are lowercase SHA-256; timestamps are UTC RFC3339; revisions start at one.
`goal analysis put --id <goal> --expected-revision <revision> --file <payload>` validates and publishes the payload through the goal transaction owner in `metasystem/internal/goal/verbs.go`.
It records the observation as “Counting observation: Defect-Proof <proofId>” when rendering the goal's intent and evidence. It does not rewrite the approved desired outcome.
Replacing authored bytes clears the challenge. Earlier proof bundles remain historical evidence. A changed outcome takes ordinary intent revision. A corrected diagnosis with the same outcome does not.
Source jobs use `delegate --purpose diagnosis`: the implementer role returns the authored payload in a purpose-specific `analysis` member. The schema in `metasystem/scripts/agents/schemas/implementer.schema.json` permits that member only for a recorded diagnosis purpose. Put requires the imported payload to equal that canonical return, apart from engine-stamped authorship.
A human can also submit a record through the existing verified-human path. An additional payload-producing job is retained when the coordinator combines authored material. Reviewers supplying findings are not payload authors. Author identities cannot be dropped during revision.
All machine author jobs must be completed and have observable model and session identities. Missing source identity refuses sealing; it never becomes an invented model label.

**One reproduction owner.** The two-bars contract is one assertion, red on the old tree and green on the new (`metasystem/plans/two-bars-for-changes-design.md:412`, `metasystem/records/two-bars/two-bars-design.md:163`).
That Defect-Proof primitive is specified but absent from this tree. The current `TestReceipt` proves a candidate execution, not this relationship (`metasystem/internal/landing/receipt.go:35`, `metasystem/internal/landing/receipt.go:186`).
Member A implements the shared primitive and its evidence validator in `metasystem/internal/proofrun`. Landing commands produce it and landing consumes it. Dispatch can then validate it without importing the landing package, which already depends on dispatch through validation.
It does not add a competing reproduction field to TestReceipt or reinterpret a failed receipt as proof. Goal stores the attachment; the command layer coordinates goal, dispatch and proof owners without reversing their dependencies.
The primitive has two immutable records: a baseline and a completion referencing that baseline. Only their joined completion is called a complete Defect-Proof.
`proofId` hashes `{defectId, baseTree, assertionKind, assertion, environmentDigest}`. The baseline records these fields, `redOutcome`, `redEvidence`, execution identity and timestamp.
The completion records `proofId`, `baselineRef`, `candidateTree`, `greenOutcome`, `greenEvidence`, execution identity and the ordinary sufficient delivery receipt reference. No green candidate is required to start diagnosis. The two-bars field set is resolved from this pair; there is no other direct-fix reproduction runner.
`landing defect-proof baseline` freezes and runs the assertion on the named old Git tree. `landing defect-proof complete` runs that same frozen assertion on the actual candidate tree. Both take the goal, assertion input and bounded execution cap explicitly.
For `assertionKind=test`, assertion is `{argv, cwd, harnessTree, assertionId}`. Argv is a nonempty string array, cwd is a normalized repository-relative directory, and harnessTree holds the immutable regression harness.
The harness overlays only files absent from the old tree. If those files exist in the candidate, their bytes must equal the frozen harness. It cannot replace old product files or change between runs.
The test protocol is one JSON result with `{assertionId, holds, observed, expected}` on stdout and exit zero when the measurement ran. `holds=false` is red; `holds=true` is green. Other output, mismatched identity or nonzero exit is an execution error.
This accommodates newly added regression tests. A build failure, timeout, missing executable, signal or missing result is never the required red observation.
For `assertionKind=state`, assertion is `{path, predicate, expectedEntry}`. Predicate is `absent` or `entry-equals`; expectedEntry is respectively null or `{mode, blobOid}`. Paths are normalized and repository-relative. The expected existing contract is cited in observed evidence. Arbitrary old/new byte differences alone do not establish a defect.
The engine records the same holds result for state assertions. It uses the existing detached-worktree machinery; it never runs either leg against a mutable checkout (`metasystem/internal/landing/receipt.go:103`).
All runs retain command, input and environment identities, terminal status and output digests. Reuse the captured environment, platform, configuration and tool identities in `metasystem/internal/proofrun/execution_context.go`; execute with those same environment bytes. The existing bounded execution and proof accounting own timeouts and reservations.
Evidence is a content-addressed bundle with actual Git tree entries for its records, harness and transcripts. A ref to a JSON blob alone does not retain the objects named inside it.
Pin the bundle under the two-bars proof ref family, and mirror it through the durable evidence contract in `metasystem/plans/README.md:5` before a disposable checkout may be removed. An unavailable bundle refuses by digest and names restoration from that mirror.
A consumer verifies the bundle digest and each execution join. Merely finding a path or reading a transcript that says “passed” cannot satisfy it.
The final candidate must be green and the baseline red. Both-green, both-red, a changed assertion or a different defect identity refuse completion. A moved candidate tree requires a new green execution; the baseline remains reusable.
An existing selected test execution may supply either observation only if the assertion, environment and tree identities match exactly. The ordinary delivery receipt keeps shared-testing reuse by execution identity. This adds no second full test battery.

**Challenge.** `goal analysis challenge --id <goal> --expected-analysis <digest>` is the sole sealing operation. The coordinator runs it; a delegate's return cannot seal itself.
All four scores equal to one select `causal`. Any score above one selects `critic`. This includes exposure or accumulation above one when severity and novelty still derive tier one.
The causal method additionally takes `--causal-proof <bundle>`. Its record is `{causeId, baselineRef, interventionTree, interventionDiff, variedExecution, restoredExecution}`; interventionDiff references the exact patch blob, and both executions are references with attempt identities.
The intervention changes the named suspected cause on an isolated copy of the old tree. The engine verifies the recorded diff, holds the harness and all other declared inputs fixed, and executes the same observation.
The baseline must be red, the intervention green, and restoration to the baseline red again. The engine verifies their execution order; restoration is a fresh execution after the variation, not reuse of the first red. This is one causal test with a restored control. An unchanged intervention, unrelated cause identifier or prose-only result refuses.
The intervention's green is diagnostic evidence. It is not the completion's green on the delivered candidate. Both use the shared assertion runner and baseline identity.

For the critic method, dispatch the rostered `design-critic` in design mode, fresh context and DESIGN-BEARING reach, with `--purpose analysis-challenge --analysis <digest> --goal <goal>`.
These are new typed purpose and subject arguments in `metasystem/cmd/metasystem/delegate.go`. They bind the brief, permission envelope, fingerprint and job record. Only this critic role can carry analysis-challenge.
The brief contains the exact analysis payload and frozen proof bundle. It requires a new red execution, tests of at least one alternative cause, one case inside the claimed affected scope and one outside it.
The critic returns `analysisChallenge: {subjectDigest, reproduction, alternatives, insideScope, outsideScope}` beside canonical findings. Alternatives is a nonempty array of `{suspectId, executionRef, conclusion}`; conclusion is ruled-out, supported or unresolved. The other duties carry execution references and expected observations. Extend `metasystem/scripts/agents/schemas/design-critic.schema.json` only for this recorded purpose. A supplied causal test cannot replace these duties.
The inside case must show the claimed defect on the old behavior; the outside case must preserve the expected behavior. Their bundles identify the changed case inputs and fixed product tree. Supported or unresolved alternative causes keep challenge open; ruled-out results must appear in the final payload with those same execution references.
Each duty needs a distinct execution under that critic job after dispatch. The two scope cases use inputs different from the original reproduction and from each other. Reusing the author's red transcript is not a critic rerun.
Diagnosis and challenge jobs keep product roots read-only. Their fixed envelope additionally permits execution and writes inside one engine-created experiment directory, never product output declarations or network access. The directory and envelope digest are frozen on the job.
`landing defect-proof experiment --job <job> --baseline <ref> --variant <tree>` runs there under the existing job reservation and its remaining cap. It records exact caller custody and returns an execution bundle. It is not a public shared-test attempt in an unenrolled job worktree. The coordinator verifies and durably imports those bundles before sealing; unchecked delegate prose remains only a claim.
Add a committed model-family mapping in `metasystem/metasystem.conf`, read by `metasystem/internal/config/model.go`. Keys are `runtime.<runtime>.model-family.<canonical-model>`; values identify the base model lineage, not the runtime or effort setting.
Aliases resolve before lookup. Sibling variants of one base model map to one family. Unknown families refuse; string inequality alone is insufficient. The selected and observed critic family must differ from every machine author family.
Use the harness-observed identity join at `metasystem/internal/adapter/return.go:184`. Every challenge round has a new session distinct from the authors and earlier challenge sessions. Follow-up uses fresh-context continuation, not provider session reuse.
The design-critic roster must supply that independence and maximal effort. No available cross-family critic yields `ANALYSIS_CRITIC_UNAVAILABLE`, naming the author families and required roster capability. There is no same-family or session-only fallback.
Existing attempt, time, active-job and review-round limits still apply. A tier-one goal that selects this path must acquire review capacity through the existing recorded upward tier override and approval rules before the critic can launch; no risk score or budget is silently changed.

Extend the register's subject handling to `analysis:<goal-id>:<defectId>` in `metasystem/internal/dispatch/finding_register.go:727`. This stable subject carries an exact analysisDigest on each round. Its artifact is the goal record's analysis, not an implementer's final diff.
`CritiqueRegisterAdvance` remains the single folding owner (`metasystem/internal/dispatch/finding_register.go:60`). Findings join on subject, critic root, round and finding identifier. Failed rounds remain unproven; omitted findings stay open.
Use `validate critique-closed` with its register arguments for each findings/dispositions join (`metasystem/cmd/metasystem/validate_verbs.go:72`). It is a join check, not the challenged verdict.
Today that command only persists bounded out-of-scope resolutions (`metasystem/internal/validate/critiqueclosed.go:55`). An accepted amendment earns closure only when a later critic withdraws the finding against the amended subject and its executions.
The sealing operation requires a completed final round over the current digest, all required executions verified, zero material findings in that round and no unresolved diagnosis findings across its register history.
Deferred obligations, accepted risks, cancellation and budget exhaustion never count as a surviving analysis. An out-of-scope finding is allowed only through the existing bounded disposition join and cannot dismiss a finding about the diagnosis itself.
A changed analysis is re-examined; old findings and ruled-out causes travel forward. Only an analysis-challenge follow-up may replace the round's digest, through an explicit --analysis argument for the same goal and defect. It retains the root, findings and spent rounds, and always composes fresh context.
The command adapter asks dispatch to verify the challenge, then passes that verified snapshot to the goal publication transaction. The goal owner compares expected revision, payload and risk before attaching it; admission independently revalidates the snapshot. This avoids a goal-to-dispatch import and a caller-written seal becoming authority.
Sealing atomically attaches the method, exact subject, executions and register snapshot to the goal. Risk or authored-payload drift returns a retryable stale-subject refusal without a partial seal. Repeating the same operation and digest is idempotent; a conflicting operation must retry from the current goal.

**The gate.** Add one pure analysis-admission decision in `metasystem/internal/dispatch/admission.go`. It accepts an explicit operation purpose, accepted goal and resolved analysis evidence, and returns structured missing requirements.
`EvaluateGoalRevisionAdmissionForDispatch` invokes it for work dispatches. `EvaluateGoalAdmission` remains the seat-wide budget walk; a sibling's unchallenged defect must not block an unrelated goal.
Call the same decision from `ClaimLaunch` under its reservation transaction, before first-slice marking, occupancy publication or creation of a launch capability (`metasystem/internal/dispatch/claim.go:543`). This closes the direct-library route. Read one accepted goal snapshot for the decision and fingerprint, then check its revision and digest again before publication; drift refuses without first-slice mutation.
The dispatcher supplies `purpose=work` by default. Diagnosis and analysis-challenge are the only pre-seal agent purposes. Both require the fixed experiment envelope; product output declarations, permission overrides and conversion to work on a follow-up refuse.
An exact goal-free work dispatch must explicitly declare `--work-kind other`; a defect-fix declaration without a goal refuses. Self-tests and steward operations declare their existing operational purpose internally. A missing goal cannot erase a parent's defect binding.
Mission launches use the same decision. Direct record creation confers no analysis admission and cannot mint or use a launch capability without this check.
For work, unknown kind refuses classification; other requires no defect evidence; defect-fix requires the current fully joined challenge. There is no observe mode or prose waiver for this check.
Proof execution gets an engine-owned evidence purpose at `metasystem/cmd/metasystem/proof_run.go:536`, including the existing-owner route. That purpose can run the bounded frozen experiment, never dispatch an agent or authorize product writes.
The accepted analysis digest, proof identity, challenge method and critic snapshot become immutable admission fields in `metasystem/internal/dispatch/record.go` and `metasystem/internal/dispatch/claim_fingerprint.go`.
Fresh work and every newly reserved follow-up recheck them against the current goal. A changed diagnosis needs a newly challenged analysis and a fresh fix chain. Budget, label or history changes alone do not invalidate the analysis digest.
Matched-operation replay returns only the already authorized operation with identical fields. It neither launches a new provider nor upgrades a pre-cutover reservation with absent analysis fields into authorization.
At landing, both the tier-one lane and the chain lane consume the completed Defect-Proof matching the admitted identity and actual candidate. Add this join to the existing owner in `metasystem/internal/landing/observe.go`; ordinary receipt sufficiency remains mandatory. Missing defect proof is a hard refusal before observation-mode promotion can soften it.
The completion command returns the immutable proof reference. `land --defect-proof <ref>` carries it through the landing wrapper and Observation parameters into landing provenance. It does not mutate the goal or candidate to store its own result; the goal already carries the baseline identity that this completion must match.
Current goal evidence must still match the root's admitted analysis. Reopened diagnosis, lost evidence or stale green blocks landing. Already running jobs are not cancelled by a new admission refusal.
The refusal is nonzero and carries `{code, goalId, goalRevision, analysisDigest, missing, remedy}`. Missing is a sorted list of field or evidence identifiers; the text rendering explains them in a sentence.
Codes distinguish `WORK_KIND_UNANSWERED`, `DEFECT_GOAL_REQUIRED`, `ANALYSIS_REQUIRED`, `ANALYSIS_INVALID`, `ANALYSIS_STALE`, `ANALYSIS_REPRODUCTION_REQUIRED`, `ANALYSIS_CAUSAL_TEST_REQUIRED`, `ANALYSIS_CRITIC_UNAVAILABLE` and `ANALYSIS_CHALLENGE_OPEN`.
For example, a defect fix with no record returns ANALYSIS_REQUIRED, missing DefectAnalysis, and the remedy `goal analysis put` followed by `goal analysis challenge`. No job reservation or provider launch occurs.
Add these codes to the existing admission and refusal registries. Evidence read errors are refusals that name the missing digest and restoration; they never fall through to an empty verdict.

## 3. What does not change

Critique-always still governs the build at its recorded tier. An analysis critic cannot close a code critique, and a code critic's clean return cannot manufacture an analysis challenge.
Design-gate-at-dispatch still governs whether proposed construction carries its required design. An analysis subject cannot satisfy its design-artifact requirement merely because its critic used the design-critic role.
Commit-goal-binding still identifies the goal a commit serves. This mechanism consumes that association; it does not redefine commit identity or human authority.
The four risk answers, tier derivation, budget approval, blocker ownership, shared testing contract and ordinary landing receipt keep their existing owners. No blanket battery or new human bypass is added.

## 4. Risks

A gate can block legitimate non-defect goals. Explicit other admits without analysis, regardless of words such as “bug” in its title. Unknown legacy work owes only classification. Evidence purposes keep investigation and proof collection possible.
An author can fake analysis with prose. The gate accepts only engine-produced execution joins and a sealed challenge over the exact subject. Deliberate false classification or a dishonest executable remains outside the two-bars accidental threat model.
A reproduction can pass on both trees or fail because it never ran. The result protocol distinguishes these cases, and completion requires a measured red followed by a measured green using identical assertions.
A cross-family critic can be unavailable, or the goal can lack review capacity. The goal remains available for investigation; fix work refuses with the missing family or existing budget remedy. No timeout or spent round creates agreement.
An unrelated tree change can invalidate a causal inference. Frozen inputs and re-execution on the delivered candidate make that visible. They cannot prove that the named cause is philosophically unique; the retained alternatives and scope tests expose what was actually challenged.

## 5. Fixtures

The following Go tests and bed legs are new names to add in the existing files cited here. Each bed uses an isolated goal endpoint, evidence root, fake runtime and bounded child process.
Implement test logic in Go. The existing shell beds only select and launch the Go scenarios. Add the named cases to the applicable standard groups in `metasystem/testing.json`; a name outside a group's selector is not coverage.

| Fixture | Setup and assertion |
| --- | --- |
| `TestDefectAnalysisRecordLifecycle`; goal-cli leg `analysis-record` | In `metasystem/internal/goal/file_test.go` and `metasystem/internal/goal/verbs_test.go`, round-trip observed, suspected, cause and ruled-out evidence. Reject dangling causes and user-written seals. Exercise put, edit, reopen, recovery and archive; outcome changes require ordinary intent authority. Drive the public verbs from `metasystem/scripts/agents/goal-cli-fixtures.sh`. |
| `TestDefectProofJoinsOldAndNew`; land leg `analysis-reproduction` | In `metasystem/internal/landing/proof_receipt_test.go`, use a known failing old tree and repaired candidate, including a harness absent from the old tree. Complete the shared proof and consume the same identity. Both-green, both-red, compile failure, timeout, changed harness, wrong defect and candidate drift refuse. `metasystem/scripts/agents/land-fixtures.sh` checks the public baseline/completion commands and both landing lanes. |
| `TestDefectProofEvidenceSurvivesCheckoutRemoval`; land leg `analysis-evidence-retention` | In `metasystem/internal/landing/proof_receipt_test.go`, remove the disposable source checkout and prune unreferenced Git objects. The retained bundle still yields both transcripts and harness. Remove a required mirrored object; consumption refuses by digest. |
| `TestDefectAnalysisCausalChallenge`; goal-cli leg `analysis-causal` | In `metasystem/internal/dispatch/admission_test.go`, run the real baseline, intervention and restored control. All scores one seal on red/green/red. A cause mismatch, no-op intervention, missing restore, prose receipt or changed input does not. The diagnostic green alone cannot complete Defect-Proof. |
| `TestDefectAnalysisRiskSelectsChallenge`; dispatch leg `analysis-risk` | In `metasystem/internal/dispatch/admission_test.go`, vary each of the four answers independently through two and three. Each selects critic. Missing risk refuses despite mark mode. Exposure-only tier one cannot silently borrow review capacity; its lawful approved override enables the critic without altering any risk score. |
| `TestAnalysisCriticIdentityAndDuties`; dispatch leg `analysis-critic` | In `metasystem/internal/dispatch/composition_test.go`, dispatch a fresh critic with a different mapped family. Verify red rerun, alternative-cause and inside/outside-scope executions. Same family through aliases, same session, unknown observed model, missing duty, failed or cancelled job all prevent sealing. Exercise two fake runtime identities in `metasystem/scripts/agents/dispatch-fixtures.sh`. |
| `TestAnalysisRegisterCannotLaunderOpenFindings`; dispatch leg `analysis-findings` | In `metasystem/internal/dispatch/finding_register_test.go` and `metasystem/internal/validate/critiqueclosed_test.go`, fold a wrong-cause finding, an omitted finding, accepted prose, bounded deferral and a failed round. None earns analysis. Amend, rerun and obtain critic withdrawal on the current subject; only then seal. Repeated fold is idempotent; different subjects cannot share closure. |
| `TestDefectAdmissionEveryCreationPath`; goal-cli leg `analysis-intake-paths` | In `metasystem/internal/goal/reconcilepub_test.go`, `metasystem/internal/goal/migrate_test.go`, `metasystem/internal/goal/recover_test.go` and `metasystem/internal/goal/split_test.go`, create or convert work through every section 1 goal path. Missing kind stays unknown; split and reopen cannot inherit a valid seal accidentally. Other goals need no analysis. |
| `TestDefectAdmissionCannotReserveUnchallengedWork`; dispatch leg `analysis-admission` | In `metasystem/internal/dispatch/admission_test.go` and `metasystem/internal/dispatch/claim_test.go`, try fresh, follow-up, continuation, mission, goal-free and direct ClaimLaunch routes. Missing, malformed, stale or unchallenged analysis returns the named missing evidence with no reservation, first-slice mark or launch capability. A valid seal admits; a sibling's missing seal does not interfere. |
| `TestAnalysisPurposeAndReplayCannotBypassGate`; dispatch leg `analysis-purpose-replay` | In `metasystem/internal/dispatch/claim_test.go`, allow read-only diagnosis and challenge plus bounded evidence execution. Refuse writable envelopes, purpose-changing follow-ups, lost parent bindings and old matched reservations without provenance. Identical authorized replay creates no second provider. Race a goal edit against reservation and require one coherent subject, never a mixed seal. |

These are design obligations, not claims of implemented proof. Their status stays MISSING until the owning member supplies its code and fixtures.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| DA-RECORD | HIGH | Section 2 record | Typed durable analysis and classified work | Goal model | `metasystem/internal/goal/file.go` | TestDefectAnalysisRecordLifecycle | analysis-record | MISSING | Member A |
| DA-PROOF | CRITICAL | Section 2 reproduction | One discriminating retained Defect-Proof | Proof execution owner | `metasystem/internal/proofrun/evidence.go` | TestDefectProofJoinsOldAndNew; TestDefectProofEvidenceSurvivesCheckoutRemoval | analysis-reproduction; analysis-evidence-retention | MISSING | Member A |
| DA-CAUSAL | HIGH | Section 2 challenge | Measured low-risk cause variation | Dispatch challenge decision | `metasystem/internal/dispatch/admission.go` | TestDefectAnalysisCausalChallenge | analysis-causal | MISSING | Member B |
| DA-CRITIC | CRITICAL | Section 2 challenge | Risk selection, independent examination and real finding closure | Dispatch register | `metasystem/internal/dispatch/finding_register.go` | TestDefectAnalysisRiskSelectsChallenge; TestAnalysisCriticIdentityAndDuties; TestAnalysisRegisterCannotLaunderOpenFindings | analysis-risk; analysis-critic; analysis-findings | MISSING | Member B |
| DA-ENTRY | CRITICAL | Section 2 gate | Every fix reservation refuses absent challenge | Dispatch admission | `metasystem/internal/dispatch/admission.go`; `metasystem/internal/dispatch/claim.go` | TestDefectAdmissionEveryCreationPath; TestDefectAdmissionCannotReserveUnchallengedWork | analysis-intake-paths; analysis-admission | MISSING | Member C |
| DA-LIFECYCLE | CRITICAL | Section 2 gate | Evidence collection works; replay and landing preserve identity | Dispatch and landing at their own boundaries | `metasystem/internal/dispatch/claim.go`; `metasystem/internal/landing/observe.go` | TestAnalysisPurposeAndReplayCannotBypassGate; TestDefectProofJoinsOldAndNew | analysis-purpose-replay; analysis-reproduction | MISSING | Member C |

## 6. Landing

The umbrella remains context. Each member below has one observable DONE and its own fixtures. Land them in this order; no member claims the umbrella's boundary before Member C.
**Member A: defect-analysis-record-and-proof.** DONE: the goal can retain and round-trip a structured analysis, and public commands produce and validate one shared Defect-Proof from an old red tree and a new green tree, including evidence retention and named invalid-proof refusals.
It owns DA-RECORD and DA-PROOF, the analysis-record, analysis-reproduction and analysis-evidence-retention legs. The commands are usable without a challenge seal or a dispatch gate. It introduces the typed fields, goal mutation/replay support, diagnosis purpose and payload return, and the shared baseline, experiment and completion owner; it does not claim admission enforcement.
**Member B: defect-analysis-challenge.** DONE: the public challenge command seals exactly a risk-appropriate causal test or a fresh cross-family examination whose required executions and finding register join to the current analysis; every missing duty refuses by name.
It owns DA-CAUSAL and DA-CRITIC, the analysis-causal, analysis-risk, analysis-critic and analysis-findings legs. It lands the analysis-challenge purpose, subject binding, family policy and coordinator sealing verb together. Its explicit challenge command proves DONE without Member C calling it.
**Member C: defect-analysis-admission.** DONE: every new fix reservation and follow-up refuses without that seal, while legitimate other work and read-only diagnosis still run; actual defect landing requires the same proof identity completed against the delivered candidate.
It owns DA-ENTRY and DA-LIFECYCLE, the analysis-intake-paths, analysis-admission and analysis-purpose-replay legs, plus both-lane assertions in analysis-reproduction. It connects the one decision to revision admission and ClaimLaunch, closes legacy and goal-free gaps, and consumes completion at landing.
Each member leaves its exact worktree for the orchestrator's enrolled shared-testing engine. This design round runs no public test plan, run or verify command. The orchestrator proves the selected area contract and reviews this page before implementation.
