# goal-machine-pinning

- State: claimed
- Intent: A backlog item can be pinned to a named machine: only that machine may claim it, because it alone has the setup, network, or resources the work needs (Wido's request 2026-08-24)
- Origin: human
- Next step: Appetite: 4-6h. A Pinned field on the goal (machine nickname, same vocabulary as claims): claim refuses on any other machine with a message naming the pin; edit/open verbs set and clear it; the queue display shows it; fixtures pin refusal, transfer (re-pin), and the unpinned default. BINDS fleet-pull: an idle machine's pull must skip items pinned elsewhere — this lands BEFORE fleet-pull's re-scope so the puller inherits the rule, and it rides the same nickname enrollment (fail-closed, hostnames never published). Claimable now; small enough to slot between larger slices.
- OpenedAt: 2026-08-23T19:15:11Z
- Revision: 2
- Claimed: machine=m1 lineage=coordinator at=2026-08-24T00:38:11Z

History:
- 2026-08-23T19:15:11Z NEK3SFJDMZY0MN8DPG6B8XZ4PH-m1-bf243850 open actor=human:wido targets=goal-machine-pinning
- 2026-08-24T00:38:11Z 0NVVXE8FS0TSG3XMAPS1Y1XNAC-m1-bf243850 claim actor=m1+coordinator targets=goal-machine-pinning
Integrity: sha256=4c94207afe6d2e81621ed4a9033f42e99b0ebdf784fd85e9b462815f90a01a05
