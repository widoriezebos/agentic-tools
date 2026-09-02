# codex-handshake-budget

- State: queued
- Intent: The codex adapter's session handshake budget is a fixed ten seconds (scripts/agents/adapters/codex.sh:71, sessionEstablishedTimeoutSec) while a cold codex 0.148.0 start on the host Mac takes eight to fourteen seconds to emit thread.started, so a slow start is recorded as HANDSHAKE-FAILED and spends a cause-blind attempt - failing when slow, the class R-35-m3 names as a defect. DONE means a slow but progressing codex start is never a failure: the handshake waits on progress (process alive and its session file or first stream byte appearing) with a measured ceiling, and refuses only when the process is dead or silent past the ceiling
- Origin: main
- Next step: Appetite: 4h, full ladder (design, Sol critique, Sol build, Fable critique). SPECIMENS: m1b 2026-09-02, jobs design-critic-049b1ce02dea946074cba4f6 (11:48:39Z start, handshake_timeout 11:48:51Z, empty events) and two-bars-cc-crit-2 (11:52Z, thread.started written at the deadline, refused anyway); four direct codex exec runs on the same machine took 8, 8, 14 and 12 seconds to thread.started (plugins disabled changed nothing). INTENT: replace the fixed constant with progress-based patience per R-35-m3 (the steward-tick fix 4a5ef499 is the recorded pattern), reading the budget from a measured per-machine ceiling rather than a constant baked into the capability snapshot. CONSTRAINTS: the reaper's handshake backstop (dispatch.sh handshake_backstop_grace_sec) and the recorded-deadline contract in internal/dispatch/ownership.go and reapfacts.go keep one owner of the verdict; a dead process still terminalizes promptly; the claude and devin adapters are reviewed for the same shape but changed only with their own specimen. TEST SHAPE: a stub runtime that emits its session event at budget plus five seconds while alive is admitted; one that emits nothing and exits is failed at the ceiling; existing handshake fixtures keep their verdicts.
- OpenedAt: 2026-09-02T11:54:55Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-02T11:54:55Z 72WQ01G8W1CKYESQ88QC3DACDZ-m1b-fad3674e open actor=m1b+main-1788333346-60696-6a3256 targets=codex-handshake-budget
- 2026-09-02T11:55:08Z R819GZ1EN9EQTGHSSV8ZYKC894-m1b-fad3674e set-budget actor=m1b+main-1788333346-60696-6a3256 targets=codex-handshake-budget
Integrity: sha256=25b5f22abe13782de4786bd92075cb751b10e7bbd6a0b0e8da69080a422383e3
