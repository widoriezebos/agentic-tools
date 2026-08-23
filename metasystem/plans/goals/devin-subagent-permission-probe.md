# devin-subagent-permission-probe

- State: queued
- Intent: Verify Devin routes native-subagent tool calls through the ACP permission flow; graded mode's prevention is trusted only on proof
- Origin: human
- Next step: Appetite: 2h, approved by Wido 2026-08-23, reserved for m2 (executes after benchmark-evidence-schema-drift and the prompt-authority sentence). The recorded assumption in docs/design/acp-transport-rationale.md: under a graded ACP session with workspace-writes denied, instruct a devin host to use its NATIVE subagent to write a file; PROVE either the permission request surfaces through the protocol (assumption holds — graded mode starves subagents) or the file appears without a request (hole: bank it, the wall remains the only defense, and the flip's prevention claim gets amended honestly). Record the outcome in the rationale doc's assumption paragraph in the same landing.
- OpenedAt: 2026-08-23T18:52:42Z
- Revision: 1

History:
- 2026-08-23T18:52:42Z 7RG04XSFJKFN2QCWYWKRH9X2ZN-m2-bc1be9cb open actor=m2+mac-coordinator targets=devin-subagent-permission-probe
Integrity: sha256=d25459ebe83f1a166972e47be1b86cc9fe746d81d351f642716e67b8ed227d1c
