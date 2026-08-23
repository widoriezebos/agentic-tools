# Why ACP: the shape, and why Devin needed it first

Captured 2026-08-23 from the bm-2d rep-1 review with Wido, the first
live run of a Devin-hosted mission under the wall. Companion doctrine:
the native-versus-metasystem subagent boundary in
`docs/orchestration.md`; the working design is
`plans/acp-transport-design.md` while it ships.

## Why the ACP shape

The metasystem drives three runtime CLIs today, each through its own
adapter dialect: launch flags, log formats, permission configuration,
cancellation behavior, usage extraction — five surfaces times three
runtimes, drifting independently. The ACP shape (Agent Client
Protocol: the runtime runs as a server; the client owns the session)
collapses that into ONE generic delegate-session capability the core
sees — open, prompt turn, event stream, ask, answer, cancel, usage,
result — with capability DECLARATION instead of runtime names
(`acp-adapter-seam` on the backlog carries this as three slices:
seam interface with byte-identical CLI wrapping, proven-native
conformance, emulator upgrades).

What the shape buys, runtime-independently:

- **Permission decisions at one protocol point the client owns.**
  Tool approval flows through the protocol per call, graded by
  capability class — not through three per-runtime config-filter
  files that can drift apart.
- **Structured event streams** instead of log scraping; **turn-boundary
  asks** instead of park-and-restart; **client-owned cancel**;
  **uniform usage accounting**.
- **Prevention as the front door, the wall as the backstop.** The
  host-implementer wall detects and refuses unattributed product
  bytes at every turn boundary; graded ACP permissions deny the
  writing tools before the bytes exist. Grading must deny by
  CAPABILITY (anything that can write the workspace — an allowed
  shell is a writing tool), and whatever grading ever misses, the
  wall still catches: bm-2d rep 1 proved the backstop holds alone.

## Why ACP for Devin first

Devin's legacy launch runs `--permission-mode dangerous` —
auto-approve every tool — under a standing human waiver, because the
CLI had no channel through which anyone could answer a permission
prompt: a graded mode would block on a confirmation nobody can give
and eat the delegate's return. The waiver is the exposure; retiring
it is the `acp-transport` goal's stated intent. ACP provides exactly
the missing channel: the client answers permission requests, so a
HOST role's workspace-writes become deniable at the tool layer.

bm-2d rep 1 is the evidence in one run. On the legacy transport the
swe-1-7 host — with the discipline rule verbatim in its prompt,
twice — dispatched design critics through the real machinery, then
satisfied "implementer" with Devin's NATIVE subagents working in the
main repo, and hand-patched the result itself. Ninety minutes of
dangerous-mode writing met zero resistance until the wall's boundary
inspection parked the mission over the whole host-authored product
tree. Under graded ACP the write tools are refused at the moment of
use — and because native subagents share their session's tool
surface, denying the session starves them too.

**Recorded assumption to verify before trusting graded mode:** that
Devin routes its native subagents' tool calls through the same ACP
permission flow as the parent session. If they bypass it, the
prevention has a hole; a cheap capability probe (or the next
benchmark repetition) must confirm it.

Claude and Codex never carried the waiver — their adapters already
grade permissions through per-runtime config filters — so for them
the ACP shape's value is the seam's uniformity and the single
permission point, arriving as honest emulators first and native ACP
where proven. Their native subagent features inherit their session's
grading the same way, and the same recorded assumption applies when
their native ACP paths land.

## Standing decisions this rationale rests on

- D61 (the dangerous-mode waiver), D81/D82 (the transport selector
  `dispatch.transport.devin`, flip gated on the sealed benchmark),
  D83 (the Devin acceptance probes), D88 (the human seal boundary),
  D100 (hosts never author product bytes), D117 (the recovery
  ladder), and Wido's 2026-08-23 word: flip on valid green, and
  model-tier naming leniency via a declared, generic equivalence
  mechanism (Devin's `swe-1-7` IS its "SWE-1.7 Max" tier; the
  spelling wobble is a known platform behavior, not substitution).
