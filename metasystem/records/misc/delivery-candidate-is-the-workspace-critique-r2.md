# Critique register: delivery-candidate-is-the-workspace design, read 2

Job dcwl-crit2-20260910 (codex, gpt-5.6-sol, design-critic). Reviewed revision 2 at local scaffold bc164d658 (sha256 dca76908f7538f987844abdec61d699c58fb13d552381395ce2d0d27c75601ae), read at 2fbd77535. Material findings: 6 of 6. The critic's runtime is read-only; the register is projected verbatim from artifacts/agents/dcwl-crit2-20260910/rounds/1/return.json. The job record says failed/protocol_error because the declared output file could not be written; the return is complete.

## DCW-08 (high)

**Claim.** DCW-08 — Section 4.4, observer verification can fail open. Recomputing the verification core in landing observe is the right architectural choice, but the design does not require observation to refuse when that later result is insufficient. The existing command checks Delivery.Sufficient after calling the core; revision 2 instead passes the raw result into readTestReceipt, where the exact and recorded-floor fast paths run before the only stated sufficiency check in the selected-identity branch. The index tree can be compared exactly, but destination or policy state, retained attempts, physical inputs, and engine or environment inputs can differ between the two runs. The fold must require any core error, insufficient delivery result, or candidate-tree mismatch to stop observation before every receipt fast path, and must specify the refusal or command-error mapping and a test where commit.sh's first run is sufficient but observation's run is not.

**Evidence.** Revision 2 section 4.4, especially metasystem/plans/delivery-candidate-is-the-workspace-design.md lines 442–468; metasystem/cmd/metasystem/test.go at 2fbd77535 lines 777–830; metasystem/cmd/metasystem/commit.sh at 2fbd77535 lines 258–278 and 435–468; metasystem/internal/landing/observe.go at 2fbd77535 lines 161–177.

## DCW-09 (high)

**Claim.** DCW-09 — Section 3 site 5 specifies an impossible attempt lifecycle and omits one affected production owner. A non-ledger index movement makes PrepareTestingReceiptPayload return an error from the launcher's success-preparation callback. The launcher converts that error into terminal failure before finalization; exact reuse requires terminal success plus a committed receipt, and component selection then requires retry. The attempt therefore does not finalize as a reusable success without a receipt, and the next landing test-receipt command cannot compose a new receipt with zero execution. The fold must either design an explicit, honest terminal-success-without-receipt outcome across the launcher and command finalization owners or replace the proposed recovery path. It must also include proof_run.go, an omitted production reader of TestResult.CandidateTree and caller of receipt preparation, and add lifecycle tests for the chosen outcome.

**Evidence.** Revision 2 section 3 at metasystem/plans/delivery-candidate-is-the-workspace-design.md lines 334–346; metasystem/cmd/metasystem/test.go at 2fbd77535 lines 632–665; metasystem/internal/proofrun/launcher.go at 2fbd77535 lines 441–465; metasystem/cmd/metasystem/proof_run.go at 2fbd77535 lines 168–169 and 632–640; metasystem/internal/proofrun/test_result.go at 2fbd77535 lines 258–264; metasystem/internal/proofrun/attempt.go at 2fbd77535 lines 590–640.

## DCW-10 (high)

**Claim.** DCW-10 — Section 8.2 does not honestly prove the older-engine plus new-land.sh cutover. The proposed shim removes workspace from the receipt and makes landing workspace unknown, but forwards all other operations to the new engine. An actual older engine rejects a receipt containing workspace because its decoder disallows unknown fields; the shim would let the new reader accept it. This can conceal accidental exposure of a new receipt to an old observer. The fold must use the actual older engine or make the shim reproduce strict old receipt decoding, and the leg must prove old-receipt exact success, moved-index exact refusal, and rejection of a workspace-bearing receipt.

**Evidence.** Revision 2 section 8.2 at metasystem/plans/delivery-candidate-is-the-workspace-design.md lines 686–696; metasystem/internal/landing/receipt.go at 2fbd77535 lines 458–462.

## DCW-11 (medium)

**Claim.** DCW-11 — Section 8.3 makes the shared per-scenario ceiling unusable for five existing direct callers. The proposed harness calls harness_fixture_scaled_cap, which refuses when METASYSTEM_FIXTURE_CAP_SCALE_MILLI has not been initialized. Of the six beds sourcing fixture-bed-scenarios.sh, only mission-fixtures.sh initializes the fixture budget. The others currently work as focused standalone beds but would fail before launching their first scenario. The numerical 120-second base ceiling itself is adequate for the reviewed healthy legs after scaling. The fold must either initialize the shared budget safely and idempotently inside the shared harness or add initialization to every caller and the file inventory, with coverage for both standalone and inherited-scale execution.

