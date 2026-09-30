package validate

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"os"
	"regexp"
	"strings"
)

var (
	roleRuntimeKey = regexp.MustCompile(`^role\.[a-z0-9-]+\.runtime$`)
	modeRuntimeKey = regexp.MustCompile(`^mode\.[a-z0-9-]+\.role\.[a-z0-9-]+\.runtime$`)
	// A launch lane (R-123) names the agent it runs on, as a role does.
	laneRuntimeKey = regexp.MustCompile(`^launch\.([a-z0-9-]+)\.runtime$`)
	laneModelKey   = regexp.MustCompile(`^launch\.([a-z0-9-]+)\.model$`)
	modelKeyRe     = regexp.MustCompile(`^(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+|launch\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
	tierKeyRe      = regexp.MustCompile(`^model\.tier\.[1-9][0-9]*$`)
)

// tailorKnownRuntimes lists every runtime a model-tier member may name
// as its prefix; a member with any other prefix is a bare model name,
// not a runtime binding, and tier filtering must leave it alone.
func tailorKnownRuntime(name string) bool { return runtimes.Supported(name) }

// TailorConf rewrites a metasystem.conf in place for the selected
// runtime set. The selected list, in the registry's canonical preference
// order whatever order it was asked in, becomes the durable
// metasystem.runtimes value; unselected runtimes lose their role, mode,
// and launch-lane runtime bindings, their per-runtime model keys, and
// their model-tier members. A selection with any detectable runtime keeps
// every agent-picking key on auto, so the adopted repository takes the
// first of its runtimes on each host's PATH; a binding that named an
// unselected runtime becomes auto and loses its runtime-independent lane
// model, so each runtime's own model applies. A selection of synthetic
// runtimes only (fake) binds the strongest of them instead, since auto
// never detects one, and a lane rebound to it gets its synthesized model,
// else its concrete role.default.model.<runtime>, else an explicit empty
// value that launch settings refuse. Selecting none ("none") drops every role,
// mode, and launch-lane runtime and model binding and empties the tiers, so no unselected runtime's model
// placeholder or mode override leaks into an adopted repository. The
// rewrite is atomic: a temporary sibling is written, then renamed over
// the original.
func TailorConf(confPath string, requested []string) error {
	selected := runtimes.Canonical(requested)
	if len(requested) == 1 && requested[0] == "none" {
		selected = nil
	}
	selectedSet := map[string]bool{}
	detectable := false
	for _, runtime := range selected {
		selectedSet[runtime] = true
		if declaration, known := runtimes.Lookup(runtime); known && declaration.Executable != "" {
			detectable = true
		}
	}
	// The fake runtime never outranks a real one: it becomes the default
	// only when it is the sole selection, which is the fixture-harness
	// shape (validation suites tailor a copied conf to the fake adapter).
	defaultRuntime := runtimes.DefaultFor(selectedSet)
	if detectable {
		defaultRuntime = config.AutoRuntime
	}
	// stale is a runtime binding this selection cannot keep. Auto never is:
	// it resolves among the selected runtimes on every host; nor is off (a
	// seat lane that starts nothing names no runtime).
	stale := func(value string) bool {
		return value != "main" && value != config.AutoRuntime && value != config.SeatRuntimeOff && !selectedSet[value]
	}

	original, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	// The file holds overrides only; the defaults are compiled in. Tailor
	// the effective committed layer, so a compiled binding of an unselected
	// runtime is rebound or dropped exactly as a spelled-out line was, then
	// write back only what differs from the compiled defaults.
	originalKeys := map[string]bool{}
	for _, raw := range splitLines(string(original)) {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || strings.HasPrefix(stripped, "#") || !strings.Contains(raw, "=") {
			continue
		}
		originalKeys[strings.TrimSpace(strings.SplitN(raw, "=", 2)[0])] = true
	}
	data := []byte(config.EffectiveCommittedContent(string(original)))

	// Tailoring to the fake runtime collapses each role's dropped
	// per-runtime model bindings into one model.fake=fake-model line (the
	// fake adapter's fixed model name); prefixes that already bind a fake
	// model keep theirs.
	hasFakeModel := map[string]bool{}
	if defaultRuntime == "fake" {
		for _, raw := range splitLines(string(data)) {
			key := strings.TrimSpace(strings.SplitN(raw, "=", 2)[0])
			if strings.HasSuffix(key, ".model.fake") && modelKeyRe.MatchString(key) {
				hasFakeModel[strings.TrimSuffix(key, ".model.fake")] = true
			}
		}
	}

	// Lane rebinding reads the whole file first, so neither the order of a
	// lane's runtime and model rows nor where role.default.model.<runtime>
	// sits changes the result.
	reboundLanes := map[string]bool{}
	laneHasModel := map[string]bool{}
	laneModel := ""
	if declaration, known := runtimes.Lookup(defaultRuntime); known && declaration.SynthesizedModel != "" {
		laneModel = declaration.SynthesizedModel
	}
	for _, raw := range splitLines(string(data)) {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || strings.HasPrefix(stripped, "#") || !strings.Contains(raw, "=") {
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if match := laneRuntimeKey.FindStringSubmatch(key); match != nil && stale(value) {
			reboundLanes[match[1]] = true
		}
		if match := laneModelKey.FindStringSubmatch(key); match != nil {
			laneHasModel[match[1]] = true
		}
		if key == "role.default.model."+defaultRuntime && laneModel == "" &&
			!(strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")) {
			laneModel = value
		}
	}

	var out []string
	sawRuntimes := false
	sawDefault := false
	for _, raw := range splitLines(string(data)) {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || strings.HasPrefix(stripped, "#") || !strings.Contains(raw, "=") {
			out = append(out, raw)
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "metasystem.runtimes" {
			out = append(out, "metasystem.runtimes="+strings.Join(selected, ","))
			sawRuntimes = true
			continue
		}

		if len(selected) == 0 {
			switch {
			case strings.HasPrefix(key, "role.") ||
				(strings.HasPrefix(key, "mode.") && strings.Contains(key, ".role.")) ||
				laneRuntimeKey.MatchString(key) || laneModelKey.MatchString(key):
				// No runtime selected: every role, mode, and lane binding goes.
			case tierKeyRe.MatchString(key):
				out = append(out, key+"=")
			default:
				out = append(out, raw)
			}
			continue
		}

		if key == "role.default.runtime" {
			out = append(out, key+"="+defaultRuntime)
			sawDefault = true
			continue
		}
		// A lane binding is rebound, not dropped: an absent lane key falls
		// back to the launch package's shipped default, which may itself
		// name an unselected runtime.
		if roleRuntimeKey.MatchString(key) || laneRuntimeKey.MatchString(key) {
			kept := value
			if stale(value) {
				kept = defaultRuntime
			}
			out = append(out, key+"="+kept)
			// A lane rebound to a synthetic runtime with no model row would
			// inherit a model that belongs to another runtime.
			if match := laneRuntimeKey.FindStringSubmatch(key); match != nil && !detectable &&
				reboundLanes[match[1]] && !laneHasModel[match[1]] {
				out = append(out, "launch."+match[1]+".model="+laneModel)
			}
			continue
		}
		if match := laneModelKey.FindStringSubmatch(key); match != nil && reboundLanes[match[1]] {
			// On auto the lane's runtime-independent model would follow it
			// to whichever runtime a host has; each runtime's own applies.
			if !detectable {
				out = append(out, key+"="+laneModel)
			}
			continue
		}
		if modeRuntimeKey.MatchString(key) {
			if !stale(value) {
				out = append(out, raw)
			}
			continue
		}

		// The template ships one placeholder model binding keyed by the
		// literal token <runtime>; it becomes the default runtime's key.
		if key == "role.code-critic.model.<runtime>" {
			switch {
			case defaultRuntime == "fake":
				out = append(out, "role.code-critic.model.fake=fake-model")
			case detectable:
				// Under auto the preferred selected runtime takes the slot.
				out = append(out, "role.code-critic.model."+selected[0]+"="+value)
			default:
				out = append(out, "role.code-critic.model."+defaultRuntime+"="+value)
			}
			continue
		}

		if match := modelKeyRe.FindStringSubmatch(key); match != nil && !selectedSet[match[1]] {
			// A launch lane never runs on fake; only roles collapse.
			if defaultRuntime == "fake" && !strings.HasPrefix(key, "launch.") {
				prefix := strings.TrimSuffix(key, ".model."+match[1])
				if !hasFakeModel[prefix] {
					hasFakeModel[prefix] = true
					out = append(out, prefix+".model.fake=fake-model")
				}
			}
			continue
		}

		if tierKeyRe.MatchString(key) &&
			!(strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")) {
			var kept []string
			for _, member := range strings.Split(value, ",") {
				member = strings.TrimSpace(member)
				if member == "" {
					continue
				}
				runtime := ""
				if colon := strings.Index(member, ":"); colon >= 0 {
					runtime = member[:colon]
				}
				if !tailorKnownRuntime(runtime) || selectedSet[runtime] {
					kept = append(kept, member)
				}
			}
			out = append(out, key+"="+strings.Join(kept, ","))
			continue
		}

		out = append(out, raw)
	}

	if !sawRuntimes {
		out = append([]string{"metasystem.runtimes=" + strings.Join(selected, ",")}, out...)
	}
	if len(selected) > 0 && !sawDefault {
		out = append(out, "role.default.runtime="+defaultRuntime)
	}
	// Every selected non-synthesized runtime gets its default-model row
	// materialized when the source carried none: a new runtime's
	// scaffold needs no per-runtime boilerplate in
	// the shipped template — the operator fills the value. Synthesized
	// runtimes (fake) already materialize their fixed model above.
	for _, runtime := range selected {
		declaration, known := runtimes.Lookup(runtime)
		if !known || declaration.SynthesizedModel != "" {
			continue
		}
		key := "role.default.model." + runtime
		if _, compiled := config.CompiledDefault(key); compiled {
			continue
		}
		present := false
		for _, line := range out {
			if strings.HasPrefix(line, key+"=") {
				present = true
				break
			}
		}
		if !present {
			out = append(out, key+"=")
		}
	}

	// A line the file did not hold whose tailored value is its compiled
	// default adds nothing: the default already says it.
	kept := out[:0]
	for _, line := range out {
		stripped := strings.TrimSpace(line)
		if stripped != "" && !strings.HasPrefix(stripped, "#") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if compiled, ok := config.CompiledDefault(key); ok && !originalKeys[key] && compiled == value {
				continue
			}
		}
		kept = append(kept, line)
	}
	out = kept

	// Through the durable-write owner: a bare write-and-rename with no
	// sync could leave a torn conf after a crash. Empty anchor
	// until adoption's caller carries the two-outcome contract.
	_, err = atomicfile.WriteText(confPath, strings.Join(out, "\n")+"\n", "")
	return err
}
