Verdict: the proposed wall catches the original zero-dispatch failure, but it does not yet prove that accepted product bytes came from certified implementers. Path-based authorization, unverified `certified` claims, and the current fault lifecycle leave several ways to reproduce D99 while passing the proposed check.

## Findings

### HIW-R1-01 — STRUCTURAL: “Certified” is currently a host assertion, not runner-verified evidence

The orchestrator return permits `certified: [{jobId, verdict, evidence}]`, but mission adjudication validates only `dispatched`; it does not adjudicate `certified`. Conclusion then copies those entries into the turn log unchanged. A host can therefore claim certification without proving conformance. [orchestrator.schema.json](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/schemas/orchestrator.schema.json:45>), [adjudicate.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/adjudicate.go:256>), [turnio.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/turnio.go:115>).

The conformance `review.json` is not a certification either. It records the implementer job, patch artifact, and reviewed tree before critic closure. Successful merge validation emits no durable, content-addressed authorization record. [conformance.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conformance.go:216>), [conformance.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conformance.go:349>), [conformance.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conformance.go:875>).

Required revision:

- Conformance validation should create a durable integration authorization only after all implementer-return, critic-chain, and merge checks pass.
- That record must bind job, mission, stream, role, patch digest, reviewed tree, changed paths, and relevant base.
- Mission adjudication must validate each returned certification against that record and the dispatch job. Only adjudicated certifications should enter the turn log.
- The benchmark extractor should consume those adjudicated facts, not raw host claims.

### HIW-R1-02 — STRUCTURAL: A path subset cannot authorize integrated bytes

The design authorizes a host change when its path is contained in a delegate’s conformance diff. That grants the host the whole file: a delegate can change one line, after which the host may replace every other line without leaving the authorized path set.

The same leak admits host-written conflict resolution. Under KI-9, the orchestrator integrates, but mission doctrine already says unattended merge conflicts park; the orchestrator does not invent resolution bytes. [known-issues.md](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/known-issues.md:23>), [orchestration.md](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/orchestration.md:124>).

The wall needs a byte-level equation, conceptually:

`post tree = pre tree + ordered certified patches + measured self-work residual + exact host-owned metadata changes`

The runner can construct this with the existing isolated-index/tree machinery. Overlapping patches, failed application, target movement requiring rebase, or conflicts must require fresh conformance or park. Conflict-resolution bytes must not receive an implicit host exemption.

### HIW-R1-03 — STRUCTURAL: The proposed snapshot is neither complete nor trustworthy across recovery

A `git status` plus `git ls-files` digest is not an adequate preimage:

- An already-modified tracked file can change again while remaining `M`.
- `git ls-files` names files; it does not hash unstaged working-tree bytes.
- An already-untracked file can change without altering an inventory of names.
- Modes, symlink targets, deletions, gitlinks, and pre-existing dirty state need explicit treatment.
- “Product paths” introduces target-layout configuration where default-deny classification would be safer.

The conformance validator already has the better primitive: create an isolated index, `read-tree HEAD`, `git add -A`, then `write-tree`. That captures the shippable, nonignored working tree, including untracked content, without mutating the real index. [conformance.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conformance.go:225>).

The pre-tree identity also cannot live only in `turn.pre`: the host can write the turn directory, and a runner crash after host mutation could restart by treating the dirty tree as its new baseline. Keep the original identity in runner memory and persist it through a runner-authenticated or hash-chained state channel. Recovery must inspect an unfinished turn before taking any new snapshot.

The design must also decide explicitly whether ignored deliverables are outside “ships.” Git-tree comparison will intentionally exclude ignored caches and build products.

### HIW-R1-04 — STRUCTURAL: The allowlist covers legitimate activity only by being dangerously broad

Legitimate changes during a host turn include:

- Runner turn state, host-start markers, logs, heartbeats, and mission events.
- Adapter transcripts, raw responses, usage records, and result files.
- Dispatch jobs, rounds, fences, worktree records, conformance patches, and reviews.
- Supervision state, hooks, census, leases, temporary commit tokens, and garbage collection.
- Git administrative state while the orchestrator integrates and commits.
- Host-authored design, disposition, review, and receipt artifacts when the mission specifically assigns them.

The first four groups are principally under `artifacts/agents/**`; Git metadata should not enter the shippable-tree comparison. [host.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/host.go:220>), [loop.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/loop.go:177>), [supervision-hook.sh](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:118>).

By contrast, blanket exemptions for `artifacts/**`, `plans/**`, and `docs/reviews/**` permit product work to be hidden in repositories where those are shipped paths. `plans/**` also contains the signed mission contract and other protected control material.

Use exact machine-owned paths, plus mission-declared host output files. Protected contracts and instructions must remain denied even inside an otherwise host-owned directory. Snapshot all shippable paths first; classify afterward instead of attempting to identify “product paths.”

### HIW-R1-05 — STRUCTURAL: Refuse-and-park needs durable taint; the current fault path can still complete

Current fault handling does not implement the design’s posture. A protocol error can continue after a first failure, and if the completion gate passes, `ConcludeFaultedTurn` completes the mission because a broken envelope does not invalidate a built product. That is precisely wrong for this invariant: the product is the disputed evidence. [cycle.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/cycle.go:164>), [cycle.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/cycle.go:201>).

The host-wall violation needs a dedicated, immediate mission-park path that takes precedence over gate success. “Affected stream” is not mechanically available for unattributed writes, so parking only a stream is unsafe unless every authorized change and dispatch job has a runner-validated stream binding.

