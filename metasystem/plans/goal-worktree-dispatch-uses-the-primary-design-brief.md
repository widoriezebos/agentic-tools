# Design brief: goal-worktree-dispatch-uses-the-primary

## Revision

Revision: first draft

Reason: goal `goal-worktree-dispatch-uses-the-primary` (tier 2, approved, priority 1, pinned to seat m1g and claimed by it) has no design record. The page settles one rule for which engine, runtime registry and settings a dispatch made from a goal worktree uses, and the three checks that read the wrong place today: the engine skew check, the brief check at review, and the brief check of a session continuation.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

### What the goal asks (the goal record, verbatim parts)

Intent: "Every dispatch made from a goal worktree (a critic read, a commit read, a builder round, a review close) runs with the seat's armed primary engine and the primary's runtime registry and settings; the worktree is only the tree under review, never the tool. A brief the build admitted is never refused at review, and a review of a frozen unit can take a corrected brief without a builder round. Generic: holds for any adopter checkout layout; nothing names this repository."

Next step, the smallest thing that works as the record states it: "one resolution rule in internal/dispatch for engine, registry and settings at dispatch time (primary wins), the skew check against main, and the brief check at build time only with a --brief correction path at review. Design 1,000 words, one Astra round, then build with Codex Sol and a Claude Opus critic; estimate 1 h design, 4 h build, 2 lane cycles."

The seven faces the record lists are the use cases. Each was seen on 2026-10-03/04 at a goal worktree, a linked Git worktree of the seat's primary checkout (here `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g-GOAL`, primary `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g`):

1. Engine. "m1g's critics ran the worktree's bin/metasystem built from the goal branch, so item P's fix on main never reached them until m1g copied the primary engine in by hand."
2. Registry. "m1k's critic was refused 'runtime adapter is not installed: codex' because the worktree installation has no built engine and no registry." Cause, read in code by seat m1l: "the critic dispatch from a goal worktree runs with root = the worktree's metasystem/, and delegationEngine (cmd/metasystem/delegate.go:314-318) defaults the engine to ROOT/bin/metasystem, which a goal worktree does not have (or has as a stale branch build); the adapter's Installed check (internal/delegation/owners.go:227) runs 'ENGINE delegate-supervisor codex signature' against it and fails. METASYSTEM_BIN (delegate.go:315) already overrides the engine; with the primary's engine the same check passes for either root." The hand route used tonight is `METASYSTEM_BIN=<primary>/bin/metasystem` on the review command.
3. Settings. "a hand-in run from a goal worktree reads landing.review.human-from-tier as the shipped default 2 ('2 · default': the worktree has no metasystem.conf.local), while every seat's primary says 4 by Wido's ruling of 2026-10-02; so a tier 2 unit with a clean critic read 'waits for a person's word before it lands'." Hand route: run `work land` from the primary.
4. Skew check (item 55), read in code by seat m1f: "engineSkewPreflight (internal/delegation/infra.go) runs 'git log --ancestry-path <engine stamp>..HEAD' in the goal worktree, so once a goal branch sits on top of the main commit the seat's engine was built from, the goal's own unit commits count as engine skew whenever they touch engine code ('the engine is older than this checkout, and engine scripts changed'); every rebased goal branch hits it, and the only way past is to rebuild the primary at a main commit that is not an ancestor of the branch. Rule: the check compares the engine against main-side commits only (merge-base of the branch with main), never against the goal's own units; the goal's engine-code changes are the subject under review, not the tool."
5. Frozen brief at review (item S): "m1l's partner-mode review is refused BRIEF_AUTHORITY_REFUSED on a frozen build brief the build itself admitted (plans/handoff-project-partner.md cited without 'new file:'), both --work and --commit forms."
6. Session continuation: "the handoff continuation of a seat session was refused by the same brief-authority check ('the brief cites paths the delegate's tree does not hold: artifacts/agents/context/handoffs/<id>/state.json'), the path being the handoff's own state file, which lives under the seat's artifacts and not in the delegate's tree; the steward then started a fresh session without the continuation, so the successor lost the handoff's context. Rule: the brief check admits paths under the seat's own artifacts and state directories (they are the brief's inputs, not the tree under review), and a refused continuation reports the path class so the steward can repair it instead of discarding the handoff."
7. The Codex sandbox denials (faces four, five and seven of the record: the Go build cache, the Go temp directory, the process listing the test fixtures need).

