package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	runtimereg "github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// Validation of the whole metasystem.conf domain: the runtime roster, capability
// floors, model tiers, role/model resolution, and the evidence root, checked
// against the adopting repository at repoRoot. It returns every problem found
// (each rendered without a prefix) rather than stopping at the first, plus
// tiersAbsent signals that no model tier is configured, a valid state whose
// INFO line belongs to the command surface. A non-nil err is a hard read
// failure.
func Validate(confPath, repoRoot string) (tiersAbsent bool, problems []string, err error) {
	content, readErr := os.ReadFile(confPath)
	if readErr != nil {
		return false, nil, fmt.Errorf("cannot read metasystem configuration: %s: %w", confPath, readErr)
	}
	repo := resolvePath(repoRoot)

	var errs []string
	add := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	// The committed file: parse every setting once, keeping insertion order so
	// downstream checks and messages are stable.
	values := map[string]string{}
	var order []string
	type capKey struct {
		source string
		line   int
		key    string
		value  string
	}
	var capKeys []capKey
	// Duplicates are a PROBLEM here, deliberately: validation names what
	// the hot-path readers tolerate by last-wins.
	parseSettings(string(content), func(lineNo int, key, value string, ok bool) {
		if !ok {
			add("%s:%d: expected key=value", confPath, lineNo)
			return
		}
		if !confKeyPattern.MatchString(key) {
			add("%s:%d: invalid key %s", confPath, lineNo, pyRepr(key))
			return
		}
		if _, exists := values[key]; exists {
			add("%s:%d: duplicate key %s", confPath, lineNo, key)
			return
		}
		values[key] = value
		order = append(order, key)
		if strings.HasPrefix(key, "cap.min.") {
			capKeys = append(capKeys, capKey{confPath, lineNo, key, value})
		}
	})

	// The .local override contributes capability floors here. Budget settings
	// are checked with the other numeric knobs below; other keys are the
	// developer's own and not template invariants.
	localPath := confPath + ".local"
	if isFile(localPath) {
		localContent, localErr := os.ReadFile(localPath)
		if localErr != nil {
			add("cannot read metasystem local configuration: %s: %v", localPath, localErr)
		}
		localSeen := map[string]bool{}
		for number, raw := range strings.Split(string(localContent), "\n") {
			lineNo := number + 1
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			name, val, _ := strings.Cut(line, "=")
			key := strings.TrimSpace(name)
			value := strings.TrimSpace(val)
			if !strings.HasPrefix(key, "cap.min.") {
				continue
			}
			if localSeen[key] {
				add("%s:%d: duplicate key %s", localPath, lineNo, key)
				continue
			}
			localSeen[key] = true
			capKeys = append(capKeys, capKey{localPath, lineNo, key, value})
		}
	}

	for _, key := range order {
		if strings.HasPrefix(key, "mode.") && !modeScopedKey.MatchString(key) {
			add("%s is not a supported mode-scoped key", key)
		}
	}
	retiredPresent := false
	for key, message := range retiredKeys {
		if _, present := values[key]; present {
			add("%s %s", key, message)
			retiredPresent = true
		}
		if isFile(localPath) {
			if _, present, lookupErr := ConfLookup(localPath, key); lookupErr != nil {
				add("%v", lookupErr)
			} else if present {
				add("%s in %s %s", key, localPath, message)
				retiredPresent = true
			}
		}
		if _, present := os.LookupEnv(EnvName(key)); present {
			add("%s in environment source %s %s", key, EnvName(key), message)
			retiredPresent = true
		}
	}

	// The runtime roster gates almost everything else.
	if _, ok := values["metasystem.runtimes"]; !ok {
		add("metasystem.runtimes is required")
	}
	var runtimes []string
	runtimeSet := map[string]bool{}
	for _, item := range strings.Split(values["metasystem.runtimes"], ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		runtimes = append(runtimes, item)
	}
	for _, r := range runtimes {
		runtimeSet[r] = true
	}
	if len(runtimes) != len(runtimeSet) {
		add("metasystem.runtimes contains a duplicate runtime")
	}
	var unsupported []string
	for r := range runtimeSet {
		if !runtimereg.Supported(r) {
			unsupported = append(unsupported, r)
		}
	}
	sort.Strings(unsupported)
	for _, r := range unsupported {
		add("metasystem.runtimes names unsupported runtime %s", pyRepr(r))
	}
	for _, problem := range validateSpendConfiguration(confPath, localPath, values, runtimeSet, fixtureBudgetLawRoot(confPath)) {
		add("%s", problem)
	}
	for _, key := range order {
		match := maximalModelsKey.FindStringSubmatch(key)
		if match == nil {
			continue
		}
		runtime := match[1]
		if !runtimereg.Supported(runtime) {
			add("%s names unsupported runtime %s", key, pyRepr(runtime))
		}
		seen := map[string]bool{}
		for _, model := range strings.Split(values[key], ",") {
			model = strings.TrimSpace(model)
			if model == "" {
				add("%s must contain only non-empty comma-separated model names", key)
				continue
			}
			if seen[model] {
				add("%s contains duplicate model %s", key, pyRepr(model))
			}
			seen[model] = true
		}
	}

	// Model aliases are committed, direct family pointers. Their targets must
	// be admitted by both the tracked maximal-model list and every configured
	// higher-precedence maximal-model value; draining sources may remain only
	// in those uncommitted admission overlays.
	type modelAlias struct {
		key, runtime, source, target string
	}
	var aliases []modelAlias
	aliasSources := map[string]bool{}
	for _, key := range order {
		match := modelAliasKey.FindStringSubmatch(key)
		if match == nil {
			continue
		}
		alias := modelAlias{key: key, runtime: match[1], source: strings.TrimSpace(match[2]), target: strings.TrimSpace(values[key])}
		aliases = append(aliases, alias)
		if alias.source != "" {
			aliasSources[alias.runtime+"\x00"+alias.source] = true
		}
	}
	containsModel := func(value, wanted string) bool {
		for _, model := range strings.Split(value, ",") {
			if strings.TrimSpace(model) == wanted {
				return true
			}
		}
		return false
	}
	for _, alias := range aliases {
		if !runtimeSet[alias.runtime] {
			add("%s names runtime %s outside metasystem.runtimes", alias.key, pyRepr(alias.runtime))
		}
		if alias.source == "" || CanonicalModel(alias.source) == "" {
			add("%s source must be non-empty", alias.key)
		} else if alias.source != CanonicalModel(alias.source) {
			add("%s source %s is non-canonical; use %s", alias.key, pyRepr(alias.source), pyRepr(CanonicalModel(alias.source)))
		}
		if alias.target == "" || CanonicalModel(alias.target) == "" {
			add("%s target must be non-empty", alias.key)
			continue
		}
		if alias.target != CanonicalModel(alias.target) {
			add("%s target %s is non-canonical; use %s", alias.key, pyRepr(alias.target), pyRepr(CanonicalModel(alias.target)))
		}
		if alias.source == alias.target {
			add("%s must not alias a model to itself", alias.key)
		}
		if aliasSources[alias.runtime+"\x00"+alias.target] {
			add("%s target %s is also an alias source; model-alias chains are not allowed", alias.key, pyRepr(alias.target))
		}
		maximalKey := "runtime." + alias.runtime + ".maximal-models"
		trackedMaximal := values[maximalKey]
		if alias.source != "" && containsModel(trackedMaximal, alias.source) {
			add("%s source %s must be absent from tracked %s", alias.key, pyRepr(alias.source), maximalKey)
		}
		if !containsModel(trackedMaximal, alias.target) {
			add("%s target %s must be present in tracked %s", alias.key, pyRepr(alias.target), maximalKey)
		}
		if localValue, present, lookupErr := ConfLookup(localPath, maximalKey); lookupErr != nil && isFile(localPath) {
			add("%v", lookupErr)
		} else if present && !containsModel(localValue, alias.target) {
			add("%s value for %s omits model-alias target %s", localPath, maximalKey, pyRepr(alias.target))
		}
		if envValue, present := os.LookupEnv(EnvName(maximalKey)); present && !containsModel(envValue, alias.target) {
			add("environment source %s for %s omits model-alias target %s", EnvName(maximalKey), maximalKey, pyRepr(alias.target))
		}
	}

	fixtureAliasOverrides := fixtureBudgetLawRoot(confPath)
	if isFile(localPath) {
		localContent, localErr := os.ReadFile(localPath)
		if localErr != nil {
			add("cannot read metasystem local configuration: %s: %v", localPath, localErr)
		} else {
			parseSettings(string(localContent), func(lineNo int, key, _ string, ok bool) {
				if !ok || modelAliasKey.FindStringSubmatch(key) == nil {
					return
				}
				if !fixtureAliasOverrides {
					add("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s:%d is refused", key, localPath, lineNo)
				}
			})
		}
	}
	if !fixtureAliasOverrides {
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if modelAliasEnvName.MatchString(name) {
				add("model aliases accept only committed root configuration outside a fixture-authorized root; environment source %s is refused", name)
			}
		}
	}

	// Capability floors must name a rostered runtime and a canonical model, and
	// carry a positive integer.
	for _, ck := range capKeys {
		parts := strings.Split(ck.key, ".")
		var prefix, rawModel string
		switch {
		case len(parts) >= 4 && runtimeSet[parts[2]]:
			prefix = "cap.min." + parts[2] + "."
			rawModel = strings.Join(parts[3:], ".")
		case len(parts) >= 5 && runtimeSet[parts[3]]:
			prefix = "cap.min." + parts[2] + "." + parts[3] + "."
			rawModel = strings.Join(parts[4:], ".")
		default:
			add("%s:%d: unsupported cap key %s", ck.source, ck.line, ck.key)
			continue
		}
		canonical := CanonicalModel(rawModel)
		canonicalKey := prefix + canonical
		if canonical == "" || ck.key != canonicalKey {
			shown := canonical
			if shown == "" {
				shown = "<empty>"
			}
			add("%s:%d: non-canonical cap key %s; use %s (canonical model %s)", ck.source, ck.line, ck.key, canonicalKey, shown)
		}
		if !positiveInteger.MatchString(ck.value) {
			add("%s:%d: %s must be a positive integer", ck.source, ck.line, ck.key)
		}
	}

	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "METASYSTEM_CAP_MIN_") && !positiveInteger.MatchString(value) {
			add("environment cap source %s must be a positive integer", name)
		}
	}

	// A role runtime, plain or mode-scoped, must be main or a rostered runtime.
	for _, key := range order {
		if roleRuntimeSuffix.MatchString(key) {
			value := values[key]
			if value != "main" && !runtimeSet[value] {
				add("%s names runtime %s outside metasystem.runtimes", key, pyRepr(value))
			}
		}
	}

	// Model tiers rank runtime-qualified models by cost. They must be numbered
	// contiguously from 1: dispatch scans until the first missing index, so a
	// gap would silently truncate the ranking.
	tierMemberCounts := map[string]int{}
	var tierKeys []string
	var tierIndices []int
	for _, key := range order {
		if !strings.HasPrefix(key, "model.tier.") {
			continue
		}
		m := tierKeyPattern.FindStringSubmatch(key)
		if m == nil {
			add("%s is not a supported model tier key", key)
			continue
		}
		tierKeys = append(tierKeys, key)
		tierIndices = append(tierIndices, atoi(m[1]))
		for _, member := range strings.Split(values[key], ",") {
			member = strings.TrimSpace(member)
			if member == "" {
				continue
			}
			runtime, model, ok := strings.Cut(member, ":")
			if !ok || runtime == "" || model == "" {
				add("%s entry %s is not runtime-qualified", key, pyRepr(member))
				continue
			}
			if !runtimeSet[runtime] {
				add("%s entry %s names a runtime outside metasystem.runtimes", key, pyRepr(member))
			}
			tierMemberCounts[member]++
		}
	}
	sort.Ints(tierIndices)
	if len(tierIndices) > 0 && !contiguousFromOne(tierIndices) {
		labels := make([]string, len(tierIndices))
		for i, n := range tierIndices {
			labels[i] = fmt.Sprintf("%d", n)
		}
		add("model tiers must be numbered contiguously from 1 (no gaps); found %s", strings.Join(labels, ", "))
	}

	// Every configured role model must name a rostered runtime and, when tiers
	// exist, appear in exactly one tier so a model change can be ranked.
	type configuredModel struct {
		key       string
		qualified string
	}
	var configuredModels []configuredModel
	for _, key := range order {
		var runtime string
		if m := plainModelKey.FindStringSubmatch(key); m != nil {
			runtime = m[2]
		} else if m := modeModelKey.FindStringSubmatch(key); m != nil {
			runtime = m[3]
		} else {
			continue
		}
		if !runtimeSet[runtime] {
			add("%s names runtime %s outside metasystem.runtimes", key, pyRepr(runtime))
		}
		configuredModels = append(configuredModels, configuredModel{key, runtime + ":" + values[key]})
	}
	if len(tierKeys) > 0 {
		for _, cm := range configuredModels {
			count := tierMemberCounts[cm.qualified]
			if count != 1 {
				add("%s model %s appears in %d model tiers; expected exactly one", cm.key, pyRepr(cm.qualified), count)
			}
		}
	}

	// Enumerate the roles and modes actually configured, then check that every
	// (mode, role) resolves to a runtime that has a model.
	roles := map[string]bool{}
	modes := map[string]bool{}
	for _, key := range order {
		if m := roleComponentKey.FindStringSubmatch(key); m != nil && m[1] != "default" {
			roles[m[1]] = true
		}
		if m := modeComponentKey.FindStringSubmatch(key); m != nil {
			modes[m[1]] = true
			if m[2] != "default" {
				roles[m[2]] = true
			}
		}
	}
	resolved := func(key, mode string) (string, bool) {
		if mode != "" {
			if v, ok := values["mode."+mode+"."+key]; ok {
				return v, true
			}
		}
		v, ok := values[key]
		return v, ok
	}
	truthy := func(v string, ok bool) bool { return ok && v != "" }

	modeScopes := append([]string{""}, sortedKeysOf(modes)...)
	for _, mode := range modeScopes {
		runtime, ok := resolved("role.default.runtime", mode)
		if truthy(runtime, ok) && runtime != "main" {
			if _, present := resolved("role.default.model."+runtime, mode); !present {
				add("%s resolves to %s but has no model.%s value", roleLabel("default", mode, true), runtime, runtime)
			}
		}
	}
	for _, role := range sortedKeysOf(roles) {
		for _, mode := range modeScopes {
			runtime, ok := resolved("role."+role+".runtime", mode)
			if !truthy(runtime, ok) {
				runtime, ok = resolved("role.default.runtime", mode)
			}
			if !truthy(runtime, ok) || runtime == "main" {
				continue
			}
			model, present := resolved("role."+role+".model."+runtime, mode)
			if !truthy(model, present) {
				_, present = resolved("role.default.model."+runtime, mode)
			}
			if !present {
				add("%s resolves to %s but has no model.%s value", roleLabel(role, mode, false), runtime, runtime)
			}
		}
	}

	// Numeric operational knobs are soft-defaulted at READ time (a malformed
	// bound must not disable bounding), which is only safe because the typo
	// surfaces HERE (review foundations-9): without this check an operator
	// who fat-fingers exec.local-timeout-sec=300s silently runs on defaults
	// and discovers it from the exact hang class the bound exists to prevent.
	for _, knob := range []string{
		"exec.local-timeout-sec", "exec.network-timeout-sec",
		"watch.interval-sec", "watch.stale-min", "watch.cap-min",
		"census.log-max-bytes", "metasystem.counselor.brief-cadence-hours", "dispatch.cap-max",
	} {
		if raw, present := values[knob]; present {
			if parsed, parseErr := strconv.Atoi(raw); parseErr != nil || parsed < 1 {
				add("%s must be a positive integer, got %s", knob, pyRepr(raw))
			}
		}
	}
	if raw, present := values["census.max-interval-share-percent"]; present {
		if parsed, parseErr := strconv.Atoi(raw); parseErr != nil || parsed < 1 || parsed > 100 {
			add("census.max-interval-share-percent must be an integer between 1 and 100, got %s", pyRepr(raw))
		}
	}
	if raw, present := values[ElapsedGracePercentKey]; present {
		if _, parseErr := parseElapsedGracePercent(raw); parseErr != nil {
			add("%v", parseErr)
		}
	}
	fixtureBudgetOverrides := fixtureBudgetLawRoot(confPath)
	if maximum, maxErr := ReviewRoundMax(confPath); maxErr != nil {
		add("%v", maxErr)
	} else if !retiredPresent {
		for tier := uint8(1); tier <= 3; tier++ {
			box, boxErr := TierBox(confPath, tier)
			if boxErr != nil {
				add("%v", boxErr)
				continue
			}
			if maximum > 0 && uint64(box.ReviewRoundLimit) > maximum {
				add("tier %d reviewRoundLimit %d exceeds %s=%d", tier, box.ReviewRoundLimit, ReviewRoundMaxKey, maximum)
			}
		}
	}
	if isFile(localPath) {
		if raw, present, lookupErr := ConfLookup(localPath, ElapsedGracePercentKey); lookupErr != nil {
			add("%v", lookupErr)
		} else if present {
			if !fixtureBudgetOverrides {
				add("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s is refused", ElapsedGracePercentKey, localPath)
			} else if _, parseErr := parseElapsedGracePercent(raw); parseErr != nil {
				add("%s: %v", localPath, parseErr)
			}
		}
	}
	if raw, present := os.LookupEnv(EnvName(ElapsedGracePercentKey)); present {
		if !fixtureBudgetOverrides {
			add("%s accepts only committed root configuration outside a fixture-authorized root; environment source %s is refused", ElapsedGracePercentKey, EnvName(ElapsedGracePercentKey))
		} else if _, parseErr := parseElapsedGracePercent(raw); parseErr != nil {
			add("environment source %s: %v", EnvName(ElapsedGracePercentKey), parseErr)
		}
	}
	if raw, present := values[SliceNormHoursKey]; present {
		if _, parseErr := parseSliceNormHours(raw); parseErr != nil {
			add("%v", parseErr)
		}
	}
	if isFile(localPath) {
		if raw, present, lookupErr := ConfLookup(localPath, SliceNormHoursKey); lookupErr != nil {
			add("%v", lookupErr)
		} else if present {
			if !fixtureBudgetOverrides {
				add("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s is refused", SliceNormHoursKey, localPath)
			} else if _, parseErr := parseSliceNormHours(raw); parseErr != nil {
				add("%s: %v", localPath, parseErr)
			}
		}
	}
	if raw, present := os.LookupEnv(EnvName(SliceNormHoursKey)); present {
		if !fixtureBudgetOverrides {
			add("%s accepts only committed root configuration outside a fixture-authorized root; environment source %s is refused", SliceNormHoursKey, EnvName(SliceNormHoursKey))
		} else if _, parseErr := parseSliceNormHours(raw); parseErr != nil {
			add("environment source %s: %v", EnvName(SliceNormHoursKey), parseErr)
		}
	}
	if raw, present := values[LedgerAttentionStaleMinutesKey]; present {
		if _, parseErr := parseLedgerAttentionStaleMinutes(raw); parseErr != nil {
			add("%v", parseErr)
		}
	}
	if isFile(localPath) {
		if raw, present, lookupErr := ConfLookup(localPath, LedgerAttentionStaleMinutesKey); lookupErr != nil {
			add("%v", lookupErr)
		} else if present {
			if !fixtureBudgetOverrides {
				add("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s is refused", LedgerAttentionStaleMinutesKey, localPath)
			} else if _, parseErr := parseLedgerAttentionStaleMinutes(raw); parseErr != nil {
				add("%s: %v", localPath, parseErr)
			}
		}
	}
	if raw, present := os.LookupEnv(EnvName(LedgerAttentionStaleMinutesKey)); present {
		if !fixtureBudgetOverrides {
			add("%s accepts only committed root configuration outside a fixture-authorized root; environment source %s is refused", LedgerAttentionStaleMinutesKey, EnvName(LedgerAttentionStaleMinutesKey))
		} else if _, parseErr := parseLedgerAttentionStaleMinutes(raw); parseErr != nil {
			add("environment source %s: %v", EnvName(LedgerAttentionStaleMinutesKey), parseErr)
		}
	}
	// The evidence root is required, must be absolute, and must live outside the
	// repository so job records never write inside the tree they observe.
	evidence := values["evidence.root"]
	switch {
	case evidence == "":
		add("evidence.root is required")
	case !filepath.IsAbs(evidence):
		add("evidence.root must be absolute")
	default:
		if withinRepo(resolvePath(evidence), repo) {
			add("evidence.root must be outside the repository")
		}
	}

	// Registration is adopted-repository state, not a template invariant (the
	// template carries development/metasystem-design.md). The fake runtime is a
	// fixture adapter with no external registration.
	if !isFile(filepath.Join(repo, "development", "metasystem-design.md")) {
		for _, runtime := range sortedKeysOf(runtimeSet) {
			declaration, _ := runtimereg.Lookup(runtime)
			for _, relative := range declaration.RegistrationDirs {
				if !isDir(filepath.Join(repo, filepath.FromSlash(relative))) {
					add("metasystem.runtimes enables %s but registration directory %s is missing", pyRepr(runtime), relative)
				}
			}
		}
	}

	return len(tierKeys) == 0, errs, nil
}

