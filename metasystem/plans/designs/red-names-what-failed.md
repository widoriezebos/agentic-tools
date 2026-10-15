# Design: a red names what failed, and a flake closes on the evidence of its fix

- Id: 01M4Z2REDNAMES000000000000A1
- Kind: design
- Status: accepted
- Goals: red-names-what-failed
- Evidence: lane test findings 321 (the 23:30 10-14 red labelled a flake on TestPendingWaitFromChildShell, a test that passed, while fast-static-build failed), 311 (the same flaky test filed twice: a flake entry and a fresh red entry), 327 (the adopt flake incident outlived its fix goal and three green proofs; nothing feeds the closure rule); the 24-hour analysis of 2026-10-15.

## Problem

A red proof of main is classified by `observeRed` (`internal/landing/plain/replay.go:42-54`) and the flake register; on 2026-10-14 23:26 the result named "fast-static-build (package), metasystem/cmd/metasystem TestPendingWaitFromChildShell" as a flake and then as the main red, although that test passed and the failing check was fast-static-build alone; the steward trusted the label for 2.5 hours. A flaky test can be filed twice (finding 311). A confirmed-flake incident closes by the rule in `internal/goal/trunkred.go:909` (`countFlakePass`: a fix commit and three distinct executed passes of the affected test), but `clearLandingIncidents` (`cmd/metasystem/intent_landing_incident.go:113`) excludes flake entries, so the lane's full proofs never feed that rule: the adopt flake stayed open 17 hours after its fix goal was done and was named as main's cause in every red headline, including a false one.

## Decisions

D1. A red names exactly the checks that failed in that proof: a package or static check that failed is named as such; a test is named only when its own result in that proof is a failure. A test that passed is never a flake candidate of that proof.

D2. One flaky test is one incident: a new red on a test with an open flake incident is a sighting of that incident, never a second entry (finding 311).

D3. The lane's full proofs feed the existing closure owner: every executed pass of an affected test on main after its fix commit is recorded as one distinct pass for `countFlakePass`; the rule itself (three passes) stays as it is — changing it is the person's policy decision. When the rule is met the incident closes by itself and the record says which proofs closed it.

D4. UX: the person reads "flaky test X: fixed by goal G, passed 2 of 3 times since; closes at the third" or "closed by the proofs of 17:39, 02:50 and 07:48"; no record paths.

## Units

- U1 (D1–D4): brief to be written; ≤120 production lines. U1 is the goal's whole delivery.

## Not in scope

The three-pass threshold; the engine-launch and non-run truth (goal lane-proof-finds-its-own-engine); headline presentation (goal the-lane-says-what-it-does).
