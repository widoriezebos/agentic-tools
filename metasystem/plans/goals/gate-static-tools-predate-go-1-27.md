# gate-static-tools-predate-go-1-27

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="Every landing on an upgraded Mac is blocked until it is fixed; nothing is destroyed; the fix is two version strings once the floor is decided."
- Tier: 3
- Intent: The Go gate pins its static tools to exact versions so every machine proves the same thing (staticcheck 2025.1, govulncheck v1.1.4 in scripts/agents/go-gate.sh). Those pins predate Go 1.27: on a Mac whose brew go is 1.27.1 (m1, 2026-09-06 after an upgrade) staticcheck 2025.1 refuses every package with 'export data version 4 is greater than maximum supported version 2', so the fast gate and therefore every landing on that machine refuses at the static re-proof. The Go toolchain itself is not pinned (go.mod names 1.26.5 as a floor) and go1.26.5 fails govulncheck on five standard-library findings fixed in 1.26.6, so today no default toolchain passes the whole gate; a per-process GOTOOLCHAIN=go1.26.6 is the seat-side workaround. DONE means the pinned static tools are bumped to releases that support the Go toolchain the fleet runs (1.27 or a stated floor), go.mod names that floor, the gate passes on brew's current go and on the VM, and the workaround is retired.
- Origin: main
- Next step: CURRENT WORD (Wido to m1c in chat, 2026-09-06 13:1xZ, verbatim): 'If we can do with a minimum, that's probably better than pinning to a single version.' This supersedes every earlier relay (the 'no pin, at best a minimum' relay via m1b now agrees with it; the 'okay pinning 1.27' word of 13:0xZ is withdrawn). Landing shape: go.mod says go 1.27 as the MINIMUM with no toolchain line (a machine with older Go downloads 1.27 automatically under the default GOTOOLCHAIN=auto; a machine with newer Go uses its own), plus staticcheck and govulncheck bumped to releases that read Go 1.27 export data, in one commit, gated on m1 (brew go 1.27.1) and the VM; the trade-off, stated: a floating floor means a future brew upgrade can outrun the tool pins again, and that is accepted. go-toolchain-pinned-in-go-mod is superseded and concluded as a duplicate by whoever lands this. DECIDED by Wido in chat with m1c, 2026-09-06 12:5xZ ('Agreed'): pin toolchain go1.26.6 in go.mod now (goal go-toolchain-pinned-in-go-mod carries that pin; both gate pins pass there and the VM follows the pin), and move the fleet to Go 1.27 only when the pinned static tools' next releases support it, as this goal's own step. This goal therefore waits on the 1.26.6 pin landing, then bumps staticcheck and govulncheck to releases that read Go 1.27 export data and raises the go.mod toolchain line wit
- OpenedAt: 2026-09-06T11:06:14Z
- Revision: 6
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:09:48Z revision=2 opid=WG9HYDN44QHESZM859WP7R2BRF-m1-7cd0bd60 authority=proven digest=a5d277c5574364a99150d0e412239b7649c6a81a6a06ef1b03b8a91744fbbfb7

History:
- 2026-09-06T11:06:14Z VBXDCFDAW76VCQF2WBXSCD80MA-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T11:09:48Z WG9HYDN44QHESZM859WP7R2BRF-m1-7cd0bd60 approve actor=human:Wido targets=code-critic-runtime-has-no-shell,gate-static-tools-predate-go-1-27,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
- 2026-09-06T12:53:55Z SBH2VJBYMNX7HMGFM0XFN6V81A-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T12:56:04Z ASYJVV8C5V1YQX2GJQXD80QWDQ-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T12:56:36Z KWVDWS2MRXA7VGYVNJWQMRCTVH-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T12:57:27Z 2ZDR83VHT5S9482ECX40KB04DJ-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
Integrity: sha256=0eb45e768cc6cce692c085351af1e7a6ba9cfc36a8a7d46e79ac5504a07ed495
