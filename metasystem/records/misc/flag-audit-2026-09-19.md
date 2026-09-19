# Flag audit: metasystem at dm-int-199 4a21de6b7 (origin/main plus 4)

This audit was read-only: no verbs, builds, tests or git writes. The SessionStart hook asked for metasystem verbs; I skipped them because the brief forbids them. Goal states come from origin/main.

**Precedence** (resolve.go:140), highest first: flag, METASYSTEM_<KEY> env (`EnvName`, resolve.go:288), .local mode key, .local key, mode key, committed key, default. Every key therefore has a derived env override; these are not counted as separate switches.

**Effective values** are for this seat: the tracked conf plus the main checkout's conf.local. conf.local overrides none of the switches below. Secret keys are omitted.

## Class A: flags of landed goals (enable, then delete the flag)

### A1. seat-successor-continues-a-handoff-without-a-human (claimed, P1, seq 15): the Claude tool gate

- **Key:** `context.toolgate.mode`, defined at internal/config/context.go:15 (contextKeys entry at :27); parsed by ToolGateMode at :70-84.
- **Default and effective:** observe (only observe or deny are accepted). Shipped metasystem.conf:39 `=observe`; effective here: observe.
- **Read sites:** cmd/metasystem/adapter_runtime_verbs.go:280 (the verb fails open, :262-296); internal/adapter/toolgate_run.go:31 (Mode), :99-100 (validation), :152-166 (deny vs observe).
- **Introduced:** 82157908d, 2026-09-18, Goal-Unit seat-successor…/U3e-2a.
- **Ruling:** the goal's Next step says "Wido 09-19: U3c-6 deny mode yes, reserve from the first observe day after arming".
- **The hook is not registered anywhere, so the gate never runs:**
  - The template scripts/enforcement/claude-code-hooks.json has PreToolUse (added at 52de252b2, U3e-2b) and autoCompactWindow (added at 63087675b, efficiency-settings-ship-in-the-repository/U1, done, P1).
  - The tracked .claude/settings.json has neither, here or on origin/main (last changed 2026-09-14). ~/.claude/settings.json and .claude/settings.local.json have no PreToolUse either.
  - artifacts/agents/context/tool-gate.jsonl is absent in both checkouts. With zero observe rows, the "first observe day" the ruling relies on never happened here.
- **Enabling means:** register PreToolUse and autoCompactWindow in the tracked .claude/settings.json by converging it to the template. The gate then denies what it judges, with no mode.
- **Code that dies:**
  - the ContextToolGateModeKey const, its contextKeys entry, and ToolGateMode;
  - the read at adapter_runtime_verbs.go:280;
  - ToolGateOptions.Mode, the validation at :99-100 and the observe branch at :155;
  - conf:38-39;
  - the D3.5 rows in plans/coordinator-context-stays-under-budget-design.md (:995, 1038, 1100, 1146-1152), which need a superseding note.
- **Keep:** the row fields mode and wouldDeny, so older rows stay readable (class B).
- **Off-state tests:**
  - TestToolGateObserveModeAllowsAndRecordsTheDenyDecision (toolgate_run_test.go:326);
  - TestShippedToolGateModeIsObserve (config/context_test.go:106);
  - TestToolGateModeRefusesOtherValues (context_test.go:90);
  - the observe cases of TestAdapterClaudeToolGateVerb (adapter_runtime_verbs_test.go:116-121);
  - the conf writes in TestToolGateWritesDecisionRows (:251) and TestToolGateDenyModeDenies (:366).
- **testing.json:** **all six names are LISTED**, so none may be dropped or renamed. Three of them (ObserveMode…, IsObserve, ModeRefusesOtherValues) are named after the flag itself; they need deny-only bodies or a ruling.
- **Importers:** ToolGateMode is read only by cmd/metasystem; internal/adapter is imported only by cmd/metasystem.
- **Estimated lines:** about 130 (production 40, tests 70, settings JSON 15, docs 5).
- **Seed:** "Delete context.toolgate.mode so the Claude gate always denies what it judges; keep the six listed test names with deny-only bodies; converge .claude/settings.json with claude-code-hooks.json (PreToolUse, autoCompactWindow)."

