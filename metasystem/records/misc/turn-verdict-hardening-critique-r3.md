# Turn-verdict hardening design — Sol critique round 3 (tvh-crit-4)

Reviewed: plans/turn-verdict-hardening-design.md revision 3 at commit f375e38ae63ede2365b2c4ab4a14b8f6d88752ad. Runtime codex (Sol lane), design-critic, read-only. Material findings: 4 of 4 — all four in HUMANSTOP (section 5, slices 4a/4b) and the R3 Move quoting (section 1.2.1); slices 1a through 3 drew no finding.

## Findings

### TVH-R2-HUMANSTOP-SEAT-AUTHORITY-UNSPECIFIED — critical, MATERIAL

The Round-2 HUMANSTOP seat-authority closure remains partial: a delegate that invokes the new verb directly can be classified as its ancestor MAIN and mint that main seat's temporary-relay stop marker.

Evidence: Section 5.2 says the relay command uses `lease.ClassifyVerbAt(..., int64(os.Getppid()))`, that the classified MAIN seat becomes `machine`, `lineage`, and `relayedBy`, and that DELEGATE is refused. But metasystem/internal/lease/directinvoker.go:11-18 explicitly documents that the general classifier checks the exact caller only for an announcement and starts runtime-signature checks at its parent, so an agent binary directly spawning a verb is judged by its ancestors. metasystem/internal/lease/classify.go:324-342 implements that order and can return an ancestor's MAIN announcement. metasystem/internal/humanauthority/authority.go:228-237 then permits fallback to the temporary proof whenever words and a review date are supplied. The design needs an exact-invoker distinction; otherwise the stated DELEGATE refusal is bypassed and ready work can be excused by a marker minted outside the authorized seat.

### TVH-R3-F11-DISCOVERED-AFTER-HUMANSTOP — high, MATERIAL

A valid HUMANSTOP cannot rescue F11, the verdict-state lock or write failure, in the prescribed execution order.

Evidence: Section 3.2(d) says the marker phase runs and `then the state-file phase` attempts its lock and write. Section 5.3 says marker comparison occurs only when the decision so far is already a class-A block, yet F11 cannot be known until that later state-file phase fails. Section 7 nevertheless says F11 blocks unless step 3 consumes, and slice 4b requires `TestHumanstopRescuesStateFileFailure`. An implementer must either move state handling before marker comparison or perform another compare-and-consume after F11; the stated single marker phase cannot satisfy the required outcome and can repeatedly refuse despite an unconsumed valid human stop.

### TVH-R3-HUMANSTOP-MARKER-PATH-UNSAFE — high, MATERIAL

The HUMANSTOP marker path is not a safe one-file-per-seat schema because it embeds an unrestricted machine nickname as a path component.

Evidence: Section 5.1 fixes the path as `<root>/artifacts/agents/humanstop/<machine>+<lineage>.json`, and section 5.2 says it is built from `goal.ResolveMachine(root)`. metasystem/internal/goal/actor.go:21-27 returns any non-empty trimmed git-config value without path validation; metasystem/internal/goal/verbs.go:1898-1913 confirms the machine vocabulary is any non-whitespace word. A currently legal nickname such as `region/a` creates nested marker paths, while `../jobs/a` escapes the humanstop directory. That defeats the claimed namespace and can place atomic writes outside the marker lock owner's directory. The design must specify a traversal-safe, injective filename encoding and boundary fixtures.

### TVH-R1-R3-NAMES-ILLEGAL-EXIT — medium, MATERIAL

The Round-1 R3 recovery command remains illegal for supported repository roots containing spaces or shell metacharacters.

Evidence: Section 1.2.1 promises byte-exact, engine-accepted commands but renders `--root <root>` without quoting on every park, release, and claim line; it discusses quoting only for the lineage. metasystem/cmd/metasystem/goalsync_mutations.go:104-159 requires the root to arrive as one flag value. The repository already owns the relevant rule in metasystem/cmd/metasystem/goalsync_verbs.go:266-270: shell-single-quote paths so spaces, dollar signs, backticks, and quotes arrive as bytes. The planned parser fixture uses no hostile root. An implementer following the literal template will emit a repeatedly refused Move in a repository whose path contains a space.

## Gaps declared by the critic

- No Go tests, shell fixtures, live Stop events, or executable specimen replays were run; the brief required read-only verification and prohibited running Go.
- The doubled builder-minute estimates remain unsupported until slice 1a produces retained authored-diff and wall-clock evidence. This critique verified reservation arithmetic and partitioning, not duration accuracy.
- Codex and Devin live Stop delivery, provider timeout behavior, and fleet-wide EPOCHREALTIME availability remain unobserved from repository source; revision 3 records these as residuals and supplies fallback behavior.
- The harness classifies this broad-read runtime as advisory and does not prove context isolation or an independently observed tool catalog.
