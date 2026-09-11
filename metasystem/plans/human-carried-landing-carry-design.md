# human-carried-landing-carry — design: the carried landing (revision 6)

Goal: plans/goals/human-carried-landing-carry.md (Wido's ruling of
2026-09-04, "All approved": the carry, points 02 and 04 to 09 of
plans/human-carried-landing-design.md revision 2.1, moves to this goal
on a fresh chain with the full file list declared). The audit, point 03,
landed under the sibling goal as ef296c39; point 01 and point 03 stay on
the old page and are cited here, never copied. Every cite below is read
at main 976cb50bb; the revision 6 worktree 316c99b2d carries the same
code (`git diff --stat 976cb50bb 316c99b2d` names this page alone); no
line number is carried over from the old page.

Why now, from plans/goals/the-metasystem-validates-itself-with-itself.md:
on 2026-09-10 every landing on one seat was blocked by the receipt
machinery for a day with no lawful way past it, and on 2026-09-11 this
seat landed 612b1e578 only through a hand-made publication under a
delegated word, because the deep receipt cannot go green on trunk (the
`goal-full-coverage` group's ten-minute timeout) and the wrapper has no
verb for a verified human to carry a landing past a named refusal. The
verb designed here is the lawful form of what was done by hand.

The shape in one paragraph. Urgency changes who carries the rigor, never
the risk. The human speaks one explicit word for one exact workspace
tree, naming the one refusal or the one testing group the landing hits;
the word is a goal-history row under a proof the tree already trusts;
the machine lands that tree with the named gate recorded instead of
enforced, the review deferred into an obligation on the goal, the use
counted, every result printed and stamped. The machine keeps exactly one
gate, identity, and two kinds of check it never carries past: a record
failure (the bytes cannot be bound) and a ledger-meaning check (the goal
is not held, the record is not owned, a ledger path changes outside a
goal verb). Past those it verifies, records, counts and asks; it never
blocks a verified human on its own judgement. One word carries one
defect: a word that names a refusal the landing does not hit, or a
landing that hits two, is a question back to the human, never a landing.

## What changed under the carry since revision 2.1

Facts at HEAD that the old page did not have, each of which changed a
decision below:

- The shared testing contract (1b12f534) is on: `metasystem.conf:7`
  names `testing.json`, and commit.sh runs `test verify` on the proved
  index for every commit, human and agent alike (scripts/agents/
  commit.sh:271-277; the branch is on `testing_contract`, not on
  `agent_commit`). Under the contract the landing evaluator is the live
  binary, `policy_engine=$ms` (commit.sh:278), not a proof-built engine;
  the proof-built engine exists only on the legacy branch (:311-317).
- The workspace receipt (612b1e578): a schema-2 receipt names a whole
  project tree and its workspace projection, the tree minus the ledger
  paths and the append-only registers (internal/landing/registers.go:
  10-27, 35-51), and observe compares receipts to candidates by that
  projection with the selected groups' execution identities
  (internal/landing/receipt.go:498-523, 602-626). A receipt is written
  only for a sufficient result (internal/landing/testing.go:27-29), so
  the case this verb exists for, one group red, has no receipt file.
- `landing observe` under the contract requires a schema-2 receipt for
  every chain (`chain-test-receipt-refused`, internal/landing/observe.go:
  181-190), and the tier-1 direct fix requires one too (tierone.go:83-89).
- The lease branch of commit.sh: the human branch is taken only when
  `lease require-holder` reports no claim epoch (commit.sh:31-41); a
  HUMAN caller gets a nil epoch (internal/lease/verbs.go:372-373), a
  seated checkout always has one, so a carried landing run from the seat
  is an agent commit and meets every agent refusal. The lock behind
  `run-held` is a bounded flock (internal/lease/lock.go:31-53); a nested
  `run-held` waits out the bound and fails with a message that already
  says "pass --lease-held" (:47-49). land.sh today never takes the lock:
  it stages, fetches, rebases and pushes outside it and only commit.sh
  runs held (land.sh:537-646; commit.sh:31-41).
- Enrollment records `os.Getppid()` as the terminal root
  (cmd/metasystem/goalsync_mutations.go:1231; internal/humanauthority/
  authority.go:567-606). A terminal enrolled through a wrapper script
  records the wrapper's pid, and every later `Prove` says
  TERMINAL_NOT_REACHED (authority.go:645-647). The enrollment must be
  typed at the pane.
- The register (internal/refusal/register.go) holds 48 Go rows and 9
  shell rows whose Override is `land.sh --carried`, the 48 marked
  `Pending: "human-carried-landing"`; `chain-full-gate-refused` is inside
  `knownRefusalCode` (internal/landing/promotion.go:121), so the defect
  the old page expected was not needed.
- The channel appends the wanted token to any authenticated reply that
  lacks it (internal/goal/verbs.go:120-124) and the consumer then finds
  it (:86-92). The channel answer row's actor is the literal
  `human:wido` (verbs.go:161; internal/channel/poll.go:322).
- `goal done` refuses only an open review obligation (verbs.go:1128-1132)
  and archives the goal (:1154-1171); `AuthenticatedChannelApproval`'s
  consumer list is `resume` and `set-obligation` (:82), and the history
  grammar admits `approvedRef=` on those two verbs alone
  (internal/goal/file.go:1465-1467).
- Recovery walks the journal only (internal/goal/recover.go:43-72): an
  entry whose owner is dead and whose trailer is absent is completed from
  its stored intent (:101-114, `completeFromIntent` :141-208); a live
  owner's entry is untouched (:115-116). An entry's owner is the process
  that created it (`CreateEntry` uses `SelfOwner`, journal.go:231-256,
  :142-151), and a second `Publish` of an existing non-terminal opid is
  refused, "the recovery rule owns it" (internal/goal/txn.go:522-543).
  `PublishRequest.AfterConfirmed` is the one seam for "an idempotent
  local effect whose omission would make confirmation a lie" and runs on
  the `AlreadyApplied` leg too (txn.go:488-491, 733-743).
- Ledger writes are commits on `main` carrying a `Goal-Transaction:
  <opid>` trailer (`git show 976cb50bb`: "goal claim
  testing-contract-owns-record-paths", trailer
  `Goal-Transaction: FR6HQ65JSKTSW64P25YCCCYW3F-m1e-892cdaec`);
  `TrailerPresent` walks a tip for exactly that trailer (txn.go:379-390).
  `refs/metasystem/goals/accepted` is the validated tip and advances
  only forward and only through the read-side validator
  (`boundedFetchAdvance`, internal/goal/project.go:123-163; `goal fetch`,
  goalsync_verbs.go:395-407); `Project` with fetchFirst false reads it
  offline and banners staleness past thirty minutes (project.go:37,
  51-66, 86-98).
- The ledger root record carries `FormatVersion` and the reader refuses
  any value but `1` as its forward-compatibility rule (internal/goal/
  root.go:17-33, 139-143). A refused tree is a `TreeReadError` for every
  write (verbs.go:235-245) and a refused advance for every fetch
  (project.go:150-152).
- The fixture-mode predicate is one function, `fixtureauth.FixtureModeRoot`
  (`metasystem.runtimes=fake` in the root's conf, internal/fixtureauth/
  fixtureauth.go:286-293), already consulted by supervision, census, the
  mission runner and the steward.
- The testing contract owns `internal/refusal/register.go` alone under
  `gate-plumbing` (testing.json:11) and owns nothing under
  internal/counselor, internal/channel, internal/humanauthority,
  internal/governance, cmd/metasystem/goalsync_mutations.go,
  goalsync_verbs.go or channel_verbs.go (testing.json:5-22); an unowned
  changed path makes the plan deep and replaces the standard set with the
  unknown diagnostic (internal/testpolicy/select.go:121-133, 155-157,
  187-190). A path pattern is an exact path or a `/**` prefix
  (select.go:251-258). Changing testing.json, commit.sh or land.sh is a
  protected policy change (internal/testpolicy/protection.go:37-49).
- The critic subject is an implementer job at four seams:
  scripts/agents/dispatch.sh:1386-1391, internal/dispatch/claim.go:
  773-789, internal/dispatch/review_reference.go:233-247,
  internal/dispatch/finding_register.go:749-753.

Facts read for revision 5, each behind one of its nine changes:

- A ledger transaction captures the canonical branch into a
  per-operation ref with `--refmap=` (txn.go:120-131), so
  `refs/remotes/origin/main` does not move when a goal verb publishes;
  a wrapper that needs origin after its own ledger write fetches again.
  In remote mode the ledger and the code share `refs/heads/main`
  (txn.go:50), so one captured tip is the freshest read of both; in
  single-machine mode the ledger is its own branch,
  `refs/heads/metasystem/goals` (txn.go:32-35).
- `DeliveryJudgment` has five fields (internal/proofrun/test_result.go:
  148-154) and `RecomputeDelivery` (:201-242) clears `Sufficient` on
  any missing group, failing group, uncovered obligation or discrepancy;
  the last two can be non-empty while both group lists are empty
  (:230-238).
- A go group names its tests or says `all` (internal/testpolicy/
  contract.go:342-362); a named test the packages do not define is
  reported `missing` and the group is incomplete (internal/proofrun/
  test_go.go:94-99, 428-439); `targetMs` is the declared cost the result
  records (test_build.go:223; test_result.go:157), not a bound.
- Staging refuses unstaged changes and untracked paths after it stages
  (land.sh:315-333), so at carry-forward the working tree equals the
  index and holds nothing untracked.
- Recovery's two confirm legs run one verb-specific effect,
  `recoverSplitConfirmedEffect`, which returns nil for every verb but
  `split` (recover.go:75-78, 93-96, 416-424).
- `AcceptedRiskDecision` stores `--why` as the accept-risk history
  row's Reason (verbs.go:1070-1071) and the record itself holds only
  Finding, Chain, By and Opid (file.go:148); the replay compares By
  alone (:1061-1068).

## HCL-IDENTITY-02: two proofs, the terminal row's wire, the name

Carried forward from the old page's 02 with the round-2 answer.

The word is the human's under exactly two proofs. The enrolled terminal:
`humanauthority.Prove` (authority.go:611-693) observed in process, valid
only through `Proof.ValidFor(root)` (:143-146); a parsed record has no
authority (:96-98). The authenticated channel word: a threaded reply to a
question this seat asked, bound by `channel poll` as an `answer` row with
AuthorityOutcome AUTHENTICATED_CHANNEL_WORD and the channel proof fields
(verbs.go:146-170; file.go:1436-1461). The temporary recorded word
(`temporaryValidFor`, authority.go:256-267) is not a carry proof: `goal
carry` proves with `Prove` alone through the set-priority pattern
(goalsync_mutations.go:620-622, 677) and never through
`ProveOrTemporaryGoalAuthority` (:302-311); the fixture proof
(`FixtureGoalProof`, :126-139, flag `--fixture-human-authority`,
goalsync_mutations.go:245-247) is admitted for fake-runtime roots so the
beds can speak. This is exactly `goal set-budget`'s gate at HEAD: after
the fleet enrolled its first terminal, the relay is refused there too
(`refuseRelayedAfterFleetEnrollment`, internal/goal/approval.go:409-415).

The terminal row's authority wire (HCL-C-02). Today no history row can
say HUMAN_AUTHORITY_PROVEN: the parser routes every non-channel outcome
through `validateRecordedTemporaryAuthority` (file.go:1437-1441), which
admits only an empty tuple or TEMPORARY_HUMAN_WORD (internal/governance/
types.go:134-158). This design adds a third authority class to the row:

- `governance/types.go` gains `AuthorityOutcomeHumanAuthorityProven =
  "HUMAN_AUTHORITY_PROVEN"` beside the two channel outcomes (:95-97), and
  `internal/goal/obligation.go:66-67` re-exports it as the other two are.
- `HistoryLine` (file.go:293-313) gains `AuthorityGeneration uint64`.
  Parse rule: `authorityOutcome=HUMAN_AUTHORITY_PROVEN` requires
  `authorityGeneration=<non-negative integer>` on the same line, an actor
  beginning `human:`, no channel key, no relay key; any other combination
  is a parse problem. Render rule: `authorityOutcome=HUMAN_AUTHORITY_PROVEN
  authorityGeneration=<n>` in the position the channel outcome takes
  (file.go:1487-1494). The generation is `Proof.TerminalGeneration`
  (authority.go:79, 627-628), which is the enrollment's generation
  (:528-531, never zero for a real proof); the fixture proof carries
  generation zero (:137-138).
- Generation zero in production (HCL-C-02). Every production reader of a
  carry word refuses a row whose generation is zero unless
  `fixtureauth.FixtureModeRoot(root)` is true (fixtureauth.go:291-293):
  the carried classification (05, step 4, code `carry-word-unproven`),
  `goal carrying`'s and `goal carried`'s mutations, `goal done`'s
  open-carry check (06), `landing carry-status`, and the cap and debt
  counts (08). The ledger
  validator is unchanged: a zero-generation row is harmless when no
  reader honors it, and refusing the whole tree for one row would wedge
  every seat's fetch (project.go:150-152).
- What the row proves, said plainly. The row is the ledger's record that
  the enrolled terminal of generation `n` wrote it; the generation
  matches the enrollment record on the seat that wrote it
  (`artifacts/agents/authority/human-terminal.json`, authority.go:508-510).
  The proof file written under the row's opid (`RecordProof`,
  authority.go:703-705; path :732; the command layer's
  `recordGoalApprovalProof`, goalsync_mutations.go:707-733) is post-act
  audit evidence, written after publication, "never a token a later
  process may present as authority" (:700-702). No reader joins the row
  to that file, and this page does not claim one does; revision 3's join
  claim is withdrawn. A missing proof file is found by the audit that
  reads the proofs directory, not by the landing.
- The format fence (HCL-C-27). A row with the new outcome is a parse
  problem for every engine at HEAD (file.go:1437-1441), so the first
  such row would break every seat that has not rebuilt. The fence is the
  ledger's own: the root record's `FormatVersion` (root.go:20, 139-143).
  The new engine reads `1` and `2`; a HUMAN_AUTHORITY_PROVEN row is a
  tree problem under a format-1 root (`ValidateTree`, validate.go:516)
  and lawful under format 2. `goal carry` on a format-1 ledger asks
  (exit 3): "the ledger is at format 1; a carry word needs format 2,
  which every engine older than <the commit that landed this parser>
  refuses to read: rebuild and re-arm every seat (`metasystem up` or
  `steward arm`), then run `goal carry --raise-format`". With
  `--raise-format` the same transaction writes `FormatVersion: 2` on the
  root record (rendered at root.go:311) and the carry row; a later
  `goal carry` on a format-2 ledger needs no flag. What refusing to read
  does to a seat that has not rebuilt: `goal fetch` refuses to advance
  the accepted ref with the problem `FormatVersion "2" is not 1`
  (project.go:150-152, root.go:142); every read keeps the last accepted
  tree and banners staleness after thirty minutes (project.go:37,
  86-98); every write is rejected by name because `loadTree` on the
  canonical tip fails (verbs.go:235-245); the steward keeps ticking on
  the stale accepted tree. The seat moves again when its engine is
  rebuilt from the landing ref (`up` re-arms an engine rebuilt at a
  commit reachable from `metasystem.steward.landing-ref`,
  docs/orchestration.md's supervision section). The other option, an
  arm-before-write rule, was not chosen because a seat's engine commit is
  a local enrollment record (`InstallIdentity.EngineBuild`,
  `LandedCommit`, internal/steward/identity.go:73-79) with no fleet-
  visible copy; `goal carry` on one seat cannot read another seat's.
- The name source. The name is `--by`, typed by the human at the proven
  pane, and the row's actor is `human:<by>` through `historyActor`
  (verbs.go:182-187), exactly as `goal set-budget --by` today
  (goalsync_mutations.go:965, 984; approval.go:417-426 writes
  `by=human:<name>`). The design invents no name registry; what binds the
  name to a person is that only the enrolled terminal can write the row.
  The channel row's name stays the channel's own fact (`human:wido`,
  verbs.go:161); this design does not change it and says so in the
  record of gaps.

Fixtures of 02 (every fixture on this page is a future test, none exists
today; each line is the bed and the oracle):

- HCL-02-TERMINAL-PROOF, internal/goal and cmd/metasystem: `goal carry`
  under `FixtureGoalProof` writes one `carry` row with
  `authorityOutcome=HUMAN_AUTHORITY_PROVEN`; without a proof the command
  exits 1 and the goal file is byte-identical.
- HCL-02-CHANNEL-PROOF, internal/channel and internal/goal: a `channel
  poll` bound reply to a `--kind carry` question writes an `answer` row
  with AUTHENTICATED_CHANNEL_WORD and the four channel keys; `landing
  observe --carried <its opid>` reaches step 5.
- HCL-02-TEMPORARY-REFUSED, cmd/metasystem: `goal carry` with
  `--temporary-human-word` and `--review-by` exits 2 with the words
  "carry takes no relayed word"; no row.
- HCL-02-AGENT-CANNOT, cmd/metasystem: `goal carry` from a caller the
  lease classifies MAIN, without a proof, exits 1; no row.
- HCL-02-TERMINAL-ROW-BYTES, internal/goal: `RenderHistoryLine` of the
  fixture-proof word is exactly
  `- <at> <opid> carry actor=human:Wido targets=<goal> authorityOutcome=HUMAN_AUTHORITY_PROVEN authorityGeneration=0 reason=carry workspace=<sha40> goal=<goal> past=<name> expires=<RFC3339> why=<quoted>`
  and parses back to the same struct; the outcome without a generation,
  or with a machine actor, is a parse problem naming the field.
- HCL-02-GENERATION-ZERO-REFUSED-IN-PRODUCTION, internal/landing: a
  ledger with a generation-0 word under a root whose conf says
  `metasystem.runtimes=claude` classifies `carry-word-unproven`; the same
  ledger under `metasystem.runtimes=fake` reaches step 5.
- HCL-27-OLD-READER-REFUSES-FORMAT-TWO, internal/goal: the known-format
  set is a package variable (`knownLedgerFormats`, the seam
  `fetchForProjection` at project.go:45 already is); with it narrowed to
  `{"1"}`, `boundedFetchAdvance` against a remote at format 2 returns an
  error containing `FormatVersion "2" is not 1` and the accepted ref is
  unchanged; `Project(e, false)` still returns the old accepted tree; a
  `Publish` of a claim is rejected with a `TreeReadError`.
- HCL-27-FORMAT-RAISE-ASKS-THEN-WRITES, goal-cli scenario `carry-word`:
  `goal carry` on the fixture ledger (migrated at format 1, migrate.go:
  447) exits 3 with the raise-format line and writes nothing; with
  `--raise-format` the root reads `FormatVersion: 2` and the row is
  present in one commit; a following `goal claim` succeeds.

## HCL-WORD-04: one word, one workspace tree, in the ledger

The word names a workspace tree, not a whole-project tree. The whole tree
changes on every peer ledger write (plans/goals moves on every `goal
claim`), which is the disease 612b1e578 cured for receipts; the word
follows the receipt: `workspace=<sha40>` is
`landing.ProjectWorkspaceTree(root, <whole tree>)` (registers.go:35-51).
A ledger-only move on origin leaves the word valid; a code move does not.

The strict token is `carry workspace=<sha40> goal=<id> past=<name>`, four
fields, contiguous, exactly once in the row's reason
(`containsContiguousFields`, verbs.go:97-109). `<name>` is one of:

- a refusal code from `refusal.Rows` whose Override begins
  `land.sh --carried` after the flip of the register below (the carryable
  set; identity rows and ledger-meaning rows are outside it by
  construction), or
- `group:<id>` where `<id>` is a group of the testing contract
  (`testing.json`, for example `group:goal-full-coverage`).

The word's seat (HCL-C-29). The seat is recorded in the row as the
machine segment of its opid: `goal carry` mints the opid from the
seat's machine and lineage (`syncReq`, goalsync_mutations.go:34-36;
`r.opid()`), and `channel poll` mints the answer row's opid from the
polling machine (`goal.Opid(ulid, c.Machine, c.Lineage)`, poll.go:
227-231). `validOpidShape` splits the machine from the right (file.go:
1513-1532), so every reader derives the seat without a second field and
HCL-02-TERMINAL-ROW-BYTES is unchanged. The landing's actor machine
(commit.sh:452-455, `--actor <machine>+<lineage>`) must equal the word's
seat; a word polled or typed on another seat is `carry-seat-mismatch`
(05, step 4a), whose ask prints the transfer line below. A human
transfer is `goal carry --supersede <opid>` on the new seat: the new
word is that seat's, and the old word is consumed in the same
transaction.

