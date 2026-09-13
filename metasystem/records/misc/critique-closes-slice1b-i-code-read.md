# Independent code read: critique-closes part 1b, unit 1b-i

Reviewer: Opus 5, independent of the builder. Worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccf-s1b/metasystem`.
Reviewed change: `git diff` at HEAD `24c367f8` (finding_register.go, mirror.go,
reads_refused.go, refusal/register.go) plus untracked read_admission.go,
read_admission_test.go, reads_refused_test.go, mirror_read_refusals_test.go.
Measured size: 117 tracked plus 1,321 untracked = 1,438 changed lines.
Spec: `plans/critique-closes-on-folded-proof-design.md` section 9c, D7 to D9,
rows B-1 to B-4 and B-7. Brief: `artifacts/reports/codex-ccf-slice1b-brief.md`.

Unit 1b-ii (command, shell, fixtures, CLI tests) is absent by design and is not
counted as a finding. I did not run the seat's build, vet, race or fast-gate
runs. Experiments below ran in a throwaway copy of the module under
`/private/tmp/.../scratchpad/ms`; no repository file was edited.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | `cleanReadsForRoot` treats ABSENT round evidence as corruption, so once a prior clean critic chain in the same role and scope has been evidence-collected, every later read in that scope errors out, including a legitimate fresh read of a completely different subject. D7 requires conservative compatibility, not a hard error. | `internal/dispatch/read_admission.go:327-333` (`!present` on `ReadRoundSubject` returns an error), `:369-375`, `:376-380`; the collector removes the whole payload while keeping job records (`internal/evidence/gc.go:123`, `internal/evidence/gc.go:183`). Probe: clean round-1 fold, `chainClosed: true`, payload removed, then `CritiqueReadAdmission` for a CHANGED subject returns `clean read root critic round 1 does not match its persisted subject` (history present) and `... recorded subject <d1> is missing` (pre-history). Both variants failed. |
| F-2 | high | yes | The same error surface is wired into `CritiqueRegisterAdvance`, which is live today, so a pre-history root whose last fold was a cancelled round can no longer fold at all. That is exactly the deadlock the cancelled-fold path exists to prevent (`internal/dispatch/finding_register.go:86-90`). | `internal/dispatch/finding_register.go:121-124` aborts the fold on any `cleanReadsForRoot` error. The cancelled fold never clears `findingRegisterSubjectDigest` (the `delete` at `:161` is only in the completed branch), so a pre-history record can carry `register=[] round=2 digest=<round 1 digest>`. Probes: round 3 fold refused with `clean read root critic round 2 subject <d2> does not match folded digest <d1>`; with a cancelled round that has no subject.json, `clean read root critic round 2 recorded subject <d1> is missing`; with round 1's `return.json` pruned, `clean read root critic round 1 return is unreadable`. All three wedge the fold; before this change the fold never read older-round artifacts. |
| F-3 | medium | yes | `highestCriticMember` is called for EVERY scoped root in the redundant pass, before any redundancy is found, so one unrelated sibling root in the same scope with a malformed member round (or a member whose role differs) errors the whole admission and blocks a legitimate fresh read. D8 scopes that strictness to rule 4 (the concurrent check on other roots), not to the redundant pass. | `internal/dispatch/read_admission.go:88-91` (unconditional call), `:399-402` (malformed round errors), `:407-409` (role mismatch errors). Probe: a `code-critic` sibling root on the same implementer, never folded, no equal subject, with member `sibling-r2` carrying `"round": "two"` produces `critic root sibling has member sibling-r2 with malformed round state` for an otherwise clean fresh read. A `warden` follow-up member on a `code-critic` root produces the role variant. |
| F-4 | medium | no | On every error return the result still carries `Decision: "ADMITTED"`, so a 1b-ii caller that writes the typed result and inspects only `Decision` would launch a read the owner refused. F-1 and F-3 make error-without-refusal a routine outcome, not a rare one. | `internal/dispatch/read_admission.go:52` presets the decision and nothing resets it on the error paths at `:70`, `:73`, `:76`, `:86`, `:90`, `:161`, `:165`, `:172`, `:178`. Probe output: `ADMISSION = {Decision:ADMITTED ...}, err = critic root sibling has member sibling-r2 with malformed round state`. |
| F-5 | low | no | The child-member assertion in `TestMirrorIncludesReadRefusals` is tautological: `mirrorLand` copies the existing manifest entries forward, so `reads-refused.jsonl` is present in the child's manifest whether or not `mirrorSources` adds it for that member. The brief's "add the source for every mirrored member" is therefore unproven by test (the code is nonetheless correct). | `internal/dispatch/mirror_read_refusals_test.go:63-69`; `internal/dispatch/mirror.go:236-239` seeds `files` from the prior manifest. The test's earlier assertions (`:23-30`, `:41-53`, `:71-78`) are genuinely discriminating. |
| F-6 | low | no | `cleanReadCanClose` runs a full `CloseCheck` (chain walk, manifest read, per-file sha256 of implementer diffs and the whole landing evidence tree) for every redundant candidate, while holding the repo-wide finding-register flock, only to pick diagnostic wording. | `internal/dispatch/read_admission.go:105` and `:450`; `internal/dispatch/close.go:15` onward; the lock is a single repo-wide flock (`internal/dispatch/finding_register.go:776-783`). No deadlock (CloseCheck takes no locks), but every fold and every other admission in the checkout waits behind it. |
| F-7 | low | no | The `round` argument (the proposed critic read round) is validated and then never used: it is not in the result, not in the event, and not in the diagnostic. A refused read leaves no record of which round was refused. | `internal/dispatch/read_admission.go:56-58` is its only use; the event id at `:129` and `result.Round` at `:122` both use the PRIOR read's round. Defensible under D9's ambiguous `<read-round>`, but worth confirming with the design owner before 1b-ii. |
| F-8 | low | no | Refusal timestamps use `time.Now().UTC().Format(time.RFC3339Nano)` while the package's own clock helper `nowISO()` emits second precision. Two time formats now coexist in dispatch evidence. Parsing is tolerant, so nothing breaks. | `internal/dispatch/read_admission.go:133`, `internal/dispatch/reads_refused.go:130`, versus `internal/dispatch/record.go:726`. |
| F-9 | low | no | The builder's result file claims `memory/receipts.log` among the unit's changed records and reports 1,499 changed lines including it. In this worktree `memory/receipts.log` is unmodified against HEAD and holds no row for this goal, and the expected-red paragraph names only one of the four probe tests as failing. | `git diff --stat HEAD -- memory/` is empty; `grep -c critique-closes-on-folded-proof memory/receipts.log` is 0; `artifacts/reports/codex-ccf-slice1b-result.md` change inventory and red/green paragraph. The claim is harmless for the diff (artifacts/ is gitignored) but the report should not be read as proof of red. |

## What I checked and found correct

**D7 exactly.** The append fires only inside the completed branch, only when the
persisted subject is present AND `ReturnBindsSubject` holds AND the register
resulting from the fold is clean (`finding_register.go:205-216`). Cancelled and
failed folds never set `completedSubjectPresent`, and an unbound return sets
`completedSubjectBound` false, so none of them can append. `readsubject.CleanRegister`
counts only `resolved/withdrawn` as clean, so accepted-risk, deferred and
out-of-scope entries block the observation (`internal/readsubject/closure.go:88-96`).
A retry of an already folded round returns `unchanged` before any history work
(`finding_register.go:112-115`), so a duplicate fold neither rewrites nor
backfills. `appendCleanRead` refuses a conflicting subject for the same round
and is idempotent for an equal one (`read_admission.go:462-475`). The pre-history
derivation reads only TODAY's folded round, never guesses an older one, and
requires register cleanliness, a present `findingRegisterSubjectDigest` equal to
the persisted subject's digest, a completed member of the right role, and a
return that binds. Nothing in the tree reads `cleanReadRounds` except the fold
and admission; `CloseCheck` and `readsubject.ReadClosedClosure` do not, and
`cleanReadCanClose` only selects wording. `dedicatedMetadataFields` blocks
generic `RecordCAS` injection (`finding_register.go:32`, `record.go:543`), and
the test proves it.

**Atomicity.** The register, the folded round, the subject digest and
`cleanReadRounds` are published in the one `writeRecord` call at
`finding_register.go:222`, inside `withRecordLock`, through
`wiredoc.Render` plus `atomicWriteText`. A crash cannot leave the history
disagreeing with the register or the round. `wiredoc.FromRaw` is a permissive
encoder, so the new field survives the round trip. I found no path that writes
the history separately from the fold.

**Subject identity.** `ReadSubject.Equal` and `Digest` are the same identity the
rest of the system uses, and provenance (`reviewedMember`, `reviewedCommit`,
`parent`) is excluded from both (`internal/readsubject/subject.go:37-74`). The
live scope key `state.chainRoot(root["reviews"])` matches how
`liveReadSubject` derives `ImplementerRoot` (`read_subject_compute.go:130`).
The design scope key `root["design"]` is the normalized workspace-relative
slash path written by `designBlobBinding` (`build.go:636`), the same
normalization `designReadSubject` applies (`read_subject_compute.go`), so the
comparison is apples to apples. Commit subjects return before the lock and are
exempt from both checks; malformed and role-mismatched commit subjects still
error.

**Admission rules.** Role filtering prevents a warden read from consuming the
code critic merge requires. The redundant check includes the requesting root, so
a follow-up over an unchanged subject refuses. The concurrent check is live-only,
excludes the requesting root, skips cancelled and folded rounds, treats
completed-but-unfolded and failed-unfolded as outstanding, and admits when the
latest round's subject is absent (the acknowledged 9b.1 F-1 race). Candidate
ordering is deterministic (closeable first, then root id). The redundant
diagnostic never promises closure when a later round exists or when the root
cannot close; the concurrent diagnostic offers waiting before cancelling. I could
not find a way for a genuinely redundant equal read to pass, other than the
declared fresh-dispatch race.

**Durable refusal record.** The path is `artifacts/agents/<prior critic
root>/reads-refused.jsonl`, never under the implementer or the proposed root
(`read_admission.go:135`). The append runs only inside the repo-wide
finding-register flock, revalidates every existing line, and republishes the
whole file through `atomicfile.WriteText` with `repoRoot` as the anchor, so two
refusals cannot interleave or be lost. The three publication outcomes follow the
atomicfile contract exactly: a pre-publication error preserves the old bytes and
refuses with `EventRecorded: false`; directory-sync doubt keeps the published
event and the same id with `EventDurable: false` and still refuses; an equal id
appends nothing and does not claim durability. Event ids satisfy `validJobID`
(lowercase hex nonce). `CONCURRENT_READ` writes no event. Making the file a
mirror source cannot break `CloseCheck`, which never asks for it, and cannot
break `Mirror`, which is idempotent by digest and keeps prior manifest entries;
a refusal appended after a mirror simply makes the next mirror not-unchanged.

**Refusal register.** Both rows are correct. `read_admission.go:187` is the
`return &OpError{` line for `CONCURRENT_READ` and `read_admission.go:210` is the
one for `REDUNDANT_READ`; each code has a single, unambiguous emission branch.
Owners, shapes, overrides and command counts match the brief, `Pending` is
empty, `SUBJECT_MISMATCH` is unmoved. `ADMITTED` is not a refusal-shaped token
(no underscore), so the completeness test at `internal/refusal/register_test.go:20`
stays satisfied. Note that the register's own tests do not verify `Site` line
numbers; I checked them by hand.

**Red proof.** I mutated the fold's history write to a no-op in the scratch copy:
`TestCleanReadHistorySurvivesChangedFold` failed in both its `native-history` and
`pre-history-backfill` subtests, so the core D7 claim is genuinely
discriminating. The admission tests do not depend on the fold writing history
(they seed the field or the pre-history shape directly), which is correct
separation of owners. Apart from F-5 I found no tautological assertion; the
commit-exemption and changed-subject cases are correctly identified as
compatibility guards.

**Scope.** The diff touches only `internal/dispatch` and
`internal/refusal/register.go`. No command, no shell, no fixture, no
`testing.json`, no coverage ratchet, no role text, no `plans/` edit, no writer or
authority change beyond D7's `cleanReadRounds` and D9's event file. No receipt
row inside the reviewed tree (`artifacts/` is gitignored, so the brief and result
files are outside the diff). Size is inside the 1,000 to 1,400 band and under the
1,500 split threshold.

## Verdict

**Not fit to land.** Three material findings stand. F-1 and F-2 share one root
cause and one fix: `cleanReadsForRoot` must distinguish "this evidence does not
prove a clean read" (return no observation, D7's conservative compatibility)
from "this evidence is internally contradictory" (error), and in particular
absent round `subject.json` or `return.json`, a persisted subject that does not
match the recorded digest, and a non-completed member at the folded round must
all degrade to no proof rather than refuse. F-2 additionally needs the fold to
stop treating clean-read history as a precondition: `CritiqueRegisterAdvance`
must keep folding when the history read yields nothing, since the cancelled-fold
path exists precisely to unwedge a chain. F-3 needs `highestCriticMember` to be
computed only for roots that actually produced a redundant candidate, so an
unrelated sibling's member state cannot refuse a fresh read. Before 1b-ii, F-4
should be fixed too, so the typed result never says `ADMITTED` alongside an
error.
# Second independent code read: critique-closes part 1b, unit 1b-i (post-fold)

Reviewer: Opus 5, independent of the builder and of the fold. Worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccf-s1b/metasystem`,
HEAD `24c367f8`. Reviewed: `git diff` (finding_register.go, mirror.go,
reads_refused.go, refusal/register.go) plus untracked read_admission.go,
read_admission_test.go, reads_refused_test.go, mirror_read_refusals_test.go.
Prior read: `artifacts/reports/opus-read-1b-i.md` (F-1 to F-9).

Probes ran in a throwaway copy of the module under
`/private/tmp/claude-501/.../scratchpad/ms`. No repository file was edited, no
fixture bed was run, and I did not repeat the seat's build, vet, race or
fast-gate runs.

Observation that frames the rest: `internal/dispatch/finding_register.go` is
byte-identical to the version the first read cited (its citations at :32,
:86-90, :121-124, :161, :205-216 still land on the same content). The entire
fold was made inside `read_admission.go`. In particular
`CritiqueRegisterAdvance` still aborts on ANY `cleanReadsForRoot` error
(`finding_register.go:121-124`); F-2 was addressed by shrinking the set of
errors, not by making the fold tolerant of a failed history read.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| R2-F-1 | high | yes | Absent evidence still errors when what is absent is an older round's MEMBER JOB RECORD. `validateCleanReadEvidence` demotes a missing `subject.json` and a missing `return.json` to "no proof", but `critiqueRecordForRound` failure is still returned as an error. So a fresh, unrelated read is still refused forever, and a live fold is still aborted, by exactly the absence class F-1/F-2 named. This is a documented `evidence-gc` outcome, not a hypothetical. | `internal/dispatch/read_admission.go:384-387` returns the error; `internal/dispatch/finding_register.go:671-690` is the failing helper; `internal/dispatch/critique.go:33-38` silently skips any unparseable record, so "unreadable" and "absent" are the same input here. Probe P10 (realistic GC shape: closed multi-round critic chain, payload collected by `internal/evidence/gc.go:183`, round member records pruned by `pruneMirroredRecords` `internal/evidence/gc.go:375-449`, ROOT record retained by the stale-manifest rule its own comment at `gc.go:410-416` describes): `CritiqueReadAdmission` for an unrelated fresh subject returns `clean read root critic round 2 has invalid member evidence: critic root critic has no record for folded round 2`, `Decision=""`. Probe P3 (same absence, open chain): `CritiqueRegisterAdvance(repo,"critic","critic-r3")` returns `""` with the same error, so the fold is wedged. |
| R2-F-2 | medium | yes | The F-3 fix moved only `highestCriticMember`. `cleanReadsForRoot` is still called unconditionally for EVERY scoped root, before any subject-equality test, and any per-root error still refuses the whole admission. So an unrelated sibling root in the same scope can still refuse a legitimate fresh read of a completely different subject. | `internal/dispatch/read_admission.go:84-89` (unconditional call and bare error propagation), versus the fixed `:100-104`. Probe P5, five variants, each an unrelated `sibling` root holding subject `a/1` while the request is the unrelated fresh subject `z/9`: `round-not-one` and `round-missing` give `clean read root sibling is not round one` (`:263-265`); `register-round-string` gives `clean read root sibling has malformed register round state` (`:277-280`); `register-malformed` gives `clean read root sibling has a malformed finding register` (`:266-269`); `history-without-register` gives `clean read root sibling has history without a finding register` (`:271-275`). All five returned `Decision=""` and an error for a read that is not redundant with anything. |
| R2-F-3 | low | no | The fold republishes `cleanReadRounds` as the re-derived list on every advance, so an observation whose evidence is momentarily unreadable is erased from the record permanently, not merely skipped for that one decision. The field is therefore a cache trimmed on each fold, not the durable record D7 describes. Consequence is a missed refusal (wasted read), never a wrong close. | `internal/dispatch/finding_register.go:219-221` writes `encodeCleanReadRounds(cleanReads)` where `cleanReads` came from `cleanReadsForRoot` at `:121`. Probe P11: after removing round 2's `subject.json`, the round-3 fold advanced and `cleanReadRounds` became `[round 1, round 3]`; round 2's observation does not return when the file does. |
| R2-F-4 | low | no | The line between "cannot prove" and "contradictory" is drawn inconsistently for the SAME disagreement. In the history path a recorded subject that disagrees with the on-disk `subject.json` errors; in the pre-history path the recorded digest disagreeing with the same file degrades to no proof. Both are defensible (a stale digest is the legitimate cancelled-fold case, a stale history entry is not) but nothing in the code says so. | `internal/dispatch/read_admission.go:405-407` (error) versus `:355-357` (no proof). Probe P9 confirms the first: rewriting round 1's `subject.json` makes the round-2 fold return `clean read root critic round 1 does not match its persisted subject`. |
| R2-F-5 | low | no | The unset decision is the empty string. `ReadAdmissionResult.Decision` has no `omitempty`, so a typed result on an error path serializes `"decision": ""`; there is no named token for "undecided". Correct against the brief (never ADMITTED alongside an error) but 1b-ii must be told that "" is the third state. | `internal/dispatch/read_admission.go:32` and `:53`; probes P6 and P12 both observed `Decision=""` with an error. |
| R2-F-6 | low | no | The third comparator in the redundant sort is unreachable. `latestEqual` yields at most one candidate per root and the second comparator already totally orders distinct roots, so `read.Round >` never runs. | `internal/dispatch/read_admission.go:90-99` and `:112-120`. |
| R2-F-7 | low | no | Carried forward unfixed from the first read (F-5): the child-member assertion in `TestMirrorIncludesReadRefusals` is still tautological. The manifest is per-ROOT and `mirrorLand` seeds `files` from the prior manifest, so the entry is present for the child whether or not `mirrorSources` adds it. The code is nonetheless correct. | `internal/dispatch/mirror_read_refusals_test.go:66-69`; `internal/dispatch/mirror.go:60-64` (one destination per `rootJob`) and `:235-238` (`files` seeded from `old`). |
| R2-F-8 | low | no | Carried forward unfixed, both non-material in the first read: the `round` argument is still validated and then unused (F-7), and refusal timestamps still use `RFC3339Nano` while `nowISO()` emits second precision (F-8). | `internal/dispatch/read_admission.go:57-58` (only use), `:122-125` and `:131` (prior round is what reaches the result and the event id); `:135` versus `internal/dispatch/record.go:726`. |

## Answers to the seven questions

**1. Where the line is drawn.** For every evidence class the fold actually
touched, correctly. `subject.json` absent (`:349-351`, `:402-404`),
`return.json` absent (`:411-413`), a stale pre-history digest (`:355-357`), an
absent pre-history digest (`:341-344`), a non-clean pre-history register
(`:338-340`), and a member that is not `completed` (`:395-397`) all yield no
observation. Structural contradictions still error: a history array that is not
an array, an entry that is not exactly `{round, subject}`, a round outside the
folded bound, duplicate or unordered rounds, a subject that fails role
validation, a return whose coordinates name a different member, a return that
does not bind, and the current fold claiming clean while its register is not
(`:368-381`). I could not reach `validateCurrentCleanFold`'s error branch from
any real operation: every register writer outside the fold
(`CritiqueRegisterAcceptRisk` `finding_register.go:436`,
`CritiqueRegisterResolveOutOfScope` `:474`, `CritiqueRegisterClose` `:530`)
refuses a finding that is not `open` or `disputed`, and a clean register has
none, so a clean current fold cannot be made non-clean behind the history.
**The one class still on the wrong side is R2-F-1: an absent member job record
errors where D7 wants no proof.** I found no case that now silently yields no
proof where the evidence is genuinely contradictory. The closest is
`:395-397` (history says the round completed, the record now says cancelled),
which is contradiction treated as no-proof, but that is precisely what the
first read's F-1 prescribed and what `folded-member-not-completed` tests.

**2. False refusals and slipped redundancy.** A legitimate fresh read can still
be refused, by R2-F-1 and R2-F-2, on the error path rather than the exit-11
path. Neither produces an OpError, so a caller that keys on the refusal code
will see a hard failure instead. On the other side I found nothing that now
slips through. The narrowing is a strict subset: `highestCriticMember` is
skipped only for roots that produced no equal clean observation (`:100-104`)
and, in the concurrent pass, only for roots with no equal persisted live
subject at ANY round (`:156-158`, `rootHasEqualPersistedLiveSubject` at
`:426-442` globs every round and returns true on a read error). Both gates are
supersets of their refusal conditions, so no refusal is lost. Redundant
admission still fires for the full equality, scope and role matrix
(`TestReadAdmissionRefusesCleanSubject`, `TestReadAdmissionChecksScopeAndRole`),
and the concurrent rules (`running`, `pending-setup`, `completed`, `failed`
unfolded refuse; cancelled, folded, different role, different subject, own root
admit) are all exercised.

**3. Does the fold keep folding.** For every case the first read probed, yes.
Probe P1: a chain whose entire payload directory was removed still folds its
next round (`advanced`). The stale-digest case is the new regression test. The
missing-return case degrades. But `CritiqueRegisterAdvance` still returns any
`cleanReadsForRoot` error verbatim (`finding_register.go:121-124`), and three
history reads still abort it: an absent member record (P3, R2-F-1), a corrupt
older `return.json` (P2, `clean read root critic round 1 return is unreadable`),
and a corrupt older `subject.json` (P4, `... subject is malformed: unexpected
EOF`). P2 and P4 need real file corruption and records are written through
`atomicWriteText`, so I count only P3 as material.

**4. Unset-then-ADMITTED.** Correct on every return path. `Decision` is left
empty at `:53`, set to a refusal token only immediately before an error return
(`:122` and `:188`), and set to ADMITTED only at `:201-203` under `err == nil`
or at `:63-65` on the commit exemption where the error is nil by construction.
Every locked evidence-validation failure (`:72`, `:75`, `:78`, `:88`, `:103`,
`:166`, `:170`, `:177`, `:183`) returns through the closure with `Decision`
still empty; probes P6 and P12 confirmed two of them by observation. The
redundant branch keeps `REDUNDANT_READ` on the nonce and append failures, which
is what D9 requires. No path returns ADMITTED with a non-nil error.

**5. Red proof of the four new regressions.** I reconstructed the pre-fold
behaviour in the scratch copy from the first read's exact error strings
(absent round subject, stale digest, missing return, non-completed member,
unconditional `highestCriticMember`, `Decision` preset to ADMITTED) and ran the
four tests. All four go red, each for its own reason:
`TestReadAdmissionTreatsUnavailableCleanEvidenceAsNoProof` fails all four
subtests with `recorded subject is missing`, `return is unreadable`, `does not
match folded digest` and `member critic is not completed`;
`TestCritiqueRegisterAdvanceContinuesWithoutHistoricalProof` fails with the
round-3 fold refused by the stale digest;
`TestReadAdmissionIgnoresMalformedUnrelatedSibling` fails both subtests with
`critic root sibling has member sibling-r2 with malformed round state` and the
role variant; `TestReadAdmissionErrorsAreNeverAdmitted` fails with
`Decision:ADMITTED` beside the invalid-job-id error. No assertion among the
four is tautological. Two weaknesses worth naming, neither material: the second
assertion in `TestReadAdmissionTreatsUnavailableCleanEvidenceAsNoProof` uses a
DIFFERENT fresh subject, so it proves only that no error is raised, not that
the observation is gone (the first assertion carries that); and none of the
four covers an absent member record, which is why R2-F-1 survived the fold.

**6. Nothing the first read verified correct was weakened.** D7's append
conditions are unchanged (`finding_register.go:205-216`: completed, subject
present, return binds, resulting register clean). Retry idempotence is intact:
`round <= foldedRound` returns `unchanged` at `:112-115`, before the history
read at `:119-124`, and the test asserts the history bytes are unchanged after a
retry. Single-record atomicity is intact: register, round, subject digest and
history publish in the one `writeRecord` at `:222` inside `withRecordLock`.
`cleanReadRounds` is still never authority: the only readers in the whole
repository are `finding_register.go:120`, `:220` and `read_admission.go:270`,
and `cleanReadCanClose` (`:485-513`) still only selects diagnostic wording.
`dedicatedMetadataFields[cleanReadRoundsField]` still blocks generic
`RecordCAS`. Both refusal rows are still correct and their site lines were
updated for the new file: `read_admission.go:192` is the `return "", &OpError{`
for `CONCURRENT_READ` and `read_admission.go:218` is the one inside
`redundantReadError` for `REDUNDANT_READ`. Nothing validates `Site` numbers
automatically, so I checked both by hand. Mirror and close-check safety are
unchanged; adding the file as a mirror source does not let `evidence-gc` eat a
newer refusal, because `unaccountedFiles` compares sha256 for every non-`jobs/`
path (`internal/evidence/gc.go:314-335`), so an appended refusal keeps the chain
from being collected until it is re-mirrored. One behavioural note, benign:
`finding_register.go:119` now aliases `state.records[rootJob]` to the copy read
under the record lock, which the fold then mutates; `refuseCrossRootClassConflict`
takes the prospective register as a parameter and `critiqueRoundAccounting` takes
`root` explicitly, so neither changes outcome.

**7. Scope is clean.** The working tree holds exactly four modified files
(`internal/dispatch/{finding_register,mirror,reads_refused}.go`,
`internal/refusal/register.go`) and four untracked test-and-source files, all
under `internal/`. No `cmd/`, no `scripts/agents/dispatch.sh`, no
`dispatch-fixtures.sh`, no `testing.json`, no coverage floor, no role text, no
`plans/` edit, no receipt row, nothing from 1b-ii. `CritiqueReadAdmission` is
exported and unwired, which is correct for 1b-i.

The builder's claim of zero material findings on the complete worktree does not
hold. Two material findings stand, both reachable by observation.

## Verdict

**Not fit to land.** Two material findings. What must change:

1. R2-F-1. Treat an absent or unreadable member job record for an older round
   the way `subject.json` and `return.json` absence is already treated: return
   no observation, not an error. `internal/dispatch/read_admission.go:384-387`
   is the line. Without it the fold's own stated intent is unmet on a path
   `internal/evidence/gc.go` produces by design, and a live fold can still be
   wedged.
2. R2-F-2. Stop letting one scoped root's evidence error refuse the whole
   admission when that root contributes no equal observation.
   `internal/dispatch/read_admission.go:84-89` should treat a per-root
   `cleanReadsForRoot` failure the way `highestCriticMember` is now treated,
   so only a root that actually produced an equal candidate, or the requesting
   root itself, can refuse a fresh read.

Both are one change in shape and could be adjudicated as a single amendment.
Nothing else in the unit blocks landing; R2-F-3 to R2-F-8 are recorded and
should not action.
# Third independent code read (deciding): critique-closes part 1b, unit 1b-i, after the second fold

Reviewer: Opus 5, independent of the builder and of both folds. Worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccf-s1b/metasystem`,
HEAD `24c367f8`. Reviewed: `git diff` (`internal/dispatch/finding_register.go`,
`mirror.go`, `reads_refused.go`, `internal/refusal/register.go`) plus the four
untracked files `internal/dispatch/read_admission.go`, `read_admission_test.go`,
`reads_refused_test.go`, `mirror_read_refusals_test.go`.

Prior reads: `artifacts/reports/opus-read-1b-i.md` (F-1 to F-9) and
`artifacts/reports/opus-read-1b-i-round2.md` (R2-F-1 to R2-F-8). Builder account:
the "Second fold round" section of `artifacts/reports/codex-ccf-slice1b-result.md`.

All probes ran in a throwaway copy of the module at
`/private/tmp/.../scratchpad/ms3`, byte-identical to the worktree `internal/`
tree (`diff -r --brief` clean before every probe run). No repository file was
edited, no commit was made, no fixture bed was run.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| R3-F-1 | low | no | The absence branch is slightly wider than the builder's claim. A member record that is still on disk but whose `round` field is unparseable is not found by the lookup, so it now degrades to no proof instead of erroring. The builder's claim enumerates only a duplicate record, a wrong critic role and an invalid status as still-erroring contradictions; a malformed round is a fourth contradiction class and it is not on that list. Consequence is a missed `REDUNDANT_READ` (one wasted critic read), never a wrong close or land, and the shape is not one `evidence-gc` produces. | `internal/dispatch/finding_register.go:684-686` skips a record whose `round` fails `numInt`, so `:694` returns the missing type, which `internal/dispatch/read_admission.go:391-393` demotes. Probe D1 (design subject, `dcritic-r2.json` with `"round": "two"`): `CritiqueReadAdmission` returns `decision="ADMITTED" err=<nil>`, where the untouched control D1b returns `REDUNDANT_READ`. On the live path the same shape is still caught, by the concurrent pass rather than the history read: probe P1 returns `decision="" err=critic root critic has member critic-r2 with malformed round state` from `read_admission.go:486`. |
| R3-F-2 | low | no | R2-F-2's fix has a narrow residual. `rootHasEqualPersistedLiveSubject` returns true on a read error, so an unrelated sibling with an unreadable `rounds/N/subject.json` is still pulled through the relevance filter into strict validation; if that sibling's own root shape is also invalid, a fresh unrelated read is refused with a hard error rather than admitted. Both preconditions are states the system never writes (records and subjects publish through `atomicWriteText`), and the round-2 read excluded the same corrupt-file class as non-material. | `internal/dispatch/read_admission.go:465-468` (`if readErr != nil { return true }`) feeding the third disjunct at `:88`. Probe P5 (sibling root mutated to `round: 2`, `rounds/1/subject.json` overwritten with `{ not json`, request is the unrelated subject `z/9`): `decision="" err=clean read root sibling is not round one`. Probe P5b, the same invalid root with the payload merely absent (the real GC shape): `decision="ADMITTED" err=<nil>`. |
| R3-F-3 | low | no | R2-F-3 (history is a cache the fold re-derives, not a durable record) is widened by this round: the shape that triggers the trim is now the routine `evidence-gc` outcome rather than accidental corruption, because a pruned member record no longer wedges the fold but silently drops its observation and the fold republishes the shortened list. The consequence is unchanged and stays a missed refusal, never a wrong close, and trimming is strictly better than the pre-fold wedge it replaces. D7 says "persist only proven clean fold observations", which the re-derivation honours literally. | `internal/dispatch/finding_register.go:219-221` republishes `encodeCleanReadRounds(cleanReads)` from the value read at `:121`. Probe P9: a clean round-1 fold refuses an equal read with `REDUNDANT_READ`; after `rounds/1` is collected and round 2 folds, `cleanReadRounds` is `[round 2]` and the same equal read returns `decision="ADMITTED"`. Probe P7 shows the companion case: with `critic-r2.json` pruned and its payload retained, round 3 advances and history becomes `[round 1, round 3]`. |
| R3-F-4 | low | no | The demotion uses a bare type assertion rather than `errors.As`, so if `critiqueRecordForRound` ever wraps its return the demotion silently reverts to the R2-F-1 wedge with no test catching it. Nothing wraps it today and no caller matches on the message, which is byte-identical to the pre-fold string. | `internal/dispatch/read_admission.go:391` (`err.(*missingCritiqueRoundRecordError)`) versus `internal/dispatch/finding_register.go:694`. A repository-wide grep for `has no record for folded round` finds only `finding_register.go:677` itself, and no `errors.Is`/`errors.As` in `internal/dispatch` or `cmd/` touches this type. |
| R3-F-5 | low | no | Two guards inside `cleanReadsForRoot` are now unreachable from admission, because `scopedCriticRoots` already excludes those roots. They remain reachable from `CritiqueRegisterAdvance`, so they are not dead code, but they no longer describe an admission behaviour. | `internal/dispatch/read_admission.go:265-267` (parent named) and `:261-264` (non-critic role) versus the filter at `:233`. Probe P6: with a sibling holding the exact requested subject, the `parent-named` and `non-critic-role` mutations both yield `decision="ADMITTED" err=<nil>`, while `round-not-one`, `register-malformed` and `history-without-register` all still error. |
| R3-F-6 | low | no | Carried forward unfixed and still non-material: the tautological child-member assertion in `TestMirrorIncludesReadRefusals` (F-5, R2-F-7); the validated-then-unused `round` argument (F-7); `RFC3339Nano` refusal timestamps beside second-precision `nowISO()` (F-8); the unreachable third comparator in the redundant sort (R2-F-6); `Decision` having no named token for its unset third state (R2-F-5); and the inconsistent but defensible line between "cannot prove" and "contradictory" for a stale digest versus a stale history entry (R2-F-4). | `internal/dispatch/mirror_read_refusals_test.go:66-69`; `read_admission.go:57-58`, `:140`, `:124`, `:32`, `:355-357` versus `:413-415`. |

## Question 1: absence versus contradiction

Every branch of `validateCleanReadEvidence` (`internal/dispatch/read_admission.go:388-432`) sorts as follows.

No proof: the member record is missing or unreadable (`:391-393`); the member
status is terminal but not `completed` (`:403-405`); the round `subject.json` is
absent (`:410-412`); the round `return.json` is absent (`:419-421`). Every one
of these is a shape `internal/evidence/gc.go` can leave.

Still an error: a duplicate member record for one folded round
(`finding_register.go:690`), a member whose role is not the chain role
(`read_admission.go:396-398`), an invalid status (`:400-402`), an unreadable
`subject.json` (`:407-409`), a persisted subject that disagrees with the
recorded one (`:413-415`), an unreadable `return.json` (`:422`), return
coordinates naming a different member (`:426`), and a return that does not bind
(`:429`).

I confirmed the three the builder claims still error, on the admission path and
on the fold path, and confirmed the pruned-member shape on both:

- Probe P2, admission, member record pruned with the payload retained: `decision="ADMITTED" err=<nil>`.
- Probe P3, admission, a second record claiming round 2: `decision="" err=... carries multiple records for folded round 2`.
- Probe P4, admission, retained member with `status: "bogus"`: `decision="" err=... has invalid status "bogus"`.
- Probe P7, `CritiqueRegisterAdvance`, member record pruned with the payload retained: `outcome="advanced"`, history `[round 1, round 3]`.
- Probe P8, `CritiqueRegisterAdvance`, duplicate member record: `outcome="" err=... carries multiple records for folded round 2`. A parseable contradiction still wedges the fold, which is what the design intends.

The one contradiction that now passes as no proof is a retained member record
with a malformed `round` field (R3-F-1). It is not a collector-produced shape,
it is caught on the live path by the concurrent pass, and on the design path its
only consequence is an extra critic read.

Direction of every demotion is toward admitting a read. None of them can
manufacture an observation, so none can produce a false refusal or a false
close. `cleanReadRounds` is not authority to close or land, so a dropped
observation cannot become a wrong gate.

## Question 2: the relevance filter

The filter is `read_admission.go:86-90`: a scoped root is validated only when it
is the requesting root, or its recorded clean-read state names an equal subject
(`rootMayHaveEqualCleanRead`, `:437-456`), or, for a live subject, it holds an
equal persisted subject at any round (`rootHasEqualPersistedLiveSubject`,
`:458-474`).

**No legitimate fresh read is refused by an unrelated root**, except the
double-corruption case in R3-F-2. `TestReadAdmissionSkipsUnrelatedSiblingCleanStateErrors`
covers the five sibling shapes the second read listed and I reproduced the
`ADMITTED` outcome for each.

**No refusal is lost.** The filter is a provable superset of the refusal
condition in both directions:

- Pre-history derivation can only ever return an observation whose digest equals
  `findingRegisterSubjectDigest` (`:346-362`), which is exactly the filter's
  first disjunct. `ReadSubject.Digest` hashes precisely the fields
  `ReadSubject.Equal` compares (`internal/readsubject/subject.go:36-72`), so
  `Equal` implies equal digest and the two gates cannot disagree.
- History observations are decoded from the same `cleanReadRounds` entries the
  filter scans, with the same `DecodeReadSubject` and the same `Equal`; the
  filter's per-entry checks are strictly looser (it omits the `len(entry) != 2`
  test), so any entry that could produce a candidate also passes the filter.
- The concurrent pass (`:156-203`) keeps its own gate and is untouched.

Probes: D3 (equal subject reachable only through an older history entry while
the register digest names a newer subject) returns `REDUNDANT_READ` naming root
`critic` round 1 with the correct "later round 2" wording; D5 (pre-history root,
no history field) returns `REDUNDANT_READ`; P10 returns `CONCURRENT_READ`.

**The adversarial case the brief asks for** is probe P6: a sibling root whose
`cleanReadRounds` names the exact requested subject while its own shape is
invalid. `round-not-one`, `register-malformed` and `history-without-register`
all still error, so equal evidence stays strict and the filter does not buy
silence at the cost of a lost guard. The two shapes that do go quiet
(`parent-named`, `non-critic-role`) are excluded one layer earlier by
`scopedCriticRoots` (`:233`) and were already excluded before this fold, so they
are not a regression (R3-F-5). Probe D4 shows the remaining case is handled by
`validateCurrentCleanFold` rather than by the filter: a history entry at the
folded round with the digest field deleted yields no proof, which is the
intended agreement check, not a filter escape.

## Question 3: nothing the earlier reads verified was weakened

- **D7 append conditions.** `finding_register.go:205-216` is unchanged: the
  append fires only when the fold completed, the subject was present, the return
  bound it, and the register resulting from the fold is clean. Cancelled,
  failed, unbound, accepted-risk, deferred and out-of-scope folds still add
  nothing.
- **Retry idempotence.** `:112-115` returns `unchanged` before the history read
  at `:119-124`, so a retry neither reads, trims, nor republishes history.
- **Single-record atomicity.** Register, folded round, subject digest and
  `cleanReadRounds` still publish in the one `writeRecord` at `:222` inside
  `withRecordLock`. The second fold added no second write.
- **`cleanReadRounds` never authority.** A repository-wide grep finds readers
  only at `finding_register.go:120`, `:220` and `read_admission.go:275`, `:441`.
  `CloseCheck`, landing and recertification do not read it, and
  `cleanReadCanClose` (`:517-545`) still only selects diagnostic wording.
  `dedicatedMetadataFields[cleanReadRoundsField]` at `finding_register.go:32`
  still blocks generic `RecordCAS` injection.
- **The `state.records[rootJob] = root` aliasing** introduced by the first fold
  (`:119`) is still benign. I checked both later consumers independently:
  `refuseCrossRootClassConflict` skips `id == currentRoot` (`:1242`) and its
  `findingRegisterSubject` reads only `reviews` and round `return.json` files
  (`:1276-1298`); `critiqueRoundAccounting` takes `root` explicitly and otherwise
  reads only member `status` (`:264-274`). Neither touches
  `findingRegisterSubjectDigest` or `cleanReadRounds`.
- **Mirror and close-check safety.** `mirror.go` and `reads_refused.go` are
  unchanged from the state the first two reads verified; the chain-level source
  is still `filepath.Join(payload, "reads-refused.jsonl")` mapped to the root
  destination.
- **The two refusal rows and their site lines.** Both rows are present at
  `internal/refusal/register.go:75-76`, and the sites are re-synced for the five
  lines the relevance filter added: `read_admission.go:197` is the
  `return "", &OpError{` for `CONCURRENT_READ` and `read_admission.go:223` is the
  `return &OpError{` inside `redundantReadError` for `REDUNDANT_READ`. I checked
  both by hand; nothing validates `Site` numbers automatically. Owners, shapes,
  overrides and command counts are unchanged and `SUBJECT_MISMATCH` is unmoved.
- **Independent green.** `go test -count=1 ./internal/dispatch ./internal/refusal
  ./internal/readsubject` in the byte-identical scratch copy: `ok` for all three
  (dispatch 74.866s). `gofmt -l internal/dispatch internal/refusal` is empty.

## Question 4: the new regressions are genuinely red and not tautological

I reconstructed the pre-fold behaviour twice in the scratch copy, one change at a
time.

Reverting only the missing-record demotion (restoring the plain error at
`read_admission.go:391-393`):

```
--- FAIL: TestReadAdmissionTreatsMissingFoldMemberAsNoProof/collected-member
        ... clean read root critic round 2 has invalid member evidence: critic root critic has no record for folded round 2
--- FAIL: TestCritiqueRegisterAdvanceContinuesWithMissingFoldMember
        round three fold = "", clean read root critic round 2 has invalid member evidence: ...
```

Reverting only the five-line relevance filter at `read_admission.go:86-90`:

```
--- FAIL: TestReadAdmissionSkipsUnrelatedSiblingCleanStateErrors/round-not-one
--- FAIL: .../round-missing
--- FAIL: .../register-round-string
--- FAIL: .../register-malformed
--- FAIL: .../history-without-register
```

Each of the three new tests is red for its own cause, and each subtest of the
five-variant table is independently red. The `contradictory-member` subtest and
the requesting-root half of the sibling test are the discriminating negatives:
they prove the amendment narrowed the error set rather than deleting it. I found
no tautological assertion among them.

## Question 5: scope

The working tree holds exactly the four modified and four untracked files, all
under `metasystem/internal/`. `git status --porcelain -- cmd scripts testing.json
plans docs memory records` is empty: no command, no shell entry point, no
fixture, no `testing.json`, no coverage floor, no role text, no receipt row, no
`plans/` edit. `CritiqueReadAdmission` has no caller outside `internal/dispatch`
(the only other hits are the design page's own prose), which is correct for
1b-i. `artifacts/` is gitignored, so the briefs and reports are outside the diff.

## Verdict

**Fit to land, as unit 1b-i.** Zero material findings. Both second-fold claims
hold under probe: the round-record lookup distinguishes absence from a parseable
contradiction on the admission path and on `CritiqueRegisterAdvance`, and the
relevance filter refuses no legitimate fresh read while losing no refusal.
R3-F-1 through R3-F-6 are recorded and should not action. The unit remains
incomplete by design: 1b-ii and the seat's fixture beds, delivery selection and
coverage measurement are still owed.
