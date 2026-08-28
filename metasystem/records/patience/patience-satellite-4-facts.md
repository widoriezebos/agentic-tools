# Satellite 4 fact pass

## Q1 — Shipped stop-loss fuse and ledger grammar

**F Q1.1.** The shipped shell stop-loss checker fires for any `falsified-dead-end`, two `no-progress` classifications, exhaustion of the declared cycle budget, or exhaustion of the trailing no-gain budget; `unresolved` does not count as `no-progress`.

`scripts/assert-stop-loss.sh:9-19`

> Reads the cycle classifications from an investigation ledger and blocks  
> further cycles when a machine-checkable stop-loss trigger has fired:
>
>     - any cycle classified falsified-dead-end
>     - two or more cycles classified no-progress
>     - as many cycles as the declared "Cycle budget:" line (when present)
>     - as many trailing cycles without a contract-improved as the declared
>       "No-gain budget:" line (when present; improve mode sets 3)
>
> unresolved (a valid measurement inside a declared noise floor) never counts  
> toward the no-progress trigger; only a declared no-gain budget bounds it.

**F Q1.2.** The shell checker extracts classifications and budgets from Markdown lines rather than using a separate state counter.

`scripts/assert-stop-loss.sh:41-48`

> classifications=$(sed -n 's/^- Classification: \([^;]*\).*/\1/p' "$ledger")  
> dead_ends=$(printf '%s\n' "$classifications" | grep -c '^falsified-dead-end$' || true)  
> no_progress=$(printf '%s\n' "$classifications" | grep -c '^no-progress$' || true)  
> total_cycles=$(grep -c '^### Cycle ' "$ledger" || true)  
> cycle_budget=$(sed -n 's/^- Cycle budget: \([0-9][0-9]*\)$/\1/p' "$ledger" | head -n 1)  
> no_gain_budget=$(sed -n 's/^- No-gain budget: \([0-9][0-9]*\)$/\1/p' "$ledger" | head -n 1)

**F Q1.3.** The shell checker’s four expiry branches return failure immediately.

`scripts/assert-stop-loss.sh:50-75`

> if (( dead_ends > 0 )); then  
>   echo "stop-loss triggered: falsified-dead-end" >&2  
>   exit 1  
> fi
>
> if (( no_progress >= 2 )); then  
>   echo "stop-loss triggered: two no-progress cycles" >&2  
>   exit 1  
> fi
>
> if [[ -n "$cycle_budget" ]] && (( total_cycles >= cycle_budget )); then  
>   echo "stop-loss triggered: cycle budget exhausted (${total_cycles}/${cycle_budget})" >&2  
>   exit 1  
> fi
>
> if [[ -n "$no_gain_budget" ]] && (( trailing_no_gain >= no_gain_budget )); then  
>   echo "stop-loss triggered: no-gain budget exhausted (${trailing_no_gain}/${no_gain_budget})" >&2  
>   exit 1  
> fi

**F Q1.4.** Mission execution has a second shipped fuse: a Go replay over the sealed contract and ledger. Its comments explicitly leave the shell checker in place for non-mission callers and exclude cycle annotations from fuse input.

`internal/missionrunner/stoploss.go:3-16`

> // The mission stop-loss verdict is intentionally a pure replay of  
> // (sealed contract, ledger). No cached counter and no state field participates  
> // in the verdict.  
> //  
> // The shell check enforced by scripts/assert-stop-loss.sh remains the  
> // compatibility path for non-mission callers. Mission execution uses this  
> // replay before launch, after every completed turn, and while resuming a  
> // mid-mission state. Non-mission callers keep scripts/assert-stop-loss.sh untouched.  
> //  
> // Replay invariant: the replay reads ONLY classification, best, and reset  
> // lines. Cycle-block annotations are part of the audit trail, never fuse input.

**F Q1.5.** Under sealed stop-loss semantics, `contract-improved` and explicit reset events clear stagnation; only `no-progress` and `unresolved` increment it.

`internal/missionrunner/stoploss.go:212-233`

> for _, event := range events {  
> 	if event.Reset {  
> 		stagnant = 0  
> 		continue  
> 	}  
> 	if event.Cycle == nil {  
> 		continue  
> 	}  
> 	cycles++  
> 	if event.Cycle.Best {  
> 		stagnant = 0  
> 		continue  
> 	}  
> 	switch event.Cycle.Classification {  
> 	case mission.ClassNoProgress, mission.ClassUnresolved:  
> 		stagnant++  
> 	}  
> }

**F Q1.6.** Sealed replay gives the total-cycle limit precedence over the no-gain limit.

`internal/missionrunner/stoploss.go:235-250`

> switch {  
> case cycleBudget > 0 && cycles >= cycleBudget:  
> 	return StopLossVerdict{  
> 		Triggered: true,  
> 		Kind:      mission.StopLossCycleBudget,  
> 		Count:     cycles,  
> 		Limit:     cycleBudget,  
> 	}, nil  
> case noGainBudget > 0 && stagnant >= noGainBudget:  
> 	return StopLossVerdict{  
> 		Triggered: true,  
> 		Kind:      mission.StopLossNoGainBudget,  
> 		Count:     stagnant,  
> 		Limit:     noGainBudget,  
> 	}, nil

**F Q1.7.** Missing stop-loss semantics selects legacy replay; semantics version 2 selects the sealed-contract replay.

`internal/missionrunner/stoploss.go:300-340`

> semantics := strings.TrimSpace(state.StopLossSemantics)  
> if semantics == "" {  
> 	semantics = StopLossSemanticsLegacy  
> }  
> switch semantics {  
> case StopLossSemanticsLegacy:  
> 	return replayLegacyStopLoss(ledgerPath)  
> case StopLossSemanticsSealed:  
> 	return replaySealedStopLoss(contract, ledgerPath)  
> default:  
> 	return StopLossVerdict{}, fmt.Errorf("unsupported stop-loss semantics %q", semantics)  
> }

**F Q1.8.** The sealed replay reads its two limits from the mission contract’s `ledger.cycle-budget` and `ledger.no-gain-budget` fields.

`internal/missionrunner/stoploss.go:343-362`

> cycleBudget, err := positiveContractInt(contract, "ledger.cycle-budget")  
> if err != nil {  
> 	return StopLossVerdict{}, err  
> }  
> noGainBudget, err := positiveContractInt(contract, "ledger.no-gain-budget")  
> if err != nil {  
> 	return StopLossVerdict{}, err  
> }

**F Q1.9.** The ledger package declares one atomic writer and the complete classification vocabulary.

`internal/mission/ledger.go:1-5`

> // Package mission owns the canonical mission contract and investigation-ledger  
> // formats. Ledger mutations are serialized and replaced atomically so readers  
> // never observe a partially appended cycle.

`internal/mission/ledger.go:22-30`

> var knownClassifications = map[string]struct{}{  
> 	"contract-improved":    {},  
> 	"falsified-continue":   {},  
> 	"falsified-dead-end":   {},  
> 	"no-progress":          {},  
> 	"unresolved":           {},  
> 	"invalid-run":          {},  
> }

**F Q1.10.** The ledger’s recognized structural lines are two positive-integer budget lines, contiguous cycle headings, classification lines, hexadecimal candidate SHAs, semicolon-delimited measurement lines, and bounded reset lines.

`internal/mission/ledger.go:41-53`

> cycleBudgetRe      = regexp.MustCompile(`(?m)^- Cycle budget: ([1-9][0-9]*)$`)  
> noGainBudgetRe     = regexp.MustCompile(`(?m)^- No-gain budget: ([1-9][0-9]*)$`)  
> headingRe          = regexp.MustCompile(`(?m)^### Cycle ([1-9][0-9]*)$`)  
> classificationRe   = regexp.MustCompile(`(?m)^- Classification: ([a-z-]+); candidate-sha=([^;\n]+); observed=(.*)$`)  
> classPrefixRe      = regexp.MustCompile(`(?m)^- Classification:`)  
> shaRe              = regexp.MustCompile(`^[0-9a-f]{40,64}$`)  
> measurementLineRe  = regexp.MustCompile(`^([a-z-]+); candidate-sha=([^;\n]+); observed=(.*)$`)  
> resetLineRe        = regexp.MustCompile(`^Stop-loss reset: ask=([^;\n]+); reason=(.*)$`)

**F Q1.11.** Four annotation prefixes are recognized, while writer-produced annotations must match one of four stricter complete-line forms.

`internal/mission/ledger.go:54-67`

> annotationReadRe = regexp.MustCompile(`(?m)^- (Return:|Outcome:|Drain:|Landed unconsumed:).*$`)  
> annotationWriteRe = regexp.MustCompile(  
> 	`^(Return: rejected:.+|Outcome: capped|Drain: stalled:(?:0|[1-9][0-9]*)|Landed unconsumed: chain=[^ \t]+ round=(?:[1-9][0-9]*|invalid|unreadable) path=[^ \t]+)$`,  
> )

**F Q1.12.** Stop-loss verdict kinds and reset reasons are bounded vocabularies or bounded text; reset reasons are flattened and limited to 500 characters.

`internal/mission/ledger.go:70-80`

> const (  
> 	StopLossCycleBudget  = "cycle-budget"  
> 	StopLossNoGainBudget = "no-gain-budget"  
> 	StopLossDeadEnd      = "falsified-dead-end"  
> 	StopLossNoProgress   = "no-progress"  
> 	StopLossResetMaxLen  = 500  
> )

**F Q1.13.** Writer-side rejection annotations are made single-line and truncated to a 200-character reason.

`internal/mission/ledger.go:111-131`

> reason = strings.Join(strings.Fields(reason), " ")  
> if reason == "" {  
> 	reason = "unspecified"  
> }  
> if len(reason) > 200 {  
> 	reason = reason[:200]  
> }  
> return "Return: rejected:" + reason

**F Q1.14.** Ledger verification requires exactly one cycle-budget line, one no-gain-budget line, contiguous cycle numbers beginning at one, and exactly one known classification line per cycle block.

`internal/mission/ledger.go:134-176`

> cycleBudgetMatches := cycleBudgetRe.FindAllStringSubmatch(text, -1)  
> if len(cycleBudgetMatches) != 1 {  
> 	return fmt.Errorf("ledger must contain exactly one Cycle budget line")  
> }  
> noGainBudgetMatches := noGainBudgetRe.FindAllStringSubmatch(text, -1)  
> if len(noGainBudgetMatches) != 1 {  
> 	return fmt.Errorf("ledger must contain exactly one No-gain budget line")  
> }  
> headings := headingRe.FindAllStringSubmatchIndex(text, -1)  
> for i, heading := range headings {  
> 	number, _ := strconv.Atoi(text[heading[2]:heading[3]])  
> 	if number != i+1 {  
> 		return fmt.Errorf("cycle headings must be contiguous: expected %d, got %d", i+1, number)  
> 	}  
> 	...  
> 	classes := classificationRe.FindAllStringSubmatch(block, -1)  
> 	if len(classes) != 1 {  
> 		return fmt.Errorf("cycle %d must contain exactly one canonical Classification line", number)  
> 	}  
> 	if _, ok := knownClassifications[classes[0][1]]; !ok {  
> 		return fmt.Errorf("cycle %d has unknown classification %q", number, classes[0][1])  
> 	}  
> }

**F Q1.15.** Ledger initialization writes the fixed heading followed by the two budget lines.

`internal/mission/ledger.go:179-194`

> text := fmt.Sprintf(  
> 	"# Mission Ledger\n\n- Cycle budget: %d\n- No-gain budget: %d\n",  
> 	cycleBudget,  
> 	noGainBudget,  
> )

**F Q1.16.** The canonical cycle writer emits one heading and one classification record with classification, candidate SHA, observed value, and an optional best suffix; validated annotations follow as separate `- ` lines.

`internal/mission/ledger.go:197-253`

> entry := fmt.Sprintf(  
> 	"\n\n### Cycle %d\n- Classification: %s; candidate-sha=%s; observed=%s%s\n",  
> 	cycle,  
> 	classification,  
> 	candidateSHA,  
> 	observed,  
> 	bestSuffix,  
> )  
> for _, annotation := range annotations {  
> 	entry += "- " + annotation + "\n"  
> }

**F Q1.17.** Annotation-only mutation is restricted to the final cycle and uses the same strict annotation grammar.

`internal/mission/ledger.go:256-294`

> if cycle != count {  
> 	return fmt.Errorf("annotations may only be appended to the final cycle %d", count)  
> }  
> for _, annotation := range annotations {  
> 	if !annotationWriteRe.MatchString(annotation) {  
> 		return fmt.Errorf("invalid ledger annotation %q", annotation)  
> 	}  
> }

**F Q1.18.** A reset is written as a standalone `Stop-loss reset` line only after both ask and reason have been validated and bounded.

`internal/mission/ledger.go:297-328`

> if strings.TrimSpace(askID) == "" || strings.ContainsAny(askID, ";\n\r") {  
> 	return fmt.Errorf("invalid reset ask id")  
> }  
> reason = strings.Join(strings.Fields(reason), " ")  
> if reason == "" {  
> 	return fmt.Errorf("reset reason is required")  
> }  
> if len(reason) > StopLossResetMaxLen {  
> 	return fmt.Errorf("reset reason exceeds %d characters", StopLossResetMaxLen)  
> }  
> entry := fmt.Sprintf("\nStop-loss reset: ask=%s; reason=%s\n", askID, reason)

**F Q1.19.** The public `mission ledger` command owns init, append, verify, and count; the CLI append path calls the same ledger writer.

`cmd/metasystem/mission.go:16-17`

> ledger       canonical investigation-ledger operations: init, append, verify, count

`cmd/metasystem/mission.go:27-45`

> return mission.InitLedger(args[0], cycleBudget, noGainBudget)  
> ...  
> return mission.AppendCycle(args[0], cycle, classification, candidateSHA, observed, best, nil)

**F Q1.20.** The mission runner initializes the ledger from the sealed contract and uses the canonical append function for cycle bookings.

`internal/missionrunner/loop.go:313-347`

> cycleBudget, err := positiveContractInt(contract, "ledger.cycle-budget")  
> ...  
> noGainBudget, err := positiveContractInt(contract, "ledger.no-gain-budget")  
> ...  
> if err := mission.InitLedger(paths.Ledger, cycleBudget, noGainBudget); err != nil {  
> 	return err  
> }

`internal/missionrunner/loop.go:636-649`

> best, err := bestMarker(paths.Ledger, measurement.Classification)  
> if err != nil {  
> 	return err  
> }  
> return mission.AppendCycle(  
> 	paths.Ledger,  
> 	cycle,  
> 	measurement.Classification,  
> 	measurement.CandidateSHA,  
> 	measurement.Observed,  
> 	best,  
> 	annotations,  
> )

**F Q1.21.** Landed-but-unconsumed returns are added after the cycle by the annotation-only writer.

`internal/missionrunner/loop.go:715-739`

> if len(landedAnnotations) > 0 {  
> 	if err := mission.AppendAnnotations(paths.Ledger, cycle, landedAnnotations); err != nil {  
> 		return err  
> 	}  
> }

**F Q1.22.** An answered mission ask writes a canonical stop-loss reset through `mission.AppendReset`.

`internal/missionrunner/answer.go:143-170`

> if err := mission.AppendReset(paths.Ledger, ask.ID, reason); err != nil {  
> 	return err  
> }

**F Q1.23.** A triggered mission fuse is durably parked by writing the ask, mission state, and anchor.

`internal/missionrunner/loop.go:592-624`

> func parkStopLoss(paths Paths, state *State, verdict StopLossVerdict) error {  
> 	ask, err := stopLossAsk(state, verdict)  
> 	if err != nil {  
> 		return err  
> 	}  
> 	return applyPark(paths, state, ask)  
> }  
> ...  
> if err := writeAsks(paths.Asks, state.Asks); err != nil {  
> 	return err  
> }  
> if err := WriteState(paths.State, state); err != nil {  
> 	return err  
> }  
> return writeAnchor(paths, state)

**F Q1.24.** The premise that the complete patience system is already shipped is false: the repository describes the stop-loss core as shipped and the satellite mechanisms as designed but unbuilt.

`docs/patience.md:3-8`

> Status: the deterministic stop-loss core is shipped. The patience floors,  
> novelty proof, auditable gain, frontier receipt, watchdog classification, and  
> the associated fixture assertions described below are designed but not yet  
> implemented.

## Q2 — Configuration grammar, contract sealing, and cap surfaces

**F Q2.1.** The committed configuration currently supplies runtime names and operational defaults for retro cadence, refactor cadence, watcher cadence, log size, census share, dispatch cap, inline size, and capability age.

`metasystem.conf:1-15`

> metasystem.version=1  
> runtimes=claude,codex,devin  
> retro.max-age-days=25  
> retro.max-receipts=30  
> refactor.max-age-min=1440  
> refactor.max-commits=40  
> watch.stale-min=20  
> watch.cap-min=180  
> watch.interval-sec=60  
> log.max-mb=10  
> census.max-interval-share-percent=50  
> dispatch.cap-min=120  
> dispatch.inline-max-kb=64  
> capability.max-age-days=30

**F Q2.2.** Local configuration is documented as an ignored-machine override and is limited by validation to cap keys.

`metasystem.conf:17-21`

> # Machine-local cap overrides belong in metasystem.local.conf, which is  
> # intentionally ignored by git. Only cap.min.* keys are accepted there.

`internal/config/validate.go:68-95`

> if !strings.HasPrefix(key, "cap.min.") {  
> 	errs = append(errs, fmt.Errorf("%s:%d: only cap.min.* keys are allowed", path, lineNo))  
> 	continue  
> }

**F Q2.3.** The committed role roster defines each role’s default runtime and model.

`metasystem.conf:27-40`

> role.architect.runtime=claude  
> role.architect.model=opus  
> role.orchestrator.runtime=claude  
> role.orchestrator.model=opus  
> role.implementer.runtime=codex  
> role.implementer.model=gpt-5.3-codex  
> role.design-critic.runtime=claude  
> role.design-critic.model=opus  
> role.code-critic.runtime=codex  
> role.code-critic.model=gpt-5.3-codex  
> role.verifier.runtime=codex  
> role.verifier.model=gpt-5.3-codex  
> role.researcher.runtime=devin  
> role.researcher.model=sonnet

**F Q2.4.** A configuration key must begin with a lowercase letter or digit and thereafter contain only lowercase letters, digits, dots, or hyphens; role runtime/model keys and mode keys have additional recognized forms.

`internal/config/resolve.go:14-19`

> var (  
> 	confKeyPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)  
> 	modeKeyPattern    = regexp.MustCompile(`^mode\.([a-z0-9][a-z0-9-]*)\.(.+)$`)  
> 	roleRuntimePattern = regexp.MustCompile(`^role\.([a-z0-9][a-z0-9-]*)\.runtime$`)  
> 	roleModelPattern   = regexp.MustCompile(`^role\.([a-z0-9][a-z0-9-]*)\.model$`)  
> )

