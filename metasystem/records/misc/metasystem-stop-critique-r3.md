# metasystem stop design critique — round 3 (revision 3, closing)

Chain: revision 3 (landed bed2eb02 as plans/metasystem-stop-verb-design.md, sha256 47d0e6731446e47135b2440e3c0d80a4f2281e7d6bb1b1e8b9f04b6e3498a72d) -> critic stopverb-crit3 (design-critic, read-only runtime). Reviewed commit 372ba0505bc2208f18e95055c747525bcc353b3e. 6 material findings. The coordinator (m1b) carried the return here because the critic's permission envelope had no write root.

## STOP-R3-001 — critical, material=True

CLAIM: The section 2 generation handshake is not a completion barrier, so a successful stop can return before a late creator has ended its process. A creator can read the open generation, pause until stop's last empty pass has returned success, publish a child, and then crash before its second read or cleanup. Dispatch and run launch make this concrete because their records and detached process starts occur in different Go and shell or command-layer seams. Register and Adopt also have no specified post-race outcome because they bind a foreign process that the design forbids signaling. The build must add an in-flight creation claim that stop joins, or another protocol that proves every pre-close creator completed cleanup before stop returns; its deterministic tests must place barriers around the real spawn boundaries for run, dispatch, proof-run, mission launcher and loop, steward runner, and owner launch.

EVIDENCE: Revision 3's proof covers whether publication is eventually seen, but not whether creator cleanup has completed when stop prints success. claimLaunchAttempt publishes the job reservation before dispatch.sh later calls launch_adapter; run.Store.Launch publishes before cmd/metasystem/run.go starts the wrapper. The named internal package tests therefore cannot exercise those actual boundaries, and section 10 names no owner-launch race proof. This reopens round-two finding STOP-R2-001 and changes the new stop-fence protocol and its family interfaces.

## STOP-R3-002 — critical, material=True

CLAIM: The reordered owner deadline still permits force-killing an orderly owner before its real teardown contract expires. The deadline must derive from every retained held identity, include the post-kill proof interval, and include the registry lock's actual bounded wait, or use an explicit owner-completion signal instead of reconstructing the bound from state.json.

EVIDENCE: ProcComponents.Stop waits up to five seconds before force-kill and then performs twenty-five additional twenty-millisecond death checks. Owner.teardownHeld processes every in-memory identity sequentially, including unproven older-generation identities that can make the set larger than two. RegistryLedger.AppendExited calls registry.LockedAppend through the owner wiring with a thirty-second lock wait. Revision 3 uses count times five seconds plus two seconds and falls back to two identities when state.json is unreadable. Its slow-owner fixture uses real elapsed time with two four-second children and no registry contention, so it cannot prove the full bound. This reopens round-two finding STOP-R2-002.

## STOP-R3-003 — high, material=True

CLAIM: The directory-gone fleet branch remains unimplementable as written because the production registry does not retain the supervision owner's process identity. It therefore cannot probe and print the promised recorded owner, and it cannot honestly say that nothing live is recorded merely because the unavailable identity cannot be read. The build must either change the registry writer and reduction contract or report an identity-unavailable survivor without probing. The fleet fixture must use production-shaped registry history and include an actual human session, not only a sleeping process shaped like a supervision owner.

EVIDENCE: RegistryLedger writes relaunched, launched, and exited rows. PublishedOwner in registry.Reduce retains checkout path, generations, retirement watermark, and closure, but no owner pid or start identity. Armed rows can carry owner identity for the separate Claim projection, but no non-test production writer emits them. The section 10 fleet fixture artificially records an open owner identity and asserts that the owner-shaped sleeper survives; it neither reproduces production registry history nor launches an announced or adapter-shaped human seat. The no-signal rule is identity-safe, but the promised probe and output are not available. This reopens round-two finding STOP-R2-007.

## STOP-R3-004 — high, material=True

CLAIM: The inventory misses a live mission host turn when its mission runner is already dead, allowing stop to report success while metasystem-launched host work remains. Open turn records must be inventoried independently of runner liveness and stopped through their recorded tag-proven groups.

