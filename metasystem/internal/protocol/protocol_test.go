package protocol

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

// Every dispatchable role carries instructions, requirements and a return
// schema compiled into the engine; the mission host carries its preamble and
// turn schema.
func TestEmbeddedRolesAreComplete(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"behavior-judge", "code-critic", "design-critic", "implementer", "investigator", "steward-continuation", "verifier", "warden"} {
		if !Dispatchable(role) {
			t.Errorf("role %s is not dispatchable", role)
		}
		for name, read := range map[string]func(string) ([]byte, error){"instructions": RoleInstructions, "requirements": RoleRequirements, "schema": RoleSchema} {
			data, err := read(role)
			if err != nil || len(data) == 0 {
				t.Errorf("%s %s: %v", role, name, err)
			}
		}
	}
	if Dispatchable("orchestrator") {
		t.Error("orchestrator is the mission host prompt, not a dispatchable role")
	}
	if _, err := RoleInstructions("orchestrator"); err != nil {
		t.Errorf("orchestrator preamble: %v", err)
	}
	if _, err := RoleSchema("orchestrator"); err != nil {
		t.Errorf("orchestrator schema: %v", err)
	}
	for _, bad := range []string{"", "../roles/warden", "roles/warden", "nosuch"} {
		if Dispatchable(bad) {
			t.Errorf("Dispatchable(%q) = true", bad)
		}
		if _, err := RoleInstructions(bad); err == nil {
			t.Errorf("RoleInstructions(%q) accepted", bad)
		}
	}
}

func TestEmbeddedPermissionPresets(t *testing.T) {
	t.Parallel()
	for _, preset := range []string{"critic", "none", "workspace"} {
		data, err := Permissions(preset)
		if err != nil {
			t.Fatalf("preset %s: %v", preset, err)
		}
		var envelope map[string]any
		if err := json.Unmarshal(data, &envelope); err != nil || envelope["writeRoots"] == nil {
			t.Fatalf("preset %s is not an envelope: %v", preset, err)
		}
	}
	for _, name := range []string{"custom", "../role-packets"} {
		if _, err := Permissions(name); err == nil {
			t.Errorf("Permissions accepted %q, which is not a shipped preset", name)
		}
	}
}

func TestEmbeddedTemplates(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"brief.md", "follow-up.md", "design-brief.md", "design-common.md", "review-brief.md", "host-turn-instruction.md"} {
		if data, err := Template(name); err != nil || len(data) == 0 {
			t.Errorf("template %s: %v", name, err)
		}
	}
	if _, err := Template("nosuch.md"); err == nil {
		t.Error("unknown template accepted")
	}
}

// A role-packet source is either an engine protocol reference, read from the
// compiled-in bytes, or an installation path; every protocol reference the
// table names resolves.
func TestRolePacketSourcesResolve(t *testing.T) {
	t.Parallel()
	var table struct {
		Roles map[string]struct {
			Sources []struct{ Slot, Path string } `json:"sources"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(RolePackets(), &table); err != nil {
		t.Fatal(err)
	}
	protocolSources := 0
	for role, recipe := range table.Roles {
		for _, source := range recipe.Sources {
			if strings.HasPrefix(source.Path, "scripts/") {
				t.Errorf("role %s names a scripts/ source %s", role, source.Path)
			}
			data, ok, err := Source(source.Path)
			if !IsReference(source.Path) {
				if ok {
					t.Errorf("installation path %s resolved as a protocol reference", source.Path)
				}
				continue
			}
			protocolSources++
			if !ok || err != nil || len(data) == 0 {
				t.Errorf("role %s source %s: ok=%v err=%v", role, source.Path, ok, err)
			}
		}
	}
	if protocolSources == 0 {
		t.Fatal("the role packet table names no protocol source")
	}
	if _, ok, err := Source("protocol:roles/nosuch.md"); !ok || err == nil {
		t.Error("an unknown protocol reference must be recognized and refused")
	}
	if _, ok, err := Source("protocol:../go.mod"); !ok || err == nil {
		t.Error("an escaping protocol reference must be refused")
	}
}

func TestFilesIsTheWholeProtocol(t *testing.T) {
	t.Parallel()
	count := 0
	if err := fs.WalkDir(Files(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			count++
			if strings.HasSuffix(path, ".go") {
				t.Errorf("Go source %s embedded", path)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count < 30 {
		t.Fatalf("embedded protocol holds %d files", count)
	}
}
