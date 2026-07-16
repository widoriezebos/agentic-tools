---
name: verify
description: Prove a code change works by driving the changed behavior end-to-end and observing the result instead of inferring it from green checks. Use before calling any change with a runnable surface complete. Do not use for changes with no runtime surface (docs, comments, pure test edits) or as a substitute for focused unit tests.
---

# Verify

Observed behavior is the only proof a change works. Tests, typechecks, and lint are supporting evidence; they do not replace running the thing.

## Choose the Surface

1. Name the behavior the change claims to alter, as a user or caller would see it.
2. Pick the smallest real entrypoint that exercises it: the commands in `docs/project-rules.md`, a CLI invocation, an API call, a service run, or a script against the built artifact.
3. Verify the artifact is rebuilt from the current source before trusting any run.

If no such entrypoint exists, say so explicitly and report the change as unverified at runtime; do not silently substitute unit tests as end-to-end proof.

## Drive and Observe

1. Exercise the changed path with realistic input, including at least one failure or boundary input when the change touches error handling, limits, or state transitions.
2. Capture the exact command and the observed output. The claim "it works" must be replaceable by that evidence.
3. Confirm the observation distinguishes new behavior from old: reproduce the before state when cheap, or point to the artifact that only the new behavior can produce.
4. If the observation contradicts expectation, stop and diagnose; switch to `skills/take-a-step-back/SKILL.md` if it stops converging.
5. If making verification pass would require changing an existing test, golden, or expected output, stop and surface it as a contract change for the human. Never silently weaken the assertion.

## Report

State: surface driven, exact commands, observed evidence, inputs not exercised, and any gap between what was proven and what the change claims. Never report inferred success as observed success.