**F Q2.5.** Configuration resolution precedence is explicit flag, environment, local file, mode-specific role value, committed file, then built-in default.

`internal/config/resolve.go:42-48`

> // Precedence is: explicit flag, environment, local config, mode-specific role  
> // config, committed config, then the supplied built-in default.

**F Q2.6.** Configuration files use one trimmed `key=value` entry per nonblank, non-comment line; splitting occurs at the first `=`, names and values are trimmed, and duplicate keys are rejected.

`internal/config/resolve.go:105-133`

> line := strings.TrimSpace(scanner.Text())  
> if line == "" || strings.HasPrefix(line, "#") {  
> 	continue  
> }  
> idx := strings.IndexByte(line, '=')  
> if idx < 0 {  
> 	return nil, fmt.Errorf("%s:%d: expected key=value", path, lineNo)  
> }  
> key := strings.TrimSpace(line[:idx])  
> value := strings.TrimSpace(line[idx+1:])  
> if !confKeyPattern.MatchString(key) {  
> 	return nil, fmt.Errorf("%s:%d: invalid key %q", path, lineNo, key)  
> }  
> if _, exists := values[key]; exists {  
> 	return nil, fmt.Errorf("%s:%d: duplicate key %q", path, lineNo, key)  
> }

**F Q2.7.** Environment-variable names are derived by uppercasing the key, replacing dots and hyphens with underscores, and adding `METASYSTEM_`.

`internal/config/resolve.go:186-193`

> func EnvName(key string) string {  
> 	replacer := strings.NewReplacer(".", "_", "-", "_")  
> 	return "METASYSTEM_" + strings.ToUpper(replacer.Replace(key))  
> }

**F Q2.8.** Cap keys support both runtime/model and role/runtime/model forms in ordinary configuration; values must be positive integers and models must be canonical for the named runtime.

`internal/config/validate.go:135-163`

> if len(parts) >= 4 {  
> 	if _, ok := runtimeSet[parts[2]]; ok {  
> 		runtime = parts[2]  
> 		model = strings.Join(parts[3:], ".")  
> 	} else if len(parts) >= 5 {  
> 		runtime = parts[3]  
> 		model = strings.Join(parts[4:], ".")  
> 	}  
> }  
> ...  
> if !capability.IsCanonicalModel(runtime, model) {  
> 	errs = append(errs, fmt.Errorf("%s: unsupported cap model %s/%s", key, runtime, model))  
> }  
> if _, err := positiveInt(value); err != nil {  
> 	errs = append(errs, fmt.Errorf("%s: %v", key, err))  
> }

**F Q2.9.** `METASYSTEM_CAP_MIN_*` environment values are also required to be positive integers.

`internal/config/validate.go:165-169`

> for key, value := range env {  
> 	if strings.HasPrefix(key, "METASYSTEM_CAP_MIN_") {  
> 		if _, err := positiveInt(value); err != nil {  
> 			errs = append(errs, fmt.Errorf("%s: %v", key, err))  
> 		}  
> 	}  
> }

**F Q2.10.** A configured role runtime must be a roster or main runtime.

`internal/config/validate.go:172-180`

> if strings.HasPrefix(key, "role.") && strings.HasSuffix(key, ".runtime") {  
> 	if value != "main" {  
> 		if _, ok := runtimeSet[value]; !ok {  
> 			errs = append(errs, fmt.Errorf("%s: unsupported runtime %q", key, value))  
> 		}  
> 	}  
> }

**F Q2.11.** The generic committed-config validator rejects unsupported `mode.*` shapes, but it contains no general rejection branch for every otherwise syntactically valid unknown non-mode key.

`internal/config/validate.go:98-101`

> if strings.HasPrefix(key, "mode.") && !modeKeyPattern.MatchString(key) {  
> 	errs = append(errs, fmt.Errorf("%s: unsupported mode key", key))  
> }

`internal/config/validate.go:337-340`

> return errs

**F Q2.12.** Dispatch cap resolution is: explicit `--cap-min`, role/runtime/model cap, runtime/model cap, `dispatch.cap-min`, then a built-in 120 minutes.

`scripts/agents/dispatch.sh:388-415`

> cap_source="built-in-default"  
> if [[ -n "$explicit_cap_min" ]]; then  
>   cap_min="$explicit_cap_min"  
>   cap_source="explicit"  
> elif resolved=$(resolve_config_value "cap.min.${role}.${runtime}.${effective_model}" ""); then  
>   cap_min="$resolved"  
>   cap_source="role-pair"  
> elif resolved=$(resolve_config_value "cap.min.${runtime}.${effective_model}" ""); then  
>   cap_min="$resolved"  
>   cap_source="runtime-pair"  
> elif resolved=$(resolve_config_value "dispatch.cap-min" "120"); then  
>   cap_min="$resolved"  
>   cap_source="dispatch-default"  
> else  
>   cap_min=120  
> fi  
> [[ "$cap_min" =~ ^[1-9][0-9]*$ ]] || die "invalid cap minutes: $cap_min"

