# goal-edit-next-append

- State: queued
- Intent: goal edit --next replaces a goal's whole next step, and the seat composes the replacement by reading its LOCAL goal file, which lags the accepted tree until the next pull; on 2026-09-02 m1 edited codex-handshake-budget-load-fragile minutes after opening it, the local file did not exist yet, the read returned empty, and the ladder text was silently replaced by one measurement sentence (restored by hand two edits later). A ledger verb that lets a stale read erase a record is a robustness defect (R-33: robustness gain, under 4h). DONE means: goal edit gains an append form for the next step (or a --next-append flag) so a seat adds a fact without retyping the step, and edit refuses when the local goal file is behind the accepted tip for that goal, naming the pull that fixes it.
- Origin: main
- Next step: Small item (4h): design paragraph (append semantics, the staleness check against the accepted tip, the refusal text), Sol critique, Sol build with a fixture that replays the empty-read overwrite and proves it refused, Fable code critique, land with --chain.
- OpenedAt: 2026-09-02T16:27:54Z
- Revision: 1
- Labels: robustness

History:
- 2026-09-02T16:27:54Z 4TPZGZPR25FVZRHWG7ADWKY5W2-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
Integrity: sha256=df3f4bb194784245ad212a322cdd6f5402e27364daa580f4821a83fe34c4c72f
