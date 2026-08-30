# timing-tests-synthetic-clock

- State: claimed
- Intent: Timing-logic tests run on an injected synthetic clock and finish in microseconds; only legs that spawn real processes keep the real clock — the race gate's 25min is mostly wall-clock waits on arithmetic (Wido 2026-08-27 evening: why does timing-sensitive testing take so long, should we use a synthetic clock)
- Origin: main
- Next step: SLICE 2 LANDED 845f76b (m2, 2026-08-30): the wind-down abandonment wedge is closed - terminateGroup rides the janitor tri-state ownership (the runner's strings.Contains substring proof retired; positional shapes only), kills through INDETERMINATE mid-death re-checks inside the bounded window, refuses only provably-recycled groups, and floors the TERM grace at 2 real seconds + a floored SIGKILL death-wait regardless of compression scale; a group surviving kill-through is loud typed evidence (leaked-group), never silent. Accounting proven on real groups incl. 4 cycles at scale 50 with zero abandoned; full missionrunner race suite green. Rider: coverage-delta's probe gained the go gate's 30m ceiling (its bare 10m default was timing out on this package and reading as failure). REMAINING SLICES: (3) 3h sub-second taint identity + recovery windows; (4) 3h t.Parallel decoupling; after those, revisit the four scale-1000 pinned families - slice 2's fix may lift some pins. Released for rotation.
- OpenedAt: 2026-08-27T17:12:26Z
- Revision: 12
- Labels: shared
- Budget: elapsedLimit=3h attemptLimit=5 reservedJobMinutesLimit=45 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T01:58:44Z revision=12
- StopCapability: generation=12 revision=12 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-27T17:12:26Z GRZ4RPVHPK0D6H2SKE8P1X46EV-m2-bc1be9cb open actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-27T17:15:51Z 8TK863Y9F7XH960CTKX092C0AN-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:00Z 6RK75MKGKCSA79BE53CY00SBD0-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:27Z 20QV9034V6V3WG3Z4STZDRR5EV-m2-bc1be9cb set-budget actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-29T14:12:41Z 6RDN6FT5A113SMWB9VKR2PGV20-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:40:54Z 20RDB340JW8DFKARY5S1KD1BKA-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-29T14:41:08Z 05QC5NP22JDT5PZFTQVNY4002B-m2-bc1be9cb release actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T00:15:28Z FMMXTJ3Q89WPVBCN7PV9XZM4GK-m2-bc1be9cb set-budget actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-30T00:15:43Z 7Q7BARE3503WDARD7AMKC2T4X2-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T01:57:52Z W2V6VQY8950XBRK5JYQYTANRCY-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T01:58:07Z J8AD9QKW9SAT01A4FEMFVYMD8Q-m2-bc1be9cb release actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
- 2026-08-30T01:58:44Z GHMBHXQPE965BG5Q19WS6Q4ATC-m2-bc1be9cb claim actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
Integrity: sha256=41fec64dfdb83a1eb996ac47be962668e29e370dbfded5a345065761044ebfe9
