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
	modelKeyRe     = regexp.MustCompile(`^(?:role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
	tierKeyRe      = regexp.MustCompile(`^model\.tier\.[1-9][0-9]*$`)
)

// tailorKnownRuntimes lists every runtime a model-tier member may name
// as its prefix; a member with any other prefix is a bare model name,
// not a runtime binding, and tier filtering must leave it alone.
func tailorKnownRuntime(name string) bool { return runtimes.Supported(name) }

// TailorConf rewrites a metasystem.conf in place for the selected
// runtime set. The selected list becomes the durable
// metasystem.runtimes value; unselected runtimes lose their role and
// mode runtime bindings, their per-runtime model keys, and their
// model-tier members; the default runtime is set to the strongest
// selected runtime. Selecting none ("none") drops every role and mode
// binding and empties the tiers, so no unselected runtime's model
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
				(strings.HasPrefix(key, "mode.") && strings.Contains(key, ".role.")):
				// No runtime selected: every role and mode binding goes.
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
		if roleRuntimeKey.MatchString(key) {
			kept := value
			if value != "main" && !selectedSet[value] {
				kept = defaultRuntime
			}
			out = append(out, key+"="+kept)
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

	// Through the durable-write owner (go-production-grade B5): the old
	// path here was WriteFile plus rename with no sync at all. Empty anchor
	// until adoption's caller carries the two-outcome contract.
	_, err = atomicfile.WriteText(confPath, strings.Join(out, "\n")+"\n", "")
	return err
}
