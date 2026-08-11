# Dispositions — patience-mission-reap-drain, critique round 4

Critic: design-critic-20260811t111706z-74d9 (codex gpt-5.6-sol, xhigh).
Verdict: 6 material. All 6 ACCEPTED; design amended in the same commit.
No machinery was challenged — all six were wiring edges of the sever —
so the loop stops by the diminishing-returns rule after four rounds and
33 accepted findings, and implementation is the next source of truth.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| 0 | high | The drain-stalled answer is rejected by the shipped answer path | ACCEPT — answer.go gains the reason with the `resume:` prefix, the same shipped pattern as the stop-loss reset. |
| 1 | high | The heal cannot distinguish the gap or recover the count after unpark | ACCEPT — the answer writes `lastDrainStall: {cycle, survivors}`; the heal consumes a matching field into the labeled line and clears it in the same write; no match heals as plain turn-lost. |
| 2 | high | Salvage obligations survived in the executable sections | ACCEPT — the Tests section is rewritten to the severed shape; no claim, no entry mode, no resumed conclusion remains anywhere. |
| 3 | high | Two incompatible re-proof positions | ACCEPT — normal resume means normal rules: re-proof lives where proving lives (the next dispatch's fence check, the next drain's reaps); the "before first dispatch" promise is deleted. |
| 4 | medium | Crash rule stated in reverse of the shipped park order | ACCEPT — aligned: state then ask; resume on a drain-stalled park re-raises a missing ask idempotently first. |
| 5 | medium | "Exactly one cycle late" is too strong | ACCEPT — softened to "at the next successful measurement, typically the next cycle". |