**F Q2.13.** Mission dispatch refuses unsigned local or environment cap overrides for either the role pair or runtime pair.

`scripts/agents/dispatch.sh:417-424`

> if [[ -n "$mission_id" ]]; then  
>   case "$cap_source" in  
>     role-pair:local|role-pair:env|runtime-pair:local|runtime-pair:env)  
>       die "mission dispatch refuses unsigned cap override from $cap_source"  
>       ;;  
>   esac  
> fi

**F Q2.14.** A mission contract requires explicit positive limits for ledger cycles, ledger no-gain, wall hours, cycles, jobs, concurrency, job-cap minutes, and host-turn-cap minutes; these fields have no parser-supplied defaults.

`internal/mission/contract.go:69-89`

> var requiredScalarKeys = []string{  
> 	"gate.command",  
> 	"gate.ref",  
> 	"gate.paths",  
> 	"truth.paths",  
> 	"truth.certification",  
> 	"direction",  
> 	"guard.cadence",  
> 	"ledger.cycle-budget",  
> 	"ledger.no-gain-budget",  
> 	"fence.wall-hours",  
> 	"fence.cycles",  
> 	"fence.jobs",  
> 	"fence.concurrency",  
> 	"fence.job-cap-min",  
> 	"host.runtime",  
> 	"host.model",  
> 	"host.turn-cap-min",  
> 	"exposure.criticality",  
> }  
> var positiveIntegerKeys = map[string]struct{}{  
> 	"ledger.cycle-budget": {}, "ledger.no-gain-budget": {},  
> 	"fence.wall-hours": {}, "fence.cycles": {}, "fence.jobs": {},  
> 	"fence.concurrency": {}, "fence.job-cap-min": {}, "host.turn-cap-min": {},  
> }

**F Q2.15.** Contract parsing permits exactly one mission block, at most one seal, and at most one approval.

`internal/mission/contract.go:193-240`

> if len(blocks) != 1 {  
> 	return nil, fmt.Errorf("contract must contain exactly one metasystem mission block")  
> }  
> if len(seals) > 1 {  
> 	return nil, fmt.Errorf("contract contains multiple mission seal blocks")  
> }  
> if len(approvals) > 1 {  
> 	return nil, fmt.Errorf("contract contains multiple Approval lines")  
> }

**F Q2.16.** Mission-block entries use exact, unpadded `key=value` lines and reject duplicate keys.

`internal/mission/contract.go:253-275`

> if strings.TrimSpace(line) != line || line == "" {  
> 	return nil, fmt.Errorf("mission block line %d must be a non-empty unpadded key=value line", lineNo)  
> }  
> parts := strings.SplitN(line, "=", 2)  
> if len(parts) != 2 || parts[0] == "" || parts[1] == "" {  
> 	return nil, fmt.Errorf("mission block line %d must be key=value", lineNo)  
> }  
> if _, exists := fields[parts[0]]; exists {  
> 	return nil, fmt.Errorf("mission block repeats key %q", parts[0])  
> }

**F Q2.17.** Contract validation has an explicit allow-list of scalar and patterned families and rejects unknown mission keys.

`internal/mission/contract.go:300-350`

> switch {  
> case scalarKeySet[key]:  
> case strings.HasPrefix(key, "cap.min."):  
> case strings.HasPrefix(key, "threshold."):  
> case strings.HasPrefix(key, "noise."):  
> case strings.HasPrefix(key, "guard."):  
> case strings.HasPrefix(key, "stream."):  
> case strings.HasPrefix(key, "envelope."):  
> default:  
> 	return fmt.Errorf("unsupported mission key %q", key)  
> }

**F Q2.18.** Signed mission cap entries support only `cap.min.<runtime>.<model>`, not a role-qualified cap key.

`internal/mission/contract.go:405-420`

> parts := strings.Split(key, ".")  
> if len(parts) < 4 || parts[0] != "cap" || parts[1] != "min" {  
> 	return fmt.Errorf("invalid mission cap key %q", key)  
> }  
> runtime := parts[2]  
> model := strings.Join(parts[3:], ".")  
> if !capability.IsCanonicalModel(runtime, model) {  
> 	return fmt.Errorf("unsupported mission cap pair %s/%s", runtime, model)  
> }  
> if _, err := positiveInt(value); err != nil {  
> 	return fmt.Errorf("%s: %v", key, err)  
> }

**F Q2.19.** Contract sealing binds gate reference and integrity, truth inputs, candidate branch, baseline measurements, all required fences, and sorted cap entries into a generated seal.

`internal/mission/contract.go:967-1012`

> generated := map[string]string{  
> 	"sealed.gate.ref":              resolvedGateRef,  
> 	"sealed.gate.integrity":        gateIntegrity,  
> 	"sealed.truth.paths":           fields["truth.paths"],  
> 	"sealed.truth.certification":   fields["truth.certification"],  
> 	"sealed.candidate.branch":      branch,  
> 	"sealed.baseline":              encodeMetrics(baseline),  
> 	"sealed.fence.wall-hours":      fields["fence.wall-hours"],  
> 	"sealed.fence.cycles":          fields["fence.cycles"],  
> 	"sealed.fence.jobs":            fields["fence.jobs"],  
> 	"sealed.fence.concurrency":     fields["fence.concurrency"],  
> 	"sealed.fence.job-cap-min":     fields["fence.job-cap-min"],  
> 	"sealed.host.turn-cap-min":     fields["host.turn-cap-min"],  
> 	"sealed.exposure.criticality":  fields["exposure.criticality"],  
> }

**F Q2.20.** Sealing refuses pre-existing seal or approval material, writes the ordered generated seal, and returns a digest over the signed bytes.

`internal/mission/contract.go:1047-1100`

> if contract.Seal != nil {  
> 	return "", fmt.Errorf("contract is already sealed")  
> }  
> if contract.Approval != "" {  
> 	return "", fmt.Errorf("contract is already approved")  
> }  
> ...  
> for _, key := range keys {  
> 	fmt.Fprintf(&builder, "%s=%s\n", key, generated[key])  
> }  
> ...  
> digest := sha256.Sum256(raw)  
> return hex.EncodeToString(digest[:]), nil

**F Q2.21.** Preflight verifies the seal, approval, repository origin, gate, supervision, and lease before mission execution.

`internal/mission/contract.go:1105-1124`

> if err := VerifySeal(contract); err != nil {  
> 	return err  
> }  
> if err := VerifyApproval(contract); err != nil {  
> 	return err  
> }  
> if err := VerifyOrigin(contract, repo); err != nil {  
> 	return err  
> }  
> if err := VerifyGate(contract, repo); err != nil {  
> 	return err  
> }  
> if err := VerifySupervision(contract, repo); err != nil {  
> 	return err  
> }  
> return VerifyLease(contract, repo)

**F Q2.22.** Preflight recomputes the generated seal keys and rejects missing, extra, or stale values.

`internal/mission/contract.go:1127-1179`

> if len(contract.Seal.Fields) != len(expected) {  
> 	return fmt.Errorf("mission seal field count mismatch")  
> }  
> for key, want := range expected {  
> 	got, ok := contract.Seal.Fields[key]  
> 	if !ok {  
> 		return fmt.Errorf("mission seal is missing %q", key)  
> 	}  
> 	if got != want {  
> 		return fmt.Errorf("mission seal field %q is stale", key)  
> 	}  
> }

**F Q2.23.** Approval is an exact SHA-256 equality check against the raw sealed contract bytes.

`internal/mission/contract.go:1182-1190`

> digest := sha256.Sum256(contract.RawSealed)  
> expected := hex.EncodeToString(digest[:])  
> if contract.Approval != expected {  
> 	return fmt.Errorf("mission approval does not match sealed contract")  
> }

**F Q2.24.** Live cap authorization rechecks that the on-disk contract SHA still equals the approved SHA.

`internal/mission/fence.go:142-157`

> raw, err := os.ReadFile(contract.Path)  
> if err != nil {  
> 	return err  
> }  
> digest := sha256.Sum256(raw)  
> liveSHA := hex.EncodeToString(digest[:])  
> if liveSHA != contract.ApprovedSHA {  
> 	return fmt.Errorf("mission contract changed after approval")  
> }

**F Q2.25.** Mission cap authorization uses the signed runtime/model cap when present, otherwise `fence.job-cap-min`; it rejects a requested cap above that authority and truncates the deadline to the remaining wall fence.

`internal/mission/fence.go:435-517`

> capKey := "cap.min." + runtime + "." + model  
> authorized := contract.Int(capKey)  
> source := capKey  
> if authorized == 0 {  
> 	authorized = contract.Int("fence.job-cap-min")  
> 	source = "fence.job-cap-min"  
> }  
> if requestedCapMin > authorized {  
> 	return CapAuthorization{}, fmt.Errorf("requested cap %d exceeds authorized cap %d", requestedCapMin, authorized)  
> }  
> deadline := now.Add(time.Duration(requestedCapMin) * time.Minute)  
> if deadline.After(wallDeadline) {  
> 	deadline = wallDeadline  
> }

**F Q2.26.** Dispatch calls mission cap authorization and additionally requires the effective job cap to be strictly below the watcher ceiling.

`scripts/agents/dispatch.sh:998-1013`

> cap_authority_json=$("$METASYSTEM_BIN" mission fence authorize-cap \  
>   --contract "$mission_contract" \  
>   --runtime "$runtime" \  
>   --model "$effective_model" \  
>   --requested-cap-min "$cap_min") || die "mission cap authorization failed"  
> ...  
> (( cap_min < watch_cap_min )) || die "dispatch cap ${cap_min}m must be below watcher cap ${watch_cap_min}m"

**F Q2.27.** The premise that patience-floor keys already exist in `metasystem.conf` or the sealed mission grammar is false: the patience document assigns them to future roster and contract surfaces, while the shipped contract parser rejects unknown keys.

`docs/patience.md:68-72`

> 3. **Patience floors.** Configuration lives where capability lives: the  
>    roster keys in `metasystem.conf`, per role and runtime:model pair,  
>    with mission contracts able to seal overrides. Defaults are generous.  
>    The shipped core is the degenerate case: floor = "any above-noise new  
>    best", window = `ledger.no-gain-budget` cycles.

`internal/mission/contract.go:343-349`

> default:  
> 	return fmt.Errorf("unsupported mission key %q", key)

## Q3 — Per-activity progress records and artifacts

**F Q3.1.** An initial job record contains identity, role, mission, runtime, round and parentage, review targets, lifecycle status, repository/workspace identity, permissions, cap and process identity, session/turn identity, requested and effective model, fallback metadata, handshake state, input, timestamps, usage, mirror state, chain state, and exhaustion markers.

`internal/dispatch/build.go:161-211`

> record := Record{  
> 	JobID:             input.JobID,  
> 	Role:              input.Role,  
> 	Mission:           input.Mission,  
> 	Runtime:           input.Runtime,  
> 	Round:             1,  
> 	ParentJob:         input.ParentJob,  
> 	Reviews:           input.Reviews,  
> 	Status:            StatusPending,  
> 	Phase:             PhaseSetup,  
> 	MainRepo:          input.MainRepo,  
> 	MainTree:          input.MainTree,  
> 	Claim:             input.Claim,  
> 	Workspace:         input.Workspace,  
> 	Base:              input.Base,  
> 	Branch:            input.Branch,  
> 	Permissions:       input.Permissions,  
> 	CapMin:            input.CapMin,  
> 	PID:               input.PID,  
> 	PGID:              input.PGID,  
> 	SessionID:         input.SessionID,  
> 	TurnID:            input.TurnID,  
> 	RequestedModel:    input.RequestedModel,  
> 	EffectiveModel:    input.EffectiveModel,  
> 	CapabilitySnapshot: input.CapabilitySnapshot,  
> 	Fallbacks:         input.Fallbacks,  
> 	Handshake:         input.Handshake,  
> 	Input:             input.Input,  
> 	StartedAt:         input.StartedAt,  
> 	EndedAt:           input.EndedAt,  
> 	Usage:             input.Usage,  
> 	Mirror:            input.Mirror,  
> 	ChainOpen:         input.ChainOpen,  
> 	Exhaustions:       input.Exhaustions,  
> }

