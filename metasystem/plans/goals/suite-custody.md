# suite-custody

- State: queued
- Intent: Validation suites run under process-group custody: killing a suite reaps its whole tree, and gate locks carry pids and self-clean when their owner dies (2026-08-24 collateral: orphaned go-gate child blocked the next battery)
- Origin: human
- Next step: RELEASED for the fresh session (2026-08-27 ~07:50): the direct-lane builder (job task-mtb2xrte) reproduced the pre-fix silence with xtrace in its sandbox but ZOMBIED mid-investigation; its changes died with the sandbox — the shared tree is clean, nothing landed, appetite still 6h. The fresh session starts from m2's standing handoff (xtrace at the unwatched-run stop-hook leg; acceptance = detached green or loud named refusal). Third sighting note: this investigation task shape has zombied twice — consider smaller task slices.
- OpenedAt: 2026-08-24T13:24:00Z
- Revision: 14
- Labels: custody

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
Integrity: sha256=9da610ca3459896d07180fe51767e0039bdb0d3ddc0231db022c65a6bce4947b
