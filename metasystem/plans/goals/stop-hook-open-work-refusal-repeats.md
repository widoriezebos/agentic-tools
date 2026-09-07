# stop-hook-open-work-refusal-repeats

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the turn end is refused for work the seat cannot do, which costs turns and teaches seats to distrust the refusal, nothing unsafe is permitted; novelty 1: a marker file and two predicates on records that exist; exposure 3: every seat's every turn end while any stale plan exists in the tree; accumulation 1: it does not compound beyond the lost turns"
- Tier: 3
- Intent: The Stop hook's open-work verdict says 'This refusal does not repeat for the same work' and then repeats it: on m1d on 2026-09-06 the same two lines (OPEN-WORK plans/goal-scope-bounds-design.md with its literal '<one line, required>' placeholder, and OPEN-WORK plans/handoff-m1-2026-09-02.md naming a never-idle-analysis job that runs nowhere) refused three turn ends in a row, twice while a delegate job of this checkout was running and once together with a deadline expiry. Both plans are another seat's notes from 2026-09-03 and carry no work this seat can do or lawfully edit; the verdict also does not count a running delegate job as work in flight, so a seat that has dispatched and is waiting is told to 'do it now'. DONE means: the once-only promise holds per plan line across turns (a durable marker, not one that a deadline expiry loses); a running job or an open chain on the checkout counts as in flight for the verdict; and a plan file whose Next step is an unfilled template placeholder is reported as a template defect, not as open work for the seat.
- Origin: main
- Next step: BUILDING (m1d, 2026-09-07 03:05 CEST). Build brief landed 3815d0f1; implementer show-build1d-20260907 (Sol) running; code-critique brief landed (this commit's neighbour). Plan: validate conformance --stage review --job show-build1d-20260907 (delete regular files under the worktree's metasystem/artifacts/agents first if it refuses); seat replays go test ./internal/report/ ./cmd/metasystem/ and supervision-hook-fixtures.sh outside the sandbox on the reviewed tree; one Fable code critic (--reviews show-build1d-20260907, fresh --op show-cc1); if zero material: register-advance, dispatch.sh close, dispositions record, git apply --index --directory=metasystem of rounds/1/diff.patch from the repo root AND CHECK ITS EXIT CODE, land.sh -m <msg> --chain show-build1d-20260907 --direct-fix register-carriage --goal stop-hook-open-work-refusal-repeats --staged-only --allow-new-plan --skip-transport (accumulation 1: area-width, no full battery receipt; the fleet-wide red battery does not block it), go-build, metasystem up, goal done.
- OpenedAt: 2026-09-06T10:53:06Z
- Revision: 5
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
Integrity: sha256=dc052930ea738ad68785412de7d2bf53c293d16b20bf02aa1f76cd4a3c5a9326