**F Q3.2.** A follow-up record inherits role, mission, runtime, reviews, workspace, base, branch, and requested model, but receives its own job ID, round, parent, status, cap, and resume mode.

`internal/dispatch/build.go:236-322`

> record := Record{  
> 	JobID:          input.JobID,  
> 	Role:           parent.Role,  
> 	Mission:        parent.Mission,  
> 	Runtime:        parent.Runtime,  
> 	Round:          input.Round,  
> 	ParentJob:      parent.JobID,  
> 	Reviews:        parent.Reviews,  
> 	Status:         StatusPending,  
> 	Workspace:      parent.Workspace,  
> 	Base:           parent.Base,  
> 	Branch:         parent.Branch,  
> 	CapMin:         input.CapMin,  
> 	RequestedModel: parent.RequestedModel,  
> 	ResumeMode:     input.ResumeMode,  
> }

**F Q3.3.** Job-record mutation has one atomic owner, a fixed lifecycle graph, immutable identity fields, and terminal-only metadata.

`internal/dispatch/record.go:1-5`

> // Package dispatch owns the canonical job-record format and state machine.  
> // Every mutation is serialized and replaced atomically.

`internal/dispatch/record.go:25-54`

> var allowedTransitions = map[Status]map[Status]bool{  
> 	StatusPending: {StatusRunning: true, StatusFailed: true},  
> 	StatusRunning: {StatusCompleted: true, StatusFailed: true, StatusTimeout: true},  
> }  
> ...  
> var immutableFields = []string{  
> 	"jobId", "role", "runtime", "round", "workspace", "base", "branch",  
> }  
> ...  
> var terminalOnlyFields = []string{  
> 	"endedAt", "error", "usage", "returnPath",  
> }

**F Q3.4.** Dispatch creates a per-job payload root with a stable brief, a first-round prompt, and the canonical job record.

`scripts/agents/dispatch.sh:1033-1053`

> payload_root="$jobs_root/$job_id"  
> round_dir="$payload_root/rounds/1"  
> mkdir -p "$round_dir"  
> brief_path="$payload_root/brief.md"  
> prompt_path="$round_dir/prompt.md"  
> record_path="$payload_root/record.json"  
> cp "$brief_source" "$brief_path"  
> cp "$prompt_source" "$prompt_path"

**F Q3.5.** Follow-up rounds have distinct round directories and prompts assembled from the original brief, prior return, and correction.

`scripts/agents/dispatch.sh:1202-1218`

> round_dir="$payload_root/rounds/$round"  
> mkdir -p "$round_dir"  
> prompt_path="$round_dir/prompt.md"  
> {  
>   printf '%s\n\n' '# Original brief'  
>   cat "$brief_path"  
>   printf '%s\n\n' '# Previous return'  
>   cat "$prior_return"  
>   printf '%s\n\n' '# Correction'  
>   cat "$correction_file"  
> } >"$prompt_path"

**F Q3.6.** Runtime setup derives per-round paths for the prompt, log, raw output, events, heartbeat, effective-model record, and return schema.

`scripts/agents/adapters/runtime-common.sh:43-72`

> record_path="$payload_root/record.json"  
> round_dir="$payload_root/rounds/$round"  
> prompt_file="$round_dir/prompt.md"  
> job_log="$round_dir/job.log"  
> raw_output="$round_dir/raw.out"  
> events_file="$round_dir/events.jsonl"  
> heartbeat_file="$round_dir/heartbeat"  
> effective_model_file="$round_dir/effective-model.json"  
> return_schema="$round_dir/return-schema.json"

**F Q3.7.** Runtime completion normalizes both `return.json` and `return.md`, then validates the structured return.

`scripts/agents/adapters/runtime-common.sh:184-199`

> normalize_return "$raw_output" "$round_dir/return.json" "$round_dir/return.md" || return 1  
> validate_return "$round_dir/return.json" "$return_schema" || return 1

**F Q3.8.** Protocol repair is limited to one attempt and leaves the repair prompt, repair output, and protocol-violation artifact.

`scripts/agents/adapters/runtime-common.sh:201-285`

> repair_prompt="$round_dir/repair-1.prompt.md"  
> repair_output="$round_dir/repair-1.out"  
> protocol_violation="$round_dir/protocol-violation.txt"  
> ...  
> if [[ -e "$repair_prompt" || -e "$repair_output" ]]; then  
>   return 1  
> fi

**F Q3.9.** Critic returns expose evidence, reviewed revision identity, and structured findings.

`scripts/agents/schemas/code-critic.schema.json:7-7`

> "required": ["summary", "evidence", "reviewedTree", "findings"],

`scripts/agents/schemas/code-critic.schema.json:22-53`

> "evidence": {  
>   "type": "array",  
>   "items": {  
>     "type": "object",  
>     "required": ["kind", "detail"],  
>     "properties": {  
>       "kind": {"enum": ["ran", "read", "inferred"]},  
>       "detail": {"type": "string"}  
>     }  
>   }  
> },  
> "reviewedTree": {"type": "string"},  
> "findings": {  
>   "type": "array",  
>   "items": {  
>     "type": "object",  
>     "required": ["id", "severity", "summary"],  
>     "properties": {  
>       "id": {"type": "string"},  
>       "severity": {"type": "string"},  
>       "summary": {"type": "string"}  
>     }  
>   }  
> }

**F Q3.10.** The evidence schema distinguishes `ran`, `read`, and `inferred`; schema-valid evidence is therefore not restricted to executed measurements.

`scripts/agents/schemas/design-critic.schema.json:22-29`

> "kind": {  
>   "enum": ["ran", "read", "inferred"]  
> }

**F Q3.11.** Implementer returns require `riskiestPart`, `diffBoundary`, and `whatWasDone`.

`scripts/agents/schemas/implementer.schema.json:7-7`

> "required": ["summary", "evidence", "riskiestPart", "diffBoundary", "whatWasDone"],

**F Q3.12.** Verifier returns require evidence, gaps, riskiest part, and work description; the shipped verifier schema has no `confirmations` field.

`scripts/agents/schemas/verifier.schema.json:7-7`

> "required": ["summary", "evidence", "gaps", "riskiestPart", "whatWasDone"],

`scripts/agents/schemas/verifier.schema.json:22-38`

> "properties": {  
>   "summary": {"type": "string"},  
>   "evidence": {"type": "array"},  
>   "gaps": {"type": "array"},  
>   "riskiestPart": {"type": "string"},  
>   "whatWasDone": {"type": "string"}  
> }

**F Q3.13.** Orchestrator returns expose dispatched jobs, certifications, stream updates, asks, proposed ledger facts, gaps, and identity.

`scripts/agents/schemas/orchestrator.schema.json:7-7`

> "required": ["summary", "dispatched", "certified", "streamUpdates", "asks", "factsForLedger", "gaps", "identity"],

`scripts/agents/schemas/orchestrator.schema.json:25-35`

> "certified": {  
>   "type": "array",  
>   "items": {  
>     "type": "object",  
>     "required": ["jobId", "verdict", "evidence"],  
>     "properties": {  
>       "jobId": {"type": "string"},  
>       "verdict": {"type": "string"},  
>       "evidence": {"type": "array"}  
>     }  
>   }  
> }

**F Q3.14.** Critique closure is computed by joining critic-return finding IDs against a Markdown dispositions table.

`scripts/assert-critique-closed.sh:7-15`

> # Assert that every material finding from a critic return has an explicit  
> # disposition in the supplied Markdown table. Exit zero means closed.

`internal/dispatch/critique.go:90-130`

> if record.Round <= 0 {  
> 	return nil, fmt.Errorf("invalid critic round %d", record.Round)  
> }  
> if record.Status != StatusCompleted {  
> 	return nil, fmt.Errorf("critic job %s is not completed", record.JobID)  
> }  
> ...  
> for _, finding := range ret.Findings {  
> 	if finding.Material() {  
> 		ids = append(ids, finding.ID)  
> 	}  
> }

**F Q3.15.** Critique exhaustion is mechanically asserted at every third round against remaining open finding IDs.

`internal/dispatch/critique.go:220-275`

> if record.Round%3 != 0 {  
> 	return nil  
> }  
> ...  
> if len(openIDs) > 0 {  
> 	return fmt.Errorf("critique chain exhausted with open findings: %s", strings.Join(openIDs, ", "))  
> }

**F Q3.16.** Conformance captures the reviewed tree and patch in an isolated index and persists a review record.

`scripts/agents/assert-conformance.sh:92-106`

> reviewed_tree=$(git write-tree)  
> git diff --cached --binary >"$review_dir/diff.patch"

`scripts/agents/assert-conformance.sh:192-203`

> "$METASYSTEM_BIN" dispatch critique write-review \  
>   --record "$record_path" \  
>   --reviewed-tree "$reviewed_tree" \  
>   --diff "$review_dir/diff.patch" \  
>   --output "$review_dir/review.json"

**F Q3.17.** Merge conformance requires a closed chain, a completed final record and return, no material findings or nonzero verdict, satisfied exhaustion rules, an unchanged reviewed tree, and model independence.

`scripts/agents/assert-conformance.sh:498-623`

> assert_chain_closed "$record_path"  
> [[ "$status" == "completed" ]] || fail "final critic record is not completed"  
> [[ -f "$return_path" ]] || fail "final critic return is missing"  
> (( material_count == 0 )) || fail "material findings remain"  
> (( verdict == 0 )) || fail "critic verdict is non-zero"  
> assert_exhaustion "$record_path"  
> [[ "$reviewed_tree" == "$final_tree" ]] || fail "reviewed tree does not match final tree"  
> assert_model_independence "$record_path"

**F Q3.18.** The mission sequence document records a current observability gap: the runner can see dispatch records, reservations, and chain state, but cannot determine critique closure from a job field.

`docs/design/mission-cycle-sequence.md:491-503`

> The runner can see dispatch records, reservations, and open-chain state. It  
> cannot currently determine critique closure mechanically because no canonical  
> job-record field carries that verdict.

**F Q3.19.** Each host turn has its own directory and record paths for the host result, raw return, adjudication, and measurement.

`internal/missionrunner/loop.go:900-950`

> turnDir := filepath.Join(paths.Turns, strconv.Itoa(state.NextTurn))  
> turnPath := filepath.Join(turnDir, "turn.json")  
> returnPath := filepath.Join(turnDir, "return.json")  
> resultPath := filepath.Join(turnDir, "result.json")  
> adjudicationPath := filepath.Join(turnDir, "adjudication.json")  
> measurementPath := filepath.Join(turnDir, "measurement.json")

**F Q3.20.** The host-result envelope carries session, outcome, usage, raw-output path, and return path.

`internal/host/result.go:5-24`

> type Result struct {  
> 	SessionID  string `json:"sessionId,omitempty"`  
> 	Outcome    string `json:"outcome"`  
> 	Usage      Usage  `json:"usage,omitempty"`  
> 	RawPath    string `json:"rawPath,omitempty"`  
> 	ReturnPath string `json:"returnPath,omitempty"`  
> }

**F Q3.21.** Accepted host turns persist adjudication and measurement artifacts before the runner concludes the turn.

`internal/missionrunner/loop.go:1030-1063`

> if err := writeJSONAtomic(adjudicationPath, adjudication); err != nil {  
> 	return err  
> }

`internal/missionrunner/loop.go:1080-1104`

> if err := writeJSONAtomic(measurementPath, measurement); err != nil {  
> 	return err  
> }  
> return ConcludeTurn(paths, state, conclusion)

