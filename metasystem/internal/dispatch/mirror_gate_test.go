package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

// dispatch-supervise-5: the payload directory is delegate-writable, so the
// scripted-failure hook must be dead outside a fake-runtime checkout.
func TestMirrorFaultHookGatedToFakeRuntime(t *testing.T) {
	for _, tc := range []struct {
		conf    string
		honored bool
	}{
		{"metasystem.runtimes=fake\n", true},
		{"metasystem.runtimes=codex\n", false},
		{"", false},
	} {
		root := t.TempDir()
		if tc.conf != "" {
			os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(tc.conf), 0o644)
		}
		if got := runtimesConfigured(root) == "fake"; got != tc.honored {
			t.Fatalf("conf %q: hook honored=%v, want %v", tc.conf, got, tc.honored)
		}
	}
}
