# suite-custody

- State: queued
- Intent: Validation suites run under process-group custody: killing a suite reaps its whole tree, and gate locks carry pids and self-clean when their owner dies (2026-08-24 collateral: orphaned go-gate child blocked the next battery)
- Origin: human
- Next step: RELEASED for the fresh session. DIAGNOSIS of the investigator zombies (2x): the task reproduces the detached-silence bug, and the reproduction itself wedges — the worker waits forever on the wedged detached child, log silent, status lying 'running'. The bug eats its investigator. INSTRUCTION: wrap every reproduction in an outer bounded timeout (the tar pit has no bound of its own), and slice the task smaller (reproduce+diagnose first, fix as a second task). m2's xtrace handoff stands; acceptance = detached green or loud named refusal; appetite 6h intact; shared tree clean.
- OpenedAt: 2026-08-24T13:24:00Z
- Revision: 16
- Labels: custody
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-24T13:24:00Z BTXPEJND104017B02XP26P6Q2N-m2-bc1be9cb open actor=human:wido targets=suite-custody
- 2026-08-25T18:30:20Z X9X4M0GTTVZ3DNET65423FCY5Y-m2-bc1be9cb claim actor=m2+mac-coordinator targets=suite-custody
- 2026-08-25T18:31:17Z HNKG26A2NPMRBE0M8CX48509G7-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-custody
- 2026-08-25T20:37:49Z MYR0WR78CKJCBA44FK4G0Q2H18-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-custody
- 2026-08-25T20:38:00Z N4007V77HVS7GE7XVD19QS0Z59-m2-bc1be9cb release actor=m2+mac-coordinator targets=suite-custody
- 2026-08-26T05:39:45Z YW1MGYSDSX5SAH5D7QW79G8XC2-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-custody
- 2026-08-26T11:08:46Z RSESH98DRSBWW7APMNCCBEE6H8-m1-bf243850 edit actor=m1+coordinator targets=suite-custody
- 2026-08-26T15:58:43Z DCRNEZ6DXXQE8TK14H0CWE3V5V-m1-bf243850 edit actor=m1+coordinator targets=suite-custody
- 2026-08-26T22:57:51Z HP1V9PA3GPD3JFGGJJ76NVT9B9-m1-bf243850 claim actor=m1+coordinator targets=suite-custody
- 2026-08-26T22:58:30Z EW73D363YYXX1EW15WGXEAS0AM-m1-bf243850 edit actor=m1+coordinator targets=suite-custody
- 2026-08-26T22:58:36Z 5N4M2N8PDCDM9GM3EWQ5NDF26C-m1-bf243850 release actor=m1+coordinator targets=suite-custody
- 2026-08-27T05:25:54Z 6H85JZHZYJT2NX47RZWGMA1K3F-m1-bf243850 claim actor=m1+coordinator targets=suite-custody
- 2026-08-27T05:47:44Z NCVC2B5BEV7YTN1CQG77364D1R-m1-bf243850 edit actor=m1+coordinator targets=suite-custody
- 2026-08-27T05:47:50Z PJ13ZY2K2F4VKXZM67QQK5TTZH-m1-bf243850 release actor=m1+coordinator targets=suite-custody
- 2026-08-27T06:09:57Z JJAJYB0EHF80HFFQ3RNDN84C0T-m1-bf243850 edit actor=m1+coordinator targets=suite-custody
- 2026-09-01T20:27:27Z M9Q8WJH6NXT28C4EVTD3K1PPVX-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=suite-custody
Integrity: sha256=3106e0f2606eddf34ef62bb280176cd9e97b4723cb6bf1cbeb19fc3428ea4141
