# Read round 2: fixture-children unit 1 — U1-1 closure only

Reviewer did not write this change. Worktree
`/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/wt-fcu1`.
Scope of this round: verify U1-1 only, plus amendment consistency, boundary, and the line count.

## U1-1: CLOSED

U1-1 is fixed and the distinction is real in the kernel, not just in the assignment. `identity_darwin.go:95` is
the only assignment to `EnvironKnown` on Darwin (`grep -rn EnvironKnown` over the tree: one production line on
Darwin, one on Linux, three in tests) and it reads `exact.EnvironKnown = environ != nil`; the `else if` fallback
branch sets `ExeKnown` only, and a failed `procArgsAndExecutable` leaves `Environ` nil and the flag false.
Measured on this host (darwin/arm64, uid 501) with a diagnostic test in a throwaway copy of the module at
`.../scratchpad/g18/mut2`, dumping the raw `kern.procargs2` buffer and the bytes remaining after argv:
a `/bin/sh -c 'sleep 30; true'` child started with `METASYSTEM_IDENTITY_TEST=known` gives `len(raw)=46 argc=3
bytesAfterArgv=0 tail=""`, so the kernel truncates the buffer immediately after argv — no environment region, no
NUL padding, no apple strings — and the parse returns `environ==nil`, probe returns `environKnown=false len=0`;
a `/bin/cat` child behaves the same; the go test binary itself gives `bytesAfterArgv=3236`, `environ==nil:false`,
`environKnown=true len=59`. So "region absent" is nil and "region present" is non-nil, and no observed path
reports a refused read as a known empty environment. The builder's mutation reproduces: reverting line 95 to
`exact.EnvironKnown = true` makes `TestProbeRestrictedShellDoesNotClaimKnownEmptyEnvironment` fail with
`identity_test.go:241: restricted child environment reported as known empty`. A second, stronger mutation I ran
(collapse the guard so `procArgsAndExecutable` always returns `environ := []string{}`, the shape a NUL-padded
trailing region would produce) fails the same way, so the witness pins the nil-versus-empty discriminator itself
and not merely the unconditional assignment. The package is green after the fix: `gofmt -l` clean on both touched
files, `go vet ./internal/identity` clean, `go test -race -count=1 ./internal/identity` ok 1.5s in the worktree.
Linux is untouched by this round: `identity_linux.go` was last written at 10:14 while `identity_darwin.go` (10:39)
and `identity_test.go` (10:38) carry the fix, the diff to `identity_test.go` adds exactly one function
(the new test), and `readEnviron` still returns `known=false` on any read error and `([]string{}, true)` only
after a successful read of an empty `/proc/<pid>/environ` — a genuinely proven empty environment, which is the
correct Linux meaning.

## Amendment consistency: consistent, and unit 1 is what makes the two-slot carrier buildable

Nothing in unit 1 contradicts two carrier slots with one matcher reading both. `Exact` now carries both slots with
an honest known-flag on each: argv reads fine on a restricted shell (the diag above reads all three argv words of
`/bin/sh`), and environment now reads `EnvironKnown=false` there instead of "known empty" — which is exactly the
input a both-slots matcher needs, because a shell whose environment is unreadable must fall through to the argv
slot rather than be read as "carries no tag". The widening itself belongs to a later unit: `AliveTaggedRef`
(`internal/identity/identity.go:227-244`) is unchanged and still argv-only, so today a process carrying the tag
only in its environment reads Dead; `HasExactToken(argv []string, tag string)` is slice-shaped and takes either
slot without a signature change. Neither is a unit 1 defect under this brief.

## New material findings this round: none

Notes, not material:
- On Darwin the kernel's exec path for a `/bin/sh` child comes back as `/bin/bash` (diag: `argv=[/bin/sh -c ...]
  exe="/bin/bash"`). The value is truthful and `Exe` is documented as the kernel's path, but any later matcher
  that keys a shell fixture on `exe == "/bin/sh"` will miss. Worth one line in the design before a consumer
  matches on `Exe`.
- The state `EnvironKnown=true, len(Environ)==0` is still representable on Darwin if a kernel ever left a
  NUL-padded trailing region. Not observed on this host, and the new test would then fail loudly on that host
  rather than mis-report, so it is a visible failure mode, not a silent one.
- N1-N10 from round 1 stand as written; the fix round touched only `identity_darwin.go` and `identity_test.go`.

## Boundary and size

Boundary clean and unchanged from round 1: the same nine files, nothing from units 2 to 8 — no scan, no custodian
wiring, no run-owner export, no `testutil.Fixture`, no verb, no health or launcher consumer. (`custodian.go`,
`survivors.go`, `tagstate.go` and `fixture.go` in this package are pre-existing trunk files, untouched.)
Size, excluding `read-fcu1.md` which is review output: `git diff --numstat HEAD` gives 307 insertions and 21
deletions, **328 changed lines**, against the 340 the seat raised for this round. Under the cap; not a finding.

## What I could not check

- Anything Linux by execution. No Linux host in this session. The Linux claim above is from reading
  `readEnviron` and the `Probe` guard plus file mtimes showing the fix round did not touch that file; the Linux
  leg of witness 1 and the cross-process readability of `/proc/<pid>/environ` still need a VM run of
  `go test -race -count=1 ./internal/identity`.
- `bash scripts/agents/go-gate.sh --fast`, deliberately not run (whole-repo gate, seat's to run).
- The restricted-read behaviour is read off this Mac's kernel, not off XNU source, so it is one OS build's
  observed truncation rule, not a proven invariant across macOS versions.
- The other packages in the diff were not re-run this round; the fix touched neither, and round 1 had them green
  (`./internal/proofrun` ok 128.4s, `./internal/census` ok 3.4s).

## Tool calls used

8 of 12: seven bash calls, one write.
