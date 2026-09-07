# stop-hook-open-work-refusal-repeats

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the turn end is refused for work the seat cannot do, which costs turns and teaches seats to distrust the refusal, nothing unsafe is permitted; novelty 1: a marker file and two predicates on records that exist; exposure 3: every seat's every turn end while any stale plan exists in the tree; accumulation 1: it does not compound beyond the lost turns"
- Tier: 3
- Intent: The Stop hook's open-work verdict says 'This refusal does not repeat for the same work' and then repeats it: on m1d on 2026-09-06 the same two lines (OPEN-WORK plans/goal-scope-bounds-design.md with its literal '<one line, required>' placeholder, and OPEN-WORK plans/handoff-m1-2026-09-02.md naming a never-idle-analysis job that runs nowhere) refused three turn ends in a row, twice while a delegate job of this checkout was running and once together with a deadline expiry. Both plans are another seat's notes from 2026-09-03 and carry no work this seat can do or lawfully edit; the verdict also does not count a running delegate job as work in flight, so a seat that has dispatched and is waiting is told to 'do it now'. DONE means: the once-only promise holds per plan line across turns (a durable marker, not one that a deadline expiry loses); a running job or an open chain on the checkout counts as in flight for the verdict; and a plan file whose Next step is an unfilled template placeholder is reported as a template defect, not as open work for the seat.
- Origin: main
- Next step: FOLDING (m1d, 2026-09-07 03:20 CEST). Round one (show-build1d-20260907, reviewed tree d512426b64fe1103aff6c45bf38b497efa6c53bb) replayed green outside the sandbox (internal/report, cmd/metasystem, the whole supervision-hook suite) but critic show-cc1-20260907 found five material defects: the verdict ignores open chains (SHO-01), a corrupt seen-state blocks every stop (SHO-02), the deadline path writes seen-state and spends refusals (SHO-03), the digest covers only the clipped 200 bytes (SHO-04), the file grows without bound (SHO-05); two notes. Fold brief landed 88761c24; follow-up job show-build1d-20260907-r2 running; round-two critique brief landed. Box: 2 of 10 attempts, 240 of 1200 minutes before the fold. NEXT: conformance --job show-build1d-20260907-r2; replays (go test internal/report cmd/metasystem; supervision-hook-fixtures.sh); critic (Fable, --reviews show-build1d-20260907-r2, fresh --op show-cc2); register-advance; if zero material: close, dispositions r1 and r2, git apply of rounds/2/diff.patch with its exit code checked, land.sh --chain show-build1d-20260907 (area-width), go-build, up, goal done.
- OpenedAt: 2026-09-06T10:53:06Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:52:20Z revision=2 opid=Q0HMY137H80MMNZMS7PMA8FPSE-m1-7cd0bd60 authority=proven digest=93ab5bcd8ccb820b62d295da654d52a6446a72410ab8a772685938718e4bc683
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=3 at=2026-09-07T00:47:48Z
- Claimed: machine=m1d lineage=main-1788683763-71870-f7f607 at=2026-09-07T00:47:01Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1d claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T10:53:06Z YP7WG37N4VN1X8VMS5RHV2JWXG-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-open-work-refusal-repeats
- 2026-09-06T12:52:20Z Q0HMY137H80MMNZMS7PMA8FPSE-m1-7cd0bd60 approve actor=human:Wido targets=human-goal-verbs-forgiving,repo-root-paths-ride-agent-commits-unjudged,stop-deadline-parent-trusts-ps,stop-hook-open-work-refusal-repeats,verbs-match-intent
- 2026-09-07T00:47:01Z AEEHJBWD71JFSAPGQSPT9XWYG2-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-open-work-refusal-repeats
- 2026-09-07T00:47:48Z 8MAYZ9F0TQHZ3AHJKD712C2BPP-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-open-work-refusal-repeats
- 2026-09-07T00:48:45Z QRQTDZ51S93X1KD4B8N5EFXB25-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-open-work-refusal-repeats
- 2026-09-07T01:14:24Z EHBPJJ0AQMMZ7CC3WS6CPH89BX-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-open-work-refusal-repeats
Integrity: sha256=1605ebf45344be8cd7718c17ffeddd7c4b99a7a66ce82c8b08cff67fed57b4aa
