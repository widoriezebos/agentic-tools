# Draft: leverage the ACP capability

Status: draft — opened on Wido's order 2026-08-22; shape into
concrete slices once acp-adapter-seam lands. Each candidate below
becomes its own promoted slice with an appetite.

What the seam + ACP-default unlock, mapped to standing goals:

1. Ask-and-continue delegation. A delegate that hits a gap asks
   through the session's ask channel and keeps working; the steward
   routes what needs a human through the delivery-gated channels.
   Kills the die-on-ambiguity failure mode.
2. The narrator's observation surface. The D122 charter (continuous
   narration, anomaly-to-action) needs a live typed event stream;
   ACP sessions provide exactly that without new plumbing.
3. Real progress signals for patience. Stall-vs-progress judged on
   session events instead of file mtimes and log tails.
4. Clean mid-turn control. Cancel/park acts on the session, not the
   process tree; budgets enforce on live usage counts; the
   cancel-race class of defects shrinks structurally.
5. Cheap runtime heterogeneity. A new runtime is a config entry
   behind the seam; benchmark rosters comparing runtimes become
   trivial to compose.
6. Session persistence at turn boundaries. Exact-session resume as
   protocol capability, not CLI flag — feeds the turn-boundary
   embedding decision.
7. A small-change lane. A micro-dispatch is one turn in an open
   session instead of a full launch; the queued small-change-lane
   goal gets its natural mechanism.