func validateSpendConfiguration(confPath, localPath string, values map[string]string, runtimeSet map[string]bool, fixtureOverrides bool) []string {
	var problems []string
	checkFixed := func(key, value string) {
		candidate := make(map[string]string, len(spendFixedDefaults))
		for fixed, fallback := range spendFixedDefaults {
			candidate[fixed] = fallback
		}
		candidate[key] = value
		if err := validateSpendFixedValues(candidate); err != nil {
			problems = append(problems, err.Error())
		}
	}
	checkPrice := func(key, value string) {
		parsed, err := parseSpendPriceKey(key)
		if err != nil {
			problems = append(problems, err.Error())
			return
		}
		if !runtimeSet[parsed.Runtime] {
			problems = append(problems, fmt.Sprintf("%s names runtime %s outside metasystem.runtimes", key, pyRepr(parsed.Runtime)))
		}
		if !spendNonnegativeDecimal.MatchString(value) {
			problems = append(problems, fmt.Sprintf("%s must be a non-negative decimal, got %s", key, pyRepr(value)))
		}
	}
	for key, value := range values {
		if _, fixed := spendFixedDefaults[key]; fixed {
			checkFixed(key, value)
		}
		if strings.HasPrefix(key, "spend.price.") {
			checkPrice(key, value)
		}
	}
	if content, err := os.ReadFile(localPath); err == nil {
		seen := map[string]bool{}
		parseSettings(string(content), func(line int, key, value string, ok bool) {
			if !ok || !strings.HasPrefix(key, "spend.") {
				return
			}
			if seen[key] {
				problems = append(problems, fmt.Sprintf("%s:%d: duplicate key %s", localPath, line, key))
				return
			}
			seen[key] = true
			if !fixtureOverrides {
				problems = append(problems, fmt.Sprintf("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s is refused", key, localPath))
				return
			}
			if _, fixed := spendFixedDefaults[key]; fixed {
				checkFixed(key, value)
			} else if strings.HasPrefix(key, "spend.price.") {
				checkPrice(key, value)
			}
		})
	}
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "METASYSTEM_SPEND_") {
			continue
		}
		if !fixtureOverrides {
			problems = append(problems, fmt.Sprintf("spend settings accept only committed root configuration outside a fixture-authorized root; environment source %s is refused", name))
			continue
		}
		for key := range spendFixedDefaults {
			if name == EnvName(key) {
				checkFixed(key, value)
			}
		}
	}
	return problems
}

