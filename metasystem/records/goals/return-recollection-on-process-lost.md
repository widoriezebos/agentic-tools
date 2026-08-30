# return-recollection-on-process-lost

- State: done
- Intent: A job that dies process-lost with a complete schema-valid return on disk gets its return recovered and adjudicated, marked recollected — D64's delivery lesson at the job level (2026-08-24 collateral: killed critic's finished findings sat undelivered in raw.out)
- Origin: human
- Next step: Appetite: 2h. On process-lost, the dispatch custody path runs the collect walk over the round dir before finalizing: a complete valid return flips the round to delivered-recollected (provenance noted), only a truly absent or invalid return stays failed. Acceptance: kill a critic after its raw.out lands — the job record carries the adjudicated findings.
- Concluded: Landed a6488e1: the reap's process-lost branch runs the recollection walk after wind-down - a complete schema-valid return.json in the newest round adjudicates and the job concludes completed with recollection provenance; absent or invalid returns keep their failed verdict, and a raw-only candidate lawfully stays lost (the schema demands the session observation a post-mortem cannot fabricate - a boundary, not deferred work). Acceptance proven in the dispatch bed via the fake's new return-then-process-loss behavior; one unreproduced first-run red recorded in the landing message with the leg standing as the watch.
- OpenedAt: 2026-08-24T13:24:18Z
- Revision: 5
- Labels: custody
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1

History:
- 2026-08-24T13:24:18Z VV6Z3A1846WEDT4KKRYEBWNVDD-m2-bc1be9cb open actor=human:wido targets=return-recollection-on-process-lost
- 2026-08-26T05:40:12Z 8G3VZJN0ABKSB9G17PHEFPHEE6-m2-bc1be9cb edit actor=m2+mac-coordinator targets=return-recollection-on-process-lost
- 2026-08-30T03:51:17Z 9ET7T4P95AVSZEA3BA39EYTAYM-m2-bc1be9cb set-budget actor=human:wido targets=return-recollection-on-process-lost
- 2026-08-30T03:51:32Z EKYJBF02STWP1N7CPCX6DH22H2-m2-bc1be9cb claim actor=m2+mac-coordinator targets=return-recollection-on-process-lost
- 2026-08-30T04:29:10Z 93F0B5763JKYE2ZWMH2ZJBJJ7T-m2-bc1be9cb done actor=human:wido targets=return-recollection-on-process-lost
Integrity: sha256=947066b9fb6c5cc5b6a7d6b527613bfff4207832445d73b1afa5fedd28ebc9a9
