package validate

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
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
	modelKeyRe     = regexp.MustCompile(`^(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
	tierKeyRe      = regexp.MustCompile(`^model\.tier\.[1-9][0-9]*$`)
)

// tailorKnownRuntimes lists every runtime a model-tier member may name
// as its prefix; a member with any other prefix is a bare model name,
// not a runtime binding, and tier filtering must leave it alone.
func tailorKnownRuntime(name string) bool { return runtimes.Supported(name) }

// TailorConf rewrites a metasystem.conf in place for the selected
// runtime set. The selected list becomes the durable
// metasystem.runtimes value; unselected runtimes lose their role, mode,
// and launch-lane runtime bindings, their per-runtime model keys, and
// their model-tier members; the default runtime is set to the strongest
// selected runtime. A lane rebound to the default runtime gets that
// runtime's model: its synthesized model, else its concrete
// role.default.model.<runtime>, else an explicit empty value that launch
// settings refuse, never the unselected runtime's model or the launch
// package's shipped default. Selecting none ("none") drops every role,
// mode, and launch-lane runtime and model binding and empties the tiers, so no unselected runtime's model
// placeholder or mode override leaks into an adopted repository. The
// rewrite is atomic: a temporary sibling is written, then renamed over
// the original.
func TailorConf(confPath string, requested []string) error {
	selected := requested
	if len(requested) == 1 && requested[0] == "none" {
		selected = nil
	}
	selectedSet := map[string]bool{}
	for _, runtime := range selected {
		selectedSet[runtime] = true
	}
	// The fake runtime never outranks a real one: it becomes the default
	// only when it is the sole selection, which is the fixture-harness
	// shape (validation suites tailor a copied conf to the fake adapter).
	defaultRuntime := runtimes.DefaultFor(selectedSet)

	data, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

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
		if match := laneRuntimeKey.FindStringSubmatch(key); match != nil && value != "main" && !selectedSet[value] {
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

		// A template-project promise never leaks into an adoption: the
		// template develops beside companion suites (the benchmark kit)
		// and declares them in validate.extra-suites; an adopted target
		// has no such sibling, and a leaked declaration would refuse its
		// every validation run (the declared-suite promise fails closed
		// by design).
		if key == "validate.extra-suites" {
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
			if value != "main" && !selectedSet[value] {
				kept = defaultRuntime
			}
			out = append(out, key+"="+kept)
			// A rebound lane with no model row would inherit the shipped
			// default model, which belongs to whichever runtime it names.
			if match := laneRuntimeKey.FindStringSubmatch(key); match != nil &&
				reboundLanes[match[1]] && !laneHasModel[match[1]] {
				out = append(out, "launch."+match[1]+".model="+laneModel)
			}
			continue
		}
		if match := laneModelKey.FindStringSubmatch(key); match != nil && reboundLanes[match[1]] {
			out = append(out, key+"="+laneModel)
			continue
		}
		if modeRuntimeKey.MatchString(key) {
			if value == "main" || selectedSet[value] {
				out = append(out, raw)
			}
			continue
		}

		// The template ships one placeholder model binding keyed by the
		// literal token <runtime>; it becomes the default runtime's key.
		if key == "role.code-critic.model.<runtime>" {
			if defaultRuntime == "fake" {
				out = append(out, "role.code-critic.model.fake=fake-model")
			} else {
				out = append(out, "role.code-critic.model."+defaultRuntime+"="+value)
			}
			continue
		}

		if match := modelKeyRe.FindStringSubmatch(key); match != nil && !selectedSet[match[1]] {
			if defaultRuntime == "fake" {
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

	// Through the durable-write owner: a bare write-and-rename with no
	// sync could leave a torn conf after a crash. Empty anchor
	// until adoption's caller carries the two-outcome contract.
	_, err = atomicfile.WriteText(confPath, strings.Join(out, "\n")+"\n", "")
	return err
}
