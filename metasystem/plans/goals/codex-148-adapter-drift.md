# codex-148-adapter-drift

- State: claimed
- Intent: m0's codex CLI is 0.148.0 while the adapter config filter (scripts/agents/adapters/codex-config-filter.v1.json) targets 0.146.0-0.146.x, so every dispatch warns and falls back to hashing all canonical configuration keys. Also record durably: codex 0.148.0 enables the code_mode_host feature by default and requires the companion binary codex-code-mode-host, absent from standalone installs — m0's first two chain attempts on supervise-start-gate-linux-red died on this; fixed 2026-08-31 by installing the vendor release binary (rust-v0.148.0, aarch64-unknown-linux-musl) at /usr/local/bin/codex-code-mode-host and re-enabling the feature in ~/.codex/config.toml.
- Origin: main
- Next step: Bump the filter range to cover 0.148.x through the lanes (Sol implements, small mechanical change), verifying which canonical keys changed between 0.146 and 0.148; land with a receipt naming the code-mode-host install
- OpenedAt: 2026-08-31T12:57:44Z
- Revision: 4
- Budget: elapsedLimit=6d attemptLimit=6 reservedJobMinutesLimit=600 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T14:18:18Z revision=4
- StopCapability: generation=4 revision=4 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T12:57:44Z 4F2MG3NFZ2E4HVTW9VAZ2CHSFG-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T12:57:44Z V1975H9RWY2PVW2DZ6XKFDMZMY-m0-c5dbf036 set-budget actor=human:Wido targets=codex-148-adapter-drift
- 2026-08-31T13:45:44Z N3WTTSXHWJPGE63RY977TWP6A5-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T14:18:18Z FQ72AD9ZASKW05FJA39CAZBA46-m0-c5dbf036 set-budget actor=human:Wido targets=codex-148-adapter-drift
Integrity: sha256=56a86a9eff9a6327c9401091bcd77cd3b3f1670ec45fa63ca4682a750f65d2ee
