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
   HUMAN (`process_verbs.go:440-455`): a caller with no recognised
   ancestor and a controlling terminal; one with neither is UNTRUSTED
   (`internal/lease/classify.go:445-478`). The arm mints
   `humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal)`
   (`internal/steward/runner.go:337`): the identity carries the gate's
   VERDICT, never a proof (`identity.go:58-71`, `runner.go:861-869`), and
   never reads `human-terminal.json`, which is the goal verbs' enrollment.
2. **The temporary path** skips the classifier (`steward_verbs.go:351-364`)
   and mints `temporary-word` with the word and date on the record
   (`runner.go:369`). Nothing enforces the date (g1-s43 1.2) and nothing
   refuses such a steward anything: the kind is read only to accept the
   record (`identity.go:156`) and to keep it out of fixture mode
   (`runner.go:818`), the word only to say TEMPORARY (`runner.go:1034`,
   `health.go:846`). A label for a human to read, and any shell mints it.
3. **The session proof** is minted in-process after the one-time code
   (`internal/ui/session/session.go:267`) with `observed` unexported, so
   only this server's memory holds a proof `SessionValidFor` accepts
   (`humanauthority/authority.go:311-320`). Acts run in-process under it,
   classified HUMAN (`internal/ui/act/act.go:190-208`), and the ledger
   records a session approval as `authority=session`, beside the
   terminal's `proven` and with no `ReviewBy` (`internal/goal/approval.go:388-400`):
   the goal domain already treats the session as the ruling says.
