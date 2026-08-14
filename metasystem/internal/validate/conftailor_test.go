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
role.default.runtime=claude
role.implementer.runtime=claude
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
`
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
		"role.default.runtime=fake\n"
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
	want := "metasystem.runtimes=devin,codex\nevidence.root=artifacts\nrole.default.runtime=codex\n"
	if got != want {
		t.Fatalf("tailored conf mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