Two ways to write the word:

- Terminal: `goal carry --root . --id <goal> --by <name> --tree <sha40>
  --past <name> --why "<text>" [--expires 2h] [--supersede <opid>]
  [--raise-format]`. `--tree` is the whole-project tree (`git write-tree`
  at the toplevel, what land.sh's `staged_project_tree` prints, land.sh:
  361-363); the command layer requires a full 40-hex id that `git
  cat-file -t` calls a tree, projects it to the workspace tree, and
  writes the row with the projection; a prefix or an unknown id is shape
  (c) with the ask for the full id. Expiry defaults to two hours after
  the row's At, ceiling four hours; a longer request is asked back. The
  verb prints the row's opid and the 08 word lines.
- Channel: the seat runs `channel ask --root . --id <goal> --kind carry
  --wants "carry workspace=<sha40> goal=<id> past=<name>"` with these
  facts, in order: the workspace tree, the refusal or group a dry
  `landing observe` says the landing hits, the goal's tier and four risk
  answers or `risk: unanswered`, the diff stat, the seat's open carries
  and the fleet's unpaid carry debt, and the expiry (two hours after the
  reply). `channel poll` binds the reply as the `answer` row. The answer
  row is the word; its opid is the carry's opid; it expires two hours
  after its At. The reply's own text must hold the token (HCL-C-12,
  below). A channel word cannot supersede; the transfer form is the
  terminal's.

`--supersede <opid>` (HCL-C-30). The preconditions are checked inside
the mutation callback, so they are rechecked on every rebuild of the
ledger transaction (`PublishRequest.Mutate` is "called again on every
rebuild", txn.go:473-478): the target is a `carry` row or a carry
`answer` row (a row that passes steps 3 and 4 of 05's classification
save the seat rule) on any live goal of the tree; its seat equals the
new word's machine (`r.Actor.Machine`), or the caller passes
`--transfer` and is the human (the seat rule is the one precondition a
human may waive, since the transfer is the point); it is unexpired at
`r.Now`; it is not in flight (no open `carrying` row for it, 08; the
rejection is `in flight on <seat> since <at>: goal carrying --abandon
<row opid> on that seat, or wait for <expires>`); and it is unconsumed
by the COMPLETE predicate of 06, rechecked on the tip the callback is
given: no `carried` row with ApprovedRef equal to it on any live or done
goal (any outcome), no `carry` row whose reason ends `supersedes=
<target>`, and no commit whose message holds `Carry: <target>` in the
target's anchored range `A..<code tip>`. The code tip the callback
scans: in remote mode the captured `tip` itself, since the ledger and
the code share `refs/heads/main` (txn.go:50) and the capture
(`CaptureTip`, txn.go:103-131) is the freshest read of both, fresher
than the tracking ref the checkout holds; in single-machine mode
`refs/heads/main` as the checkout has it, since the ledger is its own
branch there (txn.go:35). The rejection names the tip it scanned. A
trailer without a row is the push-before-row interval of 06, and the
rejection is `consumed on origin by <sha> at <tip>; the record is
incomplete: rerun land.sh --carried <target>`; nothing is written, and
the row that completes the record is the landing's, never supersede's.
The transaction writes two rows: the new `carry` row with
`supersedes=<opid>` after the token in its reason, and on the target's
goal a `carried` row `approvedRef=<target> reason=superseded by=<new
opid>` (08). When the target changed before publication (a rebuild finds
it consumed, expired, or absent), the callback returns an error naming
the target's current state (`consumed by <opid>`, `expired at <stamp>`,
`missing`), which `terminalFromMutate` records as a rejection
(txn.go:756-762); nothing is written, and the ask repeats the state.

The word does not claim: the ordinary claim law stands; a goal the
landing machine does not hold is shape (c) at the landing, printing the
`goal steal` line (`Steal`, verbs.go:1691). The word does not need the
goal to be held at the time of the word.

The cap and the debt are asked at the word (terminal) and again at the
landing (both kinds); they are point 08's two additions and are specified
there.

Fixtures of 04:

- HCL-04-ROW-FORM, internal/goal: `Carry` writes the row of
  HCL-02-TERMINAL-ROW-BYTES with `expires=` two hours after At by default.
- HCL-04-CHANNEL-ROW, internal/channel: the bound answer row's reason is
  the human's text verbatim and its opid's machine segment is the polling
  machine.
- HCL-04-FULL-ID-ONLY, cmd/metasystem: `--tree` of 39 hex digits, or of a
  blob id, exits 3 with "the full 40-digit tree id"; no row.
- HCL-04-EXPIRY-ASKS, cmd/metasystem: `--expires 5h` exits 3 naming the
  four-hour ceiling; `--expires 4h` writes `expires=` At plus four hours.
- HCL-04-NOT-HELD-ASKS, land leg `carried-asks`: a valid word on a goal
  this machine does not hold exits 3 with the `goal steal` line; no
  commit.
- HCL-04-WORKSPACE-NOT-WHOLE, cmd/metasystem: `--tree <whole>` stores
  `workspace=` equal to `landing workspace --tree <whole>` and not equal
  to `<whole>`.
- HCL-04-LEDGER-MOVE-KEEPS-WORD, land leg `carried-forward`: a peer
  `goal claim` published between the word and the landing changes
  nothing: one commit lands on origin under the same opid.
- HCL-04-SECOND-WORD-LANDS, land leg `carried-forward`: origin moves code
  after the first word; the first attempt exits 3 printing the new
  workspace tree and the `goal carry` line and leaves the index staged;
  a second `goal carry --supersede <first>` and a second `land.sh
  --carried <second>` land one commit; the first opid has a `carried`
  row with reason `superseded by=<second>`.
- HCL-30-SUPERSEDE-PRECONDITIONS, internal/goal: `--supersede` of an
  expired word, of a consumed word, of another seat's word without
  `--transfer`, and of an opid that is no carry row each return the
  named rejection; the goal files are unchanged.
- HCL-30-SUPERSEDE-TARGET-MOVED, internal/goal: through `BeforePush`
  (txn.go:484-487) a competitor consumes the target between capture and
  push; the rebuild rejects with `consumed by <competitor opid>`; no new
  row on either goal.
- HCL-30-SUPERSEDE-PUSH-BEFORE-ROW, internal/goal: the target's carried
  commit is on the fixture origin with `Carry: <target>` and no
  `carried` row exists; `--supersede <target>` is rejected with
  `consumed on origin by <sha>` naming the tip it scanned; no row on
  either goal; the `origin:` branch of 06 then completes the record.
- HCL-30-SUPERSEDE-SCANS-CAPTURED-TIP, internal/goal: through
  `BeforePush` the competitor pushes the target's carried commit (the
  trailer, no row) between capture and push; the rebuild rejects
  `consumed on origin`; no row.
- HCL-30-SUPERSEDE-IN-FLIGHT, internal/goal: an open `carrying` row for
  the target rejects with `in flight on <seat>`; after `goal carrying
  --abandon <row>` the same command writes both rows.
