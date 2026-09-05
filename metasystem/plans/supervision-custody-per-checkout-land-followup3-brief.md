Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout-land)
Date: 2026-09-05

# Follow-up: tests only, to restore the registry package's coverage floor

The landing gate refused your third round on the coverage ratchet: `./internal/registry` measured 88.0% against its recorded floor of 90.8% (the new `selection.go`, `compact.go` and reduction paths are under-covered); `./internal/up` is fine at 56.8% against 55.0%. Add tests only, in `internal/registry`, until the package measures at least 91.0% with `go test -cover ./internal/registry/`: cover the selection of a published owner by checkout path, the drop-and-list of a row naming a second checkout for a known identity, the compaction that retains an active legacy publication, and the error branches of the new code. Change no non-test file. Every path in your return is relative to the repository root (starting with `metasystem/`). Return within thirty minutes with the measured percentage in whatWasDone.
