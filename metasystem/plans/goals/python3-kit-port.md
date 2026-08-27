# python3-kit-port

- State: queued
- Intent: Retire the kit's last python3 dependency: the pty-based TTY escalation driver becomes an engine verb, and python3 moves to the banned-interpreter list (Wido funded the full port 2026-08-26; confirmed open-from-draft 2026-08-27; draft: plans/goals-drafts/python3-kit-port.md)
- Origin: main
- Next step: Appetite: 5h, two slices per the draft. Slice 1 (3h): engine 'proc pty-run' verb via creack/pty (typed line, expected exit, caps, transcript out) with Go unit tests; oracle is transcript parity with the python driver on the same legs. Slice 2 (2h): dispatch-fixtures' TTY legs call the verb; delete the python heredoc and both presence checks; dependency-ratchet moves python3 from declared-sites to banned. Full dispatch suite green is acceptance. Concludes extractor-port (os-dependency-reduction slice two) as superseded on open — its subject is contained in slice 2. New dependency creack/pty rides the go.mod note.
- OpenedAt: 2026-08-27T06:07:48Z
- Revision: 1

History:
- 2026-08-27T06:07:48Z PVR42MBNXJG8VYCNEMTZ6TBB8P-m2-bc1be9cb open actor=human:wido targets=python3-kit-port
Integrity: sha256=7f11973ab5b2f4cea885960a899be8718e617a13e99a8360ae4cc09d278d3548
