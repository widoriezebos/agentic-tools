# python3-kit-port

- State: approved
- Priority: 3
- Sequence: 92
- Intent: Retire the kit's last python3 dependency: the pty-based TTY escalation driver becomes an engine verb, and python3 moves to the banned-interpreter list (Wido funded the full port 2026-08-26; confirmed open-from-draft 2026-08-27; draft: plans/goals-drafts/python3-kit-port.md)
- Origin: main
- Next step: Appetite: 5h, two slices per the draft. Slice 1 (3h): engine 'proc pty-run' verb via creack/pty (typed line, expected exit, caps, transcript out) with Go unit tests; oracle is transcript parity with the python driver on the same legs. Slice 2 (2h): dispatch-fixtures' TTY legs call the verb; delete the python heredoc and both presence checks; dependency-ratchet moves python3 from declared-sites to banned. Full dispatch suite green is acceptance. Concludes extractor-port (os-dependency-reduction slice two) as superseded on open — its subject is contained in slice 2. New dependency creack/pty rides the go.mod note.
- OpenedAt: 2026-08-27T06:07:48Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=faf8a58ed7b395968477476f7ee741ce8c8521b337a0ee9c31726b06ae7e1295

History:
- 2026-08-27T06:07:48Z PVR42MBNXJG8VYCNEMTZ6TBB8P-m2-bc1be9cb open actor=human:wido targets=python3-kit-port
- 2026-09-01T20:29:19Z SXT4WPENF46QYHNZ8F0CXYWFN4-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=python3-kit-port
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=python3-kit-port reason=sweep
- 2026-09-08T16:04:31Z QV7WEVTPT820BPV3PM236MJF4G-m1-7cd0bd60 set-priority actor=human:Wido targets=python3-kit-port reason=priority-order subject=python3-kit-port from=unranked to=3:92 requested-sequence=92
Integrity: sha256=a02642aa4b72df46396d585635f52e79d538e9993af57a193d92bd3f14436472
