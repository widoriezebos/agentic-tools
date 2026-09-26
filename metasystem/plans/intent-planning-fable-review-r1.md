# Fable critique r1: Complete design work through intent

- Kind: critique report
- Reviewed record: 01M3EFDSFTKWEMSDCP1BB7TDGQ (draft), `metasystem/plans/designs/intent-planning-continuity.md`
- Reviewed design SHA-256: `495456bf4e7175abef81392d37e9a2e066b0808e48d4d2270b578d195a09e7d7`
- Parent contract: 01M3EC3QT7M2TC36P7ZVNRF0RW (accepted), `plans/designs/intent-workflows.md`
- Model: claude-fable-5-1 (Fable 5.1). Round 1 of at most 2. Worktree HEAD b5d7b6819, uncommitted product files present.
- `project design-of --goal verbs-match-intent` (live m1e executable) lists the draft, the accepted parent and the superseded `plans/designs/verbs-match-intent.md`: the record is registered and readable.
- Tool calls used: 29 of 32. Read-only; no product file touched; conf.local not read.
- MATERIAL count: 3 (IP-C1, IP-C2, IP-C3). Verdict: REVISE the draft on three bounded contract corrections; the premise and the owner composition stand.

Criterion applied to every material item: would step 1 be built DIFFERENT or WRONG because of it; does step 1 WORK and is it SAFE without it. Everything below is a read of the named lines at this worktree, not a recollection.

## Premise check

The premise is grounded. `internal/dispatch/read_admission.go:228-252` scopes design-critic roots by `root["design"] == subject.DesignPath`, so a changed design keeps hitting the old root's CONCURRENT_READ/REDUNDANT_READ refusals until that chain folds; a fresh root would evade the round cap. Same-chain follow-up is the smallest honest continuation. `cmd/metasystem/intent_delivery.go:631-705` (`reviewDesign`) hashes the page into a generated brief and always dispatches `--role design-critic` fresh, as the record says. `scripts/agents/roles/` has no design-author role; `implementer.md:3,13` forbids design decisions and `plans/`; so the launch design lane (`internal/launch/launch.go:75-215`, `settings.go:17-34`, `metasystem.conf:212-213` runtime claude, model claude-fable-5-1) is the correct smallest owner. `internal/project` has `ParseRecord`, `Homes`, `HomeFor`, `NewID` and no writer (`project.go:198-275`, `record.go:130`); a focused publication operation there is right. Declared-output custody (`declared_outputs.go:43-120`) archives then `os.Remove`s a prior file at the declared path before the child runs, so the design's rule never to declare the live page as output is necessary and correct.

## Material findings

### IP-C1: build admission is the wrong admission for design authoring (MATERIAL)

The record says `design` "uses exactly the parent build route's lawful goal/approval/claim admission". That route is `unitRequest` in `cmd/metasystem/intent_work.go` (line 490 onward): it calls `prepareGoalWorktree(id)`, and `intent_work.go:670-690` refuses when no worktree has `goal/<id>` checked out. The parent contract (`intent-workflows.md:96-101`) has build acquire the goal claim, which starts the elapsed fence and attempt quota and makes any other seat a foreign holder. Meanwhile the existing critique admission, `reviewBriefFacts` (`intent_delivery.go`, shown above), needs only an approved goal with a review-round budget and takes no claim and no worktree.

Consequences if built as written: (1) requesting a design claims the goal for the author's session, so the builder on another seat is refused as a foreign holder until a `release`; (2) design authoring time burns the execution box's elapsed budget and attempts, a duplicate-spend risk on planning; (3) the author's WorkingDirectory becomes the goal worktree while `reviewDesign` resolves FILE against `inv.cwd` and the invoking checkout's homes (`intent_delivery.go:637-660`), so the "visible document" continuation can name a path that does not exist in the checkout the user is sitting in.

Different: yes, the admission owner, working directory and publication checkout all change. Works/safe without it: works for one seat in one checkout; wrong across seats and worktrees, and unsafe for budget.

Smallest correction: admission = the `reviewBriefFacts` admission (approved goal with review-round budget, session identity as the launch owner needs), no claim acquisition, no goal worktree. WorkingDirectory is the invoking checkout root (`roots.Checkout`); the staged output lives under `artifacts/agents/intent-design/<record>-<attempt>/draft.md` beside the existing `intent-review` precedent; publication targets the invoking checkout's design home. State whether design attempts count against the approval box's attempts (recommend: no, only against `launch.design.*` baselines). Fixture: `TestIntentDesignAuthorJourney` runs with no claim held, from a second worktree, and the continuation path resolves in the invoking checkout; add `TestIntentDesignAdmissionNoClaim` proving no claim record appears.

### IP-C2: the crash-before-supervisor custody refusal does not exist (MATERIAL)

The record: "A stable request reserves its launch ID before spawning; a crash rejoins that same launch under the supervisor's custody rules. An uncertain process-start outcome ... report the existing custody refusal until the owner can prove its state." Owner facts: `Store.Create` (`internal/launch/record.go:146-161`) refuses an existing ID with "launch already exists"; `Manager.Start` (`launch.go:75-215`) creates the record and calls `StartSupervisor` in one call, and the StartCap wait loop lives in that caller; `Census` (`launch.go:580-600`) reports `orphan-running` only for `State == Running`. There is no reserve-then-spawn API and no owner rule for a `Starting` record with `Supervisor == nil` and `Child == nil`. A crash between `Store.Create` and `StartSupervisor` leaves exactly that record: replay finds the bound launch ID, `Store.Read` succeeds, nothing is alive, census is silent, and a second `Start` with the same ID is refused. The "existing custody refusal" the record leans on is not there; the request is stuck with no public remedy.

