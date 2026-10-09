package adopt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyModuleSnapshotSurvivesSourceChanges(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "snapshot")
	path := filepath.Join(source, "fixture.go")
	const original = "package fixture\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyModule(source, destination); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "fixture.go"))
	if err != nil || string(data) != original {
		t.Fatalf("source edit changed the snapshot: %q, %v", data, err)
	}
}
