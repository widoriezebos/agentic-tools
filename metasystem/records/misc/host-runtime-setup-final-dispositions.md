# Host setup final design disposition

Goal: `host-runtime-setup`. Companion to the original design and its revision.
The final design round, `host-setup-design-crit1-r3`, returned one material
finding and no gaps. No further design round is authorized or dispatched.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HOST-D3-001 | accepted | `scripts/agents/adapters/devin.sh` has a separate `runtime_repair_invoke` path. The every-provider-child contract includes it, but the named proof must force that actual secondary launch. | Initialize the canonical job context in the shared adapter supervision setup so secondary children inherit it; explicitly exercise Devin delivery repair in linked and shared workspaces, covering start/receipt/stop/end. |

The critic classified this finding severe because an omitted child can cross
the coordinator boundary. That classification is retained; this is an
instruction to implement and prove the correction, not acceptance of residual
risk or a claim that the design register is closed. Do not certify or land an
implementation that leaves the correction or its proof missing. Independent
code critique must examine the actual repair launch and named regression.

Keep propagation owned by the already job-bound adapter process, after its
record/root are resolved. Children inherit the evidence-location hints on
initial launch, ordinary resume and delivery repair. A hint alone still cannot
suppress a hook; job custody and current ancestry must agree. Do not export
this context from the coordinator or a mission-host launcher.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| HOST-D3-001 | HIGH | Final design critique | Devin repair children receive authenticated delegate context and cannot mutate coordinator state | scripts/agents/adapters/runtime-common.sh; scripts/agents/adapters/devin.sh | scripts/agents/adapters/runtime-common.sh and scripts/agents/adapters/devin.sh | scripts/agents/runtime-hook-fixtures.sh: TestDevinDeliveryRepairHookIsolation | artifacts/host-runtime-setup/recovery-r8-runtime-native.log: actual secondary launch in linked and shared workspaces passes; earlier R6 independent review inspected the repair | DONE | Final cumulative independent review and landing; preserve historical finding |
