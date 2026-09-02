# goal-edit-next-append

- State: queued
- Intent: goal edit --next replaces a goal's whole next step, and the seat composes the replacement by reading its LOCAL goal file, which lags the accepted tree until the next pull; on 2026-09-02 m1 edited codex-handshake-budget-load-fragile minutes after opening it, the local file did not exist yet, the read returned empty, and the ladder text was silently replaced by one measurement sentence (restored by hand two edits later). A ledger verb that lets a stale read erase a record is a robustness defect (R-33: robustness gain, under 4h). DONE means: goal edit gains an append form for the next step (or a --next-append flag) so a seat adds a fact without retyping the step, and edit refuses when the local goal file is behind the accepted tip for that goal, naming the pull that fixes it.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside an existing owner, the goal edit verb): build (Sol: an append form for the next step and a refusal when the local goal file is behind the accepted tip, with a fixture replaying the empty-read overwrite), one code review (Fable), land with --chain. No design round. Box stays 4h/10/240m/1.
- OpenedAt: 2026-09-02T16:27:54Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-02T16:27:54Z 4TPZGZPR25FVZRHWG7ADWKY5W2-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-02T16:28:05Z 930R2N46704Z7F94JMWWE1ZEZZ-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-02T18:36:37Z W30259HPZ24R2HAWKSEF7KF8CJ-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
Integrity: sha256=d23d0d7226473e2a6d2bffd292d3593946ffe58d2a20babfaf136886d7e6577d
