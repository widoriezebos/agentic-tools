# Architecture Guidance

## Code Quality Baseline

For Java/Kotlin code changes:

### Readability & safety
- Prefer pure functions and side-effect isolation; avoid static mutable state and hidden globals.
- Fail fast with clear exceptions rather than returning null/sentinel values; validate inputs at boundaries.
- Use immutable collections/data classes where possible; avoid exposing internal mutable state.
- Treat self-documenting code as a design requirement: a maintainer should understand the main path from names, types,
  method shape, and local control flow without private context.
- Prefer intention-revealing names, named predicates/policies, and rich domain types over vague helpers, primitive
  plumbing, generic maps, or comments that merely translate confusing code.

### Error handling
- Map exceptions to domain-specific errors; avoid swallowing exceptions or logging without context.
- Keep logging single-level: log at the boundary, not at every layer; include correlation IDs where available.
- Do not use broad `catch (Exception)` unless followed by specific handling and rethrow with context.

### Design hygiene
- Enforce single responsibility: classes/functions should have one reason to change; extract helpers instead of long methods.
- Do not accept responsibility splitting as sufficient when the resulting behavior is still hard to follow locally or
  requires excessive file-hopping to find the real decision.
- Avoid duplication: reuse existing helpers/builders; consolidate shared logic instead of copy/paste.
- Keep threading/concurrency explicit: prefer executors/structured concurrency over ad-hoc threads; guard shared resources.

### Testing expectations
- New behaviour requires unit or integration coverage of success and failure paths; bug fixes add a failing test first.
- Avoid mocking internals; mock only IO boundaries and use real domain objects for behaviour-heavy code.

---

## Ports & Adapters Guardrails

Keep hexagonal boundaries intact when adding services, controllers, or infrastructure adapters:

### Boundary rules
- Domain/application services must not depend on HTTP clients, file/OS access, logging frameworks, or Spring/DI annotations; depend on ports/interfaces only.
- Controllers/resolvers/adapters translate transport DTOs into domain models and map domain errors back to transport-friendly responses; never leak web/GraphQL types into the domain.
- Keep side effects behind ports: do not pass raw `Path`, `HttpClient`, or database clients deep into domain logic.
- Avoid static singletons and shared mutable state; prefer constructor injection with explicit dependencies.

### Change management
- When moving ports or adding adapters, update ArchUnit/module tests so the boundary stays enforceable.
- Map infrastructure exceptions into domain-level errors at the boundary instead of leaking client-specific ones upward.
- Keep services small and cohesive; if a use case needs multiple collaborators, wrap them behind a facade/aggregator instead of widening public APIs.
