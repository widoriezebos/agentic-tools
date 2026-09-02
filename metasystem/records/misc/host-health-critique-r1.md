# Host-health role design critique — round 1 (Sol)

Chain: design revision 1 -> critic host-health-crit1 (codex {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}, reviewed commit f69bfbe6abcc017039dd470adfab1d82d4d74f10), 2026-09-02. 7 material findings. Full return: artifacts/agents/host-health-crit1/rounds/1/return.json.

## HH-R1-LINUX-CPU-LIFETIME — high, material=True

CLAIM: On Debian, the process CPU threshold can stay silent for a long-lived process that suddenly becomes runaway. The design says procps pcpu is measured over the process lifetime and then claims that “the consecutive-tick rule below tolerates that.” It does not: a process idle for time T and hot for time t reports roughly t divided by T plus t, so it may never reach the 90 percent threshold during the three-tick window. The verified Linux platform therefore lacks the recent CPU signal the macOS implementation has.

EVIDENCE: metasystem/plans/host-health-role-design.md:94-98 acknowledges the incompatible semantics but treats the streak as a remedy. The procps manual states that CPU utilization covers the process's entire lifetime. By contrast, the installed macOS manual describes a decaying average over at most one minute. The flags are portable; the verdict meaning is not.

## HH-R1-CENSUS-PID-ONLY — high, material=True

CLAIM: The census ownership join discards the process identity that makes census ownership trustworthy. The design says, “The pid appears in the inventory ... ours,” and its fixture plants the fseventsd PID into a CUSTODY row. A census row identifies a process by PID plus start identity; a later ps row supplies only PID. PID reuse, or a stale census row, can therefore label an unrelated foreign process as ours. The separate census-freshness role only reports another verdict and does not gate this join.

EVIDENCE: metasystem/plans/host-health-role-design.md:169-173 and 188-190 specify PID-only matching; case 5 at lines 242-243 enshrines it. metasystem/internal/census/run.go:253-257 stores PID and start identity, while lines 300-315 require the complete identity for CUSTODY or ANNOUNCED ownership. metasystem/internal/steward/health.go:529-569 computes freshness as an independent role without authorizing consumers. A runtime-signature-free fseventsd CUSTODY inventory row is not a census-producible fixture.

## HH-R1-INVENTED-OWNERSHIP-SHAPES — high, material=True

CLAIM: The fallback ownership rules invent broader and narrower process shapes than the census and janitor use. Rule 2 calls any process ours when any argument names a path under the repository, metasystem root, or the directory containing InstallPath; a foreign Java, editor, or build process mentioning such a path is therefore called an engine process. Rule 3 requires a standalone metasystem word, so it misses valid script-based engine shapes. The resulting ownership label and remedy can be wrong in both directions.

EVIDENCE: metasystem/plans/host-health-role-design.md:174-185 defines path containment and word presence as ownership. metasystem/internal/census/signature.go:83-93 requires a runtime signature before scope, and metasystem/internal/census/run.go:204-258 applies scope only after that filter. metasystem/internal/janitor/killproof.go:41-63 lists exact shapes, including watch-background-jobs.sh and adapter scripts that do not require a standalone metasystem word.

## HH-R1-MEMORY-NOT-SCALED — medium, material=True

CLAIM: The fixed 8 GiB resident-memory threshold is not a portable host-pressure threshold because the reader does not measure physical or available memory. On the named 64 GB Mac it fires at only 12.5 percent of RAM, so a healthy large workload can alert; on a smaller VM it can be unreachable before memory exhaustion. If that VM has no swap, the design explicitly disables its only other memory-pressure signal. The healthy fixture merely assumes every process is under 8 GiB and therefore cannot validate the default.

EVIDENCE: metasystem/plans/host-health-role-design.md:79-85 lists no physical or available-memory fact and says zero swap disables the threshold. Lines 109-116 fix resident memory at 8 GiB. Lines 233-240 define healthy by already being under every threshold. metasystem/plans/host-health-role-critique-brief.md:26-30 requires the defaults to work for both the shared VM and the 64 GB Mac.

## HH-R1-HOST-ONLY-ATTRIBUTION — medium, material=True

CLAIM: A load-only or swap-only episode need not name an offender or its ownership, despite the goal requiring that information. Process findings appear only when a process crosses its own CPU or resident threshold, while a load finding blindly borrows the top CPU process's remedy and a swap finding borrows the top resident process's remedy. For example, 81 percent swap with all processes below 8 GiB produces a swap finding with no process name, while treating the largest resident process as causally responsible without evidence.

EVIDENCE: metasystem/plans/host-health-role-design.md:157-165 separates host findings from threshold-crossing process findings; lines 192-202 assign host findings the top consumer's remedy. The required contract at metasystem/plans/host-health-role-design-brief.md:39-47 says the verdict names the process, whether it is ours, and the remedy. None of the proposed fixture cases exercises a host-only crossing.

## HH-R1-CONFIG-REMEDY-BLIND — medium, material=True

CLAIM: The advertised configuration repair command will not diagnose the proposed invalid keys. The design makes a malformed threshold produce an unknown role with the remedy “metasystem config validate,” but it adds no validation rules for those six keys. An operator can therefore run the named remedy successfully and leave the role permanently unknown.

EVIDENCE: metasystem/plans/host-health-role-design.md:106-123 names the six bounded keys and the validator remedy, while its file list at lines 252-263 omits configuration validation. metasystem/internal/config/validate.go:330-346 validates an explicit numeric-key list that contains none of the host-health keys; metasystem/cmd/metasystem/config_verbs.go:130-155 reports only the problems returned by that validator.

## HH-R1-SWEEP-REMEDY-NOOP — medium, material=True

CLAIM: The command presented as the owned-process sweep remedy is report-only and cannot repair the dead role. The design emits “metasystem janitor orphans --root ... --older-than-min 0” without --apply, and its fixture checks only that text. The custody contract says that exact invocation merely prints what it would do. This contradicts the health verdict's repair-command contract and leaves the named owned offender running.

EVIDENCE: metasystem/plans/host-health-role-design.md:194-208 specifies the command, and case 4 at lines 241-242 checks only its rendering. metasystem/plans/proof-harness-custody-design.md:235-255 states that --apply performs the kill and omission is report-only. metasystem/internal/steward/health.go:78-89 describes Remedy as the exact command that repairs a non-alive result.

## Critic-declared gaps

- The critique brief does not declare the numbered failsafe round required by metasystem/skills/design-critique/SKILL.md, so this return cannot verify the critique chain's stop behavior beyond round 1.

- The four-hour fit with a correction round cannot be verified from the tree. metasystem/plans/host-health-role-design.md:265-272 says the precedent's elapsed time exists only on another machine and supports its estimate by line count alone; the proposed scope is roughly 715 lines before the corrections above.

- The generated runtime notice says this broad-read launcher cannot prove context isolation or the provider tool catalog. This return is therefore advisory despite using the requested model and an observed session identifier.