- HCL-29-SUPERSEDE-TRANSFERS-SEAT, goal-cli scenario `carry-word`: a
  word minted with machine `seat-a` is superseded on `seat-b` with
  `--transfer`; the new row's opid carries `seat-b`; `landing observe
  --carried <new> --actor seat-b+<lineage>` passes step 4a and
  `--actor seat-a+<lineage>` is `carry-seat-mismatch`.

## HCL-LANDING-05: `land.sh --carried`, one refusal, everything else advisory

`land.sh -m <msg> --goal <id> --carried <opid> [--chain <root> |
--direct-fix <class> ...] [--test-receipt <path>] [--staged-only |
<pathspec>...] [--skip-transport]`. `--carried` composes with every
ordinary declaration: the ordinary classification still runs and the
word names the one refusal it hits. Without a chain or direct fix the
ordinary classification is `missing-declaration` (observe.go:110-112),
which is the refusal a chainless tree names. The usage line (land.sh:9)
names `--carried <opid>` beside `--goal`.

### One lease epoch for the whole sequence (HCL-C-04)

land.sh in carried mode re-executes itself under the lease exactly the
way commit.sh does today (commit.sh:31-41): before its first step
(`run_required_step "verify checks"`, land.sh:537) it runs `lease
require-holder --caller-pid $$`; with a claim epoch it execs `lease
run-held --expected-epoch <epoch> -- "$0" __lease-held <epoch> "$@"`,
without one `-- "$0" __lease-held human "$@"`. Inside, the whole carried
sequence runs under that one lock and epoch: fetch, `goal fetch`,
carry-forward, stage, commit, push, `goal carried`, transport. commit.sh
is invoked as `commit.sh __lease-held <epoch> ...` so it skips its own
re-exec (commit.sh:31); a nested `run-held` would wait out the lock
bound and fail (lock.go:39-49). `RunHeld` runs a HUMAN caller ungated
(verbs.go:470-472), so a human at a pane on an unseated checkout runs
the same sequence without the lock, as every human commit does today;
on a seated checkout the caller is the MAIN holder and the lock is held
throughout. A lease mutation from another process during the sequence
(`lease renew`, verbs.go:428-460, takes the same lock at :430) fails
with "checkout lease lock is busy" (lock.go:47-49).

### Order in carried mode, before any commit

The carried landing lands `main`: `verify_checks` (land.sh:293-313)
refuses any other branch in carried mode with the ask (`carry asks: the
carried landing lands main; you are on <branch>`).

1. `fetch_origin` (land.sh:432-434), then one fresh ledger tip
   (HCL-C-28): `goal fetch --root .` (goalsync_verbs.go:395-416, the verb
   that advances `refs/metasystem/goals/accepted` through
   `boundedFetchAdvance`, project.go:123-163). A failed fetch or a
   refused advance is an ask (exit 3) printing the verb's error: the
   landing does not read a ledger it could not validate. Call the
   accepted ref's value after this call `L0`. The tip every later
   engine call takes as `--ledger-tip` is `L`, set at step 5; the
   classification refuses (`carry-ledger-moved`, an ask to rerun) when
   the accepted ref is no longer `L` when it reads, so the word, the cap
   and the debt are read from one tree.
2. `landing carry-status --root . --carried <opid> --goal <id>
   --ledger-tip L0` (06) and the branch it selects.
3. Carry-forward when HEAD is not `refs/remotes/origin/main` and the
   index holds the candidate (fresh word). The working tree equals the
   index and holds nothing untracked, since staging refused otherwise
   (land.sh:315-333). `base=$(git rev-parse HEAD)`; `wip=$(git
   commit-tree "$(git write-tree)" -p HEAD -m "carried wip")`; `git
   update-ref HEAD "$wip"` (plumbing; no hook runs); `git rebase
   refs/remotes/origin/main`; `git reset --soft refs/remotes/origin/
   main`. The index now holds the candidate on the new base and the
   working tree equals the index. A rebase conflict leaves one posture
   (HCL-C-32): `git rebase --abort` (HEAD is the wip again, the index
   and the working tree its tree), then `git reset --soft "$base"`. The
   posture: HEAD is the wip commit's parent, the base the human staged
   on; the index holds the candidate exactly as it was staged; the
   working tree equals the index; nothing is untracked; no
   `rebase-merge` or `rebase-apply` directory exists; the wip commit is
   unreferenced. It is the state before this step began, so a rerun
   starts the same way. Then the ask (exit 3): "rebase conflict on
   <paths>; resolve by hand against origin/main, stage, rerun".
4. Compare: the projected workspace tree of the index equals the word's
   `workspace=`, continue; different (origin moved code), print the new
   workspace tree and the `goal carry --supersede <opid>` line (terminal)
   or post the `--kind carry` question for it (channel), leave the index
   staged on origin/main, exit 3. The second word lands through the same
   path with HEAD equal to origin/main and the set already staged: the
   commit is never amended, it is rebuilt on the new base under the new
   word.
5. The reservation (HCL-C-33): `goal carrying --root . --id <goal> --ref
   <opid> --tree <staged project tree> --by human:<name>` publishes the
   `carrying` row of 08 as one goal transaction, so every seat that
   fetches sees this carry in flight before any code is pushed. Its
   Mutate rechecks, on the tip it builds on, steps 3 to 10 of the
   classification below as record reads (the row exists and is proven,
   the seat, the token against the projection of `--tree`, carryable,
   unexpired, unconsumed, the debt, the cap) and refuses by the same
   code, which the wrapper prints before exit 3. When an open
   `carrying` row for this word already exists on this seat (a rerun),
   the verb runs no transaction and prints that row. The verb prints
   `carrying=<row opid> ledger=<tip>`, the tip being the accepted ref
   after confirmation (txn.go:690-701): that is `L`. The row is a
   ledger commit on `main`, so origin moved, and the per-operation
   capture does not update the tracking ref (txn.go:120-131), so:
6. `fetch_origin` again, then step 3 again onto the new origin/main. A
   ledger-only move rebases the candidate without conflict, since a
   candidate that touches a ledger path is `ledger-path-not-goal-verb`,
   never carried; a peer's code move between steps 1 and 6 is possible
   and is found by step 4 again. On either ask here (a conflict, a
   different workspace tree) the wrapper first releases its own
   reservation, `goal carrying --abandon <row opid>` (08), so nothing
   stays reserved for a landing that is not happening, then asks as
   above with the same posture.

### The carried classification

`landing observe --root . --tree <landing tree> --project-tree <sha40>
--carried <opid> --goal <id> --actor <machine+lineage> --ledger-tip L
--judge live|base [--live-failure <code>|exit=<n>] [ordinary flags]` is a
new classification, bar `d` (`BarCarried = "d"` beside `a`, `b`, `c`,
observe.go:28-32), in a new file internal/landing/carried.go;
`ObserveParams` (observe.go:43-55) gains `Carried`, `ProjectTree`,
`LedgerTip`, `Judge`, `LiveFailure`. Its checks, in order, each a record
read and nothing else; every failure is `refuse(code)` (mode refuse
regardless of the promotion record, observe.go:1271-1275) with the ask
text in `Observation.Refusal`:

1. The ledger tree is `goal.Project(e, false, now)` (project.go:51-66)
   and its Tip equals `--ledger-tip`; else `carry-ledger-moved`. Every
   later step reads this one tree. The goal is live in it; else
   `carry-goal-not-live`.
2. The goal is held by the actor: `heldGoalError` at the base tree
   (observe.go:776-779). `goal-item-not-held` stands as today; the
   refusal text carries the `goal steal` line. Never carried.
3. The row exists on the goal: Opid equals `--carried`, and either Verb
   `carry` with AuthorityOutcome HUMAN_AUTHORITY_PROVEN or Verb `answer`
   with AuthorityOutcome AUTHENTICATED_CHANNEL_WORD; else
   `carry-word-missing` (the ask says to fetch, since the ledger row may
   be newer than the checkout).
4. The row is proven: a terminal row's generation is non-zero, or the
   root is `FixtureModeRoot`; else `carry-word-unproven`.
   4a. The seat: the machine segment of the row's opid equals the
   actor's machine; else `carry-seat-mismatch`, printing the transfer
   line of 04.
5. The token: the reason holds `carry workspace=<W> goal=<id> past=<name>`
   exactly once, where `W` equals `ProjectWorkspaceTree(root,
   --project-tree)`; a different workspace is `carry-tree-mismatch` and
   the ask names both ids and the `goal carry` line for the candidate.
6. `<name>` is carryable (the set above); else `carry-not-carryable`,
   which for an identity code or a ledger-meaning code says so in words.
7. Unexpired: `expires=` on a terminal row, At plus two hours on a channel
   row; else `carry-word-expired` with the fresh-word line.
8. Unconsumed: no history row on any live or done goal of the tree with
   Verb `carried` and ApprovedRef equal to the opid (any outcome; a
   `superseded` row prints the superseding opid), and no commit in the
   anchored range of 06 whose message holds the trailer `Carry: <opid>`;
   else `carry-word-consumed` with where it landed. The wrapper
   distinguishes "consumed on origin, record incomplete" from "consumed
   in the ledger" through `landing carry-status` (06).
9. The debt (08): none of its three forms exists. An open obligation
   with Chain `human-carried` on any live goal of the tree whose
   Artifact commit is an ancestor of HEAD; an open `carrying` row (08)
   on any live goal of the tree whose ApprovedRef is not `--carried`
   (another carry in flight; this landing's own reservation is not
   debt); a landed-but-unrecorded carry (HCL-C-33), that is a `Carry:
   <w>` trailer in the anchored range of 06 for any proven carry word
   `w` (a `carry` row or a carry `answer` row, steps 3 and 4) on any
   live or done goal of the tree (`TreeGoals.Live` and `.Done`,
   validate.go:27-32) that has no `carried` row with ApprovedRef `w`
   (any outcome) on any live or done goal. That arm reads two facts,
   the trailer and the missing row, and no state of `w`: not whether
   `w` is expired, not whether its reservation is open, expired or
   abandoned, not the cap's open-word predicate of 08 (a superseded
   word has its `superseded` row and so is not row-less). The scan is
   the scan of 06, run once per row-less word. Else `carry-debt-unpaid`,
   naming the obligation, or the row and its seat, or the commit and
   its word with `land.sh --carried <w>` as the closer.
10. The cap (08): fewer than `metasystem.budget.carry-open-max` OTHER
    open words for this seat in the tree; else `carry-cap-reached`.
11. The base judge's blindness (HCL-C-03), when `--judge base`: the
    candidate changes nothing under an owner of the base engine's
    policy or wire formats. The check is `git diff-tree -r --name-only
    <HEAD^{tree}> <--project-tree> --` (every added, changed, deleted or
    renamed path) filtered, relative to the metasystem prefix, to the
    owners: every path under `internal/landing/`, `internal/goal/`,
    `internal/proofrun/`, `internal/testpolicy/`,
    `internal/behaviorsurface/`, `internal/config/`,
    `internal/refusal/`, `internal/governance/`,
    `internal/humanauthority/` and `internal/fixtureauth/` (the
    classification and the receipt, the ledger grammar, the testing
    result, the selection, the behaviour policy, the budget law, the
    register, and the three owners of the authority wire the ledger
    grammar reads (HCL-C-03): `internal/governance/` owns the outcome
    constants and the recorded-authority validation the parser calls
    (types.go:91-101, 108-115, 134-158; imported at file.go:19, called
    at :1436-1441) and is where 02 puts the proven outcome;
    `internal/humanauthority/` owns the proof whose
    `TerminalGeneration` (authority.go:79) is the value the row records
    as `authorityGeneration=` (02) and the outcome constant the goal
    package compares (`OutcomeVerifiedChannel`, authority.go:38;
    approval.go:387, 398; norm.go:114); `internal/fixtureauth/` owns
    `FixtureModeRoot` (fixtureauth.go:291), the one predicate under
    which step 4 admits a generation-zero row. Those three are every
    compiled package the goal package imports for an authority fact
    (`go list -f '{{.Imports}}' ./internal/goal` at HEAD names
    governance and humanauthority; humanauthority imports fixtureauth),
    so a candidate that changes what a row's authority means is fenced
    whichever of them it edits. A new file under any owner counts,
    since a new file is how carried.go itself arrives), the two root
    policy files `metasystem.conf` and `testing.json`, and the three
    policy inputs the observer reads, `scripts/agents/
    landing-classes.json`, `scripts/agents/path-classes.txt` and
    `scripts/agents/landing-promotion.json` (observe.go:923, 1291;
    promotion.go:13). A non-empty result is `carry-base-judge-blind`:
    "the base judge cannot judge a candidate it cannot read: <paths>;
    rebuild the live engine (`steward arm` at a good commit) and retry".
    Said plainly: the base judge runs the base's policy, so a candidate
    that changes the policy is judged by nobody, and the machine says so
    instead of pretending. The fence is by directory, not by file, so an
    owner the exact list of revision 4 missed (receipt.go, which owns
    `TestReceipt` at receipt.go:33-51) and every owner this chain adds
    are fenced the day they land.
12. The ordinary classification O: `observe(params)` with `Carried`
    cleared and the ordinary flags as given (observe.go:96-134), promotion
    applied (promotion.go:25-44).
13. The testing verdict: `VerifyTesting` is set (landing_verbs.go:
    47-52; the carried wrapper always sets it) and R is its result, a
    `TestResult` whose `Delivery` is the machine's own judgment
    (internal/proofrun/test_result.go:148-154): `Sufficient`,
    `MissingGroups`, `FailingGroups`, `UncoveredObligations`,
    `Discrepancies`, recomputed from the group evidence (:201-242).
    Call M the union of the two group lists, U the uncovered
    obligations, D the discrepancies. A verify error means there is no
    R: the battery is `unverified`, and no word carries it (below).
14. The match (HCL-C-21): one word carries one defect, and a testing
    defect is one only when it is the whole insufficiency.
    - `<name>` is a code: the landing hits it when O.Verdict is
      `would-refuse`, O.Code equals `<name>`, AND R exists with
      `R.Delivery.Sufficient` true (no missing group, no failing group,
      no uncovered obligation, no discrepancy). When `<name>` is
      `evaluator-unavailable`: `--judge base` with a `--live-failure`, O
      passes, and R is sufficient.
    - `<name>` is `group:G`: the landing hits it when R exists, M equals
      `{G}`, U is empty, D is empty, AND (O.Verdict is `pass` OR O.Code
      is one of `chain-test-receipt-refused`, `chain-full-gate-refused`,
      `tier1-receipt-refused`, `tier1-full-gate-refused`, the refusals
      that are the same red group seen through the receipt). An
      uncovered obligation or a discrepancy beside G is a second defect
      with no name of its own, so it asks.
    - No R (a verify error): `carry-battery-unverified`, "test verify
      failed: <error>; no word carries an unverified battery; repair the
      testing tool or its evidence and rerun". The machine cannot read
      its own testing result, which is a record failure in everything
      but its exit code.
    - O passes and R is sufficient (HCL-C-22): the carry is unneeded. The
      landing does not consume the word and does not land:
      `carry-unneeded`, "the refusal you named did not occur; land
      without --carried, or name what you see". The word stays open; it
      expires or is superseded.
    - Anything else is `carry-refusal-mismatch`, and the ask names
      O.Code, M, U, D and the word's `<name>`.
15. The result: `Observation{Mode: observe, Bar: "d", Verdict: pass,
    Code: "human-carried", Provenance: "carried opid=<opid> past=<name>
    seat=<machine> ledger=<L> judge=<live|base[ live-failure=<x>]> " +
    O.Provenance, VerdictTrailer: "pass bar=d carried=<name> base=" +
    O.VerdictTrailer, Detail: <battery summary>}`.

The fifteen ask codes (`carry-ledger-moved`, `carry-goal-not-live`,
`carry-word-missing`, `carry-word-unproven`, `carry-seat-mismatch`,
`carry-tree-mismatch`, `carry-not-carryable`, `carry-word-expired`,
`carry-word-consumed`, `carry-debt-unpaid`, `carry-cap-reached`,
`carry-base-judge-blind`, `carry-battery-unverified`, `carry-unneeded`,
`carry-refusal-mismatch`) are
Question rows in the register with no Override and are not in
`knownRefusalCode`, as the tier-1 codes are not; commit.sh prints
`Observation.Refusal` and exits 3 on every one of them, for human and
agent branch alike. Exit 3 is the wrapper's shape (c) code, distinct
from 1 (failure) and 2 (usage).

### The evaluator (HCL-C-03)

commit.sh in carried mode resolves the judge in two steps and stamps the
choice:

1. The live engine, `$ms` (commit.sh:5, 278). commit.sh runs `landing
   observe --carried ... --judge live`. If it returns a complete decision
   (the four fields the wrapper already requires, :483-484), judge is
   `live`.
2. Otherwise the base-tree engine: `git -C "$toplevel" worktree add
   --detach <scratch> HEAD`, then `go build -o <judge> ./cmd/metasystem`
   inside `<scratch>/<prefix>`, the worktree removed after the build.
   HEAD at this point is origin/main (carry-forward has run), so the
   base is the enrolled trunk and never the candidate. commit.sh then
   runs `"$judge" landing observe --carried ... --judge base
   --live-failure <code|exit=<n>>`, where the value is the live
   engine's `code` when it returned any JSON and `exit=<n>` when it did
   not. The live-failure state is therefore an input the observer
   records in Provenance and the trailer, not a guess the observer
   makes: observe.go has no such parameter today (:43-55), and the
   wrapper is the only process that saw the failure. The fallback's
   output is parsed by the judge itself: every `json get` in the carried
   branch runs as `"$judge" json get` (the `json` family is in every
   engine build, cmd/metasystem/main.go:429-436), never as `$ms json
   get` as the ordinary branch does (commit.sh:478-482), and the shape
   it reads is `Observation` (observe.go:59-72: `mode`, `bar`, `verdict`,
   `code`, `provenance`, `verdictTrailer`, `refusal`, `detail`), fixed
   for both judges.

If neither judge decides, the machine cannot read the word and says so:
shape (c), exit 3, naming the build failure and `steward arm` at a good
commit. `evaluator-unavailable` is therefore carryable only in the sense
of step 14: the live engine failed, the base judge took over and passed
the tree, R is sufficient, and step 11 found nothing the base cannot
read. No engine-less path reads the ledger by hand.

Every judgement refusal is advisory in carried mode when the word names
it: the carryable rows of the register, and the testing groups. Every
record failure stays a stop in every mode, each with its site: the index
cannot be proved as a tree (commit.sh:262-265); the index or a gate input
moved while the proof ran (:435-445); projected working-tree bytes the
commit would not record (:345-353); a staged gitlink (:356-370); a
critical symlink (:376-392); an assume-unchanged or skip-worktree entry
(:397-411); no machine nickname (:452-454); the landed tree or the
trailer set differs from what was proved (:573-592); an unreadable
message (land.sh:191-194); an empty, unstaged or untracked staging set
(land.sh:320-333); HEAD not on a branch (:294-297); a bare rulings id
(:241-291); the brain fence (:117-127, BRAIN_REFUSED's own override is
`brain withdraw`). `test verify` itself is not a stop in carried mode:
its result is R of step 13, printed and stamped, and the word decides;
a verify error is the one ask of step 14 that no word answers.

### Trailers

The wrapper stamps, in the colon form it owns (commit.sh:566-571):
`Carried-By: human:<name>` (the row's actor), `Carry: <opid>`,
`Carried-Tree: workspace=<sha40> project=<sha40>`, `Carried-Past:
<name>`, `Carried-Battery: green | red missing=<csv> failing=<csv>`
(the `unverified` value of revision 4 is withdrawn: step 14 lands no
unverified battery, so no commit can carry it), `Carried-Judge: live
sha256=<digest> | base tree=<sha40> sha256=<digest>
live-failure=<code|exit=n>`, `Carried-Ledger: <L>`,
beside the ordinary `Machine`, `Landing-Provenance` (the carried
provenance of step 15), `Landing-Provenance-Verdict` and `Goal-Item`.
The message scan that refuses a typed `Goal-Item` (:159-235) refuses a
typed line with any of the seven carried keys, and the postcondition
(:573-592) requires exactly one of each after the commit in carried mode
and none of them otherwise, and that `Carried-Judge` equals the judge
and live-failure value the wrapper passed. A commit without a carry row
cannot carry them.

### The push, and recovery from a local carried commit (HCL-C-19)

The push is a single attempt (`push_origin`, land.sh:440-442). A
moving-origin rejection (`push_was_moving_origin_rejection`, :444-446) is
an ask (exit 3): "origin moved during the push; rerun `land.sh --carried
<opid>`". There is no retry loop in carried mode (the ordinary loop is
:622-641). The rerun takes the `local:<sha>` branch of 06:

1. `fetch_origin`, `goal fetch`, `landing carry-status`.
2. If origin moved: `git rebase refs/remotes/origin/main`. The carried
   commit is rebased, its message and trailers intact; it is never reset
   away and never restamped. A conflict: `git rebase --abort`; HEAD is
   `C` again and the index and the working tree are `C`'s tree, the one
   posture of this path (HCL-C-32), then the ask.
2a. The reservation must be open before the push (HCL-C-33). Step 1's
   `carry-status` reported it: `open`, keep it; `abandoned`, reserve
   again (05 step 5's transaction, whose Mutate rechecks the debt);
   `expired`, the word is expired too (one expiry, 08), so this is the
   fresh-word ask with the local commit kept.
3. Re-run the postconditions on the rebased commit: its whole-project
   tree's workspace projection equals the word's `workspace=` (else exit
   3 printing the new tree and the `goal carry --supersede` line: origin
   moved code, and a new word is needed; the local commit stays for the
   human to rebuild from); the message holds exactly one of each carried
   trailer and `Carry:` equals the opid (else a record failure, exit 1,
   the commit is not pushed); the ordinary `Goal-Item` rule of
   commit.sh:575-579 on the rebased message. The stamped battery stays
   true by construction: receipts and verify results bind the workspace
   projection (receipt.go:498-523), which the check just proved equal.
4. The intent entry for the rebased commit (06 step 5's `--commit`
   form, which closes the stale entry itself), then the push, single
   attempt; continue at 06 step 6.

Fixtures of 05:

- HCL-05-LANDS-WITHOUT-CHAIN, land leg `carried-fresh`: no `--chain`, a
  word naming `missing-declaration`, a green battery; one commit on
  origin whose message holds the seven carried trailers with
  `Carried-Past: missing-declaration` and `Carried-Battery: green`.
- HCL-05-FOREIGN-TREE-ASKS, land leg `carried-asks`: the staged set's
  workspace tree differs from the word's; exit 3, the output holds both
  ids, `git rev-parse HEAD` is unchanged.
- HCL-05-RED-BATTERY-LANDS, land leg `carried-fresh`: a word naming
  `group:<g>` where `<g>` is the leg's one failing group; the commit
  lands with `Carried-Battery: red failing=<g>`.
- HCL-05-TRAILER-OWNED, land leg `carried-fresh`: a message file holding
  a typed `Carry:` line exits 1 before any commit.
- HCL-05-REBASE-ASKS, land leg `carried-forward`: origin changes the same
  file the candidate changes; exit 3 with "resolve by hand"; the one
  posture of 05 step 3 holds: `git rev-parse HEAD` equals the HEAD the
  bed recorded before the run (the wip commit's parent), `git
  write-tree` equals the staged tree recorded before the run, `git diff
  --quiet` succeeds (the working tree equals the index), `git ls-files
  --others --exclude-standard` prints nothing, neither
  `.git/rebase-merge` nor `.git/rebase-apply` exists, and the goal holds
  no `carrying` row (the conflict is before the reservation).
- HCL-32-RECOVERY-CONFLICT-KEEPS-COMMIT, land leg `carried-crash`: the
  `local:<sha>` rerun with origin changing the same file; exit 3; `git
  rev-parse HEAD` is `<sha>`, `git status --porcelain` prints nothing,
  no rebase directory exists; the reservation stays open.
- HCL-32-LEDGER-MOVE-RELEASES-ON-ASK, land leg `carried-forward`: a
  peer's code move between step 1 and step 6 makes step 4 ask a second
  time; the goal then holds the row's closer (`carrying ...
  reason=abandoned`), the posture of step 3 holds, and `landing
  carry-status` says `reservation: abandoned:<row>`.
- HCL-05-UNPROVABLE-INDEX-REFUSES, land leg `carried-record-failures`: an
  unmerged index entry under a valid word exits 1 with the words of
  commit.sh:263.
- HCL-05-CHAIN-PLUS-GROUP, land leg `carried-chain-group`: a closed chain
  whose only red group is named lands with `Carried-Past: group:<id>`
  and `Landing-Provenance` beginning `carried ... base=chain`.
- HCL-05-WRONG-REFUSAL-ASKS, internal/landing table test: the word names
  `missing-declaration`, O is `path-unclassified`, M empty; code
  `carry-refusal-mismatch`; the refusal text holds all three names.
- HCL-21-TWO-FAILURES-ONE-NAME, internal/landing table test and land leg
  `carried-asks`: O is `missing-declaration` and M is
  `{goal-full-coverage}` under a word naming either one alone; the code
  is `carry-refusal-mismatch`; no commit.
- HCL-21-EACH-NAMED-ALONE-LANDS, land leg `carried-asks`: the same tree
  after the coverage group goes green lands under the code word, and the
  same tree with a chain declared lands under the group word; each leaves
  exactly one commit.
- HCL-21-VERIFY-ERROR-ASKS, internal/landing table test and land leg
  `carried-asks`: `VerifyTesting` returns an error under a word naming
  `missing-declaration` with O `missing-declaration`; the code is
  `carry-battery-unverified` and the refusal holds the error text; no
  commit, no `Carried-Battery` anywhere.
- HCL-21-UNCOVERED-OBLIGATION-ASKS, internal/landing table test: R with
  both group lists empty, `UncoveredObligations` `{x}` and `Sufficient`
  false (the shape test_result.go:230-234 produces), O
  `missing-declaration` under the word `missing-declaration`; the code
  is `carry-refusal-mismatch` and the refusal names `x`.
- HCL-21-DISCREPANCY-ASKS, internal/landing table test: R with M `{G}`
  and `Discrepancies` `{y}` under the word `group:G`, O passing; the
  code is `carry-refusal-mismatch` and the refusal names `y`; the same R
  with `Discrepancies` empty lands.
- HCL-21-CODE-WORD-NEEDS-SUFFICIENT, internal/landing table test: O
  `missing-declaration` under that word with R failing `{G}`, and again
  with R uncovered `{x}`; both `carry-refusal-mismatch`; with R
  sufficient the tree lands.
- HCL-05-UNNEEDED-ASKS, land leg `carried-asks` and internal/landing:
  the word names `missing-declaration`, a chain is declared and O
  passes, R sufficient; code `carry-unneeded`, exit 3; `git rev-parse HEAD`
  unchanged, no `carried` row on the goal, and `landing carry-status`
  says `ok` and `none`.
- HCL-05-LIVE-JUDGE-DEAD-BASE-JUDGES, land leg `carried-asks`:
  `METASYSTEM_BIN` points at a binary that exits 9; the commit lands with
  `Carried-Judge: base tree=<HEAD tree> sha256=<digest> live-failure=exit=9`
  and `Landing-Provenance` holding `judge=base live-failure=exit=9`.
- HCL-03-LIVE-FAILURE-STAMPED, internal/landing: `--judge base
  --live-failure evaluator-unavailable` yields Provenance containing
  `judge=base live-failure=evaluator-unavailable`; `--judge base` with
  no `--live-failure` is a usage error (exit 2) from landing_verbs.go.
- HCL-03-BASE-JUDGE-BLIND-ASKS, internal/landing table test: one row per
  owner of step 11, each a candidate that changes one file under
  `--judge base`: `internal/landing/receipt.go`, a NEW file
  `internal/landing/carried_probe.go`, `internal/goal/file.go`,
  `internal/proofrun/test_result.go`, `internal/testpolicy/select.go`,
  `internal/behaviorsurface/policy.v2.json`, `internal/config/budget.go`,
  `internal/refusal/register.go`, `internal/governance/types.go`,
  `internal/humanauthority/authority.go`,
  `internal/fixtureauth/fixtureauth.go`, `metasystem.conf`,
  `testing.json`, `scripts/agents/landing-classes.json`,
  `scripts/agents/path-classes.txt`,
  `scripts/agents/landing-promotion.json`; every row is
  `carry-base-judge-blind` and the refusal names the path; a
  candidate that changes `internal/dispatch/claim.go` under `--judge
  base` reaches step 12, and every row under `--judge live` reaches
  step 12.
- HCL-05-NO-JUDGE-ASKS, land leg `carried-asks`: a dead live engine and
  `go` absent from PATH; exit 3 naming the build failure and `steward
  arm`; no commit.
- HCL-05-MAIN-ONLY, land leg `carried-asks`: on branch `topic`, exit 3
  with "the carried landing lands main; you are on topic".
- HCL-05-RECORD-FAILURES-STAY, land leg `carried-record-failures`: one
  leg drives the list above (a staged gitlink, a symlinked AGENTS.md, a
  skip-worktree entry, an empty staging set, a typed `Goal-Item`); each
  exits non-zero under a valid word and leaves HEAD unchanged.
- HCL-04-LEASE-HELD-THROUGHOUT, land leg `carried-forward`: with the
  fixture pause seam `METASYSTEM_LAND_FIXTURE_PAUSE=carry-forward`
  (honored only when `FixtureModeRoot`), a concurrent `lease renew` from
  the bed exits non-zero with "checkout lease lock is busy"; after the
  pause the landing completes and `lease require-holder` reports the
  same claim epoch it started with.
- HCL-19-RECOVERY-KEEPS-REBASED-COMMIT, land leg `carried-crash`: a
  local carried commit `C` with origin moved by a ledger commit; the
  rerun pushes a commit whose tree's workspace projection equals `C`'s,
  whose parent is the new origin tip, whose `Carry:` equals the opid;
  `git log --grep 'Carry: <opid>' origin/main` prints exactly one sha.
- HCL-28-PEER-DEBT-SEEN, land leg `carried-asks` with two clones: clone
  B lands a carry after clone A's word was written; clone A's landing,
  whose accepted ref was behind B's row before the run, exits 3 with
  `carry-debt-unpaid` naming B's obligation; `Carried-Ledger` is never
  stamped.
- HCL-33-TWO-SEATS-INTERLEAVE, land leg `carried-asks` with two clones:
  clone A's wrapper is paused at the seam after its push (the `carrying`
  row is on the ledger, the commit is on origin, no `carried` row);
  clone B, with its own word on its own goal, runs `land.sh --carried`;
  B exits 3 with `carry-debt-unpaid` naming A's `carrying` row and A's
  seat, at B's reservation (05 step 5); origin holds no commit from B
  and B's goal holds no `carrying` row; A resumes and completes; B's
  rerun exits 3 naming A's obligation instead (HCL-28's oracle).
- HCL-33-TRAILER-WITHOUT-ROW-IS-DEBT, land leg `carried-asks` with two
  clones, two legs: A's wrapper is killed after its push (the commit is
  on origin, no `carried` row), then (a) A's reservation is released by
  hand (`goal carrying --abandon` on A; `landing carry-status` on A says
  `reservation: abandoned:<row>`), or (b) the bed advances its clock
  past A's word's expiry, so the word and its reservation are both
  expired (`carry-status` says `expired` and `reservation:
  expired:<row>`); in both legs B's landing, with its own word on its
  own goal, exits 3 with `carry-debt-unpaid` naming A's commit and A's
  word and `land.sh --carried <A's word>` (the third form of step 9),
  origin holds no commit from B and B's goal holds no `carrying` row;
  in both legs A's rerun takes the `origin:` branch of 06 and completes
  the record (the `carried` row is written under an expired word, 06
  step 6), after which B's rerun names A's obligation instead.
- HCL-29-SEAT-MISMATCH-ASKS, internal/landing: a word whose opid machine
  is `seat-a` observed with `--actor seat-b+<lineage>` is
  `carry-seat-mismatch` and the refusal holds `goal carry --supersede
  <opid> --transfer`.

## HCL-TRANSACTION-06: the order of writes, what a crash leaves

Seven steps, in this order, each idempotent by the carry opid:

1. The row (04) exists in the ledger. Nothing else is written by the word.
2. Carry-forward (05) so HEAD equals origin/main, under the lease, and
   the tree compare.
3. The reservation (HCL-C-33): `goal carrying --root . --id <goal> --ref
   <opid> --tree <sha40> --by human:<name>` publishes the `carrying` row
   of 08 as one agent-run goal transaction through `Publish`, Intent
   `{Verb: "carrying", Targets: [<goal>], Args: {approvedRef, workspace,
   tree, expires, by}}`; its journal entry confirms on the push as every
   ledger verb's does, and `carryingRequest(r, args)` is the constructor
   both the verb and `requestForEntry` (recover.go:217-244) use, so a
   dead seat's created reservation entry is completed or abandoned by
   the ordinary rule. Then carry-forward again onto the ledger commit it
   made (05 step 6). The accepted ref after it is `L`.
