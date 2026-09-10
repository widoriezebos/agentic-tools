# hp-terminal-grade-for-stopping-acts, critique round two: dispositions

Chain hp-terminal-build1-20260909. The second critic
hp-terminal-crit2-20260909 (claude-opus-5) reviewed the fold
hp-terminal-build1-20260909-r2 at tree
a03c1ef78f6a48f5a76bbbbb88c98ef84048d1b2 and returned two material
findings and two notes. The seat-side gate on the same tree had the Go
packages green (internal/goal 82.4 percent, internal/humanauthority
80.4 percent) and the proof-grades scenario still failing with no
message: its pseudo-terminal holder dies at once because `script` has
no standard input, and its assertion output went to that vanished
pseudo-terminal. Round three folds all four facts.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HPT-04 | accepted | On Linux, util-linux `script -c` returns the child's status only with --return, and the scenario's next line is an unconditional exit 0, so on the validation host the scenario cannot fail. | Round three passes --return on Linux, checks the status on both platforms, and fails the scenario on a non-zero inner run. |
| HPT-05 | accepted | The headless assertion runs from the bed's own child, which keeps the controlling terminal of whoever started the bed, and the scenario deletes every real adapter signature so a real agent is not recognised. | Round three runs the headless case with no controlling terminal (setsid on Linux; on Darwin a child started through nohup with standard input from /dev/null and its tty checked to be none) and keeps the real adapter signatures in the clone, removing only what the fake runtime needs removed, with a comment saying why. |
| HPT-06 | noted | ProveTerminal checks the terminal before the agent signature where the old helper checked the signature first, so one shape (an agent on another terminal) is now named TERMINAL_NOT_REACHED instead of AGENT_IN_AUTHORITY_CHAIN; both refuse. | Round three restores the old order so the name is unchanged; no behaviour widens either way. |
| HPT-07 | noted | ProveTerminal has no parent-continuity check (PROCESS_REUSED) where Prove does; the brief asked for Enroll's walk, which had none either, so this is a design fact, not a defect of the build. | Recorded on goal hp-every-by-proves-a-human as a design point: whether the terminal grade should carry the continuity check too. |
