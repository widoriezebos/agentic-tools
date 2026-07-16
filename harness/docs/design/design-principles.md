# Design Principles

## Priority

When principles conflict, prefer: correctness and invariant protection; clear ownership and honest boundaries; operability and diagnosability; local reasoning and changeability; measured performance; then ceremony minimization.

## Core Standard

- Give every important behavior one clear owner and one obvious home.
- Keep domain policy, orchestration, infrastructure, transport, and external adapters at honest boundaries.
- Make data flow, lifecycle, state transitions, concurrency, invariants, timeouts, retries, partial success, and failure semantics explicit.
- Design diagnostic evidence at decision boundaries: structured outcomes, stable identifiers, bounded context, and stack traces for unexpected failures. Redact secrets and personal data from all of it.
- Put tests at the responsibility that owns behavior. Prove outcomes and state transitions, not merely calls or object existence.
- Never weaken, skip, or delete a failing test to make work pass; a red test is a contract question for its owner. Write tests that can fail on their own merits: assert outcomes instead of echoing the implementation back at itself.
- Keep tests deterministic: fake clocks and controlled concurrency instead of sleeps and wall-clock waits; fixed seeds; isolated state and fixtures.
- Prefer cohesive direct code over ceremonial abstractions. Introduce indirection for real variability, a real boundary, or demonstrated change pressure.
- Name the current production consumer before adding frameworks, registries, DSLs, plugin systems, bulk migrations, or other support layers.
- Choose the robust solution with the lowest total lifecycle complexity, including migration, recovery, testing, monitoring, and operations.
- Optimize only against measured budgets and keep the optimization inside the owning component.
- Make primary behavior changes as clean cutovers when feasible; remove superseded internal paths in the same change.

## Responsibility-Driven Design

The core standard above is responsibility-driven. The practical method:

- Start from responsibilities: name the decisions and duties the change involves before naming classes or modules.
- Assign each responsibility to one owner, and place behavior next to the state and invariants it guards. If a function needs another object's data to make a decision, the decision probably belongs to that object.
- Express collaborators as roles with small contracts stating what they promise, and let callers depend on the role instead of the implementation.
- When a class is hard to name, hard to test, or hard to document, treat that as an ownership problem. Split or move the responsibility before polishing the code.

## Domain-Driven Design

Apply this section in proportion to how much domain logic the system carries. A CRUD service or an infrastructure tool does not need aggregate ceremony; a system full of business rules does.

- Use the domain's own language in code. Names for classes, methods, and events come from how the business talks, and one term means one thing within a context. When a domain expert would not recognize a name, the model has drifted.
- Draw bounded contexts. A model is only valid inside its boundary, and the same word may mean different things in different contexts: an "order" in checkout differs from an "order" in shipping. Translate at the boundary instead of sharing models; a small translation layer is cheaper than a shared model that serves nobody well.
- Cluster state that must change together into one aggregate, with one owner enforcing its invariants. A transaction changes one aggregate; effects on other aggregates travel as events.
- Distinguish entities, where identity matters and attributes change, from value objects, where only the value matters and immutability is the default.
- Keep domain rules in the domain objects. When every rule lives in service classes operating on data bags, ownership is lost and the responsibility rules above are being violated at scale.

## Documentation

- Code is self-documenting first: if a comment would translate confusing code into English, improve the names, types, or shape instead.
- Document what code cannot express: why an owner exists, the boundary or lifecycle it protects, invariants, failure semantics, and deliberate non-responsibilities.
- Durable comments never reference plans, tickets, phases, branches, dates, or other project state; translate history into the standing contract that remains true in the code.
- Skip boilerplate: remove any comment that restates the name, lists methods, or could describe any similar class.
- Update or delete documentation in the same change that moves the behavior it describes; stale purpose documentation is worse than none because it sends changes to the wrong owner.

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
