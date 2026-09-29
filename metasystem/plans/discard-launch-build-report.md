# Build report: discard a stopped launch from the fleet page

Branch `discard-launch` in `/Users/wido/LocalStorage/GitHub/agentic-tools-discard`, built from
`plans/discard-launch-build-brief.md`. Nothing was pushed or merged.

## What was built

1. **Discard launch.** A stopped (failed) launch card (`LaunchCard.tsx`, `Full` → `Retry`) has
   a secondary button, "Discard launch", beside Retry in the same row of actions. It is never
   disabled and asks for no confirmation. A refusal renders through `Trouble` with the refusal's
   code.
2. **Durable on the server.**
   - `launch.Record` gains `DiscardedAt *string` (`json:"discardedAt"`, RFC3339 UTC). The record
     is kept.
   - `launch.Discard(checkout, id, now)` (`internal/seat/launch/record.go`) loads the record, sets
     the field and writes it with `SaveAt`, the atomic writer every record already uses. It is
     idempotent: a record that is already discarded is returned as it is and nothing is written.
   - Refusals: `SEAT_LAUNCH_ID_INVALID` for a bad id (the existing `Path` refusal),
     `SEAT_LAUNCH_UNKNOWN` for no such record (new), and `SEAT_LAUNCH_DISCARD_RUNNING` for a launch
     still `starting` or `running` (new, see Deviations). Both new codes have register rows
     anchored at `record.go#Discard`.
   - The route is `POST /api/fleet/launches/{id}/discard` (`internal/ui/httpd/discard.go`, route
     `discard-launch`). It is dispatched through `writeRouteOf`/`write`, so it gets the
     write policy every write gets: allowed host, same-site check, single allowed origin, POST
     only, and a bounded body that must be the empty object. It answers 200 with the record, 404
     for unknown, 409 for running and 422 otherwise, in the `writeActRefusal` shape with `code`.
     It has a `describe.go` row, and `Info.DiscardLaunch` is wired in `cmd/metasystem/ui.go` via
     `launchDiscarder`. Rewriting the record changes the launches directory's fingerprint, so the
     existing `LaunchWatch` announces the `fleet` event and other open pages re-read.
   - Payload: the record's field travels as it is, and `showsCard`/`cardFor` skip discarded
     launches (the smaller of the two options). The page capture for the Partner uses
     `showsCard`, so it skips them too. The fleet reading's text lines (`fleet/lines.go`) also
     leave discarded launches out.
3. **Durable Dismiss.** The `Folded` card's Dismiss calls the same route. FleetPane's
   `useState` "dismissed" string is gone. The card is now `visibleCard(launches, started,
   hidden)`: `hidden` holds the ids whose discard the server answered, which is an optimistic
   hide until the next read carries the mark. When the newest card is discarded, the next launch
   still worth a card takes its place, just as it will after the read.
4. **Truthful leftovers.** The fixed sentence from `launching.ts#failedRemedy` is removed. The
   fleet payload's launches are now `fleet.Launch{launch.Record; DestinationPresent bool}`.
   `Inputs.Present` is wired to the new exported `launch.Present` (the former private `present`,
   an `Lstat`). While the clone exists, the stopped card shows one line: "The clone at <path>
   stays on disk; delete it yourself." When it is gone, nothing is shown. `metasystem machine
   remove` does not exist on main: `intent_process.go` has only `machine list` and `machine
   start`, and `plans/goals/machine-remove.md` is queued and unbuilt. So no verb is named.
   Discard never deletes files.
5. Nothing was added for cancelling a running launch.

Also changed:
- The Retry refusal now carries its code to `Trouble` (`failureCode`), as the sheet's does.
  This follows the brief's rule that the card's failure sentence carries the refusal code.
- The walkthrough fixture (`internal/ui/httpd/walkthrough/fleet.go`, `main.go`) gained:
  - an older, already discarded launch (m1h), which the page must not draw;
  - an in-memory `fixtureDiscards` behind `DiscardLaunch`, so the button works in a browser;
  - `Present` true for the failed m1g clone, so the leftover line shows.

## Tests (each written before its code and failing against the code before it)

Go, `internal/seat/launch/discard_test.go`:
- `TestDiscardMarksAStoppedLaunchAndKeepsItsRecord`: the record is kept, the clone is untouched.
- `TestDiscardingADiscardedLaunchChangesNothingAndSucceeds`: the file is byte-identical and the
  original timestamp is kept.
- `TestDiscardRefusesALaunchStillRunningAndOneThatIsNotThere`: starting/running, unknown and
  invalid id.
- `TestPresentSaysWhetherTheDestinationIsStillOnDisk`

Go, `internal/ui/httpd/discard_test.go`:
- `TestADiscardMarksTheLaunchThePathNamesAndAnswersTheRecord`
- `TestADiscardRefusalCarriesItsCodeAndTheStatusThatFitsIt`
- `TestADiscardTakesTheWritePolicyAndNothingLess`: cross-origin 403, a body with fields 400,
  GET 405, and the act is not reached.
- `TestAnEngineThatCannotDiscardSaysSo`
- `describe_test.go#TestEveryWriteRouteIsAnActWithAHand` now lists `routeDiscardLaunch`.