**F Q3.22.** The durable turn log records classification, measurement, acceptance or rejection, certifications, proposed ledger facts, and gaps.

`internal/missionrunner/cycle.go:71-106`

> type TurnConclusion struct {  
> 	Classification string                 `json:"classification"`  
> 	Measurement    *mission.Measurement   `json:"measurement,omitempty"`  
> 	Accepted       []AcceptedDispatch     `json:"accepted,omitempty"`  
> 	Rejected       []RejectedDispatch     `json:"rejected,omitempty"`  
> 	Certified      []CertifiedDispatch    `json:"certified,omitempty"`  
> 	FactsForLedger []string               `json:"factsForLedger,omitempty"`  
> 	Gaps           []string               `json:"gaps,omitempty"`  
> }  
> ...  
> log := TurnLog{  
> 	Classification: conclusion.Classification,  
> 	Measurement:    conclusion.Measurement,  
> 	Accepted:       conclusion.Accepted,  
> 	Rejected:       conclusion.Rejected,  
> 	Certified:      conclusion.Certified,  
> 	FactsForLedger: conclusion.FactsForLedger,  
> 	Gaps:           conclusion.Gaps,  
> }

**F Q3.23.** Resume rereads the state, turn record, adjudication, return, result, and measurement artifacts.

`internal/missionrunner/turnio.go:62-81`

> if err := readJSON(paths.State, &state); err != nil {  
> 	return TurnArtifacts{}, err  
> }  
> if err := readJSON(turnPath, &turn); err != nil {  
> 	return TurnArtifacts{}, err  
> }  
> if err := readJSON(adjudicationPath, &adjudication); err != nil {  
> 	return TurnArtifacts{}, err  
> }  
> if err := readJSON(returnPath, &ret); err != nil {  
> 	return TurnArtifacts{}, err  
> }  
> if err := readJSON(resultPath, &result); err != nil {  
> 	return TurnArtifacts{}, err  
> }  
> if err := readJSON(measurementPath, &measurement); err != nil {  
> 	return TurnArtifacts{}, err  
> }

**F Q3.24.** Claude adapter rounds retain settings, signal, provider result, usage, events, raw output, and job log artifacts.

`scripts/agents/adapters/claude.sh:101-112`

> settings_file="$round_dir/settings.json"  
> signal_file="$round_dir/signal.json"  
> provider_result="$round_dir/provider-result.json"  
> usage_file="$round_dir/usage.json"  
> events_file="$round_dir/events.jsonl"

`scripts/agents/adapters/claude.sh:153-175`

> cp "$provider_result" "$raw_output"  
> cp "$provider_result" "$job_log"  
> write_events "$provider_result" "$events_file"  
> write_usage "$provider_result" "$usage_file"

**F Q3.25.** Codex adapter rounds retain an event stream, raw output, usage, and job log.

`scripts/agents/adapters/codex.sh:135-159`

> usage_file="$round_dir/usage.json"  
> events_file="$round_dir/events.jsonl"  
> raw_output="$round_dir/raw.out"  
> ...  
> tee "$events_file" "$job_log"

`scripts/agents/adapters/codex.sh:176-176`

> write_usage "$events_file" "$usage_file"

**F Q3.26.** Devin adapter rounds retain configuration, transcript, session snapshots, signal, usage, prompt, events, raw output, and provenance.

`scripts/agents/adapters/devin.sh:263-294`

> config_file="$round_dir/devin-config.json"  
> transcript_file="$round_dir/transcript.json"  
> session_before_file="$round_dir/session-before.json"  
> session_after_file="$round_dir/session-after.json"  
> signal_file="$round_dir/signal.json"  
> usage_file="$round_dir/usage.json"  
> prompt_devin="$round_dir/prompt.devin.md"  
> events_file="$round_dir/events.jsonl"  
> raw_output="$round_dir/raw.out"  
> provenance_file="$round_dir/provenance.json"

**F Q3.27.** The fake adapter writes prompt, log, raw, events, heartbeat, effective-model, handshake, and deliberately malformed-return/protocol-violation artifacts.

`scripts/agents/adapters/fake.sh:134-143`

> cp "$prompt_file" "$round_dir/prompt.fake.md"  
> : >"$job_log"  
> : >"$raw_output"  
> : >"$events_file"  
> touch "$heartbeat_file"

`scripts/agents/adapters/fake.sh:203-211`

> printf '%s\n' '{"malformed":' >"$round_dir/return.json"  
> printf '%s\n' 'synthetic protocol violation' >"$round_dir/protocol-violation.txt"

**F Q3.28.** The Claude host stores turn record, raw return, normalized return, provider result, usage, and log per turn.

`scripts/agents/hosts/claude.sh:55-61`

> turn_json="$turn_dir/turn.json"  
> raw_output="$turn_dir/raw.out"  
> return_json="$turn_dir/return.json"  
> provider_result="$turn_dir/provider-result.json"  
> usage_file="$turn_dir/usage.json"  
> host_log="$turn_dir/host.log"

**F Q3.29.** The Codex host stores turn record, raw return, normalized return, event stream, usage, and log per turn.

`scripts/agents/hosts/codex.sh:59-65`

> turn_json="$turn_dir/turn.json"  
> raw_output="$turn_dir/raw.out"  
> return_json="$turn_dir/return.json"  
> events_file="$turn_dir/events.jsonl"  
> usage_file="$turn_dir/usage.json"  
> host_log="$turn_dir/host.log"

**F Q3.30.** The Devin host stores turn record, raw return, normalized return, transcript, session usage, configuration, usage, and log.

`scripts/agents/hosts/devin.sh:56-64`

> turn_json="$turn_dir/turn.json"  
> raw_output="$turn_dir/raw.out"  
> return_json="$turn_dir/return.json"  
> transcript_file="$turn_dir/transcript.json"  
> session_usage_file="$turn_dir/session-usage.json"  
> config_file="$turn_dir/devin-config.json"  
> usage_file="$turn_dir/usage.json"  
> host_log="$turn_dir/host.log"

**F Q3.31.** Landed-return classification is mechanically derived from the repository tree and turn log, limits output to one return per chain and 20 total, and distinguishes accepted or certified dispatches from unconsumed landed returns.

`internal/mission/landed.go:13-39`

> // LandedReturns derives the unconsumed landed-return set from the current tree  
> // and the durable turn log. It returns at most one result per chain.  
> ...  
> const maxLandedReturns = 20

`internal/mission/landed.go:237-273`

> if accepted[record.JobID] || certified[record.JobID] {  
> 	continue  
> }  
> landed = append(landed, LandedReturn{  
> 	ChainID: chainID,  
> 	Round:   record.Round,  
> 	Path:    returnPath,  
> })

**F Q3.32.** The premise that orchestrator `factsForLedger` are current ledger-line writers is false: they are copied into the turn conclusion and durable turn log, while the ledger append receives only the computed measurement and annotations.

`internal/missionrunner/turnio.go:97-105`

> conclusion.FactsForLedger = append([]string(nil), ret.FactsForLedger...)  
> conclusion.Gaps = append([]string(nil), ret.Gaps...)

`internal/missionrunner/loop.go:636-649`

> return mission.AppendCycle(  
> 	paths.Ledger,  
> 	cycle,  
> 	measurement.Classification,  
> 	measurement.CandidateSHA,  
> 	measurement.Observed,  
> 	best,  
> 	annotations,  
> )

## Q4 — Current runner progress booking

**F Q4.1.** Mission progress measurement is defined as deterministic evaluation of the sealed gate, guards, and metric contract against a candidate revision.

`internal/mission/measure.go:14-18`

> // ContractMeasure evaluates the sealed gate, guard set, and metric contract  
> // against one candidate revision.

**F Q4.2.** A measurement records candidate SHA, classification, observed rendering, metrics, guard results, and whether the gate passed.

`internal/mission/measure.go:20-30`

> type Measurement struct {  
> 	CandidateSHA   string            `json:"candidateSha"`  
> 	Classification string            `json:"classification"`  
> 	Observed       string            `json:"observed"`  
> 	Metrics        map[string]float64 `json:"metrics,omitempty"`  
> 	Guards         []GuardResult     `json:"guards,omitempty"`  
> 	GatePassed     bool              `json:"gatePassed"`  
> }

**F Q4.3.** Current contract measurement emits `contract-improved` only for an above-noise improvement without guard regression, emits `unresolved` when every change is within its noise floor, and otherwise emits `no-progress`.

`internal/mission/measure.go:99-134`

> switch {  
> case improved && !regressed:  
> 	result.Classification = ClassContractImproved  
> case withinNoise:  
> 	result.Classification = ClassUnresolved  
> default:  
> 	result.Classification = ClassNoProgress  
> }

**F Q4.4.** Consequently, the current runner measurement path produces only three of the ledger’s six accepted classifications.

`internal/mission/measure.go:99-134`

> result.Classification = ClassContractImproved  
> ...  
> result.Classification = ClassUnresolved  
> ...  
> result.Classification = ClassNoProgress

`internal/mission/ledger.go:22-30`

> "contract-improved": {},  
> "falsified-continue": {},  
> "falsified-dead-end": {},  
> "no-progress": {},  
> "unresolved": {},  
> "invalid-run": {},

**F Q4.5.** The comparison baseline is the latest complete turn-log measurement when one exists; otherwise it is the sealed baseline.

`internal/missionrunner/contract.go:145-173`

> for i := len(state.TurnLog) - 1; i >= 0; i-- {  
> 	entry := state.TurnLog[i]  
> 	if entry.Measurement != nil && len(entry.Measurement.Metrics) > 0 {  
> 		return cloneMetrics(entry.Measurement.Metrics), nil  
> 	}  
> }  
> return cloneMetrics(contract.SealedBaseline), nil

**F Q4.6.** A measurement error is converted into a `no-progress` booking with an `unmeasurable:` observed value and no measurement object.

`internal/missionrunner/loop.go:755-776`

> measured, err := mission.ContractMeasure(contract, repo, prior)  
> if err != nil {  
> 	return mission.Measurement{  
> 		CandidateSHA:   candidateSHA,  
> 		Classification: mission.ClassNoProgress,  
> 		Observed:       "unmeasurable:" + compactError(err),  
> 		GatePassed:     false,  
> 	}, nil, err  
> }

**F Q4.7.** Ordinary completed-turn progress is durably booked by selecting the best marker and appending the canonical cycle line before writing `measurement.json` and concluding the turn.

`internal/missionrunner/loop.go:636-649`

> best, err := bestMarker(paths.Ledger, measurement.Classification)  
> ...  
> return mission.AppendCycle(...)

`internal/missionrunner/loop.go:1080-1104`

> if err := appendLedger(paths, state.NextCycle, measurement, annotations); err != nil {  
> 	return err  
> }  
> if err := writeJSONAtomic(measurementPath, measurement); err != nil {  
> 	return err  
> }  
> return ConcludeTurn(paths, state, conclusion)

**F Q4.8.** A host result of `capped` is classified as a breaker-feeding fault and adds the canonical `Outcome: capped` ledger annotation.

`internal/missionrunner/loop.go:1005-1014`

> case host.OutcomeCapped:  
> 	fault = TurnFault{  
> 		Outcome:      host.OutcomeCapped,  
> 		Detail:       "host turn cap exhausted",  
> 		FeedsBreaker: true,  
> 		Annotations:  []string{mission.CappedAnnotation()},  
> 	}

**F Q4.9.** Rejected host returns feed the breaker except for the explicitly unwitnessed session fault, and add a canonical rejected-return annotation.

`internal/missionrunner/loop.go:1030-1052`

> feedsBreaker := true  
> if adjudication.Code == SessionFaultUnwitnessed {  
> 	feedsBreaker = false  
> }  
> fault = TurnFault{  
> 	Outcome:      host.OutcomeRejected,  
> 	Detail:       adjudication.Detail,  
> 	FeedsBreaker: feedsBreaker,  
> 	Annotations:  []string{mission.RejectedReturnAnnotation(adjudication.Detail)},  
> }