4. The commit with the carried trailers, on local main (commit.sh with
   `--ledger-tip L`).
5. The intent, then the push. Before the push the wrapper writes the
   `carried` intent as a local journal entry through the goal command
   layer (HCL-C-18): `goal carrying --root . --id <goal> --ref <opid>
   --carrying <row opid> --commit <sha40> --tree <sha40> --workspace
   <sha40> --past <name> --battery <green|red> [--missing <csv>]
   [--failing <csv>] --judge <live|base> [--judge-tree <sha40>]
   --judge-digest <sha256> [--live-failure <code|exit=n>] --ledger <L>
   --by human:<name> --owner-pid $$`. With `--commit` the verb writes no
   ledger row: it requires an open `carrying` row for `--ref` on this
   seat at the accepted tip whose `workspace=` equals `--workspace`
   (else it refuses: reserve first), mints one operation ulid, derives
   the entry's opid from the seat's machine and lineage (`syncReq`, the
   same identity every agent verb uses), and calls a new
   `goal.CreateCarryingEntry`, which is `CreateEntry` (journal.go:
   231-256) with two differences: the Intent is `{Verb: "carried",
   Targets: [<goal>], Args: {approvedRef, carrying, commit, tree,
   workspace, past, battery, missing, failing, judge, judgeTree,
   judgeDigest, liveFailure, ledger, by, outcome: landed}}`, which is
   every field the `carried` row, the obligation and the counselor line
   need (08; HCL-C-25); and the owner is not `SelfOwner` but the process
   named by `--owner-pid`, which must be a live ancestor of the caller
   (the parent walk `humanauthority.Prove` already makes,
   authority.go:611-693, with the kernel prober's start identity,
   journal.go:142-151). The owner is therefore the running land.sh, alive
   for the whole sequence, so recovery leaves the entry to its owner
   (`ActionLeaveToOwner`, recover.go:115-116) while the wrapper lives and
   completes it only when the wrapper is provably dead. A created
   `carried` entry for the same ref whose owner is dead (a rerun after a
   crash before the push, whose commit was rebased to a new id) is
   closed `abandoned` first (`MarkTerminal`, journal.go:314-335); a live
   owner's is refused. The entry's opid is printed and becomes the
   `carried` row's opid. Then the push (05), one attempt.
6. `goal carried --root . --entry <entry opid>`, one agent-run goal
   operation that consumes the entry: it reads the entry, requires
   Intent.Verb `carried` and Phase `created` (journal.go:31-33), requires
   the caller to be a descendant of the live owner (the same ancestor
   walk; a dead owner is taken over as `TakeOver` does, :368-396), and
   reassigns the owner to itself, then runs the transaction under that
   entry through a new `goal.CompleteEntry(e, req)`, an exported wrapper
   over `runTransaction` (txn.go:556) that requires an existing created
   entry whose Intent equals `req.Intent`. `Publish` is not used, since it
   refuses an existing entry (txn.go:522-543). The request is
   `carriedRequest(r, args)`, the constructor recovery also uses; its
   Mutate appends the `carried` row, which closes the `carrying` row
   (08), and the obligation (07); when an open `carrying` row for the
   ref exists its `workspace=` must equal the intent's, else the
   rejection names both (a landing under a reservation for another tree
   is a wrapper defect); when none exists (expired, abandoned, a
   rebuilt journal) the row is written anyway, since the trailer on
   origin is the truth. Its `AfterConfirmed` (txn.go:488-491) appends
   the counselor line (08) from the `carried` row it reads on the
   confirmed tip, which is the postcondition `landing carry-status`
   reports as `counselor: written|missing`. The two are one command
   whose success means both are present. `goal carried` runs no fetch
   of its own: the ledger transaction captures the canonical tip itself
   (`CaptureTip`, txn.go:566).
7. Transport sync (scripts/agents/sync-transport.sh, which mirrors
   origin's ref, never the local branch).

Consumption (HCL-C-24), without a clock. A word is consumed when a
`carried` row with ApprovedRef equal to its opid exists on the tree, or
when the code history carries the trailer `Carry: <opid>` in the
anchored range. The anchor is the ledger commit that published the word
row: the commit on the code tip whose `Goal-Transaction` trailer equals
the row's opid, found by the walk `TrailerPresent` makes (txn.go:379-390),
`git log --format='%H %(trailers:key=Goal-Transaction,valueonly)'
<code tip>` and the line whose value is the opid; call it `A`. The code
tip is `refs/remotes/origin/main` in remote mode and `refs/heads/main`
in local mode (`Endpoint.LocalMode`, project.go:74). The range is
`A..<code tip>`, scanned with `git log --format='%H
%(trailers:key=Carry,valueonly)' A..<code tip>` for the line whose value
is exactly the opid; the carried commit is pushed on top of a fetched
tip that already holds `A`, so it is always in the range, and no
timestamp is read. `A` absent from the code tip means the word row is not
published where the landing can see it: `carry-word-missing` with "fetch
the ledger". `git log --since` is not used anywhere on this page.

`landing carry-status --root . --carried <opid> --goal <id> --ledger-tip
L` is a small read-only verb that reports the word's state (`ok`,
`missing`, `unproven`, `expired`), the consumption (`none`,
`origin:<sha40>`, `ledger:<opid of the carried row>`,
`superseded:<opid>`, `local:<sha40>` for a commit at HEAD with the
trailer that is not on origin), the reservation (`reservation:
none|open:<row opid>|expired:<row opid>|abandoned:<row opid>|closed:<row
opid>`, the newest `carrying` row for this ref on the goal and its state
under 08's rule), the intent (`carrying:<entry opid>` when a
created-phase `carried` entry for this ref exists in the journal, else
`none`) and, when the consumption is `ledger:`, the counselor
postcondition (`counselor: written|missing`, by the line id `cl-<opid>`
in `records/counselor/carried-landings.jsonl`). land.sh runs it in step
2 and branches:

- `none`, HEAD holds no carried commit: the fresh path, steps 2 to 7;
  with `reservation: open:<row>` the wrapper runs no transaction at 3
  and continues under that row.
- `local:<sha>`: crash after 4 before the push completed. The recovery
  path of 05: rebase if origin moved, re-check, make sure the
  reservation is open (05 recovery step 2a), push the rebased commit,
  continue at 5's intent entry (a fresh entry; the stale `carried`
  entry for this ref, which names a commit no longer at HEAD, is closed
  by the `--commit` form itself, as 5 says).
- `origin:<sha>` and no carried row: crash after the push before 6.
  Print "already landed as <sha>; completing the record". With a
  `carrying:<entry>` for this ref whose commit is `<sha>`: `goal carried
  --entry <entry>` then 7, exit 0. Without one (the entry was lost with
  the checkout, or `goal recover` already closed it): `goal carried`
  with `--rebuild-from-commit <sha>`, which derives every intent field
  from the commit's trailers (`Carried-Tree`, `Carried-Past`,
  `Carried-Battery`, `Carried-Judge`, `Carried-Ledger`, `Carried-By`;
  the judge's mode, tree, digest and live failure are all in
  `Carried-Judge`, the two group lists in `Carried-Battery`) and runs as
  a fresh publish; then 7, exit 0. The `carried` row closes the
  reservation whatever its state.
- `ledger:<opid>`: crash after 6 before 7, or a replay. When `counselor:
  missing`, run `goal carried --repair-counselor --root . --ref <carry
  opid>` (HCL-C-26): a read of the accepted tip and no transaction; it
  finds the `carried` row with ApprovedRef `<carry opid>` on any live or
  done goal, builds the line from that row's fields alone with
  `counselor.CarriedLandingLine(row)`, the function the confirmed hook
  also runs (below), and appends it idempotently by id
  (`appendRegisterLine`, register.go:56-96); no row means exit 1 naming
  the ref. Then 7 unless `--skip-transport`; print where it landed; exit
  0. A replay lands nothing.
