The draft needs revision. Checked by reading the current repository and benchmark kit; 12 material findings remain: 10 structural and 2 mechanical-grain. No files were modified and no tests were run.

1. **DH-R1-01 — CRITICAL — STRUCTURAL — Automatic `go clean -cache` has machine-wide blast radius.**

   The draft makes unconditional cache cleaning a pressure-recovery action ([disk-hygiene-design.md:63](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:63)). The gate does not set a private `GOCACHE`; all of its builds, tests, and `go run` tools use the user’s default cache ([go-gate.sh:206](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-gate.sh:206), [go-gate.sh:219](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-gate.sh:219)). Cleaning it can evict concurrent work from unrelated checkouts and still race new writers.

   The design must instead require suite and benchmark Go commands to use a metasystem-owned, checkout-keyed cache with explicit custody and a cap. Automatic recovery may clean only that owned cache. A legacy oversized shared cache is diagnosed and left for explicit operator action unless exclusivity is independently proven.

2. **DH-R1-02 — CRITICAL — STRUCTURAL — `/tmp/tmp.*` cannot safely identify metasystem ownership.**

   The class model assumes an absolute base plus pattern ([disk-hygiene-design.md:22](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:22)) and assigns `/tmp/tmp.*` to the suite ([disk-hygiene-design.md:64](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:64)). The actual suite uses unqualified `mktemp -d` for witness state, snapshots, and its primary fixture root ([validate-metasystem.sh:137](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:137), [validate-metasystem.sh:674](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:674)); `go-gate.sh` likewise creates generic temporary files ([go-gate.sh:230](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-gate.sh:230)). That pattern also matches unrelated applications’ temporary files.

   The design must require writers to migrate under a dedicated physical namespace keyed by checkout and run identity. Each root needs an ownership record containing class, process identity including start time, and creation time. Sweeping must reject symlinks, mount crossings, malformed ownership records, and any path outside that canonical namespace. Existing generic `tmp.*` paths can only be reported, never deleted.

3. **DH-R1-03 — CRITICAL — STRUCTURAL — Age and a single sweeper do not prove that a writer is finished.**

   The proposed sweep acts by age and ceiling and claims safety from single-writer discipline ([disk-hygiene-design.md:43](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:43)). That serializes sweepers, not artifact writers. Current cleanup explicitly waits because supervision processes can still write after shutdown begins ([validate-metasystem.sh:732](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:732)). Run descendants may also continue changing a log after the recorded leader dies ([conclude.go:105](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/run/conclude.go:105)).

   The design must give every destructive class a class-specific quiescence proof: exact PID plus start time, process-group emptiness, terminal job/run lifecycle, released benchmark custody, or another owning state machine. Unknown or unreadable liveness must report and retain. Manual and pressure sweeps need the same proof; age alone is never an orphan proof.

4. **DH-R1-04 — CRITICAL — STRUCTURAL — `distill-then-delete` has no crash-safe or race-safe transaction.**

   The lifecycle merely says to run a distiller and then delete ([disk-hygiene-design.md:27](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:27)). It does not prevent the source changing between distillation and unlink, establish durable residue before deletion, or define recovery after any intermediate crash. The evidence collector’s existing standard is materially stronger: deletion requires a byte-identical durable copy ([gc.go:659](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:659)), and publication uses a temporary file plus rename ([gc.go:722](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:722)).

   The design must specify a transaction: prove quiescence; atomically claim the exact object, preferably by same-filesystem rename into an owned quarantine; durably record intent; write and sync residue through atomic publication; verify it; delete that quarantined object; then record completion. Every step must be idempotently recoverable. Failure or source mutation retains the source.

5. **DH-R1-05 — CRITICAL — STRUCTURAL — Pressure mode has neither a reclaim target nor filesystem-aware semantics.**

   Pressure is defined only as “tighter ceilings” with evidence floors ([disk-hygiene-design.md:51](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:51)), although floors are absent from the declared class schema. A single free-space check is insufficient: benchmark targets may resolve to any explicit path or configured trials root ([provision.sh:55](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/provision.sh:55)), while the repository, `TMPDIR`, target, evidence sibling, and Go cache can occupy different filesystems. On an already-full filesystem, residue and the required report may themselves be unwritable.

   The design must define required bytes and floor per physical filesystem, calculate the deficit before acting, and reclaim only eligible artifacts on that same device. Evidence floors must be schema fields that pressure cannot override. A bounded emergency reporting reserve or preallocated journal is required. The result must distinguish `recovered` from `still-below-floor`; the latter refuses startup and reports which owned, unknown, or protected bytes prevented recovery.

6. **DH-R1-06 — CRITICAL — STRUCTURAL — The lifecycle algebra cannot represent the existing archival contracts.**

   Every class must choose exactly one of three lifecycles ([disk-hygiene-design.md:25](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:25)), yet the evidence row uses an undefined “pointer” to another enforcer ([disk-hygiene-design.md:70](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:70)). Neither `archive-with-cap` nor `distill-then-delete` represents “copy whole, verify byte identity, age, then delete,” which is the evidence collector’s actual contract ([gc.go:659](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:659)) and the benchmark kit’s stated doctrine ([README.md:34](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/README.md:34)).

   The design must distinguish local capped retention from `archive-verified-then-delete`, and separate lifecycle policy from enforcement ownership. A delegated owner is not another lifecycle. Durable archival requires a named destination, manifest/digest verification, minimum retention, and deletion only after verification.

