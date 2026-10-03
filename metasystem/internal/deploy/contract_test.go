package deploy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A project without deploy.json, or without the file deploy.contract names,
// declares no deploy; one with it has its contract read strictly.
func TestDeployContractIsDeclaredByItsFile(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(installation, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(installation, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metasystem.conf", "")
	if _, _, err := LoadContract(installation); !errors.Is(err, ErrNoContract) {
		t.Fatalf("no deploy.json = %v; want ErrNoContract", err)
	}
	write("deploy.json", `{"schema":1,"adapter":{"argv":["./deploy.sh"],"cwd":"ops"},"tools":[{"id":"mvn","executable":"mvn"}]}`)
	contract, path, err := LoadContract(installation)
	if err != nil || contract.Adapter.CWD != "ops" || path != filepath.Join(installation, "deploy.json") {
		t.Fatalf("contract = %+v %s %v", contract, path, err)
	}
	write("metasystem.conf", "deploy.contract=ops/deploy.json\n")
	if _, err := Declared(installation, installation); !errors.Is(err, ErrNoContract) {
		t.Fatalf("deploy.contract naming a missing file = %v; want ErrNoContract, whatever deploy.json holds", err)
	}
	write("ops/deploy.json", `{"schema":1,"adapter":{"argv":["x"],"cwd":"../out"},"retries":3}`)
	if _, _, err := LoadContract(installation); err == nil || !strings.Contains(err.Error(), "retries") {
		t.Fatalf("an unknown field = %v; want it named", err)
	}
	write("ops/deploy.json", `{"schema":2,"adapter":{"argv":[],"cwd":"../out"}}`)
	_, _, err = LoadContract(installation)
	for _, fault := range []string{"schema must be 1", "adapter.argv", "adapter.cwd"} {
		if err == nil || !strings.Contains(err.Error(), fault) {
			t.Fatalf("faults = %v; want %q named", err, fault)
		}
	}
}

// Every spelling of one repository's origin names one project, and the
// name is safe as a directory name; another repository has another.
func TestProjectKeyNamesTheRepositoryNotTheCheckout(t *testing.T) {
	t.Parallel()
	one, err := ProjectKey("https://github.com/widoriezebos/agentic-tools.git")
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{"git@github.com:widoriezebos/agentic-tools.git", "ssh://git@GitHub.com/widoriezebos/agentic-tools", "https://github.com/widoriezebos/agentic-tools/"} {
		if key, err := ProjectKey(spelling); err != nil || key != one {
			t.Fatalf("%s = %s %v; want %s", spelling, key, err, one)
		}
	}
	if other, _ := ProjectKey("https://github.com/widoriezebos/other.git"); other == one {
		t.Fatal("two repositories share a project key")
	}
	if len(one) != 16 || strings.Trim(one, "0123456789abcdef") != "" {
		t.Fatalf("key %q is not short hexadecimal", one)
	}
	if _, err := ProjectKey("  "); err == nil {
		t.Fatal("a checkout without origin has a project key")
	}
}

// A port other than the scheme's own names another server: two URLs that
// differ only in it are two projects, while a trailing .git or the scheme's
// own port is spelling.
func TestProjectKeyKeepsANonDefaultPort(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		one, other string
		same       bool
	}{
		{"ssh://git@host:2222/team/app.git", "ssh://git@host:3333/team/app.git", false},
		{"ssh://git@host:2222/team/app.git", "ssh://git@Host:2222/team/app", true},
		{"ssh://git@host:22/team/app.git", "git@host:team/app.git", true},
		{"https://host:8443/team/app.git", "https://host/team/app.git", false},
	} {
		one, err := ProjectKey(row.one)
		other, otherErr := ProjectKey(row.other)
		if err != nil || otherErr != nil || (one == other) != row.same {
			t.Errorf("%s = %s %v, %s = %s %v; want the same key %v", row.one, one, err, row.other, other, otherErr, row.same)
		}
	}
}
