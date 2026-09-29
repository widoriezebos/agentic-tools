# Build brief: discard a stopped launch from the fleet page

**Wido, 2026-09-29, with a screenshot of a stopped testbed launch card:** "I tried to add
a fleet member and that failed. I don't want to retry but the error message stays there
so I would like to have a way to dismiss that one. Cancel, dismiss but not retry. Figure
out from a UX perspective what the best way to handle this is." He accepted the UX
below, and said to build it here now if it doesn't interfere with another seat. Seat
ui-07 (fleet page lane, g1-s43) confirmed it doesn't.

## Workspace

The worktree `/Users/wido/LocalStorage/GitHub/agentic-tools-discard`, branch
`discard-launch`, based on origin/main. Run every Go command from `metasystem/`. Don't
merge main while you work; the coordinator merges before landing.

## What to build (step 1; nothing more)

1. **"Discard launch".** A quiet secondary button beside Retry on a stopped (failed)
   launch card (`internal/ui/web/_app/src/fleet/LaunchCard.tsx`, `Full`). It is always
   enabled. One press, no confirmation dialog: it deletes nothing.
2. **Durable on the server.** The press calls a new route that marks the launch record
   discarded: a field on the record (for example `discardedAt`, RFC3339 UTC), with the
   record kept for the trail and written the way records are already written. It is
   idempotent: discarding a discarded record changes nothing and succeeds. Find the
   record type in `internal/seat/launch/record.go` and the route that already serves
   launches under `internal/ui/httpd` (launch and retry: `cmd/metasystem/ui_launch.go`).
   The route follows the existing act routes' conventions: method, session, CSRF or
   nonce, the audit or act record if acts have one, and a `describe.go` entry like the
   other routes. The fleet payload leaves discarded records out of `cardFor`'s
   candidates, or carries the field and `cardFor` skips them; pick the smaller option.
3. **The joined-machine Dismiss becomes durable the same way.** The `Folded` card's
   Dismiss uses the same route. FleetPane's `useState` "dismissed" gives way to the
   server's record (an optimistic hide until the next read is fine).
4. **Truthful leftovers.** Replace the fixed sentence "Nothing here removes it: <path>
   is a directory you delete" (find where it is produced) as follows. The fleet payload
   reports whether the launch's destination directory still exists. While it exists,
   the stopped card shows one line: "The clone at <path> stays on disk; delete it, or
   remove it with metasystem machine remove". Check whether `machine remove` exists on
   main (`bin/metasystem machine`); if it doesn't, say "delete it yourself" and don't
   name a verb that doesn't exist. When the directory is gone, show nothing. Discard
   never deletes files.
5. **Out of scope:** cancelling a launch that is still running. Add nothing for it.

## Rules that bind this build (from the ui lane and the fleet)

- The card's failure sentence renders through `Trouble` (`src/shell/Trouble.tsx`) with
  the launch's refusal code, as the sheet does since ask-what-happened landed. The
  structural rule in `src/trouble.test.ts` refuses any other refusal shape.
- Any commit that touches `web/_app`, tests included, rebuilds the bundle in the same
  commit: `npx -y -p node@24.21.0 sh -c 'npm run bundle'` (Node 24.21.0 isn't installed
  globally). Otherwise `TestBundleIsCurrent` goes red on main.
- Since batch 14 the fast gate refuses dead Go code: the new field and route must be
  reached by production code or a test.
- Refusal-register rows, if you add any, anchor as `file.go#Symbol`.
- Walkthrough fixtures (`internal/ui/httpd/walkthrough/fleet.go`) gain a discarded
  launch if the page's fixtures need one.
- Never start, stop or restart the standing interface on 127.0.0.1:7878. Never kill
  processes by pattern (no pkill or killall); stop only a pid you started, by its number.
- Behaviour tests run with real Git unavailable. Use the existing seams; no `t.Setenv`.
- Each behaviour is held by a test that failed first: Go tests for the route and the
  record field, and the web tests (vitest, as the fleet tests already do) for the
  button, the durable dismissal and the leftover line.

## Proof, before you return

- `go run ./cmd/devgate static` passes.
- `go test` passes for `./internal/ui/...`, `./internal/seat/launch/`,
  `./internal/refusal/`, and `./cmd/metasystem/ -run 'Launch|Fleet|Machine'`.
- The web tests pass (`npm test` or the package's own script, run through the same npx
  Node 24.21.0 form).
- `METASYSTEM_TESTING_WORKERS=9`.

## Return

Write `metasystem/plans/discard-launch-build-report.md` and commit it. It lists what was
built, each test by name, any deviation and why, and the proof commands with their
results. Print it as your answer, with the last commit.