### Scope: the sandbox faces are another goal's

Face 7 is out of this page. The ledger gives it to item AG of goal `machinery-blocks-of-2026-10-04` (seat m1j), recorded there on 2026-10-04 08:50: "a host setting (default off, on for this trusted host) under which builder and critic jobs run with --sandbox danger-full-access (the engine's commit guard, declared outputs and the worktree remain the protection), applied in both the adapter mapping and the launcher; one test per path. Generic: the setting is per host, named by the adopter. This removes faces 4, 5 and 7 of goal-worktree-dispatch-uses-the-primary and ends the blind Codex builds." The coordinator seat confirmed the split on 2026-10-04 09:10: this page covers engine, registry, settings, skew check, frozen brief and the continuation path, and refers to item AG for the sandbox in one line.

### The rule that already exists, and where it is already applied

`landpath.SystemInstallation` (excerpt below) maps an installation to the one whose running system serves it: an armed installation runs its own system; an unarmed linked worktree's system is the same installation in its primary checkout. Four recent fixes applied it one reader at a time, each for a single symptom at a goal worktree:

- f8c7b2b20 "delegation: a goal worktree's census gate reads its primary checkout's system" (`supervisedInstallation`, excerpt below).
- 19fb3a6c9 "delegation: an armed linked worktree keeps reading its own census".
- 3bd13e559 "delegation: an unarmed goal worktree judges holder authority by its primary's lease".
- d294e76fd "review: collect a goal's commit read where its branch lives, from its critic's store" (`branch.CriticStore`, `internal/goal/branch/read.go:371-419`).

The engine, the runtime registry and the settings still read the dispatch root itself. The page should say whether one resolver now owns all of them, which readers move to it, and which stay on the root because they are the tree under review (the reviewed commit, the diff, the worktree the delegate edits).

### Generic

The goal says nothing may name this repository. The design critic will test the page against an adopter whose project is Java, with no npm and no Go on the host: that adopter's engine is the installed `bin/metasystem` binary in its installation folder, which its goal worktrees do not hold because the binary is not committed; its settings live in its own installation's `metasystem.conf` and `metasystem.conf.local`. The rule must not depend on `go run ./cmd/devgate build`, on the installation folder being called `metasystem`, or on this repository's engine sources; the skew check watches the installation-relative trees `internal` and `cmd` (`steward.EngineSkewPathspecs`, `internal/steward/rearm_resolver.go:512-517`), the engine's own sources; say what the check means for an adopter whose installation holds no engine sources.

### Facts the page should settle

- What "the primary" is when the dispatch root is already the primary, an armed linked worktree, or a worktree Git cannot map (keep the root, as `SystemInstallation` does).
- Whose committed `metasystem.conf` a dispatch reads when the goal branch itself changes that file (the change is the subject under review; the tool's settings are the seat's).
- What `METASYSTEM_BIN` means afterwards (today an override of the engine for any root).
- Whether the skew check needs the main ref, the merge-base, or only the engine's stamp being an ancestor test against main-side commits, and what it does when the goal branch has no merge-base with the local main (fetch state, shallow clone).
- For the brief at review: what is checked at build, what a critic's brief quotes as frozen, and how `--brief` gives a review of an already built unit a corrected brief without a builder round (the record's words). Today `work review G --work W` takes no `--brief`; the `--changes`, `--patch` and `--commit` forms do.
- For the continuation: which directories count as the seat's own (artifacts and state of the installation whose system serves the root), and what the steward does with a refusal.
- Units in landing order, each with its witness test by name and the mutation that turns it red. Tests stub Git and processes through existing seams; no real Git in behaviour tests.

### What the code does today (read at main c806e15cf)

Nothing in configuration, the runtime registry or engine choice falls back from a goal worktree to its primary; only the census, the lease and the critic store do.

