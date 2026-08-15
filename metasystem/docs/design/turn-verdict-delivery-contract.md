# The turn-verdict delivery contract

The turn verdict is ONE engine decision — `report turn-verdict` returns
`{shouldBlock, blockSource, goal, ledgerStatus, display, ...}` — and this
contract says what an adapter must do to claim conformance for its
runtime. The mechanism is identical for every runtime; only delivery
automation differs, and the table below says so in public
(goal-system design, D66: exchangeability — any runtime fills any seat).

## What conformance means

An adapter conforms when its runtime's turn end:

1. invokes `report turn-verdict --root <checkout> --session <safe id>
   --watchdog-surfaced <digest|empty>` (the shipped
   `scripts/agents/supervision-hook.sh` does this for every runtime it
   serves);
2. honors `shouldBlock` through its runtime's own refusal mechanism,
   with `display` transported into the block reason byte-verbatim;
3. transports `display` into its non-blocking channel verbatim when not
   blocking;
4. never suppresses the degraded path: a nonzero verb exit surfaces the
   hook's fixed message ("turn-verdict unavailable: ..."), never
   silence, and never an all-clear the verb did not produce.

## The universal fallback (no hooks required)

Any runtime's orchestrator can read the same information by
instruction: `goal next` prints the one orientation line. AGENTS.md
instructs every main to read it at turn end. Under exchangeability this
is the same verdict on the only transport every runtime has — plain
command output.

## Conformance table (the DISTRIBUTION, not any installation)

States: declared (a shipped Stop config exists) / installed (the config
is wired by adoption) / observed (hooks seen firing live, by date) /
blocking-capable (a live block observed, by date). Rows carry only
evidenced states; upgrades are recorded here with their dates.

| Runtime | State | Evidence |
| --- | --- | --- |
| claude | installed; fixture-proven EMISSION | scripts/enforcement/claude-code-hooks.json wires the hook; the supervision fixture proves the hook emits decision:block; hooks observed firing live (2026-08, this repository) |
| codex | declared | scripts/enforcement/codex-hooks.json ships; live observation pending (backlog item 16's audit upgrades this row) |
| devin | declared | scripts/enforcement/devin-hooks.json ships; live observation pending (backlog item 16's audit upgrades this row) |

Which runtimes THIS checkout installed is answerable from
metasystem.conf and is not this table's job. The instruction audit
checks this table's claims against the shipped enforcement configs.