4. **The launch chain.** The route requires Live and
   `SessionValidFor(Sessions.Root())` (`httpd/launch.go:72-92`), then an
   empty pair is refused (`:103`); `launchStarter` IGNORES `signed`
   (`cmd/metasystem/ui_launch.go:40`), re-checks the pair and dates,
   writes the record at 0600 (`atomicfile.go:85`) and spawns `seat launch`
   detached under setsid with the pair on argv (`:196`, `:214`); step 7
   runs the clone's `steward arm` with the pair (`internal/seat/launch/
   sequence.go:671-672`), refuses a resume without it (`:674-678`) and
   writes "temporary enrollment, review due <date>" as the step's words
   (`:683`). Without the pair the arm classifies the setsid'd launch
   process, which has no terminal (`identity/terminal.go:9-14`):
   UNTRUSTED, refused. That is the crux.
5. **Where the date shows.** Only the launch record's `ReviewBy`
   (`record.go:136`): the card (`LaunchCard.tsx:134`), the folded line
   (`launching.ts:320`), the walkthrough fixture (`walkthrough/fleet.go:278`).
   The fleet payload and presence carry no enrollment fact; the Decisions
   inbox's review dates are goal approvals' (`decisions.go:826`). The one
   record on this host older than this slice (`launches/32ED1FCF…`, failed
   at the evidence-root step) is discarded.
6. `internal/seat/launch` imports `steward`, so `steward` cannot read a
   launch record; `cmd/metasystem` imports both.

## 2. What you want from the launch

- **L1. Signed in is enough.** No words to type, no date to pick.
- **L2. The machine is mine.** Enrolled as a human's, permanently, and
  its health says whose and from where.
- **L3. Retry is a press.** And nothing on the fleet page says
  temporary or names a review.

## 3. The moment

You press Launch a machine: Nickname, Where, What it gets, Launch. The
enrollment step says "enrolled as wido from a signed-in browser
session"; the card folds to "m1f joined 2 min ago". At the new machine,
`system check` reads "enrollment generation 1 human-witnessed (engine
…); from a signed-in browser session of wido (session 01…, launch
01…)". A stopped launch offers Retry and Discard launch, nothing to fill
in. A browser nobody is signed into gets the sign-in sheet, as today.

## 4. Decisions

- D1. **The interface's launch is the gated act, and the record carries
  its verdict.** `launchStarter`, the one process holding an accepted
  proof, checks `signed.Proof.SessionValidFor(roots.StateRoot)` itself
  (today it trusts the route) and writes into the record it creates
  `enrollment {kind: "human-session", provider, human, session, at}` from
  the proof's `ChannelProvider`, `ChannelUser`, `ChannelRef` and the
  server's clock, never from the body. A retry re-stamps a record that
  carries one under the current session; a record created without one
  is never stamped later (S72-02). Then it records the proof as the audit
  evidence every session act leaves, `RecordSessionProof(roots.StateRoot,
  "<launch>-<at>", "seat launch", proof)` (`authority.go:971`), before
  spawning; an unrecordable proof is a launch that does not start.
  Nothing about the human travels on argv. Why a verdict: the enrollment
  happens minutes later in a detached verb (g1-s43 D4), a parsed proof
  has no authority (`authority.go:202`), and the identity itself is a
  recorded verdict (1.1). Rejected: the server minting in-process (needs
  the built clone, so a request blocked for minutes or a wait for a
  browser); the verb verifying a reference against the in-memory session
  store (an endpoint and a token); the facts on argv (the temporary
  path's forgeable shape, which is what made it temporary).
- D2. **`steward arm --launch-record <path>`: the clone's arm reads the
  verdict, binds it, and classifies its caller.** In `cmd/metasystem`
  (1.6): mutually exclusive with the pair; the record owner-only and
  owned by the caller, as `VerifyIdentity` asks of the identity
  (`identity.go:134-139`); kind `human-session` with provider, human and
  session non-empty; its destination the repository top of `--repo` after
  canonicalization, so one record arms one clone. Then the caller is
  classified as the terminal path classifies it (`humanTerminalCheck`'s
  injectable classifier, `process_verbs.go:31`, `:418`): MAIN, DELEGATE,
  STEWARD, SUPERVISION and ADAPTER-SUPERVISOR refused by name; HUMAN and
  a caller with no recognised ancestor pass, since the browser's detached
  launch has no terminal and the record is what vouches. `ui serve` and
  `seat launch` are neither agent runtimes nor steward plumbing
  (`classify.go:494-507` matches the installed binary's `steward` family
  only), so the real chain passes. Then `steward.ArmSessionWithLineage(repo,
  bin, session, lineage)`. What this defends and what it does not
  (S72-01): a same-user adversary is out of scope repo-wide
  (`identity.go:8`); the repository defends against accident, a Claude or
  Codex session, a delegate or the target steward acting where a human
  must, and each is refused by name. The residual, stated: a same-user
  process that has escaped its ancestry, or another checkout's steward
  (recognition is target-repository local, `classify.go:513`), could
  forge a 0600 record and mint a `human-session` identity; ownership and
  destination binding do not prove the record was the server's. The same
  actor arms a labelled machine with the pair today (1.2). The pair path
  and its TEMPORARY message stay for R-29-m2's case.
- D3. **A kind of its own: `human-session`.** `EnrollmentHumanSession`
  joins `VerifyIdentity`'s accepted kinds (`identity.go:156`; an unknown
  kind makes every reader, `OpenEnrolledBinary` included, refuse the
  identity); `InstallIdentity.Session *EnrolledSession {Provider, Human,
  Reference, Launch, From}` (the launching checkout, where the record and
  proof live); `MintedBy: "human-session"`; `mintPlan.Session`; the
  machine-rebuild carry keeps it beside `Enrollment` (`runner.go:483-485`);
  `EnrollmentProvenance` counts it human-witnessed with `human-terminal`
  and `human-word` (`runner.go:1021`) and names the human, session and
  launch; the witnessed generation is set as for every non-rebuild mint
  (`:861`); a fixture root still forces `fixture` (`:818`); `replace` is
  true as the temporary path's (`:369`), so a resume that must arm again
  after the bytes changed replaces the stale runner. Not `human-terminal`,
  which says a person stood at this checkout's shell.
- D4. **The sequencer carries the verdict; the browser's word leaves,
  the terminal's stays.** Step 7 passes `--launch-record <path>`
  (`Sequencer.RecordPath`, set by `seat_launch.go` from the path it owns)
  when the record carries a session enrollment; with the pair and no
  enrollment it forwards the pair as today (`sequence.go:671-672`,
  S72-03); with neither, a fresh launch runs plain `steward arm`, the
  terminal caller's classification deciding, and a resume refuses under
  `CodeWordRequired` (`:674-678`) with new words: discard the launch on
  the fleet page and launch again, or resume at a terminal with the pair
  (S72-02: a record with no enrollment of its own is never armed by the
  new path, so a pre-slice clone's engine, kept at its HEAD by a resume,
  `:265`, is never given a flag it does not know). The pair beside a
  session-enrolled record is refused at the verb's edge under
  `CodeWordInvalid`. Step words: "enrolled as <human> from a signed-in
  browser session", "enrolled at this terminal", or the temporary line as
  today (`:683`). `seat launch` keeps its flags; `Request` keeps `Word`
  and `ReviewBy`, loses `ClientToday`; `Record` keeps `ReviewBy`, which
  only the pair path writes; `CodeReviewDatePast`, `ValidDay` and
  `ReviewDateBefore`, the interface's date rules, retire.
- D5. **The interface forgets the word.** Gone: the "Your authorization"
  section and `useSecret(draft.word)` (`LaunchSheet.tsx:69-70`, `:179-212`);
  `LaunchDraft.word/reviewBy`; the word and date checks in
  `blockedForLaunch` and its `now`; `TEMPORARY_RULE`, `RETRY_ASKS_AGAIN`,
  `REVIEW_DAYS`, `proposedReviewBy`, `earliestReviewBy`, `clientToday`,
  `reviewByRefusal`, `reviewDay`, `isoDay`; the folded line's clause
  (`launching.ts:320`); the card's review line (`LaunchCard.tsx:134`) and
  the Retry form's fields, so Retry is one press beside Discard launch
  (`:184-262`); `Launch.reviewBy` from the page's type (nothing on the
  page reads the record's field); `LaunchRequest` is `{machine?,
  destination?, resume?}` (`api.ts:242`, `:276-286`); the help entries
  `temporary-word` and `review-by` (`terms.ts:74-75`, `:394-401`);
  `launchBody.Word/ReviewBy/Today` and `launchNeedsAuthorization` with
  its refusal (`launch.go:46`, `:97-107`). The walkthrough's launches drop
  `ReviewBy` and its launch hook stamps the signed session's enrollment.
  `useSecret` stays for the sign-in code (`SignInSheet.tsx:58`).
- D6. **Nothing else changes.** The pair on `steward arm`, `seat launch`
  and the goal relay verbs (`goalsync_mutations.go:2700-3415`);
  `human-terminal.json`; the session store; the launch's lock, record,
  reconciliation and discard; presence; the Partner's never-proposed
  fields (`uitools/propose.go:474` names a relay flag).

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
"session":"<opaque>","at":"…"}`; the proof artifact is `artifacts/agents/
authority/proofs/<launch>-<at>.json` under the launching seat's state
root. `POST /api/fleet/launch` takes `{machine, destination}` or `{resume}`.

