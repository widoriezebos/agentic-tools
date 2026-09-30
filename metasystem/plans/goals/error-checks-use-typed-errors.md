# error-checks-use-typed-errors

- State: parked
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="In-process refactor with no process boundary; behaviour preserved; tests pin each decision; reverts cleanly"
- Tier: 1
- Intent: Go code decides on errors with errors.Is/As and typed errors, never by searching an error's text for a code or phrase
- Origin: main
- Next step: Replace the 35 in-process error-text decision points (24 sites) inventoried in /Users/wido/LocalStorage/agentic-tools-evidence/structured-output-20260930/structured-output.md with typed errors; extend the static audit to refuse new ones. Priority 2 per Wido 2026-09-30 (D4)
- OpenedAt: 2026-09-30T12:03:35Z
- Revision: 1
- BlockedBy: processes-read-structured-results
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Parked: by=human:wido at=2026-09-30T12:03:35Z blocker=processes-read-structured-results because=blocked by processes-read-structured-results; returns when it is done

History:
- 2026-09-30T12:03:35Z JMRCDJGB10YRZKEABA5JT58418-m1e-b6a4eb0a open actor=human:wido targets=error-checks-use-typed-errors,processes-read-structured-results
Integrity: sha256=5c45cb69934e70fa3fd942a29f9d627dcb7cc3cb509a64143f76078fe166fe99
