# goal-machine-pinning

- State: queued
- Intent: A backlog item can be pinned to a named machine: only that machine may claim it, because it alone has the setup, network, or resources the work needs (Wido's request 2026-08-24)
- Origin: human
- Next step: Appetite: 4-6h. A Pinned field on the goal (machine nickname, same vocabulary as claims): claim refuses on any other machine with a message naming the pin; edit/open verbs set and clear it; the queue display shows it; fixtures pin refusal, transfer (re-pin), and the unpinned default. BINDS fleet-pull: an idle machine's pull must skip items pinned elsewhere — this lands BEFORE fleet-pull's re-scope so the puller inherits the rule, and it rides the same nickname enrollment (fail-closed, hostnames never published). Claimable now; small enough to slot between larger slices.
- OpenedAt: 2026-08-23T19:15:11Z
- Revision: 1

History:
- 2026-08-23T19:15:11Z NEK3SFJDMZY0MN8DPG6B8XZ4PH-m1-bf243850 open actor=human:wido targets=goal-machine-pinning
Integrity: sha256=62d0d45f3bf02075a6db56bbce4ad127061197077f2bbad5a6ec3d27927da925
