# Draft: report text still carries review tags

Status: draft — needs an appetite and promotion.

The comment sweep left string literals alone by design; a few carry
process tags into text a HUMAN reads, which the write-to-a-human rule
forbids for the same reason the comment standard does:

- internal/validate/conformance.go:541 — refusal message says
  "(no exception, D100)"
- internal/lease/disk.go:232 — error text "(SLC-R4-001)"
- internal/lease/claim.go — event message "same-process identity
  reconciled (KI-33)"; test names and t.Fatal texts naming KI-33
  (TestKI33SameProcessReannounce)

Scope: rewrite each message to name the invariant, update the pinned
tests. Small — likely under an hour. Note: turnio.go's "devin host
candidate" naming is NOT this draft; it belongs to the
agent-agnosticism backlog item.
