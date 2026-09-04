# The validator normalisation fix: code review (chain rnb-build1-cc1)

Reviewed tree 26c9f8813b406771718d9add77b3bb0f690d762d (chain rnb-build1, round 1). Critic: Claude Fable 5.1. Three material findings; a correction round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| RNB-01 | accepted | The fixture leg re-runs review over a round that already holds a successful review; the conformance-round-immutability rule (2026-09-01) refuses that before the boundary check, so the normalisation was never the cause. The brief's diagnosis was the seat's error. | The leg tests the mismatch on a fresh round; the validator narrowing stays. |
| RNB-02 | accepted | The unit test runs the mismatch before any review exists, the reverse of the fixture. | The test mirrors the fixture's order on a fresh round. |
| RNB-03 | resolved | Seat-side `go test ./internal/validate/...` passed (51 seconds) with the new test. | none |
| RNB-04 | noted | The git-derived prefix path is exercised only through the fallback. | none now |
| RNB-05 | noted | Role-mode validation never consults git; no production caller validates implementer returns in role mode. | none now |
| RNB-06 | noted | The refusal enumeration narrowed to unresolvable entries; clearer, nothing hidden. | none |
| RNB-07 | noted | A git failure while deriving the prefix falls back to the nested layout; production always has git. | none |
