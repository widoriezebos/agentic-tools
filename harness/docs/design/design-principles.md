# Design Principles

## Priority

When principles conflict, prefer: correctness and invariant protection; clear ownership and honest boundaries; operability and diagnosability; local reasoning and changeability; measured performance; then ceremony minimization.

## Core Standard

- Give every important behavior one clear owner and one obvious home.
- Keep domain policy, orchestration, infrastructure, transport, and external adapters at honest boundaries.
- Make data flow, lifecycle, state transitions, concurrency, invariants, timeouts, retries, partial success, and failure semantics explicit.
- Design diagnostic evidence at decision boundaries: structured outcomes, stable identifiers, bounded context, and stack traces for unexpected failures.
- Put tests at the responsibility that owns behavior. Prove outcomes and state transitions, not merely calls or object existence.
- Never weaken, skip, or delete a failing test to make work pass; a red test is a contract question for its owner. Write tests that can fail on their own merits — assert outcomes, not the implementation echoed back at itself.
- Prefer cohesive direct code over ceremonial abstractions. Introduce indirection for real variability, a real boundary, or demonstrated change pressure.
- Name the current production consumer before adding frameworks, registries, DSLs, plugin systems, bulk migrations, or other support layers.
- Choose the robust solution with the lowest total lifecycle complexity, including migration, recovery, testing, monitoring, and operations.
- Optimize only against measured budgets and keep the optimization inside the owning component.
- Make primary behavior changes as clean cutovers when feasible; remove superseded internal paths in the same change.

## Design Questions

Before implementation, be able to state:

1. The user/production contract, success criteria, and non-goals.
2. The owner sequence and data flow.
3. Each invariant and where it is enforced.
4. Failure, timeout, retry, cancellation, and partial-result behavior.
5. Observability and the runtime artifact that distinguishes important outcomes.
6. Focused tests at each owning boundary.
7. Migration/cutover and cleanup.
8. Why direct change, deletion, reuse, or no change is insufficient.

Use `design-obligation-gate.md` when the design will be implemented, expensively validated, reviewed for readiness, or called complete.
