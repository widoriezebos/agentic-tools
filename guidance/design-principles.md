# Design Principles

## Purpose And Status

This document is the canonical design standard for this repository.

It defines how to build, evolve, and refactor systems in this codebase so they stay easy to understand, easy to extend,
robust in the ways that matter, and diagnosable under production pressure.

It applies to:

- new features
- architectural changes
- refactors of legacy areas
- design reviews and pull-request reviews

It is not a description of the current implementation. It is the design standard the implementation should move toward.

No design is literally future-proof. The goal is to make future change cheap, safe, local, and understandable.

## How To Use This Document

- Use it before introducing new classes, packages, or abstractions.
- Use it when deciding whether a refactor improves or worsens the system.
- Use it in reviews to evaluate ownership, boundaries, invariants, operability, and testability.
- Use the companion [Design Obligation Gate](./design-obligation-gate.md) whenever a design will be implemented,
  validated with expensive runtime evidence, or called complete.

This document is intentionally opinionated. It rejects design that is clever but hard to operate, and design that is
ceremonial but adds no real clarity.

## Quick Start: The Five Things That Matter Most

If you remember only five things, remember these:

- Give each important behavior one clear owner.
- Keep boundaries honest: domain rules stay in the domain, external concerns stay at adapters and ports.
- Make state, data flow, and failure modes explicit.
- Design observability, diagnosability, timeouts, and retries up front.
- Refactor toward cleaner ownership with tests and delete superseded code instead of preserving dual internal designs.

## When Principles Conflict

The principles in this document are not equal. Use this default priority order when trade-offs are real:

1. Correctness, safety, and invariant protection
2. Clear ownership and honest boundaries
3. Operability and diagnosability
4. Local reasoning and changeability
5. Measured performance and efficiency against real budgets
6. Ceremony minimization and stylistic purity

This ordering exists because a mission-critical system that is elegant but incorrect, opaque, or operationally fragile is
not well designed.

Example:

- If stronger observability appears to conflict with encapsulation, do not expose raw internals broadly just for
  debugging. Prefer owned diagnostic views, structured events, trace attributes, or explicit debug snapshots at
  boundaries. That preserves encapsulation while satisfying operability.
- If a measured hot path needs a lower-allocation implementation, optimize inside the owning component and document the
  reason. Do not use performance as a blanket excuse to ignore ownership and boundaries everywhere else.

## What Good Design Optimizes For

A well-designed system in this repository should make the following true:

- A newcomer can find where a behavior lives without tribal knowledge.
- A change usually has one obvious home.
- Business and domain rules are enforced where they belong, not scattered across helpers and adapters.
- State transitions and invariants are explicit.
- Infrastructure concerns stay at boundaries instead of leaking into domain logic.
- Failures are visible, actionable, and explainable from logs, metrics, traces, and structured runtime state.
- Tests can validate behavior at the level where it is owned.
- Large redesigns can happen by replacing or reshaping one responsibility at a time, not by editing one giant class.

## Pragmatism Over Dogma

These rules exist to produce systems that are easier to understand and safer to change. They are not an excuse to
inflate the design.

Apply the rules with judgment:

- Prefer one well-named, cohesive class over five ceremonial wrappers.
- Prefer direct composition over indirection introduced only to satisfy pattern vocabulary.
- Introduce interfaces when there is real variability, a true boundary, or a stable abstraction to protect.
- Split code when responsibilities are mixed, not merely because a slogan says smaller is always better.
- Avoid speculative generalization. Design for the next likely change, not every imaginable change.

The standard is strict about ownership, boundaries, invariants, and operability because those are the areas where
mission-critical systems usually fail.

## Deviations Must Be Explicit

This standard is governing guidance, but not every local optimum is identical.

A deliberate deviation is acceptable only when it is explicit and justified. When you intentionally break or soften a
principle, document in the PR or ADR:

