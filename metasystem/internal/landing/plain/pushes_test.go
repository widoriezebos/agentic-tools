package plain

import (
	"os"
	"testing"
)

func TestPushesPreservesDamageCount(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pushesPath(install), []byte("{\"commit\":\"a\",\"old\":\"base\"}\n{\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pushes, damaged, err := Pushes(install)
	if err != nil || damaged != 1 || len(pushes) != 1 || pushes[0].Commit != "a" || pushes[0].Old != "base" {
		t.Fatalf("pushes = %+v, damaged %d, error %v", pushes, damaged, err)
	}
}
