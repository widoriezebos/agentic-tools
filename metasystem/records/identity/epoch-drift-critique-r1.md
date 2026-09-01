# Epoch-drift identity design — Sol critique round 1, seat dispositions

Critic: codex gpt-5.6-sol (job epoch-drift-critique-r1) against the
design at 5d7374d2 (landed 7d986c98). 12 findings, ALL material (1
critical). All ACCEPTED, binding on fold round 2. Budget note: the
goal's fence sits exactly two launches away - fold and re-critique
must converge or the chain waits at the fence.

## ED-R1-010 [critical]

Attack line 4, migration: the legacy fallback knowingly lets a drifted record override a stronger live observation. Sections 2.3 and 6 retain exact-second comparison for every pairless record, while AliveRef converts a readable mismatch into definitive death. That result authorizes lock takeover, lease takeover, job terminalization, and pruning; describing locks and guards as short-lived does not make those authorizations safe. The same hole reaches lineage: an old pairless announcement that disagrees by one second fails the finder and causes a new main lineage identifier to be minted for the same live process. Remedy: make legacy mismatch non-authorizing, such as Unknown, until an explicit per-schema enrichment, drain, or secondary proof resolves the record, including a defined legacy-announcement reuse path.

Evidence: Design sections 2.3, 3, 6, and 8 at metasystem/records/identity/epoch-drift-design.md:72-75,99-110,349-383,451-469; definitive-death mapping at metasystem/internal/identity/identity.go:194-216; announcement fallback and mint at metasystem/internal/lease/verbs.go:121-139,173-204; live-holder succession and takeover branches at metasystem/internal/lease/claim.go:78-125.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-001 [high]

Attack line 1, false equality: the proposed Linux triple is not globally unique, but the design calls it identity without declaring its comparison scope. Start ticks have 10-millisecond granularity; a process identifier can be recycled within a tick on a sufficiently fast, low-PID-limit system. Separate PID namespaces can repeat a PID while exposing the same host boot identifier, and restoring or cloning a running virtual-machine snapshot duplicates the boot identifier, tick state, PID table, and existing processes. SameRef has no machine, PID-namespace, or clone-incarnation input, so records from two real processes can compare equal if they meet in shared or aggregated state. Remedy: declare and enforce a single machine-and-PID-namespace comparison domain, rejecting cross-domain joins, or add a non-clonable machine/namespace incarnation to every token and record.

Evidence: The unscoped law is in metasystem/records/identity/epoch-drift-design.md:63-71,127-145; Ref contains only PID, time, ticks, and boot identifier at metasystem/internal/identity/identity.go:21-31; Linux fixes user clock ticks at 100 per second at metasystem/internal/identity/identity_linux.go:21-25. The collision cases are inferred from those fields and the absence of any domain field.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-002 [high]

Attack line 2, false inequality: a legitimate Linux start tick of zero creates a token that compares unequal to itself. The prober accepts zero, Exact.Ref records zero ticks plus a non-empty boot identifier, but Ref.Mode treats tick presence as greater than zero and classifies that record as a partial invalid shape. The proposed omitempty schemas also omit the zero tick while retaining the boot identifier. Compare is declared unchanged, so an early-boot process can never match its own persisted Ref. Remedy: represent exact-field presence independently of numeric value, or consistently reject zero at the probe boundary and prove that rejection matches the kernel contract.

Evidence: The design leaves Compare unchanged and makes all new Linux fields omitempty at metasystem/records/identity/epoch-drift-design.md:153-155,162-175; zero is accepted at metasystem/internal/identity/identity_linux.go:51-59; Exact.Ref persists the boot identifier with zero ticks at metasystem/internal/identity/identity.go:49-58; Mode then marks the shape invalid at metasystem/internal/identity/identity.go:72-93.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-003 [high]

Attack lines 1 and 4: SameRef is specified to weaken malformed and incompatible exact records to seconds. Its mechanical rule never rejects CompareInvalid: after the two exact-mode cases, any two positive seconds are compared. A partial Linux pair, a mixed Linux/Darwin record, or a record carrying both exact shapes can therefore compare equal by seconds even though live-versus-record Compare correctly refuses malformed exact data. This is a new false-equality path at the central comparator. Remedy: reject either invalid mode first and permit fallback only for an explicitly valid legacy-plus-native-exact combination within the same declared platform scope.

