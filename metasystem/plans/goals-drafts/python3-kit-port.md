# Draft: python3-kit-port — retire the last python3 fixture dependency

Status: DRAFT for Wido (he ruled "fund full kit port" 2026-08-26,
answering the parked extractor-port decision package). Opens as a
goal only after he reads this draft — draft-first ruling R-2.

## What the critique priced vs what exists today

EPC-01 (extractor-port critique) priced a multi-slice arc because
python3 then pervaded the kit: pairs.py, compare.py,
system-fingerprint.py, provision blocks. Those helpers have since
been retired in the perl-removal/lease-retirement work. The
dependency-ratchet today pins python3 to EXACTLY three declared
sites (validate-metasystem.sh, preflight-commands.sh,
dispatch-fixtures.sh), and all three exist for one capability: the
TTY escalation driver in dispatch-fixtures.sh
(run_tty_agent_fixture_once), which needs a pseudo-terminal to
prove the --approve-escalation TTY path. The other two sites are
presence checks for that driver.

So the funded "full kit port" is one real port, not an arc of
translations: give the engine a pty runner and delete the last
python3.

## Slices

1. **Engine pty verb** (~3h): `metasystem proc pty-run --typed
   <line> --expect-exit N --cap-sec N -- command...` allocating a
   pty (creack/pty, the standard cgo-free package), feeding the
   scripted line, enforcing caps, emitting the transcript to
   stdout. Unit tests in Go against /bin/cat and a refusing stub.
   Oracle: byte-comparable transcript behavior with the python
   driver on the same fixture legs.
2. **Fixture conversion + ratchet flip** (~2h): dispatch-fixtures'
   TTY legs call the verb; delete the python heredoc and both
   presence checks; dependency-ratchet moves python3 from
   "declared sites" to the banned interpreter list. Full dispatch
   suite green is the acceptance.

Appetite: 5h total, single machine, codex builds both slices under
the standard loop (1 design round for slice 1 — translation with an
oracle; certification each slice).

## Interaction with parked goals

- extractor-port (os-dependency-reduction slice two) concludes as
  superseded the moment this opens: its subject matter is contained
  in slice 2.
- New Go dependency creack/pty must ride the go.mod ratchet note;
  it is the only new dependency and is stdlib-adjacent.

## Risks

- macOS/Linux pty semantics differ mildly (EIO on closed slave);
  the python driver already handles this — port its handling, the
  Go package exposes the same surface.
- If creack/pty is unwanted as a dependency, fallback is
  `script -q` invocation, but flag differences across BSD/GNU make
  that the worse trade; the draft recommends the package.
