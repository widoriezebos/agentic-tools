# Documentation Guidelines

## Purpose

This document defines when and how to add code-level documentation in Yoda, with class-level documentation as the main
focus.

The goal is not to increase documentation volume. The goal is to make consequential classes easier to understand,
change, review, and operate by explaining the system context that names, types, method shape, and local control flow
cannot reasonably carry by themselves.

The expected documentation is deep behavior-and-purpose documentation, not boilerplate summaries. It should explain the
class's reason for being, behavioral responsibility, system role, and durable contract. It should not merely restate what
the declaration is named, list methods, or provide generic Javadoc that could apply to any similar class.

Code documentation should help a maintainer answer questions that the class name, method names, and types cannot answer
quickly enough:

- What responsibility does this class own?
- Why does this class exist instead of being inline code, a simple data type, or part of another owner?
- How does the rest of Yoda use it?
- Which stable subsystem, entrypoint, or caller role depends on this class, at what lifecycle point, and why does that
  dependency exist?
- What invariant, boundary, lifecycle, policy, or failure behavior must be preserved?
- What nearby responsibility does this class deliberately not own?

If documentation does not improve one of those answers, prefer no documentation, a clearer name, a richer type, or a
cleaner structure.

## Governing Standard

These guidelines follow the repository design standard in `docs/design/design-principles.md`.

Class documentation should reinforce the same design goals:

- one clear owner for each important behavior;
- honest boundaries between domain rules, infrastructure, adapters, and orchestration;
- explicit state, lifecycle, transitions, and invariants;
- visible failure behavior and diagnosability;
- tests and changes that target the responsibility that owns the behavior;
- clean cutovers instead of parallel undocumented paths.

Documentation must describe the responsibility and contract proven by the code. It must not promise behavior the code
does not implement.

## Self-Documenting Code Baseline

Yoda code should be self-documenting first. A maintainer should understand the main path from intention-revealing names,
domain types, method shape, package ownership, and local control flow without private context.

Documentation must not compensate for code that is vague, over-general, or hard to follow. If a comment is needed to
translate confusing code into English, improve the code before adding the comment.

Prefer code changes over documentation when the issue can be solved by:

- renaming a class, method, field, variable, predicate, or policy to reveal the owned behavior;
- replacing primitive values, boolean flags, generic maps, or loosely typed strings with domain types;
- extracting a named predicate, policy, or transition that represents a real concept;
- splitting mixed responsibilities into honest owners;
- moving behavior to the object that owns the state or invariant;
- reshaping a method so the main path reads top-to-bottom at one level of abstraction.

Use documentation for information that code cannot express cleanly:

- why this owner exists;
- which boundary or lifecycle point this class protects;
- what invariant, failure behavior, concurrency rule, or operational signal must be preserved;
- what callers rely on beyond the method signature;
- what nearby responsibility is intentionally owned elsewhere.

## Code Documentation Scope

These rules apply to Javadoc, block comments, and other durable comments in production and test code.

Class-level documentation explains the reason for a class's existence in the system. Method-level documentation explains
a method contract that callers must understand to use or change the method safely.

Use method-level documentation when a method has a non-obvious contract, such as:

- required call ordering;
- preconditions that cannot be expressed in the type signature;
- side effects on owned state, external systems, execution logs, telemetry, or workspace content;
- failure, retry, timeout, truncation, or exception translation semantics;
- concurrency or idempotency expectations;
- return semantics that are easy to misuse;
- package-private workflow entrypoints used by neighboring collaborators.

Skip method-level documentation when it only repeats the method name, parameters, return type, or obvious implementation
steps. Prefer clearer names, narrower types, and simpler method shape over comments that explain a confusing signature.

## No Plan Or Stateful References

Code documentation must be about the code and the code-owned contract alone.

Do not reference implementation plans, design-obligation ledgers, refactor checkpoint ledgers, issues, PRs, branch names,
benchmark runs, frontier tags, dates, temporary phases, migration states, or other stateful project artifacts from
Javadoc or durable code comments.

Those artifacts can explain how the code got here, but they are not the code's contract. If a historical artifact
matters, translate it into the stable invariant, boundary, failure behavior, or ownership rule that remains true in the
code.

Prefer:

> It rejects repository-qualified paths before workspace resolution so file access cannot escape the selected
> repository boundary.

Avoid:

> Added for the repository path plan; keep in sync with the phase 2 migration ledger.

When historical context is still useful for reviewers, keep it in the owning plan, ledger, PR description, or tracked
issue. Do not put it in code documentation.

## Classification

Add class-level documentation to every class of consequence. Use this classification before editing a package so the
documentation pass stays consistent across agents and reviewers.

### Mandatory

Document a class when changing or reviewing it if it owns or protects any of the following:

- a domain behavior, policy, or invariant;
- an orchestration step in a user-visible or model-visible workflow;
- a boundary between domain code and infrastructure;
- a provider, repository, workspace, tool, servlet, CLI, or persistence contract;
- mutable state, lifecycle state, concurrency behavior, retry behavior, timeout behavior, cleanup behavior, or ownership
  transfer;
- security, path validation, repository access, workspace isolation, authentication, authorization, or data exposure
  rules;
- telemetry, execution logging, diagnostic state, audit output, or failure translation;
- prompt, model-turn, retrieval, ranking, scoring, source-recall, tool-execution, answer-synthesis, or budget behavior;
- behavior that is package-private but still important to a subsystem's correctness.

### Usually Document

Document unless the class is genuinely self-explanatory:

- parsers, planners, composers, resolvers, scorers, rankers, coordinators, gates, executors, validators, and adapters;
- factories that encode source priority, ownership, lifecycle, or dependency-selection rules;
- support classes with more than one caller or any non-obvious policy;
- classes whose names contain broad role words such as `Manager`, `Coordinator`, `Support`, `Service`, `Helper`, or
  `Util`;
- records, enums, and exceptions that carry system meaning.

Examples of records, enums, and exceptions that normally deserve documentation:

- a record that is a cross-boundary payload or durable state shape;
- an enum that represents a state machine, policy mode, or externally meaningful option;
- an exception that signals a specific failure contract another component handles.

### Usually Skip

Do not add class-level documentation for documentation's sake.

Usually skip documentation for:

- simple beans;
- trivial DTOs, request objects, response objects, and projections;
- records whose fields fully explain the type;
- obvious enums with self-explanatory constants;
- constants-only holders;
- test fixtures and small test data builders;
- one-method wrappers whose purpose is clearer from the method and type names than from a comment.

If the only honest documentation would be "Represents X", "Handles Y", or "Utility methods for Z", do not add that
documentation. Improve the name or leave the class undocumented.

### Borderline Decision Tree

When the classification is unclear, answer these in order:

1. Would a wrong change to this class break a workflow, boundary, invariant, security property, runtime diagnosis, or
   model-visible behavior? If yes, document it.
2. Does the class decide anything that callers rely on but cannot see from the public method names? If yes, document it.
3. Does the class mainly carry data with no local policy or lifecycle? If yes, usually skip it.
4. Would a clearer name, domain type, package boundary, or method shape make the proposed comment unnecessary? If yes,
   improve the code instead of documenting around it.
5. Would the proposed comment only restate the class name or list methods? If yes, skip the comment and consider a
   clearer name.
6. Is the class hard to document because it has mixed responsibilities? If yes, record the design issue and avoid using
   documentation to hide the ownership problem.

## Required Content

A useful class-level comment should usually cover these points.

### Responsibility

State what the class owns in system terms.

Prefer:

> Owns the decision that turns an execution request and effort profile into the budgets used by exploration tools.

Avoid:

> Handles budget logic.

### Purpose

Explain why this class exists as a separate owner.

Useful purpose statements describe what complexity, invariant, boundary, or lifecycle would otherwise be scattered.

Prefer:

> This class keeps budget derivation separate from tool execution so tool runners consume an already resolved policy
> instead of reinterpreting effort settings.

Avoid:

> This class exists to make the code cleaner.

### System Usage

Describe how other parts of Yoda use the class at the subsystem level.

For consequential classes, this is part of the class's reason for being. A useful usage statement explains the stable
subsystem, entrypoint, or caller role that relies on the class, the lifecycle point where it is used, and the purpose of
that dependency. The goal is to show why the class belongs in the system, not to create a caller index.

Use stable roles, not fragile call-site inventories. Specific class names are acceptable when they are the durable owner,
entrypoint, adapter, or public integration point that gives the usage its meaning. Avoid listing incidental helpers,
private call paths, or every current caller.

Prefer:

> Chat execution calls this before model turns are built, and tool budget gates consume the resulting limits during
> exploration.

Avoid:

> Called by `FooService`, `BarRunner`, and `BazController`.

Also avoid usage statements that name a caller without explaining the purpose of the relationship.

Prefer:

> Provider adapters use this after each tool batch so they can make a provider-neutral continuation decision before
> adding another model turn.

Avoid:

> Used by `OpenAIChatAssistant`, `AnthropicChatAssistant`, and `BedrockClaudeAssistant`.

### Invariants And Failure Semantics

Document rules that must remain true for the class to be safe to change.

Examples:

- ordering requirements;
- idempotency expectations;
- ownership or locking rules;
- path-safety or workspace-boundary rules;
- budget, timeout, retry, or truncation rules;
- what happens when parsing, I/O, provider calls, or validation fails;
- whether errors are propagated, translated, logged, or recorded as execution evidence.

