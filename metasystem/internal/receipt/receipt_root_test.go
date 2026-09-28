package receipt

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// EM-02: run from a checkout root whose installation is its metasystem/
// directory, the retro numbers were refused with a false diagnosis (the
// ledger "does not parse: the root record is missing") after a Go open
// error for the configuration. The owner names the root as what is wrong,
// and the installation it can be pointed at.
func TestReceiptReadsFromANonInstallationNameTheRoot(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	installation := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(t.TempDir(), "receipts.log")
	if err := os.WriteFile(ledger, []byte("1790000000|RECEIPT|type=implement|outcome=shipped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := checkout + " is not a metasystem installation (it has no metasystem.conf); nothing was read; name the installation with --root " + installation

	check := Check(Options{Root: checkout, File: ledger, Now: fixedNow, LookupEnv: noEnv})
	if check.Code != 2 || len(check.Err) != 1 || check.Err[0] != want {
		t.Fatalf("check from the checkout root = %+v, want code 2 and %q", check, want)
	}
	readItems := func(string) ([]string, bool, error) {
		return nil, true, errors.New("the ledger tree at 16e4150ff74b does not parse: plans/goals/backlog.md: the root record is missing")
	}
	stats := Stats(Options{Root: checkout, File: ledger, Now: fixedNow, LookupEnv: noEnv, ReadItems: readItems})
	if stats.Code != 2 || len(stats.Err) != 1 || stats.Err[0] != want {
		t.Fatalf("stats from the checkout root = %+v, want code 2 and %q", stats, want)
	}

	// With no installation below it either, the refusal says how to name one.
	bare := t.TempDir()
	check = Check(Options{Root: bare, File: ledger, Now: fixedNow, LookupEnv: noEnv})
	if want := bare + " is not a metasystem installation (it has no metasystem.conf); nothing was read; run this inside the installation, or name it with --root INSTALLATION"; check.Code != 2 || check.Err[0] != want {
		t.Fatalf("check from a bare directory = %+v, want %q", check, want)
	}

	// A ledger failure inside a real installation keeps its own message.
	stats = Stats(Options{Root: installation, File: ledger, Now: fixedNow, LookupEnv: noEnv, ReadItems: readItems})
	if stats.Code != 2 || stats.Err[0] != "cannot read goal ledger for retro: the ledger tree at 16e4150ff74b does not parse: plans/goals/backlog.md: the root record is missing" {
		t.Fatalf("stats in the installation = %+v", stats)
	}
}
