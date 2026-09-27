package main

import (
	"errors"
	"strings"
	"testing"
)

// The production command preflight names every missing command with its
// Debian-family package and refuses; a complete host passes silently.
func TestAuditProductionCommandsNamesEveryMissingCommandWithItsPackage(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	complete := func(string) (string, error) { return "/usr/bin/x", nil }
	if code := auditProductionCommands(nil, complete, &out); code != 0 || out.Len() != 0 {
		t.Fatalf("complete host code=%d output=%q", code, out.String())
	}
	lacking := func(name string) (string, error) {
		if name == "pgrep" || name == "awk" {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}
	out.Reset()
	if code := auditProductionCommands(nil, lacking, &out); code != 1 {
		t.Fatalf("lacking host code=%d", code)
	}
	want := "command preflight: this host is missing production commands:\n  pgrep (package: procps)\n  awk (package: mawk or gawk)\n"
	if out.String() != want {
		t.Fatalf("output=%q want %q", out.String(), want)
	}
	out.Reset()
	if code := auditProductionCommands([]string{"extra"}, complete, &out); code != 2 {
		t.Fatalf("usage code=%d", code)
	}
}