- Engine. `delegationEngine(root)` (excerpt) is `METASYSTEM_BIN`, else `ROOT/bin/metasystem`; its one caller is `newDelegationLifecycle`. `delegation.NewOwnerPorts` repeats the default (excerpt), and the adapter supervisor's `ProcessDeps(root)` repeats it again inside the supervisor process (excerpt), with root its `--root`. A critic dispatched for a goal's work runs the delegate with `rootOverride` the goal worktree's installation (`cmd/metasystem/goal_branch.go:160-170`). A builder round's launch supervisor is `os.Executable()` (`internal/launch/process.go:141`), the engine that ran the command.
- Registry. "runtime adapter is not installed" is raised by `selectSnapshot` (`internal/delegation/admission.go:466`) when `Installed` fails (excerpt): it runs `ENGINE delegate-supervisor RUNTIME signature --root ROOT`. Inside the supervisor, `external.Load(root)` (`internal/runtimes/external/external.go:547`) reads the compiled-in runtimes plus `ROOT/adapters/NAME` and `adapters.NAME.use` from `ROOT/metasystem.conf` and its `.local`.
- Settings. Every setting resolves by layers: flag, environment, `metasystem.conf.local`, `metasystem.conf`, compiled default (`internal/config/resolve.go:303-363`). The landing gate reads them at `ROOT/metasystem.conf` (excerpt) with root the selected installation; the unit launcher's `launch.build.*` and role settings read `ROOT/metasystem.conf` too (`cmd/metasystem/intent_work.go:88-100`, `internal/launch/settings.go:138`). The one bridge today is `criticDelegateEnvironment` (excerpt): it resolves the primary's code-critic roster, `metasystem.runtimes` and the maximal-model mapping and passes them to the worktree's dispatch as per-key environment settings, which outrank every file.
- Skew check. `engineSkewPreflight` (excerpt) lists `git log --ancestry-path STAMP..HEAD` in the dispatch root's repository and refuses when a listed commit touches `internal` or `cmd` under the installation.
- Brief at dispatch. `briefAuthority` (excerpt) admits a brief against the reviewed commit and the dispatcher's HEAD, with runtime paths (`artifacts/...`, Git-ignored paths) looked up on the disk of `s.repoScope`, the dispatch root's work tree (`internal/dispatch/brief.go:384-412`). The dispatch phase calls it for every dispatch (excerpt). A critic's brief embeds the unit's build brief as "Supplied accepted implementation brief (frozen at dispatch)", so every path the build brief cites is checked again at review, against the goal worktree's disk.
- Continuation. The steward's handoff brief (`internal/steward/stage.go:139-176`) cites the handoff's state file relative to the repository root (`internal/steward/handoff.go:36-45`); the revive runs `internal delegate --revive`, becomes `dispatch --steward-intent` (`cmd/metasystem/delegate.go:439-443`, `internal/delegation/dispatch_phase.go:118-137`), and meets the same `briefAuthority` check. A `--follow-up` continuation has its own check (`internal/delegation/followup.go:375`).

Critique findings being answered:

1. none (first draft)

Cited code excerpts:

1. `cmd/metasystem/delegate.go:312-319`

   ```text
   // delegationEngine is the installation's engine the lifecycle launches and
   // records: METASYSTEM_BIN when set, else the installation's own.
   func delegationEngine(root string) string {
   	if engine := os.Getenv("METASYSTEM_BIN"); engine != "" {
   		return engine
   	}
   	return filepath.Join(root, "bin", "metasystem")
   }
   ```

2. `internal/delegation/owners.go:61-68`

   ```text
   	root, err := filepath.Abs(config.Root)
   	if err != nil {
   		return Ports{}, err
   	}
   	engine := config.Engine
   	if engine == "" {
   		engine = filepath.Join(root, "bin", "metasystem")
   	}
   ```

3. `internal/delegation/owners.go:221-228`

   ```text
   	return append([]string{o.engine}, args...), nil
   }

   // Installed reports whether the installation resolves the runtime to an
   // adapter: the entry's signature read succeeds (dispatch.sh's
   // `delegate-supervisor RUNTIME signature` check).
   func (o ownerAdapter) Installed(runtime string) bool {
   	_, err := o.run(context.Background(), runtime, "signature")
   ```

