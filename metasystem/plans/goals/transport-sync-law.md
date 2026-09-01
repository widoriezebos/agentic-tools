# transport-sync-law

- State: queued
- Intent: Repository topology law, updated at the 2026-08-31 reconciliation: m0's Debian guest originally saw ONLY the transport bare repository (/Users/wido/LocalStorage/transport/agentic-tools.git) — no host checkout, no network remote — and machines that forget to push/pull the relay diverge invisibly; that exact gap caused the m0/m2 fleet divergence reconciled at landing b700f44e (m0's original line preserved as branch machine/m0). NOW every machine's origin is GitHub, which removes the relay-forgetfulness class for machines that fetch before landing and push after. Remaining law to encode: land.sh (or the landing gate) fetches the canonical branch before landing and pushes after, so a machine can never land onto a stale base silently; the transport bare repo is legacy — Wido decides whether it retires or stays as the guest-mount fallback (its main still holds m0's pre-reconciliation line and refuses fast-forward).
- Origin: main
- Next step: Small mechanical slice, claimable by any machine (R-33: robustness, well under 4h): scripts/agents/sync-transport.sh hard-fails when the 'transport' remote is not configured (m0 removed its pointer after the GitHub migration; every m0 landing now ends with a failed 'sync transport' step unless --skip-transport is passed by hand). Make the script a graceful, loudly-logged no-op when 'git remote get-url transport' fails - a machine without a transport remote has nothing to sync, which violates no invariant. Then the second slice as before: pre-landing fetch verification inside land.sh; then Wido's word on retiring the transport bare repo entirely.
- OpenedAt: 2026-08-31T19:08:55Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-31T19:08:55Z QC2534N1Z2GZ8JJTFMZQCJN330-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=transport-sync-law
- 2026-09-01T07:32:36Z QFRKT8JJ51G3KX7QZ1P5X6H9DK-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=transport-sync-law
- 2026-09-01T20:27:40Z YFMMT2F7N6DZQ9PHW1CJE342TY-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=transport-sync-law
Integrity: sha256=448ab3c39f1c35bce9564ad56ee0b70e04f48eeda5a3d8c7c6ffb93340a118b4