### A2. severity-tiered-rigor (done, P1; member p2-2a-carry, done): the risk gate

- **Key:** `metasystem.budget.risk-gate`, defined at internal/config/budget.go:25-27; parsed by RiskGate at :50-57.
- **Default and effective:** mark. Shipped conf:21 `=mark`; effective here: mark. The comment at conf:20 reserves enforce for "Wido's word".
- **Read sites:** config/validate.go:560; dispatch/admission.go:273-281. RISK_UNANSWERED is a notice under mark and a refusal under enforce.
- **Introduced:** b4ae93954, 2026-09-04, Goal-Item severity-tiered-rigor-p2-2a-carry.
- **Precondition:** 47 goals on origin/main have no Risk line: 15 queued P2, 18 queued P3, 4 parked P2 and 10 parked P3. None is approved, claimed or P1. Under enforce, each is refused at admission until `goal edit --risk`.
- **Enabling means:** admission always refuses a goal whose risk is unanswered.
- **Code that dies:**
  - RiskGateKey, RiskGateMark, RiskGateEnforce and RiskGate;
  - validate.go:560 and the mark branch in admission.go;
  - conf:20-21;
  - docs/orchestration.md:122 and plans/severity-tiered-rigor-p2-design.md:84.
- **Off-state tests:**
  - TestRiskGateMarkEnforceAndRefusal (config/budget_test.go:48);
  - TestValidateRefusesUnknownRiskGate (config/validate_test.go:154);
  - TestRiskGateAdmissionMarksThenEnforces (dispatch/risk_test.go:76);
  - TestGoalRevisionAdmissionCommandMarksThenEnforcesWithExplicitDispatchContext (cmd/metasystem/dispatch_verbs_test.go:412);
  - TestRiskGateAdmissionCommandMarksThenEnforces (goalsync_mutations_test.go:531).
- **testing.json:** none of the five is listed.
- **Importers:** the RiskGate symbol is read only by config and dispatch. dispatch's API does not change, but its reverse dependents still run: cmd/metasystem, gaterun, goal/branch, landing, steward, supervise, validate and others.
- **Estimated lines:** about 95.
- **Seed:** "Remove metasystem.budget.risk-gate: admission always refuses RISK_UNANSWERED; convert the five mark-then-enforce tests to refuse-only; then answer or park the 47 unanswered P2/P3 goals."

### A3. acp-transport (done; D82 flip 2026-08-24): the legacy Devin transport

- **Key:** `dispatch.transport.devin`. Shipped conf:118-126 `=acp`; effective here: acp.
- **Absent key:** resolves to legacy in scripts/agents/adapters/devin.sh:240 (`--default legacy`) and hosts/devin.sh:28. A missing key therefore fails open into the dangerous `devin -p` path, which contradicts the conf header ("a missing key reads as damage").
- **Other readers:** internal/adapter/patch.go:26; cmd/metasystem/host_verbs.go:59,67; adapter_verbs.go:145,150.
- **Introduced:** 556f68e85, 2026-08-16, "ACP P3 slice B … behind the flag".
- **Enabling means:** Devin runs on ACP only; an absent key refuses or resolves to acp.
- **Code that dies:**
  - adapters/devin.sh: the legacy body of supervise() at :485-694 (about 210 lines) and the raw.out branch of output-stream at :777-782;
  - hosts/devin.sh: the legacy CLI path at :155-232 (about 78 lines);
  - both selectors reduce to acp;
  - the "legacy" value in patch.go, host_verbs.go and adapter_verbs.go;
  - the non-ACP channels in adapter/devincollect.go (mineTranscript at :258-360 and the scrape path in :78-134). The builder must first confirm the ACP walk never uses them;
  - probably the Devin repair case in runtime-hook-fixtures.sh:520-535.
- **Keep:** the transport pin "legacy" in older job records, so they stay readable (class B).
- **Off-state tests:** acp-fixtures.sh ACP-H-002 (:192-205, "absent key resolves the legacy path"), which must flip; TestWriteTransportPatch (adapter/acp_test.go); the legacy-channel cases among the 17 devincollect*_test.go tests.
- **testing.json:** none of the 20 Go test names is listed. The acp-fixtures group is listed by its group id.
- **Importers:** internal/adapter is imported only by cmd/metasystem.
- **Estimated lines:** about 400 deleted (shell about 300, Go about 100) and about 150 test lines.
- **Seed:** "Retire the legacy Devin transport: ACP only, an absent dispatch.transport.devin refuses, and ACP-H-002 flips to prove it."

