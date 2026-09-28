package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestHostRuntimeAdmissionUsesTheRegistry: the mission runner admits a host
// turn on a built-in host runtime and on an external runtime whose describe
// declares the host role (VOA-25), and refuses one that does not, or a
// refused adapter with its fix.
func TestHostRuntimeAdmissionUsesTheRegistry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conf := ""
	for name, host := range map[string]bool{"newhost": true, "nohost": false, "loose": true} {
		describe := `{"schemaVersion":1,"name":"` + name + `","capabilities":{"host":` + map[bool]string{true: "true", false: "false"}[host] +
			`},"match":["^([^[:space:]]*/)?` + name + `([[:space:]]|$)"],"positive":"` + name + ` run","lookalike":"` + name + `-helper"}`
		path := filepath.Join(root, "adapters", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte("#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '%s\\n' '"+describe+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		conf += "adapters." + name + ".use=external\n"
	}
	if err := os.Chmod(filepath.Join(root, "adapters", "loose"), 0o775); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	for runtime, want := range map[string]string{
		"claude":  "",
		"newhost": "",
		"nohost":  "does not declare the host capability",
		"loose":   "chmod go-w",
		"ghost":   "no such runtime",
	} {
		got := hostRuntimeUnavailable(root, runtime)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("hostRuntimeUnavailable(%s) = %q, want %q", runtime, got, want)
		}
	}
}
