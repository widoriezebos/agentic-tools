# events-package-has-a-test-group

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a regression in the event emitter that wait rows and wait measure read can land unproven, but no record is corrupted by the missing group itself; novelty 1: one Go group and one surface path, following dozens of existing groups; exposure 2: every landing that touches internal/events; accumulation 1: each unproven landing is one more untested change, and the fix stops the accumulation at once"
- Tier: 2
- Intent: No testing.json group runs the Go tests of metasystem/internal/events, and no surface names the package, so a change there falls to the residual surface and lands without its own tests ever running. Found on 2026-09-16 when coordinator-wakes-on-events-not-polls changed internal/events/emit.go (landed c900c08d4): its deep proof selected and passed 53 groups, none of which ran the package, and the seat ran go test -count=1 ./internal/events by hand (ok, 0.25s). DONE: (1) a Go group in metasystem/testing.json runs go test over internal/events with tests all; (2) a change under metasystem/internal/events/** selects that group without narrowing what it selects today: the package has importers on the residual surface (cmd/metasystem/event.go, internal/validate/authorization.go) and the fallback cannot declare dependsOn, so a narrow surface would drop their groups; (3) test plan on a candidate tree with a one-line change under metasystem/internal/events selects that group and every group it selects at 235e6d564, and the result is recorded in this goal's Next step; (4) the testing contract's own tests stay green, including TestContextTestingContractSelectsProof (context-budget mirrors the residual breadth), and the change lands with a deep proof because testing.json is a protected policy file.
- Origin: human
- Next step: Seat m1b: one Codex gpt-5.6-sol unit (testing.json only, no Go code change; the expected shape is the group added to the residual standard list and to context-budget's, which TestContextTestingContractSelectsProof holds equal), the seat's own read, test plan evidence on a one-line change, then a deep proof on the lane's next stack. No design round. Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 about 20:45 CEST (m1b drafted the open).
- OpenedAt: 2026-09-16T18:41:29Z
- Revision: 1
- Budget: elapsedLimit=2h attemptLimit=3 reservedJobMinutesLimit=120 activeJobLimit=1 reviewRoundLimit=1
- BudgetExceptions: 0

History:
- 2026-09-16T18:41:29Z AFWPGKZ6YKSFVAXZ4WTFWVKT95-m1e-c6925449 open actor=human:Wido targets=events-package-has-a-test-group
Integrity: sha256=a36758d81a86cd4d41fee343cda3d0826f4ecb43431382bc7fe7ba7ee351f17d
