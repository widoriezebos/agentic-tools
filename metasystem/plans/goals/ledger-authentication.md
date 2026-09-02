# ledger-authentication

- State: queued
- Intent: The accepted goal ledger is unauthenticated by design (ValidateAcceptedTree accepts any structurally-valid tree), so an agent with checkout write access can forge any in-repo human-authority record - session-stop, resume, set-obligation all share this. Wido 2026-09-02, deferring true forge-proofing: this is the foundational goal for authenticating ledger history (signing or equivalent) so a forged human act is mechanically rejected. Wide blast radius; its own design program. NOT the idle fix (that ships under bar (a): impossible by accident/honest agent). Opened as the home for the forge threat so idle-with-backlog does not carry it.
- Origin: main
- Next step: INTENT: forged human-authority ledger records become mechanically rejectable. This is a design-first foundational goal, not a slice - it touches every human-only verb's trust boundary. CONSTRAINTS: must not break the existing unauthenticated fast-path for ordinary agent ledger ops; the authentication binds only human-reserved acts. FREEDOMS: signing scheme, key custody, whether it is per-op or per-tree - all design. Budget: Wido's word at claim (design-bearing, likely multi-slice). Back of queue unless Wido prioritises.
- OpenedAt: 2026-09-02T11:31:48Z
- Revision: 1

History:
- 2026-09-02T11:31:48Z QTBA5B021YKN5DPBQRV2A5YWVY-m0-c5dbf036 open actor=human:Wido targets=ledger-authentication
Integrity: sha256=953f383e0d1e10c6b71463319e9e00b2ac1c405e9c07d44ac5a820424c836f75