### Non-Responsibilities

Add a non-responsibility statement when it prevents likely scope creep.

This is useful for boundary classes, orchestration classes, adapters, and classes with tempting names such as
`Manager`, `Coordinator`, `Support`, or `Service`.

Prefer:

> It does not decide which tools are allowed; it only applies the resolved tool policy to the current turn.

Avoid non-responsibility sections when the boundary is already obvious.

## Recommended Shapes

Use short, specific Javadoc. One to three paragraphs is usually enough.

The following is one acceptable shape, not a required template. Omit any sentence that does not add real design
information for the class being documented.

```java
/**
 * Owns <specific responsibility> for <specific system flow>.
 *
 * <p>This class exists so <design reason, boundary, invariant, or lifecycle rule>. It is used by
 * <subsystem or stable caller role> when <important lifecycle point>, and delegates <separate responsibility> to
 * <collaborator role>.
 *
 * <p>It must preserve <key invariant, failure behavior, or ordering rule>. It does not <nearby responsibility>.
 */
```

Use bullet lists only when the class has several real responsibilities, modes, or invariants that would be harder to
scan in prose.

For simple consequential classes, a shorter shape is often better:

```java
/**
 * Owns <specific responsibility> at <specific boundary or lifecycle point>.
 *
 * <p>Callers rely on this class to preserve <key invariant or failure behavior> before <downstream subsystem action>.
 */
```

For an adapter or boundary class, emphasize translation and ownership limits:

```java
/**
 * Translates <external/system input> into <Yoda-owned contract> for <subsystem>.
 *
 * <p>It isolates <external concern> from <domain owner>. It must <boundary or failure rule>, and it does not
 * <domain decision owned elsewhere>.
 */
```

## Review Gate

Before accepting a class-level comment, ask whether it helps a maintainer answer at least one of these questions faster:

- Is this information impossible or unreasonable to express through better names, types, package ownership, or local
  structure?
- Where does this behavior belong?
- Why is this class the owner?
- What should not be added to this class?
- What invariant would a change risk breaking?
- Which subsystem relies on this contract?
- Does the documentation explain why that subsystem relies on this class, rather than only naming a caller?
- What failure behavior should callers expect?
- What operational signal or execution evidence should exist when this class fails?

If the answer is "none", remove the comment.

If the first answer is "no", improve the code and then reassess whether any documentation remains necessary.

## Smells

Documentation is suspicious when it:

- repeats the class name in sentence form;
- lists every method instead of explaining the class purpose;
- repeats a method signature in prose without adding a caller contract;
- explains what unclear names, primitive values, boolean flags, or generic maps mean instead of improving them;
- explains control flow that should be readable from method shape and local structure;
- describes implementation steps without explaining the system contract;
- uses vague words such as "various", "common", "miscellaneous", "helper", or "utility" without a precise boundary;
- says "currently", "for now", "phase", or "temporary" to paper over design uncertainty;
- explains history that no longer constrains the current design;
- references plans, ledgers, tickets, PRs, branches, dates, benchmark runs, frontier labels, or other project state;
- records caller names that will become stale after a refactor;
- names a caller without explaining the stable subsystem role, lifecycle point, or purpose of that dependency;
- promises fallback, compatibility, shadow behavior, or feature-flag behavior that is not part of the current clean
  design;
- hides an ownership problem behind a broad class comment.

When a consequential class cannot be documented without vague language, treat that as design evidence. The class may
need a clearer name, a smaller responsibility, or a boundary split before documentation will be useful.

When documentation seems necessary because the code is not self-documenting, treat the missing readability as the first
problem. Add documentation only after the names, types, ownership, and method shape are as clear as the local design
allows.

## Maintenance Rule

Keep existing documentation synchronized with ownership and behavior changes.

When a patch changes any of the following, update the relevant class-level or method-level documentation in the same
patch:

- the class's owned responsibility;
- a method's caller-visible contract;
- the subsystem or lifecycle point that uses it;
- an invariant, state transition, ordering rule, concurrency rule, or cleanup rule;
- failure propagation, exception translation, logging, telemetry, or execution evidence;
- a boundary between domain, adapter, infrastructure, tool, provider, repository, workspace, servlet, or CLI code;
- a non-responsibility that moved into or out of the class.

When editing a class, read its existing documentation before changing code. Remove or rewrite documentation that:

- describes a responsibility the class or method no longer owns;
- explains code that is now self-documenting through better names, types, or structure;
- references a completed plan, migration, issue, PR, branch, date, benchmark run, frontier label, or other transient
  project state;
- documents implementation history instead of a code-owned invariant or contract;
- contradicts the behavior proven by the code and tests.