Evidence: The weakening rule is explicit at metasystem/records/identity/epoch-drift-design.md:127-147; current Mode defines partial and mixed exact shapes as invalid at metasystem/internal/identity/identity.go:72-93, and current Compare refuses invalid mode at metasystem/internal/identity/identity.go:119-137.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-004 [high]

Attack line 5, Darwin implementability: the declared Darwin exact comparator is not propagated through the schemas the design calls already correct or through most Linux-only schema additions. Census authentication, announcements and leases, steward identities, goal ownership, supervision owner/state records, and human-authority records lack a Darwin exact-microsecond field and therefore keep seconds equality on Darwin. This also makes gap G1's non-foreclosure claim misleading: authority and identity formats are already hard-committed to seconds unless this design changes them. Remedy: carry one platform-tagged exact token through every identity-bearing schema and adapter, adding Darwin microseconds wherever the design adds Linux ticks and boot identifier.

Evidence: The Darwin law and strongest-identity writer law are at metasystem/records/identity/epoch-drift-design.md:68-71,81-95, while section 4.2 adds only Linux fields and calls announcements, leases, steward, goal, and supervision writers already correct at lines 162-190. Representative omissions are metasystem/internal/census/verbs.go:30-64, metasystem/internal/lease/classify.go:21-29,149-175, metasystem/internal/steward/runner.go:39-43,562-569, and metasystem/internal/humanauthority/authority.go:40-46,277-289.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-005 [high]

Attack line 3, missed caller: dispatch's watcher-ceiling admission gate compares the armed watcher state to its heartbeat using PID, drift-prone epoch second, and tag only. Those seconds come from independent probes: the owner probes the child after launch, while the child probes itself on startup before writing heartbeats. This directly violates the flows-by-copy premise and can refuse every dispatch even after the listed census fixes. The site and heartbeat schema are absent from Ruling R. Remedy: persist the full platform Ref in both state and heartbeat, then perform the record join through SameRef before accepting the ceiling attestation.

Evidence: The design's copy law and dispatch enumeration are at metasystem/records/identity/epoch-drift-design.md:81-95,301-309. The seconds-only join is metasystem/internal/dispatch/attest.go:109-149; the child independently probes itself at metasystem/cmd/metasystem/supervise_component.go:76-81 and writes a seconds-only heartbeat through metasystem/internal/supervise/component.go:32-53; the parent separately probes at metasystem/internal/supervise/proc.go:66-86.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-006 [medium]

Attack line 3, missed caller: the custody-add command re-probes a process and rejects when its derived epoch second differs from the caller's freshly recorded second before invoking CustodyAdd, even though CustodyAdd itself already performs an exact start/group/start sandwich and records the native token. A clock step between the shell read and command probe makes one process compare unequal to itself and strands custody registration. Remedy: remove the redundant seconds gate or pass the caller's full Ref and compare it through the central comparator.

Evidence: The omitted gate is metasystem/cmd/metasystem/dispatch_verbs.go:1164-1192, called after a separate shell probe at metasystem/scripts/agents/dispatch.sh:2160-2170. The exact implementation it precedes is metasystem/internal/dispatch/custody.go:16-27,59-66,81-85. Ruling R's dispatch section at metasystem/records/identity/epoch-drift-design.md:301-309 does not list it.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-007 [high]

Attack line 3, missed caller family: changing identity.Custodian to accept Ref is insufficient because ReaperConfig itself accepts only PID and epoch second, reapOne loads only those fields, and both production bindings preserve that truncation. The mission usage-recovery gate also aliases and calls the old seconds signature. Job records already carry the exact pair, so an implementer can make the signature compile while still constructing a seconds-only Ref, leaving false terminalization and false death proofs. Remedy: change ReaperConfig and every Custodian binding to accept a full Ref loaded from the job object, including steward reconciliation, command-layer reaping, mission drain, acknowledgement expiry, and mission usage recovery.

Evidence: The design changes only the core signature at metasystem/records/identity/epoch-drift-design.md:148-152 and lists only selected callers at lines 267,315. The truncating interface and loader are metasystem/internal/supervise/reaper.go:35-49,126-147; production bindings are metasystem/cmd/metasystem/supervise_component.go:295-329 and metasystem/internal/steward/reap.go:80-104; the missed mission alias and call are metasystem/internal/mission/fence.go:617-620,765-795. Exact job fields already exist at metasystem/internal/dispatch/ownership.go:84-96.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-008 [high]

