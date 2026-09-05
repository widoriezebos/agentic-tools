Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout-land)
Date: 2026-09-05

# Follow-up: the critic's register, three material findings accepted

The critic's register for your round is in chain scp-build1-cc1 (findings SCC-41 to SCC-45; read its return for the full text). Accepted:

- SCC-41 (critical): `internal/registry/selection.go` with `requireOwnerCheckout` selects the owner's checkout from an open reduced claim, and a claim opens only on an arming-reservation record that no production writer emits; the owner ledger writes relaunched, launched and exited rows only, and the reduction discards those without a reservation. Every live owner on this machine, including this seat's, reads as having no registry checkout: the lockout in a new form. Fix: read the owner's checkout path from the rows production actually writes (the launched and relaunched rows carry `checkoutPath`, keyed by the owner's tag and pid), resolved by the reduction the way it already binds owners, and never from claims.
- SCC-42 (high): `internal/supervise/arming_test.go` seeds arming and armed rows that the previous binary never wrote, so it passes against a registry shape that does not exist and could not catch SCC-41. Fix: the legacy-owner cases seed exactly what the previous binary wrote, relaunched and launched rows holding the state root, and nothing else; the test must fail on the current selection before your fix and pass after.
- SCC-43 (medium): `ShutdownAt` calls the guard before any liveness check; a dead owner without a row, or with a stale path after a checkout move, makes `metasystem up --shutdown` refuse where it used to proceed. Fix: shutdown guards live owners only, exactly as arming now does.

Seat-side these three also show in the supervision suite, which now fails two scenarios that were green before your round: operator-layout, "nested operator shutdown failed … supervision request for checkout … names owner tag … with no registry checkout"; and foreign-owner, "foreign-owner refusal did not name the cause" with the same detail. Both must be green again; the foreign-owner refusal must keep naming its cause (the recorded owner belongs to another checkout, both paths named). SCC-44 (the hard-coded in-bed flag) goes with this round; SCC-45 is noted, no change.

`gofmt -l`, `go vet`, staticcheck on `./internal/registry/... ./internal/supervise/... ./internal/up/... ./cmd/metasystem/`; `go test` on those four; `bash -n` on both scripts. Return within ninety minutes. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator runs the supervision suite seat-side and re-arms this checkout as the proof.
