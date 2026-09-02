# Recovery analysis critique — round 1 (Sol)

Chain: analysis (plans/recovery-analysis.md) -> critic recovery-crit1b (codex {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}, reviewed commit 599225dc3d69aa512d148d67ea92fcf23b5182a2), 2026-09-02. 10 material findings. Full return: artifacts/agents/recovery-crit1b/rounds/1/return.json.

## REC-A01-FALSE-DEAD-BRANCH — high, material=True

CLAIM: Correction B correctly identifies the full-reference versus seconds-only mismatch, but its causal conclusion is false and would produce the wrong S2 implementation. Metasystem/plans/recovery-analysis.md:387-389 says releasing a falsely dead owner's lock is “how a checkout ends with a drifted owner alive beside a new one,” and line 429 requires that the Dead branch never signal a group whose full reference is alive. Metasystem/internal/supervise/decide.go:83-88 and owner.go:226-240 expressly make a superseded live owner retire on its next cycle, while arming.go:368-431 authenticates full-reference-live component groups precisely so takeover can safely stop them. Specimen 4 has no primary record proving a persistent dual owner. An implementer following S2 could disable necessary component teardown while leaving the actual comparator defect as the only established cause.

EVIDENCE: Metasystem/internal/supervise/arming.go:712-726 does not signal the old owner; it stops authenticated components, releases the lock, and launches a successor. The old owner observes NamesOther or NoLock and exits. The analysis itself admits at metasystem/plans/recovery-analysis.md:441-451 that the second half lacks matching specimen evidence.

## REC-A02-UNSUPPORTED-SILENCE-PLACEMENT — high, material=True

CLAIM: The analysis presents specimen 2's thirteen minutes as pre-announcement delay without evidence of the up invocation time. Metasystem/plans/recovery-analysis.md:404-409 says the records “place” the delay before announcement, but runner start is only evidence that steward enrollment had completed; it is not an up-start timestamp. The placement is merely consistent with code order. Consequently S1's claimed replay of specimen 2 has no observed delayed operation to reproduce, although the narrower claim that ordinary up prints component lines only after returning is true.

EVIDENCE: Metasystem/artifacts/agents/steward/runner.json:5-6 records 11:21:42, arming.log:8463 records announcement at 11:35:48, and state.json:4-6 records the owner at 11:35:49. Metasystem/internal/up/up.go:428-479 proves announcement precedes supervision only after up has begun. No record establishes that up began at 11:21:42.

## REC-A03-STORED-WORD-REPLAY — critical, material=True

CLAIM: Correction A's existing-command facts are correct, but its conclusion that drift is “not a law defect” and S5's stored-word self-repin rule silently widen enrollment authority. Under current law, the seat must invoke steward arm with the relayed word for each new engine generation. Allowing plain up to replay TemporaryHumanWord from the previous identity lets changed invoking bytes bless themselves. That converts an audited human act into a reusable capability and weakens the enrolled-byte breaker. Wido's later requirement that up repair its pin can authorize a new mechanism, but the design must explicitly bind it to the standing rebuild ruling, a gated commit, validity and expiry; “a word exists in the old identity” is insufficient.

EVIDENCE: Metasystem/memory/rulings.md:57-58 and 75 authorize and consume the word through steward arm, while line 64 requires rebuild re-arms to consume gated commits. Metasystem/internal/humanauthority/authority.go:139-156 says steward word validation checks transport shape only and does not enforce human provenance or expiry. Nevertheless metasystem/plans/recovery-analysis.md:432 authorizes self-repin from any standing word stored at identity.go:40-47.

## REC-A04-REFUSAL-INVENTORY-INCOMPLETE — high, material=True

CLAIM: The refusal inventory is not exhaustive enough to support S1's “every remedy” contract. Section 3 reports 14 runnable, 3 terminal-only, 1 nonexistent and roughly 45 non-command texts, but a conservative sample of the six mandated package roots found 56 distinct omitted templates containing neither “remedy” nor “refusal.” The table also mixes grouped strings and rows with multiple verdicts, so its four totals have no stable counting unit. An implementer using it would leave many seat-visible failures without an actionable command.

EVIDENCE: Examples include metasystem/internal/up/up.go:160-220 and 295-310; internal/steward/runner.go:84,224,325,337,415,465-501; internal/lease/verbs.go:377,398-410,490-493; internal/dispatch/admission.go:49,131; internal/goal/fetchadvance.go:94,114,118; and cmd/metasystem/delegate.go:220-365. The sample totals 56 distinct omitted templates. Metasystem/plans/recovery-analysis.md:259-261 does not define whether grouped messages or mixed rows count once or separately.

## REC-A05-LEAK-SURFACE-INCOMPLETE — high, material=True

CLAIM: The leak account's launcher and cleanup model is incomplete, so S4 would repair the wrong fixture surface. Metasystem/plans/recovery-analysis.md:283 says every other fixture arms nothing, but supervision-go-fixtures directly starts real owners. Lines 299-313 omit its tag-scoped cleanup, omit delegate-caps-fixtures' process backstop, and imply health-fixtures loses a restarted runner process identifier even though cleanup rereads the durable record. The proposed owner bed-vanish work is also partly redundant because owners already self-retire when their checkout disappears.