- which principle is being bent
- why the default rule is insufficient here
- what alternatives were considered and rejected
- what compensating controls preserve safety, operability, and maintainability
- when the deviation should be revisited or removed

Undocumented exceptions are usually just design drift.

## External Reference Designs Must Become Yoda

External agent implementations are useful evidence for agent behavior, but Yoda must not combine raw upstream
sub-designs as independent internal fragments. When a design imports external reference behavior, the plan must first
translate it into one canonical Yoda ownership boundary, state shape, tool contract, provider contract, prompt contract,
and runtime trace contract.

This is mandatory because otherwise locally correct sub-solutions can fail when composed: one upstream transcript
lifecycle may not fit another upstream tool-result budget, one upstream tool semantic may not fit another upstream prompt
contract, and none of that is safe until the integrated behavior has a Yoda owner and Yoda-level invariants.

A mixed external reference design is not approved by citing one upstream for one part and another upstream for another.
It is approved only when the combined behavior has been reconciled into a single provider-neutral Yoda contract, with
any adapter-specific differences isolated behind that contract. Upstream names may appear in source-review evidence, but
Yoda implementation names, obligation ids, runtime events, tests, contracts, and behavior labels must be Yoda-owned.

Adhere by:

- naming the upstream behavior being adopted and the exact Yoda owner that makes it canonical;
- preserving behavior parity, not upstream naming or incidental implementation structure;
- defining the normalized Yoda data shape before implementation;
- recording a design-obligation row for the canonical Yoda contract whenever external reference lessons are combined;
- documenting upstream assumptions that conflict and the Yoda invariant that resolves them;
- proving that mixed upstream lessons compose through Yoda-owned tests and runtime evidence;
- deleting or replacing any Yoda-local logic that conflicts with the canonicalized behavior.

Avoid:

- layering upstream-shaped mechanisms beside each other without a single Yoda owner;
- accepting a design because each piece has a reference source while the composed Yoda behavior remains undefined;
- letting provider-specific or tool-specific details leak into generic Yoda infrastructure;
- treating multiple upstream parity claims as independently sufficient if the combined behavior has not been
  reconciled into Yoda.

## Core Principles

### 1. Responsibilities Are The Unit Of Design

Design around responsibilities, not around arbitrary layers, pipeline step names, or utility groupings.

Adhere by:

- giving each class or module one real reason to change
- keeping the state needed for a behavior with the object that owns that behavior
- making collaborators explicit and narrow
- grouping code by domain role or capability, not by convenience

Avoid:

- god classes
- helper or utility dumping grounds
- vague `Manager`, `Support`, `Misc`, or `Common` containers
- wrapper chains that only rename a large underlying blob

Practical rule:

- If a class owns unrelated decisions, many independent flags, or large amounts of plumbing state, it owns too much.
- A class approaching 1000 lines should be treated as a strong smell and usually must be split by responsibility.
- The line count is a heuristic, not a license. A 300-line incoherent class is already wrong, and a larger class needs
  explicit design justification.

Yoda-specific example:

- `ToolObservationParser`, `CoveragePlanner`, and `AnswerComposer` each own distinct responsibilities.
- `Executor -> Orchestrator -> Runner` around one giant implementation is not a domain model.

### 2. Keep Entrypoints Thin And Real

A use case may have a thin entrypoint that sequences collaborators. It must not become a disguised dumping ground.

Adhere by:

- having one real entrypoint for a use case when a single entrypoint improves clarity
- keeping orchestration explicit
- delegating owned behavior to responsibility-owning collaborators

Avoid:

- multiple façade layers around the same logic
- entrypoints rebuilding collaborator-local maps, sets, or decisions inline
- entrypoints that know internal details of every downstream component

Entrypoints may coordinate. They must not absorb the behavior they coordinate.

### 3. Encapsulation And Information Hiding Are Mandatory

An object owns its invariants and protects them.

Adhere by:

- exposing intention-revealing operations instead of raw internals
- keeping mutable state private to its owner
- returning immutable views or copies where needed
- preventing other modules from mutating data structures they do not own

Avoid:

- exposing internal collections for external mutation
- reaching into another object to manually keep its state consistent
- data bags that require outside code to enforce all business rules

If another component must know too much about an object's internal state to use it correctly, the ownership boundary is
wrong.

### 4. High Cohesion, Low Coupling

A module should know only what it needs for its job.

Adhere by:

- depending on narrow, intention-revealing collaborators
- passing domain types instead of primitive soups where semantics matter
- keeping dependencies pointed toward stable concepts
- making package boundaries reflect real ownership

Avoid:

- broad context objects passed everywhere
- free-floating `Map<String, Object>` or `List<Object>` plumbing across modules
- modules that know about unrelated concerns only because it is convenient

Coupling is not just about import counts. It is about how many unrelated facts a component must understand to do its
job.

### 5. Tell, Do Not Ask, When Behavior Is Owned

Move behavior to the object that has the information and responsibility to make the decision.

Adhere by:

- asking an object to perform its job
- letting the owner compute derived decisions from its own state
- keeping policies near the objects or modules that enforce them

Avoid:

- pulling raw state out of an object and making the real decision elsewhere
- orchestration code that reimplements collaborator logic through conditionals
- anemic models with all real behavior in services around them

Pragmatic exception:

- Simple transport DTOs, records, and projections may remain passive when they are only carrying data across a
  boundary.

### 6. Prefer Composition Over Inheritance

Inheritance is for true subtype relationships with preserved invariants, not for code reuse by default.

Adhere by:

- composing behaviors from focused collaborators, strategies, or policies
- using delegation when variation is orthogonal to the owning type
- preferring small policy objects over deep class hierarchies

Avoid:

- inheritance used only to share helper methods
- base classes that force unrelated subclasses to inherit state or lifecycle
- subtype trees where callers need `instanceof` to stay safe

If a subtype cannot stand in for its parent without surprising behavior, the inheritance model is wrong.

### 7. Use Polymorphism Only Where Variation Is Real

Interfaces and abstraction layers are useful when they protect a real boundary or represent genuine alternative
behaviors.

Adhere by:

- introducing interfaces at infrastructure boundaries and stable domain seams
- using strategy-style abstractions when behavior varies independently of the owner
- keeping interface contracts narrow and intention-revealing

Avoid:

- one interface and one implementation with no true boundary reason
- abstraction layers created only to satisfy a pattern checklist
- generic executor or framework classes that absorb domain-specific decision logic

Repository rule:

- Do not put tool-specific logic into generic infrastructure classes. Keep it in the owned tool implementation or in
  explicit configuration.

### 8. Model State, Lifecycle, And Transitions Explicitly

Hidden state is a major source of bugs and fear of change.

Adhere by:

- modelling important states and transitions explicitly
- naming transition preconditions and failure cases
- keeping mutable workflow state in clear owners rather than spreading it across helpers
- making retries, resumption, and terminal conditions first-class where they matter

Avoid:

- implicit mode changes hidden in booleans, strings, or shared mutable maps
- partially updated state spread across many collaborators
- state transitions that depend on call ordering nobody can see from types

Examples:

- prefer `Draft -> Active -> Archived` over a set of loosely related flags
- prefer a dedicated turn or workflow state object over scattered counters, sets, and scratch variables

#### Data Flow And Stage Contracts Must Also Be Explicit

The same rule applies to multi-step pipelines, agent loops, and evidence-processing flows.

Adhere by:

- giving each stage a clear contract: inputs, outputs, ownership, and termination behavior
- keeping data transformations typed and intention-revealing
- preserving provenance when later stages depend on evidence from earlier stages
- making partial-failure, timeout, and retry semantics explicit at stage boundaries

Avoid:

- shapeless payloads passed through many stages
- multiple steps mutating the same shared bag of state without ownership
- generic loops that hide what data is produced, consumed, or dropped at each step