**F Q4.10.** Faulted turns are drained, measured, appended to the ledger with annotations, written to `measurement.json`, and then concluded through the fault path.

`internal/missionrunner/loop.go:807-880`

> if err := drainTurn(...); err != nil {  
> 	return err  
> }  
> measurement, _, measureErr := measureCandidate(...)  
> if err := appendLedger(paths, state.NextCycle, measurement, fault.Annotations); err != nil {  
> 	return err  
> }  
> if err := writeJSONAtomic(measurementPath, measurement); err != nil {  
> 	return err  
> }  
> return ConcludeFaultedTurn(paths, state, conclusion, fault)

**F Q4.11.** A faulted turn that nevertheless passes the gate completes the mission; otherwise a breaker-feeding fault increments the consecutive failure count and parks after the second consecutive failure.

`internal/missionrunner/cycle.go:151-215`

> if conclusion.Measurement != nil && conclusion.Measurement.GatePassed {  
> 	state.Status = StatusCompleted  
> 	state.ConsecutiveFailures = 0  
> 	return persistConclusion(paths, state, conclusion)  
> }  
> if fault.FeedsBreaker {  
> 	state.ConsecutiveFailures++  
> } else {  
> 	state.ConsecutiveFailures = 0  
> }  
> if state.ConsecutiveFailures >= 2 {  
> 	state.Status = StatusParked  
> 	state.ParkReason = ParkHostFailure  
> }

**F Q4.12.** Prior-session context counts only resumable, breaker-feeding faults and stops at a completed turn.

`internal/missionrunner/contract.go:103-134`

> for i := len(state.TurnLog) - 1; i >= 0; i-- {  
> 	entry := state.TurnLog[i]  
> 	if entry.Status == StatusCompleted {  
> 		break  
> 	}  
> 	if !entry.FeedsBreaker || !entry.Resumable {  
> 		continue  
> 	}  
> 	failures++  
> }

**F Q4.13.** A missing usable host return is booked as `no-progress` with an `unmeasurable:` observation, appended to the ledger, and passed to the failed-turn recorder.

`internal/missionrunner/loop.go:669-712`

> measurement := mission.Measurement{  
> 	CandidateSHA:   candidateSHA,  
> 	Classification: mission.ClassNoProgress,  
> 	Observed:       "unmeasurable:" + compactError(err),  
> }  
> if appendErr := appendLedger(paths, state.NextCycle, measurement, nil); appendErr != nil {  
> 	return appendErr  
> }  
> return RecordFailure(paths, state, detail)

**F Q4.14.** Failed turns have no measurement object and park on the second consecutive failure.

`internal/missionrunner/cycle.go:218-248`

> log := TurnLog{  
> 	Status:       StatusFailed,  
> 	Measurement:  nil,  
> 	FeedsBreaker: true,  
> }  
> state.ConsecutiveFailures++  
> if state.ConsecutiveFailures >= 2 {  
> 	state.Status = StatusParked  
> 	state.ParkReason = ParkHostFailure  
> }

**F Q4.15.** A start-unverified host turn is immediately recorded with a failure count sufficient to park.

`internal/missionrunner/loop.go:990-991`

> return recordFailedTurn(paths, state, turn, "start-unverified", 2)

**F Q4.16.** Drain expiry immediately parks the mission and writes state, anchor, and an ask, but deliberately does not write a ledger line until healing on resume.

`internal/missionrunner/drain.go:321-363`

> // Drain expiry is parked immediately, but no ledger line is written here.  
> // Resume healing owns the single canonical no-progress booking.  
> state.Status = StatusParked  
> state.ParkReason = ParkDrainStalled  
> ...  
> if err := WriteState(paths.State, state); err != nil {  
> 	return err  
> }  
> if err := writeAnchor(paths, state); err != nil {  
> 	return err  
> }  
> return writeAsks(paths.Asks, state.Asks)

**F Q4.17.** Resume healing converts a drain-stalled turn into a canonical `no-progress` measurement with `unmeasurable:drain-stalled` and a `Drain: stalled:<n>` annotation.

`internal/missionrunner/loop.go:411-428`

> // healInterruptedTurn converts a previously parked drain-stalled turn into  
> // the single canonical cycle booking before normal execution resumes.

`internal/missionrunner/loop.go:452-468`

> observed := "unmeasurable:turn-lost"  
> annotations := []string(nil)  
> if state.ParkReason == ParkDrainStalled {  
> 	observed = "unmeasurable:drain-stalled"  
> 	annotations = []string{mission.DrainStalledAnnotation(state.NextTurn)}  
> }  
> measurement := mission.Measurement{  
> 	CandidateSHA:   candidateSHA,  
> 	Classification: mission.ClassNoProgress,  
> 	Observed:       observed,  
> }

**F Q4.18.** Stop-loss replay does not use capped, rejected-return, drain-stalled, or landed-return annotations as fuse input.

`internal/missionrunner/stoploss.go:13-16`

> // Replay invariant: the replay reads ONLY classification, best, and reset  
> // lines. Cycle-block annotations are part of the audit trail, never fuse input.

## Q5 — Existing tolerated-wait bounds

**F Q5.1.** Stop-loss cycle and no-gain bounds are positive contract integers with no default. Their units are completed cycles and stagnant replay events; expiry parks the mission with a durable ask, state, and anchor.

`internal/mission/contract.go:78-89`

> "ledger.cycle-budget",  
> "ledger.no-gain-budget",  
> ...  
> "ledger.cycle-budget": {}, "ledger.no-gain-budget": {},

`internal/missionrunner/stoploss.go:235-250`

> case cycleBudget > 0 && cycles >= cycleBudget:  
> ...  
> case noGainBudget > 0 && stagnant >= noGainBudget:

`internal/missionrunner/loop.go:611-624`

> if err := writeAsks(paths.Asks, state.Asks); err != nil {  
> 	return err  
> }  
> if err := WriteState(paths.State, state); err != nil {  
> 	return err  
> }  
> return writeAnchor(paths, state)

**F Q5.2.** The host-failure breaker tolerates one consecutive breaker-feeding failed turn and parks on the second; the threshold is hard-coded in turns and is durably represented in turn log and mission state.

`internal/missionrunner/cycle.go:201-215`

> if fault.FeedsBreaker {  
> 	state.ConsecutiveFailures++  
> } else {  
> 	state.ConsecutiveFailures = 0  
> }  
> if state.ConsecutiveFailures >= 2 {  
> 	state.Status = StatusParked  
> 	state.ParkReason = ParkHostFailure  
> }

**F Q5.3.** The ordinary job cap defaults to 120 minutes through `dispatch.cap-min`; explicit, role-pair, runtime-pair, and global configuration may replace it.

`metasystem.conf:13-13`

> dispatch.cap-min=120

`scripts/agents/dispatch.sh:388-414`

> if [[ -n "$explicit_cap_min" ]]; then  
>   cap_min="$explicit_cap_min"  
> ...  
> elif resolved=$(resolve_config_value "dispatch.cap-min" "120"); then  
>   cap_min="$resolved"  
> ...  
> else  
>   cap_min=120  
> fi

**F Q5.4.** Reaping treats a running job as cap-expired at its recorded cap deadline or, for legacy records, at `startedAt + capMin` minutes.

`internal/supervise/reaper.go:135-152`

> if record.CapDeadline != "" {  
> 	deadline, err := time.Parse(time.RFC3339, record.CapDeadline)  
> 	if err != nil {  
> 		return false, err  
> 	}  
> 	return !now.Before(deadline), nil  
> }  
> started, err := time.Parse(time.RFC3339, record.StartedAt)  
> ...  
> return !now.Before(started.Add(time.Duration(record.CapMin) * time.Minute)), nil

**F Q5.5.** An expired running job is transitioned durably to timeout with `budget-cap`.

`internal/supervise/reaper.go:91-96`

> if expired {  
> 	return dispatch.CAS(path, dispatch.StatusRunning, dispatch.StatusTimeout, dispatch.Patch{  
> 		Error: "budget-cap",  
> 	})  
> }

**F Q5.6.** Dispatch expiry performs group wind-down and attempts the timeout CAS; mission dispatch also writes a fence ask, and mirrored status is updated.

`scripts/agents/dispatch.sh:837-851`

> wind_down_group "$job_pgid"  
> "$METASYSTEM_BIN" dispatch record cas \  
>   --path "$record_path" \  
>   --from running \  
>   --to timeout \  
>   --error budget-cap || true  
> if [[ -n "$mission_id" ]]; then  
>   "$METASYSTEM_BIN" mission fence ask ... || true  
> fi  
> mirror_record "$record_path" || true

**F Q5.7.** Mission wall, cycle, job, concurrency, and job-cap bounds have no defaults: all are required positive contract fields.

`internal/mission/contract.go:78-89`

> "fence.wall-hours",  
> "fence.cycles",  
> "fence.jobs",  
> "fence.concurrency",  
> "fence.job-cap-min",  
> ...  
> "fence.wall-hours": {}, "fence.cycles": {}, "fence.jobs": {},  
> "fence.concurrency": {}, "fence.job-cap-min": {},

**F Q5.8.** Fence enforcement compares elapsed hours, used cycles, used jobs, active jobs, and requested job-cap minutes against those contract values.

`internal/mission/fence.go:243-268`

> if elapsed >= time.Duration(contract.Int("fence.wall-hours"))*time.Hour {  
> 	violations = append(violations, "wall-hours")  
> }  
> if state.Cycles >= contract.Int("fence.cycles") {  
> 	violations = append(violations, "cycles")  
> }  
> if state.Jobs >= contract.Int("fence.jobs") {  
> 	violations = append(violations, "jobs")  
> }  
> if active >= contract.Int("fence.concurrency") {  
> 	violations = append(violations, "concurrency")  
> }  
> if requestedCapMin > contract.Int("fence.job-cap-min") {  
> 	violations = append(violations, "job-cap-min")  
> }

**F Q5.9.** A fence breach during cycle reservation writes an ask; the runner then parks the mission, making expiry durable.

`internal/mission/fence.go:426-432`

> if len(violations) > 0 {  
> 	return reservation, writeFenceAsk(paths, state, violations)  
> }

`internal/missionrunner/loop.go:888-890`

> if reservation.Blocked {  
> 	return parkFence(paths, state, reservation)  
> }

**F Q5.10.** Host-turn duration is a required positive mission-contract value in minutes with no default.

`internal/mission/contract.go:84-89`

> "host.turn-cap-min",  
> ...  
> "host.turn-cap-min": {},

`internal/missionrunner/contract.go:242-248`

> turnCapMin := contract.Int("host.turn-cap-min")  
> if turnCapMin <= 0 {  
> 	return HostPlan{}, fmt.Errorf("host.turn-cap-min must be positive")  
> }

**F Q5.11.** Host-turn cap enforcement computes a deadline, terminates the host at expiry, waits for it, and patches the turn outcome to `capped`; subsequent runner processing durably books the fault and measurement.

`internal/missionrunner/host.go:291-329`

> deadline := started.Add(time.Duration(turn.CapMin) * time.Minute)  
> if !now.Before(deadline) {  
> 	if err := terminateHost(turn); err != nil {  
> 		return host.Result{}, err  
> 	}  
> 	if err := waitForHost(turn, 5*time.Second); err != nil {  
> 		return host.Result{}, err  
> 	}  
> 	if err := patchTurnOutcome(turnPath, host.OutcomeCapped); err != nil {  
> 		return host.Result{}, err  
> 	}  
> }

**F Q5.12.** Gate and guard commands use `fence.job-cap-min` minutes as their timeout. Seal or preflight expiry is returned as an error; runtime measurement expiry becomes the runner’s durable `unmeasurable:` no-progress booking.

`internal/mission/contract.go:858-868`

> timeout := time.Duration(contract.Int("fence.job-cap-min")) * time.Minute  
> if timeout <= 0 {  
> 	return nil, fmt.Errorf("fence.job-cap-min must be positive")  
> }  
> return measureCommand(repo, command, timeout)

