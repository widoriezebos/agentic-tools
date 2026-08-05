# Project Rules

Replace this file when adopting the metasystem. Keep facts concrete and repository-specific.

## Project Map

- Adopted from template SHA: `<template sha>`. Filled by `scripts/adopt.sh`; future template migrations diff against it.
- Purpose: `<one paragraph>`
- Production entrypoints: `<paths>`
- Test roots: `<paths>`
- Generated files: `<paths and ownership>`
- Sensitive or protected areas: `<paths>`
- Durable evidence root: `<path outside the repository>`. Outside the repository and every build tree; holds the only-copy run evidence that must survive (rules in `plans/README.md`).

## Commands

- Fast focused test: `<command>`
- Full unit suite: `<command>`
- Integration/end-to-end suite: `<command>`
- Build/package: `<command>`
- Format/lint/typecheck: `<command>`
- Local run: `<command>`
- Refactor acceptance gate: `<command>`. The full behavior-preservation proof (full suite, benchmark, or golden run) that accepts a refactor candidate. State its cadence backstop if it differs from the defaults owned by `scripts/refactor-baseline.sh`.
- Improvement evaluation: `<command>`. The on-demand evaluation for improvement goals. State the evaluation type (deterministic, stochastic, hidden-information, or dynamic; see the improve skill), the primary metric and its direction (max or min), guard metrics with floors, the noise floor (minimum meaningful delta), any cheaper canary or subset variant, and the holdout or case-rotation policy.
- Frontier preservation policy: `<policy>`. How a new best-known state is preserved: tag pattern, push target, and who may move it.

State realistic timeouts and prerequisites here. Do not repeat these commands in skills or root instructions.

## Budgets

Where agents can spend real money (model calls, paid APIs, cloud runs), state the facts that make the spend governable:

- Spend fence, covering total spend across providers and agents rather than per run: `<amount and period>`
- Proactive warning threshold below the fence: `<warning threshold>`
- Who approves overage and resource-tier changes: `<who approves>`
- The authoritative usage source that spend is measured from (never estimates): `<usage source>`

## Local Invariants

List only rules that cannot be inferred from code or tooling and apply broadly in this repository. Prefer an executable check whenever the rule is binary.

- A commit is gated on the suite run that produced its verdict, in one shell chain: `scripts/validate-metasystem.sh && ... && git commit && git push`. Never read a verdict from a log tail, a previous shell, or a check whose failure does not stop the chain. Two pushes on a red suite happened in one day, both that way (IL-17).
- A receipt is appended in the same commit as the work it describes. Bookkeeping-only commits hide the ratio of records to evidence, which is the retro's own inversion test (IL-19).

## Decisions Reserved for Humans

These require explicit in-task approval even when technically easy. Default set, which adaptation may extend but should not silently shrink:

- Production deployments, production data, and migrations.
- Changes to API or schema contracts consumed outside this repository.
- Adding or upgrading dependencies.
- Deleting or disabling user-visible behavior or failing tests.
- Publishing anything outside the repository.
- Spending past a stated budget, and moving work to a more expensive resource tier (model class, hardware, paid service). "Use a stronger X" in an approved plan means the cheapest untested increment, never a silent jump to a higher price class.

Project-specific additions: `<list them here>`

### Mission envelope eligibility

Mission contracts may pre-authorize only the categories marked `yes` below, and only with the stated comma-separated literal-token bound. The category ids in this table are the machine-readable values used by `envelope.<category>` in a mission contract. A missing category is not pre-authorizable.

| Category id | Reserved decision | Pre-authorizable | Required bound |
| --- | --- | --- | --- |
| `production-deployment` | Production deployments | no | never |
| `production-data` | Production data | no | never |
| `migration` | Migrations | no | never |
| `publishing` | Publishing outside the repository | no | never |
| `history-rewrite` | Rewriting published history | no | never |
| `delete-user-visible` | Deleting user-visible behavior or failing tests | no | never |
| `api-schema` | Named API or schema contract changes | yes | named surfaces |
| `dependencies` | Adding or upgrading dependencies | yes | dependency allowlist |
| `spend-overage` | Spending past the stated budget | yes | explicit amount and currency |
| `tier-move` | Moving to a more expensive resource tier | yes | tier ceiling |

## Security Posture

- Untrusted content sources agents will read (web, issues, third-party code) and how to handle them: `<sources and handling>`
- Commands and paths forbidden beyond the runtime's own defaults: `<forbidden list>`
- Network egress expectations for agent sessions: `<policy>`
- Where secrets live and how they are provided to builds and tests; they never enter commits, logs, or plans: `<location>`

## External State and Ownership

Document who owns deployments, proxies, credentials, production data, migrations, and other actions agents must not mutate without explicit authorization.
