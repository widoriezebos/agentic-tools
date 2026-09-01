# budget-death-on-return

- State: queued
- Intent: Three consecutive Fable delegate runs completed their entire product and then died on the native budget cap during the return protocol (ledger-attention design chain, 2026-08-30: $5.07, $8.19, $10.01 - every product recovered whole from stream or worktree by hand). The cap protects spend correctly but the RETURN is part of the work: a run that finishes its product deserves the few cents to say so, and hand-recovery does not scale. Cousin of the process-lost recollection law (a6488e1) at the adapter layer.
- Origin: main
- Next step: ROOT CAUSE FOUND (m0, 2026-09-01): the claude adapter's native budget defaults to $5.00 (internal/adapter/claude.go ClaudeBudget), overridable per-dispatch via METASYSTEM_CLAUDE_MAX_BUDGET_USD - never set anywhere. Every Fable design round on two-bars costs more than $5, so five sessions died at $5.4-5.6 (cap plus final-iteration overshoot) mid-work; the three ledger-attention specimens ($5.07, $8.19, $10.01) suggest the same default plus one prior override. The recorded mitigation for m0's dispatches: export the override sized to the round class (design rounds $15, recorded per R-28 as a thought-through spend decision - quality over cost per Wido's R-34-m2 word). The mechanical fix this goal still owes: a session hitting its native cap must emit its return FIRST (write-early return protocol) or the adapter must treat cap-death with an intact worktree as completed-with-recovery, not runtime_error.
- OpenedAt: 2026-08-30T21:19:46Z
- Revision: 3

History:
- 2026-08-30T21:19:46Z 2QMEPF28QYF4AGGRQV640JV0MV-m2-bc1be9cb open actor=m2+mac-coordinator targets=budget-death-on-return
- 2026-09-01T06:38:52Z Y1G939KMTVTWRYCM862SDPVZAH-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=budget-death-on-return
- 2026-09-01T08:18:28Z WA3E7X3ER2TP7XDERKFQZNK5CH-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=budget-death-on-return
Integrity: sha256=7b2839ed122ca01549b35a0cf7b98b5d244de10d6c215e16a595535d324178b3
