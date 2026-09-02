# goal-edit-next-append

- State: queued
- Intent: goal edit --next replaces a goal's whole next step, and the seat composes the replacement by reading its LOCAL goal file, which lags the accepted tree until the next pull; on 2026-09-02 m1 edited codex-handshake-budget-load-fragile minutes after opening it, the local file did not exist yet, the read returned empty, and the ladder text was silently replaced by one measurement sentence (restored by hand two edits later). A ledger verb that lets a stale read erase a record is a robustness defect (R-33: robustness gain, under 4h). DONE means: goal edit gains an append form for the next step (or a --next-append flag) so a seat adds a fact without retyping the step, and edit refuses when the local goal file is behind the accepted tip for that goal, naming the pull that fixes it.
- Origin: main
- Next step: Small item (4h): design paragraph (append semantics, the staleness check against the accepted tip, the refusal text), Sol critique, Sol build with a fixture that replays the empty-read overwrite and proves it refused, Fable code critique, land with --chain.
- OpenedAt: 2026-09-02T16:27:54Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-02T16:27:54Z 4TPZGZPR25FVZRHWG7ADWKY5W2-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-02T16:28:05Z 930R2N46704Z7F94JMWWE1ZEZZ-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
Integrity: sha256=af9b155eb0986ac73c0775c04cbcef800ea6646e63d5e43fa6d6233ea6a818ec
