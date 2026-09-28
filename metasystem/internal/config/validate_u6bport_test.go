package config

import (
	"strings"
	"testing"
)

// dispatch-fixtures agent_config validate cases (lines 1328-1339) that no
// other validation test names: a mode-scoped role runtime and a role model
// keyed by a runtime outside the roster, and a roster naming only an
// unsupported runtime.
func TestValidateRefusesRuntimesOutsideTheRosterU6bPort(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, conf, expect string
	}{
		{"invalid mode runtime", validConf + "mode.refactor.role.implementer.runtime=ghost\n",
			"mode.refactor.role.implementer.runtime names runtime 'ghost' outside metasystem.runtimes"},
		{"invalid model runtime", validConf + "role.default.model.ghost=ghost-model\n",
			"role.default.model.ghost names runtime 'ghost' outside metasystem.runtimes"},
		{"unsupported runtime alone", strings.Replace(validConf, "metasystem.runtimes=claude,codex,fake", "metasystem.runtimes=ghost", 1),
			"unsupported runtime 'ghost'"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if problems := validateRepo(t, c.conf); !hasProblem(problems, c.expect) {
				t.Fatalf("expected a problem containing %q; got %v", c.expect, problems)
			}
		})
	}
}