**Evidence.** Revision 2 section 8.3 at metasystem/plans/delivery-candidate-is-the-workspace-design.md lines 698–716; metasystem/scripts/agents/fixture-budget.sh lines 431–438; metasystem/scripts/agents/mission-fixtures.sh line 17; the other consumers metasystem/scripts/agents/brain-fixtures.sh, metasystem/scripts/agents/land-fixtures.sh, metasystem/scripts/agents/return-schema-fixtures.sh, metasystem/scripts/agents/second-session-fixtures.sh, and metasystem/scripts/agents/witness-gate-fixtures.sh contain no harness_fixture_budget_init call.

## DCW-12 (high)

**Claim.** DCW-12 — Section 8.3's reused timeout loop does not reap a timed-out scenario's process group. fixture-bed-scenarios.sh starts an ordinary background child, and both its cleanup and fixture-budget.sh's poll-and-kill loop signal only that direct PID. Descendants can survive, retain locks or state, and contaminate subsequent scenarios or beds. The fold must place every scenario in a distinct process group or session, terminate and then kill that group after a grace period, wait for the direct child, apply the same cleanup on parent signals, and add a scenario whose child spawns a termination-ignoring descendant and proves no survivor remains.

**Evidence.** Revision 2 section 8.3 at metasystem/plans/delivery-candidate-is-the-workspace-design.md lines 698–716; metasystem/scripts/agents/fixture-bed-scenarios.sh lines 20–29 and 47–78; metasystem/scripts/agents/fixture-budget.sh lines 359–369.

## DCW-13 (high)

**Claim.** DCW-13 — Section 5's engine identity is not a complete build identity, so the byte-identity guarantee and DCW-03/DCW-04 closure remain unsupported. Both the existing shell and Go implementation hash the output of go version and selected go env values, but not the bytes or authenticated closure of the compiler and toolchain actually selected. Two different compiler executables can report identical values and produce different engine bytes. The proposed fixture varies a records file and go.sum but never varies an unkeyed compiler input while holding the reported identity constant. The fold must bind the selected compiler and toolchain bytes or another authenticated toolchain closure into the identity, or weaken the claimed byte-identity contract and rebuild comparison; it must add the missing same-report/different-output toolchain fixture.

**Evidence.** Revision 2 sections 5 and 8.1 at metasystem/plans/delivery-candidate-is-the-workspace-design.md lines 533–556 and 663–684; metasystem/scripts/agents/go-gate.sh at 2fbd77535 lines 186–190; metasystem/internal/proofrun/execution_context.go at 2fbd77535 lines 91–110; the relevant engine-build environment construction in metasystem/artifacts/agents/dcwl-context/rbce-neighbor.patch lines 95–108.

## Gaps the critic named

- The requested register was not created at metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r2.md because this runtime is read-only; this job return is the register for coordinator projection.
- No test bed was run, as the brief explicitly required a read-only design critique with no bed execution.
- The older-engine behavior was established from the historical strict decoder rather than by executing a historical engine binary.
- The healthy shared-bed ceiling assessment is a static audit of declared waits and control flow, not a timed execution measurement.

## Coordinator dispositions (m1d, 2026-09-10, binding on fold 2)

Every finding was checked against the tree before disposition: launcher.go:445-451 turns a PrepareSuccess error into exit 1 and terminal failure (DCW-09); proof_run.go:160-172 calls PrepareTestingReceiptPayload with TestResult.CandidateTree (DCW-09); harness_fixture_scaled_cap refuses without METASYSTEM_FIXTURE_CAP_SCALE_MILLI and only mission-fixtures.sh among the six sourcing beds initialises it (DCW-11); fixture_bed_parent_cleanup and the calibration loop signal the direct pid only (DCW-12).

- DCW-08: accepted. `landing observe` refuses before any receipt fast path when the verify core errors, is not sufficient, or its candidate tree is not the `--tree`; the refusal and its detail are named; a test where commit.sh's run is sufficient and the observer's is not.
- DCW-09: accepted, by honesty not by new machinery: section 3's site-5 residual (a non-ledger index move under a running battery) is a terminal failure at today's cost, a re-run, as launcher.go makes it; the "success without receipt" lifecycle is withdrawn. proof_run.go joins the readers and the files list.
- DCW-10: accepted. The cutover leg's shim reproduces the old strict decoding (a receipt carrying `workspace` is refused by every verb that reads receipts, `landing workspace` is an unknown verb) or the leg builds the base engine; the leg proves old-receipt exact success, moved-tip exact refusal, and rejection of a field-bearing receipt.
- DCW-11: accepted. The shared harness initialises the fixture budget itself, idempotently, when the scale is unset; standalone and inherited-scale runs are both covered.
- DCW-12: accepted. Each scenario runs in its own process group; on timeout or parent signal the group is TERMed, given a grace, KILLed, and the direct child reaped; one scenario spawns a TERM-ignoring descendant and proves no survivor.
- DCW-13: accepted as a narrowed constraint on the engine-bed chain: the engine identity binds the toolchain closure (the selected `go` executable's bytes and its GOROOT VERSION), or the byte-identity claim is dropped in favour of "same identity, same bytes, proven once per toolchain"; the same-report/different-output case is a unit test of the identity function, not a two-compiler bed.
