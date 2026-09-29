# g1-s72: signed in is enough — a machine launched from the browser is a human's own enrollment

- Kind: design
- Id: 01M3NZQN2WCRCZB8JVHE8XPH59
- Status: draft
- Goals: browser-interface

Wido, 2026-09-29, looking at the Launch sheet's "Your authorization"
section: "I see very little value in this extra information, this
field. If it is only for describing why I launched it, I know why I
launched it. So it's my machine because I'm launching it on my
machine." And: "the browser is my terminal if i am logged in there's no
distinction." On the proposal that engine enrollment accept the
signed-in session as full human enrollment and the form drop the
section: "Yes, do this." Author Fable. Every cite re-read at
`16b58b7e3`. Refines g1-s43 D3, which chose the temporary word and named
this step as the bridge design's (`plans/designs/user-interface-
design.md:206`: the browser principal "grants human authority for
human-originated actions without separate browser enrollment").

## 1. What exists and binds

1. **How a human-terminal enrollment is proven.** `steward arm` without
   the pair calls `requireHumanTerminal` (`cmd/metasystem/steward_verbs.go:353`),
   which classifies the arm's PARENT with `lease.ClassifyAt` and requires
   HUMAN (`process_verbs.go:440-455`). HUMAN is a caller with no
   recognised ancestor and a controlling terminal; a caller with neither
   is UNTRUSTED (`internal/lease/classify.go:445-478`). The arm then mints
   `humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal)`
   (`internal/steward/runner.go:337`). The identity carries the gate's
   VERDICT, not a proof: `Enrollment`, `MintedBy`, the witnessed
   generation (`identity.go:58-71`, `runner.go:861-869`). It never reads
   `human-terminal.json`; that enrollment is the goal verbs' (`humanauthority.Prove`).
2. **The temporary path** skips the classifier entirely
   (`steward_verbs.go:351-364`) and mints `temporary-word` with the word
   and date on the record (`runner.go:369`). Nothing enforces the date
   (g1-s43 1.2) and nothing refuses a temporary-word steward anything:
   the kind is read only to accept the record (`identity.go:156`) and to
   keep it out of fixture mode (`runner.go:818`); the word only to say
   TEMPORARY (`runner.go:1034`, `health.go:846`). Its standing is a label
   for a human to read, and any shell can mint it.
3. **The session proof** is minted in-process after the one-time code
   (`internal/ui/session/session.go:267`), with `observed` unexported, so
   only this server's memory ever holds a proof `SessionValidFor` accepts
   (`humanauthority/authority.go:311-320`); sessions live in the store's
   map (`session.go:272`). Acts run in-process under it, classified HUMAN
   (`internal/ui/act/act.go:190-208`), and the ledger already records a
   session approval as `authority=session`, beside the terminal's
   `proven` and unlike the relayed word's `ReviewBy`
   (`internal/goal/approval.go:388-400`). The goal domain already treats
   the session as the ruling says; the steward does not.
4. **The launch chain.** The route requires Live and
   `SessionValidFor(Sessions.Root())` (`httpd/launch.go:72-92`), then
   refuses an empty pair (`:103`); `launchStarter` IGNORES `signed`
   (`cmd/metasystem/ui_launch.go:40`, `_`), re-checks the pair and the
   dates, writes the record at 0600 (`atomicfile.go:85`) and spawns `seat
   launch` detached under setsid with the pair on argv (`:196`, `:214`); step 7 runs the clone's `steward arm` with the pair
   (`internal/seat/launch/sequence.go:671-672`) and refuses a resume
   without it (`:674-678`). Without the pair the arm classifies its
   caller, the setsid'd launch process, which has no terminal
   (`identity/terminal.go:9-14`): UNTRUSTED, refused. That is the crux.
5. **Where the date shows.** Only the launch record's `ReviewBy`
   (`record.go:136`): the card (`LaunchCard.tsx:134`), the folded line
   (`launching.ts:320`), the walkthrough fixture (`walkthrough/fleet.go:278`).
   The fleet payload and the presence record carry no enrollment fact. The
   Decisions inbox's review dates are goal approvals' (`decisions.go:826`).
6. `internal/seat/launch` imports `steward`, so `steward` cannot read a
   launch record; `cmd/metasystem` imports both.

## 2. What you want from the launch

- **L1. Signed in is enough.** No words to type, no date to pick.
- **L2. The machine is mine.** Enrolled as a human's, permanently, and
  its health says whose and from where.
- **L3. Retry is a press.**
- **L4. Nothing on the fleet page says temporary or names a review.**

## 3. The moment

You press Launch a machine: Nickname, Where, What it gets, Launch. The
card runs its steps; the enrollment step says "enrolled as wido from a
signed-in browser session"; the row appears and the card folds to "m1f
joined 2 min ago". At the new machine's terminal, `system check` reads
"enrollment generation 1 human-witnessed (engine …); from a signed-in
browser session of wido (session 01…, launch 01…)". A launch that
stopped at the build offers Retry and Discard launch, nothing to fill
in. A browser nobody is signed into is answered with the sign-in sheet,
and a session whose proof does not stand for this checkout is refused
before anything is written, as today.

## 4. Decisions

- D1. **The interface's launch is the gated act, and the record carries
  its verdict.** `launchStarter`, the one process holding an accepted
  proof, checks `signed.Proof.SessionValidFor(roots.StateRoot)` itself
  (today it trusts the route) and, on every launch and every retry,
  writes into the record `enrollment {kind: "human-session", provider,
  human, session, at}` from the proof's `ChannelProvider`, `ChannelUser`,
  `ChannelRef` and the server's clock, never from the body; then records
  the proof as the audit evidence every session act leaves,
  `RecordSessionProof(roots.StateRoot, "<launch>-<at>", "seat launch",
  proof)` (`authority.go:971`), before spawning; a proof that cannot be
  recorded is a launch that does not start. Nothing about the human
  travels on argv. Why here and why a verdict: the enrollment happens
  minutes later in a detached verb (g1-s43 D4), a parsed proof has no
  authority (`authority.go:202`), and the identity itself is a recorded
  verdict (1.1), so the verdict is what crosses. Rejected: the server
  minting the identity in-process (needs the built clone, so it either
  blocks a request for minutes or waits for a browser to come back, and
  it would make the server the new runner's parent); the verb verifying
  a reference against the session store (in memory; an endpoint and a
  one-time token is a cathedral); the facts on argv (the temporary
  path's own shape, forgeable by any shell, which is what made that path
  temporary).
- D2. **`steward arm --launch-record <path>`: the clone's arm reads the
  verdict, binds it, and classifies its caller.** In `cmd/metasystem`
  (1.6): mutually exclusive with the pair; the record must be owner-only
  and owned by the caller, as `VerifyIdentity` asks of the identity
  (`identity.go:134-139`), carry kind `human-session` with provider,
  human and session non-empty, and name as its destination the
  repository top of `--repo` after canonicalization, so one record arms
  one clone. Then the caller is classified as the terminal path
  classifies it (`humanTerminalCheck`'s injectable classifier,
  `process_verbs.go:31`, `:418`) and MAIN, DELEGATE, STEWARD, SUPERVISION
  and ADAPTER-SUPERVISOR are refused by name; HUMAN and a caller with no
  recognised ancestor pass, because the browser's detached launch has no
  terminal and the record is what vouches. `ui serve` and `seat launch`
  are neither agent runtimes nor steward plumbing (`classify.go:494-507`
  matches the installed binary's `steward` family only), so the real
  chain passes. Then `steward.ArmSessionWithLineage(repo, bin, session,
  lineage)`. Why the classification: today an agent's shell can arm only
  under a label (1.2); without this one existing call it could mint a
  permanent human enrollment with one command. The pair path and its
  TEMPORARY message stay for R-29-m2's case.
- D3. **A kind of its own: `human-session`.** `EnrollmentHumanSession`
  joins `VerifyIdentity`'s accepted kinds (`identity.go:156`; an unknown
  kind makes every reader, `OpenEnrolledBinary` included, refuse the
  identity as malformed); `InstallIdentity.Session *EnrolledSession
  {Provider, Human, Reference, Launch, From}` (the launching checkout,
  where the record and the proof artifact live); `MintedBy:
  "human-session"`; `mintPlan.Session`; the machine-rebuild carry keeps it
  beside `Enrollment` (`runner.go:483-485`); `EnrollmentProvenance`
  counts it human-witnessed with `human-terminal` and `human-word`
  (`runner.go:1021`) and names the human, the session and the launch.
  The witnessed generation is set as for every non-rebuild mint
  (`runner.go:861`). A fixture root still forces `fixture`
  (`runner.go:818`). `replace` is true, as the temporary path's
  (`runner.go:369`): a resume that must arm again after the bytes changed
  replaces the stale runner rather than reporting "already armed". Why
  not `human-terminal`: that kind says a person stood at this checkout's
  shell, and provenance readers say so; the record would say what did
  not happen.
- D4. **The sequencer carries the verdict, and the word leaves the
  launch.** Step 7 passes `--launch-record <path>` (`Sequencer.RecordPath`,
  set by `seat_launch.go` from the path it owns) when the record carries
  a session enrollment, else plain `steward arm`, whose own
  classification of a terminal caller decides as today; the resume
  refusal (`sequence.go:674-678`) goes; the step's words are "enrolled
  as <human> from a signed-in browser session" or "enrolled at this
  terminal". `Request` loses `Word`, `ReviewBy`, `ClientToday`; `Record`
  loses `ReviewBy` (old records still decode, `LoadAt` ignores unknown
  fields, `record.go:238`); `CodeWordRequired`, `CodeWordInvalid`,
  `CodeReviewDatePast`, `ValidDay` and `ReviewDateBefore` retire; `seat
  launch` loses its two flags and its pair check (`seat_launch.go:44-55`).
- D5. **The interface forgets the word.** Gone: the "Your authorization"
  section and `useSecret(draft.word)` (`LaunchSheet.tsx:69-70`, `:179-212`);
  `LaunchDraft.word/reviewBy`; the word and date checks in
  `blockedForLaunch` and its `now`; `TEMPORARY_RULE`, `RETRY_ASKS_AGAIN`,
  `REVIEW_DAYS`, `proposedReviewBy`, `earliestReviewBy`, `clientToday`,
  `reviewByRefusal`, `reviewDay`, `isoDay`; the folded line's clause
  (`launching.ts:320`, now "m1f joined 2 min ago"); the card's review
  line (`LaunchCard.tsx:134`); the Retry form's two fields, so Retry is
  one press beside Discard launch (`:184-262`); `Launch.reviewBy` and
  `LaunchRequest` is `{machine?, destination?, resume?}` (`api.ts:242`,
  `:276-286`); the help entries `temporary-word` and `review-by`
  (`terms.ts:74-75`, `:394-401`). The route's `launchBody` loses `Word`,
  `ReviewBy`, `Today`, and `launchNeedsAuthorization` with its refusal
  (`launch.go:46`, `:97-107`); the walkthrough's launches drop
  `ReviewBy` and its launch hook stamps the enrollment the signed session
  names. `useSecret` stays for the sign-in code (`SignInSheet.tsx:58`).
- D6. **Nothing else changes.** The pair on `steward arm` and on the goal
  relay verbs (`goalsync_mutations.go:2700-3415`); `human-terminal.json`
  and the goal verbs' proof; the session store; the launch's lock,
  record, reconciliation and discard; presence; the Partner's
  never-proposed fields (`uitools/propose.go:474` names a relay flag).

## 5. Step 1, the smallest thing that works

D1 to D6 as one slice, engine first. Not in it: a bound on a verdict's
age; revoking a session-enrolled machine from the page; re-arming the
fleet's existing temporary-word machines (a human re-arms at a terminal
or relaunches); the fleet card naming each machine's enrollment kind.

## 6. The records afterwards

The clone's `artifacts/agents/steward/identity.json`:

```
{ "repoIdentity": "/…/agentic-tools-m1f", "generation": 1,
  "installPath": "/…/agentic-tools-m1f/metasystem/bin/metasystem", "installDigest": "sha256:…",
  "mintedAt": "…", "enrollment": "human-session", "mintedBy": "human-session",
  "session": { "provider": "browser", "human": "wido", "reference": "<opaque>",
               "launch": "01M3…", "from": "/…/agentic-tools-ui" },
  "humanWitnessedGeneration": 1, "humanWitnessedAt": "…", "engineBuild": "<stamp>" }
