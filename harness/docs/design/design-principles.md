# Design Principles

This is the canonical design standard. It describes the standard the code should move toward, and it applies to new features, architectural changes, refactors, and reviews. No design is future-proof. The goal is to make future change cheap, safe, local, and understandable.

## Priority

When principles conflict, prefer in this order: correctness and invariant protection; clear ownership and honest boundaries; operability and diagnosability; local reasoning and changeability; measured performance; then ceremony minimization.

Two examples of what that order means in practice. When observability seems to conflict with encapsulation, expose owned diagnostic views or structured events at boundaries instead of raw internals. When a measured hot path needs a low-level implementation, optimize inside the owning component and document the measured reason; performance is never a blanket excuse to ignore ownership elsewhere.

## What Good Design Achieves

- A newcomer can find where a behavior lives without tribal knowledge.
- A change usually has one obvious home.
- Rules are enforced where they belong instead of being scattered across helpers and adapters.
- State transitions and invariants are explicit, and infrastructure stays at the boundaries.
- Failures are explainable from logs, metrics, traces, and structured runtime state.
- Tests validate behavior at the level where it is owned.
- Large redesigns happen by replacing one responsibility at a time.

## Core Standard

- Give every important behavior one clear owner and one obvious home.
- Keep domain policy, orchestration, infrastructure, transport, and external adapters at honest boundaries, with dependency direction pointing at stable concepts. The domain never depends on delivery or storage mechanics.
- Entrypoints may coordinate collaborators. They must not absorb the behavior they coordinate, and they must not rebuild collaborator decisions inline.
- Objects own and protect their invariants: keep mutable state private, expose intention-revealing operations, and never let outside code mutate structures it does not own. If a caller must understand an object's internals to use it safely, the boundary is wrong.
- Prefer composition over inheritance. Inheritance is for true subtype relationships with preserved invariants; if a subtype cannot stand in for its parent without surprises, the model is wrong.
- Make data flow, lifecycle, state transitions, concurrency, invariants, timeouts, retries, partial success, and failure semantics explicit. Prefer named states and transitions over collections of loosely related flags, and make invalid states unrepresentable where practical.
- Validate input shape at the boundary and enforce invariants in the owning model. These are different jobs and both matter.
- Design diagnostic evidence at decision boundaries: structured outcomes, stable identifiers, bounded context, and stack traces for unexpected failures. Redact secrets and personal data from all of it. Log a failure once, at the boundary that owns it. New behavior defines the logs, metrics, or events that prove it worked and explain its failures.
- Put tests at the responsibility that owns behavior. Prove outcomes and state transitions, and cover failure, retry, and timeout paths as deliberately as the happy path.
- Never weaken, skip, or delete a failing test to make work pass; a red test is a contract question for its owner. Write tests that can fail on their own merits: assert outcomes instead of echoing the implementation back at itself.
- Reference truth (goldens, fixtures, expected outputs, evaluation labels) is established only from direct source evidence or explicit human instruction, never from the system's own output. Never edit truth to make output pass; when the evidence is ambiguous, record the ambiguity and leave truth unchanged. Whether the truth is verified and whether the software reproduces it are separate facts: verified truth the software cannot reproduce is a software defect, not a reason to change the truth.
- Keep tests deterministic: fake clocks and controlled concurrency instead of sleeps and wall-clock waits; fixed seeds; isolated state and fixtures.
- Prefer cohesive direct code over ceremonial abstractions. Introduce indirection for real variability, a real boundary, or demonstrated change pressure. Design for the next likely change rather than every imaginable one.
- Name the current production consumer before adding frameworks, registries, DSLs, plugin systems, bulk migrations, or other support layers.
- Choose the robust solution with the lowest total lifecycle complexity, including migration, recovery, testing, monitoring, and operations.
- Optimize only against measured budgets and keep the optimization inside the owning component. Design hot and high-volume paths around bounded work, bounded memory, and bounded time.
- Make primary behavior changes as clean cutovers when feasible, and remove superseded internal paths in the same change. No feature flags, shadow modes, or dual primary paths for internal design corrections; when blast radius is high, make rollback explicit at the true external boundary instead. A switch that outlives its measurement and rollback window is a decommission task: delete the losing path and the switch itself, and when removal cannot land in the same change, the cutover decision names the removal as its own tracked work item.
- Keep changeable semantics declarative: pattern libraries, prompts, and policy tables live in configuration while code owns control flow, state transitions, and validation. Use model-based reasoning only for genuinely ambiguous problems, and deterministic code or configuration for stable, finite policies. Never hardcode domain terms or language-specific heuristics into generic logic.
- Make expensive external calls (model inference, paid APIs) replayable: cache or record responses keyed by the full request content, and document what deliberately stays outside the key, because those paths replay invisibly. An accumulated cache of paid responses is a paid artifact: back it up with verified copies, and treat any change to what enters the key, including a cosmetic byte change to a request's static prefix, as a reviewed decision that prices the invalidation before making it. Speed and provider-behavior measurements taken through a warm cache are fiction; bypass the cache on both arms of such comparisons.
- When adopting an external reference design, reconcile it into one canonical owner and contract in this system before composing it with anything else. Locally correct fragments from different sources are unsafe until the combined behavior has a single owner and named invariants.