4. `internal/adapter/supervisor/deps.go:101-106`

   ```text
   // ProcessDeps builds the production seams for an installation root.
   func ProcessDeps(root string) Deps {
   	engine := os.Getenv("METASYSTEM_BIN")
   	if engine == "" {
   		engine = filepath.Join(root, "bin", "metasystem")
   	}
   ```

5. `cmd/metasystem/landing_gate.go:33-36`

   ```text
   // landingGateSettings resolves the two settings of the installation at root.
   func landingGateSettings(root string) (goal.GateSettings, error) {
   	return goal.ResolveGateSettings(filepath.Join(root, "metasystem.conf"))
   }
   ```

6. `cmd/metasystem/goal_branch.go:663-682`

   ```text
   // criticDelegateEnvironment carries the selected installation's configured
   // code-critic roster to a critic dispatched from another installation (a
   // generated goal worktree, whose tracked roster may be a template and which
   // never has the selected checkout's metasystem.conf.local). The roster is
   // resolved by the roster owner in the selected installation for the working
   // mode dispatch reads from the same frozen brief, and is passed as the
   // configuration owner's per-key process settings. Those outrank every file
   // entry, mode-scoped or not, so dispatch resolving that mode in the worktree
   // obtains exactly this pair as its configured default rather than an
   // override: an explicit --runtime/--model still escalates by the unchanged
   // roster policy. The selected installation's maximal-model mapping for that
   // runtime is carried as its exact value (empty when it has none), so the
   // worktree's hazard check admits or refuses the selected model by the
   // selected installation's authorization. An unreadable brief mode or a
   // selected roster that does not resolve is refused before any dispatch.
   func criticDelegateEnvironment(selected, root, brief string) ([]string, error) {
   	if selected == "" || filepath.Clean(selected) == filepath.Clean(root) {
   		return nil, nil
   	}
   	mode, err := dispatchcore.BriefModeOnly(brief)
   ```

7. `internal/landing/landpath/precommit.go:276-304`

   ```text
   // SystemInstallation names the installation whose running system serves
   // the installation at root, and that installation's checkout. An
   // installation armed itself (its own arming record or census exists) runs
   // its own system, and checkout is "". Otherwise a linked worktree's (a goal
   // worktree's) is the same installation in its primary checkout
   // (PrimaryInstallation), with symbolic links resolved. A root git cannot
   // map keeps its own, and checkout is "". git runs in root.
   func SystemInstallation(git func(args ...string) GitResult, root string) (installation, checkout string) {
   	own := filepath.Join(root, "artifacts", "agents", "supervision")
   	for _, name := range []string{"state.json", "last-census.json"} {
   		if _, err := os.Lstat(filepath.Join(own, name)); err == nil {
   			return root, ""
   		}
   	}
   	checkout, installation, problem := PrimaryInstallation(git, root)
   	if problem != "" || installation == "" {
   		return root, ""
   	}
   	if resolved, err := filepath.EvalSymlinks(installation); err == nil {
   		installation = resolved
   	}
   	if installation == root {
   		return root, ""
   	}
   	if resolved, err := filepath.EvalSymlinks(checkout); err == nil {
   		checkout = resolved
   	}
   	return installation, checkout
   }
   ```

8. `internal/delegation/infra.go:45-82`

   ```text
   // engineSkewPreflight refuses a dispatch whose engine is older than the
   // checkout when engine or agent sources changed since the engine's commit.
   // The stamp is the running engine's build stamp unless a focused fixture
   // supplies one.
   func (s *session) engineSkewPreflight(stamp string) error {
   	if stamp == "" {
   		stamp = supervise.BuildStamp
   	}
   	if stamp == "" || stamp == "dev" {
   		return nil
   	}
   	out, _, err := s.l.ports.Git.Run(s.ctx, s.repoScope, "log", "--format=commit %H", "--name-only", "--ancestry-path", stamp+"..HEAD")
   	if err != nil {
   		return nil
   	}
   	text := string(out)
   	checkoutCommit := text
   	if index := strings.IndexByte(text, '\n'); index >= 0 {
   		checkoutCommit = text[:index]
   	}
   	checkoutCommit = strings.TrimPrefix(checkoutCommit, "commit ")
   	prefix := ""
   	if s.root != s.repoScope {
   		prefix = strings.TrimPrefix(s.root, s.repoScope+"/") + "/"
   	}
   	relevant := false
   	for _, line := range strings.Split(text, "\n") {
   		for _, tree := range steward.EngineSkewPathspecs() {
   			if strings.HasPrefix(line, prefix+tree+"/") {
   				relevant = true
   			}
   		}
   	}
   	if checkoutCommit != "" && relevant {
   		return s.die(1, fmt.Sprintf("dispatch refused: the engine (%s) is older than this checkout (%s), and engine scripts changed\nrebuild with go run ./cmd/devgate build, then arm the steward again", stamp, checkoutCommit))
   	}
   	return nil
   }
   ```

