package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateAdmitsANamedExternalRuntime: configuration validation takes
// the runtime universe from the registry's trust rules (design 3.5,
// VOA-30): a runtime the engine does not ship is valid once the
// configuration names it and its executable is safe; an unsafe or unnamed
// one is refused with the fix. Validation never executes the adapter.
func TestValidateAdmitsANamedExternalRuntime(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mode   os.FileMode
		named  bool
		expect string
	}{
		{name: "named and safe", mode: 0o755, named: true},
		{name: "group-writable", mode: 0o775, named: true, expect: "chmod go-w"},
		{name: "unnamed", mode: 0o755, named: false, expect: "add adapters.newagent.use=external"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			for _, dir := range []string{"development", "adapters"} {
				if err := os.MkdirAll(filepath.Join(repo, dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			putFile(t, filepath.Join(repo, "development", "metasystem-design.md"), "x\n")
			putFile(t, filepath.Join(repo, "testing.json"), minimalTestingContract)
			marker := filepath.Join(repo, "adapter-ran")
			adapter := filepath.Join(repo, "adapters", "newagent")
			putFile(t, adapter, "#!/bin/sh\n: >'"+marker+"'\n")
			if err := os.Chmod(adapter, test.mode); err != nil {
				t.Fatal(err)
			}
			// Required proof commands belong to the repository configuration.
			conf := "proof.full=true\nproof.cheap=true\nmetasystem.runtimes=fake,newagent\ntesting.contract=testing.json\nevidence.root=" + t.TempDir() +
				"\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\n"
			if test.named {
				conf += "adapters.newagent.use=external\n"
			}
			putFile(t, filepath.Join(repo, "metasystem.conf"), conf)
			_, problems, err := Validate(filepath.Join(repo, "metasystem.conf"), repo)
			if err != nil {
				t.Fatal(err)
			}
			if test.expect == "" && len(problems) != 0 {
				t.Fatalf("problems = %v", problems)
			}
			if test.expect != "" && !hasProblem(problems, "names unsupported runtime 'newagent'") || test.expect != "" && !strings.Contains(strings.Join(problems, "\n"), test.expect) {
				t.Fatalf("problems = %v, want the refusal naming %q", problems, test.expect)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Fatal("validation executed the adapter")
			}
		})
	}
}
