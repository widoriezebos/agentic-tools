# Project Rules

Replace this file when adopting the harness. Keep facts concrete and repository-specific.

## Project Map

- Purpose: `<one paragraph>`
- Production entrypoints: `<paths>`
- Test roots: `<paths>`
- Generated files: `<paths and ownership>`
- Sensitive or protected areas: `<paths>`

## Commands

- Fast focused test: `<command>`
- Full unit suite: `<command>`
- Integration/end-to-end suite: `<command>`
- Build/package: `<command>`
- Format/lint/typecheck: `<command>`
- Local run: `<command>`
- Refactor acceptance gate: `<command>`. The full behavior-preservation proof (full suite, benchmark, or golden run) that accepts a refactor candidate. State its cadence backstop if it differs from the defaults owned by `scripts/refactor-baseline.sh`.
- Improvement evaluation: `<command>`. The on-demand evaluation for improvement goals. State the primary metric, guard metrics with floors, the noise floor (minimum meaningful delta), any cheaper canary or subset variant, and the holdout or case-rotation policy.
- Frontier preservation policy: `<policy>`. How a new best-known state is preserved: tag pattern, push target, and who may move it.

State realistic timeouts and prerequisites here. Do not repeat these commands in skills or root instructions.

## Local Invariants

List only rules that cannot be inferred from code or tooling and apply broadly in this repository. Prefer an executable check whenever the rule is binary.

## Decisions Reserved for Humans

These require explicit in-task approval even when technically easy. Default set, which adaptation may extend but should not silently shrink:

- Production deployments, production data, and migrations.
- Changes to API or schema contracts consumed outside this repository.
- Adding or upgrading dependencies.
- Deleting or disabling user-visible behavior or failing tests.
- Publishing anything outside the repository.

Project-specific additions: `<list them here>`

## Security Posture

- Untrusted content sources agents will read (web, issues, third-party code) and how to handle them: `<sources and handling>`
- Commands and paths forbidden beyond the runtime's own defaults: `<forbidden list>`
- Network egress expectations for agent sessions: `<policy>`
- Where secrets live and how they are provided to builds and tests; they never enter commits, logs, or plans: `<location>`

## External State and Ownership

Document who owns deployments, proxies, credentials, production data, migrations, and other actions agents must not mutate without explicit authorization.
