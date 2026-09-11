# hp-refusals-print-the-one-command

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: refusal text, it changes no decision; novelty 1: rendering codes to commands; exposure 3: every refused human act on every seat; accumulation 1: nothing compounds"
- Tier: 3
- Intent: Every refusal in the authority layer prints exactly one command that would have succeeded from where the human sits, or says in words that none would: TERMINAL_NOT_REACHED names the enrolled terminal by its tmux session or tty, says the current terminal is not it, that enrolling here needs no prior authority and retires the other terminal, and that the refused command is then retried; a structured no-enrollment outcome shared by both proofs maps to the enroll command; an unreadable or incomplete enrollment says no command can safely complete until the record is repaired, and how. Design revision 2 section 3 with findings HPA-07, HPA-08, HPA-14 and HPA-15 folded.
- Origin: main
- Next step: Sol builds the outcome code, the refusal renderer with the ephemeral carrier for the failed enrolled attempt (the CLI builder returns the request plus the structured attempt), and the texts for every code in internal/refusal/register.go the authority layer owns; fixtures assert each text against its code. One Sol round, one Opus review, land. Coordinate with goal human-goal-verbs-forgiving, which owns the same rule for the goal verbs' other refusals. ORDER: after hp-terminal-grade-for-stopping-acts lands. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed). CARRIED IN from hp-terminal-grade-for-stopping-acts (2026-09-09): its implementer stopped on the gap that the builder was asked to keep the full walk's refusal for a later renderer but no carrier was named; the orchestrator accepted the gap and assigned it here (design finding HPA-14): the CLI builder returns the engine request plus the structured enrolled attempt, and the refusal renderer takes that attempt. Until this goal lands, a terminal-grade proof that is refused at an enrolled row renders the generic grade refusal, not the enrolled-terminal recovery row.
- OpenedAt: 2026-09-09T14:35:08Z
- Revision: 4
- Labels: comfort
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:08Z XSKZ0PEHVECNWBBMSF49PR7V2Z-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-refusals-print-the-one-command
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
- 2026-09-09T15:34:52Z 89X5JB5NPDJYMQQ87PX9S2YWWT-m1-1701c13c edit actor=m1+main-1788940932-18533-7fa6c2 targets=hp-refusals-print-the-one-command
- 2026-09-11T08:00:02Z 3KGMKXAPN2Y597E1K0HRX7SX7T-m1-c6925449 unapprove actor=human:Wido targets=hp-refusals-print-the-one-command reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
Integrity: sha256=9daddf386610d16396c8d7a3e04b6e45b7f538b88590c721400204cfecf508a8
