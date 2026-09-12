# engine-runs-re-arm-themselves-on-a-landed-engine: design

Goal engine-runs-re-arm-themselves-on-a-landed-engine (tier 2, origin human,
Wido 2026-09-12). One critique read of this page, then the build behind the
fixtures in section 5 under R-98-m1e, one code read, human commit; the field
proof is the next day of engine-driven runs on the seats.

## 1. The tax, and what already exists

`metasystem test run` resolves the trusted policy engine
(cmd/metasystem/test.go `trustedPolicyEngine`) by opening the enrolled
binary and proving that its source commit still owns the ENGINE projection at
the captured policy base (`VerifySourceAtDestination`,
internal/steward/rearm_resolver.go). The policy base is the local view of
the landing ref (`metasystem.steward.landing-ref`, refs/remotes/origin/main)
and nothing in the run fetches it, so the base is whatever the seat last
fetched; and when any seat lands engine code, the enrolled engine falls
behind the base's ENGINE projection and the run refuses with
`TEST_POLICY_ENGINE_REQUIRED`. The seat then does by hand what the landing
already proved: `git fetch origin && git reset --hard origin/main`,
`scripts/agents/go-build.sh`, `bin/metasystem up --repo <checkout>`. On m1b
that was twelve times on 2026-09-12; on m1e before every cadence run.

The re-arm itself already exists and is automatic: `metasystem up` opens the
enrolled binary, and when the bytes in bin/ were rebuilt from a landed
commit (`ErrEngineRebuilt`) it mints the next generation through
`steward.ReArmRebuiltEngine` ("machine-rebuild", the landing ref and the
landed commit recorded, the human witness carried), then re-verifies every
supervision component. That path requires the checkout's HEAD to be the
rebuilt engine's commit or a landed descendant with no engine or agent-script
change in between (`verifyEnrollmentBuildSource`). What is missing is the
three steps before it, done by the run instead of a person: fetch the landing
ref, bring the checkout to the tip, rebuild.

## 2. The mechanism: the run brings itself to the landed tip

Before `prepareTesting` captures the candidate index and resolves the policy
base, the outermost `test run` runs `landedRearm`. Only that verb: the
pinned child `test plan` the run execs for its policy decision, `test verify`
(which launches nothing and is called mid-landing) and the moved-inputs
printer judge the engine as they find it, so one run fetches once and no
child fast-forwards under its parent (critique M2).

1. **Fetch.** `git fetch <remote> <branch>` for the landing ref's remote and
   branch, bounded by the re-arm resolve seconds. When the fetch fails (no
   such remote, as in every fixture installation that writes the landing
   ref by hand; or the remote unreachable) the local ref is judged as it is
   today, and only a re-arm is refused, naming the fetch error: a re-arm
   from a tip that could not be fetched would be the stale base DONE
   forbids.
2. **Compare.** tip = the landing ref's commit after the fetch; head = the
   checkout's HEAD; source = the enrolled engine's source commit. If the
   enrolled engine still owns the ENGINE projection at tip
   (`VerifySourceAtDestination(tip)` passes), nothing happens: today's path,
   with the policy base now the fetched tip.
3. **Refuse what is not landed-only.** The run refuses with the same typed
   reason as today, naming source and tip and the one command
   (`git fetch origin && git reset --hard origin/main && scripts/agents/go-build.sh && bin/metasystem up --repo <checkout>`),
   when: head is not an ancestor of tip (local commits the remote does not
   have); the checkout is dirty in an ENGINE-projection path (index or
   working tree against HEAD, classified by the behavior-surface policy,
   the same classification go-build.sh uses for its dirty stamp); or the
   fast-forward below cannot be done. DONE clause 2, as written. Two more
   refusals from the critique: a delivery run that names `--tree` (the
   landing receipt captures the accepted index before the run and refuses
   a moved index after it, `PublishCommittedReceipt`), so such a run keeps
   the manual path and says so (M1); and a proof attempt of this
   installation without a terminal, because the engine is never rebuilt
   under a live attempt, the seats' box rule (M4).
4. **Fast-forward.** `git merge --ff-only <tip>` in the checkout. Dirty
   files outside the ENGINE projection survive a fast-forward that does not
   touch them; one that would is refused by git and reported under 3.
5. **Rebuild.** `scripts/agents/go-build.sh` in the installation, which
   builds beside bin/metasystem and renames over it (the running process
   keeps its own inode). Its stamp is the tip's commit, so
   `ReArmRebuiltEngine`'s landed-source proof holds by construction.