Go, `internal/ui/fleet/launching_test.go`:
- `TestThePageSaysWhetherEachLaunchesCloneIsStillOnDisk`: this includes the JSON shape, with
  `destinationPresent` flattened beside the record's own fields.
- `TestTheReadingLeavesADiscardedLaunchOut`

Go, `cmd/metasystem/ui_launch_discard_test.go`:
- `TestLaunchDiscarderMarksTheRecordInThisCheckout`

Web (vitest):
- `src/fleet/LaunchCard.test.tsx`:
  - "removes nothing, and names the clone while it is still on disk". This replaces the old
    "is a directory you delete" test and asserts that no `machine remove` is named.
  - "says nothing about a clone that is gone"
  - "offers Discard launch beside Retry, always enabled"
  - The folded test now asserts the Dismiss button.
- `src/fleet/launching.test.ts`:
  - "leaves out a launch a human discarded"
  - "the card the fleet block shows" (four cases): newest when nothing was started; following
    the server's record of a started launch; hidden the moment the discard answers, before the
    read; kept hidden once the read carries the mark, even for a launch started here.
  - "what a stopped launch leaves on disk" (two cases)
- `src/fleet/discard.test.ts`:
  - "posts to the launch's own address beneath the fleet"
  - "carries a refusal's words and code to the trouble line"

## Deviations, and why

- **Hand: the checkout writes' policy, not a signed-in session.** The launch route asks for a
  signed-in session because a launch spends disk, a build and the human's authorization. A
  discard spends nothing, deletes nothing and records no authority: it rewrites one file under
  this checkout's `artifacts/`. That is `checkoutHand`'s definition, and asking for a session
  would put a sign-in prompt in front of a dismiss. There is also a structural reason. A
  `sessionHand` row counts as a goal act in `proposals_test.go#goalActs`, and that test would
  then require a Partner proposal catalogue entry for it. The route still takes the full write
  policy (host, same-site, origin, POST, bounded empty-object body). There is no act or audit
  record beyond the record's own `discardedAt`, and acts of this kind have none. If the brief
  meant "signed-in session" strictly, the change is small: gate `discardLaunch` with the same
  `signedIn`/`SessionValidFor` check as `launchMachine`, change the describe row to
  `sessionHand`, and add a proposal catalogue row.
- **A running or starting launch is refused (`SEAT_LAUNCH_DISCARD_RUNNING`, 409).** This is not
  a cancel. The verb rewrites the record from its own copy after every step, so a mark written
  mid-run would be written away and the card would come back. In practice, the one card that can
  show this refusal is a folded card whose machine joined while its record still says `running`.
  That window is brief, and the refusal renders through `Trouble` with its code.
- **Discarding the newest card surfaces the next older launch that is still worth a card.**
  This follows from "cardFor skips them". The old local "dismissed" string hid the whole block.
  Now an older failed launch that nobody discarded shows next, and it can be discarded in turn.
- Not changed: a retry through the CLI of a discarded launch keeps its `discardedAt`, because
  `launchRecordFor` does not clear it. No page path reaches it, since a discarded card offers
  no Retry.
- Noticed and left alone: `launch-machine` itself has no `describe.go` row on main, and
  `TestEveryWriteRouteIsAnActWithAHand` does not list it. That is outside this slice.

## Proof

All runs used `METASYSTEM_TESTING_WORKERS=9`, from `metasystem/`.

- `go run ./cmd/devgate static`: exit 0. "fast mode passed (dependency ratchet, parallel
  ratchet, gofmt, shell parse, vet, staticcheck, dead code, refusal register, SessionStart exit
  audit, Stop decision surface audit, build)".
- `go test ./internal/ui/... ./internal/seat/launch/ ./internal/refusal/`: all ok, exit 0.
- `go test ./cmd/metasystem/ -run 'Launch|Fleet|Machine'`: ok (138.7s).
- Web, from `internal/ui/web/_app`:
  - `npx -y -p node@24.21.0 sh -c 'npx vitest run'`: 111 files, 1628 tests passed.
  - `npx tsc --noEmit`: exit 0.
  - `npm run bundle`: rebuilt in the same commit as the web change, and
    `go test ./internal/ui/web/...` is ok.
- End to end on the walkthrough fixture (own port 127.0.0.1:47391, own TMPDIR under the
  scratchpad, stopped by its own pid):
  - `GET /api/fleet` carried `discardedAt` and `destinationPresent`.
  - `POST .../01M3BQ8000000000000000000A/discard` answered 200 with the mark, and a second POST
    answered 200 with the same timestamp.
  - In Chromium (Playwright): the stopped m1g card showed the leftover line, Retry, and
    "Discard launch". Pressing it removed the card at once, and it stayed gone after a reload.