## Failure and Concurrency

- Name the failure categories that matter to the domain: validation, precondition, timeout, cancellation, dependency failure, overload, partial failure, invariant breach. A single undifferentiated "operation failed" hides materially different situations.
- Distinguish retryable from terminal failures at the owner with the best information, and preserve the causal chain when translating errors across boundaries.
- Represent partial success and partial failure explicitly in multi-step flows.
- Prefer immutability or single ownership for shared state. Make retries idempotent where commands may repeat. Surface conflicts and timeouts as explicit outcomes; never overwrite silently.
- Use bounded parallelism, explicit timeouts, and cancellation contracts for concurrent work, and propagate correlation context across threads and executors.
- Give each stage of a pipeline or agent loop a contract: what it consumes, what it produces, what it guarantees, and how it terminates. Preserve provenance when later stages depend on evidence from earlier ones.

## Responsibility-Driven Design

The core standard above is responsibility-driven. The practical method:

- Start from responsibilities: name the decisions and duties the change involves before naming classes or modules.
- Assign each responsibility to one owner, and place behavior next to the state and invariants it guards. If a function needs another object's data to make a decision, the decision probably belongs to that object.
- Express collaborators as roles with small contracts stating what they promise, and let callers depend on the role instead of the implementation.
- Group code by domain role or capability rather than by convenience. Packages named `util`, `misc`, `support`, or `common` are dumping grounds for code whose owner was never decided.
- When a class is hard to name, hard to test, or hard to document, treat that as an ownership problem. Split or move the responsibility before polishing the code.
- A very large class is a strong smell and usually needs splitting by responsibility. A small class with mixed responsibilities is already wrong. Size is a heuristic, never a license.
- If the reader must hop through several files or forwarding classes to find the actual decision, the ownership boundary or abstraction shape is wrong.

## Domain-Driven Design

Apply this section in proportion to how much domain logic the system carries. A CRUD service or an infrastructure tool does not need aggregate ceremony; a system full of business rules does.

- Use the domain's own language in code. Names for classes, methods, and events come from how the business talks, and one term means one thing within a context. When a domain expert would not recognize a name, the model has drifted.
- Draw bounded contexts. A model is only valid inside its boundary, and the same word may mean different things in different contexts: an "order" in checkout differs from an "order" in shipping. Translate at the boundary instead of sharing models; a small translation layer is cheaper than a shared model that serves nobody well.
- Cluster state that must change together into one aggregate, with one owner enforcing its invariants. A transaction changes one aggregate; effects on other aggregates travel as events.
- Distinguish entities, where identity matters and attributes change, from value objects, where only the value matters and immutability is the default.
- Keep domain rules in the domain objects. When every rule lives in service classes operating on data bags, ownership is lost and the responsibility rules above are being violated at scale. Simple transport DTOs and projections may stay passive; they only carry data across a boundary.

