package external

import (
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const describeScript = `#!/bin/sh
case "$1" in
  describe) printf '{"schemaVersion":1,"match":["^([^[:space:]]*/)?newagent([[:space:]]|$)"],"exclude":["newagent-helper"]}\n' ;;
  *) exit 64 ;;
esac
`

func install(t *testing.T, root, name, body string, mode os.FileMode, use bool) {
	t.Helper()
	path := filepath.Join(root, Dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	if use {
		conf := filepath.Join(root, "metasystem.conf")
		existing, _ := os.ReadFile(conf)
		if err := os.WriteFile(conf, append(existing, []byte(UseKey(name)+"="+UseValue+"\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestDiscoveryTrustRules: an adapter runs only when named in the
// configuration and safe on disk; a built-in's name is an override; every
// other file is reported, never executed.
func TestDiscoveryTrustRules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	install(t, root, "newagent", describeScript, 0o755, true)
	install(t, root, "unnamed", describeScript, 0o755, false)
	install(t, root, "loose", describeScript, 0o777, true)
	install(t, root, "fake", describeScript, 0o755, true)
	install(t, root, "Bad_Name", describeScript, 0o755, true)
	adapters, refusals, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range adapters {
		names = append(names, a.Name+map[bool]string{true: "(override)", false: ""}[a.Overrides])
	}
	if strings.Join(names, " ") != "fake(override) newagent" {
		t.Fatalf("adapters = %v", names)
	}
	reasons := map[string]string{}
	for _, r := range refusals {
		reasons[r.Name] = r.Reason
	}
	if !strings.Contains(reasons["unnamed"], "adapters.unnamed.use=external") ||
		!strings.Contains(reasons["loose"], "writable") || !strings.Contains(reasons["Bad_Name"], "grammar") {
		t.Fatalf("refusals = %v", reasons)
	}
	if _, _, err := Lookup(root, "unnamed"); err == nil {
		t.Fatal("an unnamed adapter resolved")
	}
}

// TestSignatureFromDescribe: the recognizers read an external runtime's
// classification signatures from describe; exit 64 is the delegation.
func TestSignatureFromDescribe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	install(t, root, "newagent", describeScript, 0o755, true)
	adapter, found, err := Lookup(root, "newagent")
	if err != nil || !found {
		t.Fatalf("lookup = %v %v", found, err)
	}
	text, err := adapter.SignatureText()
	if err != nil || text != "match ^([^[:space:]]*/)?newagent([[:space:]]|$)\nexclude newagent-helper\n" {
		t.Fatalf("signature = %q %v", text, err)
	}
	if _, err := adapter.Call("prepare", nil, nil); !errors.Is(err, ErrDelegated) {
		t.Fatalf("exit 64 = %v, want ErrDelegated", err)
	}
}
