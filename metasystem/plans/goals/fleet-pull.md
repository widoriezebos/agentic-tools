# fleet-pull

- State: claimed
- Intent: An idle machine picks up claimable shared-backlog work by itself: fleet liveness is the steward's duty, never a human's memory
- Origin: main
- Next step: Appetite: 4h for the DESIGN (Wido's expectation 2026-08-23: new claimable items on the remote backlog are always picked up by an idle machine). Scope: the steward's tick gains the fleet-pull duty — machine holds no claim AND the synced backlog carries a claimable tokened goal → launch or wake a coordinator session THROUGH THE RUNTIME ADAPTER SEAM (agent-agnostic; no runtime named in core). Design with the launch mechanics per runtime, the no-thrash bound (one pull attempt per tick, backoff on failure), the interplay with the session-level dead-man's switch (which stays as defense in depth, cadence then tunable or retirable), and fixtures. Composes with the ACP seam but must not wait for it. Design first, codex-reviewed; implementation slices follow their own tokens.
- OpenedAt: 2026-08-23T08:06:12Z
- Revision: 6
- Claimed: machine=m1 lineage=coordinator at=2026-08-23T10:31:21Z

History:
- 2026-08-23T08:06:12Z B9R7HVR9Z1H2XQ1C9GGHX63018-widos-m5-pro-bf243850 open actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:06:31Z KDK3WC7GZ75KDA6DPFYDN1WYQ9-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:13:18Z CS6TRWQKADPT3S22HKSKS5TH11-widos-m5-pro-bf243850 release actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:16:21Z 97YYDE2BR2DX4NNP111C3NQ5NZ-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:17:58Z QM2KW7V24PHYDJJBGXAD22SBXD-widos-m5-pro-bf243850 release actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T10:31:21Z 81MNRPXTSD15RD4CXHW7943YEJ-m1-bf243850 claim actor=m1+coordinator targets=fleet-pull
Integrity: sha256=73fa34faaef09df6a2f4e3d36cd36b893c8ea3131ce82dea0fa096067ad4dc98