Stale purpose documentation is worse than no documentation because it sends future changes to the wrong owner.

## Recording Design Issues

Do not leave vague TODO comments in production code when documentation exposes an ownership problem.

Record the issue in the artifact that owns the current work:

- the active design plan or design-obligation ledger for design or implementation work;
- the refactor checkpoint ledger for refactor work;
- the PR description or review notes when the issue is discovered during review;
- a tracked issue only when the problem is outside the current change and needs separate scheduling.

The record should name the class, the unclear responsibility, the likely owner or boundary, and the reason documentation
would be misleading until the design is cleaned up.

The code documentation itself should not point back to that artifact. Once the issue is fixed, the code should describe
the resulting responsibility, not the plan that produced it.

## Examples

### Useful

```java
/**
 * Owns conversion from user-facing repository paths into workspace-qualified file access requests.
 *
 * <p>This class exists so servlet handlers and tool implementations do not each reinterpret repository prefixes,
 * default repository selection, or workspace-relative path rules. Callers use it before reading source content through
 * the file access boundary.
 *
 * <p>It must reject ambiguous or escaping paths before any filesystem-backed accessor is invoked. It does not perform
 * the read itself or decide whether the caller is authorized to read the target.
 */
```

### Not Useful

```java
/**
 * Utility class for path handling.
 */
```

The second comment adds no ownership, boundary, system usage, invariant, or failure information.

### Comment That Should Become Code

```java
// If the path starts with a repository alias, strip it before validating the remaining local path.
String normalized = normalize(input);
```

Prefer a named operation or type that carries the concept:

```java
RepositoryRelativePath path = RepositoryRelativePath.fromUserInput(input);
```

The second version makes the main path readable without translating primitive string manipulation through a comment.

### Plan Reference To Rewrite

```java
/**
 * Temporary path validation from the repository-access plan. Remove after phase 2.
 */
```

Rewrite this as the durable contract if the behavior still matters:

```java
/**
 * Rejects repository-qualified paths that cannot be resolved inside the selected workspace boundary.
 */
```

If the behavior is temporary, keep that scheduling detail in the owning plan or issue, not in code documentation.

### Orchestration Class

```java
/**
 * Sequences source discovery, source validation, and answer drafting for a single chat execution.
 *
 * <p>This class exists to keep the execution lifecycle visible while leaving retrieval policy, model-turn construction,
 * and final answer composition with their owning collaborators. Chat entrypoints use it after request scope resolution
 * and before execution logs are closed.
 *
 * <p>It must preserve the order of evidence capture before answer synthesis. It does not decide which sources are
 * relevant or how the final answer is worded.
 */
```

### State Holder

```java
/**
 * Tracks required-read failures observed during one execution.
 *
 * <p>Exploration and answer-synthesis gates use this state to distinguish missing required evidence from ordinary tool
 * failures. The state is intentionally bounded so diagnostic output remains useful without exposing an unbounded list of
 * paths.
 *
 * <p>It must be safe for concurrent tool activity to record failures. It does not decide whether execution may continue;
 * downstream gates interpret the recorded state.
 */
```

### Passive Record To Skip

```java
public record CreateChannelRequest(String name) {
}
```

Do not add class-level documentation here unless the request shape carries a non-obvious API contract. The type name and
field already explain the passive payload.

### Method Documentation

```java
/**
 * Records a required-read failure without throwing so later gates can report all missing evidence together.
 *
 * <p>The stored path list is bounded for diagnostics; the aggregate failure count still includes failures beyond that
 * bound.
 */
void recordFailure(String path, RequiredReadFailureReason reason, String message) {
  ...
}
```

This method-level documentation is useful because it explains caller-visible failure and truncation semantics. A comment
that only says "Records a failure" would not be useful.

## Applying This Across The Codebase

When documenting existing code, work by ownership area rather than alphabetically.

For each package or subsystem:

1. Identify the consequential classes first.
2. Read existing class and method documentation before changing code or adding new comments.
3. Read the class and its direct collaborators before writing documentation.
4. First check whether clearer names, richer types, smaller methods, or cleaner ownership would remove the need for the
   proposed comment.
5. Document only the responsibility that the code proves.
6. Remove or rewrite documentation that describes plans, migrations, issue state, branch state, dates, benchmark runs, or
   other transient project artifacts.
7. Remove comments that merely translate now-readable code into English.
8. Skip passive carriers unless they encode a meaningful system contract.
9. Treat vague documentation as a design smell and record the design issue in the owning plan, ledger, PR note, or
   tracked issue instead of adding filler.

The expected result is not that every Java file has a comment. The expected result is that every class with a real
system responsibility has documentation that explains its reason for being.
