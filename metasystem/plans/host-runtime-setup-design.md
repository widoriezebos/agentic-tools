# Host setup for interchangeable agent runtimes

Goal: `host-runtime-setup`. User request, 2026-09-07: make Claude Code,
Devin, and Codex work here, automatically where possible, while preserving
Claude behavior. This is host registration, not the wider installation
rewrite tracked by `runtime-install-execution`.

## Observed problem and facts

- This checkout has only `.claude/settings.json` at the repository root.
  Its actual contract and routing live under `metasystem/`; neither a root
  `AGENTS.md` nor the runtime skill registrations exist. Codex therefore
  started without the repository contract or lifecycle registration.
- `internal/runtimes/registration.go` already declares each runtime's
  skill trees, profiles, and hook settings destination. These rows describe
  the hard-coded registration branches in `scripts/adopt.sh`.
- `internal/hooks/hooks.go` checks hooks structurally. The CLI refuses
  runtimes without `Declaration.SelfCheck`; currently only Claude declares
  it. The template validation at `scripts/validate-metasystem.sh:2193`
  checks only Claude.
- `scripts/agents/supervision-hook.sh` owns lifecycle behavior and already
  resolves the nested template's installation root. Its existing identity
  lookup restricts the ancestor search to the runtime named by the hook.
  `proc find-ancestor` also supports an unrestricted nearest-agent search.
- Devin imports `.claude/settings.json` hooks by default as well as its
  own `.devin/config.json`. Non-tool events need an empty/absent matcher.
  Sources: https://docs.devin.ai/cli/extensibility/hooks/overview and
  https://docs.devin.ai/cli/extensibility/hooks/lifecycle-hooks (read).
- Codex loads `.codex/hooks.json` next to its project config layer. Hooks
  run from the session working directory and require trust of their exact
  definitions. `SessionStart` includes `compact` as well as startup/resume.
  Source: https://developers.openai.com/codex/hooks (read).
- Claude's existing Stop behavior includes a separate receipt-cadence
  notice and the supervision verdict. Both must survive.

## Contract

One repeatable command prepares an existing installation for every
registered adoptable runtime by default:

```
bin/metasystem runtime setup --repo <repository-or-path-inside-it>
bin/metasystem runtime setup --repo <path> --runtimes claude,codex
bin/metasystem runtime setup --repo <path> --check
```

The default means all runtime entry points are ready; it does not choose a
model, launch an agent, change permissions, or depend on which binaries
happen to be on PATH. An explicit comma-separated runtime set uses the same
implementation. Registration destinations come from the registry, not a
second list in shell. The command reports configuration readiness and any
runtime trust/restart step separately from observed lifecycle execution.

The repository and installation roots are resolved through the existing
state-root/layout owner. Root and nested-template layouts are supported.
Setup never copies product source, upgrades dependencies, rewrites the
model roster, approves runtime trust, or alters credentials.

## Owners and behavior

1. A small `internal/hostsetup` owner plans and applies host registration;
   `cmd/metasystem/runtime_setup.go` is its CLI boundary. It consumes the
   existing registration rows. `internal/runtimes` remains pure data.
2. For a nested installation, root `AGENTS.md` and `CLAUDE.md` are short
   pointers to the canonical contract/routing and local project facts.
   Existing application instructions are preserved, using a delimited
   managed pointer block if an instruction file already exists. An adopted
   root already contains the canonical files and is left intact.
3. Skill trees use relative symlinks; profiles follow their declared copy
   or in-place operation. Shared `.agents/skills` entries are deduplicated.
   A foreign file, conflicting link, or modified profile is reported and
   preserved. There is no force-overwrite flag.
4. `internal/hooks` owns rendering and structural merging/checking of
   lifecycle settings. It preserves unrelated JSON keys, events, matchers,
   and handlers. Only recognized MetaSystem handlers for this installation
   may be replaced; uncertain ownership is a conflict. All inputs and
   conflicts are checked before writes. Writes are atomic per file;
   rerunning after interruption completes the same plan without duplicates.
5. Generated commands resolve their repository with `git rev-parse` and
   use the installation's relative path, so starting from a subdirectory
   and moving/cloning the checkout do not bind hooks to an old absolute
   path. Shell paths are quoted; no evaluation of config data as code.
