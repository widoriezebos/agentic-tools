# events-package-has-a-test-group

- State: claimed
- Priority: 1
- Sequence: 78
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a regression in the event emitter that wait rows and wait measure read can land unproven, but no record is corrupted by the missing group itself; novelty 1: one Go group and one surface path, following dozens of existing groups; exposure 2: every landing that touches internal/events; accumulation 1: each unproven landing is one more untested change, and the fix stops the accumulation at once"
- Tier: 2
- Intent: No testing.json group runs the Go tests of metasystem/internal/events, and no surface names the package, so a change there falls to the residual surface and lands without its own tests ever running. Found on 2026-09-16 when coordinator-wakes-on-events-not-polls changed internal/events/emit.go (landed c900c08d4): its deep proof selected and passed 53 groups, none of which ran the package, and the seat ran go test -count=1 ./internal/events by hand (ok, 0.25s). DONE: (1) a Go group in metasystem/testing.json runs go test over internal/events with tests all; (2) a change under metasystem/internal/events/** selects that group without narrowing what it selects today: the package has importers on the residual surface (cmd/metasystem/event.go, internal/validate/authorization.go) and the fallback cannot declare dependsOn, so a narrow surface would drop their groups; (3) test plan on a candidate tree with a one-line change under metasystem/internal/events selects that group and every group it selects at 235e6d564, and the result is recorded in this goal's Next step; (4) the testing contract's own tests stay green, including TestContextTestingContractSelectsProof (context-budget mirrors the residual breadth), and the change lands with a deep proof because testing.json is a protected policy file.
- Origin: human
- Next step: Seat m1b: one Codex gpt-5.6-sol unit (testing.json only, no Go code change; the expected shape is the group added to the residual standard list and to context-budget's, which TestContextTestingContractSelectsProof holds equal), the seat's own read, test plan evidence on a one-line change, then a deep proof on the lane's next stack. No design round. Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 about 20:45 CEST (m1b drafted the open).
- OpenedAt: 2026-09-16T18:41:29Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-16T18:41:36Z revision=3 opid=Y4SHVAC3108FGYZD6MS9MS6E1N-m1e-c6925449 authority=proven digest=ea15b55d2e1e05afbc7c732a380b5fdcae2d06fef110f62867083d93cb259b40
- Sliced: machine=m1b lineage=main-1789560571-22295-8d907a revision=4 at=2026-09-16T18:44:51Z
- Claimed: machine=m1b lineage=main-1789560571-22295-8d907a at=2026-09-16T18:44:37Z revision=4 accountingRevision=4 episodeAt=2026-09-16T18:44:37Z episodeRevision=4
- StopCapability: generation=4 revision=4 machine=m1b claimEpoch=3 fenceEpoch=0

History:
- 2026-09-16T18:41:29Z AFWPGKZ6YKSFVAXZ4WTFWVKT95-m1e-c6925449 open actor=human:Wido targets=events-package-has-a-test-group
- 2026-09-16T18:41:33Z B0Q2Q2AJJMK9FSWYJCWXN5G665-m1e-c6925449 set-priority actor=human:Wido targets=census-lifecycle-scenario-holds-under-load,events-package-has-a-test-group,pipelines-never-lose-a-truncated-producer reason=priority-order subject=events-package-has-a-test-group from=unranked to=1:78 requested-sequence=78
- 2026-09-16T18:41:36Z Y4SHVAC3108FGYZD6MS9MS6E1N-m1e-c6925449 approve actor=human:Wido targets=events-package-has-a-test-group
- 2026-09-16T18:44:37Z DWBTAC4BWBQ23KQ3G0PWT4ZE76-m1b-d0e95724 claim actor=m1b+main-1789560571-22295-8d907a targets=events-package-has-a-test-group
- 2026-09-16T18:44:51Z JGNAAVYX6395P05PABMETCSQW3-m1b-d0e95724 slice-start actor=m1b+main-1789560571-22295-8d907a targets=events-package-has-a-test-group
Integrity: sha256=f6fed9be3cd577bbd650fadf4666586d51aa4860266031719f5c3e276c1b82b2