EVIDENCE: missionrunner.cleanupStaleLease explicitly handles the existing state in which a dead runner left an open host-turn group. Revision 3 section 3 selects only running runner records whose runner identity is alive and then examines turns for each selected runner. A dead runner's live turn therefore falls through to the untracked census branch, which is printed but never signaled. A deterministic fixture must seed a dead runner record plus a live owned turn group and prove that stop ends the group and records turn-lost.

## STOP-R3-005 — high, material=True

CLAIM: Proof-run inventory loses its only authoritative suite identity when the watchdog dies, so a live suite can survive a successful stop. The proof-run launcher must durably publish the suite, launcher, and watchdog identities, or provide an equally authoritative identity source that survives any one process's death.

EVIDENCE: LaunchSuite starts the suite in its own process group and passes the suite identity only to the watchdog arguments. The launcher arguments contain the suite name and root but not the child identity, and revision 3 explicitly adds no proof-run record. Section 3 discovers the suite through a live watchdog, so a dead watchdog with a live launcher and suite leaves no identity-safe way to select the suite group. The existing proof-run-stop fixture covers a healthy watchdog only.

## STOP-R3-006 — medium, material=True

CLAIM: Section 12 is not a credible single unsliced implementation within the remaining goal box. The first build slice must be the complete per-checkout vertical feature: stop, status, arm, the transition fence, all local creator handshakes, all recorded process families, and their honesty fixtures. The optional --all fleet form, including directory-gone behavior, should be a later slice after that local acceptance test passes.

EVIDENCE: The scope spans eight complex existing packages, two new packages, command routing, three shell surfaces, an adapter, and sixteen cross-process scenarios. The configured dispatch ceiling is two hours per attempt. This critique consumes one of ten attempts, and mandatory code review consumes at least one more. The goal itself says fleet support is welcome only if cheap, while per-checkout behavior is required. Dispatching the whole section 12 scope as one undivided implementer brief creates a genuine implementation-choice and budget-exhaustion risk.

## Gaps the critic named

- The declared register metasystem/records/misc/metasystem-stop-critique-r3.md could not be written because this job's enforced permission envelope has no write roots and exposes read-only tools. The coordinator must carry this canonical return into that record, as it did for round two.
- No process was launched or signaled. Live proof was not required, so runtime behavior beyond the cited deterministic code contracts remains unexecuted.
- The runtime notice classifies context isolation and the provider tool catalog as advisory, so neither was independently proven.

## Coordinator disposition (m1b, 2026-09-06)

All six are accepted. The design ladder is closed by the implementation-first ruling (D81): no fourth prose round. The folds are build decisions, written as section 13 of plans/metasystem-stop-verb-design.md and carried into the slice-1 build brief:

- STOP-R3-001 (no completion barrier): creation claims. Every creator opens a claim file under the state root before it publishes or spawns anything and closes it after its second fence read; stop, after closing the fence, waits for every claim carrying the pre-close generation to close (stale claims of dead creators are removed), then inventories. Register and Adopt after the race fail their record and leave the foreign process untouched.
- STOP-R3-002 (owner deadline reconstructed): the owner publishes its own teardown ceiling in state.json whenever it publishes state; the caller waits for the owner's death by identity up to that ceiling and only then KILLs. No count is reconstructed by the caller.
- STOP-R3-003 (registry keeps no owner identity): the fleet form is slice 2; in it, a directory-gone checkout is reported as having no identity recorded and is not signalled and not a survivor; the fixture uses production-shaped registry history and a real announced session.
- STOP-R3-004 (dead runner, live turn): open turn records are inventoried on their own, whatever the runner's state, and stopped through their tag-proven groups.
- STOP-R3-005 (dead watchdog, live suite): the proof-run launcher publishes a record carrying the suite, launcher and watchdog identities; the inventory reads the record, not the watchdog's argv.
- STOP-R3-006 (scope): slice 1 is the per-checkout vertical (stop, status, arm, fence, claims, every local family, every section 10 scenario except fleet); `--all` is refused in slice 1 and built in slice 2.
