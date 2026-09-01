# Epoch-drift design — Sol critique round 2, seat dispositions

Critic: codex gpt-5.6-sol (epoch-drift-critique-r2c) against revision 2
(104a2f2d). 11 findings, ALL material (2 critical). Trajectory 12 -> 11:
the pure-loop STALL SIGNATURE, now on a THIRD chain - R-38-m0's clause
fires: the pattern is a finding against the R-25 lane structure for
Wido's review. All findings ACCEPTED; the chain holds at this fork:
joint round (Wido's word per case) or fence.

## ED-R2-001 [critical]

A pairless legacy seconds match is still treated as authoritative even when the live Linux probe carries a native token. Process identifier reuse plus boot-time drift can make a stale announcement match a different process, and a repeated command hash then classifies that process as the old main with checkout authority. Remedy: make every legacy-record/native-probe join Unknown, whether seconds match or mismatch, unless an independent per-launch proof binds it.

Evidence: Design sections 2.2.3 and 4.1.2 explicitly say a seconds match proves the join at metasystem/records/identity/epoch-drift-design.md:162-173 and map matches to Alive at metasystem/records/identity/epoch-drift-design.md:316-333. The announcement authenticator accepts matching process identifier, command hash, and seconds at metasystem/internal/lease/classify.go:149-175, after which classification returns MAIN at metasystem/internal/lease/classify.go:320-325.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-002 [high]

The native Linux triple is knowingly non-injective: two processes can share a process identifier and start tick after same-tick recycling, while snapshot clones can duplicate the complete process table and boot identifier. A documentation-only single-machine precondition does not prevent those records meeting through a shared artifacts root. Remedy: enforce a machine and process-namespace incarnation at every join and use a stronger per-spawn discriminator, or reject environments where that domain cannot be proved.

Evidence: Design section 2.1 admits both same-tick recycling and clone duplication while providing no runtime domain check at metasystem/records/identity/epoch-drift-design.md:108-138; section 7D explicitly rejects an incarnation field at metasystem/records/identity/epoch-drift-design.md:899-905. The comparison object has only process identifier, seconds, microseconds, ticks, and boot identifier at metasystem/internal/identity/identity.go:21-31.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-003 [critical]

The legacy-announcement migration merges different processes because session identifier plus command hash is not a per-launch identity proof. A restarted mission runner that reuses a process identifier repeats both values, receives the predecessor's main lineage identifier, and then renews the existing lease before holder liveness is checked. Remedy: require the recorded native token or a unique per-launch nonce; any unproved new process must mint a new main identifier and use owner lineage for succession.

Evidence: Design section 6.6 authorizes reuse on session and command alone while preserving mainId at metasystem/records/identity/epoch-drift-design.md:805-820. Mission-runner sessions are derived from mission plus process identifier and use a constant announcement tag at metasystem/internal/missionrunner/loop.go:2260-2266. The lease treats equal mainId as renewal before liveness at metasystem/internal/lease/claim.go:78-86.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-004 [high]

The zero-start-tick fold changes only the core representation while multiple boundaries still define zero as token absence, so a legitimate first-tick Linux process is rejected or downgraded and can compare unequal to itself. Remedy: make non-empty boot identifier the sole Linux presence bit at every validator, command interface, schema adapter, and shell boundary, with an end-to-end zero-tick fixture.

Evidence: The intended rule and limited core edits are in design sections 2.2.1 and 4.1.3 at metasystem/records/identity/epoch-drift-design.md:144-151 and metasystem/records/identity/epoch-drift-design.md:349-361; the guard exempts literal-zero checks at metasystem/records/identity/epoch-drift-design.md:590-605. Remaining rejecting boundaries include metasystem/internal/lease/verbs.go:85-98, metasystem/internal/gaterun/fence.go:66-71 and 116-123, metasystem/cmd/metasystem/census.go:154-170, and metasystem/scripts/agents/go-gate.sh:205-225.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-005 [high]

Ruling R misses the mission-contract supervision preflight, which compares state and heartbeat process identifiers and drift-prone seconds loaded into local variables. Independent probes can therefore refuse mission admission after the documented schema changes. Remedy: load both complete native references and join them through identity.SameRef, and add this caller to the ruling and fixtures.

Evidence: The live comparison is at metasystem/internal/contract/contract.go:1364-1402. Revision 2 enumerates only the later contract liveness wrapper and fixture table at metasystem/records/identity/epoch-drift-design.md:746-759, while acknowledging that the state and heartbeat originate from independent probes at metasystem/records/identity/epoch-drift-design.md:480-485.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-006 [medium]

The abstract-syntax-tree recurrence guard still misses aliases, map extractions, helper equality functions, struct equality, and the design's own prefixed field names. The missed contract comparison compares local variables and would pass the proposed guard unchanged. Remedy: enforce the boundary with identity-typed data flow or inaccessible identity fields, rather than a selector-name list.

Evidence: The analyzer specification only rejects binary expressions whose operand resolves directly to one of eight selector names at metasystem/records/identity/epoch-drift-design.md:582-605. The missed production comparison uses heartbeatStarted and started locals at metasystem/internal/contract/contract.go:1384-1401. Existing schemas also use names absent from the list, including TargetPidStartedAt at metasystem/internal/supervise/disk.go:264-269 and SuccessPidStartedAt at metasystem/internal/steward/component_evidence.go:33-44.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-007 [high]

The lock conversion does not enumerate custom owner codecs. After the census-writer self identity gains native token fields, its codec still serializes only process identifier, seconds, and tag; ReleaseNamed then compares the stripped decoded value to the full self value by raw struct equality and cannot release its own lock. Remedy: require every custom codec to round-trip every identity field and add acquire-release round-trip fixtures for each codec.

Evidence: Design section 4.1.6 names constructors and probes but not the codec field maps at metasystem/records/identity/epoch-drift-design.md:431-466, and the schema table lacks the census-writer custom owner format at metasystem/records/identity/epoch-drift-design.md:472-493. The codec drops token fields at metasystem/internal/supervise/censuslock.go:40-71, while raw release equality is enforced at metasystem/internal/lock/lock.go:238-254.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-008 [high]

The declared Darwin non-foreclosure is false: the design hard-codes the native token as an integer epoch microsecond and hard-codes a Darwin-microseconds comparison mode. If gap G1 disproves stability or uniqueness, a different token shape cannot be substituted without changing schemas and compatibility semantics. Remedy: either validate G1 before shipping or define a versioned, platform-tagged opaque native-token union now.

Evidence: Design section 2.2.2 fixes Darwin identity to a spawn-record microsecond at metasystem/records/identity/epoch-drift-design.md:154-161; sections 4.2b and G1 claim any replacement is only a value change at metasystem/records/identity/epoch-drift-design.md:499-538 and metasystem/records/identity/epoch-drift-design.md:944-953. The current core representation and comparator are specifically StartedAtUnixMicro and CompareDarwinMicroseconds at metasystem/internal/identity/identity.go:25-31 and 61-69, with numeric comparison at metasystem/internal/identity/identity.go:128-136.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-009 [high]

Darwin propagation remains incomplete because the durable waiter record is listed as safe but is absent from the Darwin schema migration. It persists only seconds plus the Linux pair, so Darwin cleanup and liveness retain the weaker comparator. Remedy: add the Darwin native token to Waiter, its writer, both readers, and its cleanup comparison, then repeat the record-format census.

Evidence: The Darwin propagation list ends without the waiter schema at metasystem/records/identity/epoch-drift-design.md:499-530, while Ruling R labels waiter comparisons safe at metasystem/records/identity/epoch-drift-design.md:732-737. The format and writer have only seconds, ticks, and boot identifier at metasystem/internal/run/waiter.go:45-55 and 138-147; liveness and cleanup consume that truncated shape at metasystem/internal/run/waiter.go:124-125 and 172-188.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-010 [high]

The copy-only law is not implementable through the announcement interface: it carries seconds and the Linux pair but no Darwin token, and the writer re-probes to mint token and command fields. If the target recycles between the upstream proof and this probe, the record combines the old session identity with the new process's token and command. Remedy: pass one complete identity.Ref end to end and use any later probe only to compare; refuse unbound legacy production inputs.

Evidence: The copy-only law is at metasystem/records/identity/epoch-drift-design.md:211-222, but no announcement-interface change is specified by the Darwin propagation list at metasystem/records/identity/epoch-drift-design.md:499-530. AnnounceWithProofAt accepts no exact microsecond at metasystem/internal/lease/verbs.go:57-65 and obtains a fresh identity at metasystem/internal/lease/verbs.go:88-101. The upstream session structure likewise has only seconds, ticks, and boot identifier at metasystem/internal/up/up.go:115-125 and calls that interface at metasystem/internal/up/up.go:428-442.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.

## ED-R2-011 [medium]

Migration can permanently strand a tokenless record whose original process is dead while its process identifier is held by a long-lived stranger. The comparison remains Unknown, the dead owner cannot perform a natural rewrite, and the one-time re-arm does not rewrite guard, marker, revision-lock, or arbitrary registry records. Remedy: provide an explicit, proved recovery or quarantine procedure for every tokenless schema and a per-machine completion check instead of relying on natural rewrites.

Evidence: Design section 6.3 admits parked husks but incorrectly says the heal below rewrites them at metasystem/records/identity/epoch-drift-design.md:776-789; the actual heal covers only owner, state, ledger, heartbeats, locks, and announcements at metasystem/records/identity/epoch-drift-design.md:790-804. A gate marker with Unknown liveness is retained indefinitely at metasystem/internal/gaterun/gaterun.go:100-143, and a lock with Unknown liveness cannot be taken over at metasystem/internal/lock/lock.go:166-195.

DISPOSITION: ACCEPTED. Binds whichever successor round Wido's fork decision creates.