## 7. Not here, later

An age bound on the record's verdict; revocation from the page; a
migration of temporary-word machines; the kind on the fleet card; a
terminal-run `seat launch --resume` of a browser record enrolls from the
record, which is honest and may one day want a choice; binding the arm
to the launching checkout's `ui serve` ancestry, the step if S72-01's
residual ever hurts; a card-side filter for pre-slice step words
(S72-04: no browser launch writes them now, a pair launch writes them
and should, and no undiscarded record on this host carries them).

Wido, 2026-09-29, on the three questions put to him (no migration of
existing temporary-word machines in step 1; a terminal resume of a
browser record enrolls as `human-session`; D2's caller classification
stays): "Yeah, agreed."

## 8. Verification and box

Go. `steward`: `VerifyIdentity` accepts `human-session` and still
refuses an unknown kind; `ArmSessionWithLineage` in the non-fixture bed
(`rearm_test.go:1049`'s) mints the kind with the session fields, no
word, no `ReviewBy`, the witnessed generation set; the rebuild carry
keeps `Session`; the provenance line. `cmd`: the pair beside
`--launch-record`, a group-readable record, a missing field and another
checkout's destination each refused by name; through the injected
classifier, DELEGATE and STEWARD refused, HUMAN and UNTRUSTED admitted;
once through the REAL classifier, a genuine detached `seat launch` →
`steward arm` chain admitted (S72-01). `seat/launch`: the command list
carries `--launch-record` and no word for a session-enrolled record
(`sequence_test.go:47`) and the pair as today for a pair request; the
pair beside a session-enrolled record refused; a fresh launch with
neither runs plain arm; a legacy record's resume with neither refuses by
name at step 7 and spawns no arm (S72-02; `:241` keeps its shape with
the new remedy). `ui_launch`: **the starter refuses a nil session and a
proof not `SessionValidFor` the state root**, beside the route's
`TestLaunchingRefusesEveryHandButASignedInOne` and
`TestALiveSessionWithNoProofLaunchesNothing` (`launch_test.go:58`, `:171`),
which stay; the enrollment stamped from the proof, not the body; a retry
re-stamps only a record that carries one; the proof artifact exists
before the spawn and an unrecordable proof starts nothing. `httpd`: the
signed-in launch asks for `{Machine, Destination}` alone (`:74`); the
no-authorization and word-in-no-answer tests (`:98`, `:194`) retire.
**One end-to-end test names the ruling:** a record and proof written by
the starter under a signed session, the sequencer's arm command run for
real as `steward arm --launch-record` against a steward bed, the
identity read back: `Enrollment == "human-session"`, `ReviewBy == ""`,
`TemporaryHumanWord == ""`, `Session.Human` the signed handle. Frontend:
`blockedForLaunch` stops at the destination; the review-date blocks
(`launching.test.ts:141`, `:239`) go; the fold line without its clause
(`:230`); no review line on any card and Retry one press
(`LaunchCard.test.tsx:108`); no authorization section. Walkthrough: the
sheet and a folded card at 1280 and 400, light and dark. Build notes: a
commit under `web/_app` rebundles in the same commit (`TestBundleIsCurrent`,
`internal/ui/web/bundle_test.go`); no new package, so no coverage floor.
Rollout proof on this host: launch from the page, read the clone's
`system check` line. Box: Astra's critique (two rounds), one Opus 5.5
lane, one Sol read with one fix round; 120 to 200 job-minutes.

## 9. Self-grade

High on D4 and D5: removals, and one branch kept. High on D3: one kind,
one struct, three readers. Medium on D1 and D2: the verdict crosses a
process boundary as the identity's own verdict already does; the caller
check refuses the actors this repository defends against, and D2 names
what it cannot refuse. Weakest: a browser launch on a server an agent
started is refused at the arm with the classifier's words on the card;
the act layer already refuses such a server. Risks weighed: a pre-slice
record retried from the page (refused at step 7 by name, never the new
flag on an old engine; the one on this host is discarded); two
checkouts' records (each names its destination); the record rewritten by
the verb after the stamp (rewritten whole from the one it loaded, so the
stamp survives).

## Dispositions (Astra round 1, 2026-09-29, under R-121 and R-124)

Read of revision 1 at `7269ad21c`, verbatim in
`g1-s72-astra-critique.md`. Four material findings: two folded, one in
part, one deferred; every cited line re-read at whole-function depth.

| id | finding | fold |
|---|---|---|
| S72-01 | a 0600 record plus an UNTRUSTED caller does not prove a signed-in human: an unrecognised headless process is UNTRUSTED by design (`classify.go:448-476`) and steward recognition is target-repository local (`:513`), so a same-user shell or another checkout's steward could forge the record and mint permanent provenance | in part, D2 and §8: the threat model named with its cite (`identity.go:8`), the residual stated instead of "today's line holds", the real classifier exercised once; no token or handoff mechanism; the ancestry binding in §7 for when it hurts |
| S72-02 | D1 stamped legacy records on retry, so a pre-slice clone's retained HEAD (`sequence.go:265`) and its old engine would be run with a flag it does not know | by removal, D1 and D4: no later stamping; a record without a creation-time enrollment is never armed by the new path, and its resume refuses by name naming Discard launch (`7831ebf30`) and a fresh launch; the only such record on this host is discarded |
| S72-03 | removing `seat launch`'s pair breaks its CLI callers and `--resume` with the pair (`seat_launch.go:45-60`, `sequence.go:670-672`) | D4 and D6: the optional pair and its forwarding branch stay for records without a session enrollment; the pair beside one is refused; the browser's route and body lose it as designed; less removal, no new mechanism |
| S72-04 | old step words "temporary enrollment, review due …" (`sequence.go:683`) still render on a pre-slice card | deferred, non-material, §7: no browser launch writes them now, a pair launch writes them and should, and no undiscarded record on this host carries them |

Astra also verified, and the design leans on, that the route already
requires a live, root-bound session; that D3 names every reader the new
kind needs; and that the detached `ui serve` → `seat launch` chain is
neither a runtime signature nor a steward-family invocation.