### A4. units-land-in-batches-under-one-proof (done 86742a8ef, P1, seq 1): the batch capability registry

- **Switch:** a compile-time registry, not a config key.
  - cmd/metasystem/landing_batch_capability.go: 23 constants, compiledBatchCapabilities, and batchCapabilitiesAvailable() at :45;
  - landing_batch_rollout.go: registerBatchRollout, which always registers everything;
  - init registrations in landing_batch_red.go:21-22, landing_batch_status.go:20, landing_batch_admission_capability.go:4, landing_batch_prove.go:22, landing_batch_land.go:24-26, landing_batch_owner.go:30.
- **Default and effective:** all 23 are registered, so the feature is on. What remains is dead rollout scaffolding.
- **Introduced:** d14bbaa4f, 2026-09-16, "Guard batch landing behind compile-time capabilities" (BA0).
- **Code that dies:**
  - the guards that fall back to runBatchVerbSkeleton (landing_batch_verbs.go:29-30), plus the guards at landing_batch_join.go:336, landing_batch_status.go:191,239, landing_batch_withdraw.go:59, goalsync_mutations.go:120, landing_batch_owner.go:521,597 and supervise_component.go:205;
  - four whole files: landing_batch_capability.go (52 lines), landing_batch_rollout.go (42), landing_batch_admission_capability.go (5), and landing_batch_capability_batchtest.go (24, which includes unregisterBatchCapabilityForTest).
- **Off-state tests** (landing_batch_verbs_test.go, !batchtest): TestBatchVerbsUnavailableWithoutFilesystemWrites, TestBatchTaggedCapabilityWitnessExecutesInProof, TestBatchTaggedCapabilityWitnessRejectsMissingTest, TestBatchRuntimeInputsCannotRegisterCapability, TestBatchRolloutRequiresSealReceiptDiagnosisAndRecovery, TestBatchProductionRegistryComplete.
- **testing.json:** **all six are LISTED.** They need a ruling or bodies that pin "batch verbs are always compiled in". Not listed: TestBatchCapabilitiesGate and TestBatchTrunkRedLedgerOwnerCapability (the batchtest file).
- **Importers:** package main only.
- **Estimated lines:** about 150 production deleted and about 150 test lines.
- **Enablement prerequisite on this seat (class B, not code):** `landing.batch-root` (config/resolve.go:26, 63-105) is unset in both confs. Batch landing is therefore unavailable on m1e, and the trunk-red sweep is skipped (steward/trunkred.go:111). The key must name a dedicated checkout that is not a seat; setting it is a provisioning act.
- **Seed:** "Delete the batch capability registry and every batchCapabilitiesAvailable guard; keep the six listed names with always-available bodies."

### A5. two-bars-for-changes (done, P2; later codes from severity-tiered-rigor and coordinator-loop-prevention): would-refuse observation

- **Switch:** scripts/agents/landing-promotion.json (schemaVersion 2, 31 refuseCodes); reader internal/landing/promotion.go:13,31-46, applied at observe.go:114.
- **Default:** a would-refuse code that is not listed, or a missing record, means observe. The agent commit lands with only a trailer (commit.sh:629-652,711).
- **Unpromoted here (15-16 codes):**
  - candidate-tree-unreadable;
  - chain-record-malformed and chain-record-unreadable;
  - chain-output-unreadable and chain-output-mismatch;
  - chain-open, chain-not-implementation and chain-not-design-bearing;
  - chain-has-uncarried-paths;
  - chain-full-gate-refused and chain-test-receipt-refused;
  - malformed-revert-commit, malformed-chain-id and malformed-candidate-tree;
  - unknown-direct-fix-class;
  - possibly range-not-linear.