EVIDENCE: Metasystem/scripts/agents/supervision-go-fixtures.sh:23-52 starts real owners with shell backgrounding rather than a new session and cleans them at lines 28-33. Delegate-caps-fixtures.sh:15-44 performs shutdown plus path-scoped TERM/KILL. Health-fixtures.sh:125-168 refreshes the process identifier in the EXIT trap and lines 382 and 457 refresh it after restart. Metasystem/internal/supervise/decide.go:70-88 already classifies a vanished root as PurposeGone.

## REC-A06-AGE-IS-NOT-CUSTODY — critical, material=True

CLAIM: S4 invents unsafe custody from age. Metasystem/plans/recovery-analysis.md:431 directs a janitor to reap engine-signature processes when their recorded bed is gone “or older than a bound.” An old bed does not prove that its live process is orphaned; a legitimate long-running fixture can satisfy both the signature and age predicates. This contradicts lines 327-329, which correctly say an unowned process must not be signalled. Implementing the stated predicate can terminate live work that no current custodian authorized the janitor to own.

EVIDENCE: Metasystem/internal/census/run.go:292-332 can classify a process and verify identity, but UNTRACKED supplies no owner or abandonment proof. Metasystem/plans/goals/proof-harness-process-custody.md:6 requires processes to be both orphaned and older than a bound; S4 drops the orphan proof from the process predicate.

## REC-A07-STOP-IGNORES-UNTRACKED — high, material=True

CLAIM: S3's stop postcondition is weaker than Wido's full-stop order. It requires only zero CUSTODY and zero ANNOUNCED entries, allowing any number of UNTRACKED processes to remain. Census also omits processes whose vanished bed is outside the checkout scope. Putting a separate janitor in later S4 does not make the S3 stop verb compose or prove that cleanup. An implementer could satisfy the stated fixture while leaving precisely the lingering processes this goal exists to eliminate.

EVIDENCE: Metasystem/plans/recovery-analysis.md:430 names only CUSTODY and ANNOUNCED. Metasystem/internal/census/run.go:261-265 can return SUCCESS with UNTRACKED inventory, lines 306-332 assign that class, and internal/census/scope.go:9-18 excludes processes outside the checkout's resolved paths. Metasystem/plans/goals/recovery-to-good-state.md:6 instead requires no process lingering and census proof.

## REC-A08-ARC-DOES-NOT-CLOSE-EIGHT-SPECIMENS — high, material=True

CLAIM: The arc cannot deliver its claimed eight-specimen DONE gate. S7 says one fixture group puts a bed into every state, but the same table leaves specimen 6's remote-control channel to never-idle, leaves specimen 7's fresh-clone join to fleet-join-bootstrap, and leaves recovery-rehearsal's enrolled-ring leg to a human. Specimen 5 can only be named, not recovered, by S6. These are not merely dependencies: the mechanisms and fixtures are outside the proposed arc, while Wido's criterion requires every specimen and never a terminal.

EVIDENCE: Metasystem/plans/recovery-analysis.md:432-434 contains the conflicting leave and all-eight claims. Metasystem/plans/goals/recovery-to-good-state.md:4-6 requires the full matrix and terminal-free recovery. Metasystem/plans/goals/fleet-join-bootstrap.md:4-6 confirms that a fresh clone lacks even the engine required to execute up.

## REC-A09-IDENTITY-CHAIN-EVASION — high, material=True

CLAIM: S2 cannot silently re-scope the virtual-machine identity goal into this new chain. Its current record says revision 2 still has eleven material findings, including two critical migration holes, and explicitly waits for Wido to choose the joint-round fork or fence. Saying its design and round-2 critique are merely “input” does not disposition those findings or preserve the critic-chain exhaustion accounting. An implementer could build the comparator core while dropping the pairless-record and different-process migration requirements.

EVIDENCE: Metasystem/plans/recovery-analysis.md:429 re-scopes the goal without enumerating its open findings. Metasystem/plans/goals/vm-epoch-identity-drift.md:6 records eleven material findings, names two critical holes, and reserves the fork for Wido. The design-critique chain law requires every material finding to receive a disposition and keeps the round budget on the original chain.

## REC-A10-PROGRESS-IS-DESIGN-BEARING — medium, material=True

CLAIM: S1 is incorrectly classified as mechanical. Today up.Run returns a completed Result and the command prints it after return. Printing each component when it completes requires a streaming boundary through ordinary, recovery, supervision and runner waits, including ordering and behaviour when a later component fails. S1 specifies no callback, writer, event contract or ownership boundary, so an implementer must invent a design-bearing interface despite the 120-minute mechanical classification.

EVIDENCE: Metasystem/cmd/metasystem/up.go:63-68 and 151 only print the returned result. Metasystem/internal/up/up.go:412-485 accumulates component outcomes locally. Metasystem/plans/recovery-analysis.md:428 requires incremental output but supplies no interface or failure-delivery contract.

## Critic-declared gaps

- Specimen 4 still has no primary m0b handoff, transcript or process record in this tree. The ten-hour process, the exact refusal and whether shutdown was attempted cannot be verified.

- Specimen 2 has no up invocation timestamp or transcript. Which operation, if any, consumed the thirteen minutes remains unknown.

- The historical process and stale-bed snapshot behind the 303-runner attribution is not present. Source code identifies plausible launchers but cannot assign those historical processes to a particular fixture run.

- The 56 omitted refusal templates are a conservative six-package sample, not an exhaustive total across every reachable branch in those packages.

- The arc gives aggregate estimates of 120 or 240 reserved minutes but no per-rung reservations, so headroom for a correction round cannot be verified. A 240-minute row may or may not include correction capacity.