`internal/mission/measure.go:153-161`

> timeout := time.Duration(contract.Int("fence.job-cap-min")) * time.Minute  
> result, err := measureCommand(repo, guard.Command, timeout)

`internal/mission/measure.go:253-263`

> ctx, cancel := context.WithTimeout(context.Background(), timeout)  
> defer cancel()  
> cmd := exec.CommandContext(ctx, "bash", "-lc", command)

**F Q5.13.** Capability handshake selection has a built-in default of 2 seconds and accepts configured values only from 1 through 60 seconds.

`internal/capability/select.go:100-103`

> timeoutSec := 2  
> if value != "" {  
> 	timeoutSec, err = boundedInt(value, 1, 60)  
> }

**F Q5.14.** Codex and Devin adapters override the generic handshake tolerance to 10 and 30 seconds respectively.

`scripts/agents/adapters/codex.sh:61-69`

> handshake_timeout_sec=10

`scripts/agents/adapters/devin.sh:73-79`

> handshake_timeout_sec=30

**F Q5.15.** Dispatch stamps and waits on a handshake deadline; expiry enters the handshake-timeout path.

`scripts/agents/dispatch.sh:608-653`

> handshake_deadline_epoch=$(( $(date +%s) + handshake_timeout_sec ))  
> ...  
> while (( $(date +%s) < handshake_deadline_epoch )); do  
>   ...  
> done  
> handshake_timeout "$record_path"

**F Q5.16.** Handshake expiry durably transitions the job to failed with `handshake_timeout` and winds down the process group.

`scripts/agents/dispatch.sh:1485-1515`

> "$METASYSTEM_BIN" dispatch record cas \  
>   --path "$record_path" \  
>   --from pending,running \  
>   --to failed \  
>   --error handshake_timeout || true  
> wind_down_group "$job_pgid"

**F Q5.17.** Reaper-side handshake backstop defaults to 2 seconds and derives an expiry deadline from the record when no explicit deadline is present.

`internal/dispatch/reapfacts.go:30-42`

> const (  
> 	defaultHandshakeTimeout = 2 * time.Second  
> 	defaultSetupTimeout     = 10 * time.Minute  
> )

`internal/dispatch/reapfacts.go:65-84`

> deadline := started.Add(defaultHandshakeTimeout)  
> if record.Handshake.Deadline != "" {  
> 	parsed, err := time.Parse(time.RFC3339, record.Handshake.Deadline)  
> 	if err != nil {  
> 		return Facts{}, err  
> 	}  
> 	deadline = parsed  
> }

**F Q5.18.** Pending setup has a hard-coded 10-minute abandonment tolerance; expiry is durably booked as failed `abandoned-setup`.

`internal/dispatch/reapfacts.go:30-36`

> defaultSetupTimeout = 10 * time.Minute

`internal/dispatch/reapfacts.go:53-59`

> facts.SetupExpired = !now.Before(started.Add(defaultSetupTimeout))

`scripts/agents/dispatch.sh:781-785`

> "$METASYSTEM_BIN" dispatch record cas \  
>   --path "$record_path" --from pending --to failed --error abandoned-setup

**F Q5.19.** Drain has a 2-second handshake grace. Its per-job deadline is the latest applicable record deadline plus that grace: pending setup plus 10 minutes, explicit cap deadline plus 2 seconds, or start plus cap minutes plus 2 seconds; malformed records are due immediately.

`internal/missionrunner/drain.go:28-31`

> const drainHandshakeGrace = 2 * time.Second

`internal/missionrunner/drain.go:89-127`

> if record.Status == dispatch.StatusPending {  
> 	return started.Add(10 * time.Minute), nil  
> }  
> if record.CapDeadline != "" {  
> 	deadline, err := time.Parse(time.RFC3339, record.CapDeadline)  
> 	if err != nil {  
> 		return now, nil  
> 	}  
> 	return deadline.Add(drainHandshakeGrace), nil  
> }  
> return started.Add(time.Duration(record.CapMin)*time.Minute + drainHandshakeGrace), nil

**F Q5.20.** Drain waits until all jobs finish or the derived deadline expires; expiry parks as `drain-stalled`, immediately writing state, anchor, and ask, with the ledger booking deferred until resume healing.

`internal/missionrunner/drain.go:42-85`

> for {  
> 	pending, nextDeadline, err := pendingDrainJobs(...)  
> 	if err != nil {  
> 		return err  
> 	}  
> 	if len(pending) == 0 {  
> 		return nil  
> 	}  
> 	if !time.Now().Before(nextDeadline) {  
> 		return parkDrainStalled(paths, state, pending)  
> 	}  
> }

`internal/missionrunner/drain.go:321-363`

> // Drain expiry is parked immediately, but no ledger line is written here.  
> // Resume healing owns the single canonical no-progress booking.

**F Q5.21.** Watcher defaults are 20 stale minutes, 180 cap minutes, and a 60-second interval; start verification defaults to 5 minutes and zero disables it.

`scripts/agents/watch-jobs.sh:52-67`

> stale_min=$(resolve_config_value "watch.stale-min" "20")  
> cap_min=$(resolve_config_value "watch.cap-min" "180")  
> interval_sec=$(resolve_config_value "watch.interval-sec" "60")  
> start_verify_min=$(resolve_config_value "watch.start-verify-min" "5")  
> # A zero start-verification window disables this check.

**F Q5.22.** Watcher expiry classifies records as done, capped, never-started, or stale and writes the observed ID/state to its watcher ledger; it does not transition the canonical job record.

`scripts/agents/watch-jobs.sh:341-356`

> case "$classification" in  
>   done) state="DONE" ;;  
>   capped) state="CAPPED" ;;  
>   never-started) state="NEVER-STARTED" ;;  
>   stale) state="STALE" ;;  
> esac  
> mark "$job_id" "$state"

`scripts/agents/watch-jobs.sh:272-273`

> printf '%s\t%s\t%s\n' "$(date -u +%FT%TZ)" "$job_id" "$state" >>"$watch_ledger"

**F Q5.23.** Watcher startup gives the supervisor identity 10 seconds to appear; expiry exits with failure and has no job-record booking at that site.

`scripts/agents/watch-jobs.sh:143-157`

> deadline=$(( $(date +%s) + 10 ))  
> while [[ ! -s "$supervisor_identity" ]] && (( $(date +%s) < deadline )); do  
>   sleep 1  
> done  
> if [[ ! -s "$supervisor_identity" ]]; then  
>   echo "supervisor identity did not appear" >&2  
>   exit 1  
> fi

**F Q5.24.** Census freshness is bounded to the smaller of twice the watcher interval or 180 seconds; stale census causes attestation refusal and no job transition at that enforcement site.

`internal/dispatch/attest.go:59-63`

> maxAge := 2 * interval  
> if maxAge > 180*time.Second {  
> 	maxAge = 180 * time.Second  
> }

`internal/dispatch/attest.go:91-95`

> if now.Sub(census.GeneratedAt) > maxAge {  
> 	return fmt.Errorf("supervision census is stale")  
> }

**F Q5.25.** Watcher heartbeat freshness permits timestamps from five seconds in the future through `2 × interval + 2` seconds old; violation refuses the watcher pass.

`scripts/agents/watch-jobs.sh:130-137`

> min_epoch=$(( now_epoch - interval_sec * 2 - 2 ))  
> max_epoch=$(( now_epoch + 5 ))  
> (( heartbeat_epoch >= min_epoch && heartbeat_epoch <= max_epoch )) || {  
>   echo "supervisor heartbeat is stale or implausible" >&2  
>   exit 1  
> }

**F Q5.26.** Supervision process observation applies the same `2 × interval + 2 seconds` heartbeat-age bound before reporting a component healthy.

`internal/supervise/proc.go:159-163`

> maxAge := interval*2 + 2*time.Second  
> if now.Sub(heartbeat) > maxAge {  
> 	return Observation{Status: ObservationStale}, nil  
> }

**F Q5.27.** Capability snapshots default to a maximum age of 30 days; stale capability data is rejected during selection without a job booking.

`metasystem.conf:15-15`

> capability.max-age-days=30

`internal/capability/select.go:75-79`

> if now.Sub(snapshot.GeneratedAt) > time.Duration(maxAgeDays)*24*time.Hour {  
> 	return Selection{}, fmt.Errorf("capability snapshot is stale")  
> }

**F Q5.28.** Supervision-owner defaults are a 60-second observation interval, a five-failure give-up threshold, a ten-minute backoff ceiling, and five establishment observations.

`cmd/metasystem/supervise_owner.go:33-33`

> interval := 60 * time.Second

`cmd/metasystem/supervise_owner.go:128-131`

> Breaker: supervise.Breaker{  
> 	GiveUpAt:                 5,  
> 	MaxBackoff:               10 * time.Minute,  
> 	EstablishmentObservations: 5,  
> },

**F Q5.29.** Healthy observations reset the breaker, unknown observations do not increment it, failures back off exponentially, and the fifth failure gives up.

`internal/supervise/decide.go:106-157`

> switch observation.Status {  
> case ObservationHealthy:  
> 	state.ConsecutiveFailures = 0  
> 	state.BackoffUntil = time.Time{}  
> case ObservationUnknown:  
> 	return Decision{Action: ActionWait}, nil  
> default:  
> 	state.ConsecutiveFailures++  
> 	if state.ConsecutiveFailures >= breaker.GiveUpAt {  
> 		return Decision{Action: ActionGiveUp}, nil  
> 	}  
> 	backoff := interval * time.Duration(1<<(state.ConsecutiveFailures-1))  
> 	if backoff > breaker.MaxBackoff {  
> 		backoff = breaker.MaxBackoff  
> 	}  
> }

**F Q5.30.** Supervision give-up tears down the component and appends an exited registry record; registry terminal writing is explicitly best-effort.

`internal/supervise/owner.go:186-204`

> if err := owner.teardown(); err != nil {  
> 	return err  
> }  
> return owner.registry.AppendExited(...)

`internal/supervise/ledger.go:82-89`

> // AppendExited is best-effort terminal bookkeeping for an owner that has  
> // already completed teardown.

**F Q5.31.** A component process may survive its group ceiling by at most one observation interval before the owner classifies it for termination.

`internal/supervise/decide.go:172-180`

> if elapsed > groupCeiling+interval {  
> 	return Decision{Action: ActionTerminate, Reason: "group-ceiling"}, nil  
> }

`cmd/metasystem/supervise_owner.go:129-129`

> GroupCeiling: 12 * time.Hour,

**F Q5.32.** Supervision-owner startup uses a 30-second owner gate, a 30-second registry append lock, a 5-second component stop grace, and a 20-second shutdown latch.

`cmd/metasystem/supervise_owner.go:49-64`

> ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

`cmd/metasystem/supervise_owner.go:93-101`

> RegistryLockTimeout: 30 * time.Second,  
> ComponentStopGrace:  5 * time.Second,

`cmd/metasystem/supervise_owner.go:122-122`

> latchTimeout := 20 * time.Second

**F Q5.33.** Process teardown sends TERM, waits five seconds, then sends KILL and polls for up to another half second.

`internal/supervise/proc.go:181-211`

> if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {  
> 	return err  
> }  
> deadline := time.Now().Add(5 * time.Second)  
> ...  
> if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {  
> 	return err  
> }  
> for i := 0; i < 25; i++ {  
> 	time.Sleep(20 * time.Millisecond)  
> }

**F Q5.34.** Shell supervision arming uses a 10-second cap-authority lock, 5-second identity-start wait, 5-second first-heartbeat wait, and `interval + 10 seconds` verification wait.

`scripts/agents/arm-supervision.sh:75-95`

> cap_lock_wait_sec=10

`scripts/agents/arm-supervision.sh:208-219`

> wait_for_identity "$owner_identity" 5

`scripts/agents/arm-supervision.sh:234-249`

> wait_for_heartbeat "$heartbeat_file" 5

