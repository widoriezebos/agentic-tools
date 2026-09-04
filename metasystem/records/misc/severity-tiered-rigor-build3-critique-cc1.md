# Tiering machinery, part three: code review (chain str-build3-cc1)

Reviewed tree 71f3ac42ee10bc934179863b79a54306cc299fe8 (chain str-build3, round 2). Critic: Claude Fable 5.1. Brief: plans/severity-tiered-rigor-build3-code-critique-brief.md. Two material findings, five notes; one correction round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | The tier-1 evaluator reads the tier only from the root record snapshot; a goal raised after dispatch still admits tier-1 landings. Material. | Correction round: compare the parsed goal file's Tier at the landing base against 1. |
| F-2 | accepted | The receipt command runs under the generic five-minute local execution bound; the full battery exceeds it, so every gateWidth-full landing fails closed. Material. | Correction round: a dedicated configurable bound for the receipt command (landing.receipt-bound-min, default sized for the full battery) with a measured run recorded. |
| F-3 | noted | A hand-written receipt file passes on content alone; binding is posture equality around the command per the approved contract. For the design owner (a signed or engine-minted receipt). | none |
| F-4 | noted | Record-class paths are landable under tier-1 without the append-only rule; follows the design. For the design owner. | none |
| F-5 | noted | The floor list omits cmd/metasystem, so a small tier-1 landing could alter the receipt verb; the diff reproduces point 12 exactly. For the design owner (add cmd/metasystem/landing_verbs.go to the floor). | none |
| F-6 | noted | Two proofs narrower than the code: folded in the correction (a foreign-tree receipt at the right path; a working tree differing while the index matches). | Correction round adds the two legs. |
| F-7 | noted | The command package red (TestGroupOwnedLiveNonOwnerExitsNotOwned, a timing test) is pre-existing on main; recorded on fixture-suite-drift-after-approval-gate. | none |
