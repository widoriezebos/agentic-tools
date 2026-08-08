# Devin self-test

Devin now executes verbs through the metasystem. The adapter self-test — every
verb, resume identity, permission probes, and usage extraction — passed against
the real CLI on 2026-08-08, `devin 3000.3.27`, model `swe-1-7`. This page is
how to reproduce that on any machine with Devin access, and what to bring back.

Getting there found and fixed a run of defects that no amount of reading the
adapter would have surfaced; they are recorded in `plans/known-issues.md` and
`metasystem/plans/devin-support.md`. The most important standing fact: on an
account where `--sandbox` is refused by policy, Devin runs UNCONTAINED — a
shell command writes outside its declared roots — so the operator's VM or
container is the boundary, not anything the metasystem enforces. The capability
snapshot declares every envelope member unenforced so this is never implied
away.

## 1. Get the repository

```
git clone <your-github-remote-for-agentic-tools>
cd agentic-tools/metasystem
```

Any checkout works; nothing here needs the main development machine.

## 2. Install and log in to Devin

```
devin auth login
devin auth status   # must report you as logged in
```

This is the only part that cannot be scripted from here.

## 3. Probe

```
scripts/agents/adapters/devin.sh probe
```

Writes a capability snapshot under `artifacts/agents/capabilities/`. Dispatch
refuses a runtime whose installed version has no snapshot, so this runs first.

## 4. Self-test

```
scripts/agents/adapters/devin.sh selftest
```

Exercises the full adapter contract for real: every verb, resume identity, the
permission probes, and usage. On a consumer account usage is token-based
(`native`); on an enterprise account it is ACU-based, reported through provider
units — the self-test accepts either, and asserts only that a turn is measured
by something the fence can meter. It spends a small amount of real Devin usage
and prints a pass or fails loudly.

## 5. Bring the evidence back

The capability snapshot must come home, because dispatch on any machine refuses
a Devin version it has no snapshot for:

```
git add artifacts/agents/capabilities/devin-*.json
git commit -m "Devin capability snapshot (<machine>, <cli-version>)"
git push
```

The snapshot alone does NOT prove acceptance — probe writes it before the
behavioural test runs, and it survives a failed test. The proof is the
self-test's own job records and returns beside the provider's exported
transcripts, so the two can be checked against each other. A durable,
redacted acceptance bundle (D-8 in `plans/devin-support.md`) is designed but
not yet built — tracked in `plans/known-issues.md`. Until it exists, copy the
self-test output into the session as the acceptance record.

## If something refuses

- `devin CLI is not installed` — step 2 did not finish.
- `devin authentication is unavailable` — run `devin auth login` again.
- `dispatch escalation refused ... model tiers are absent` — the roster and the
  requested pair are unranked; `model.tier.*` in `metasystem.conf.local` ranks
  them (see the Devin entries there).
- Anything else: copy the exact output back into the session. A refusal here is
  an adapter finding, which is what the self-test exists to surface.