### 9. Invariants Belong In The Domain Model

Validation at the boundary and invariant enforcement in the model are different jobs and both matter.

Adhere by:

- validating input shape and required fields at boundaries
- enforcing domain invariants inside the owning model or policy object
- making invalid states unrepresentable where practical
- failing fast with actionable errors when invariants are violated

Avoid:

- relying on database constraints or UI validation alone
- letting invalid state leak deeper and fail far from the source
- hiding domain failures inside generic infrastructure exceptions

Repository rule:

- Map infrastructure failures to domain-relevant errors at the boundary instead of leaking transport or client details
  upward.

#### Model Failure Taxonomy And Partial Failure Explicitly

Failure design is part of the object model and pipeline design, not an afterthought.

Adhere by:

- naming the important failure categories in the domain: validation, precondition, timeout, cancellation, dependency
  failure, rate limit or overload, partial failure, and internal invariant breach when those distinctions matter
- distinguishing retryable from terminal failures at the owner with the best information
- preserving the causal chain when mapping errors across boundaries
- representing partial success and partial failure explicitly in multi-step flows rather than collapsing everything into
  a generic error

Avoid:

- a single undifferentiated "operation failed" outcome for materially different failure modes
- retry loops that ignore whether a failure is safe to retry
- error translation that destroys the information needed for recovery, auditing, or debugging

### 10. Prefer Rich Domain Types Over Primitive And Collection Soup

Use named types when semantics matter.

Adhere by:

- introducing value objects for meaningful concepts, identifiers, or bounded values
- preferring immutable records and collections where shared mutation is not required
- using builders for construction when an object has many inputs or optional settings

Avoid:

- long argument lists of loosely related primitives
- parallel collections or parameter packs that callers must keep in sync
- untyped maps carrying important domain meaning

Repository rules:

- Use a builder pattern for classes with constructors that take 5 or more arguments.
- Do not use reflection in production or tests.

### 11. Keep Semantics Declarative And Choose Between Configuration And Model-Based Reasoning Deliberately

Semantics that change with domain, language, or archetype should not be hardcoded into generic logic.

Adhere by:

- putting stable pattern libraries, archetype metadata, manifests, and prompt contracts in configurable sources
- keeping code responsible for orchestration, control flow, state transitions, and validation
- using model-based reasoning only when the problem is genuinely ambiguous, language-sensitive, or too brittle to encode
  safely with explicit deterministic rules
- preferring deterministic configuration or typed code paths for stable, finite policies and lookups

Avoid:

- hardcoded domain-specific strings in generic selection, routing, or ranking logic
- English-only keyword rules, suffix heuristics, or morphology tricks in production logic
- configuration that tries to emulate language understanding through brittle keyword lists
- model calls for fixed deterministic lookups that should be explicit and testable

Repository rules:

- Build intent and pattern detection through data-driven or model-driven mechanisms, not hardcoded keyword branches.
- Do not encode language rules or domain-specific terminology in production logic or configuration for retrieval,
  routing, ranking, or selection.

### 12. Keep Boundaries Clean And Dependency Direction Intentional

The domain should not depend on delivery or storage mechanics.

Adhere by:

- translating HTTP, GraphQL, CLI, and persistence concerns at adapters
- depending on ports or owned abstractions at boundaries
- isolating file, network, database, and framework concerns from domain logic
- keeping dependency direction stable and obvious

Avoid:

- leaking transport DTOs into the domain
- passing raw infrastructure clients deep into business logic
- mixing orchestration, domain decisions, and I/O details in the same class

Repository rules:

- All file operations must go through the `Workspace` abstraction.
- Do not use direct `Files.*`, `Path.*`, or `new File()` access where `Workspace` is the correct boundary.
- Workspace-relative flat paths are the standard interface for file operations in workspace-owned logic.

