# stop-refusal-fits-on-one-screen

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a refusal nobody can read is a refusal nobody acts on, and the one actionable line drowns; novelty 1: the same bound the Telegram ask got last night, applied to the hook's refusal text; exposure 3: every seat on every machine ends every turn through this text; accumulation 1: each refusal is the same wall, it does not compound"
- Tier: 3
- Intent: Wido, 2026-09-06: 'the stop message is still insanely long.' The Stop hook's refusal text has no bound: on m1's first Stop after c1525b90a (the refusal carries the turn verdict) it printed about two hundred run records from August ('no continuation recorded') and one actionable line (an unwatched job) in a single refusal. The Telegram ask got its bound last night (renderQuestion, 1600 runes, the token first, the rest trimmed with a notice); the hook's refusal and its systemMessage get the same discipline. DONE means: the refusal reads, in order, the verdict in one line, the actionable items (each with the command that clears it), then at most a few lines of everything else summarized by class and count ('244 runs without a recorded continuation, oldest 2026-08-16; full list: <path>'), the whole thing bounded to roughly a screen; the full unbounded text is written to a file under the checkout's supervision evidence and the refusal names it. Not a change to what is judged - only to what is printed.
- Origin: main
- Next step: MECHANICAL, one chain: the renderer of the Stop refusal and systemMessage (the turn-verdict text builder the hook prints, in Go under internal/supervise or the hook's emit path), a class-and-count summariser for repeated lines, the full-text file beside the stop refusal records, a fixture that feeds two hundred stale run lines and asserts the bounded shape with the actionable line first; the stale-run listing itself (whether those runs are open work at all) stays with open-work-scanner-blindspots. Any free seat.
- OpenedAt: 2026-09-06T14:30:14Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T14:32:44Z revision=2 opid=C6Y1SKS3RC6T936TYZDM0W0EVS-m1-7cd0bd60 authority=proven digest=f34eb5076eb486ff58b6e2af66442a9d66304b18d5c89874afcd4ac88edb1846

History:
- 2026-09-06T14:30:14Z AE3TVHYT4WVWE623BDZZK3XWBK-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=stop-refusal-fits-on-one-screen
- 2026-09-06T14:32:44Z C6Y1SKS3RC6T936TYZDM0W0EVS-m1-7cd0bd60 approve actor=human:Wido targets=stop-refusal-fits-on-one-screen
Integrity: sha256=bbe6e430258ec9af4c2dd44c5279ae6841f802100178205d4d81ac94a44a2d7f
