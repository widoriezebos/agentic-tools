# Dispositions: code critique round 3 of slice 1 (phd-build1-crit4-20260910)

Goal proof-groups-detect-hangs-by-progress-not-the-clock. Reviewed: chain
phd-build1-20260910 round 12, tree ca14dbc6129f24cff17781d286348b80174fcf5e.
Critic: Opus 5 (claude runtime). Three material findings, two notes.
Decisions on the design page under "Build decisions".

| Finding | Severity | Disposition |
| --- | --- | --- |
| PHD-15 the host-wait row is decided by a count of scripted samples, which a late tick breaks | medium | Folded (D-R13-1): the fixture seam gains `OnReading`; the row serves waiting samples until the reading is reported. |
| PHD-16 on Linux the counter decreases when a non-root member reaps its own child (D-R11-5 read literally) | medium | Folded (D-R13-2): every live Linux member counts its own time plus its own reaped-children fields, no retention; the build-tagged row adds a grandchild reaped by a non-root member. Replaces D-R11-5. |
| PHD-17 nothing tests Darwin retention of vanished members | low | Folded (D-R13-3): a scripted row that fails without retention. |
| PHD-18 stage-result growth can move the last-output time backwards | note | Folded (D-R13-4): the later of the two times is kept. |
| PHD-19 the quiet run's share is not measured before the wall comparison (PHD-06, PHD-12 again) | note | Accepted; proof row 4 asks for the comparison and the measured share precondition guards it. |

Round 13 carries the fold; the fourth code critique reviews round 13.