### 13. Design For Concurrency, Retries, And Conflicts Explicitly

Mission-critical systems fail when concurrency and retry behavior are left implicit.

Adhere by:

- preferring immutability or single ownership for shared state
- making retry semantics explicit and idempotent where commands may be retried
- protecting contested state with clear ownership, versioning, or synchronization
- surfacing conflict and timeout behavior as explicit outcomes

Avoid:

- ad-hoc shared mutable state
- hidden thread-local assumptions
- silent overwrites on conflicting updates
- retry loops that can duplicate side effects

Clarity beats cleverness here. If concurrent behavior is important, it must be visible in the design.

Repository rules:

- Prefer explicit executors or structured-concurrency-style orchestration over ad-hoc thread creation.
- Use bounded parallelism, explicit timeouts, and cancellation contracts for concurrent work.
- Propagate correlation and tracing context across executors and virtual threads.
- Document the concurrency contract when it matters: ownership, thread safety, cancellation, shutdown, and retry
  behavior.

#### Performance Is A Design Constraint When Budgets Matter

Performance is part of design whenever latency, throughput, memory, cost, or token budgets are material to correctness
or usability.

Adhere by:

- measuring before optimizing
- designing hot or high-volume paths around bounded work, bounded memory, and bounded time
- isolating performance-sensitive code inside focused owners instead of smearing low-level concerns across the model
- documenting measured reasons when a cleaner abstraction must be bent for a real performance budget

Avoid:

- premature optimization
- using performance as a vague excuse to ignore ownership or boundary discipline
- unbounded accumulation, repeated parsing or serialization, or uncontrolled fan-out in hot flows
- performance fixes that make failures, timeouts, or backpressure harder to reason about

### 14. Operability Is Part Of The Design

A design is incomplete if the runtime cannot be understood when it fails.

Adhere by:

- logging errors once at meaningful boundaries
- logging exceptions with stack traces, not message-only fragments
- emitting stable, structured events and metrics for important outcomes
- propagating correlation or execution identifiers through important flows
- exposing enough explicit runtime state to explain why a decision was taken

Avoid:

- swallowing exceptions
- logging the same failure at every layer
- free-form log text with no stable keys
- opaque failure states that require guesswork or local reproduction

Repository rules:

- When catching exceptions, log the `Throwable` so stack traces are preserved.
- When structured execution logging exists, include full exception data rather than only `e.getMessage()`.
- New behavior should define the logs, metrics, or execution events that prove it worked and that explain failure.

Good design produces systems that are debuggable without heroics.

### 15. Test The Responsibility That Owns The Behavior

Test structure should mirror design structure.

Adhere by:

- keeping one or a few thin entrypoint tests for top-level sequencing and invariants
- writing focused module tests where the behavior actually lives
- mocking I/O boundaries, not behavior-heavy domain objects
- making tests deterministic and explicit about success, failure, retry, and timeout paths

Avoid:

- testing all behavior only through the top-level flow
- excessive mocking of internals
- nondeterministic tests driven by wall-clock timing or shared global state

Repository rules:

- Bug fixes should add a test that fails without the fix.
- Tests should cover negative and failure-mode behavior, not only happy paths.

### 16. Evolve Systems Through Clean Cuts

Refactoring should reduce complexity, not preserve it through compatibility scaffolding.

Adhere by:

- designing the final ownership model before moving code
- using final names and final package structure as early as practical
- assessing blast radius before cutover and making verification explicit
- preserving compatibility only at true external boundaries when needed
- deleting superseded code as soon as the new slice fully owns the responsibility

Avoid:

- feature flags, shadow modes, or dual-path primary behavior changes
- temporary namespaces that exist only to be renamed later
- migration wrappers that preserve internal mess for too long
- keeping old and new behavior alive in parallel without a hard reason

Repository rules:

- For internal design corrections, prefer direct cutovers with obsolete code removed in the same change.
- If blast radius is high, make rollback operationally explicit through tests, observability, deployment rollback, or
  version rollback at the true external boundary rather than by keeping dual primary behaviors alive in the code.
- Do not use phased rollout logic or dual-path compatibility code for primary design corrections.

### 17. Name And Package By Role, Not By Convenience

Names and package structure should tell the truth about ownership.

Adhere by:

- using names that express the behavior or policy the type owns
- grouping code by responsibility or bounded context
- keeping cross-cutting infrastructure separate from domain ownership

Avoid:

- `util`, `misc`, `support`, or `model` groups that hide unrelated concepts
- `FooService` when the class actually owns a specific policy, parser, planner, ledger, or composer role
- packages that grow without a clear ownership rule

Practical rule:

- Use `*Service` only when the type really is a cross-cutting service rather than a more precise domain role.

### 18. Code Must Be Locally Readable And Self-Documenting

Responsibility-driven design is not complete if the resulting code is hard for a normal maintainer to read.

A future maintainer who knows Java and the domain vocabulary, but not the current incident or design history, should be
able to understand the main behavior from names, types, method shape, and local control flow without reconstructing a
hidden protocol from scattered helpers.

Adhere by:

- using names that describe the behavior, decision, state, or invariant being owned
- keeping each method at one clear level of abstraction
- making the main path read top-to-bottom before implementation details
- replacing boolean soup, primitive packs, and generic maps with intention-revealing types
- naming complex predicates, policies, and transitions instead of embedding dense conditions inline
- making side effects, retries, failure outcomes, and mutation visible in method names, result types, or state types
- keeping comments focused on why, constraints, tradeoffs, invariants, or non-obvious runtime facts
- using focused tests as executable documentation for the behavior owned by the class

Avoid:

- code that is only understandable by jumping through many tiny forwarding classes
- clever chains of abstractions where no file clearly owns the real decision
- vague verbs such as `process`, `handle`, `execute`, `manage`, or `build` when a more specific responsibility exists
- comments that merely translate confusing code into English
- deeply nested conditionals whose business meaning is not named
- methods that mix orchestration, policy decisions, data transformation, logging, and error handling at the same level
- extracting helpers that hide complexity instead of naming a real concept

Practical rule:

- If a reviewer needs a paragraph of explanation to understand what a method does, first improve the names, types, and
  structure. Add a comment only for intent that cannot be expressed cleanly in code.
- If the reader must bounce through several files to find the actual decision, the ownership boundary or abstraction
  shape is probably wrong.
- A design is not done until the code is readable locally, not merely responsibility-partitioned globally.

## Building New Features

When adding new behavior, use this sequence:

1. Define the outcome.
   - What user, business, or system behavior is being added?
   - What must be true when it succeeds, and what must be prevented?

2. Identify the responsibility owners.
   - Which object or module should own each rule, decision, or state transition?
   - Which parts are orchestration, which parts are policy, and which parts are boundary adaptation?

3. Define the boundary.
   - What enters from transport, persistence, filesystem, or external services?
   - Where is translation into domain types performed?

4. Model state and invariants.
   - What states exist?
   - What transitions are allowed?
   - What makes an input or transition invalid?

5. Shape the collaborators.
   - Keep dependencies narrow.
   - Introduce interfaces only when there is real variation or a true boundary.
   - Use builders or value objects where construction would otherwise become error-prone.

6. Define data flow.
   - What are the major stages?
   - What does each stage consume, produce, and guarantee?
   - Where are provenance, timeout, retry, and partial-failure semantics made explicit?

7. Define failure and operability.
   - What does failure look like to callers?
   - Which failure categories are retryable, terminal, partial, or cancellable?
   - Which logs, metrics, traces, and execution events are needed?
   - How will a production engineer explain a bad outcome?

8. Define the tests.
   - Which module owns the behavior?
   - What unit or integration tests prove success, failure, retry, timeout, and edge cases?

