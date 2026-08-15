package config

import (
	"fmt"
	runtimereg "github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Validation of the whole metasystem.conf domain: the runtime roster, capability
// floors, model tiers, role/model resolution, and the evidence root, checked
// against the adopting repository at repoRoot. It returns every problem found
// (each rendered without a prefix) rather than stopping at the first, plus
// tiersAbsent to signal that no model tier is configured — a valid state the
// caller notes because dispatch overrides then always escalate. A non-nil err
// is a hard failure that prevented validation (an unreadable committed file).
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

	// The .local override contributes only capability floors to validation; its
	// other keys are the developer's own and not template invariants.
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
		"census.log-max-bytes",
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

var (
	modeScopedKey     = regexp.MustCompile(`^mode\.[a-z0-9-]+\.role\.[a-z0-9-]+\.(?:runtime|model\.[a-z0-9-]+)$`)
	positiveInteger   = regexp.MustCompile(`^[1-9][0-9]*$`)
	roleRuntimeSuffix = regexp.MustCompile(`(?:^|\.)role\.[a-z0-9-]+\.runtime$`)
	tierKeyPattern    = regexp.MustCompile(`^model\.tier\.([1-9][0-9]*)$`)
	plainModelKey     = regexp.MustCompile(`^role\.([a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
	modeModelKey      = regexp.MustCompile(`^mode\.([a-z0-9-]+)\.role\.([a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
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