6. `scripts/agents/runtime-hook.sh` is a narrow lifecycle transport entry.
   Before any lifecycle mutation it asks `proc find-ancestor` for the
   nearest actual runtime. A proven different runtime exits successfully
   without output or state changes. This prevents Devin's imported Claude
   hooks from registering or stopping a second session. A matching runtime
   forwards stdin unchanged to the existing supervision hook. Unproven
   identity keeps the existing hook's classification/failure behavior;
   it must never become a new success exemption.
7. The launcher also carries the existing receipt-cadence notice so an
   imported Claude receipt handler is subject to the same runtime guard.
   Claude keeps its startup/stop/end events, timeouts, and failure verdicts.
   Runtime-specific matchers and trust notes remain declared at the runtime
   seam. Do not add broad provider configuration changes to suppress
   imports. Codex compaction reuses the existing start/rejoin behavior.
8. `hooks check` supports all registered host runtimes and checks the
   expected event, action, runtime, matcher and timeout in the actual
   destination. Keep the existing positional Claude check working. The
   template's validation checks every installed host configuration rather
   than certifying the whole installation from Claude alone.
9. Adoption uses the same host-registration owner for its selected runtime
   set, retaining its existing default selection and fresh-target safety
   rules. Existing Codex hook content cannot be silently overwritten.
   Documentation gives one setup/check procedure and an explicit trust or
   restart step where the provider requires it.

## Verification and scope bounds

First reproduce missing registrations and the Codex self-check refusal.
Tests then cover both layouts, all three runtime selections, repeated
setup, a path containing spaces, unrelated settings/handlers, an imported
foreign runtime hook, unknown runtimes, malformed JSON, conflicting links,
and partial installation followed by a retry. Prove that existing Claude
receipt and Stop refusal behavior survives. Tests use isolated repositories
and their own process identities; they never arm or stop the live seat's
supervision as fixture cleanup.

Run focused hostsetup/hooks/runtimes/CLI tests and the affected adoption and
hook fixtures. The main coordinator runs process fixtures; delegates must
not be asked to defeat their process-visibility restrictions. Then run the
new setup and check on this checkout, inspect generated diffs, and verify
the actual Codex announcement/lease. Provider-driven lifecycle observations
are recorded per provider; configuration checks never count as live proof.
No model calls are needed to test settings generation or routing.

Review: first round with no material findings stops. Failsafe is round 2;
bounded, fixture-expressible findings become named test obligations. A
finding is material only if it changes an implementation artifact or test.
Do not expand this into unattended re-engagement, spend accounting,
terminal authority, a general updater, or migration of private memories.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| HOST-1 | HIGH | Contract | All selected hosts discover contract, skills and lifecycle settings in both layouts | internal/hostsetup | internal/hostsetup/setup.go | internal/hostsetup/setup_test.go | runtime setup and check in isolated and live checkouts | MISSING | Implement and exercise setup |
| HOST-2 | HIGH | Owners 3-4 | Preserve unrelated instructions, settings, handlers and skill files; retries do not duplicate | internal/hostsetup and internal/hooks | internal/hostsetup/setup.go and internal/hooks/setup.go | preservation, malformed-input, conflict and retry tests | compare pre/post files and a second setup | MISSING | Implement preservation tests first |
| HOST-3 | HIGH | Owners 6-7 | Imported hooks cannot mutate lifecycle state for the wrong runtime; Claude behavior survives | runtime-hook transport and existing supervision owner | scripts/agents/runtime-hook.sh | isolated hook routing fixtures | captured routing outputs and unchanged foreign state | MISSING | Add routing fixtures |
| HOST-4 | HIGH | Owners 8-9 | Check all installed hosts honestly and use shared registration during adoption | hooks CLI and adoption | cmd/metasystem/hooks.go and scripts/adopt.sh | CLI and adoption fixtures | setup/check outputs per runtime | MISSING | Generalize checks |
| HOST-5 | MEDIUM | Verification | Report trust/restart needs and distinguish configuration from live execution | setup output and documentation | docs/project-adaptation.md | output assertions | observed Codex host; record other providers separately | MISSING | Document and run available proof |