`scripts/agents/arm-supervision.sh:266-299`

> verify_deadline=$(( $(date +%s) + interval_sec + 10 ))

**F Q5.35.** Supervision join waits five seconds for owner identity; stop waits five seconds before kill and then one additional second.

`scripts/agents/arm-supervision.sh:441-450`

> wait_for_identity "$owner_identity" 5

`scripts/agents/arm-supervision.sh:177-199`

> stop_deadline=$(( $(date +%s) + 5 ))  
> ...  
> kill -KILL "$pid" 2>/dev/null || true  
> sleep 1

**F Q5.36.** Dispatch lifecycle-lock reaping waits five seconds; cap-authority waits ten seconds; the chain lock is attempted without a wait loop.

`scripts/agents/dispatch.sh:855-870`

> lifecycle_lock_deadline=$(( $(date +%s) + 5 ))

`scripts/agents/dispatch.sh:339-353`

> cap_lock_deadline=$(( $(date +%s) + 10 ))

`scripts/agents/dispatch.sh:295-305`

> if ! mkdir "$chain_lock" 2>/dev/null; then  
>   die "chain is locked: $chain_id"  
> fi

**F Q5.37.** Lease-lock acquisition defaults to ten seconds and is configurable through `METASYSTEM_LEASE_LOCK_WAIT_SEC`.

`internal/lease/lock.go:13-51`

> const defaultLockWait = 10 * time.Second  
> ...  
> if raw := os.Getenv("METASYSTEM_LEASE_LOCK_WAIT_SEC"); raw != "" {  
> 	seconds, err := strconv.Atoi(raw)  
> 	if err != nil || seconds <= 0 {  
> 		return nil, fmt.Errorf("invalid METASYSTEM_LEASE_LOCK_WAIT_SEC")  
> 	}  
> 	wait = time.Duration(seconds) * time.Second  
> }

**F Q5.38.** Canonical dispatch records, mission ledgers, and mission fences use blocking `flock` calls without repository-defined time bounds.

`internal/dispatch/record.go:88-110`

> if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {  
> 	return err  
> }  
> defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)

`internal/mission/ledger.go:410-425`

> if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {  
> 	return err  
> }

`internal/mission/fence.go:804-818`

> if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {  
> 	return err  
> }

**F Q5.39.** Adapter process-identity verification waits five seconds; launch failure is durably booked as `launch_failed`.

`scripts/agents/dispatch.sh:586-606`

> identity_deadline=$(( $(date +%s) + 5 ))

`scripts/agents/dispatch.sh:1058-1063`

> "$METASYSTEM_BIN" dispatch record cas \  
>   --path "$record_path" \  
>   --from pending \  
>   --to failed \  
>   --error launch_failed

**F Q5.40.** Runtime CLI-custody verification defaults to five seconds; adapter failure uses the durable `custody_registration` failure path.

`scripts/agents/adapters/runtime-common.sh:74-86`

> custody_timeout_sec="${METASYSTEM_CUSTODY_TIMEOUT_SEC:-5}"  
> custody_deadline=$(( $(date +%s) + custody_timeout_sec ))

`scripts/agents/adapters/claude.sh:156-156`

> fail_pending "custody_registration"

**F Q5.41.** Host-start proof defaults to five seconds. Expiry terminates the host and patches the turn `start-unverified`; the runner then books the failure.

`internal/missionrunner/host.go:223-276`

> startProofTimeout := scaledDuration(5 * time.Second)  
> deadline := time.Now().Add(startProofTimeout)  
> ...  
> if !verified {  
> 	_ = terminateHost(turn)  
> 	_ = patchTurnOutcome(turnPath, "start-unverified")  
> 	return HostStart{}, fmt.Errorf("host start could not be verified")  
> }

`internal/missionrunner/loop.go:990-991`

> return recordFailedTurn(paths, state, turn, "start-unverified", 2)

**F Q5.42.** The outer mission-launch verifier has a 15-second default and a five-second wind-down; expiry returns an error after termination and does not itself define a separate ledger record.

`internal/missionrunner/launch.go:389-443`

> verifyDeadline := time.Now().Add(scaledDuration(15 * time.Second))  
> ...  
> if err := terminateProcessGroup(pgid, scaledDuration(5*time.Second)); err != nil {  
> 	return err  
> }  
> return fmt.Errorf("mission runner start could not be verified")

**F Q5.43.** Dispatch process-group wind-down allows two seconds after TERM and two after KILL.

`scripts/agents/dispatch.sh:257-275`

> kill -TERM "-$pgid" 2>/dev/null || true  
> wait_deadline=$(( $(date +%s) + 2 ))  
> ...  
> kill -KILL "-$pgid" 2>/dev/null || true  
> wait_deadline=$(( $(date +%s) + 2 ))

**F Q5.44.** Adapter child wind-down uses a two-second TERM grace before KILL.

`scripts/agents/adapters/runtime-common.sh:175-182`

> kill -TERM "$child_pid" 2>/dev/null || true  
> deadline=$(( $(date +%s) + 2 ))  
> ...  
> kill -KILL "$child_pid" 2>/dev/null || true

**F Q5.45.** Mission host-group wind-down allows five seconds after TERM, then sends KILL; host wait sites use a five-second grace.

`internal/missionrunner/host.go:91-127`

> if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {  
> 	return err  
> }  
> deadline := time.Now().Add(5 * time.Second)  
> ...  
> return syscall.Kill(-pgid, syscall.SIGKILL)

`internal/missionrunner/host.go:314-320`

> if err := waitForHost(turn, 5*time.Second); err != nil {  
> 	return host.Result{}, err  
> }

**F Q5.46.** Claude enforcement hooks allow 15 seconds at start, 5 seconds at stop, and 3 seconds at end; these hook configurations contain no repository-side durable booking instruction for timeout.

`scripts/enforcement/claude.json:9-11`

> "timeout": 15

`scripts/enforcement/claude.json:24-26`

> "timeout": 5

`scripts/enforcement/claude.json:35-37`

> "timeout": 3

**F Q5.47.** Codex enforcement hooks use the same 15-second start, 5-second stop, and 3-second end bounds.

`scripts/enforcement/codex.json:9-11`

> "timeout": 15

`scripts/enforcement/codex.json:20-22`

> "timeout": 5

`scripts/enforcement/codex.json:31-33`

> "timeout": 3

**F Q5.48.** Devin enforcement hooks also use 15-second start, 5-second stop, and 3-second end bounds.

`scripts/enforcement/devin.json:7-9`

> "timeout": 15

`scripts/enforcement/devin.json:18-20`

> "timeout": 5

`scripts/enforcement/devin.json:29-31`

> "timeout": 3

**F Q5.49.** Adapter self-test waits default to 240 seconds, except Devin’s 900 seconds; timeout returns failure without cancelling or durably booking a job.

`scripts/agents/adapters/runtime-common.sh:370-382`

> timeout_sec="${SELFTEST_TIMEOUT_SEC:-240}"  
> ...  
> if (( $(date +%s) >= deadline )); then  
>   return 1  
> fi

`scripts/agents/adapters/devin.sh:401-401`

> SELFTEST_TIMEOUT_SEC=900

**F Q5.50.** The adapter listener defaults to 180 seconds and exits cleanly on idle expiry without creating a request record.

`cmd/metasystem/adapter_listener.go:108-125`

> idleTimeout := 180 * time.Second  
> ...  
> case <-time.After(idleTimeout):  
> 	return nil

**F Q5.51.** Retro cadence defaults are 30 receipts or 25 days; an expired check returns failure, while durable receipt output is written only when retro is run.

`scripts/receipt.sh:27-30`

> max_age_days=25  
> max_receipts=30

`scripts/receipt.sh:80-85`

> max_age_days=$(resolve_config_value "retro.max-age-days" "$max_age_days")  
> max_receipts=$(resolve_config_value "retro.max-receipts" "$max_receipts")

`scripts/receipt.sh:239-245`

> if (( receipts_since >= max_receipts || age_days >= max_age_days )); then  
>   echo "retro due" >&2  
>   exit 1  
> fi

`scripts/receipt.sh:174-179`

> printf '%s\n' "$receipt" >>"$receipt_log"

**F Q5.52.** Refactor cadence defaults are 1,440 minutes or 40 commits; expiry fails enforcement and does not update the baseline.

`scripts/refactor.sh:23-25`

> max_age_min=1440  
> max_commits=40

`scripts/refactor.sh:54-59`

> max_age_min=$(resolve_config_value "refactor.max-age-min" "$max_age_min")  
> max_commits=$(resolve_config_value "refactor.max-commits" "$max_commits")

`scripts/refactor.sh:111-117`

> if (( age_min >= max_age_min || commits_since >= max_commits )); then  
>   echo "refactor due" >&2  
>   exit 1  
> fi

**F Q5.53.** Completed but unclosed chains remain current for a default 5,400-second grace configured by `METASYSTEM_CHAIN_GRACE_SECONDS`; expiry changes open-work reporting rather than the canonical job record.

`scripts/open-work.sh:34-40`

> chain_grace_seconds="${METASYSTEM_CHAIN_GRACE_SECONDS:-5400}"

`scripts/open-work.sh:165-185`

> if [[ "$status" == "completed" && "$chain_open" == "true" ]]; then  
>   if (( now_epoch - ended_epoch <= chain_grace_seconds )); then  
>     current=true  
>   fi  
> fi

**F Q5.54.** Evidence garbage collection uses the same default 5,400-second chain grace; after expiry it removes mirrored job evidence.

`scripts/evidence-gc.sh:44-48`

> chain_grace_seconds="${METASYSTEM_CHAIN_GRACE_SECONDS:-5400}"

`cmd/metasystem/evidence_verbs.go:23-23`

> chainGraceSeconds := envPositiveInt("METASYSTEM_CHAIN_GRACE_SECONDS", 5400)

`internal/evidence/gc.go:401-405`

> if err := os.Remove(recordPath); err != nil && !os.IsNotExist(err) {  
> 	return err  
> }

**F Q5.55.** Mission anchoring waits eight half-second intervals for the Git index lock, then removes it only when its age is at least four seconds; this recovery is not separately ledger-booked.

`internal/missionrunner/anchor.go:51-69`

> for attempt := 0; attempt < 8; attempt++ {  
> 	if _, err := os.Stat(lockPath); os.IsNotExist(err) {  
> 		return nil  
> 	}  
> 	time.Sleep(500 * time.Millisecond)  
> }  
> info, err := os.Stat(lockPath)  
> if err == nil && time.Since(info.ModTime()) >= 4*time.Second {  
> 	return os.Remove(lockPath)  
> }

**F Q5.56.** Watcher cap authority is derived as the maximum configured dispatch, fence, or pair cap plus 30 minutes; dispatch requires its cap to remain below that ceiling.

`scripts/agents/arm-supervision.sh:43-68`

> watch_cap_min=120  
> ...  
> if (( configured_cap > watch_cap_min )); then  
>   watch_cap_min=$configured_cap  
> fi  
> watch_cap_min=$(( watch_cap_min + 30 ))

`scripts/agents/dispatch.sh:1011-1013`

> (( cap_min < watch_cap_min )) || die "dispatch cap ${cap_min}m must be below watcher cap ${watch_cap_min}m"

**F Q5.57.** The premise that every wait is bounded is false: runtime gate waiting is an unbounded loop whose termination depends on external gate state.

`scripts/agents/adapters/runtime-common.sh:43-50`

> while [[ -e "$gate_path" ]]; do  
>   sleep 1  
> done

**F Q5.58.** Dispatch’s job wait loop also has no independent shell deadline; it relies on job-record terminal transitions and reaping.

`scripts/agents/dispatch.sh:656-678`

> while true; do  
>   status=$("$METASYSTEM_BIN" dispatch record get --path "$record_path" --field status)  
>   case "$status" in  
>     completed|failed|timeout) return 0 ;;  
>   esac  
>   reap_job "$record_path" || true  
>   sleep 1  
> done
