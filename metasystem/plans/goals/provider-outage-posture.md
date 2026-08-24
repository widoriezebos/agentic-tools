# provider-outage-posture

- State: claimed
- Intent: A model-provider outage (529/overloaded) stalls the coordinator's brain but must never damage, mislead, or silence the metasystem: local machinery keeps working, the patience clocks stop counting, and a long outage reaches the human without the provider (Wido, 2026-08-24, after a live 529 halted everything)
- Origin: human
- Next step: Appetite: 1d SLICE ONE — IMPLEMENTED IN THE WORKING TREE, codex round 2 folded, landing battery in progress. What stands: (1) THE MARK — internal/outage, artifacts/agents/outage.json: bounded-flock Record/Clear (a hint never wedges its caller; runners hold their lease while clearing), 30-min horizon so an unfed mark lapses and can never blind the steward to a real stall, classification from CLI stderr logs (5xx adjacent to bounded error/status/http/code vocabulary) and is_error-gated provider documents; the bare word overloaded needs provider framing on the same line. (2) THE STEWARD — one outage sample governs each whole tick: aging pauses (progress still resets), the ladder holds revival (notify, never spawn), the narration says the fixed line. ACCEPTED RESIDUAL (amending the map's absolute clock claim): a mark landing inside the revival's consume-to-launch window still costs at most ONE dry revival at outage onset — the recheck sits directly before intent consumption, and full atomicity would couple outage writers into the steward arbitration lock. (3) THE RUNNER — overloaded host exits record feedsBreaker=false turn entries (PriorContext skips them without resetting the streak), never park host-failure, back off scaled only when a retry actually follows; clears happen ONLY on proven provider conversations (exit 0, exit 6) — never on our own cap cutting a stalled call. (4) DELEGATES — both paid calls (initial, repair) detect at the Go adjudication seam; a zero-exit CLI clears only with a correlated handshake, and a provider error document on a zero exit records instead. ACCEPTED RESIDUAL: the Devin delivery-repair launch bypasses this seam; the runner-side net is independent. Linux coverage baseline gained internal/outage AND the pre-existing missing internal/covenant. SLICE TWO (open): provider-independent human alert; the harness dead-man firing-loop backoff/coalescing lives in session-level harness configuration, out of repo scope; coordinator-seat failover far rung.
- OpenedAt: 2026-08-24T05:59:11Z
- Revision: 4
- Claimed: machine=m1 lineage=coordinator at=2026-08-24T05:59:16Z

History:
- 2026-08-24T05:59:11Z 2RJ9PF936ZPEWYD25Q0D93E565-m1-bf243850 open actor=human:wido targets=provider-outage-posture
- 2026-08-24T05:59:16Z 40V9KXZ96SBH55P2NCWKV2ZGG2-m1-bf243850 claim actor=m1+coordinator targets=provider-outage-posture
- 2026-08-24T06:02:33Z BY47CNZP5795PVF45DGF5BXD5R-m1-bf243850 edit actor=m1+coordinator targets=provider-outage-posture
- 2026-08-24T07:21:31Z 2DKV4AWD0J7E6NXZEF84ZD9NMC-m1-bf243850 edit actor=m1+coordinator targets=provider-outage-posture
Integrity: sha256=91fcf94deec8307640c562d3ba583a8f500fccee35a844485e3942de04995535
