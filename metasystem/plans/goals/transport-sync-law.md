# transport-sync-law

- State: queued
- Intent: Repository topology law, updated at the 2026-08-31 reconciliation: m0's Debian guest originally saw ONLY the transport bare repository (/Users/wido/LocalStorage/transport/agentic-tools.git) — no host checkout, no network remote — and machines that forget to push/pull the relay diverge invisibly; that exact gap caused the m0/m2 fleet divergence reconciled at landing b700f44e (m0's original line preserved as branch machine/m0). NOW every machine's origin is GitHub, which removes the relay-forgetfulness class for machines that fetch before landing and push after. Remaining law to encode: land.sh (or the landing gate) fetches the canonical branch before landing and pushes after, so a machine can never land onto a stale base silently; the transport bare repo is legacy — Wido decides whether it retires or stays as the guest-mount fallback (its main still holds m0's pre-reconciliation line and refuses fast-forward).
- Origin: main
- Next step: Small mechanical slice: pre-landing fetch and post-landing push verification inside land.sh; then Wido's word on retiring the transport remote. Under 4h, robustness gain (R-33)
- OpenedAt: 2026-08-31T19:08:55Z
- Revision: 1

History:
- 2026-08-31T19:08:55Z QC2534N1Z2GZ8JJTFMZQCJN330-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=transport-sync-law
Integrity: sha256=a49bf571090b5529b30fb6b0d3921c88687643f51e137b6ef8578d9c889c52b2