Different: yes, the implementer must add the resolution or the replay hangs or refuses forever. Works/safe without it: the happy path works and no duplicate spend occurs; the crash path loses the attempt with no public recovery.

Smallest correction: in the launch owner, a `Starting` record with no supervisor whose `StartedAt + StartCap` has passed and whose supervisor/child refs are absent resolves to `Failed` with reason `supervisor-start-unrecorded` under the record lock (reuse `m.fail`); the public replay then reports failed attempt N and offers `design G --after N`. No new store. Fixture: `TestDesignRequestCrashBeforeSupervisor` writes a `Starting` record with no supervisor, replays the same request, asserts one failed N and zero new launches.

### IP-C3: `wait proof ATTEMPT` takes an input no public command shows (MATERIAL)

`cmd/metasystem/wait_verb.go:162` defines `--attempt "proof attempt identifier"`; the identifier is minted and retained inside `cmd/metasystem/proof_run.go:243-291` (`attempt.AttemptID`). A grep of `intent_delivery.go`, `intent_process.go` and `intent_selection.go` finds only the build attempt number (`intent_selection.go:105`, `workAttempt`) and never a proof attempt ID. Section "Orientation and retained waits" makes the wait discoverable but names no public source for ATTEMPT, so the actual task still needs internal knowledge, which is the requirement failure this iteration exists to close.

Different: yes, a public producer of the identifier is missing from the record. Works/safe without it: the wait executes; the task does not.

Smallest correction: `status G` and the landing result print the running proof attempt's identifier with `wait proof ATTEMPT` as the continuation; `TestIntentPublicProofAndFileWait` must obtain ATTEMPT from that public output, not from fixture knowledge. If landing exposes no attempt yet, drop `wait proof` from the public grammar this round rather than advertise a dead input.

## Notes (non-material, guesses an implementer will otherwise make)

- N1 first-creation path. "The project owner's default design-home path from the goal" names nothing in `internal/project`. Self-hosted layouts have three design homes (`project.go:198-215`: state-root `plans/designs`, checkout `plans/designs`, historical `*-design.md` glob). State the rule: first `KindDesign` home in `Homes` order, file `<goal>.md`, refuse with an explicit `--out` when that path exists and is not this goal's draft. Never the historical home.
- N2 design-author contract. `internal/launch/claude.go:32-60` sends the brief as stdin with no role preamble; `templates/design-brief.md` is a human template. The adapter must write a minimal author contract: reserved record ID and head grammar (`Kind/Id/Status: draft/Goals`), the staged output path, never write the live page or `plans/` otherwise, stop on a missing decision. Name it in the record so it is not invented per implementer.
- N3 attempt-N retention. "Store those facts in the existing launch record" and "per-document lock in its existing store" together imply a per-document entry like the named-unit entry (`unit_named.go:130-183`, `.inputs/<key>/request.json`). Say that precedent explicitly; otherwise "no second job index" reads as forbidding it.
- N4 binding line for a design subject. `intent_review_binding.go:33-47` requires `work=` and `attempt=`; a design has neither. Define `work=design:<recordID> attempt=<examination round>` or a distinct `Design review binding:` line. The subject and return digests already prevent a stale file resolving another review, so this is shape only.
- N5 `wait file PATH`. The legacy `--path` is "absolute path to observe" (`wait_verb.go:166`); the public form should resolve relative to cwd or say absolute.
- N6 root nine. `design` is absent from the nine root rows while it is the new step-1 capability; the parent allows focused-help discovery, but `open` and `design` should probably swap or share a row.
- N7 `stop design G` maps cleanly onto `Manager.Cancel` (`launch.go:532-570`), which proves processes dead before marking cancelled; good.

## Unexamined scope (not certified)

- Same-chain replay after completion. `dispatch.sh follow_up` admits a completed newest record by minting child `rN+1` (lines 2440-2446), takes `--operation-id` as an override (2415, 2586-2589) and replays on `PREFLIGHT-MATCHED` (2819-2820). Whether the preflight matches a frozen operation ID whose child already completed, and whether follow-up refreshes a design subject digest (`subject_temp` locals exist), I did not trace to a proof. `TestDesignCritiqueReplayAndCap` must cover replay after completion with a bumped goal revision and multiple legacy roots.
- Publication conditional write. `atomicfile.WriteText` exists (`launch.go:181`); a compare-then-write under a document lock is new code in `internal/project`. Not examined for the lock's identity across worktrees.
- `intent_review_binding.go` and `unit_revise.go` are uncommitted and changing under Opus; line numbers above are from this read.
- The design-critic follow-up message composition for a changed design (old findings plus new subject) was not read.

## Adjudication summary

Three material corrections, all contract-level, none adding a mechanism: use the critique admission instead of build admission and fix the working/publication checkout (IP-C1); replace the assumed custody refusal with one owner rule for an unsupervised `Starting` record (IP-C2); give `wait proof` a public input or withdraw it (IP-C3). With these folded the record supports step 1 and round 2 should be unnecessary.
