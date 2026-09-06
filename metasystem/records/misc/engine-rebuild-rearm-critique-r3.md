# Engine re-arm design critique — round 3 (revision 3)

Chain: revision 3 (landed ad84c90cd, sha256 8cb59f6981a65a613a8a5c6a687e5d50c66a6d406b046cd432cee54985bf7bca) -> critic err-crit3-20260906 (Fable, design-critic, read-only runtime; the critic could not write this file, so the coordinator carried its return here verbatim). verdictMaterialCount=5.

## ERAR-R3-01-STOP-STAGE-HOLES — high, material=True

CLAIM: ERAR-R3-01-STOP-STAGE-HOLES — Decision 4 still cannot truthfully report every destructive pre-mint outcome. StageStopping is defined only for an error returned by stopRunnerForReplacement and is described as proving that the restart marker was written and the runner was signalled. The function can instead fail while writing the marker before any signal, or because SIGTERM or SIGKILL was not delivered. There is a second uncovered window: if the runner stops successfully and MintIdentity then fails, the proposed state remains StageBeforeMint even though that stage promises that no runner stopped. An implementer must preserve a stop-attempt or stopped state through mint failure and expose which marker and signal effects actually occurred; Decision 4's rendering and fixture F12 must cover marker-write failure and successful-stop-then-mint-failure.

EVIDENCE: Design section: metasystem/plans/engine-rebuild-rearm-design.md:323-340, 376-397, 445-450, 731-748, and 788-812. Code evidence: metasystem/internal/steward/runner.go:479-508 returns at marker creation, termination, kill, and remained-alive errors. Build change if this stands: change ArmStage or armOutcome, the ordinary outcome mapping, and F12 so no return falsely says nothing changed or that a signal was delivered.

## ERAR-R3-02-IDENTITY-NOT-DURABLE — high, material=True

CLAIM: ERAR-R3-02-IDENTITY-NOT-DURABLE — Decision 3 reports the re-arm fact before its record of truth is crash-durable. Calling MintIdentity an atomic rename proves reader atomicity, not persistence: the current function performs neither a temporary-file sync nor a parent-directory sync. The repository already distinguishes a durable publication from a committed publication whose durability is unknown. The design must use that contract or specify an equivalent one, make successful reporting conditional on confirmed durability, and define the post-rename directory-sync-failure outcome because the new generation may be visible but not proven durable.

EVIDENCE: Design section: metasystem/plans/engine-rebuild-rearm-design.md:604-639 and 951-973. Code evidence: metasystem/internal/steward/identity.go:50-62 has only WriteFile and Rename; metasystem/internal/atomicfile/atomicfile.go:1-22 and 83-113 defines the repository's file-sync, rename, directory-sync, and committed-with-doubt outcomes; metasystem/internal/steward/intervene.go:297-313 already applies it to the notification. Build change if this stands: change identity publication and arm's stage/result contract, plus add fault-injection proof for pre-publication and post-publication synchronization failures.

## ERAR-R3-03-LANDING-REF-NOT-OWNED — high, material=True

CLAIM: ERAR-R3-03-LANDING-REF-NOT-OWNED — The proposed landing ref is neither demonstrably installation-scoped nor assigned an exclusive software owner. git config --get reads all configuration scopes, so a global or included value can satisfy an installation whose local key is unset. The selected refs/heads branch is then movable by ordinary Git operations; the arm flock does not coordinate those writes, and the landing wrapper is not its only mover. Consequently a stray same-user process can change the configuration or ref and make a previously unlanded stamp reachable. Failing closed for an absent or dangling ref is correctly specified, but it does not fix this authority gap. The build needs a local-scope read and an explicit movement contract, with a fixture showing that global configuration and non-owner ref movement cannot grant eligibility—or the design must explicitly concede that they can under its accident threat model.

EVIDENCE: Design section: metasystem/plans/engine-rebuild-rearm-design.md:131-186, 418-430, 769-773, and 926-940. Code evidence: metasystem/scripts/agents/commit.sh:514-525 advances the current local branch at commit time and only optionally pushes at lines 547-568; metasystem/scripts/agents/land.sh:232-236 merely selects the current branch. The ran git config --list --show-origin --show-scope command showed system, global, and local inputs, while the proposed read has no --local selector. Build change if this stands: change configuration lookup, define and enforce the ref's movement owner or dedicated namespace, and extend F8 with scope and movement cases.

## ERAR-R3-04-ARBITRARY-HISTORY-CAP — medium, material=True

CLAIM: ERAR-R3-04-ARBITRARY-HISTORY-CAP — The newest-64-commits rule adds an eligibility restriction that the human decision did not authorize and the design does not justify. A witness-stamped engine whose matching commit is reachable but is the sixty-fifth candidate is landed by the governing rule, yet C20 refuses it. Saying that the ordinary case usually matches the tip and that refusal is loud does not establish why 64 is safe, and F11 currently locks the arbitrary refusal into the build. The resolver needs an evidence-based resource contract that preserves all reachable eligible commits, such as an exhaustive or indexed lookup with a measured operational bound; otherwise the design must obtain an explicit narrowing decision.