Attack line 3, missed lock probes: adding fields to lock.Identity does not update the command-layer registry lock. The supervision owner constructs its registry-lock identity without the pair, and kernelProbe discards any new fields when checking a contender. The diagnostic supervision-status reader likewise drops the pair from owner.json. A drifted live registry writer can therefore still be declared dead and lose its lock, admitting concurrent writers. Remedy: update every lock constructor, custom codec, and Probe binding to preserve and compare the full platform Ref, and enumerate the command-layer bindings explicitly.

Evidence: The generic schema addition is specified at metasystem/records/identity/epoch-drift-design.md:162-168, but the Ruling R tables do not include command-layer registry locking. The seconds-only self and probe are metasystem/cmd/metasystem/supervise_owner.go:89-115,200-209; the lock protects registry append at metasystem/internal/registry/append.go:12-32. The additional dropped-pair status reader is metasystem/cmd/metasystem/supervise.go:105-135.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-009 [high]

Attack line 3, missed record format: census InventoryItem observes but discards Linux start ticks and boot identifier. The design says acknowledged records must replace Linux's drifting derived microseconds with the pair, but acknowledgement must first prove that the live process is the same process the census observed. Without the pair in the inventory, an implementer must either keep the drifting microsecond check or re-probe and silently lose the census-to-live binding across PID reuse. Remedy: add the full native Ref to every census inventory item and make acknowledgement compare that recorded Ref before writing its entry.

Evidence: The strongest-writer and acknowledged-entry requirements are at metasystem/records/identity/epoch-drift-design.md:93-95,172-174. Process carries the pair, but InventoryItem and its constructor omit it at metasystem/internal/census/run.go:29-64,253-258. Acknowledge currently requires the census exact token at metasystem/internal/supervise/acknowledged.go:146-180,252-307; on Linux that microsecond value is derived from drifting btime.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-011 [medium]

Attack line 3, comparator retention: the design's centralization law conflicts with its conversion and recurrence plans. It says no package outside identity may compare start fields, but converts only sites labeled vulnerable, explicitly allows safe structural comparisons to remain, and proposes a lexical grep that misses aliases, composite map keys, helper calls such as looseEqual, uppercase PIDStartedAt, inequality on StartedAt.Unix, and shell comparisons. Current hand-written pair-or-seconds comparators would therefore remain sanctioned and future drifting comparators can evade the guard. Remedy: require all production joins to call the identity API before the guard lands, and enforce the boundary structurally through imports or syntax analysis with no comparison allowlist.

Evidence: The one-owner law is metasystem/records/identity/epoch-drift-design.md:79-95; conversion is limited to vulnerable labels at lines 192-208; the grep patterns and continuing allowlist are lines 210-219. Current surviving examples include metasystem/internal/lease/identity.go:84-97, metasystem/internal/lease/claim.go:344-355, and metasystem/internal/steward/health.go:1029-1037; the missed inequality form is metasystem/cmd/metasystem/dispatch_verbs.go:1188.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## ED-R1-012 [medium]

Attack line 5, implementability: the promised one canonical JSON-object-to-Ref loader has no specified way to decode the design's own heterogeneous field names. The existing loader accepts pid, pidStartedAt, pidStartedAtExactMicro, pidStartTicks, and bootId, while proposed records use ownerPidStartTicks, custodianPidStartTicks, watcherStartTicks, wrapperPidStartTicks, and existing startTicks forms. An implementer must guess whether to add aliases, pass a prefix/profile, rename durable schemas, or leave duplicate loaders. Remedy: specify typed schema adapters or a loader API with an explicit field mapping and reject ambiguous aliases.

Evidence: The one-loader contract is metasystem/records/identity/epoch-drift-design.md:156-160, while the schema table itself introduces multiple prefixes at lines 164-175. The fixed-key implementation being relocated is metasystem/internal/dispatch/ownership.go:99-115; noncanonical existing formats include metasystem/internal/steward/runner.go:39-43, metasystem/internal/goal/journal.go:69-75, and metasystem/internal/humanauthority/authority.go:40-46.

DISPOSITION: ACCEPTED. Folds into revision round 2.