## Self-Documenting Code

Write code a maintainer can follow from names, types, method shape, and local control flow, without private context. This is a design requirement. Comments are the fallback for what code cannot say (next section).

- Choose intention-revealing names. A name should state what the thing owns or decides. If the honest name would be vague ("helper", "util", "manager"), the responsibility is probably wrong.
- Avoid vague verbs such as `process`, `handle`, `execute`, or `manage` when a more specific responsibility exists.
- Prefer rich domain types over primitive values, boolean flags, and generic maps. A named type carries meaning a string cannot.
- Extract named predicates, policies, and transitions when a condition or rule represents a real concept. `isEligibleForRetry(order)` tells the reader what a three-clause boolean expression makes them work out.
- Shape methods so the main path reads top to bottom at one level of abstraction. Extract a named step when the reader must hold two levels at once, and never extract a helper that hides complexity without naming a real concept.
- Make side effects, retries, failure outcomes, and mutation visible in method names, result types, or state types.
- When a comment seems needed to explain what code does, fix the code first: rename, introduce a type, extract a named operation, split mixed responsibilities, or move the behavior to the owner of its state. Then reassess whether anything still needs a comment.

## Documentation

- Documentation covers what self-documenting code cannot express (previous section). If a comment would translate confusing code into English, fix the code instead.
- Document what code cannot express: why an owner exists, the boundary or lifecycle it protects, invariants, failure semantics, and deliberate non-responsibilities.
- Durable comments never reference plans, tickets, phases, branches, dates, or other project state; translate history into the standing contract that remains true in the code.
- Skip boilerplate: remove any comment that restates the name, lists methods, or could describe any similar class.
- Update or delete documentation in the same change that moves the behavior it describes; stale purpose documentation is worse than none because it sends changes to the wrong owner.
- Document and test only invariants the implementation actually provides. Verify a claim like "every X has exactly one Y" in the code before writing it down; when verification contradicts the claim, report the contradiction instead of recording the claim. Writing it down anyway turns a false statement into truth by repetition.

## Deviations

A deliberate deviation from this standard is acceptable only when it is explicit. Record in the review or the owning plan: which principle is being bent, why the default is insufficient here, what alternatives were rejected, what compensates for the risk, and when to revisit. Undocumented exceptions are design drift.

## Design Smells

Strong signals that the design needs correction:

- One class coordinates parsing, validation, policy, telemetry, and output construction.
- A wrapper chain renames the same blob without owning anything.
- The real rules live outside the objects that own the state.
- Important behavior rides in raw maps, untyped payloads, or many unrelated booleans.
- Domain logic calls filesystem, network, database, or framework APIs directly.
- Infrastructure exceptions leak upward unchanged, or failures are logged as message-only text.
- Behavior is testable only through large end-to-end flows.
- An inheritance hierarchy exists mainly for code reuse.
- New variation arrives as more conditionals in a generic class instead of a new owned policy.
- A refactor keeps old and new paths alive for the same primary behavior without a hard external reason.
- A pipeline cannot explain what each stage consumes, produces, or drops.
- Concurrency exists but ownership, limits, cancellation, and timeout behavior are invisible in the design.
- Comments are needed to translate confusing names, primitive plumbing, or dense conditionals into English.

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

Ground the answers in traced evidence from the current system: file-and-line facts about how the mechanism works today, including the alternatives the design rejects. When the mechanism is uncertain, trace it before designing against it; a design written ahead of the facts encodes guesses as contracts.

Use `design-obligation-gate.md` when the design will be implemented, expensively validated, reviewed for readiness, or called complete.

## The Final Rule

Do not ask whether a design uses the right pattern names. Ask whether responsibilities are clear, boundaries are honest, invariants are protected, runtime behavior is observable, failures are diagnosable, and change is local and safe. If those are true, the design is on the right track. If they are false, pattern vocabulary will not save it.
