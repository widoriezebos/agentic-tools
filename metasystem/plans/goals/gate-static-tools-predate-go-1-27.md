# gate-static-tools-predate-go-1-27

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="Every landing on an upgraded Mac is blocked until it is fixed; nothing is destroyed; the fix is two version strings once the floor is decided."
- Tier: 3
- Intent: The Go gate pins its static tools to exact versions so every machine proves the same thing (staticcheck 2025.1, govulncheck v1.1.4 in scripts/agents/go-gate.sh). Those pins predate Go 1.27: on a Mac whose brew go is 1.27.1 (m1, 2026-09-06 after an upgrade) staticcheck 2025.1 refuses every package with 'export data version 4 is greater than maximum supported version 2', so the fast gate and therefore every landing on that machine refuses at the static re-proof. The Go toolchain itself is not pinned (go.mod names 1.26.5 as a floor) and go1.26.5 fails govulncheck on five standard-library findings fixed in 1.26.6, so today no default toolchain passes the whole gate; a per-process GOTOOLCHAIN=go1.26.6 is the seat-side workaround. DONE means the pinned static tools are bumped to releases that support the Go toolchain the fleet runs (1.27 or a stated floor), go.mod names that floor, the gate passes on brew's current go and on the VM, and the workaround is retired.
- Origin: main
- Next step: Find the first staticcheck release that reads Go 1.27 export data and the current govulncheck; decide the toolchain floor with Wido (brew stable 1.27.1 on the Macs, the VM's Debian go); brief one round: bump the two pins and the go.mod line, prove the gate on m1 and the VM.
- OpenedAt: 2026-09-06T11:06:14Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:09:48Z revision=2 opid=WG9HYDN44QHESZM859WP7R2BRF-m1-7cd0bd60 authority=proven digest=a5d277c5574364a99150d0e412239b7649c6a81a6a06ef1b03b8a91744fbbfb7

History:
- 2026-09-06T11:06:14Z VBXDCFDAW76VCQF2WBXSCD80MA-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T11:09:48Z WG9HYDN44QHESZM859WP7R2BRF-m1-7cd0bd60 approve actor=human:Wido targets=code-critic-runtime-has-no-shell,gate-static-tools-predate-go-1-27,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
Integrity: sha256=f20d80607c6d1ae12d9203001fdd4496eb9c5115c2df2eb748d2942738a4538e
