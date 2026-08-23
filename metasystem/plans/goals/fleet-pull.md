# fleet-pull

- State: claimed
- Intent: An idle machine picks up claimable shared-backlog work by itself: fleet liveness is the steward's duty, never a human's memory
- Origin: main
- Next step: Appetite: 4h for the DESIGN (Wido's expectation 2026-08-23: new claimable items on the remote backlog are always picked up by an idle machine). Scope: the steward's tick gains the fleet-pull duty — machine holds no claim AND the synced backlog carries a claimable tokened goal → launch or wake a coordinator session THROUGH THE RUNTIME ADAPTER SEAM (agent-agnostic; no runtime named in core). Design with the launch mechanics per runtime, the no-thrash bound (one pull attempt per tick, backoff on failure), the interplay with the session-level dead-man's switch (which stays as defense in depth, cadence then tunable or retirable), and fixtures. Composes with the ACP seam but must not wait for it. Design first, codex-reviewed; implementation slices follow their own tokens.
- OpenedAt: 2026-08-23T08:06:12Z
- Revision: 2
- Claimed: machine=widos-m5-pro lineage=coordinator at=2026-08-23T08:06:31Z

History:
- 2026-08-23T08:06:12Z B9R7HVR9Z1H2XQ1C9GGHX63018-widos-m5-pro-bf243850 open actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:06:31Z KDK3WC7GZ75KDA6DPFYDN1WYQ9-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=fleet-pull
Integrity: sha256=8e04a9bab2ef112de8f4a3416df626d1cba7b41f3906368ac92bd05a25901cc5