```

No `temporaryHumanWord`, no `reviewBy`. The launch record gains
`"enrollment": {"kind":"human-session","provider":"browser","human":"wido",
"session":"<opaque>","at":"…"}` and loses `reviewBy`; the proof artifact
is `artifacts/agents/authority/proofs/<launch>-<at>.json` under the
launching seat's state root, one per launch and per retry. `POST
/api/fleet/launch` takes `{machine, destination}` or `{resume}`; its
session refusals are unchanged.

## 7. Not here, later

An age bound on the record's verdict; revocation from the page; a
migration of temporary-word machines; the kind on the fleet card; a
terminal-run `seat launch --resume` of a browser record enrolls from the
record, which is honest and may one day want a choice.

## 8. Verification and box

Go. `steward`: `VerifyIdentity` accepts `human-session` and still
refuses an unknown kind; `ArmSessionWithLineage` in the non-fixture bed
(`rearm_test.go:1049`'s) mints the kind with the session fields, no
word, no `ReviewBy`, the witnessed generation set; the rebuild carry
keeps `Session`; the provenance line. `cmd`: the pair beside
`--launch-record`, a group-readable record, a missing field and a
destination that is another checkout, each refused by name; through the
injected classifier, DELEGATE and STEWARD refused, HUMAN and UNTRUSTED
admitted. `seat/launch`: the command list carries `--launch-record` and
no word (`sequence_test.go:47`); a record without an enrollment runs
plain arm; the resume that asked for the word (`:241`) arms with the
record's; the record round-trips without `ReviewBy`. `ui_launch`: **the
starter refuses a nil session and a proof not `SessionValidFor` the
state root**, beside the route's `TestLaunchingRefusesEveryHandButASignedInOne`
and `TestALiveSessionWithNoProofLaunchesNothing` (`launch_test.go:58`,
`:171`), which stay; the enrollment is stamped from the proof, not the
body; the proof artifact exists before the spawn and an unrecordable
proof starts nothing; a retry re-stamps. `httpd`: the signed-in launch
asks for `{Machine, Destination}` alone (`:74`); the no-authorization
and word-in-no-answer tests (`:98`, `:194`) retire with the word. **One
end-to-end test names the ruling:** a record and proof written by the
starter under a signed session, the sequencer's arm command run for real
as `steward arm --launch-record` against a steward bed, the identity
read back: `Enrollment == "human-session"`, `ReviewBy == ""`,
`TemporaryHumanWord == ""`, `Session.Human` the signed handle.
Frontend: `blockedForLaunch` stops at the destination; the review-date
blocks (`launching.test.ts:141`, `:239`) go; the fold line without its
clause (`:230`); no review line on any card and Retry one press
(`LaunchCard.test.tsx:108`); no authorization section; the help ids.
Walkthrough: the sheet and a folded card at 1280 and 400, light and
dark. Build notes: a commit under `web/_app` rebundles in the same
commit (`TestBundleIsCurrent`, `internal/ui/web/bundle_test.go`); no new
package, so no coverage floor. Rollout proof on this host: launch from
the page, read the clone's `system check` line. Box: Astra's critique
(two rounds), one Opus 5.5 lane, one Sol read with one fix round; 120 to
200 job-minutes.

## 9. Self-grade

High on D4 and D5: removals. High on D3: one kind, one struct, three
readers. Medium on D1 and D2: the verdict crosses a process boundary
the way the identity's own verdict already does, and the caller check
keeps today's line between a labelled and a permanent enrollment; what
it cannot do is verify the record was the server's, which is the
same-user boundary this repository draws everywhere (`identity.go:8`).
Weakest: a browser launch on a server an agent started is refused at
the arm with the classifier's words on the card, and the card's remedy
is a terminal; the act layer already refuses such a server, so this is
consistent rather than kind. Risks weighed: an old `running` record
without an enrollment resumed from the page (the retry stamps it); two
checkouts' records (each names its destination, so a record cannot arm
the other's clone); the record rewritten by the verb after the stamp
(the verb rewrites the whole record from the one it loaded, so the
stamp survives; the sequencer never touches the field).