EVIDENCE: Design section: metasystem/plans/engine-rebuild-rearm-design.md:220-228, 811, and 926-928. Code evidence: metasystem/scripts/agents/go-gate.sh:497-502 stamps the ENGINE digest without a history-window condition, and metasystem/internal/behaviorsurface/policy.go:330-430 defines the digest without such a bound. Build change if this stands: alter resolveWitnessStamp, C20, the cost/failure contract, and F11's sixty-fifth-commit assertion.

## ERAR-R3-05-UNJUSTIFIED-SEEDS — medium, material=True

CLAIM: ERAR-R3-05-UNJUSTIFIED-SEEDS — Migration chooses refs/heads/main for every live fleet machine and as the fleet-bootstrap default without proving that it is each installation's landing branch. The current checkout being on main proves only this checkout. It also conflicts with the design's reason for withdrawing a hard-coded main ref and with the shipped landing wrapper, which selects the checked-out branch. Adoption's proposed current-branch seed is justified, but fleet bootstrap's main default is not; moreover the referenced bootstrap design currently defines neither --landing-ref nor this seed. Each installation must derive its checked-out branch as adoption does or require an explicit ref, and seed behavior needs a non-main fixture.

EVIDENCE: Design section: metasystem/plans/engine-rebuild-rearm-design.md:832-848. Code evidence: metasystem/scripts/agents/land.sh:232-236 and metasystem/scripts/agents/commit.sh:550-556 select the current branch; metasystem/scripts/adopt.sh:381-389 is the existing local-configuration seeding location; metasystem/plans/fleet-join-bootstrap-design.md:209-221 currently owns only machine identity and notification configuration. Build change if this stands: change migration/bootstrap defaults and add adoption and join fixtures proving a non-main installation receives the correct fully qualified ref.

## Gaps the critic named

- The witness resolver and fixture F11 do not exist at the reviewed commit, so F11 could not be executed. The digest walk, prefix mapping, and current Git pathspec member set were checked directly; F11 would falsify a mismatch if implemented literally.
- Only the m1 checkout was available. The actual branches and installation-local Git configuration on the other fleet machines were not observable, so the claim that refs/heads/main is correct for every live machine remains unproved.
- The declared new Markdown record metasystem/records/misc/engine-rebuild-rearm-critique-r3.md was not created because the critic role forbids repository edits and this runtime is read-only. This return contains the complete record.
- Revision 3 has not reached the design-critique stopping point: five findings would change the build. The witness digest reconstruction itself and the every-Result wrapper shape need no further design change based on the evidence read.

## Coordinator dispositions (m1, 2026-09-06)

Revision 3 is the third prose revision and this is the third critique. Every
finding above changes what gets built, and every one of them is answerable
as a build obligation with a fixture rather than as a fourth revision, so
the chain moves to the build with the five findings folded into the build
brief (the implementation-first ruling: after two prose budgets, build
behind fixtures). The design chain itself cannot close today (goal
design-chain-has-no-lawful-close); the build cites this record by id.

- ERAR-R3-01-STOP-STAGE-HOLES: the stages become BeforeMint (nothing
  changed), StopAttempted (marker written or a signal attempted; the runner
  may or may not be gone), Stopped (runner confirmed gone), Minted. A mint
  failure after a stop reports Stopped, never BeforeMint; a marker-write or
  signal failure reports StopAttempted with the failing step named. One
  fixture per transition.
- ERAR-R3-02-IDENTITY-NOT-DURABLE: MintIdentity publishes through the
  repository's existing durable-publication contract (temporary file
  synced, renamed, parent directory synced). Success is reported only on
  confirmed durability; a directory-sync failure after the rename is its
  own named outcome (visible, durability unproven) that health shows and
  the next arm re-checks.
- ERAR-R3-03-LANDING-REF-NOT-OWNED (coordinator decision, within Wido's
  word "an owned landing ref"): the key is read with the local scope only
  (git config --local); a global or included value never satisfies it. The
  ref lives in a metasystem-owned namespace, refs/metasystem/landing/<branch>,
  and is moved fast-forward only by the engine's own push paths (the landing
  wrapper and the goal verbs, which are the only lawful pushes to the
  canonical branch) after their push succeeds. Ordinary Git operations
  (checkout, commit, pull, rebase, reset) never move it. The boundary is
  stated honestly: a hostile same-user process can move any ref or write
  the identity file directly, and no software boundary in this design
  claims otherwise; the threat the rule closes is an unlanded build
  re-arming through ordinary operation.