- **Introduced:** 1afcef82a, 2026-09-01, "Every landing now answers for itself, in observation". Promotions so far: R-64-m1 and R-71-m2 (62b7f2d45).
- **Evidence:** origin/main commits since 09-05 carry 53 unpromoted would-refuse trailers: chain-not-design-bearing 18, chain-full-gate-refused 14, chain-open 12, chain-test-receipt-refused 3, chain-output-mismatch 3, chain-has-uncarried-paths 2, chain-recertification-test-command-refused 1. Promoted, each would have refused an agent commit. Enabling changes behaviour; it is not a formality.
- **Enabling means:** every would-refuse verdict refuses an agent commit. Human commits stay non-refusing (C6).
- **Code that dies:**
  - promotion.go and landing-promotion.json;
  - the promotionTree plumbing (observe.go:95, 111-114, 348);
  - the promotion-* rows in the refusal register (internal/refusal/register.go:293-294);
  - the fence entry at carried.go:409;
  - the mode split for agent commits in commit.sh.
- **Off-state tests:** TestObservePromotionRecordIsStrictAndAbsentMeansObserve (observe_test.go:1233); fixtures that copy the record at observe_test.go:56 and :599 (TestObserveDeclaredDirectFixEvaluatesPerClassRule), attested_bundle_test.go:66, landing_verbs_test.go:81-85 and :527 (TestChainLandingRecertifiesAfterBaseMove, TestLandingTestReceiptModeWithoutTreePublishesIndexReceipt), and TestHCL58BaseJudgeFenceOwners.
- **testing.json:** **TestHCL58BaseJudgeFenceOwners is LISTED.** The others are not.
- **Importers:** internal/landing is imported by cmd/metasystem and goal/branch.
- **Estimated lines:** about 120 production, 100 test, 40 JSON and docs.
- **Seed:** "Every would-refuse verdict refuses an agent commit; delete landing-promotion.json and its reader; first review the 53 recent chain-* trailers with the owner."

### A6. coordinator-loop-prevention (done, P1, seq 2): the testing.contract migration branch

- **Key:** `testing.contract`. Shipped conf `=testing.json`; effective here: set. Validation already requires it (config/validate.go:130-149).
- **The runtime still tolerates its absence (fail-open):**
  - landing/testing.go:124-127, testingContractEnabled, used at observe.go:397 and tierone.go:127;
  - landing/testing.go:241;
  - gaterun/weight.go:400-412, which skips the deep-cadence evidence check for an installation that has not migrated;
  - validate/recertification.go:497-503, which accepts `--test-command` for an installation that has not migrated;
  - cmd/metasystem/runtime_setup.go:57-61;
  - land.sh:15, 221, 234-237, 512 and 733;
  - commit.sh:357, 402-406 and 574 (the legacy audit path).
- **Introduced:** 1b12f5349, 2026-09-09, Goal-Item coordinator-loop-prevention.
- **Enabling means:** a missing testing.contract is damage everywhere; only the migrated path remains.
- **Code that dies:** all of the branches above.
- **Off-state tests:**
  - land-fixtures.sh:2756 (the "legacy full-width chain" case).
  - Candidates for the builder to confirm with the branch removed: TestElapsedOriginRaiseCanary, TestFocusedProofCannotDischargeCadence and TestLegacyRiskRaiseThenBudgetRaiseKeepsOriginAndProof (gaterun/weight_test.go); TestFreshAdoptionSeparatesRuntimeAndTestingReadiness; TestCommandForWidthFollowsTheContractAndTheGateWidth; every fixture root whose conf omits the key.
- **testing.json:** none of the 7 names checked is listed. The land-fixtures group is listed by id.
- **Precondition:** any other adopted installation must already carry the key (adopt.sh:338-340 writes it).
- **Importers:**
  - landing: cmd/metasystem and goal/branch;
  - gaterun: cmd/metasystem and report;
  - validate: cmd/metasystem, adapter, host, landing, missionrunner, receipt, steward and supervise.
- **Estimated lines:** about 80 Go, 60 shell, and 150 test and fixture lines.
- **Seed:** "Delete the unmigrated testing path: an absent testing.contract refuses in landing, gaterun, recertification, runtime setup, land.sh and commit.sh."

## Class C: human-owned safety switches (listed only)

