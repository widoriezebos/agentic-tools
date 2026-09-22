# The MetaSystem: doctrine

- Kind: doctrine
- Id: 01M348YTJ2MWB2A7QPQQKDD5VK
- Status: accepted

## Context

The concepts document says what the machine is for: "a machine for letting agents build software unattended without surrendering the properties a careful human team would keep", where "each mechanism below exists to keep one of those properties under adversarial conditions — crashes, races, model error, and the permanent temptation of an agent to narrate success instead of proving it" (`docs/concepts.md`). The architecture map says what exists: "the metasystem's decisions live in one Go binary", recorded as "MAP, not design", with the package docs as "the per-package authority" (`docs/architecture.md`). The documents below are bound where they are, in reading order: the concepts, the map, the principles designs are held to, the gate they pass, and the contracts of the mechanisms that have shipped. This index adds nothing to them.

## Chapters

- doc:metasystem/docs/concepts.md — The metasystem in concepts
- doc:metasystem/docs/architecture.md — The engine: what the Go binary is and how it is laid out
- doc:metasystem/docs/design/design-principles.md — Design Principles
- doc:metasystem/docs/design/design-obligation-gate.md — Design Obligation Gate
- doc:metasystem/docs/design/wire-documents.md — Wire documents: the on-disk JSON contract
- doc:metasystem/docs/design/dispatch-sequence.md — The dispatch path's actual call sequence
- doc:metasystem/docs/design/mission-cycle-sequence.md — The mission runner's actual cycle sequence
- doc:metasystem/docs/design/turn-verdict-delivery-contract.md — The turn-verdict delivery contract
- doc:metasystem/docs/design/evidence-that-can-only-refute.md — Evidence that can only refute
- doc:metasystem/docs/design/stop-loss-core.md — The stop-loss core: the fuse contract
- doc:metasystem/docs/design/flight-recorder.md — The flight recorder: the event-stream contract
- doc:metasystem/docs/design/supervision-lifecycle.md — The supervision lifecycle: what owners, watchers, and reapers must do
- doc:metasystem/docs/design/supervision-registry.md — The supervision registry: the machine-wide custody contract
- doc:metasystem/docs/design/acp-transport-rationale.md — Why ACP: the shape, and why Devin needed it first
