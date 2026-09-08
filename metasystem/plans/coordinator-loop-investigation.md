# Coordinator loop prevention

## Contract
- Symptom and impact: ordinary Claude and Codex coordinators kept extending repair and validation work for nearly a day. The last portability source finally landed at 8ed97738, after a green 45-section suite and another normal landing that reran package coverage.
- Reproduction and exact state: origin/main 8ed9773876bc99212b6a19b853c68a27ef6aaeae; the only pre-existing local changes are unrelated receipts and narrator records. R27 full suite own exit 0, 3380.39 seconds. Landing own exit 0. Its complete preservation window took 682.739 seconds; isolated coverage timing was not retained. Canonical engine now runs this landed source at generation 27.
- Success: shared execution machinery prevents repeating unchanged proof obligations without new evidence, propagates actual proof failures without automatic expensive retry, and lets landing consume valid coverage proof without rerunning it. Existing failure, source identity, platform, coverage-floor, review, and approval boundaries remain enforced. Verify the same decisions through all supported runtime entrypoints.
- Non-goals: arbitrary host shell commands outside MetaSystem control, replacing mission stop-loss, a new custodian service, weaker coverage floors, changing paid model defaults, historical proof import, or restarting the already delivered portability work.
- Budget and stop conditions: diagnosis limited to ten minutes per independent read-only investigation. Before implementation, settle the smallest shared owner and a falsifiable stopping invariant. Focused canaries only while changing; final full validation once settled. A failed proof creates a named decision before any subsequent repair cycle; do not conceal an automatic fallback as recovery.
- Cycle budget: 6

## Existing evidence
| Artifact | Fact established | Reliability/limits |
| --- | --- | --- |
| artifacts/host-runtime-setup/recovery-r27-full-validator-result.json | Full validation succeeded with own exit 0 | Historical portability proof; never authority for changed loop-prevention source |
| artifacts/host-runtime-setup/landing-preservation-20260908T050433Z-419397df/state.json | Normal landing succeeded and unrelated records were restored | 682.739 seconds is the whole landing window, not isolated coverage time |
| scripts/agents/commit.sh:249-259; scripts/agents/coverage-delta.sh:200 | Commit unconditionally runs package coverage | Read source and observed actual child processes |
| scripts/agents/witness-gate.sh:128-154 | Clean witness path conflates failed execution and unavailable witness, then can rerun the full gate | Source trace; regression fixture still required |
| internal/validate/stoploss.go; cmd/metasystem/validate_verbs.go | Ordinary stop-loss runs only when explicitly invoked | It does not gate direct proof launch |
| internal/proofrun/launcher.go:66-79 | Shared direct suite launcher owns child creation | Currently lacks durable cross-run obligation identity |
| internal/steward/verdict.go:115-122; internal/steward/delivery.go:297-300 | A live stalled coordinator produces notification; delivery has no automatic remedy | Existing feedback is present; no evidence that ordinary main actions are constrained |
| internal/dispatch/governed.go | Existing governed admission owns budgets, assumptions and prior attempt breakers | Direct proof launch bypasses it; ENGINE equality is insufficient for a full suite |

## Theories
| Id | Theory | Support | Contradiction | Decisive check | Status |
| --- | --- | --- | --- | --- | --- |
| C1 | Missing shared action admission lets ordinary coordinator proof cycles bypass existing stop machinery | Only mission runner automatically applies stop-loss; direct proof launch has no call | Existing goal dispatch budgets do constrain delegate jobs | Trace existing run admission and exact proof inputs before choosing the smallest connection | supported gap; implementation boundary being settled |
| C2 | Proof handoff forces avoidable repeated tests | Commit coverage is unconditional; durable landing receipt lacks measured coverage | Existing live witness already reuses some nested engine proof | Receipt producer to normal consumer must launch zero additional coverage tests for identical relevant inputs | supported |
| C3 | Automatic fallback repeats actual failed proof | Clean witness branch invokes full gate again on any failure | Missing witness alone is not evidence that tests failed | Distinguish pre-execution unavailability from nonzero execution; record child launch count | supported source path; focused reproduction required |

## Do not retry
- Do not rerun the portability full suite; it is green and its source is already remote.
- Do not resurrect a dead controller witness or trust a command string named full battery as coverage proof: the existing landing command begins with --fast.
- Do not claim process liveness or another warning establishes control over coordinator actions.
- Do not claim that removing final commit coverage alone fixes the nearly day-long repair loop.

## Cycles
### Cycle C1
- Mechanism/question: does existing machinery actually admit or reject ordinary coordinator proof cycles?
- Novel decision-relevant fact: direct proof launch bypasses governed admission and ordinary stop-loss; a separate clean witness fallback reruns failed gates.
- Command/artifact/files: read-only source trace in the existing proofrun, dispatch, landing, steward, validate and witness owners; independent reports from closure_route_readonly and shutdown_failures_readonly.
- Contract signal for progress: identify an existing process-launch owner and the exact absent facts; distinguish the late duplicate coverage from the broader loop.
- Budget and stop condition: ten minutes per reader; no tests, edits or paid native role dispatch until a bounded design is ready.
- Result: duplicate coverage alone is insufficient; the shared launch/admission boundary is the larger missing control.
- Classification: falsified-continue
- Checkpoint/revert: portability source remains landed; this record is the only new tracked task artifact so far.
- Next action: bind the new work to its own goal, settle a minimal design around existing proof/admission owners, then independent critique and implementation.

### Cycle C2
- Mechanism/question: does preserving the clean witness gate's executed status remove its automatic duplicate launch while retaining explicit pre-execution fallback?
- Novel decision-relevant fact: the dirty witness path already captures and returns its gate status; the clean path conflates that outcome with optimization availability.
- Command/artifact/files: native implementer brief artifacts/coordinator-loop-prevention/witness-brief.md; scripts/agents/witness-gate.sh and a Go-driven shell boundary regression.
- Contract signal for progress: old source starts two gates after an executed failure; corrected source starts one and returns its exact nonzero status, including with errexit disabled.
- Budget and stop condition: 25-minute implementation target, approved 120-minute native ceiling; focused canaries only; stop on a material specification gap.
- Recoverable checkpoint: landed source 8ed97738 and isolated native worktree; unrelated local records remain untouched.
- Expected classification and next action: contract-improved if the before/after runtime invariant holds; independently critique the correction and continue the shared admission design without a full-suite rerun.