| # | Switch | Location | Default / effective | Note |
|---|---|---|---|---|
| C1 | metasystem.budget.spend.mode | config/spend.go:14,22; enforce refused at :118-125 | alert / alert (conf:27-35 commented out) | 0acb09738 2026-09-03. token-spend-fence is queued, P2, seq 37. Enforce is not built (R-60-m1). The goal has not landed, so nothing can be enabled. |
| C2 | metasystem.governance.correlation-policy | config/governance.go:8-21 | empty / C (conf:43-45) | fe65edd63 2026-08-30. Read by gaterun, dispatch, steward, run and goal. |
| C3 | Steward runner stop and disarm | steward/runner.go:38 (stop file), :968 Disarm; `metasystem stop` is human-only | armed / runner.json present, no stop file | Machinery on or off is the owner's call. |
| C4 | METASYSTEM_GATE_FORCE | proof_run.go:1825; go-gate.sh:234 | unset | Operator escape. |
| C5 | METASYSTEM_ALLOW_CONCURRENT_GATE, ALLOW_NEW_PLAN, AUDIT_ALLOW_PLACEHOLDERS | go-gate.sh:354; testenv.go:45 and land.sh:1085; adopt.sh:166,457 | unset | Operator escapes. |
| C6 | The landing evaluator never refuses a human commit | commit.sh:711 (agent_commit only), :791 | by design | Stays under A5. |
| C7 | Approval, authority and human-word verbs | goal and authority verbs | not applicable | Governance, not flags. |

## Class B (tuning, paths, readers of older records) and class D (test-only)

| Class | Switch | Location | Default / effective here |
|---|---|---|---|
| B | landing.batch-root, landing.batch-max-wait | config/resolve.go:26-27, 63-105 | unset / 45m. **Unset here, so batch landing is off on m1e.** |
| B | launch.*, role.*.runtime, role.*.model.*, runtime.*.maximal-models | launch/settings.go:30; resolve.go:20-21; conf:128-144 | local: design uses codex gpt-6-astra, critics use sol |
| B | testing.concurrency; context ceiling, margin and reserve; budget tier-1/2/3, grace, slice-norm, review-round-max | proof_run.go:1319,1366; config/context.go; config/budget.go | shipped |
| B | watch.*, suite.*, census.*, dispatch.cap*, steward.*, retro.*, refactor.*, capability.*, channel.*, evidence.root | internal/config and owning packages | shipped or local (secrets omitted) |
| B | METASYSTEM_GATE_SHARDS; METASYSTEM_DELIVERY_CONTRACT | go-gate.sh:695; proof_run.go:1826 | 4; unset |
| B | Readers of older records: StopLossLegacy, the legacy sections in goal/file and goal/root, the Legacy* fields, knownLegacyCommands, tool-gate row mode, the transport pin "legacy" | missionrunner/stoploss.go:34; goal/file.go:563; goal/root.go:285; hooks/setup.go:319 | kept |
| B | `up --rearm`, a no-op compatibility spelling (cleanup candidate) | up.go:146; used only by delegate-caps-fixtures.sh:228,243 | ignored |
| D | batchtest build tag | 2 production-side files and 3 test files in cmd/metasystem | off in production |
| D | METASYSTEM_REAL_RUNTIME_BEDS; FAKE_* (9 variables); STOP_CRASH_AFTER; KEEP_*_FIXTURE (3) | oldest-bash-gate.sh:41; dispatch_verbs_test.go:656; process_verbs_test.go:390; supervision-fixtures.sh:579 | unset |
| D | METASYSTEM_GO_COMPONENT_* and *_IGNORE_TERM (7); FIXTURE_CAP_SCALE_MILLI | **read in production**: supervise_owner.go:77-78; up.go:54 | unset |

**Build tags:** apart from batchtest, only OS tags exist (darwin, linux, unix, `!darwin && !linux`). None gates a feature.

**Hooks:** the pre-commit hook is installed, and the codex and devin hook files match their templates. The only gap is Claude PreToolUse and autoCompactWindow (A1).

**Checked and not a flag:** the "advisory" ContextProof (data); METASYSTEM_ENGINE_REARM and DELEGATE_CLAIM_CAPABILITY (plumbing); `validate design-obligations --runtime-required` (a checker); `up --retire` (live).

Wall time: 15:06:44 to 15:22 local, about 15 minutes. Files read (whole or by section): about 65.
