# hp-terminal-grade-for-stopping-acts, critique round four: dispositions

Chain hp-terminal-build1-20260909. The fourth critic
hp-terminal-crit4-20260909 (claude-opus-5) reviewed the fold
hp-terminal-build1-20260909-r6 at tree
a58c68ae31edf9f06d3b1605146c4059b99cc1c9 and returned one material
finding and one note. Round seven folds both.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HPT-13 | accepted | Executed on the host: every macOS ancestry ends at process 1, /sbin/launchd, root-owned, arguments withheld from an ordinary user; the stable read refuses withheld arguments unless the executable is exactly /usr/bin/login, so the walk to the root always ends ARGV_UNREADABLE. Every real human is refused the terminal grade and, since Enroll now runs the same walk, enrollment is broken on macOS; the seat's bed and the unit tests cannot see it (agent-descended bed; fake processes always readable, uid 501). | Round seven admits, anywhere in the walk, a process whose arguments are withheld only when it is root-owned and its executable is a system-owned image the invoking user cannot have written (root-owned, not writable by group or others), recorded in the proof with its arguments-withheld flag, and continues past it; the walk still ends at process 1 and still refuses a user-owned process with withheld arguments. The rule subsumes /usr/bin/login and does not end at the first root-owned ancestor, because a setuid login above an agent's pseudo-terminal would hide the agent. Tests model the real shape (uid 0, arguments withheld, /sbin/launchd at process 1) and a live walk of the test's own ancestry asserts the outcome is never ARGV_UNREADABLE. |
| HPT-14 | accepted | If the shared readiness allowance expires across the boundary of the keeper and holder waits, the holder's start time is never recorded and cleanup skips the holder, leaving a 300-second sleep orphaned (never blocking the bed). | Round seven lets cleanup terminate the holder by its identity-file pid when the start time was never recorded, after proving the pid's command is the fake agent. |
