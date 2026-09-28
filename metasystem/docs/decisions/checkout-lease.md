# Checkout lease: decisions

- Kind: decision
- Id: 01M3MKDZKHW0X90ERJ2M3YS36V
- Status: accepted

Mechanism: `internal/lease` (one writer per checkout; every other session is a
read-only advisor). Designed 2026-08-07 after a second session in one checkout
shared its git index and was commanded by the turn-end hook to start streams
the first session was running. Distilled 2026-09-28 from
`records/misc/multi-main-coexistence.md` (removed that day; tag
`records-archive-2026-09-28`).

## Chosen

- One writer per checkout. Takeover only on provable holder death: liveness
  matches pid and start time, and a false "alive" only delays takeover, which
  is the safe direction.
- Authentication matches pid, start time and a fresh command-line hash read at
  every call. The residual is a same-second pid recycle with the same command
  line inside one read, and it fails by refusal.
- Callers are classified by an ancestry walk over kernel facts. No environment
  markers, because a hook subprocess cannot set its parent's environment.
- The human is never gated: a walk that reaches init with no match is the
  human's own tools, and direct human commits pass. This is cooperative
  discipline against accident, not a defence against a lying agent.

## Rejected

- Live two-writer coexistence, TTL-expiry takeover, takeover-in-progress
  adoption states, environment markers, and detection of foreign edits. Each
  was attacked in critique and removed rather than deferred. The census stays
  observation-only except for the gated verbs.

## Accepted residual

- A peer editing shared files with its own editor is invisible to any
  harness; three refusals and the paved worktree path are the defence.