9. `internal/delegation/infra.go:114-125`

   ```text
   // supervisedInstallation names the installation whose running system
   // supervises this session's root, and its repository scope, by the one
   // rule landpath.SystemInstallation states: an armed installation runs its
   // own system; an unarmed linked worktree's is its primary checkout's. A
   // root that keeps its own must then have its own census.
   func (s *session) supervisedInstallation() (root, repo string) {
   	installation, checkout := systemInstallation(s.ctx, s.l.ports.Git, s.root)
   	if checkout == "" {
   		return s.root, s.repoScope
   	}
   	return installation, checkout
   }
   ```

10. `internal/delegation/dispatch_phase.go:372-384`

   ```text
   	authorityBase := s.repoScope
   	if a.useWorktree && isDir(filepath.Join(s.worktrees, job)) {
   		authorityBase = filepath.Join(s.worktrees, job)
   	} else if a.workspaceSelected {
   		resolved, err := physicalDir(a.workspace)
   		if err != nil {
   			return s.die(1, "workspace does not exist: "+a.workspace)
   		}
   		authorityBase = resolved
   	}
   	if err := s.briefAuthority(a.brief, authorityBase, a.reviews); err != nil {
   		return s.die(1, "brief authority admission refused: "+err.Error())
   	}
   ```

11. `internal/delegation/admission.go:609-616`

   ```text
   // briefAuthority is brief_authority. A brief that reviews a commit is also
   // read against that commit's tree, which the critic reads.
   func (s *session) briefAuthority(brief, baseTree, reviews string) error {
   	_, err := dispatch.ReadReviewBriefAdmission(brief, s.root, baseTree, s.repoScope, reviews)
   	if err != nil {
   		s.noteRefusal(err)
   	}
   	return err
   ```

12. `internal/dispatch/brief.go:144-155`

   ```text
   // ReadReviewBriefAdmission admits a critic's brief against the trees the
   // critic reads: the reviewed commit when reviews names one
   // ("commit:<sha>", the unit under review, which may hold files main does
   // not), and the dispatcher's HEAD, whose checkout is the critic's
   // workspace. A cited path either tree holds is admitted; a reviewed commit
   // the repository does not hold refuses.
   func ReadReviewBriefAdmission(briefPath, installRoot, baseTree, diskRoot, reviews string) (BriefAdmission, error) {
   	reviewed, named := strings.CutPrefix(reviews, "commit:")
   	if !named {
   		reviewed = ""
   	}
   	return readBriefAdmissionWithFacts(briefPath, installRoot, baseTree, diskRoot, reviewed, false, gitBriefTreeFacts{})
   ```

Example page:

`/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/agent-defaults-follow-the-host.md` (structure and level of detail): short, in Wido's plain English, what exists first, then what changes, then units with their witness tests and mutations, then what is out of scope and deferred.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 60

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 1500 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead. The goal record asks for about 1,000 words; scope comes first on the page.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md

The page opens with its head, exactly this shape, the blank line after the title included:

```text
# The page's own title

- Kind: design
- Id: a new 26-character ULID that you generate
- Status: draft
- Goals: goal-worktree-dispatch-uses-the-primary
```

Do not edit any other file, run test suites, build engines, claim or release goals, or commit.

Return only these two lines, N being the page's word count:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md
DESIGN: ready (N words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md
DESIGN: blocked (the reason in one line)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.
