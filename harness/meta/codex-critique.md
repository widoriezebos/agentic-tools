# Codex Critique of the Agent Harness

## Executive Summary

The harness is a strong compact engineering baseline, but it is not yet the universal project template it aims to be. It handles verification and stuck investigations well; ambiguity, durable learning, Domain-Driven Design (DDD), and refactor mode are materially incomplete.

The structural validation passes, but it currently proves internal consistency more than behavioral effectiveness.

## Findings

### 1. Critical: Refactor mode does not exist

`refactor` only appears as a routing trigger to the general design principles in `wow.md`. There is no workflow defining:

- preservation of externally observable behavior;
- characterization tests before structural changes;
- dependency and ownership mapping;
- incremental seams and checkpoints;
- deletion of superseded paths;
- architectural fitness checks;
- explicit separation of mechanical, structural, and semantic changes;
- a stop condition for a refactor.

Consequently, an agent asked to “refactor this” receives general design advice but no operational mode. This is the largest gap against the intended outcome.

Recommendation: add a triggered `skills/refactor/SKILL.md`, with modes such as `characterize`, `restructure`, and `migrate-boundary`. Require a refactor contract: preserved behavior, intended structural improvement, affected owner or boundary, characterization proof, forbidden semantic changes, incremental checkpoints, and completion and deletion criteria.

### 2. High: DDD coverage is architectural hygiene, not Domain-Driven Design

The design standard says to separate domain policy from orchestration and infrastructure, give behavior an owner, and make invariants explicit. Those are good foundations, but they do not ensure DDD.

Missing concepts include:

- discovering and naming bounded contexts;
- ubiquitous language and terminology conflicts;
- aggregates and transaction boundaries;
- entities versus value objects;
- domain services versus application services;
- domain events and integration events;
- repositories as domain-facing abstractions;
- context maps and upstream/downstream relationships;
- anti-corruption layers;
- explicit rules against anemic domain models;
- tests expressed in domain language.

“Bounded context” appears once, but in an observability sense, not as a DDD boundary.

Recommendation: add a concise `docs/design/domain-driven-design.md`, triggered only for domain modeling, new capabilities, boundary changes, or consequential refactors. Do not force aggregate ceremony onto CRUD or infrastructure projects. Adaptation should explicitly classify the repository as domain-rich, domain-light, or infrastructure/tooling.

### 3. High: Ambiguity is handled only after it blocks progress

The only meaningful ambiguity protocol is to ask when a reserved or ambiguous decision blocks progress. This is too late and too coarse.

There is no early distinction among:

- discoverable ambiguity: inspect code and conventions;
- low-risk ambiguity: make and state a reversible assumption;
- outcome ambiguity: propose an interpretation and success criterion;
- consequential ambiguity: ask before proceeding;
- scope ambiguity discovered during work;
- conflicting evidence or instructions.

Agents can therefore either ask too much or silently choose a direction that materially changes the outcome.

Recommendation: put a small ambiguity ladder in the always-loaded contract:

1. Resolve ambiguity from repository evidence.
2. For reversible choices, make the smallest assumption and state it.
3. For choices affecting contracts, scope, data, architecture, or user-visible behavior, stop and ask with a recommendation.
4. Convert the chosen interpretation into an observable acceptance criterion.

### 4. High: The learning rule conflicts with review-only scope

The harness correctly says review and explanation tasks should not mutate files. But it also requires every user correction to be recorded in an owning document.

During a review-only task, that second instruction authorizes—or appears to require—an unrequested repository edit. It also risks turning conversational preferences into permanent project policy after one correction.

Recommendation: distinguish:

- apply now in conversation;
- propose durable capture in a completion report;
- persist automatically only during an implementation task and when the correction is clearly repository-wide;
- require confirmation for uncertain or high-impact policy changes.

This would preserve learning without violating task scope.

### 5. High: “Do not forget” is only partially supported

The handoff mechanism is useful, but narrow. It operates only for work expected to span sessions and is deleted when the stream ships.

The harness lacks durable owners for:

- architectural decisions and their rationale;
- domain vocabulary;
- context boundaries;
- accepted project conventions;
- recurring failure patterns;
- rejected architectural approaches and reopening conditions;
- a lightweight task decision log for complex single-session work.

“Promote durable lessons into code/docs” is directionally right, but there is no routing destination beyond general project rules.

Recommendation: define explicit durable stores:

- `docs/domain/` for vocabulary and context maps;
- `docs/decisions/` for short architectural decision records;
- `docs/project-rules.md` for stable operational conventions;
- task ledgers for temporary hypotheses and dead ends;
- code and tests for executable invariants.

Do not attempt broad “agent memory.” Durable, scoped ownership is more reliable.

### 6. Medium: Anti-rabbit-hole behavior triggers too late

The `take-a-step-back` skill is the strongest part of the harness. It has useful cycle contracts, evidence requirements, and stop-loss rules.