7. **DH-R1-07 — HIGH — STRUCTURAL — There is no interface by which the kit can declare its dynamic classes.**

   The registry is described as Go-owned inside `internal/janitor` ([disk-hygiene-design.md:19](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:19)), while the kit supposedly declares its own classes ([disk-hygiene-design.md:68](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:68)). Actual targets are dynamically resolved and may be arbitrary absolute paths ([provision.sh:62](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/provision.sh:62)). Each provision also creates an omitted `.origin.git` sibling in addition to `.evidence` ([provision.sh:81](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/provision.sh:81)).

   The design must define a versioned kit-to-engine registration surface. Provisioning registers the canonical target, origin, evidence root, cohort directory, identity, custody, and lifecycle before writing. The kit owns those declarations; the engine validates containment and executes only against registered instances. Kit-specific paths must not be compiled into the core registry.

8. **DH-R1-08 — HIGH — STRUCTURAL — Evidence integration creates an undefined second owner and relies on a nonexistent global GC lock.**

   The draft says the evidence GC supplies “single writer” discipline and that the registry points at it ([disk-hygiene-design.md:49](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:49)). Current code explicitly supports two collectors running simultaneously ([gc.go:514](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:514)), and every stop hook may invoke it ([supervision-hook.sh:123](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:123)). Its ownership is encoded through `reservedDirs` and detailed chain-state rules, not a generic path class ([gc.go:42](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:42)).

   The design must say that evidence GC remains the sole policy and deletion owner for its trees. Janitor must neither walk nor pressure-age them. Integration is an orchestration boundary returning a structured GC outcome for inclusion in the janitor report. If one combined pass requires serialization, both the existing GC entrypoint and janitor must share the same operation-wide lock. New infrastructure directories under `artifacts/agents` must be reserved or placed outside the GC’s chain namespace.

9. **DH-R1-09 — HIGH — STRUCTURAL — Run-record logs cannot be safely managed as a generic path class.**

   The registry assumes base-and-pattern discovery, but the draft says the distiller follows an individual run record’s log path ([disk-hygiene-design.md:90](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:90)). Run records accept any resolved path inside the repository or a temporary directory ([verbs.go:365](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/run/verbs.go:365)). Their existing prune operation deletes the record and sidecars but not the log ([verbs.go:328](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/run/verbs.go:328)), so discovery can disappear before the proposed distiller runs.

   The design must assign registered-log lifecycle to `internal/run`. The run state machine seals and claims the log only after terminality and process-group quiescence, records the residue path and completion state, and refuses record pruning until that lifecycle finishes. Generic path sweeping must never discover or delete run logs independently.

10. **DH-R1-10 — HIGH — STRUCTURAL — The scratchpad class has no runtime-neutral ownership source.**

    The draft assigns session scratchpad worktrees to deletion and lists `dispatch.sh` as their declaration point ([disk-hygiene-design.md:69](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:69), [disk-hygiene-design.md:113](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:113)). Dispatch only owns worktrees under `artifacts/agents/worktrees` ([dispatch.sh:34](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:34)); otherwise it accepts an arbitrary caller-provided workspace ([dispatch.sh:916](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:916)). It neither allocates nor records a session scratchpad base.

    The design must split dispatch-owned worktrees from externally created session scratchpads. Only runtime-neutral roots explicitly allocated or registered with custody may be swept. If an adapter can report a provider-created scratch root, that belongs behind an adapter capability and is recorded in the job lifecycle; the janitor core must not encode a provider/runtime-specific pathname. Unregistered scratchpads remain report-only.

11. **DH-R1-11 — HIGH — MECHANICAL-GRAIN — The writer census omits existing unbounded classes.**

    The “known surfaces, each assigned” table ([disk-hygiene-design.md:59](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:59)) misses at least:

    - Go-gate failure logs under `artifacts/agents/gate-failures`, preserved without a cap ([go-gate.sh:230](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-gate.sh:230)).
    - Receipt-probe failures under the persistent `${TMPDIR}/receipt-evidence` directory ([validate-metasystem.sh:2002](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:2002)).
    - The append-only supervision `hooks.log` ([supervision-hook.sh:126](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:126), [supervision-hook.sh:145](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:145)).
    - Benchmark `.origin.git` siblings, as noted in finding 7.

    The design must add each writer and its exact lifecycle to the inventory. Unknown-accumulation reporting is a diagnostic fallback, not compliance with “every byte written gets a lifecycle.”

12. **DH-R1-12 — HIGH — MECHANICAL-GRAIN — The janitor report’s cap is self-referential and does not bound one append-only file.**

    Every reap appends to “a janitor REPORT,” while that report is itself merely called an `archive-with-cap` class ([disk-hygiene-design.md:43](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:43)). A count/oldest-first cap does not bound a single ever-growing file. Reaping an old report also recursively requires reporting that reap, with no defined destination or ordering. The proposed tests only say “the report’s own cap,” without specifying these semantics ([disk-hygiene-design.md:100](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:100)).

    The design must require one immutable, atomically published, size- and entry-bounded report per sweep. The new report records any old-report eviction without recursively writing into the file being removed. Validation must cover report rotation, crash boundaries, an unwritable/full report filesystem, concurrent sweeps, symlink swaps, live-writer refusal, and mutation during distillation.

REVISE — structural findings remain
DH1-DONE