9. Check performance only where it is material.
   - What are the real latency, throughput, memory, or token budgets?
   - Do not optimize without measurement.

10. Stop when the design is simple and honest.
   - Do not add abstraction just because it might be useful later.
   - Do not keep complexity hidden in a single "coordinator" because splitting feels inconvenient.
   - Do not accept responsibility splitting as sufficient if the resulting code is still hard to read locally.

## Refactoring Legacy Areas

Use refactoring to improve ownership and clarity, not to reshuffle names.

1. Characterize current behavior first.
   - Capture existing success paths, failure paths, invariants, and observability signals before moving code.

2. Find the real responsibility seams.
   - Separate orchestration, parsing, policy, persistence, telemetry, validation, and presentation if they are mixed.

3. Design the target shape up front.
   - Choose the final owners, names, and dependency direction before editing broadly.

4. Move behavior to the correct owners.
   - Put state with the responsibility that owns it.
   - Stop reimplementing collaborator logic inline.

5. Cut over cleanly.
   - Switch to the new ownership model in one pass when feasible.
   - Keep compatibility only where an external caller truly depends on it.
   - If blast radius is high, make rollback and verification explicit without preserving dual primary behaviors in the
     runtime design.

6. Delete superseded code.
   - Do not leave the old path in place "just in case" once the new owner is authoritative.

7. Re-verify tests and operability.
   - Ensure the new design is easier to test and at least as observable and diagnosable as before.

8. Prefer meaningful improvement over theoretical perfection.
   - Each refactor should clearly improve ownership, clarity, or safety.
   - Do not churn names or packages without a real design gain.

## Design Smells And Red Flags

The following are strong signals that the design needs correction:

- A class coordinates exploration, parsing, validation, telemetry, and answer construction in one place.
- A wrapper chain exists only to rename the same blob.
- The real rules live outside the objects that own the state.
- Important behavior depends on raw maps, untyped JSON-shaped structures, or many unrelated booleans.
- Domain logic depends directly on filesystem, HTTP, database, or framework APIs.
- Infrastructure exceptions leak upward unchanged.
- Failures are logged as message-only text without stack traces or correlation context.
- Behavior is testable only through large end-to-end flows.
- An inheritance hierarchy exists mainly for code reuse.
- New variation is added by more conditionals in a generic class instead of by a new owned policy or collaborator.
- Packages like `util`, `support`, `misc`, or `model` become the default home for code that has no owner.
- A class name ends in `Helper`, `Manager`, or `Service` because the real role was never clarified.
- The design relies on domain-specific hardcoded strings or English-only heuristics in generic logic.
- A refactor preserves both old and new paths for the same primary behavior without a hard external-compatibility reason.
- Failure outcomes do not distinguish retryable, terminal, timeout, cancellation, or partial-failure cases where the
  distinction matters.
- A pipeline or loop cannot explain what data is consumed, produced, or lost at each stage.
- Concurrency exists but ownership, cancellation, parallelism limits, or timeout behavior are not obvious from the
  design.
- The real behavior is split across many small methods or classes, but no local reading path explains the decision.
- Comments are required to translate confusing names, primitive plumbing, or dense conditionals into English.

## Review Checklists

### Design Review Checklist

- Is the ownership model clear?
- Does each class or module have one real reason to change?
- Are invariants enforced by the owning model or policy?
- Is the dependency direction clean and boundary-respecting?
- Is important mutable state explicitly owned?
- Are data flow, stage contracts, and partial-failure semantics explicit where relevant?
- Are concurrency, cancellation, and timeout semantics explicit where relevant?
- Are failure modes and operational signals designed, not left implicit?
- Is performance driving the design because of a measured budget rather than guesswork?
- If a principle is being bent, is the deviation explicit and justified?
- Can a newcomer find where the behavior lives?
- Can a newcomer follow the main behavior locally without private context or excessive file-hopping?

### New Feature Checklist

