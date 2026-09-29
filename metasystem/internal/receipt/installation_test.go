package receipt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// templateInstallation lays out the template's nested installation
// (<repo>/metasystem beside <repo>/development/metasystem-design.md), the
// layout the state root resolves without consulting Git.
func templateInstallation(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	installation := filepath.Join(repo, "metasystem")
	for _, dir := range []string{installation, filepath.Join(repo, "development")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return installation
}

func TestAddToInstallationWritesTheInstallationLedgerWithVerbDefaults(t *testing.T) {
	t.Parallel()
	installation := templateInstallation(t)
	// A caller's File and Root are replaced: the ledger belongs to the
	// installation, not to the calling executable.
	decoy := filepath.Join(t.TempDir(), "decoy.log")
	result, err := AddToInstallation(installation, Options{
		Type: "implement", Outcome: "shipped", Verify: "clean", File: decoy, Root: "/elsewhere",
		Now: fixedNow, LookupEnv: noEnv,
	})
	if err != nil || result.Code != 0 {
		t.Fatalf("add to installation failed: %+v %v", result, err)
	}
	ledger := filepath.Join(installation, "memory", "receipts.log")
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("installation ledger not written: %v", err)
	}
	want := "1755000000|2025-08-12T12:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|"
	if !strings.HasPrefix(string(data), want) {
		t.Fatalf("defaults or explicit field wrong:\n got %s\nwant prefix %s", data, want)
	}
	if _, err := os.Stat(decoy); !os.IsNotExist(err) {
		t.Fatalf("the caller's file was written: %v", err)
	}
}

func TestAddToInstallationReturnsTheVerbRefusal(t *testing.T) {
	t.Parallel()
	installation := templateInstallation(t)
	result, err := AddToInstallation(installation, Options{Type: "not-a-type", Outcome: "shipped", Now: fixedNow, LookupEnv: noEnv})
	if err == nil || result.Code == 0 {
		t.Fatalf("an invalid receipt was accepted: %+v %v", result, err)
	}
	if !strings.HasPrefix(err.Error(), "receipt refused: ") || !strings.Contains(err.Error(), result.Err[0]) {
		t.Fatalf("the refusal does not carry the verb diagnostic: %v (%v)", err, result.Err)
	}
	if _, statErr := os.Stat(filepath.Join(installation, "memory", "receipts.log")); !os.IsNotExist(statErr) {
		t.Fatalf("a refused receipt reached the ledger: %v", statErr)
	}
}

func TestLedgerWritesReportUnwritablePaths(t *testing.T) {
	t.Parallel()
	// A ledger path that is a directory cannot be opened for append.
	opts := baseOptions(t)
	opts.File = t.TempDir()
	opts.Type, opts.Outcome, opts.Summary = "implement", "shipped", "tuned"
	for name, result := range map[string]Result{"add": Add(opts), "retro": Retro(opts)} {
		if result.Code != 2 || !strings.HasPrefix(result.Err[0], "cannot write receipt file: ") {
			t.Fatalf("%s: unwritable ledger not reported: %+v", name, result)
		}
	}
	// A ledger whose parent is a regular file cannot have its directory made.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	opts.File = filepath.Join(blocker, "receipts.log")
	for name, result := range map[string]Result{"add": Add(opts), "retro": Retro(opts)} {
		if result.Code != 2 || !strings.HasPrefix(result.Err[0], "cannot create receipt directory: ") {
			t.Fatalf("%s: uncreatable ledger directory not reported: %+v", name, result)
		}
	}
}