Not reverting is correct for evidence preservation, but the workspace must be durably marked tainted. No later host turn may take a new baseline until either:

- the workspace is restored to a known safe tree, or
- Wido explicitly adopts the disputed tree through a typed resolution that records what attribution and delegation claims are being waived.

A generic acknowledgement must not clear the taint.

### HIW-R1-06 — STRUCTURAL: `selfWork: [{path, reason}]` is too weak, and its policy requires Wido’s ruling

A path and prose reason do not establish which bytes are exempt, which stream incurred them, or how a cumulative ceiling is calculated. Mixed delegate and self-work edits to one file are also ambiguous.

If the exception remains, the runner should compute the residual patch after exact certified patches are applied. The return should at least bind the explanation to a stream and canonical paths; actual files and additions/deletions must be measured, not trusted from the host. Binary changes, submodules, symlinks, mode-only changes, dependencies, contracts, and deletions require an explicit policy rather than accidentally costing zero lines.

The ceiling must be cumulative per stream and per mission, not merely per turn. Otherwise the host can implement a product through repeated small turns, with one sham implementer satisfying the delegation floor.

More fundamentally, “the host never ships implementer work” and “small host product work is allowed” are different invariants. The existence and numeric size of this exception need Wido’s ruling. The existing 30-line prose waiver is a critic-chain waiver for delegated Markdown; it is not precedent for host implementation authority.

### HIW-R1-07 — STRUCTURAL: Ownership should be split across existing boundaries

A runner check is necessary, but it should not become a second conformance system.

- **Dispatch** should record and validate immutable mission, turn, stream, and role provenance. Current adjudication trusts the role and stream supplied in the host return rather than comparing all of them with the job record. [adjudicate.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/adjudicate.go:281>).
- **Conformance validation** should own exact patch/tree evidence and issue the durable integration authorization.
- **Missionrunner adjudication** should validate returned claims, perform the pre/post tree equation, and enforce taint and parking.
- **The completion gate** should describe the required operating discipline but should not attempt attribution.
- **The benchmark delegation floor** should remain a defense-in-depth outcome signal and consume runner-validated records.

This allocation keeps each important fact under the boundary that can actually prove it.

### HIW-R1-08 — MECHANICAL-GRAIN: The mission-only boundary is mechanically drawable

Bind the wall specifically to the missionrunner transition after a runner-created mission host exits and before that turn is accepted. The mission ID, turn ID, runner-owned pre-snapshot, and orchestrator return make that boundary objective. [loop.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/loop.go:1116>), [missionrunner.go](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/missionrunner.go:97>).

Do not put it in generic Git hooks, the commit wrapper, shared host adapters, or interactive orchestration helpers. Interactive development does not traverse missionrunner acceptance and therefore retains KI-27’s direct-implementation model. Add a boundary test proving an interactive direct commit is unaffected.

### HIW-R1-09 — MECHANICAL-GRAIN: The benchmark updates are underspecified

All four bm-2 manifests—bm-2, bm-2d, bm-2dc, and bm-2s—currently use outcome-only completion-gate language. Provisioning copies that text into the signed mission contract, so rewording it is useful, but it is guidance rather than enforcement. [provision.sh](</Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/provision.sh:438>).

Adding `selfWork` or stronger certification to the runtime schema also requires updating the benchmark’s mirrored evidence schema. Otherwise the extractor can reject newly valid records. [benchmark orchestrator schema](</Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/schemas/evidence/orchestrator.schema.json:1>).

The current delegation floor counts completed jobs appearing in accepted `dispatched` and `certified` lists. It does not prove a nonempty conformant diff, validated implementer role, or exact integration. [extractor.py](</Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/extractor.py:744>). It should be hardened to consume the new adjudicated certification facts, while remaining a second line of defense.

## Residual laundering analysis

| Shape | Layer that catches it under the current proposal |
|---|---|
| Zero dispatch plus a large product diff | Wall; delegation floor also fails later |
| Delegate touches one line in each target file; host writes the rest | Nothing: path-subset wall passes and floor passes |
| Host edits the delegate worktree before conformance | Nothing: both layers see apparently delegated output; stronger process isolation is required to prove authorship |
| Empty or token delegate jobs plus repeated small `selfWork` turns | Floor only if no qualifying sham implementer exists; otherwise nothing |
| Product code stored under broad allowed paths | Nothing when a sham delegate satisfies the floor |
| Code hidden under an allowed path and moved into product paths later | Later wall, unless a prior refusal/crash let the dirty workspace become the next baseline |
| A violating post-tree inherited as the next turn’s pre-tree | Nothing without durable taint and recovery checks |
| Host claims a wrong-role or foreign job as certified | Nothing reliable today; certification and dispatch provenance adjudication must catch it |
| Several genuine delegates provide exact disjoint patches | Wall should accept once exact patch composition replaces path authorization |
| Merge conflict requiring new product bytes | Must park; neither the floor nor a path allowlist can certify the host’s resolution |

Even after the revisions, a host able to mutate delegate worktrees can manufacture apparent delegate authorship. The wall can be a strong defect detector, but not an adversarial security boundary, unless delegate worktrees and conformance evidence are protected from the host process.

Evidence level: read the design, runner lifecycle, dispatch/conformance validation, schemas, KI-9/KI-27 records, bm-2 manifests, provisioning, and extractor. No tests were run and no files were modified.

Proposed review receipt, not written: host-implementer-wall round 1 — revise; exact-byte attribution, durable certification, cumulative self-work policy, and tainted-workspace recovery remain unresolved.

REVISE — structural findings remain