- Is there one obvious home for the new behavior?
- Are domain rules separated from transport, persistence, and I/O concerns?
- Are interfaces introduced only where they buy real value?
- Are state transitions, retries, and conflicts explicit where they matter?
- Are data flow and stage contracts explicit where the feature spans multiple steps?
- Are logs, metrics, and error shapes defined?
- Are retryable, terminal, timeout, cancellation, and partial-failure outcomes modelled where they matter?
- Are focused tests added at the owning responsibility?
- Does the implementation read clearly from names, types, method shape, and local control flow?

### Refactor Checklist

- Does the new structure improve ownership instead of only moving code around?
- Are old façade layers, wrappers, or dumping grounds removed?
- Is compatibility preserved only at genuine external boundaries?
- Is rollback explicit for high-blast-radius changes without preserving two primary internal designs?
- Was superseded code deleted in the same cutover?
- Did test structure move closer to responsibility ownership?
- Did observability stay intact or improve?
- Did readability improve for a maintainer who did not participate in the refactor?

### Self-Documenting Code Prompt

Before implementing or approving code, apply a readability pass:

Read the changed code as a future maintainer with no private context. Verify that each class has one obvious reason to
exist, each public method name says what outcome it provides, and each important decision is expressed through named
types, named predicates, or owned policy objects.

Do not accept code merely because responsibilities were split. Reject designs where the real behavior is hidden behind
forwarders, vague helpers, generic managers, primitive or map plumbing, or dense conditionals.

Prefer clearer names, smaller coherent methods, richer domain types, and direct ownership over explanatory comments. Use
comments only for why, invariants, constraints, and tradeoffs that the code cannot express by itself.

The code passes only when a normal maintainer can find the owner, follow the main path, understand the state changes, and
identify where to safely change behavior.

## Worked Examples In This Repository

The repository already contains useful concrete examples of these principles:

- [chat-target-model.md](./chat-target-model.md) is a worked example of responsibility-driven OO redesign for a complex
  subsystem. It demonstrates clear ownership, explicit state, thin real entrypoints, narrow dependencies, and clean-cut
  refactoring.
- [chat-domain-audit.md](./chat-domain-audit.md) shows how to audit package boundaries, ownership decisions, and naming
  honesty against this standard.
- [architecture.md](./guidance/architecture.md) captures baseline architecture and
  ports-and-adapters guardrails, especially boundary discipline and narrow dependencies.
- [observability.md](./guidance/observability.md) captures operational expectations
  that are part of good design, not an afterthought, especially error visibility, tracing, and metrics.
- [testing.md](./guidance/testing.md) captures the regression safety net expected of
  architectural and behavioral changes, especially success, failure, retry, and timeout coverage.

Use those documents as companion references. This document remains the governing standard for the design philosophy
behind them. If a companion document diverges from this one, this document wins and the companion document should be
corrected.

## Repository-Specific Rules Index

The following repository rules are embedded throughout this document and are intentionally mandatory:

- Use `Workspace` for workspace-owned file access; do not bypass it with direct filesystem APIs.
- Use a builder for constructors with 5 or more arguments.
- Do not use reflection in production or tests.
- Log exceptions with stack traces and preserve full exception data in structured execution logs where available.
- Keep tool-specific logic out of generic infrastructure classes.
- Do not hardcode domain-specific terminology, English-only heuristics, or morphology rules in generic logic.
- Prefer direct cutovers for internal design corrections; do not keep dual primary behaviors alive for the same runtime
  responsibility.
- Use the [Design Obligation Gate](./design-obligation-gate.md) before implementation, before expensive validation, and
  before claiming a design implementation complete.

## Final Rule

Do not ask whether a design uses the right pattern names. Ask whether:

- responsibilities are clear
- boundaries are honest
- invariants are protected
- runtime behavior is observable
- failures are diagnosable
- change is local and safe

If those are true, the design is probably on the right track. If those are false, pattern vocabulary will not save it.