var (
	modeScopedKey     = regexp.MustCompile(`^mode\.[a-z0-9-]+\.role\.[a-z0-9-]+\.(?:runtime|model\.[a-z0-9-]+)$`)
	positiveInteger   = regexp.MustCompile(`^[1-9][0-9]*$`)
	roleRuntimeSuffix = regexp.MustCompile(`(?:^|\.)role\.[a-z0-9-]+\.runtime$`)
	tierKeyPattern    = regexp.MustCompile(`^model\.tier\.([1-9][0-9]*)$`)
	plainModelKey     = regexp.MustCompile(`^role\.([a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
	modeModelKey      = regexp.MustCompile(`^mode\.([a-z0-9-]+)\.role\.([a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
	maximalModelsKey  = regexp.MustCompile(`^runtime\.([a-z0-9-]+)\.maximal-models$`)
	modelAliasKey     = regexp.MustCompile(`^runtime\.([a-z0-9-]+)\.model-alias\.(.*)$`)
	modelAliasEnvName = regexp.MustCompile(`^METASYSTEM_RUNTIME_[A-Z0-9_]+_MODEL_ALIAS_[A-Z0-9_]+$`)
	roleComponentKey  = regexp.MustCompile(`^role\.([a-z0-9-]+)\.(?:runtime|model\.[a-z0-9-]+)$`)
	modeComponentKey  = regexp.MustCompile(`^mode\.([a-z0-9-]+)\.role\.([a-z0-9-]+)\.(?:runtime|model\.[a-z0-9-]+)$`)
)

// roleLabel names a role in an error message, matching the plain and
// mode-scoped phrasings the validator emits.
func roleLabel(role, mode string, isDefault bool) string {
	name := "role " + role
	if isDefault {
		name = "role.default"
	}
	if mode != "" {
		return fmt.Sprintf("mode %s %s", pyRepr(mode), name)
	}
	return name
}

func contiguousFromOne(sorted []int) bool {
	for i, n := range sorted {
		if n != i+1 {
			return false
		}
	}
	return true
}

func sortedKeysOf(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// resolvePath makes path absolute and follows symlinks. Parts that do not exist
// yet cannot be resolved, so it follows symlinks on the deepest existing
// ancestor and re-attaches the remaining tail lexically — the same way the
// evidence-root check must compare a not-yet-created directory against the repo.
func resolvePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	remainder := ""
	current := abs
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			if remainder == "" {
				return resolved
			}
			return filepath.Join(resolved, remainder)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return abs
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

// withinRepo reports whether path is the repository root or lives beneath it.
func withinRepo(path, repo string) bool {
	rel, err := filepath.Rel(repo, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// pyRepr renders a string the way the reference tooling quoted it in messages:
// single quotes, unless the text holds a single quote and no double quote, with
// backslashes, the quote, and control characters escaped.
func pyRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			b.WriteString(fmt.Sprintf(`\x%02x`, r))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
