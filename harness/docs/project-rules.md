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
- Refactor acceptance gate: `<command>` — the full behavior-preservation proof (full suite, benchmark, or golden run) that accepts a refactor candidate; state its cadence backstop if it differs from the harness defaults (24 hours / 40 commits)

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

## External State and Ownership

Document who owns deployments, proxies, credentials, production data, migrations, and other actions agents must not mutate without explicit authorization.