6. **Re-arm through the up path.** The run executes the REBUILT binary's
   own `bin/metasystem up --repo <checkout>` from the installation: the
   enrollment rules that mint the generation are the landed ones, not this
   process's older linked copy (code read, finding 5), and the invocation
   is exactly what a person types from this shell, with the same witnesses
   (the human witness of the prior generation is carried by the mint plan)
   and the same component re-verification. up mints the generation first
   and proves the session and the components after, so a run launched
   detached (no runtime ancestor, as `run launch` passes are) or beside a
   closed fence gets a failed up whose enrollment is already re-armed
   (critique M3). The run therefore judges the enrollment, not up's exit:
   it re-opens the enrollment and continues when the generation advanced,
   recording up's result line and outcome; an enrollment that did not
   advance is the refusal, with up's own words. Steps 4 to 6 run under the
   installation's proof mutation lock, the lock admission takes, and the
   live-attempt check is repeated under it, so no attempt is admitted while
   the engine is rebuilt (code read, finding 2).
7. **Restart on the landed engine.** The run re-executes itself
   (`syscall.Exec` of bin/metasystem with its own arguments) with the
   re-arm record in `METASYSTEM_ENGINE_REARM`, which is also the loop
   guard: the restarted run finds the record, never re-arms again, and
   carries the record into its attempt. The record is believed only when
   the enrollment bears it (that generation, that landed commit); a record
   the enrollment does not bear is ignored and the run judges the engine
   afresh (code read, finding 6), so an exported record cannot skip the
   fetch. So the whole run, the watchdog
   and the authenticated helpers it execs by path are one generation
   (critique M5). If the re-exec itself fails, the run continues on its
   own bytes with the new enrollment as its policy engine, as any run whose
   invoking binary is not the pin.
8. **Continue.** `prepareTesting` opens the enrolled binary (the new
   generation), resolves the policy base (now tip) and runs as today. The
   attempt's test result carries `engineRearm`: the source commit before,
   the tip, the generations, up's re-armed line and outcome, and the time.

## 3. What does not change

- The enrollment contract: the minted generation is `ReArmRebuiltEngine`'s,
  with its landed-commit and landing-ref proof; a human enrollment's
  witness carries as it does under `up`. No new authority.
- The first testing transition (no base contract) keeps using the running
  binary as the policy engine.
- Runs inside a delegate worktree: none exist (delegates never run proof
  verbs, delegate-proof-runs-inside-the-sandbox). A job worktree has no
  enrollment, so `OpenEnrolledBinary` fails before this mechanism.

## 4. Risks

- The dirty-engine classification is go-build.sh's: `git diff` against HEAD
  plus untracked files, ENGINE paths only. Both ignore files git ignores,
  so an ignored `.go` file under cmd/ or internal/ would be compiled into
  a build stamped as the tip's commit, by the manual path as by this one
  (code read, finding 1). The classification is shared and stays shared;
  the run pins `--no-relative`, `core.useReplaceRefs=false`, hooks off and
  maintenance off on every git call it makes (findings 3 and 4).

- The fetch adds a network round trip to every run (about a second on m1);
  a bounded fetch that fails refuses the run instead of proving against a
  stale base, which is DONE clause 1's "never a stale local HEAD" spelled
  for runs.
- A fast-forward moves the seat's HEAD under it while it works. It is what
  the seat does by hand today before every run; the dirty-engine refusal
  keeps a seat mid-landing on engine code on the manual path, where the
  three-way carry is a human decision.
- A re-exec that fails leaves the run on its old bytes with the new pin as
  its policy engine; that is the situation every run is in when its
  invoking binary is not the pin, and the pin is what judges.
- The landing receipt (`landing test-receipt`, a delivery run naming its
  tree) never re-arms; the seat that lands engine code keeps the manual
  path there, which is where the three-way carry is its decision anyway.

## 5. Fixtures

1. cmd/metasystem `TestLandedRearmDecidesFromTheThreeFacts`: the decision
   from (source owns tip, head is an ancestor of tip, dirty engine paths,
   a named delivery tree, live attempts): nothing to do; re-arm; refused
   for a diverged head; refused for a dirty engine path; refused for a
   delivery run naming its tree; refused under a live attempt; each
   refusal naming source, tip and the command.
2. cmd/metasystem `TestLandedRearmReadsTheCheckoutAgainstItsRemote`: a
   fixture checkout with a local remote; the remote lands a commit that
   changes an engine input (a fake landing); the reader reports head behind
   tip, head an ancestor, the checkout clean; after a local commit the
   remote lacks, head is not an ancestor; after an edit to an engine path,
   the path is listed as dirty; an edit outside the projection is not.
3. cmd/metasystem `TestLandedRearmFastForwardsRebuildsAndReArms`: the same
   fixture with the rebuild, the up call and the enrollment read behind
   seams; the run fast-forwards the checkout to the tip, calls the rebuild
   in the installation, calls up with the checkout's options, and records
   the re-arm with the new generation; a failed up over an enrollment that
   advanced still continues; an enrollment that did not advance refuses
   with up's remedy; a refused decision calls none of the acts. The re-exec
   is not fixtured: it is the field proof's first line.
4. The next day's receipts and diagnostics on this seat (DONE clause 4) are
   the field proof, recorded on this page's section 6 by the seat.

## 6. Field record

To be written after the first day.
