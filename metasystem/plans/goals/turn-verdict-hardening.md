# turn-verdict-hardening

- State: claimed
- Intent: HIGHEST PRIORITY (Wido's order 2026-09-02, verbatim: 'we need machinery (not you, your behaviour, yourself but deterministic Go code) that should make this impossible or at least give us the highest chance of this never happening again'). Three seat-stops with ready work on the board (records/misc/seat-stop-analysis.md) each passed through the EXISTING turn-exit gate: scripts/agents/supervision-hook.sh calls report turn-verdict at the Stop event and internal/report/stopblock.go refuses a quiet exit — but the gate has four deterministic escapes, named by Sol in records/misc/seat-stop-analysis-critique-r1.md: (1) block-once — the refusal fires once per unchanged open-work signature and promises not to repeat, so a hollow continuation plus a second stop passes; (2) existential INFLIGHT — any unrelated same-session job or watch launders idleness on every other ready goal; (3) fail-open — missing engine, unresolved root (goal supervision-hook-wrong-root), lookup failure, timeout or turn-verdict I/O error all allow the exit; (4) relay-minted HUMANSTOP — the temporary relay path cannot verify who supplied the words. Close them in the existing owner, in Go, deterministically: no block-once for READY work; INFLIGHT relevant to the ready frontier (joined by goal and revision), not existential; fail CLOSED with a complete outcome table (missing, unreadable, stale, timeout, identity-unknown, wrong-root, lock, emission); READY scoped to the machine+lineage owner pair with a stated testable admission predicate and a freshness proof for the projected board; HUMANSTOP only from a human-classified caller with an atomic compare-and-consume lifecycle bound to the Stop decision. Residual to state honestly: the Stop hook is a valid re-prompting point but not exclusive or mandatory (trust, disabled hooks) — the design owns enrollment and version compatibility or names the residual. Two-seat fixtures required (shared machine)
- Origin: main
- Next step: Design first (Fable 5.1 lane): one design covering the five closures with the outcome table and the two-seat fixtures; then Sol critique; build in slices of at most 240 reserved minutes (slice 1: block-once removal + relevant INFLIGHT + fail-closed table, the three that would have caught all three specimens; slice 2: seat-scoped READY + freshness; slice 3: HUMANSTOP lifecycle); Fable code critique per slice; land with --chain. Sequenced with supervision-hook-wrong-root: either land that fix first or carry its resolution in slice 1
- OpenedAt: 2026-09-02T07:19:48Z
- Revision: 3
- Labels: priority-1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-02T07:56:13Z revision=3
- StopCapability: generation=3 revision=3 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-02T07:19:48Z KA597TSPF61Y7YC5ZFY7DFCVQ6-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=turn-verdict-hardening
- 2026-09-02T07:19:52Z ACEYSWFMRYQ6B8YR66EJPK077S-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=turn-verdict-hardening
- 2026-09-02T07:56:13Z FEY1SHQPXEY6GM0FFAYN2H5JBN-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=turn-verdict-hardening
Integrity: sha256=4beead0bfe97dfe590d8960913c25f447414523b50f279b21ec7226ded069355