- ERAR-R3-04-ARBITRARY-HISTORY-CAP: no count cap. The resolver walks every
  commit reachable from the landing ref, newest first, with a digest cache
  keyed by tree hash so repeated arms cost one walk, and stops at the first
  match. The walk is bounded by wall time (config key, default 20s); expiry
  is a loud refusal naming the bound, never silent. F11 gains the case
  where the matching commit is the sixty-fifth candidate.
- ERAR-R3-05-UNJUSTIFIED-SEEDS: no fleet-wide main seed. The human arm and
  restart verbs seed metasystem.steward.landing-ref from the checked-out
  branch when it is unset and say so in their output (adoption's rule);
  the machine path never seeds and refuses when the key is unset (fail
  closed, as revision 3 already specifies). Fleet bootstrap gets its
  --landing-ref when that design lands; until then the arm-time seed
  covers every installation.

## Coordinator decisions on the builder's gaps (m1, 2026-09-06, build round 1)

The first build round (err-build1-20260906) stopped under the gap rule
before writing a byte: three of the dispositions above left a contract
undefined. Decided here, on the code as it stands.

- Wall-time bound for witness resolution (ERAR-R3-04): the key is
  metasystem.steward.rearm-resolve-seconds, an integer number of seconds,
  read exactly as TickSeconds reads metasystem.steward.tick-seconds
  (internal/steward/runner.go: git config --get on the installation,
  positive integer, anything else falls back to the default). Default 20.
  The arm's output names the effective budget. Expiry is the loud refusal
  the disposition already names; an unparsable value is not a refusal, it
  is the default, the same as the tick cadence.
- Persisted durability doubt on the identity (ERAR-R3-02): reuse the
  mechanism the steward's component evidence already has
  (internal/steward/component_evidence.go: the durability-pending marker
  beside the record, written before the publication, cleared after a
  durable one; health.go reports DURABILITY_PENDING while it stands).
  MintIdentity publishes through atomicfile.WriteText and applies the same
  pattern to the identity file: a marker beside identity.json (same
  suffix, .durability-pending) is written before the publish and removed
  only after WriteText reports durable=true. If the publish reports
  durable=false the marker stays; health shows the steward identity as
  DURABILITY_PENDING with the generation it names; the next arm, human or
  machine, re-publishes the identity content through the same call under
  the arm lock before its eligibility decision and clears the marker when
  that publish is durable. A marker with no identity file beside it (the
  crash the doubt warned of) is removed and reported as unenrolled through
  the existing no-identity path. No new record, no new file format.
- Mapping into the result contract (ERAR-R3-01/02): the stage stays
  Minted - the identity is visible. ReArmOutcome gains one typed boolean,
  DurabilityPending, carried by every Result that carries the re-arm fact;
  Status stays the success status so launches gated on it proceed; the
  hook and health lines read "re-armed generation N (durability pending)"
  while the marker stands. No new stage, no new error type.

## Coordinator decisions after code critique round 2 (m1, 2026-09-06)

The second code critique (err-review2-20260906) found two high defects,
both in the coordinator's own dispositions above, not in the design or
the builder's reading of it. Corrected here.

- ERR-CC2-02 overturns the landing-ref decision under ERAR-R3-03. A ref
  moved only by this clone's own pushes is never ahead of a pulled
  landing, so every seat except the one that landed a Go change would be
  refused after the ordinary pull, rebuild, up. The owned landing ref is
  the REMOTE-TRACKING ref of the canonical branch (refs, then remotes,
  then the remote name, then the branch: what git writes as
  refs/remotes/origin/main). It is owned by the remote: a fetch or pull
  moves it to the remote's truth and nothing else lawful moves it -
  local commits, rebases and resets cannot, and the only pushes to the
  canonical branch are the landing wrapper and the goal verbs, which
  update it on push. Eligibility is therefore "ancestor of the
  remote-tracking ref", which a pulled landing satisfies and an unpushed
  local commit never does. Consequences: the refs/metasystem/landing
  namespace and the post-push advance in the goal transaction and the
  landing wrapper are removed with their tests and fixture legs; the key
  metasystem.steward.landing-ref holds a remote-tracking ref and nothing
  else (the machine path refuses closed on any other value and names it);
  the human arm and restart verbs seed it from the checked-out branch's
  upstream (git's @{upstream}) and print that they did; no upstream or a
  detached HEAD arms without seeding and says why. The witness walk
  enumerates commits reachable from that ref. The fixture bed creates the
  ref by hand in its scratch repositories as it did before.
- ERR-CC2-01: the dirty-tree stamp missed untracked engine files because
  the two git enumerations print paths from different roots (diff from
  the toplevel, ls-files from the current directory). The untracked
  enumeration prints full names so both lists carry the toplevel prefix
  the policy selector expects; a fixture proves an untracked Go file
  under the engine surface yields the dirty stamp in the nested layout.
- Notes folded because they are cheap: a doubted marker publication is
  treated like a doubted identity publication (continue with the
  durability-pending outcome, never an error, per the atomicfile
  contract); the digest cache is written once on return and once on
  expiry, not per candidate.
