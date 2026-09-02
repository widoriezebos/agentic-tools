# host-health-role

- State: queued
- Intent: The steward watches its own roles but not the machine they run on: on the m1 Mac, Apple's fseventsd ran at 100 percent CPU and grew to 17 GB resident for 17 days, swap reached 94 percent full, and 488 leaked fixture processes accumulated over four days, and nothing in the metasystem noticed; the seat found it by hand on 2026-09-02 while chasing a 14-second Codex start. Wido's role-liveness order (2026-08-28) wants a visible one-line health message every interval and immediate named action on unhealthy state; the host itself deserves the same role. DONE means a steward health role reports host load, swap use and the top CPU and memory consumers with named thresholds, raises an alert episode through the existing path when a threshold is crossed (naming the process, whether it is ours, and the remedy), and stays quiet otherwise; proven by a fixture that feeds it a recorded snapshot of 2026-09-02.
- Origin: main
- Next step: Small item (4h): design paragraph (the facts read, the thresholds, what counts as ours via the census, the alert text), Sol critique, Sol build, Fable code critique, land with --chain. Depends on nothing; the alert channel goal later carries its episodes externally.
- OpenedAt: 2026-09-02T16:28:16Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-02T16:28:16Z 879DSYMN4C7E7TFRJGPMYNA5FF-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=host-health-role
- 2026-09-02T16:28:28Z GZWV4SHCAQ6J8NGC28GPREP8WF-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=host-health-role
Integrity: sha256=f307940b65bc5b7f4f5a7261d6013303ab315ca77bf98c7377e1ee1461b9d28a
