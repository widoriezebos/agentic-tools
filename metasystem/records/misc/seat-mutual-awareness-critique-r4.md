# Design critique register: seat-mutual-awareness, round 4 (job sma-crit4c-20260907)

Design under review: metasystem/plans/seat-mutual-awareness-design.md revision 4 at commit 7f8c13a6 (SHA-256 49e2a8c7cae11fc0ed6af0e5d59325e8313a0d0e9d9723a736c70e6817ba2164). Critic runtime: codex, model {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}. Material findings: 3 of 3. The critic ran read-only and could not write this record itself; the seat rendered it from the critic's return (artifacts/agents/sma-crit4c-20260907/rounds/1/return.json) without editing the words. Ruling A (Wido, 2026-09-06: the rollout is operational) was not under review; no finding asks for a marker or a repair verb back.

## Gaps the critic declared

- No seat implementation exists yet, so none of the proposed seat fixtures or unit tests could be executed; the findings are grounded in the written slice gates and shipped rollout, budget, validation, and re-arm code.

- This read-only design-critic run cannot create metasystem/records/misc/seat-mutual-awareness-critique-r4.md; the orchestration seat must render that declared record from this return without changing the findings.

- The launcher classified this broad-read run as advisory, so independent context isolation is not proven even though the job record exposes the runtime session identifier.

## Findings

### SMA-C-25 (material: True, severity: high)

Claim: Seat Mutual Awareness finding SMA-C-25 is reopened: the operational rollout can declare a machine ready to hear questions without proving that its steward reader was re-armed. Section 8 allows the seat of record to accept the `engineBuild` field from `metasystem supervise status` as confirmation that the machine runs the new engine. The shipped command fills that field from the build stamp of the command process itself and never reads the enrolled steward or runner. A rebuilt executable can therefore display the new stamp even when `metasystem up` failed to replace the old runner. Because section 10 relies on this confirmation before exposing `seat ask`, an implementer following metasystem/plans/seat-mutual-awareness-design.md can again permit an ask to a member that cannot hear it. Resolving this does not require restoring a marker or repair verb; every allowed confirmation must prove the running reader generation.

Evidence: Metasystem/plans/seat-mutual-awareness-design.md:649-662 admits `metasystem supervise status` field `engineBuild` as one of three alternative confirmations, and lines 873-880 rely on that rollout rule to close SMA-C-25. Metasystem/cmd/metasystem/supervise.go:105-159 sets `engineBuild` directly to `supervise.BuildStamp` and inspects only supervision owner and state files. Metasystem/internal/up/up.go:561-605 separately performs and records re-arm, proving that invoking rebuilt bytes and successfully replacing the enrolled runner are distinct states.

### SMA-C-26 (material: True, severity: high)

Claim: Seat Mutual Awareness finding SMA-C-26: the first implementation slice has an impossible proof gate. Section 10 requires the complete SMA-F-SPOILED-TIP fixture before slice one can land, but that fixture requires the next steward tick to republish a presence record after the human removes the spoiled file. Presence publication is owned by slice two, not slice one. An implementer must therefore weaken or skip the assertion, or build slice-two behavior early; both differ from the declared slice contract. Metasystem/plans/seat-mutual-awareness-design.md must assign each assertion at or after the slice that owns it.

Evidence: Metasystem/plans/seat-mutual-awareness-design.md:745-761 defines SMA-F-SPOILED-TIP, including `A's next tick republishes presence` at line 754. Lines 882-889 make all of SMA-F-SPOILED-TIP a slice-one gate. Lines 890-894 place presence in the tick in slice two. A repository search of metasystem/internal/goal, metasystem/internal/steward, metasystem/internal/up, and metasystem/cmd/metasystem found no existing ValidateSeatTree, RunSeatAttention, seat-member, or seat-presence implementation that could satisfy the assertion before these slices land.

### SMA-C-20 (material: True, severity: high)

Claim: Seat Mutual Awareness finding SMA-C-20 is reopened: the five-member implementation budget authorizes more work than the declared fifteen reservations. Section 10 adds five earlier reservations to the future box and asks Wido for twenty attempts and 2,400 reserved job-minutes. The shipped `goal set-budget` operation binds a fresh accounting revision, and budget projection ignores every job from an earlier revision; after approval, all twenty attempts and 2,400 minutes would be newly available. The declared implementation needs fifteen attempts and 1,800 minutes, leaving five unnamed attempts and 600 minutes of authority. Metasystem/plans/seat-mutual-awareness-design.md must either request the exact declared box or name the additional reservations and what they build.

Evidence: Metasystem/plans/seat-mutual-awareness-design.md:915-932 correctly totals the future work as fifteen reservations and 1,800 minutes but requests twenty and 2,400. Metasystem/internal/goal/verbs.go:745-760 calls bindClaim after set-budget; bindClaim at lines 252-268 sets AccountingRevision to the new revision. Metasystem/internal/dispatch/budget.go:239-253 reads that accounting revision and lines 344-360 skip older job records. The current goal at metasystem/plans/goals/seat-mutual-awareness.md:15 has accountingRevision 24. Its authoritative revision-24 job records contain two reservations totaling 240 minutes, while six reservations totaling 720 minutes exist after the 2026-09-06 re-approval, so the design's intermediate count of five is supported by neither accounting scope.

## Rigor

- SMA-C-25: severe; reopening trigger: Re-examine when every confirmation allowed by the rollout rule reads a successful re-arm outcome or the enrolled running steward's generation, and a focused fixture proves that merely invoking rebuilt bytes cannot satisfy the gate.

- SMA-C-26: severe; reopening trigger: Re-examine when the slice-one gate uses only slice-one capabilities and the presence-republication assertion is assigned to slice two or split into a separately named, slice-owned proof.

- SMA-C-20: severe; reopening trigger: Re-examine when the requested five-member budget matches the reservations available after set-budget starts its fresh accounting revision, with every authorized attempt and reserved minute mapped to a named build or review artifact.
