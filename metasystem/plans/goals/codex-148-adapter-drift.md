# codex-148-adapter-drift

- State: claimed
- Intent: m0's codex CLI is 0.148.0 while the adapter config filter (scripts/agents/adapters/codex-config-filter.v1.json) targets 0.146.0-0.146.x, so every dispatch warns and falls back to hashing all canonical configuration keys. Also record durably: codex 0.148.0 enables the code_mode_host feature by default and requires the companion binary codex-code-mode-host, absent from standalone installs — m0's first two chain attempts on supervise-start-gate-linux-red died on this; fixed 2026-08-31 by installing the vendor release binary (rust-v0.148.0, aarch64-unknown-linux-musl) at /usr/local/bin/codex-code-mode-host and re-enabling the feature in ~/.codex/config.toml.
- Origin: main
- Next step: SCOPE SETTLED BY WIDO (decision-ask 2026-08-31, tuple raised to 48h/6/600m/1): the goal becomes a design-bearing slice — filter schema v2 carries a LIST of verified minor lines (0.146.x with its keys, 0.148.x with its keys), the loader picks the line matching the running CLI, out-of-list stays fail-open. Ground truth learned this chain: versionInRange's wildcard max is prefix-equality by design (one verified minor per file), so the closed chain codex-filter-range-148's certified diff is REFUTED — it would have made m1/m2's 0.146 CLIs warn on every dispatch; the diff is discarded unlanded, the chain record stands as the evidence. R-25 lanes: Fable fresh-context designs, Sol critiques the design, Sol implements, Fable critiques the implementation. m0 observation for the design: 0.148.0 churns projects.*.trust_level entries (behavior-relevant, must stay hashed); notice/tui churn keys unobservable in exec-only use, retained from the 0.146 verification.
- OpenedAt: 2026-08-31T12:57:44Z
- Revision: 5
- Budget: elapsedLimit=6d attemptLimit=6 reservedJobMinutesLimit=600 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T14:18:18Z revision=4
- StopCapability: generation=4 revision=4 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T12:57:44Z 4F2MG3NFZ2E4HVTW9VAZ2CHSFG-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T12:57:44Z V1975H9RWY2PVW2DZ6XKFDMZMY-m0-c5dbf036 set-budget actor=human:Wido targets=codex-148-adapter-drift
- 2026-08-31T13:45:44Z N3WTTSXHWJPGE63RY977TWP6A5-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
- 2026-08-31T14:18:18Z FQ72AD9ZASKW05FJA39CAZBA46-m0-c5dbf036 set-budget actor=human:Wido targets=codex-148-adapter-drift
- 2026-08-31T14:18:19Z 9T63FXCXKSXTSVCHCCYBDYWKA7-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=codex-148-adapter-drift
Integrity: sha256=e865ca4d1282025dedbda15ecd94d473a81474243786e3950359e70101305c87
