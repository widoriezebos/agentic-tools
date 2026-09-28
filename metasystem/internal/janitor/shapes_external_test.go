package janitor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// TestShapesAtTakeAnExternalRuntimesClaimShapes: an external runtime's
// supervisors and its CLI's declared claim-bound invocation shape join the
// kill proof's shapes (VOA-29); a correctly tagged CLI matches, and a wrong
// tag, a tag in another argument, or an unrelated process does not.
func TestShapesAtTakeAnExternalRuntimesClaimShapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	describe := `{"schemaVersion":1,"name":"newagent","capabilities":{"host":true},"match":["^([^[:space:]]*/)?newagent([[:space:]]|$)"],"positive":"newagent -p task","lookalike":"newagent-helper serve","invocations":[{"includes":["newagent","-p"],"tagFlag":"--tag"}]}`
	path := filepath.Join(root, "adapters", "newagent")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte("#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '%s\\n' '"+describe+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("adapters.newagent.use=external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shapes := ShapesAt(root)
	for _, test := range []struct {
		argv []string
		tag  string
		want bool
	}{
		{[]string{"/opt/bin/newagent", "-p", "--tag", "job-tag"}, "job-tag", true},
		{[]string{"/opt/bin/newagent", "-p", "--tag", "other-tag"}, "job-tag", false},
		{[]string{"/opt/bin/newagent", "-p", "--note", "job-tag"}, "job-tag", false},
		{[]string{"/bin/metasystem", "delegate-supervisor", "newagent", "dispatch", "--instance-tag", "job-tag"}, "job-tag", true},
		{[]string{"/bin/metasystem", "delegate-supervisor", "newagent", "start-turn", "--instance-tag", "job-tag"}, "job-tag", true},
		{[]string{"/bin/sleep", "--tag", "job-tag"}, "job-tag", false},
	} {
		if _, got := MatchShape(shapes, test.argv, test.tag); got != test.want {
			t.Errorf("MatchShape(%q, %q) = %v, want %v", test.argv, test.tag, got, test.want)
		}
	}
	// The kill proof over those shapes: the orphaned, correctly tagged CLI
	// is killable; the same pid reused by another process, or a wrong tag,
	// is left alone.
	recorded := &registry.ProcessRef{Pid: 41, PidStartedAt: 100}
	cli := []string{"/opt/bin/newagent", "-p", "--tag", "job-tag"}
	if name, ok := Killable(observed(41, 100, cli...), recorded, shapes, []string{"job-tag"}); !ok || name != "adapter-cli-newagent" {
		t.Fatalf("the tagged orphan = %q, %v", name, ok)
	}
	if _, ok := Killable(observed(41, 200, cli...), recorded, shapes, []string{"job-tag"}); ok {
		t.Fatal("a reused pid was killable")
	}
	if _, ok := Killable(observed(41, 100, cli...), recorded, shapes, []string{"other-tag"}); ok {
		t.Fatal("a wrongly tagged CLI was killable")
	}
	if _, got := MatchShape(DefaultShapes(), []string{"/opt/bin/newagent", "-p", "--tag", "job-tag"}, "job-tag"); got {
		t.Fatal("the installation-free shapes know an external runtime")
	}
}
