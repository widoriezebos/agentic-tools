# The flake protocol

The landing lane enforces one repeat per tree in code. When tests fail,
it allows a repeat only when the batch cannot have caused the red, using
main's testing contract and register. An incomplete test report gets no repeat.
A check that stopped or ran no test gets one more whole check.

When every failed test is on its unit's open pending flake entry, the
known flake's unit runs once more alone. Otherwise, an unaffected batch
gets one more whole check. A second red gets no further check of that tree.

A test that passes after failing is recorded in `plans/goals/trunk-red.json`
and routed to `fix-flaky-UNIT` at the first sighting. The record names the
tests, both attempts, tree, logs, load and holding surfaces. The unit's fix
goal owns the repair; another sighting extends that goal or reopens it.
Without a confirmed record, the lane stays red.

This puts R-19's working model in code and follows R-103-m1e: a flaky test
is fixed. The repeat decides whose fault the red is and routes the repair.
`memory/flake-registry.md` is the old hand register.
