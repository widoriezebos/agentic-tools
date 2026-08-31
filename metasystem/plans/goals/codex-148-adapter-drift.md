# codex-148-adapter-drift

- State: queued
- Intent: m0's codex CLI is 0.148.0 while the adapter config filter (scripts/agents/adapters/codex-config-filter.v1.json) targets 0.146.0-0.146.x, so every dispatch warns and falls back to hashing all canonical configuration keys. Also record durably: codex 0.148.0 enables the code_mode_host feature by default and requires the companion binary codex-code-mode-host, absent from standalone installs — m0's first two chain attempts on supervise-start-gate-linux-red died on this; fixed 2026-08-31 by installing the vendor release binary (rust-v0.148.0, aarch64-unknown-linux-musl) at /usr/local/bin/codex-code-mode-host and re-enabling the feature in ~/.codex/config.toml.
- Origin: main
- Next step: Bump the filter range to cover 0.148.x through the lanes (Sol implements, small mechanical change), verifying which canonical keys changed between 0.146 and 0.148; land with a receipt naming the code-mode-host install
- OpenedAt: 2026-08-31T12:57:44Z
- Revision: 1

History:
- 2026-08-31T12:57:44Z 4F2MG3NFZ2E4HVTW9VAZ2CHSFG-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
Integrity: sha256=7252c7710b944c19a5419ca1db6b93379abfad01012128badd4ed165977ecccd
