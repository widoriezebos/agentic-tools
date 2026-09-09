# hp-refusals-print-the-one-command

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: refusal text, it changes no decision; novelty 1: rendering codes to commands; exposure 3: every refused human act on every seat; accumulation 1: nothing compounds"
- Tier: 3
- Intent: Every refusal in the authority layer prints exactly one command that would have succeeded from where the human sits, or says in words that none would: TERMINAL_NOT_REACHED names the enrolled terminal by its tmux session or tty, says the current terminal is not it, that enrolling here needs no prior authority and retires the other terminal, and that the refused command is then retried; a structured no-enrollment outcome shared by both proofs maps to the enroll command; an unreadable or incomplete enrollment says no command can safely complete until the record is repaired, and how. Design revision 2 section 3 with findings HPA-07, HPA-08, HPA-14 and HPA-15 folded.
- Origin: main
- Next step: Sol builds the outcome code, the refusal renderer with the ephemeral carrier for the failed enrolled attempt (the CLI builder returns the request plus the structured attempt), and the texts for every code in internal/refusal/register.go the authority layer owns; fixtures assert each text against its code. One Sol round, one Opus review, land. Coordinate with goal human-goal-verbs-forgiving, which owns the same rule for the goal verbs' other refusals. ORDER: after hp-terminal-grade-for-stopping-acts lands. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:08Z
- Revision: 1
- Labels: comfort
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:08Z XSKZ0PEHVECNWBBMSF49PR7V2Z-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-refusals-print-the-one-command
Integrity: sha256=d9ef65e0727a36c7e54384799ee4476e01adccdd4d5c0653157e18a6af8d5eb4
