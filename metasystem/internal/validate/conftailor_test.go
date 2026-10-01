package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const tailorFixture = `# adopted configuration
metasystem.runtimes=<runtimes>
role.default.runtime=<runtime>
role.implementer.runtime=codex
role.verifier.runtime=main
mode.cheap.role.implementer.runtime=devin
mode.cheap.role.verifier.runtime=main
role.code-critic.model.<runtime>=<model>
role.implementer.model.codex=frontier-code
role.implementer.model.claude=frontier-general
model.tier.1=claude:big,codex:bigger,local-model
model.tier.2=<members>
evidence.root=artifacts
`

func TestTailorConfSelectsOneRuntime(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	writeFile(t, conf, tailorFixture)
	if err := TailorConf(conf, []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, conf)
	want := `# adopted configuration
metasystem.runtimes=claude
role.default.runtime=auto
role.implementer.runtime=auto
role.verifier.runtime=main
mode.cheap.role.verifier.runtime=main
role.code-critic.model.claude=<model>
role.implementer.model.claude=frontier-general
model.tier.1=claude:big,local-model
model.tier.2=<members>
evidence.root=artifacts
`
	if got != want {
		t.Fatalf("tailored conf mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestTailorConfNoneDropsEveryBinding(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	writeFile(t, conf, tailorFixture)
	if err := TailorConf(conf, []string{"none"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, conf)
	if !strings.Contains(got, "metasystem.runtimes=\n") {
		t.Fatalf("runtimes line must be emptied, got:\n%s", got)
	}
	if strings.Contains(got, "role.") || strings.Contains(got, "mode.") {
		t.Fatalf("no role or mode binding may survive a none selection, got:\n%s", got)
	}
	if !strings.Contains(got, "model.tier.1=\n") || !strings.Contains(got, "model.tier.2=\n") {
		t.Fatalf("tier values must be emptied, got:\n%s", got)
	}
	if !strings.Contains(got, "evidence.root=artifacts\n") {
		t.Fatalf("unrelated keys must survive, got:\n%s", got)
	}
}

func TestTailorConfFakeRuntimeCollapsesModelBindings(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	writeFile(t, conf, tailorFixture)
	if err := TailorConf(conf, []string{"fake"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, conf)
	want := `# adopted configuration
metasystem.runtimes=fake
role.default.runtime=fake
role.implementer.runtime=fake
role.verifier.runtime=main
mode.cheap.role.verifier.runtime=main
role.code-critic.model.fake=fake-model
role.implementer.model.fake=fake-model
model.tier.1=local-model
model.tier.2=<members>
evidence.root=artifacts
launch.landing.runtime=fake
launch.landing.model=fake-model
`
	// The compiled roles and launch lanes are auto, which resolves to the
	// only selected runtime, so nothing is rebound for them; the landing
	// lane alone is compiled to claude (its tool gate is a Claude hook), so
	// it is rebound to fake with fake's model.
	if got != want {
		t.Fatalf("tailored conf mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestTailorConfFakeRuntimeKeepsExplicitFakeModel(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	writeFile(t, conf, "metasystem.runtimes=<runtimes>\n"+
		"role.implementer.model.fake=pinned-model\n"+
		"role.implementer.model.codex=frontier-code\n")
	if err := TailorConf(conf, []string{"fake"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, conf)
	want := "metasystem.runtimes=fake\n" +
		"role.implementer.model.fake=pinned-model\n" +
		"role.default.runtime=fake\n" +
		"launch.landing.runtime=fake\n" +
		"launch.landing.model=fake-model\n"
	if got != want {
		t.Fatalf("an explicit fake model binding must win over synthesis:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestSetConfKeysReplacesAppendsAndDedupes(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	writeFile(t, conf, "# heading\n"+
		"watch.interval-sec=60\n"+
		"evidence.root=artifacts\n"+
		"watch.interval-sec=120\n")
	err := SetConfKeys(conf, []ConfSetting{
		{Key: "watch.interval-sec", Value: "1"},
		{Key: "census.log-max-bytes", Value: "350"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := readFile(t, conf)
	want := "# heading\n" +
		"watch.interval-sec=1\n" +
		"evidence.root=artifacts\n" +
		"census.log-max-bytes=350\n"
	if got != want {
		t.Fatalf("set mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestTailorConfInsertsMissingDurableKeys(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	writeFile(t, conf, "evidence.root=artifacts\n")
	if err := TailorConf(conf, []string{"devin", "codex"}); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, conf)
	// The selection is written in the canonical preference order whatever
	// order it was asked in. Every compiled role and lane but landing is auto
	// and every selected runtime has compiled models; the landing lane's
	// compiled claude is not selected, so it becomes auto.
	want := "evidence.root=artifacts\nmetasystem.runtimes=codex,devin\nlaunch.landing.runtime=auto\n"
	if got != want {
		t.Fatalf("tailored conf mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

// Launch lanes name their runtime and model (R-123). A lane's model row the
// file does not hold is the compiled one, rendered after the file's own lines.
// A lane on a selected runtime keeps its pair byte-for-byte; a lane on an
// unselected runtime becomes auto and loses its runtime-independent model,
// so each runtime's own model applies; a fake-only selection rebinds it to
// fake and its synthesized model; none drops every lane binding. Unrelated
// lane settings stay.
func TestTailorConfRebindsLaneRuntimesAndModels(t *testing.T) {
	t.Parallel()
	const lanes = `launch.build.runtime=claude
launch.build.model=claude-opus-5-5
launch.build.effort=xhigh
launch.critique.model=gpt-6-sol
launch.critique.runtime=codex
launch.read.runtime=devin
launch.read.window.tokens=0
`
	for _, tc := range []struct {
		name, extra string
		runtimes    []string
		want        string
	}{
		{"selected pairs unchanged", "", []string{"claude", "codex", "devin"},
			lanes},
		{"rebound to auto drops the lane's own model", "role.default.model.claude=configured-claude\n", []string{"claude"},
			"launch.build.runtime=claude\nlaunch.build.model=claude-opus-5-5\nlaunch.build.effort=xhigh\n" +
				"launch.critique.runtime=auto\nlaunch.read.runtime=auto\nlaunch.read.window.tokens=0\n"},
		{"a lane on a selected runtime keeps its model", "role.default.model.codex=<model>\n", []string{"claude", "codex"},
			"launch.build.runtime=claude\nlaunch.build.model=claude-opus-5-5\nlaunch.build.effort=xhigh\n" +
				"launch.critique.model=gpt-6-sol\nlaunch.critique.runtime=codex\n" +
				"launch.read.runtime=auto\nlaunch.read.window.tokens=0\n"},
		{"fake takes its synthesized model", "", []string{"fake"},
			"launch.build.runtime=fake\nlaunch.build.model=fake-model\nlaunch.build.effort=xhigh\n" +
				"launch.critique.model=fake-model\nlaunch.critique.runtime=fake\n" +
				"launch.read.runtime=fake\nlaunch.read.window.tokens=0\n" +
				"launch.landing.runtime=fake\nlaunch.landing.model=fake-model\nlaunch.read.model=fake-model\n"},
		{"none drops lane runtime and model", "", []string{"none"},
			"launch.build.effort=xhigh\nlaunch.read.window.tokens=0\n"},
	} {
		conf := filepath.Join(t.TempDir(), "metasystem.conf")
		// The default-model row sits after the lanes: rebinding must not
		// depend on having read it first.
		writeFile(t, conf, lanes+tc.extra)
		if err := TailorConf(conf, tc.runtimes); err != nil {
			t.Fatal(err)
		}
		var got strings.Builder
		for _, line := range strings.Split(readFile(t, conf), "\n") {
			if strings.HasPrefix(line, "launch.") {
				got.WriteString(line + "\n")
			}
		}
		if got.String() != tc.want {
			t.Fatalf("%s (%v): lane rows\n%s\nwant\n%s", tc.name, tc.runtimes, got.String(), tc.want)
		}
	}
}
