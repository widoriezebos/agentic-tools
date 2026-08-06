# Devin self-test, on the other laptop

Devin is the one runtime that has never executed a single verb. This page is
everything needed to change that from a machine that has Devin access. Steps 1
and 2 are yours; everything after them is copy-paste.

## 1. Get the repository

```
git clone <your-github-remote-for-agentic-tools>
cd agentic-tools/metasystem
```

Any checkout works; nothing here needs the main development machine.

## 2. Install and log in to Devin

Install the Devin CLI the way Cognition documents for your account, then:

```
devin auth login
devin auth status   # must report you as logged in
```

This is the only part that cannot be scripted from here, and the reason this
runs on your other laptop.

## 3. Probe

```
scripts/agents/adapters/devin.sh probe
```

This writes a capability snapshot under `artifacts/agents/capabilities/`.
Dispatch refuses to use a runtime whose installed version has no snapshot, so
this must succeed before anything else.

## 4. Self-test

```
scripts/agents/adapters/devin.sh selftest
```

This exercises the full adapter contract for real: every verb, resume
identity, write-access mapping, and usage extraction. It spends a small amount
of real Devin usage. It prints a pass or fails loudly; there is no partial
credit.

## 5. Bring the evidence back

The snapshot is the only artifact that must come home, because dispatch on any
machine refuses a Devin version it has no snapshot for:

```
git add artifacts/agents/capabilities/devin-*.json
git commit -m "Devin capability snapshot and selftest evidence (other laptop)"
git push
```

Also copy the selftest output (or a screenshot of it) into the session when
you are back; the obligation matrix's three PARTIAL rows (ORCH-2, ORCH-10,
ORCH-20) all wait on exactly this run and its evidence.

## If something refuses

- `devin CLI is not installed` — step 2 did not finish; the adapter checks
  `command -v devin`.
- `devin authentication is unavailable` — run `devin auth login` again;
  the adapter refuses to guess.
- Anything else: copy the exact output back into the session. A refusal here
  is an adapter finding, which is precisely what the self-test exists to
  surface, so nothing that happens on this page is wasted.
