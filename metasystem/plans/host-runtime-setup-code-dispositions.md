# Host setup code review dispositions

Goal: `host-runtime-setup`. Code critic: `host-setup-code-crit1`, round 1,
reviewed tree `f89d4c37f4c57415d38ded226f2c63ea6f35972f`.
The critic returned six material findings and two non-material observations.
All six material findings are accepted corrections; none is waived or yet
claimed fixed in this reviewed tree. The next implementation round owns them.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HOST-C1-001 | accepted | The coordinator's native N2 log reproduces the missing launcher diagnostic. The fixture runs the raw template after command rendering moved into setup. | Make supervision-hook-fixtures.sh execute the installed generated launcher from an unresolvable Git cwd; retain block and diagnostic assertions. |
| HOST-C1-002 | accepted | Read the stub at supervision-hook-fixtures.sh:884: it emits only pid and pidStartedAt. The new entry reads runtime first and records a Stop failure when absent. | Emit the fixture's Claude runtime and exercise the full existing bed outside the delegate sandbox. |
| HOST-C1-003 | accepted | The new validator expands empty setup_mode and registration_roots under set -u; the critic reproduced unbound-variable errors with this host's /bin/bash 3.2. | Use the file's existing Bash 3.2-safe empty-array convention and prove linked/copy/none paths. |
| HOST-C1-004 | accepted | Adoption represents none as an empty configured runtime value, while setup explicitly rejects an empty provided --runtimes argument. | Translate the lawful empty configuration to none, or skip the host registration check when none is selected. |
| HOST-C1-005 | accepted | Read classify.go:255-264 and evidence/gc.go:447: existing job scanning tolerates disappearing records, and GC actually removes them. The new scan's unconditional read error creates a real race. | Tolerate ENOENT only for an unhinted scan entry; retain refusal for a missing explicitly hinted job and for unreadable/corrupt evidence. Add a deterministic regression, preserving exact identity checks. No broad parser refactor is required to fix the named defect. |
| HOST-C1-006 | accepted | The critic ran a cosmetic rewrite with preserved foreign env data: public hooks check passed but setup --check failed. The setup owner compares re-serialized bytes. | Treat structurally correct live hooks as ready and preserve their original bytes when no contract change is needed, including harmless sibling grouping and metadata. Retain real drift/malformed/conflict checks. |
| HOST-C1-007 | noted | Dead legacy self-check code is optional cleanup and does not change the brief's behavior or proof. | none |
| HOST-C1-008 | noted | The degraded-path receipt diagnostic change is outside the accepted normal notice/Stop guarantee; the failed launcher still reports failure and this observation was non-material. | none |

Additional coordinator findings, also required in the correction:

- A1: `hooks check` accepted async=true on the owned Claude Stop handler. Official
  Claude hooks documentation states that asynchronous hooks cannot enforce a
  blocking decision. Require the synchronous semantics of owned handlers while
  allowing equivalent default false/absent values and harmless metadata. Proof:
  `artifacts/host-runtime-setup/async-check-probe.json`.
- S1: the generated Codex SessionStart matcher matches startup, resume, compact
  and misses clear. The old shipped matcher covered clear, and the official
  Codex hooks reference still lists all four sources. Preserve all four and
  add matcher coverage. Proof: `artifacts/host-runtime-setup/codex-start-sources-probe.json`.

The reviewer did not run native fixtures by instruction and did not recompute
the immutable tree from its read-only sandbox. Main-issued conformance and
main-owned native/full verification own those facts. The runtime's advisory
context-isolation classification is retained; this change makes no claim of
perfect prompt/context isolation. No completion claim follows from this join.
