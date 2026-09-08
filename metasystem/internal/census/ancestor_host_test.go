package census

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllHostSignaturesIgnoreExecutionRoster(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, runtime := range []string{"codex", "devin", "claude"} {
		body := "#!/usr/bin/env bash\n[[ ${1:-} == signature ]] || exit 2\nprintf 'match ^" + runtime + "([[:space:]]|$)\\n'\n"
		if err := os.WriteFile(filepath.Join(dir, runtime+".sh"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configured, err := signaturesFor(root, "")
	if err != nil || len(configured) != 1 {
		t.Fatalf("configured signatures = %d, %v; want one", len(configured), err)
	}
	all, err := signaturesFor(root, "", true)
	if err != nil || len(all) != 3 {
		t.Fatalf("all-host signatures = %d, %v; want three", len(all), err)
	}
	if got := Runtime("devin --project x", all); got != "devin" {
		t.Fatalf("Devin outside roster was not discoverable: %q", got)
	}
}
