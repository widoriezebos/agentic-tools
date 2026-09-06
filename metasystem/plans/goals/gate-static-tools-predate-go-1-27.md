# gate-static-tools-predate-go-1-27

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="Every landing on an upgraded Mac is blocked until it is fixed; nothing is destroyed; the fix is two version strings once the floor is decided."
- Tier: 3
- Intent: The Go gate pins its static tools to exact versions so every machine proves the same thing (staticcheck 2025.1, govulncheck v1.1.4 in scripts/agents/go-gate.sh). Those pins predate Go 1.27: on a Mac whose brew go is 1.27.1 (m1, 2026-09-06 after an upgrade) staticcheck 2025.1 refuses every package with 'export data version 4 is greater than maximum supported version 2', so the fast gate and therefore every landing on that machine refuses at the static re-proof. The Go toolchain itself is not pinned (go.mod names 1.26.5 as a floor) and go1.26.5 fails govulncheck on five standard-library findings fixed in 1.26.6, so today no default toolchain passes the whole gate; a per-process GOTOOLCHAIN=go1.26.6 is the seat-side workaround. DONE means the pinned static tools are bumped to releases that support the Go toolchain the fleet runs (1.27 or a stated floor), go.mod names that floor, the gate passes on brew's current go and on the VM, and the workaround is retired.
- Origin: main
- Next step: DECIDED by Wido in chat with m1c, 2026-09-06 12:5xZ ('Agreed'): pin toolchain go1.26.6 in go.mod now (goal go-toolchain-pinned-in-go-mod carries that pin; both gate pins pass there and the VM follows the pin), and move the fleet to Go 1.27 only when the pinned static tools' next releases support it, as this goal's own step. This goal therefore waits on the 1.26.6 pin landing, then bumps staticcheck and govulncheck to releases that read Go 1.27 export data and raises the go.mod toolchain line with them, proven on m1 and the VM. Previous next step: Find the first staticcheck release that reads Go 1.27 export data and the current govulncheck; decide the toolchain floor with Wido (brew stable 1.27.1 on the Macs, the VM's Debian go); brief one round: bump the two pins and the go.mod line, prove the gate on m1 and the VM.
- OpenedAt: 2026-09-06T11:06:14Z
- Revision: 9
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:09:48Z revision=2 opid=WG9HYDN44QHESZM859WP7R2BRF-m1-7cd0bd60 authority=proven digest=a5d277c5574364a99150d0e412239b7649c6a81a6a06ef1b03b8a91744fbbfb7
- Sliced: machine=m1b lineage=main-1788680071-18713-e76d5d revision=8 at=2026-09-06T15:47:37Z
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-06T15:46:37Z revision=8 accountingRevision=8
- StopCapability: generation=8 revision=8 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T11:06:14Z VBXDCFDAW76VCQF2WBXSCD80MA-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T11:09:48Z WG9HYDN44QHESZM859WP7R2BRF-m1-7cd0bd60 approve actor=human:Wido targets=code-critic-runtime-has-no-shell,gate-static-tools-predate-go-1-27,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
- 2026-09-06T12:53:55Z SBH2VJBYMNX7HMGFM0XFN6V81A-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T12:56:04Z ASYJVV8C5V1YQX2GJQXD80QWDQ-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T12:56:36Z KWVDWS2MRXA7VGYVNJWQMRCTVH-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T12:57:27Z 2ZDR83VHT5S9482ECX40KB04DJ-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T12:58:59Z J7Z1HC41JX75BMM0NFP2K3EABE-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=gate-static-tools-predate-go-1-27
- 2026-09-06T15:46:37Z ZBQP2YRBPJACY9K77XM7ZKMG0S-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=gate-static-tools-predate-go-1-27
- 2026-09-06T15:47:37Z J6E9H8K2XDEAHBMXWNVVJBFPCE-m1b-c6925449 slice-start actor=m1b+main-1788680071-18713-e76d5d targets=gate-static-tools-predate-go-1-27
Integrity: sha256=4226c734efa7f371c1a3c7632a957ed61f1a2ab7feded3854bec5f1b464f976a