However, it is triggered when work is already stuck, repetitive, expensive, or uncertain. Ordinary implementation work has no lightweight up-front budget or definition of progress. An agent can wander for quite a while before recognizing that the specialist workflow applies.

Recommendation: add a small always-on preflight for nontrivial tasks:

- observable outcome;
- non-goals;
- next proof-producing action;
- maximum exploration before reassessment.

Reserve the full skill and ledger for actual investigations.

Also clarify the apparent mismatch between `no-progress: stop stacking changes` and “two no-progress cycles” as a stop trigger. The practical rule should be: one no-progress result blocks another attempt in the same mechanism; two total no-progress cycles end the investigation.

### 7. Medium: Validation overstates what has been proven

The validation scripts pass, but mostly check:

- required paths exist;
- skill frontmatter is valid;
- word count is below a threshold;
- obligation matrices contain allowed status strings.

They do not verify:

- all Markdown links resolve;
- routing entries have matching assets;
- adaptation removes `meta/` and optional skills correctly;
- placeholders in `docs/project-rules.md` were replaced after adoption;
- DDD, refactor, and ambiguity requirements exist;
- correction capture respects task scope;
- a model actually triggers and follows the skills.

There is also stale or incorrect proof in `meta/harness-design.md`:

- it claims 411 always-loaded words; the current validator reports 595;
- `OBL-TRACE` cites `docs/source-analysis.md`, but the file is under `meta/source-analysis.md`;
- it claims presence and link checks, but `validate-harness.sh` does not check that meta reference.

The matrix marks these obligations `DONE`, illustrating precisely why structural status checks are not semantic proof.

### 8. Medium: Adoption is manual and easy to get subtly wrong

`docs/project-adaptation.md` provides sensible instructions, but a template used for every project should automate or verify them.

Not currently enforced:

- removing `meta/`;
- excluding unused optional skills;
- replacing project placeholders;
- configuring artifact ignore paths;
- installing runtime adapters;
- recording the project’s domain profile;
- selecting applicable design principles;
- proving project commands actually run.

Recommendation: add an idempotent bootstrap or adaptation script plus an “adopted project” validation mode distinct from harness-maintenance validation.

## What Is Already Good

Keep these parts:

- The small root contract and single routing index are excellent.
- “One rule, one home, one owner” is the right organizing principle.
- The verification skill properly distinguishes observed runtime proof from green tests.
- The stop-loss skill is unusually strong and evidence-oriented.
- Project-specific facts are correctly separated from portable workflow guidance.
- The obligation matrix is useful for risky changes when treated as a reasoning aid rather than proof.
- The first-week outcome receipt is a good empirical feedback mechanism.

## Recommended Target Architecture

Evolve the harness rather than replace it:

```text
AGENTS.md
wow.md
docs/
  project-rules.md
  collaboration.md
  design/
    design-principles.md
    domain-driven-design.md
    design-obligation-gate.md
  domain/                  # created for domain-rich projects
    glossary.md
    context-map.md
  decisions/               # lightweight ADRs
plans/
skills/
  clarify/                 # optional; ambiguity for consequential work
  refactor/                # required missing mode
  take-a-step-back/
  verify/
scripts/
  adapt-harness.sh
  validate-harness.sh
  validate-adoption.sh
```

The always-loaded contract should stay small. Add only:

- the ambiguity ladder;
- a lightweight nontrivial-task contract;
- explicit routing to refactor and DDD guidance;
- a scope-safe learning rule.

## Overall Judgment

Current readiness: **good engineering assistant baseline, not yet the intended universal harness**.

| Objective | Assessment |
| --- | --- |
| Avoid rabbit holes | Strong after drift is recognized; moderate preventively |
| Handle ambiguity | Weak |
| Learn from corrections | Moderate, but scope-conflicting |
| Do not forget | Moderate for unfinished streams; weak for durable knowledge |
| Apply DDD | Weak |
| Refactor mode | Missing |
| Verification discipline | Strong |
| Portability/adoption | Moderate |
| Machine-enforced guarantees | Narrower than claimed |

This critique is based on the complete harness contents and a successful run of `scripts/validate-harness.sh` from the harness root.

## Disposition (2026-07-16)

Reviewed against the harness change gate with an explicit anti-over-engineering constraint from the user.

- Implemented: #1 refactor mode (landed independently in the same session, before this review was read); #3 as a one-bullet ambiguity ladder in `AGENTS.md`; #4 scope-safe correction capture in `AGENTS.md` and `docs/collaboration.md`; #6's no-progress clarification in the step-back skill; #7's stale receipts amended in `meta/harness-design.md`.
- Deferred with named triggers, not rejected: #7 link/routing checks (first broken link or restructure regression); #8 adoption automation (observed friction during the first real adoption); #5 decision records (first project accumulating decisions worth indexing); #2 DDD guidance (first domain-rich adoption).
- Rejected: #6's always-on task preflight — ceremony without an observed failure; the design questions and default completion check already own that space.
