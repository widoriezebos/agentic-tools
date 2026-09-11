Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry)
Date: 2026-09-11

# Review brief: first independent critique of the carried-landing design, revision 3

FINDING IDS: chain-unique, continue the carry's register: HCL-C-21, HCL-C-22, ... never F-n. A re-opened finding from the old chain (HCL-C-02 to HCL-C-20) keeps its own id.

## What you are reading

`metasystem/plans/human-carried-landing-carry-design.md`, revision 3, at
commit e8bdd13f4 in your worktree (a local scaffold commit: the page lands
on `main` together with this read), sha256
eaf9ffd4d831317f1dab1aa90d45313a9d2a74aab3916d4d6c920bc16a2a9698, authored
on the Fable lane (job hcl-design3-20260911). The "Declared Outputs"
digest line the dispatcher stamps into your prompt is the digest of the
outputs manifest, not of the design. The goal record is
`metasystem/plans/goals/human-carried-landing-carry.md`; the brief the
page answered is `metasystem/artifacts/agents/hcl-context/design-brief.md` (unlanded; it lands with the page);
the old page it carries forward is
`metasystem/plans/human-carried-landing-design.md` (revision 2.1, whose
last table lists the thirteen findings this page answers).

Write your register as this new file: `metasystem/records/misc/human-carried-landing-carry-critique-r1.md`
If your runtime is read-only, return it in the job return; the
coordinator projects it.

Read-only design critique; implement nothing, run no bed. A material
finding is a place where two implementers would build different things,
a claim the page rests on that the tree does not support, a fixture
without a decidable pass condition, or a way a carry lands something the
human did not name or refuses a verified human on the machine's own
judgement. This is fold-read cycle 0; material findings are expected.
Read the code at HEAD (the page cites main 976cb50bb; your worktree is
one plans-only commit past it).

## Settled, do not re-derive

Point 01 (refusals bind agents; a verified human is never refused) and
point 03 (the audit) stand on the old page. Wido's ruling of 2026-09-04
put the carry on this goal; HCL-C-20 is closed by it. The two proofs
(enrolled terminal, authenticated channel answer) and the exclusion of
the temporary word are settled. The umbrella's two additions (a fleet cap
on open carries, no stacking on unpaid debt) are required, not optional.

## Mandate, in order of consequence

1. **The match rule (05, step 12).** The word names one refusal code or
   one testing group; the landing "hits" it when the ordinary verdict O
   and the testing groups M line up as the table says. Find a landing the
   rule lands that the human did not name (two things wrong, one named),
   and a landing it refuses that the human did name. Check `unneeded`
   (O passes, M empty: lands as ordinary, the word consumed with no
   obligation) against point 01 and against a human who named a refusal
   that then did not fire.
2. **The two-step judge (05, HCL-C-03).** The live engine decides; if it
   cannot return a complete decision, commit.sh builds a judge from HEAD
   in a detached worktree (`go build` at commit time) and uses it. Check
   what "cannot return a complete decision" means against commit.sh's
   four required fields, whether a candidate the live engine cannot read
   is honestly judged by a base engine that predates the candidate, the
   cost, and the adopted-binary case the page says cannot carry.
3. **Carry-forward (05, HCL-C-04 and HCL-C-19).** A wip commit through
   plumbing, `git rebase`, `git reset --soft`, then the workspace-tree
   comparison and the single push. Check each step on a leased seat
   checkout (hooks, the lease's `run-held`, the LANDING comparison, an
   index that holds unrelated staged paths), the conflict path, the
   detection of a local carried commit by trailer, and whether the second
   word after a code move really lands with one commit on origin.
4. **The transaction and `landing carry-status` (06, HCL-C-18).** Six
   writes, the consumption definition (a `carried` row, a superseding
   `carry` row, or a `Carry:` trailer on origin since the row's At), the
   five states. Find a crash or race that lands twice, consumes without
   landing, or leaves an obligation unwritten with no recovery; check
   `goal recover`'s two closers and the AlreadyApplied rule keyed by
   ApprovedRef against recover.go and verbs.go as they are.
5. **The authority wire (02, HCL-C-02).** A third outcome class on
   history rows, `authorityGeneration`, the parse and render rules, the
   name from `--by` joined through the enrollment generation and the
   proof file. Check every reader of history lines for a row it now
   refuses or misreads, what an old engine does with the new outcome on
   a ledger it fetches, and whether the fixture proof's generation zero
   can be told from a real proof by any reader that matters.
6. **The cap and the debt (08).** The cap counted by the opid's machine
   segment, asked at the word and the landing; the debt as an open
   `human-carried` obligation on any live goal whose commit is an
   ancestor of the base. Check the budget-law reading (`lawValue`,
   `config validate`), the seat attribution of a channel word, the
   `--supersede` transaction, and whether "any live goal" is readable at
   the landing without a fetch the page does not make.
7. **The obligation and its discharges (07, HCL-C-05, -06, -07).** The
   full-id finding, the command-layer effects (a) to (d) for chain
   `human-carried`, `AcceptedRiskDecision`'s one change, `done` refusing
   an open carry word. Check each against goalsync_mutations.go and
   verbs.go at HEAD; name any effect the page missed.
8. **The commit subject (09, HCL-C-09).** Four seams. Check
   `validateFreshCriticReviewsLatest`, `critiqueSubjectForRound`,
   `reviewReferenceBinding` and `StampClaimedReviewReference` as they are,
   and what `dispatch.sh close` does with a critic chain whose subject is
   a commit.
9. **The register flip (HCL-C-10, -11).** The 48 rows, the three
   overrides that change meaning, the nine ask codes as Question rows, the
   ShellRows rewrite and the relaxed test. Check `TestHCL03NoPendingAfterSlice2`'s
   two conditions are decidable from `collectGoalVerbs` and the register.
10. **Fixtures and files.** Every fixture named has a decidable pass
    condition; every mechanism the page leans on exists by that name or
    is in "Files this design touches"; the five packages the page leaves
    to a testing.json check. Name any test or bed the page names that
    does not exist at HEAD.
11. **Scope.** Two slices, 2a (engine) and 2b (wrapper and seams), each
    estimated 90 minutes. Say whether 2a is buildable in one Sol round
    and name any piece that can be a follow-up without weakening the
    goal's sentence: a verified human carries one named refusal, the
    review is deferred not deleted, every use counts, the cap and the
    debt ask.

## What is not in your mandate

Style, the length of the page, whether the feature should exist (Wido's
ruling), durations. Do not run beds.

## Return

One register at the path above: findings by id (HCL-C-21 onward, or a
re-opened old id), each with the section, the claim, the evidence (file
and line at HEAD), and what changes if the finding stands. End with a
verdict line: buildable as is, or one fold naming which findings.
Wall-clock budget: 45 minutes.
