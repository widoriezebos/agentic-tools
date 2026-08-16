# ACP as the delegate transport (backlog item 18)

- Status: DRAFT r1 — awaiting critique r1
- Goal: acp-transport
- Next step: Fold the critique verdict when run acp-critique-r1 concludes; implement only after convergence.
- In flight right now: run acp-critique-r1 (codex xhigh critique; watch it with: bin/metasystem run watch --id acp-critique-r1 --root .)

The human's question that raised this (2026-08-15, on the Devin
delivery failures): "Is there no way to use ACP to make this more
robust?" There is: the installed CLI ships `devin acp` (an Agent
Client Protocol server over stdio, re-verified at devin 3000.4.25
today). ACP replaces both failure roots the bm-2 arm exposed, and it
is runtime-neutral by construction — Claude Code and Codex-class CLIs
speak it too, so adapters shrink toward launch configs behind one
client.

## What ACP buys, mapped to standing defects

1. **Permissions become policy answers, retiring the D61 waiver.**
   Today a Devin dispatch runs full auto-approve because permission
   prompts dead-stop the CLI (D61's ruled waiver). Under ACP,
   permission requests arrive as JSON-RPC calls
   (`session/request_permission`) that the ADAPTER answers per tool
   call from the job's permission envelope — the graded-permissions
   restoration is this design's acceptance story.
2. **Delivery becomes a typed event, retiring stdout scraping.** The
   D62 delivery ladder (stdout candidate, named file, transcript
   walk) exists because delivery is inferred. ACP ends a turn with a
   typed `session/update` stream and a final response; the ladder
   becomes a live tap that records what the protocol SAYS was
   delivered. The B1 host-recollection capability stays as the
   fallback for non-ACP runtimes.
3. **Streamed tool-call updates feed the monitor facility.** Runs and
   jobs get real-time state from protocol events instead of log
   polling — the runtime-neutral push channel the accelerator ruling
   (docs/architecture.md) anticipates: harness accelerators keep
   working, and ACP narrows what only they could do.

## The client (the one architecture change)

`internal/acp` — a long-lived Go protocol client owned by the adapter
seam:

- **Session lifecycle**: initialize (capability negotiation), spawn
  `<cli> acp` under the job's process group with the launch config
  the runtime's adapter declares, `session/new` / `session/load`,
  prompt turns, cancellation (`session/cancel` maps from today's
  cancel verb), teardown. The child rides the SAME custody machinery
  as every delegate: pgid, instance tag, records — ACP changes the
  wire, not the supervision contract.
- **Permission policy**: the envelope's readRoots/writeRoots/network/
  approvals map to a pure decision function
  `internal/acp.Decide(envelope, request) allow|deny|escalate`;
  escalations surface exactly like today's approval requests. The
  decision function is CORE (runtime-neutral); what each runtime can
  ASK is capability-declared.
- **Usage parity**: ACP usage/metering events flow into the SAME
  typed-usage owner (internal/usage) through a registered recoverer/
  reporter — no second accounting path.
- **Evidence**: every JSON-RPC exchange appends to the job's round
  events (the flight recorder already carries the stream shape);
  the turn's typed result lands as the return candidate with
  protocol provenance.

## Runtime declarations (registry, per the agnosticism doctrine)

Each runtime declares its ACP capability: {command argv to start the
server, protocol version pinned, permission-request dialect quirks,
usage event shape}. `devin` declares first (the prototype); a runtime
without the declaration keeps its current adapter path unchanged —
ACP adoption is per-runtime and reversible.

## Prototype plan (design gate before build stands)

P1: a throwaway Go probe (cmd scratch, never shipped) that speaks
initialize/new-session/prompt/cancel against `devin acp` and records
the actual wire traffic — the protocol-reality check the critique
should demand before trusting this document's verb names.
P2: the internal/acp client with the fake runtime speaking a stub ACP
server (fixtures drive every path: permission grades, cancellation,
typed delivery, usage events, mid-turn death).
P3: devin's declaration + adapter integration behind a conf flag;
bm-style live smoke; the D61 waiver retires only when the graded
envelope demonstrably enforces (the acceptance story), with the D62
ladder kept as fallback until then.

## Blast radius

internal/acp (NEW), internal/runtimes (ACP declaration),
internal/adapter (devin integration + fake stub server),
internal/usage (ACP usage reporter), scripts/agents/adapters/devin.sh
(dispatch path behind the flag), dispatch/job records (protocol
provenance fields), fixtures (stub-server suite), docs
(orchestration's transport section).

## Loop discipline

Critique rounds with codex at xhigh; two-budget allowance; stop on
zero unrefuted material findings or the ratified exits. The critique
should especially attack: protocol assumptions not yet verified
against the real `devin acp` wire (P1 exists to fix that), the
permission decision function's completeness against today's envelope
semantics, custody/kill-path preservation for a long-lived child, and
the fallback story when ACP dies mid-turn.