- `superseded:<opid>` (HCL-C-23): a terminal no-landing state. Print
  "word <carried> was superseded by <opid>; land under it: `land.sh
  --carried <opid>`", exit 3. No commit, no row, no transport.
- `expired` or `missing` or `unproven` with no consumption: the ask for
  a fresh word (`goal carry`). An expired word with no intent and no
  trailer needs no recovery: it cannot be used, its state is `expired`,
  and `goal done` treats it as closed (below).

Idempotence and replay (HCL-C-18, HCL-C-25). `goal carried`'s Mutate
finds an existing `carried` row with ApprovedRef equal to the intent's
`approvedRef`, on any live or done goal, before and independent of
`opidLanded` (verbs.go:444-451). First the target: the intent's one
target, `Intent.Targets[0]` (journal.go:57-66 makes targets part of the
complete intent; 06 step 5 stores the goal there), must equal the goal
the row is on (the row's `targets=`, `HistoryLine.Targets`,
file.go:298, which is the goal file the row was found in); a row on
another goal is refused before any field is read, `carried replay
refused: goal differs: row=<a> intent=<b>`, recorded as a rejection,
nothing written. It then compares every field the row
carries with the intent, and the row carries every field the
obligation and the counselor line need (08): `commit`, `workspace`,
`project` (the intent's `tree`), `past`, `battery`, `missing`,
`failing`, `judge`, `judgeTree`, `judgeDigest`, `liveFailure`,
`ledger`, the outcome (the reason's first word) and `by`: fourteen
fields, compared as strings after the row's parse, an absent list
being `-`. All equal: `AlreadyApplied{}` (txn.go:449-457), whose leg
runs `AfterConfirmed` and so repairs a missing counselor line
(txn.go:733-743; `appendRegisterLine` is idempotent by id,
register.go:56-96). Any difference: an error naming the field and both
values (`carried replay refused: commit differs: row=<a> intent=<b>`),
recorded as a rejection; nothing is written. The comment at
txn.go:449-455 gains one sentence: for `carried` the truth predicate is
the ApprovedRef, since a word is consumed exactly once, and a
fresh-opid replay is lawful there alone.

Recovery (HCL-C-18, HCL-C-33). `carried` gains a `requestForEntry` case
(recover.go:244): it is agent-run and proof-free, so the case rebuilds
`carriedRequest(r, in.Args)` from the entry's args through the same
constructor the live verb uses (unlike `resume` and `set-budget`,
:146-152, :309-310); `carried` carries no `claimEpoch`. Before the
rebuilt Mutate writes anything it proves the postcondition its intent
asserts: the intent's commit is in the anchored range with `Carry:
<approvedRef>` (the scan above, on the code tip as the clone has it
after `git fetch origin main`, which recovery runs first for a `carried`
entry). Present: the row and the obligation are written, the counselor
line appended by `AfterConfirmed`. Absent, with the reservation named
by the entry's `carrying` arg still open on the tip: the callback
writes its closer instead, a `carrying` row `reason=abandoned of=<row
opid> why=the carried commit <sha> is not on origin` (08) under the
entry's opid, since a dead wrapper's reservation must stop blocking the
fleet; the entry confirms and the report says "the carried commit <sha>
is not on origin; the reservation <row> closed; the word <opid> stays
open"; the hook then finds no `carried` row for the ref on the tip and
appends nothing. Absent, with no open reservation: the callback returns
`NothingToDo{"the carried commit <sha> is not on origin; the word
<opid> stays open"}`, which terminalizes the entry as abandoned
(txn.go:744-749) and writes nothing. Either way the human reruns
`land.sh --carried`, which finds `local:<sha>`, reserves again if it
must, and pushes. A `carried` entry whose owner is alive is never
touched (recover.go:115-116). The consumer list of
`AuthenticatedChannelApproval` (verbs.go:82) and the history grammar's
`approvedRef=` rule (file.go:1465-1467) both gain `carrying` and
`carried`.

The confirmed leg (HCL-C-26). A crash between the confirming refetch
and the hook (txn.go:671-682) leaves the entry `pushed`, the `carried`
row on the ledger and no counselor line; a hook error leaves the same
world with the entry `pushed` (:677-681). Recovery classifies the entry
`ActionConfirm` (journal.go:483-485) and today runs
`recoverSplitConfirmedEffect` on both confirm legs, a no-op for every
verb but `split` (recover.go:75-78, 93-96, 416-424). This design
renames that call `recoverConfirmedEffect(e, tip, entry)`, a switch on
the entry's verb whose `split` case is today's body and whose
`carried` case is the replay: read the `carried` row whose ApprovedRef
is `entry.Intent.Args["approvedRef"]` from the tip's tree (the payload
source is the row, never the entry's args), build the line with
`counselor.CarriedLandingLine(row)`, the one function `carriedRequest`'s
`AfterConfirmed` and `goal carried --repair-counselor` also call, and
append it through `appendRegisterLine`, idempotent by id
(register.go:56-96). No verb reaches another verb's effect. When the
row is absent from the tip (the closer leg above landed under this
opid) the case appends nothing. `goal recover` therefore owns the crash
between publication and the hook; `--repair-counselor` owns a line lost
after the entry closed, when the journal has nothing left to say.

`goal done` and open words (HCL-C-05). `doneRequest` (verbs.go:1106-1182)
gains, beside the open-obligation check (:1128-1132), a refusal while
any carry word on the goal is open: proven, unexpired at `r.Now`, and
unconsumed by the definition above. The refusal names the opid and the
closers: land it, supersede it, or let it expire. An expired word is
closed for `done` with no row written: expiry is a fact of the row (`expires=`
or At plus two hours) that every reader computes the same way, so no
recovery and no `expired` row is needed; revision 3's `expired` outcome
and its recovery case are withdrawn. A word consumed on origin with an
incomplete record (`origin:<sha>` and no row) is closed by the landing's
own `origin:` branch or by `goal recover` when a `carried` entry
exists; `done` refuses it until then, expired or not (the third debt
form of 08 reads the same two facts), naming `land.sh --carried <opid>`
as the closer. A `carrying` row needs no rule of its own here: it is
open only while its word is (one expiry, and the same `carried` row
closes both, 08), so the open-word refusal covers it.

Fixtures of 06:

- HCL-06-ORDER, land leg `carried-fresh`: the seven steps in order,
  proved by the bed's step log (the `run_required_step` names) and by
  the world after each of six crash points: after 3 (the `carrying` row
  is on origin's ledger, HEAD is origin/main, the index holds the
  candidate); after 4 (HEAD holds `Carry:`, origin does not, the journal
  has no `carried` entry); after the intent entry (the entry is
  `created`, owner the paused land.sh, origin unchanged); after the push
  (origin holds the commit, no `carried` row, the `carrying` row open);
  after `goal carried` (the row and the counselor line exist, the
  `carrying` row is closed, transport does not hold the commit); after 7
  (transport holds it). The pause seam of HCL-04-LEASE-HELD-THROUGHOUT
  names each point.
- HCL-06-CRASH-BEFORE-PUSH-RESUMES, land leg `carried-crash`: killed at
  the pause after 4; the rerun reports `local:<sha>` and `reservation:
  open:<row>`, pushes `<sha>` (origin unmoved), writes the row; one
  commit on origin, one `carrying` row on the goal.
- HCL-06-CRASH-AFTER-PUSH-COMPLETES-RECORD, land leg `carried-crash`:
  killed at the pause after the push; the rerun prints "already landed
  as <sha>; completing the record", exits 0; exactly one `carried` row
  and one counselor line; no second commit.
- HCL-06-REPLAY-REFUSED, land leg `carried-crash`: a third run after a
  complete landing exits 0, prints where it landed, and the commit count
  on origin and the row count on the goal are unchanged.
- HCL-06-CARRIED-IDEMPOTENT, internal/goal: `goal carried` twice with
  the same entry opid: the second returns `OutcomeConfirmed` with Detail
  `idempotent` through `Publish`'s terminal-entry leg (txn.go:522-540);
  one row.
- HCL-06-CONSUMED-BY-APPROVED-REF, internal/goal: a second `carried`
  with a fresh opid and the same intent is `AlreadyApplied`; one row;
  the counselor line count is one.
- HCL-25-REPLAY-MISMATCH-REFUSED, internal/goal: a second `carried` with
  a fresh opid, the same ref and a different commit is rejected with
  `commit differs`; one row; a table drives the goal target first and
  then each of the fourteen fields in turn (the target: a second
  `carried` whose intent names another live goal of the tree with the
  same ref and every field equal is rejected `goal differs` naming both
  goals, and that other goal holds no row; then `commit`, `workspace`,
  `project`, `past`, `battery`, `missing`, `failing`, `judge`,
  `judgeTree`, `judgeDigest`, `liveFailure`, `ledger`, the outcome,
  `by`), each rejected naming its field and both values, the row count
  staying one and the counselor line count one.
- HCL-26-COUNSELOR-REPAIRED, land leg `carried-crash`: the counselor
  file is deleted after a complete landing; `landing carry-status` says
  `counselor: missing`; the rerun runs `goal carried --repair-counselor`,
  which repairs it (one line, id `cl-<opid>`, fields equal to the row's)
  and lands nothing.
- HCL-26-CRASH-BEFORE-HOOK-RECOVERS, internal/goal: through a package
  seam `carriedCounselorAppend` (a variable holding the append function,
  the shape of `fetchForProjection`, project.go:45) the hook fails once;
  the transaction returns with the entry `pushed` and the row on the tip
  (txn.go:677-681); `RecoverWithPolicy` reports `confirm`, the line
  exists exactly once with id `cl-<opid>`, the entry is `confirmed`; a
  second `RecoverWithPolicy` changes nothing; the goal file holds no row
  the split effect would have written.
- HCL-26-REPAIR-FROM-ROW-ALONE, cmd/metasystem: with the journal
  directory removed and the line deleted, `goal carried
  --repair-counselor --ref <opid>` appends one line whose fields equal
  the `carried` row's; a second run appends nothing; with no `carried`
  row for the ref it exits 1 naming the ref and appends nothing.
- HCL-23-SUPERSEDED-LANDS-NOTHING, land leg `carried-asks`: `land.sh
  --carried <superseded opid>` exits 3 printing the superseding opid;
  `git rev-parse HEAD` unchanged, `git log origin/main` unchanged, no new
  `carried` row, the transport ref unchanged.
- HCL-24-CONSUMPTION-WITHOUT-CLOCK, land leg `carried-crash`: the
  carried commit is created with `GIT_COMMITTER_DATE` one day before the
  word's At; after the push, `landing carry-status` says
  `origin:<sha>` and the classification says `carry-word-consumed`.
- HCL-18-RECOVER-COMPLETES-CARRIED, internal/goal: a `created`
  `carried` entry with a dead owner whose commit is on the fixture
  origin; `RecoverWithPolicy` reports "completed from the stored intent",
  the row, the obligation and the counselor line exist.
- HCL-18-RECOVER-ABANDONS-UNPUSHED-INTENT, internal/goal: the same
  entry with the commit absent from origin, in two legs. With the
  reservation open: the report says "not on origin; the reservation
  <row> closed; the word stays open", the entry is terminal `confirmed`
  with its opid on the closer row, the goal holds `carrying ...
  reason=abandoned of=<row>`, no `carried` row, and `landing
  carry-status` says `ok`, `none`, `reservation: abandoned:<row>`. With
  no open reservation: the report says "not on origin; the word stays
  open", the entry is terminal `abandoned`, nothing is written.
- HCL-33-RESERVATION-ROW, internal/goal: `goal carrying` writes the row
  of 08 with `workspace=` the projection of `--tree`, `expires=` equal
  to the word's, and the opid's machine equal to the caller's; a second
  call on the same seat prints the same row opid and creates no journal
  entry; `RenderHistoryLine` of the row parses back to the same struct.
- HCL-33-RESERVATION-CAS, internal/goal: through `BeforePush` a
  competitor publishes a `carrying` row on another live goal between
  capture and push; the rebuild rejects `carry-debt-unpaid` naming the
  competitor's row and seat; no row for this word.
- HCL-33-RESERVATION-RECHECKS-WORD, internal/goal: `goal carrying` for a
  consumed word, an expired word and another seat's word is rejected
  with `carry-word-consumed`, `carry-word-expired` and
  `carry-seat-mismatch`; no row.
- HCL-33-ABANDON-CLOSES, internal/goal and cmd/metasystem: `goal carrying
  --abandon <row>` from the row's seat writes the closer and
  terminalizes the created `carried` entry for the ref whose owner is
  dead; from another seat it exits 3 naming the seat and the expiry;
  with the entry's owner alive it exits 3 with "in flight".
- HCL-33-EXPIRED-RESERVATION-NOT-DEBT, internal/landing: a `carrying`
  row past its `expires=` is not counted by the in-flight form of step
  9 and `carry-status` says `reservation: expired:<row>`; the same row
  before its expiry is `carry-debt-unpaid` for another word's landing;
  the same expired row with its word's `Carry:` trailer on the code tip
  and no `carried` row is `carry-debt-unpaid` by the third form, naming
  the commit and the word, not the row.
- HCL-33-CARRIED-CLOSES-RESERVATION, internal/goal: after `carried` the
  goal's `carrying` row is closed (`carry-status` says
  `closed:<row>`), and a `carried` whose intent workspace differs from
  the open row's is rejected naming both trees.
- HCL-18-LIVE-OWNER-UNTOUCHED, internal/goal: a `created` `carried`
  entry whose owner pid is the test process; `RecoverWithPolicy` reports
  "a live owner's entry; untouched".
- HCL-18-OWNER-MUST-BE-ANCESTOR, cmd/metasystem: `goal carrying
  --owner-pid <a live pid that is not an ancestor>` exits 2 with "the
  owner must be a live ancestor of the caller"; no entry.
- HCL-06-DONE-REFUSES-OPEN-CARRY, internal/goal: `done` on a goal with
  an unexpired unconsumed word is rejected naming the opid and the three
  closers.
- HCL-06-EXPIRED-WORD-IS-CLOSED, internal/goal: `done` on a goal whose
  only word is expired succeeds; the goal archives; no `carried` row was
  written.
- HCL-06-CRASH-BEFORE-TRANSPORT-COMPLETES, land leg `carried-crash`:
  killed after 6; the rerun runs transport only; the transport ref
  equals origin's.
- HCL-06-CARRIED-REBUILDS-FROM-COMMIT, land leg `carried-crash`: after
  the push the journal directory is removed; the rerun's `origin:`
  branch rebuilds the intent from the trailers and writes a row whose
  fields equal the trailers.

## HCL-OBLIGATION-07: review deferred, never deleted

`goal carried` with outcome `landed` appends one `ReviewObligation`
(file.go:147, 528-532, 1236-1238; the schema is unchanged): Finding
`carried:<sha40>`, or `carried:<sha40>:battery-red` when the battery
trailer is red so the discharge cannot forget it (no landed battery is
unverified, 05 step 14); Chain
`human-carried`; Artifact `commit:<sha40>`; Test `pending`; State `open`.
The finding carries the full forty-digit commit id (HCL-C-06); the match
in `reviewObligationMatch` (verbs.go:1021-1035) is on Finding and Chain,
and two carries of one commit cannot occur (one word, one commit). It
also increments BudgetExceptions (saturating, as set-budget does,
verbs.go:754-756) and writes the history row of 08.

Discharge, two ways:

- By critic. A code-critic chain dispatched with `--reviews
  commit:<sha40>` (09) whose terminal return has `verdictMaterialCount`
  0 and whose register is closed; then `goal discharge-review-obligation
  --id <goal> --finding carried:<sha40>[:battery-red] --chain
  human-carried --by <name> --test <critic root job id>`. The command
  layer (`runGoalDischargeReviewObligation`, goalsync_mutations.go:
  289-305) gains, for chain `human-carried` only: read the named job
  record and its terminal return; require role `code-critic`, `reviews`
  equal to `commit:` plus the obligation's Artifact commit, status
  `completed`, material count zero, register closed
  (`findingRegisterRoundField` folded through the terminal round, the
  test `requireFoldedCritique` already makes, review_reference.go:
  219-231). Any other citation is refused with what was expected. The
  goal package's verb (verbs.go:989-1019) is unchanged.
- By accepted risk. `goal accept-risk --id <goal> --finding
  carried:<sha40>[:battery-red] --chain human-carried --by <name> --why
  "<text>"` from the human. Command-layer effects for chain
  `human-carried` (HCL-C-07), each named: (a) skip
  `CritiqueRegisterDecisionFinding` (goalsync_mutations.go:326;
  finding_register.go:296-334 reads a job record that does not exist);
  (b) skip `CritiqueRegisterAcceptRisk` (:354; :359-385); (c) write the
  counselor line through a new `counselor.AppendCarriedAcceptedRisk`,
  defined in full below; (d) record the proof as today (:358). The goal
  package's `AcceptedRiskDecision` (verbs.go:1037-1074) gains one change,
  for chain `human-carried` only: after appending the AcceptedRisk record
  it marks the matching open obligation discharged with Test
  `accepted-risk:<opid of the accept-risk row>`.

The carried accepted-risk counselor record (HCL-C-07). Today's line
(`AppendAcceptedRisk`, register.go:26-47; the types at sources.go:48-76)
draws title, claim, evidence and class from the critic finding the
command layer reads (goalsync_mutations.go:326, 350). For chain
`human-carried` every field has a deterministic source in the commit and
the command line, and none needs a job record:

| Field (sources.go:48-76) | Source for chain `human-carried` |
| --- | --- |
| `schemaVersion` | 1 |
| `id` | `ar-human-carried-<finding>` (register.go:45's shape with the chain in place of the root job) |
| `recordedAt` | `req.Now`, RFC3339 UTC |
| `kind` | `accepted-risk` |
| `class` | `carried-green` or `carried-red`, from the commit's `Carried-Battery` trailer (the class today is the finding's rigor class; the battery is the carried landing's rigor fact; `carried-unverified` is withdrawn with the trailer value, 05) |
| `title` | the finding id, `carried:<sha40>[:battery-red]` (under 120 bytes) |
| `acceptanceStatus` | `accepted` |
| `acceptanceReason` | `--why` verbatim |
| `specimenFacts` | one fact per carried trailer line of `git log -1 --format=%B <sha>` (`Carry`, `Carried-By`, `Carried-Tree`, `Carried-Past`, `Carried-Battery`, `Carried-Judge`, `Carried-Ledger`, `Landing-Provenance`), each with one citation `{kind: commit, target: <sha40>, detail: <finding>}` (today's citation is `job-record`, register.go:35) |
| `reviewLinks` | two: `{kind: goal, target: plans/goals/<id>.md, detail: opid=<accept-risk opid>}` as today (register.go:45) and `{kind: commit, target: <sha40>, detail: <finding>}` |

Replay is idempotent at two levels, and a replay that changes an
immutable input is refused (HCL-C-07). The record's immutable inputs
are the finding, the chain, `--by` and `--why`; the class and the
specimen facts derive from the commit the finding names, so once the
finding matches they cannot differ. `AcceptedRiskDecision`
(verbs.go:1037-1074) for chain `human-carried` finds the prior record
by finding and chain (:1061-1062), then the history row whose Opid is
the record's Opid (the row `touch` wrote, :1070, whose Reason is the
recorded `--why`, :1071), and compares `by` and that Reason with the
new `--by` and `--why` byte for byte. Both equal: `AlreadyApplied`. A
different `by`: refused as today (:1063-1065). A different why: refused
`accepted-risk replay refused: why differs: recorded=<a> given=<b>`. A
record whose history row cannot be found: refused as unrepeatable,
naming the opid. The other chains keep :1061-1068 unchanged; their
same gap belongs to the critique register's design and is listed under
the gaps. The goal transaction runs before effect (c), so a refused
replay appends no counselor line, and `appendRegisterLine` skips a line
whose id already exists (register.go:56-96) on the accepted replay; a
second `goal accept-risk` with the same four inputs therefore writes
nothing and exits 0. A commit whose trailers cannot be read (not a
carried commit) is refused before any write, naming the missing trailer.

`goal done` refuses while the obligation is open (existing law,
verbs.go:1128-1132).

Fixtures of 07:

- HCL-07-OBLIGATION-WRITTEN, internal/goal: after `carried` with a green
  battery the goal holds one obligation `{carried:<sha40>,
  human-carried, commit:<sha40>, pending, open}` and BudgetExceptions is
  one higher.
- HCL-07-CONCLUDE-REFUSED-OPEN, internal/goal: `done` on that goal is
  rejected with the existing open-obligation text.
- HCL-07-DISCHARGE-BY-CRITIC, cmd/metasystem: a fixture job record
  (role `code-critic`, `reviews commit:<sha>`, completed, material count
  zero, register closed) discharges; Test equals the job id.
- HCL-07-DISCHARGE-REFUSES-ARBITRARY-TEXT, cmd/metasystem: `--test
  "reviewed by hand"` exits 1 with "expected a code-critic root job
  reviewing commit:<sha>"; State stays `open`.
- HCL-07-DISCHARGE-BY-ACCEPTED-RISK, cmd/metasystem: `accept-risk` for
  the finding marks the obligation discharged with Test
  `accepted-risk:<opid>` and appends the AcceptedRisk record.
- HCL-07-RED-BATTERY-IN-FINDING, internal/goal: a red battery yields
  Finding `carried:<sha40>:battery-red`.
- HCL-07-FULL-ID-IN-FINDING, internal/goal: two obligations whose
  commits share seven leading digits discharge independently.
- HCL-07-ACCEPT-RISK-SKIPS-REGISTER, cmd/metasystem: no job record named
  `human-carried` is read (the fixture filesystem has none) and the
  command exits 0.
- HCL-07-ACCEPTED-RISK-LINE-COMPLETE, internal/counselor and
  cmd/metasystem: the appended line decodes to every field of the table
  above with the values derived from a fixture commit's trailers; a
  commit without `Carried-Battery` is refused with that key named and
  no line is appended.
- HCL-07-ACCEPTED-RISK-REPLAY-IDEMPOTENT, cmd/metasystem: a second
  `accept-risk` with the same `--by` and `--why` exits 0, the register
  holds one line with id `ar-human-carried-<finding>`, the goal holds
  one AcceptedRisk record; a second with another `--by` exits 1 naming
  the first.
- HCL-07-CHANGED-WHY-REFUSED, internal/goal and cmd/metasystem: a second
  `accept-risk` with the same `--by` and a different `--why` is
  rejected with `why differs` naming both texts; the goal holds one
  record whose history row still carries the first why, the register
  one line, and the obligation's Test is unchanged.

## HCL-COUNTER-08: every use counts, the voice, the cap, the debt

The `carried` row (HCL-C-25): `carried actor=<machine+lineage>
targets=<goal> approvedRef=<opid> reason=landed commit=<sha40>
workspace=<sha40> project=<sha40> past=<name> battery=<green|red>
missing=<csv|-> failing=<csv|-> judge=<live|base> judgeTree=<sha40|->
judgeDigest=<sha256> liveFailure=<code|exit=n|-> ledger=<sha40>
by=human:<name>`, the reason parsed as key-value fields after its first
word the way `NormApproval` is (`parseKVRecord`, file.go:831-848), so
the row holds every field the obligation (07) and the counselor line
(below) need and the replay of 06 compares all fourteen; `superseded
by=<opid>` is the other outcome (written by `goal carry --supersede`,
04), with no obligation and no exception. The `expired`
and `unneeded` outcomes of revision 3 are withdrawn: an expired word is
closed by its own expiry, and an unneeded word stays open (05, step 14).
Only `landed` increments BudgetExceptions; at two on one goal the
existing `repeated exception: defect signal` fires
(internal/steward/health.go:1027-1030).

The appetite line (health.go:1100-1104) gains ` carried=<k>` per claimed
goal, k the count of `landed` rows, and the health summary gains one
known line `CARRIED today=<n> open=<m> inflight=<i> debt=<d>`: landed
carries on the UTC day across live and done goals, open words, open
`carrying` rows, open `human-carried` obligations. `goal carried` with
outcome `landed` appends a line to
`records/counselor/carried-landings.jsonl` (`{schemaVersion:1, id:
"cl-<opid>", recordedAt, goal, opid, commit, workspace, project, past,
battery, missing, failing, judge: {mode, tree, digest, liveFailure},
ledger, by}`, every value read from the `carried` row by
`counselor.CarriedLandingLine(row)`, 06) through the same append the
accepted-risk register uses (`appendRegisterLine`, register.go:56-96),
from the `AfterConfirmed` hook of the `carried` transaction (06), so the
row and the line are one command and a replay repairs the line. The
design does not change how that file reaches origin and says so in the
gaps. This line is the "receipt marked carried" the umbrella asks for: a
schema-2 receipt asserts sufficiency (testing.go:27-29) and refuses
unknown fields (:173-182), so a carried landing writes no receipt and the
counselor line carries the verify result instead.

What the machine says, then does. At the word: the goal's tier and four
answers, or `risk: unanswered` (file.go:29 permits nil); the critic
rounds the tuple holds and that they are skipped; the seat's open carries
and the fleet's unpaid debt; the expiry; the ledger's format and, at
format 1, the raise-format line. At the landing, before the push: the
reservation's opid and the ledger tip `L`; the judge and, for the base
judge, the live failure; the ordinary verdict O and the testing result
R (its sufficiency and its four lists) as they will be stamped; the
obligation finding it will write; the exception count after this one.
Then it acts. Nothing in the lines is a condition. From the word to the
push is two commands (`goal carry`, `land.sh --carried`) or one channel
reply and one command; nothing asks a second question once the word is
proven and the landing hits what it names.

### The cap on open carries per seat (addition 1)

Key: `metasystem.budget.carry-open-max`, default 1, a committed line in
`metasystem.conf` beside the tier boxes (:16-18). It is budget law: it is
read through `TierBoxSet.lawValue` (internal/config/budget.go:168-203),
which refuses an environment or `.local` source outside a
fixture-authorized root with the text every budget-law key uses
(`... accepts only committed root configuration outside a
fixture-authorized root; .local source ... is refused`), and `config
validate` adds the same two refusals for the key that
`ElapsedGracePercentKey` has (validate.go:521-538). `config.CarryOpenMax
(confPath) (uint64, error)` is the one reader; zero is refused as
malformed (a cap of zero would refuse every human).

Open words for a seat: proven words on live goals of the one ledger tree
the caller read (`L` at the landing; the accepted ref as `goal carry`
finds it, which says which tip it read), unconsumed and unexpired, whose
seat (04: the opid's machine segment) is the seat's machine. Open is
the cap's predicate (step 10, `goal carry`'s count, the health line's
`open=`, and what `done` calls open in 06); the debt of addition 2
never reads it: its third form scans row-less words whether open or
not (HCL-C-33). At the
word, `goal carry` counts the OTHER open words for its machine; at or
above the cap it asks (exit 3): it names them, their trees and expiries,
and the resolutions, `--supersede <opid>` (04, which consumes the old
word in the same transaction) or letting one expire. At the landing,
step 10 of the classification asks the same way for both kinds of word
(`carry-cap-reached`). No threshold refuses; the cap asks which word
stands. Because the landing's actor must be the word's seat (05, step
4a), a seat cannot spend another seat's allowance and the count is the
same whichever process makes it.

### No stacking on unpaid carry debt (addition 2)

Unpaid has three forms, one per interval of a carry's life, so no
interval is invisible to another seat (HCL-C-33):

- Reviewed late: an obligation with Chain `human-carried` and State
  `open` on any live goal of the one ledger tree whose Artifact commit
  is an ancestor of the base. The base is `refs/remotes/origin/main` as
  the checkout has it at the word (`goal carry` does not fetch; it says
  which ref and which ledger tip it read) and HEAD at the landing
  (after the fetch, the fresh ledger tip and the carry-forward of 05,
  HEAD is origin/main and the tree is `L`). The check is `git
  merge-base --is-ancestor <commit> <base>`.
- In flight: an open `carrying` row (the reservation, below) on any
  live goal of the tree, other than the one whose ApprovedRef is the
  word being landed. No ancestry: it reserves a landing that is on no
  base yet.
- Landed, record incomplete (HCL-C-33): a `Carry: <w>` trailer in the
  anchored range of 06 for any proven carry word `w` (terminal or
  channel) on any live or done goal of the tree that has no `carried`
  row with ApprovedRef `w` (any outcome) on any live or done goal. The
  arm reads those two facts and nothing about `w`'s state: `w` may be
  expired, its reservation open, expired or abandoned; the trailer
  itself is what consumed it, so the open-word predicate of the cap
  (which requires unconsumed and unexpired) can never see it and is
  not consulted here. This is the interval after the push and before
  `goal carried`, however long it lasts, and it is what makes the
  expiry of a reservation harmless: the commit itself is the debt from
  the moment it is on the base, until `goal carried` (the landing's
  `origin:` branch of 06, or `--rebuild-from-commit`) or `goal
  recover`'s completed `carried` entry writes the row. The scan is the
  scan of 06 (`A..<code tip>` for `Carry:` equal to `w`), run once per
  row-less word; the row-less words are every carry word ever spoken
  that was neither landed with its row nor superseded, so their number
  grows only with unused expired words, each a bounded `git log`. The
  code tip: at the landing, the code tip of 06 after the fetch; in
  `goal carrying`'s Mutate, the captured tip in remote mode and
  `refs/heads/main` in single-machine mode, exactly as 04's supersede
  scan; at the word, `refs/remotes/origin/main` as the checkout has it
  (the gap already recorded for `goal carry`).

The checks run in the command layer at the word, in `goal carrying`'s
Mutate at the reservation, and in the classification at the landing
(step 9). All are shape (c): the ask names the obligation, its goal and
the two closers (`goal discharge-review-obligation` with a critic of the
commit and `goal accept-risk`), or the row, its seat and its expiry, or
the commit and its word with `land.sh --carried <w>` as the closer, and
stops with exit 3. Nothing is written. A carry therefore never lands on
a base whose last carry is unreviewed, in flight or unrecorded; the
fleet pays one debt before it takes the next, and a debt a peer landed
after the word is seen because the landing reads a validated tip it
fetched itself (HCL-C-28).

### The reservation: one carry in flight, visible to every seat (HCL-C-33)

Before revision 5, between a seat's code push and its `carried` row
nothing another seat could fetch said a carry was in flight: the
`carrying` entry was a local journal file, and seat B could pass the
debt check and stack a second carry on A's unrecorded one. The
reservation is that interval made fleet-visible, and it is taken before
the push, under the ledger's own compare-and-swap.

The row: `carrying actor=<machine+lineage> targets=<goal>
approvedRef=<word opid> reason=open workspace=<sha40> project=<sha40>
expires=<RFC3339> by=human:<name>`. `workspace=` is the projection of
`--tree` (registers.go:35-51) and must equal the word's; `project=` is
the whole tree staged at the row, on the base before the row's own
ledger commit; that commit moves the base by ledger paths alone, so the
landed commit's whole tree differs from `project=` and its projection
does not, which is why every reader compares `workspace=` and the
`carried` row records the landed `project=`. The seat is the opid's
machine segment (04). `expires=` equals the word's expiry (`expires=`
of a terminal row; At plus two hours of a channel row): a reservation
cannot outlive the word it reserves, and one expiry means an expired
reservation is never debt and needs no closing row, exactly as an
expired word needs none (06). The closer: `carrying
actor=<machine+lineage> targets=<goal> approvedRef=<word opid>
reason=abandoned of=<row opid> why=<quoted>`.

Open means: reason `open`, no `carried` row with the same ApprovedRef
on the goal (any outcome: `goal carried` and `goal carry --supersede`
both close it), no closer naming it, and `expires=` not passed at the
reader's now. `landing carry-status` reports `open`, `expired`,
`abandoned` or `closed` by that rule.

Who writes it: `goal carrying` at 05 step 5, a `Publish` whose Mutate
rechecks the word and the debt on the tip it builds on; because the
transaction runs Mutate again on every rebuilt tip (txn.go:473-478,
703-716) and the push is a compare-and-swap, two seats that reserve at
once are serialized: exactly one open reservation exists fleet-wide,
and the other seat's Mutate rejects `carry-debt-unpaid` naming it. On
the same seat a second `goal carrying` for the same word finds the open
row and runs no transaction.

Who closes it: `goal carried` (the `carried` row, any outcome); `goal
carrying --abandon <row opid>`, seat-bound like the word (the caller's
machine equals the row's opid machine), which writes the closer as a
transaction and terminalizes any created `carried` entry for the ref
whose owner is dead, and refuses while that entry's owner is alive ("in
flight; the wrapper is alive"); `goal recover` on the seat, when a
dead `carried` entry's commit is not on origin (06's closer leg); and
the expiry. The wrapper runs `--abandon` itself on every ask after the
row and before the push (05 step 6). Another seat has no release: it
waits for the expiry, at most the word's four-hour ceiling, because the
seat rule of 04 is what makes the count the same on every seat; that
limit is stated under the gaps.

Who counts it: 05 step 9, `goal carry`'s debt line, `goal carrying`'s
Mutate, the health line's `inflight=`, and `goal done` through the word
it belongs to (06).

Fixtures of 08:

- HCL-08-COUNTED, internal/goal: after `carried` the goal's history
  holds the row above with outcome `landed` and BudgetExceptions is one
  higher; a `superseded` row leaves BudgetExceptions unchanged.
- HCL-08-FLEET-LINE, internal/steward: a tree with two landed carries
  today, one open word, one open `carrying` row and one open
  `human-carried` obligation renders `CARRIED today=2 open=1 inflight=1
  debt=1`; the claimed goal's appetite line ends ` carried=2`.
- HCL-08-CARRYING-ROW-BYTES, internal/goal: `RenderHistoryLine` of the
  reservation is exactly `- <at> <opid> carrying actor=<machine+lineage>
  targets=<goal> approvedRef=<word opid> reason=open workspace=<sha40>
  project=<sha40> expires=<RFC3339> by=human:<name>` and parses back to
  the same struct; the closer likewise with `reason=abandoned of=<opid>
  why=<quoted>`; `approvedRef=` on any other verb than `resume`,
  `set-obligation`, `carrying` and `carried` is still a parse problem.
- HCL-08-CARRIED-ROW-BYTES, internal/goal: `RenderHistoryLine` of a
  landed row holds the fourteen fields of the form above in that order
  and parses back; a row missing `judgeDigest` or `ledger` is a parse
  problem naming the field.
- HCL-08-DEBT-CARRYING-ROW-AT-WORD, goal-cli scenario `carry-word`: with
  an open `carrying` row of another seat on the bed's ledger, `goal
  carry` exits 3 naming the row, its seat and its expiry; no row; after
  the bed advances its clock past the expiry the same command writes
  the word.
- HCL-08-WORD-LINES, goal-cli scenario `carry-word`: `goal carry`'s
  stdout holds, in order, the tier line or `risk: unanswered`, the
  rounds-skipped line, the open-carries line, the debt line, the expiry
  and the opid.
- HCL-08-LANDING-LINES-THEN-ACT, land leg `carried-fresh`: the step log
  shows the reservation, ledger tip, judge, O, R, finding and exception
  lines before the `push origin` step name.
- HCL-08-TWO-COMMANDS, land leg `carried-fresh`: the bed's transcript
  holds exactly two commands typed by the bed between the staged set and
  the pushed commit, `goal carry` and `land.sh --carried`.
- HCL-08-CAP-KEY-DEFAULT-ONE, internal/config: an absent key reads 1;
  `0` is refused as malformed; `2` reads 2.
- HCL-08-CAP-LOCAL-REFUSED, internal/config: a `.local` line for the key
  fails `config validate` outside a fixture root with the budget-law text
  and is honored inside one.
- HCL-08-CAP-ASKS-AT-WORD, goal-cli scenario `carry-word`: a second
  `goal carry` on the same seat with one open word exits 3 naming the
  open opid and `--supersede`; no row.
- HCL-08-CAP-ASKS-AT-LANDING, land leg `carried-asks`: two open words
  (the cap raised to 2 for the first, then lowered) yield
  `carry-cap-reached` naming the other word.
- HCL-08-SUPERSEDE-CONSUMES, internal/goal: after `--supersede`, the old
  word's goal holds a `carried` row `approvedRef=<old> reason=superseded
  by=<new>` and the new word's reason ends `supersedes=<old>`; `landing
  carry-status --carried <old>` says `superseded:<new>`.
- HCL-08-DEBT-ASKS-AT-WORD, goal-cli scenario `carry-word`: with an open
  `human-carried` obligation whose commit is on `refs/remotes/origin/main`
  in the bed, `goal carry` exits 3 naming the obligation and both
  closers; no row.
- HCL-08-DEBT-ASKS-AT-LANDING, land leg `carried-asks`: the same
  obligation created after the word yields `carry-debt-unpaid`; no
  commit.
- HCL-08-DEBT-PAID-LANDS, land leg `carried-asks`: after `goal
  accept-risk` for that obligation the next carry lands.
- HCL-08-COUNSELOR-LINE, internal/counselor and land leg `carried-fresh`:
  `records/counselor/carried-landings.jsonl` gains one line whose fields
  equal the commit's trailers and whose id is `cl-<opid>`.
- HCL-08-UNNEEDED-NOT-COUNTED, land leg `carried-asks`: after
  HCL-05-UNNEEDED-ASKS the appetite line's `carried=` count and
  BudgetExceptions are unchanged.

## HCL-CRITIC-09: a critic can review a commit

The deferred review needs a code-critic subject that is a landed commit.
The subject form is `commit:<sha40>`; four seams, all declared (HCL-C-09):

1. scripts/agents/dispatch.sh:1386-1391: for the code-critic role,
   `--reviews commit:<sha40>` skips the implementer-record check, requires
   `git cat-file -t <sha>` to say `commit`, and before the round writes
   `git diff --binary --full-index <sha>^ <sha>` as the round's
   `diff.patch` and `<sha>^{tree}` as `reviewedTree` in the round
   directory, so the critic's subject reader is unchanged from there on.
2. internal/dispatch/claim.go:773-789: `validateClaimReviews` admits the
   form for `code-critic` (a regular expression `^commit:[0-9a-f]{40}$`
   beside `validJobID`); `validateFreshCriticReviewsLatest` (:791-830)
   returns nil for it since no chain is reviewed.
3. internal/dispatch/finding_register.go:749-808:
   `critiqueSubjectForRound` resolves a `commit:` subject to the changed
   paths of `git diff-tree -r --no-commit-id --name-only <sha>^ <sha>`
   and tree `<sha>^{tree}`; `reviewedSubject{ReviewsTarget:
   "commit:<sha40>"}` (:42-52) unites two chains on the same commit as
   two chains on one job subject unite today.
4. internal/dispatch/review_reference.go, the fourth seam:
   `reviewReferenceBinding` (:109-133) admits the form for `code-critic`;
   `StampClaimedReviewReference` (:19-36) returns nil for it, since there
   is no reviewed chain root to point at, and `ReconcileReviewReference`
   (:42-107) refuses it with the words "a commit subject carries no
   chain pointer; the obligation on the goal is its record". The
   obligation's Test citation (07) is the pointer.

Fixtures of 09:

- HCL-09-COMMIT-SUBJECT-DISPATCHES, dispatch-fixtures scenario: a
  code-critic dispatch with `--reviews commit:<sha40>` writes a job
  record whose `reviews` field is the subject and a round directory
  holding `diff.patch` and `reviewedTree`.
- HCL-09-DIFF-IS-PARENT-TO-COMMIT, dispatch-fixtures scenario:
  `diff.patch` equals `git diff --binary --full-index <sha>^ <sha>` byte
  for byte and `reviewedTree` equals `git rev-parse <sha>^{tree}`.
- HCL-09-BAD-FORM-REFUSED, internal/dispatch: `commit:<7 digits>`,
  `commit:<a tree id>` and `commit:<40 zeros>` are each refused by
  `validateClaimReviews` or the dispatcher with the form named.
- HCL-09-REVIEW-REFERENCE-ADMITS, internal/dispatch: the binding for a
  `commit:` subject yields the critique field and no stamp; a reconcile
  is refused with the words above.
- HCL-09-TWO-CHAINS-UNITE, internal/dispatch: two roots on one
  `commit:` subject share one `reviewedSubject`.

## The channel word must be in the human's own text (HCL-C-12)

`answerRequest` appends the wanted token to any authenticated reply that
lacks it (verbs.go:120-124), so an authenticated "no" would bind as a
carry. For a `carry` question, `channel poll` passes an empty `wants` to
`goal.Answer` (poll.go:351), the row's reason is the human's text
verbatim, and the carry consumer (step 5 of the classification) requires
the token contiguous exactly once in that text. `channel ask --kind
carry` (question.go:201-205 gains `carry`) requires `--wants` matching
the token grammar and refuses a budget on the question (`validateQuestion
Budget`, :169-183). The general defect, the appended token on every other
kind, is the channel gateway design's; this page carries the carry's
rule only.

Fixtures of 12:

- HCL-12-NO-DOES-NOT-CARRY, internal/channel and internal/landing: an
  authenticated reply "no" binds as an answer row whose reason is the
  word "no" and nothing else; `landing observe --carried <its opid>` says
  `carry-word-missing`; nothing lands.
- HCL-12-VERBATIM-TOKEN-CARRIES, internal/channel: a reply holding the
  exact token once binds a row whose reason is the reply text and the
  classification reaches step 6.
- HCL-12-KIND-CARRY-REQUIRES-WANTS, cmd/metasystem: `channel ask --kind
  carry` without `--wants`, or with a `--wants` that is not the four-field
  token, or with `--budget`, exits 2 naming the rule.

## The register flip and the entry-point test (HCL-C-11, HCL-C-10)

One chain flips every row marked `Pending: "human-carried-landing"`
(register.go:76-124, 48 rows) in the same change that lands the verbs,
the classification and the wrapper, and writes
`TestHCL03NoPendingAfterSlice2` (register_test.go, beside :71-93). The
test fails on any row whose Pending is non-empty, and it fails if any
row's Override begins `land.sh --carried` while any of the real entry
points is absent: `carry`, `carrying` and `carried` in the goal verb
table of `cmd/metasystem/main.go` (the table `collectGoalVerbs` reads,
register_test.go:248-284); `carry-status` in the landing verb table and
the string `--carried` in `cmd/metasystem/landing_verbs.go`; `--carried
<opid>` in the usage line of `scripts/agents/land.sh` (:9); and a
`--carried)` case in `scripts/agents/commit.sh`'s argument loop
(:72-121). Each is a file read of the source tree from the test's
working directory, the way the test already reads main.go
(register_test.go:53). Because the flip and the entry points land in one
change, no landed tree can say `land.sh --carried` for a verb that does
not exist.

Three rows change their Override instead of losing Pending, because they
are ledger-meaning checks the carry never passes: `goal-item-not-held`
(:83) becomes `goal steal` (Commands 1); `record-not-owned` (:84) becomes
`goal claim of the owning goal` (Commands 1); `ledger-path-not-goal-verb`
(:80) becomes `the owning goal verb` (Commands 1, the words
commit.sh:515 uses). `evaluator-unavailable` (:76) keeps `land.sh
--carried` with the note that the base-tree judge is how it is carried.
The nine ShellRows (:196-206) are rewritten: the four land.sh usage
refusals (:197-200) get Override `the usage line` since a usage error is
not a judgement; the three legacy-branch commit.sh rows (:201-203,
coverage, go-gate, audit) keep `land.sh --carried` and gain the note
`legacy branch, contract off`; the landing-verdict row (:204) keeps
`land.sh --carried`; the Goal-Item postcondition row (:205) becomes
Override `` (a record failure) and the test that requires an Override on
every Shell row is relaxed to allow the empty form with a `Record: true`
field. The fourteen carry ask codes above enter `Rows` as Shape Question
with no Override. New shell prose sites this chain adds (the exit-3 asks
in land.sh and commit.sh: main only, rebase conflict, moving origin,
ledger fetch failed, no judge, superseded, the fresh-word ask) enter
`ShellRows` by hand with their current line cited, and register.go:1
still says no test proves that list complete: that limit stands.

What slice 1 landed of HCL-C-10, read at HEAD: tokens are found inside
literals (register_test.go:16-17 patterns; `TestHCL03EveryCodeRowed`),
the landing set is collected from `wouldRefuse`, `carriageError` and
`knownRefusalCode` (the walk the test implements over internal/landing),
and `chain-full-gate-refused` sits inside `knownRefusalCode`
(promotion.go:121), so no Defect entry was needed. What remains: the
hand-kept ShellRows, whose line numbers are already stale at HEAD (land.sh
usage refusals are at :106, :132, :136 and :9 now) and which this chain
rewrites; the design accepts that a shell row is prose and asks the
implementer to cite the current line in each row.

Fixtures of 11:

- TestHCL03NoPendingAfterSlice2, internal/refusal: as specified above;
  driven negatively by a table that removes each entry point from a copy
  of the source and expects the named failure.
- HCL-11-ENTRY-POINTS-PRESENT, internal/refusal: every `Rows` entry whose
  Override begins `land.sh --carried` has empty Pending, and every
  `carry-*` code of 05 is a Question row with empty Override.

## The chain question is closed (HCL-C-20)

The critic asked which root reviews the carry after revision 1's frozen
output list. The question went to Wido; his ruling of 2026-09-04 ("All
approved") made the carry this goal on a fresh chain with the full file
list declared (the goal's Intent line). This page is that declaration;
the finding is closed by the ruling, not by the design.

## Contract ownership before the first canary (HCL-C-31)

testing.json changes in this chain, before its first canary, so the
proof plan selects the packages this chain changes instead of falling to
the unknown diagnostic (select.go:121-133, 187-190). Following the shape
of the existing surfaces (contract.go:49-58) and groups (:82-99):

- A new surface `human-authority-and-channel`: paths
  `metasystem/internal/humanauthority/**`, `metasystem/internal/channel/**`,
  `metasystem/internal/governance/**`, `metasystem/internal/counselor/**`,
  `metasystem/cmd/metasystem/channel_verbs.go`,
  `metasystem/cmd/metasystem/channel_verbs_test.go`; dependsOn
  `dispatch-goal-mission`; standard `["authority-standard"]`; deep
  `["section/authority-regression-fixtures"]` (the section group exists,
  testing.json:68); critical `["human-authority-integrity"]`.
- A new group `authority-standard`, kind `unit`, adapter `go`, cwd
  `metasystem`, inputs `go.mod`, `go.sum` and the four package globs,
  tools `go`, obligations `["human-authority-integrity"]`, platforms
  `any`, targetMs 8000, packages `internal/humanauthority`,
  `internal/channel`, `internal/governance`, `internal/counselor`, tests
  `"all"` (the shape of `runtime-owner-standard`, testing.json:39).
- `gate-plumbing` (testing.json:11) replaces
  `metasystem/internal/refusal/register.go` with
  `metasystem/internal/refusal/**` and adds a standard group
  `refusal-register-standard` (unit, go, packages `internal/refusal`,
  tests `"all"`, obligations `["gate-integrity"]`, targetMs 5000).
- `dispatch-goal-mission` (testing.json:7) adds the three unowned
  command-layer files this chain edits:
  `metasystem/cmd/metasystem/goalsync_mutations.go`,
  `metasystem/cmd/metasystem/goalsync_mutations_test.go`,
  `metasystem/cmd/metasystem/goalsync_verbs.go`.
- The obligation `human-authority-integrity` has exactly one provider,
  the new group, so the standard set gains it whenever the surface is
  affected (select.go:170-177).
- The new Go fixtures of this page execute, not merely sit in an owned
  package (HCL-C-34). The standard groups that own the packages the
  fixtures land in run fixed lists today: `goal-decision-standard`,
  four names in internal/goal (testing.json:32);
  `landing-command-standard`, two in cmd/metasystem and internal/landing
  (:31); `dispatch-proof-standard`, two in internal/dispatch (:37);
  `config-standard`, two in internal/config and internal/behaviorsurface
  (:40); `policy-canary`, three in internal/testpolicy (:25). Only
  `governed-standard` (internal/steward, :41), `obligationstate-standard`
  (:33) and the deep `goal-full-coverage` (:35, thirty minutes) say
  `all`. The choice is the named list, not `all`: `all` for
  internal/goal is the deep coverage group's thirty minutes and would
  enter every standard plan, and cmd/metasystem `all` is larger still;
  and a named list is self-checking, since a name the packages do not
  define is reported `missing` and the group is incomplete
  (test_go.go:94-99, 428-439), so a fixture renamed or dropped fails the
  plan instead of vanishing from it. The naming rule that makes the
  list mechanical: every fixture id on this page that lives in a Go
  package is the test function `TestHCL<nn><Name>`, the id's words in
  camel case (HCL-05-WRONG-REFUSAL-ASKS is `TestHCL05WrongRefusalAsks`;
  `TestHCL03NoPendingAfterSlice2` keeps its landed name). Three new
  groups carry the lists, each `unit`, adapter `go`, cwd `metasystem`,
  tools `go`, platforms `any`, inputs `go.mod`, `go.sum` and its
  package globs: `carry-goal-standard` (packages `internal/goal`;
  obligations `["carried-landing-goal"]`; targetMs 20000; tests: every
  internal/goal fixture of 02, 04, 06, 07 and 08 by name),
  `carry-landing-standard` (packages `internal/landing`,
  `cmd/metasystem`; obligations `["carried-landing-landing"]`; targetMs
  20000; tests: every fixture of this page whose bed is internal/landing
  or cmd/metasystem, by name; the go adapter expands one named list
  against every package that defines the name, test_go.go:91-102, so
  one list serves both packages) and
  `carry-plumbing-standard` (packages `internal/dispatch`,
  `internal/config`, `internal/testpolicy`; obligations
  `["carried-landing-plumbing"]`; targetMs 10000; tests: the 09, the
  08 cap, the 31 and the 34 fixtures by name). The packages whose
  groups already say `all` (internal/refusal, internal/counselor,
  internal/channel, internal/humanauthority, internal/governance,
  internal/steward) need no list. Each obligation has exactly its one
  group as provider and is added to the `critical` list of the surfaces
  that own the group's packages (`dispatch-goal-mission` for the goal
  and plumbing obligations, `configuration-runtime` and
  `testing-policy` for the plumbing one, and for the landing one every
  owner of a file the group's tests live in, next paragraph),
  so the group enters the standard set whenever one of those surfaces
  is affected (select.go:162-177); each group is also in its owning
  surfaces' `standard` lists.
- Execution by owner (HCL-C-34). cmd/metasystem is owned file by file,
  not as a package (testing.json:5-7; `matchesPath`, select.go:251-258,
  matches an exact path or a `/**` prefix), so the command fixtures of
  this page live in exactly three files with three owners, and in no
  other file: `cmd/metasystem/channel_verbs_test.go` (12) under the new
  `human-authority-and-channel` surface, `cmd/metasystem/
  goalsync_mutations_test.go` (02, 04, 07, 18, 26, 33) under
  `dispatch-goal-mission`, `cmd/metasystem/landing_verbs_test.go` (03,
  05's carry-status) under `proof-and-landing`. No command fixture goes
  in a new `_test.go` file, since a new cmd/metasystem file is owned by
  no surface and sends the plan to the unknown diagnostic
  (select.go:130-133, 187-190). The group stays one and is attached to
  every owner: `carried-landing-landing` is in the `critical` list of
  `proof-and-landing`, `dispatch-goal-mission` and
  `human-authority-and-channel`, and `carry-landing-standard` is in
  each one's `standard` list, so a candidate touching any of the three
  files, or internal/landing, runs every command fixture
  (select.go:162-177; consumers are affected with their providers,
  :134-144, never the reverse, which is why the attachment goes to each
  owner and not to `proof-and-landing` alone). Not chosen: a group per
  owner. That split needs a fourth group and obligation for the one
  channel command file (`authority-standard` says `all` for its four
  packages and cannot take cmd/metasystem, whose `all` is the whole
  command package), and the fixtures are one feature's: a channel verb
  change that breaks the landing's read of the answer row is what the
  landing fixtures exist to catch. One group on several critical lists
  is already this page's pattern for the plumbing obligation. The cost
  is twenty declared seconds when internal/channel alone changes.

A testing.json edit is a protected policy change (protection.go:37-44),
as are the edits to commit.sh and land.sh (:44), so this chain's own
landing runs in deep mode; the goal's recorded gate width is `full`
already (accumulation 2 or higher). The implementer verifies the edited
contract with `metasystem test check` before the first canary and stops
on any path the plan still reports unowned.

Fixtures of 31 and 34:

- HCL-31-CONTRACT-OWNS-FIVE-PACKAGES, internal/testpolicy: a selection
  whose changed paths are one file in each of the five packages and the
  three command-layer files reports zero `no surface owns changed path`
  uncertainties and includes `authority-standard`,
  `refusal-register-standard` and the three carry groups in its
  standard set.
- HCL-34-PLAN-EXECUTES-EVERY-FIXTURE, internal/testpolicy: one row per
  owner path. For each internal package this page adds fixtures to, a
  selection whose changed path is one new file of that package (the
  files list below); for each of the three command files, a selection
  whose changed path is that one file
  (`metasystem/cmd/metasystem/channel_verbs_test.go`,
  `metasystem/cmd/metasystem/goalsync_mutations_test.go`,
  `metasystem/cmd/metasystem/landing_verbs_test.go`). Each yields a
  plan whose standard groups, expanded through `GoTests`
  (contract.go:342-362), name every `TestHCL` function the test scans
  out of that package's `_test.go` files (internal packages) or out of
  that one file (command files), or say `all` for the package; the
  oracle is set inclusion of function names, never surface membership.
  Each scan is a file read of the source tree from the test's working
  directory, the way register_test.go:53 reads main.go. Driven
  negatively: a `TestHCL` function added to a copy of internal/goal's
  sources and not to `carry-goal-standard`, and one added to a copy of
  `goalsync_mutations_test.go` and not to `carry-landing-standard`,
  each fail the test naming the function; a `TestHCL` function in any
  cmd/metasystem `_test.go` file other than the three fails naming the
  file.

## Fixtures of this revision, and where they live (HCL-C-32)

No fixture named on this page exists at HEAD: a search of the tree for
`HCL-0`, `HCL-1`, `HCL-2`, `HCL-3` and `TestHCL03NoPendingAfterSlice2`
finds them only under plans/. Each is a future test the chain writes,
with the oracle stated at its point above, a decidable pass condition
and a two-minute ceiling (the bed scaling of scripts/agents/
fixture-budget.sh applies to the shell beds). A fixture that lives in a
Go package is the function `TestHCL<nn><Name>` (the rule of the
contract section) and is named in its carry group's list. The beds:

- Go tests: internal/goal (02 row bytes and format fence, 04 verbs and
  supersede with the complete consumption predicate, 06 verbs, journal,
  recovery and the confirmed leg, 07 goal package and the changed-why
  refusal, 08 counter, rows and reservation), internal/landing (05
  classification, the match table of step 14 with R's four lists, the
  fence table by owner, carry-status), internal/channel (12),
  internal/dispatch (09 seams 2 to 4), internal/refusal (11),
  internal/config (08 cap key), internal/counselor (07 line, 08 line),
  internal/steward (08 lines), internal/testpolicy (31, 34),
  cmd/metasystem (02 verb wiring, 06 `carrying` forms, owner rule and
  `--abandon`, `--repair-counselor`, 07 command-layer effects, 08 cap
  and debt asks at the word, 12 ask flags).
- scripts/agents/land-fixtures.sh, new legs beside the thirteen
  (:30-33): `carried-fresh` (05 lands without chain, trailers, 06 order
  with its six crash points, 08 lines then act, counselor line),
  `carried-chain-group` (05 chain plus group), `carried-forward` (04
  second word, ledger move keeps word, the rebase posture, the release
  on a second ask, lease held throughout), `carried-crash` (06 the
  intervals through `landing carry-status`, recovery keeps the rebased
  commit and its conflict posture, counselor repaired from the row,
  consumption without a clock, rebuild from commit), `carried-asks` (05
  wrong refusal, two failures one name, the verify error, unneeded,
  main only, no judge, dead live engine, superseded, cap and debt at
  the landing, peer debt, two seats interleaved and the trailer without
  a row, each with two clones), `carried-record-failures` (05 the stop
  list). Each
  leg seeds through `make_leg` (:161), enrolls its fixture engine with
  `arm_receipt_runner` (:466) where a receipt or `test verify` is
  exercised, raises the fixture ledger to format 2 once, and writes the
  word with `goal carry --fixture-human-authority`. The pause seam
  `METASYSTEM_LAND_FIXTURE_PAUSE=<step>` is read by land.sh only when
  `FixtureModeRoot` is true, and the bed kills the paused wrapper by its
  recorded pid.
- scripts/agents/goal-cli-fixtures.sh, new scenarios beside the
  fifteen (:82-83): `carry-word` (02, 04, 08 word lines, cap and debt
  asks at the word including the in-flight row, supersede, transfer and
  the push-before-row refusal, the format raise), `carried-record` (06
  idempotence and the fourteen-field replay refusal, the reservation,
  its abandon and its closers, done refuses, expired is closed, recover
  completes, closes and abandons, the confirmed leg), `carried-discharge`
  (07 both ways, accept-risk skips the register, the line, its replay
  and the changed-why refusal).
- The critic subject (09 seam 1) is a scenario of
  scripts/agents/dispatch-fixtures.sh, the bed that already drives
  `--reviews` for the code-critic role; the commit form is added there.

## Files this design touches

- scripts/agents/land.sh: `--carried` in the usage line, the lease
  re-exec, main only, `goal fetch` and `--ledger-tip`, the carry-status
  branches (superseded, local, origin, ledger, the reservation states),
  carry-forward with its one conflict posture, the reservation and the
  second carry-forward, the release on an ask, the single push and the
  recovery path, the two `goal carrying` forms, `goal carried` with
  `--entry`, `--rebuild-from-commit` and `--repair-counselor`, the
  fixture pause seam, exit 3 asks.
- scripts/agents/commit.sh: `__lease-held` entry from land.sh, carried
  mode, the two-step judge with `--judge` and `--live-failure`, the
  judge-parsed JSON, `test verify --json` as R, the seven trailers, the
  scan and the postcondition, exit 3 asks.
- scripts/agents/dispatch.sh: `--reviews commit:<sha40>` for code-critic.
- scripts/agents/sync-transport.sh: unchanged; named because 06 step 6
  depends on its origin-mirroring rule.
- scripts/agents/land-fixtures.sh, scripts/agents/goal-cli-fixtures.sh,
  scripts/agents/dispatch-fixtures.sh: the legs and scenarios above.
- cmd/metasystem/main.go: `carry`, `carrying`, `carried` in the goal
  verb table; `carry-status` in the landing table.
- cmd/metasystem/goalsync_mutations.go: `runGoalCarry` (with
  `--supersede`, `--transfer`, `--raise-format`), `runGoalCarrying`
  (the reservation; the `--commit` form with `--owner-pid` and the
  judge and list flags; `--abandon`), `runGoalCarried` (with `--entry`,
  `--rebuild-from-commit`, `--repair-counselor`), the `human-carried`
  effects in discharge and accept-risk, `parseSyncFlags` lists.
- cmd/metasystem/goalsync_verbs.go: `goal recover` unchanged in shape;
  named because its policy path now meets a `carried` entry and fetches
  the code remote for it.
- cmd/metasystem/landing_verbs.go: `--carried`, `--project-tree`,
  `--ledger-tip`, `--judge`, `--live-failure`, `carry-status`.
- cmd/metasystem/channel_verbs.go: `--kind carry` with `--wants`.
- cmd/metasystem/*_test.go for the above.
- internal/governance/types.go: the proven outcome constant.
- internal/goal/obligation.go: the re-export.
- internal/goal/file.go: `AuthorityGeneration`, the parse and render
  rules, `approvedRef=` on `carrying` and `carried`, the key-value
  reason of the `carrying` and `carried` rows.
- internal/goal/root.go: `knownLedgerFormats` (`1`, `2`), the render.
- internal/goal/validate.go: the format-2 rule for the proven outcome.
- internal/goal/verbs.go: `Carry` (supersede's complete consumption
  predicate and in-flight rule, the format raise), `carryingRequest`
  (the reservation's rechecks) and its closer, `carriedRequest`,
  `Carried`, the fourteen-field comparison, the consumer list,
  `doneRequest`'s open-carry refusal, `AcceptedRiskDecision`'s discharge
  and why comparison for `human-carried`.
- internal/goal/journal.go: `CreateCarryingEntry` with the ancestor-owner
  rule; `TakeOver`'s descendant leg.
- internal/goal/txn.go: `CompleteEntry`; the `AlreadyApplied` comment.
- internal/goal/recover.go: the `carrying` and `carried` cases of
  `requestForEntry`, the on-origin proof and the closer leg,
  `recoverConfirmedEffect` (the verb switch; the `carried` replay from
  the row).
- internal/goal/*_test.go.
- internal/humanauthority/authority.go: action constant `goal carry`
  and `RecordCarryProof` (a thin wrapper over `recordProof` with
  `ValidFor`).
- internal/channel/question.go, internal/channel/poll.go, and tests.
- internal/landing/carried.go (new: the classification, the fence by
  owner, the match over R, the three debt forms), observe.go
  (`BarCarried`, the five new params, the dispatch into carried.go),
  carry-status with the reservation state, landing tests.
- internal/dispatch/claim.go, review_reference.go, finding_register.go,
  and tests.
- internal/refusal/register.go, register_test.go.
- internal/config/budget.go, validate.go, and tests; metasystem.conf
  (the key line).
- internal/counselor/register.go (`AppendCarriedAcceptedRisk`,
  `CarriedLandingLine`, `AppendCarriedLanding`) and tests.
- internal/steward/health.go and tests.
- internal/testpolicy tests (31, 34).
- testing.json: the surface, the two groups and the path additions of
  HCL-C-31; the three carry groups, their obligations and the critical
  and standard list entries of HCL-C-34.
- docs/orchestration.md and AGENTS.md: the carried landing named beside
  the tiers and the landing forms (the docs ride the closing round).

## One chain, and its budget (HCL-C-11)

The goal's box (plans/goals/human-carried-landing-carry.md:13): one
working day, twelve attempts, 1200 reserved minutes, one active job,
three review rounds; one exception already recorded. Design critique
rounds on this page: up to three, stopping at the first with no material
finding (R-60-m1). Then one implementation chain lands everything in the
files list together: the engine (governance, goal, channel, landing,
config, counselor, steward, refusal, testpolicy), the verbs, the wrapper
(land.sh, commit.sh, dispatch.sh), the three dispatch seams, the
register flip with `TestHCL03NoPendingAfterSlice2`, the testing.json
ownership, and every bed leg and scenario. The docs ride the chain's
closing round. Revision 3's two slices are withdrawn: a first slice that
flipped the register and landed the verbs without the wrapper would have
told every agent to run a command that did not exist, and its test could
not see that.

The budget for one chain, inside the box: the root implementer job is
bounded at `dispatch.cap-max` (120 minutes, metasystem.conf:56); the
estimate is one root round at the cap and two correction rounds (60 and
45 minutes) against the three code-critic rounds (30 minutes each), a
reservation of about 315 of the 1200 minutes and five of the twelve
attempts, one job active at a time, within the working day. The chain
lands through the ordinary chain landing; if it is itself blocked by the
machinery it repairs, the hand path of 2026-09-11 stands until it lands
and is recorded on the goal.

## What the code cannot answer, said plainly

- How `records/counselor/*.jsonl` lines reach origin after the command
  layer appends them (goalsync_mutations.go:350; register.go:56-60): not
  traced here; the carried-landings line uses the same path, whatever it
  is.
- The channel row's actor is the literal `human:wido` (verbs.go:161,
  poll.go:322); the carry records what the channel records. The name's
  source for channel words is the channel gateway design's.
- `go build` in a detached worktree of HEAD is the base judge; whether a
  host without a Go toolchain (an adopted binary checkout) can carry at
  all is answered no: such a host carries only while its live engine
  decides.
- `goal carry` reads `refs/remotes/origin/main` and the accepted ref
  without fetching for the debt and cap checks, and says which it read;
  a stale ref can miss a debt at the word, and the landing's checks on
  the fresh tip `L` are the ones that bind.
- Which seats have rebuilt before the format raise is the human's
  knowledge, not the ledger's: no fleet-visible record of a seat's engine
  commit exists (identity.go:73-79 is local), so the raise is an explicit
  human flag with the consequence printed.
- The lock wait bound (`lockWaitSeconds`, lock.go:39) against the length
  of a carried sequence that builds a base judge: a peer's `lease renew`
  during a long build fails rather than waits; the design accepts that
  as the fixture asserts it, and does not lengthen the bound.
- A dead seat's open reservation blocks every other seat's carry until
  it expires with its word, at most four hours (08): no cross-seat
  release exists, because the seat rule of 04 is what makes the count
  the same on every seat. A human who needs it sooner reaches the
  seat, or lowers the word's expiry next time.
- The accepted-risk replay on chains other than `human-carried` still
  compares `--by` alone (verbs.go:1061-1068); the same gap there is the
  critique register's design, not this page's.
- The reservation's `project=` is the tree staged before the row's own
  ledger commit rebased the candidate (08); the landed tree is on the
  `carried` row, and only `workspace=` binds.

## Revision 3 dispositions of the second critique (chain hcl-design-cc1, round 2)

| Finding id | Evidence at HEAD | Answered by |
| --- | --- | --- |
| HCL-C-02 | file.go:1437-1441 routes non-channel outcomes to the temporary validator; types.go:134-158 admits empty or TEMPORARY_HUMAN_WORD; the name is `--by` with no join to the proof. | 02: `AuthorityOutcomeHumanAuthorityProven`, `AuthorityGeneration` on the row, the parse and render rules, the name as `--by`; HCL-02-TERMINAL-ROW-BYTES. Revised in revision 4 (below). |
| HCL-C-03 | commit.sh:271-278: under the contract the judge is the live `$ms`; :311-317 the proof engine is legacy only; :472-499 `evaluator-unavailable` refuses. | 05: the two-step judge (live, then built from HEAD after carry-forward), `Carried-Judge`, the advisory list and the record-failure list with sites. Revised in revision 4 (below). |
| HCL-C-04 | land.sh:320-323 refuses an empty staged set; :618-641 rebases after the commit and pushes in a loop; the whole tree changes on every ledger move. | 04 and 05: the word names the workspace projection; carry-forward before the commit; rebuild, never amend; single push. Revised in revision 4 (below). |
| HCL-C-05 | verbs.go:1154-1171 archives on `done`; :1000-1003 and :1051-1053 refuse archived goals. | 06: `done` refuses an open carry word; HCL-06-DONE-REFUSES-OPEN-CARRY. Recovery's `expired` case withdrawn in revision 4. |
| HCL-C-06 | verbs.go:1021-1035 keys on Finding and Chain. | 07: Finding `carried:<sha40>`; HCL-07-FULL-ID-IN-FINDING. |
| HCL-C-07 | goalsync_mutations.go:326, 350, 354 need a critic job record and register. | 07: the four command-layer effects (a) to (d) for chain `human-carried`, and the discharge citation check. Completed in revision 4 (below). |
| HCL-C-09 | claim.go:773-789; review_reference.go:233-247 requires an implementer; finding_register.go:749-753; dispatch.sh:1386-1391. | 09: four seams, review_reference.go the fourth; HCL-09-REVIEW-REFERENCE-ADMITS. |
| HCL-C-10 | register_test.go:16-17, 20-49; promotion.go:121 holds `chain-full-gate-refused`. | The register section: what slice 1 landed, what remains (ShellRows by hand, stale lines rewritten). |
| HCL-C-11 | register.go:76-124, 48 pending rows; register_test.go:71-93 admits them. | The register section: the flip, the three ledger-meaning overrides, `TestHCL03NoPendingAfterSlice2`. Revised in revision 4 (below). |
| HCL-C-12 | verbs.go:120-124 appends `wants`; :86-92 then finds it. | The channel section: empty `wants` for `carry` questions, the token in the human's text; HCL-12-NO-DOES-NOT-CARRY. |
| HCL-C-18 | goalsync_mutations.go:108 mints a fresh ulid per command; verbs.go:441-451 compares only the opid; recover.go:146-177 and :244-413 have no `carried`. | 06: AlreadyApplied on ApprovedRef; `carried` rebuilt from intent in `requestForEntry`. Revised in revision 4 (below). |
| HCL-C-19 | land.sh:294 any branch; :618-641 the loop; :644-646 transport as a separate write. | 05 and 06: main only, single push, the ordered six writes, `landing carry-status` and the interval table. Revised in revision 4 (below). |
| HCL-C-20 | plans/goals/human-carried-landing-carry.md:8 (the ruling). | Closed by the ruling; this page is the declaration. |
| Addition 1 | budget.go:168-203, validate.go:521-538: how budget-law keys refuse `.local` and the environment. | 08: `metasystem.budget.carry-open-max`, default 1, per seat by the opid's machine, asks at the word and the landing, `--supersede`; HCL-08-CAP-*. |
| Addition 2 | file.go:528-532: the obligation record; observe.go:776-779: base-tree reads. | 08: unpaid defined, ancestor-of-base check at the word and the landing, the ask with both closers; HCL-08-DEBT-*. |

## Revision 4 record: the first read of this page (Sol, hcl-crit1-20260911)

Nineteen findings, all material, all accepted by the coordinator
(artifacts/agents/hcl-context/critique-r1.md, dispositions at its end).
Everything revision 3 settled outside these nineteen stands: point 01
and point 03 on the old page; the two proofs; the word as a workspace
tree; the one named refusal or group; the review deferred never deleted;
the cap and the debt as asks; the four seams of the commit subject.

| Finding | Evidence at HEAD | What moved in revision 4 |
| --- | --- | --- |
| HCL-C-21 (critical) | Revision 3 step 12 required M exact only for group words; the brief (design-brief.md:154-156) requires one named refusal, never a blanket. | 05 step 14: a code word hits only when O.Code equals it AND M is empty; a group word only when M equals `{G}` AND O passes or is one of the four receipt-refusal codes; anything else is `carry-refusal-mismatch`. HCL-21-TWO-FAILURES-ONE-NAME, HCL-21-EACH-NAMED-ALONE-LANDS. |
| HCL-C-22 | Revision 3 consumed the word as `unneeded` and landed; the old page's rule (human-carried-landing-design.md:56-64) makes a word for something that no longer exists a question back. | 05 step 14: `carry-unneeded`, exit 3, no commit, no row, the word stays open. HCL-05-UNNEEDED-ASKS, HCL-08-UNNEEDED-NOT-COUNTED. |
| HCL-C-03 | observe.go:43-55 has no live-failure input; commit.sh:477-490 parses with `$ms` even after fallback; :308-317 makes the candidate-built engine the policy owner on the legacy branch. | 05: `--judge base --live-failure <code|exit=n>` as a recorded input; the judge parses its own output with `"$judge" json get` (main.go:429-436) in the fixed `Observation` shape (observe.go:59-72); step 11 refuses `carry-base-judge-blind` when the candidate changes the named policy and wire-format files; `Carried-Judge` carries the live failure and the postcondition checks it. HCL-03-LIVE-FAILURE-STAMPED, HCL-03-BASE-JUDGE-BLIND-ASKS. |
| HCL-C-04 | land.sh:537-590 stages and commits, commit.sh:31-54 alone runs held, verbs.go:462-481 holds the lock only around that child. | 05: land.sh re-executes itself under `lease run-held` before its first step and calls `commit.sh __lease-held <epoch>`; the whole carried sequence is one epoch; lock.go:39-49 names why nesting is not used. HCL-04-LEASE-HELD-THROUGHOUT. |
| HCL-C-19 | Revision 3's carry-forward step 2 reset the carried commit away and then pushed. | 05 recovery path: rebase the carried commit, re-run the tree and trailer postconditions on it, push it; never reset or restamp there. HCL-19-RECOVERY-KEEPS-REBASED-COMMIT. |
| HCL-C-18 | recover.go:43-72 walks journal entries only; a crash after the push and before `goal carried` left no entry. | 06 step 4: `goal carrying` writes a `created` entry with Intent.Verb `carried` and every field before the push, owned by the live wrapper (`--owner-pid`, a proven ancestor); `goal carried --entry` consumes it through `CompleteEntry`; recovery's `carried` case proves the commit is on origin before writing and otherwise abandons with the word open; `--rebuild-from-commit` covers a lost journal. HCL-18-*, HCL-06-CARRIED-REBUILDS-FROM-COMMIT. |
| HCL-C-26 | goalsync_mutations.go:341-362 appends counselor state after publication; revision 3's ledger branch ran transport only. | 06 and 08: the counselor line is the `carried` transaction's `AfterConfirmed` (txn.go:488-491, 733-743); `landing carry-status` reports `counselor: written|missing`; the `ledger:` branch replays and repairs it. HCL-26-COUNSELOR-REPAIRED. |
| HCL-C-23 | Revision 3's carry-status had `superseded:<opid>` and land.sh had no branch for it. | 06: a terminal no-landing state; land.sh prints the superseding opid and exits 3; no commit, row or transport. HCL-23-SUPERSEDED-LANDS-NOTHING. |
| HCL-C-24 | `git log --since=<At>` reads committer timestamps. | 06 consumption: the anchor is the ledger commit carrying `Goal-Transaction: <opid>` (976cb50bb's shape; `TrailerPresent`, txn.go:379-390); the range `A..<code tip>` is scanned for the exact `Carry:` trailer; no clock. HCL-24-CONSUMPTION-WITHOUT-CLOCK. |
| HCL-C-25 | txn.go:728-743 turns AlreadyApplied into confirmed success; revision 3 compared only the ApprovedRef. | 06: the replay compares commit, workspace, project tree, past, battery, judge, outcome and by; a difference is a named rejection. HCL-25-REPLAY-MISMATCH-REFUSED. |
| HCL-C-02 | authority.go:123-145 binds fixture authority in process only; :700-740 makes the proof file post-act audit evidence; revision 3 claimed a join and let generation zero mean "fixture". | 02: production readers refuse generation zero unless `FixtureModeRoot` (fixtureauth.go:291-293), code `carry-word-unproven`; the join claim is withdrawn and the proof file is named as audit evidence. HCL-02-GENERATION-ZERO-REFUSED-IN-PRODUCTION. |
| HCL-C-27 | file.go:1436-1441 and types.go:134-140 make the new outcome a parse problem for every engine at HEAD; no version or fence existed. | 02: the ledger `FormatVersion` (root.go:139-143) is the fence: `goal carry --raise-format` writes 2 with the first row; old engines refuse to advance, keep the stale accepted tree, reject writes; the arm-before-write alternative is declined because engine commits are local records (identity.go:73-79). HCL-27-OLD-READER-REFUSES-FORMAT-TWO, HCL-27-FORMAT-RAISE-ASKS-THEN-WRITES. |
| HCL-C-28 (critical) | project.go:48-66 with fetchFirst false reads the accepted ref offline; land.sh:432-437 fetches only the code branch. | 05 step 1: `goal fetch` (goalsync_verbs.go:395-407, `boundedFetchAdvance` project.go:123-163) once after the code fetch; its tip `L` is passed as `--ledger-tip` and every check reads that one tree; `Carried-Ledger: <L>` is stamped. HCL-28-PEER-DEBT-SEEN. |
| HCL-C-29 | Revision 3 attributed the word to the opid's machine but let any actor use it. | 04 and 05 step 4a: the seat is the opid's machine segment (file.go:1513-1532; poll.go:227-231); the landing actor's machine must equal it, else `carry-seat-mismatch`; the transfer is `goal carry --supersede <opid> --transfer` on the new seat. HCL-29-SEAT-MISMATCH-ASKS, HCL-29-SUPERSEDE-TRANSFERS-SEAT. |
| HCL-C-30 | Revision 3's `--supersede` named no target preconditions. | 04: inside the Mutate callback (txn.go:473-478) the target must be a carry word of the same seat (or `--transfer`), unexpired, unconsumed; a target that changed before publication rejects with its current state; the transaction writes the `superseded` row on the target's goal. HCL-30-SUPERSEDE-PRECONDITIONS, HCL-30-SUPERSEDE-TARGET-MOVED. |
| HCL-C-07 | register.go:26-46 requires title, reason, evidence facts, class and review link; revision 3 gave only a commit citation. | 07: the field table with a deterministic source for every field of sources.go:48-76, and the two-level idempotent replay (verbs.go:1061-1068; register.go:70-81). HCL-07-ACCEPTED-RISK-LINE-COMPLETE, HCL-07-ACCEPTED-RISK-REPLAY-IDEMPOTENT. |
| HCL-C-11 | register_test.go:248-284 reads only the goal verb table; revision 3 put the flip in slice 2a and the wrapper in 2b. | One chain; the test checks the goal verb table, `landing observe --carried` and `carry-status`, the usage line of land.sh and commit.sh's carried case; the budget is restated for one chain. TestHCL03NoPendingAfterSlice2, HCL-11-ENTRY-POINTS-PRESENT. |
| HCL-C-31 | testing.json:4-22 owns none of the five packages and only register.go of internal/refusal; select.go:119-190 sends unowned paths to the unknown fallback. | The contract-ownership section: a new surface and group, `internal/refusal/**` under gate-plumbing with its own group, the three command-layer files under dispatch-goal-mission, all in this chain before its first canary. HCL-31-CONTRACT-OWNS-FIVE-PACKAGES. |
| HCL-C-32 | No HCL identifier exists outside plans/; revision 3 called the fixtures existing and HCL-06-ORDER named four writes. | Every fixture is listed at its point as a future test with its bed and oracle; the fixtures section says none exists; HCL-06-ORDER is the six-step transaction with five crash points. |

## Revision 5 record: the second read of this page (Sol, hcl-crit2-20260911)

Nine findings, all material, two critical, all accepted by the
coordinator (artifacts/agents/hcl-context/critique-r2.md, dispositions
at its end; the fold brief restated them as decisions). Everything
revision 4 settled outside these nine stands: points 01 and 03 on the
old page; the two proofs; the word as a workspace tree; one named
refusal or group per word; the review deferred never deleted; the cap
and the debt as asks; one chain; the four seams; the format fence; the
seat binding and `--transfer`. Two consequences of the nine are
recorded here so nothing moves unnamed: the `unverified` battery value
is withdrawn from the trailer, the row, the line and the class table,
because the match of step 14 lands no unverified battery (HCL-C-21);
and the ask codes are fifteen, `carry-battery-unverified` being the
verify error's own name.

| Finding | Evidence at HEAD | What moved in revision 5 |
| --- | --- | --- |
| HCL-C-33 (critical) | Revision 4's `carrying` entry was a local journal file (06 step 4) and debt was an open obligation alone (08); nothing fetchable said a carry was in flight between the push and the `carried` row. The ledger transaction runs Mutate on every rebuilt tip and pushes by compare-and-swap (txn.go:473-478, 703-716), and its capture leaves `refs/remotes/origin/main` untouched (:120-131). | 05 step 5, the reservation: `goal carrying` publishes a `carrying` row (word opid, workspace, staged tree, seat by opid, expiry equal to the word's) as a goal transaction before any code is pushed, with the word and the debt rechecked in its Mutate, so two seats serialize under the CAS; 05 step 6 fetches and carries forward again over the row's own ledger commit; 08's debt has three forms (reviewed late, in flight, landed without a row) and a reservation subsection with the open rule, the closers (`goal carried`, `goal carrying --abandon` seat-bound, `goal recover`'s closer leg, expiry) and who counts it; `landing carry-status` reports the reservation; the `--commit` form of `goal carrying` is the local intent entry and requires the open row. HCL-33-TWO-SEATS-INTERLEAVE, HCL-33-TRAILER-WITHOUT-ROW-IS-DEBT, HCL-33-RESERVATION-ROW, HCL-33-RESERVATION-CAS, HCL-33-RESERVATION-RECHECKS-WORD, HCL-33-ABANDON-CLOSES, HCL-33-EXPIRED-RESERVATION-NOT-DEBT, HCL-33-CARRIED-CLOSES-RESERVATION, HCL-08-CARRYING-ROW-BYTES, HCL-08-DEBT-CARRYING-ROW-AT-WORD, HCL-32-LEDGER-MOVE-RELEASES-ON-ASK; HCL-06-ORDER has six crash points. |
| HCL-C-21 (critical) | Revision 4's step 13 reduced R to M and set M empty on a verify error; `RecomputeDelivery` clears `Sufficient` on uncovered obligations and discrepancies with both group lists empty (test_result.go:201-242). | 05 steps 13 and 14: R is the `TestResult` with its five-field `Delivery`; a code word hits only with `Sufficient` true; a group word only when M is `{G}` and U and D are empty; no R is `carry-battery-unverified` and no word carries it; the mismatch ask names O.Code, M, U, D. The `unverified` battery value is withdrawn everywhere. HCL-21-VERIFY-ERROR-ASKS, HCL-21-UNCOVERED-OBLIGATION-ASKS, HCL-21-DISCREPANCY-ASKS, HCL-21-CODE-WORD-NEEDS-SUFFICIENT. |
| HCL-C-30 | Revision 4's supersede checked only a `carried` row; 06 defined consumption as row or trailer and named the push-before-row interval. The capture is the freshest read of `refs/heads/main`, which carries ledger and code alike in remote mode (txn.go:50, 103-131); single-machine mode keeps the ledger on its own branch (:35). | 04: inside Mutate the target must be unconsumed by the complete predicate (no `carried` row, no superseding `carry` row, no `Carry:` trailer in `A..<code tip>`) and not in flight (no open `carrying` row); the code tip scanned is the captured tip in remote mode and `refs/heads/main` in single-machine mode, and the rejection names it; a trailer without a row rejects `consumed on origin by <sha>` and sends the human to `land.sh --carried`. HCL-30-SUPERSEDE-PUSH-BEFORE-ROW, HCL-30-SUPERSEDE-SCANS-CAPTURED-TIP, HCL-30-SUPERSEDE-IN-FLIGHT. |
| HCL-C-26 | recover.go:75-78 and 93-96 run `recoverSplitConfirmedEffect`, a no-op for every verb but `split` (:416-424); revision 4's ledger branch ran `goal carried --entry` with a fresh opid, which `--entry` cannot do. | 06: `recoverConfirmedEffect`, a verb switch whose `carried` case reads the `carried` row from the tip and appends the counselor line through `counselor.CarriedLandingLine(row)`, the one function the live hook and the repair verb also call; the ledger branch of `landing carry-status` runs `goal carried --repair-counselor --ref <opid>`, a read of the row and no transaction; `goal recover` owns the crash between publication and the hook. HCL-26-CRASH-BEFORE-HOOK-RECOVERS, HCL-26-REPAIR-FROM-ROW-ALONE; HCL-26-COUNSELOR-REPAIRED names the verb. |
| HCL-C-25 | Revision 4's row lacked missing, failing, the judge's tree and digest; the replay compared eight fields; the line carried values the row did not. | 08: the `carried` row carries fourteen fields (commit, workspace, project, past, battery, missing, failing, judge, judgeTree, judgeDigest, liveFailure, ledger, outcome, by) parsed as key-value fields like `NormApproval` (file.go:831-848); 06's replay compares all fourteen and names the field; the intent entry, `--rebuild-from-commit` and the counselor line carry the same set. HCL-25-REPLAY-MISMATCH-REFUSED drives fourteen rows; HCL-08-CARRIED-ROW-BYTES. |
| HCL-C-03 | Revision 4's exact list omitted receipt.go (`TestReceipt`, receipt.go:33-51), the new carried.go and metasystem.conf. | 05 step 11: the fence is by owner: every path under internal/landing, internal/goal, internal/proofrun, internal/testpolicy, internal/behaviorsurface, internal/config and internal/refusal (new files included), plus metasystem.conf, testing.json and the three observer policy inputs. HCL-03-BASE-JUDGE-BLIND-ASKS is a table with one row per owner and a new file. |
| HCL-C-34 | testing.json:31, 32, 37, 38 and 40 run fixed test lists in the packages revision 4 assigned new fixtures to; a named test the packages lack is `missing` (test_go.go:94-99, 428-439); `all` for internal/goal is the thirty-minute deep group (:35). | The contract section: the named list, not `all`, with the reason; the naming rule `TestHCL<nn><Name>`; three groups (`carry-goal-standard`, `carry-landing-standard`, `carry-plumbing-standard`) with one obligation each on the owning surfaces' critical lists. HCL-34-PLAN-EXECUTES-EVERY-FIXTURE asserts function-name inclusion, driven negatively; HCL-31-CONTRACT-OWNS-FIVE-PACKAGES names the groups. |
| HCL-C-07 | `AcceptedRiskDecision` compares By alone (verbs.go:1061-1068) and stores `--why` as the history row's Reason (:1070-1071). | 07: for chain `human-carried` the replay finds the history row by the record's Opid and compares `by` and `why` byte for byte; a changed why is refused naming both; an unfindable row is refused; the other chains are unchanged and the gap is listed. HCL-07-CHANGED-WHY-REFUSED; HCL-07-ACCEPTED-RISK-REPLAY-IDEMPOTENT names both inputs. |
| HCL-C-32 | Revision 4's step 3 said abort restores HEAD to the wip; HCL-05-REBASE-ASKS said HEAD at the wip's parent with the candidate staged. Staging refuses unstaged and untracked paths (land.sh:315-333). | 05 step 3: one posture, `git rebase --abort` then `git reset --soft "$base"`: HEAD at the wip's parent, the index the candidate as staged, the working tree equal to the index, nothing untracked, no rebase directory, the wip unreferenced; the recovery path's conflict leaves HEAD at the carried commit. HCL-05-REBASE-ASKS asserts all five facts; HCL-32-RECOVERY-CONFLICT-KEEPS-COMMIT. |

## Revision 6 record: the third read of this page (Sol, hcl-crit3-20260911), the last fold

Four findings, all material, one critical, all re-opened from earlier
reads, all accepted by the coordinator
(artifacts/agents/hcl-context/critique-r3.md, dispositions at its end;
the fold brief restated them as four decisions). The goal's three
design reads are spent: this fold gets no fourth read, the page is built
as it stands, and the implementation's closing read verifies these four
as named checks. Everything revision 5 settled outside these four
stands. Two consequences are recorded here so nothing moves unnamed:
`carry-landing-standard`'s test list is stated by bed (every fixture
whose bed is internal/landing or cmd/metasystem) instead of by point
number, because revision 5's enumeration omitted the 04, 18 and 26
command fixtures and a list by bed is what the owner mapping needs; and
`done`'s refusal of a trailer-without-row word says `expired or not`,
since expiry is the case the critical finding raised and 06 must read
the same two facts as 08's third form.

| Finding | Evidence at HEAD | What moved in revision 6 |
| --- | --- | --- |
| HCL-C-33 (critical) | Revision 5 defined an open word as unconsumed and unexpired (08) and made the third debt form scan open words alone (05 step 9, 08); the trailer it looked for is what consumes the word (06), so the arm could never find its subject, and after an abandoned or expired reservation seat B saw no debt. `TreeGoals` carries live and done goals (validate.go:27-32); the trailer walk is `TrailerPresent`'s (txn.go:379-390). | 05 step 9 and 08's third form: a landed-but-unrecorded carry is a `Carry: <w>` trailer in `w`'s anchored range for any proven carry word `w` on any live or done goal with no `carried` row (any outcome) on any live or done goal; the arm reads those two facts and no state of `w` (expiry, reservation state, the cap's open-word predicate are not consulted); the scan runs once per row-less word, and the code tip per caller is named. 08's open-word definition is restated as the cap's predicate that the debt never reads. 06's `done` rule says `expired or not`. HCL-33-TRAILER-WITHOUT-ROW-IS-DEBT has two legs, abandoned and expired after the push, B refused `carry-debt-unpaid` naming A's word in both, A's rerun completing the record under an expired word; HCL-33-EXPIRED-RESERVATION-NOT-DEBT distinguishes the in-flight form from the third. |
| HCL-C-25 | `Intent.Targets` is part of the complete normalized intent (journal.go:57-66); 06 step 5 stores the goal there; the `carried` row's `targets=` is `HistoryLine.Targets` (file.go:298); revision 5's replay compared fourteen reason fields and never the goal, so a row on another goal with the same ref was `AlreadyApplied`. | 06's replay: before any field comparison the intent's one target must equal the goal the row is on; a row on another goal is refused `goal differs` naming both goals, recorded as a rejection, nothing written. HCL-25-REPLAY-MISMATCH-REFUSED drives the target case first, then the fourteen fields. |
| HCL-C-03 | The parser imports governance (file.go:19) and routes every non-channel outcome through its recorded validation (:1436-1441; types.go:91-101, 108-115, 134-158); the goal package compares `humanauthority.OutcomeVerifiedChannel` (approval.go:387, 398; norm.go:114) and 02 records `Proof.TerminalGeneration` (authority.go:79) on the row; step 4 admits generation zero only under `fixtureauth.FixtureModeRoot` (fixtureauth.go:291); revision 5's fence had none of the three. | 05 step 11: the fence gains `internal/governance/`, `internal/humanauthority/` and `internal/fixtureauth/`, named as the three compiled owners of the authority wire the ledger grammar reads, with the rule that every package the goal package imports for an authority fact is fenced. HCL-03-BASE-JUDGE-BLIND-ASKS gains a row for `governance/types.go`, `humanauthority/authority.go` and `fixtureauth/fixtureauth.go`. |
| HCL-C-34 | testing.json:5-7 owns cmd/metasystem file by file (`matchesPath`, select.go:251-258); revision 5 put every command fixture in `carry-landing-standard` and attached its obligation to `proof-and-landing` alone; selection adds groups only from affected surfaces and their critical obligations' providers (select.go:162-177), and consumers are affected with providers, never the reverse (:134-144), so a change to a channel or goalsync command file ran none of them. The go adapter expands one named list per defining package (test_go.go:91-102). | The contract section: the command fixtures live in exactly three files with three owners (channel_verbs_test.go, goalsync_mutations_test.go, landing_verbs_test.go) and in no new file; the group stays one and `carried-landing-landing` enters the critical lists of `proof-and-landing`, `dispatch-goal-mission` and `human-authority-and-channel`, with the group in each standard list; the split alternative is named and declined with the reason. HCL-34-PLAN-EXECUTES-EVERY-FIXTURE has one row per owner path, the three command files as exact paths, and two more negative drives. |

## Coordinator's notes at landing (m1d, 2026-09-11)

Three amendments decided in the implementation folds, binding on the page
as built (the reads: records/misc/human-carried-landing-carry-code-read-r1.md
to -r6.md; the folds: plans/human-carried-landing-carry-build2-fold-r1-brief.md,
build3-fold-r2 to -r6):

- The stop list of HCL-LANDING-05 ("an empty staging set is a stop in every
  mode") excludes the rerun states of `land.sh --carried`: `local:<sha>`,
  `origin:<sha>`, `ledger:` and the counselor repair read `landing
  carry-status` and take their branch before staging; only a fresh word
  stages (HCL-C-35).
- Single-machine mode is out of scope by name: `goal carry`, `goal carrying`
  and the carried classification ask `carry-remote-required` when the
  checkout has no code remote; the anchor rule of HCL-TRANSACTION-06 holds
  for remote mode (HCL-C-40).
- `goal carry` speaks its word lines after its transaction, because the
  word's opid is minted inside it; the landing's eight lines are spoken
  before the push as HCL-COUNTER-08 says (HCL-C-44).

Page fixtures recorded as follow-ups on the goal, by finding id: HCL-C-58
(the dead-live-judge and no-judge legs), HCL-C-59 (the ledger repair leg,
the goal recover tests), HCL-C-60 (abandon from another seat, the
reservation's compare-and-swap), HCL-C-61 (the forward-path legs, the cap
and unneeded asks at the landing, main-only, the record-failure stop list,
the seven-step order, two commands), HCL-C-64 (weaker bed assertions),
HCL-C-70 to -73, HCL-C-78, HCL-C-81 to -85 (recorded, low).
