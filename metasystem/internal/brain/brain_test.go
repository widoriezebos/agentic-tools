package brain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testLedger = "01J5X0000000000000000BRAIN"

func TestReadThreeStatesAndReasons(t *testing.T) {
	root := t.TempDir()
	if got := Read(root, testLedger); got.State != Undeclared {
		t.Fatalf("absent state = %#v", got)
	}
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Read(root, testLedger); got.State != Corrupt || !strings.Contains(got.Reason, "not JSON") {
		t.Fatalf("malformed state = %#v", got)
	}
	record := Record{Schema: 1, Ledger: testLedger, Machine: "brain", DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z"}
	data, _ := json.Marshal(record)
	if err := os.WriteFile(Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Read(root, testLedger); got.State != Declared || got.Record.Machine != "brain" {
		t.Fatalf("declared state = %#v", got)
	}
	if got := Read(root, "01J5X0000000000000000OTHER"); got.State != Corrupt || !strings.Contains(got.Reason, "not this checkout's ledger") {
		t.Fatalf("wrong-ledger state = %#v", got)
	}
}

func TestDeclarationCaps(t *testing.T) {
	stamp := "2026-09-07T00:00:00Z"
	if err := ValidateDeclaration(testLedger, strings.Repeat("m", MachineBytes), strings.Repeat("b", DeclaredByBytes), stamp); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"machine":    ValidateDeclaration(testLedger, strings.Repeat("m", MachineBytes+1), "Wido", stamp),
		"declaredBy": ValidateDeclaration(testLedger, "brain", strings.Repeat("b", DeclaredByBytes+1), stamp),
		"control":    ValidateDeclaration(testLedger, "brain", "Wi\ndo", stamp),
	} {
		if err == nil {
			t.Fatalf("%s cap accepted", name)
		}
	}
}

func TestMaximumDeclarationHeaderFitsExplicitBound(t *testing.T) {
	if HeaderBytes != 320 {
		t.Fatalf("header bound = %d, want 320 bytes", HeaderBytes)
	}
	root := t.TempDir()
	record := Record{
		Schema: Schema, Ledger: testLedger, Machine: strings.Repeat("m", MachineBytes),
		DeclaredBy: strings.Repeat("b", DeclaredByBytes), DeclaredAt: "2026-09-07T00:00:00Z",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	_, payload := PhaseOne(root, root, testLedger, 10000)
	header, _, _ := strings.Cut(payload, "\n")
	if len(header) > HeaderBytes {
		t.Fatalf("maximal declaration header is %d bytes, exceeds %d-byte bound", len(header), HeaderBytes)
	}
}

func TestDeclareSerializesHostPointerAndWithdrawsCorrupt(t *testing.T) {
	registry := t.TempDir()
	one := t.TempDir()
	two := t.TempDir()
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	options := []DeclareOptions{
		{StateRoot: one, RegistryHome: registry, LedgerIdentity: testLedger, Machine: "one", DeclaredBy: "Wido", Now: now},
		{StateRoot: two, RegistryHome: registry, LedgerIdentity: testLedger, Machine: "two", DeclaredBy: "Wido", Now: now},
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for index := range options {
		wg.Add(1)
		go func(index int) { defer wg.Done(); _, errs[index] = Declare(options[index]) }(index)
	}
	wg.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("declare errors = %v, %v", errs[0], errs[1])
	}
	winner := one
	if errs[1] == nil {
		winner = two
	}
	pointer, err := os.ReadFile(PointerPath(registry, testLedger))
	if err != nil {
		t.Fatal(err)
	}
	winnerPath, _ := canonicalPath(winner)
	if strings.TrimSpace(string(pointer)) != winnerPath {
		t.Fatalf("pointer = %q, want %q", pointer, winnerPath)
	}
	if err := os.WriteFile(Path(winner), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Withdraw(winner, registry, testLedger)
	if err != nil || !removed {
		t.Fatalf("withdraw = %v, %v", removed, err)
	}
	if _, err := os.Stat(PointerPath(registry, testLedger)); !os.IsNotExist(err) {
		t.Fatalf("pointer remains: %v", err)
	}
}

func TestWithdrawWithoutLedgerIdentityDoesNotTouchPointerDirectory(t *testing.T) {
	registry := t.TempDir()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pointerDir := PointerPath(registry, "")
	if err := os.MkdirAll(pointerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(pointerDir, "keep")
	if err := os.WriteFile(marker, []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := Withdraw(root, registry, "")
	if err != nil || !removed {
		t.Fatalf("empty-ledger withdraw = %v, %v", removed, err)
	}
	if _, err := os.Stat(Path(root)); !os.IsNotExist(err) {
		t.Fatalf("brain record remains: %v", err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "untouched\n" {
		t.Fatalf("pointer directory marker changed: %q, %v", data, err)
	}
	if _, err := os.Stat(pointerDir + ".lock.d"); !os.IsNotExist(err) {
		t.Fatalf("empty-ledger withdraw touched the pointer lock: %v", err)
	}
}

func TestFenceVerbs(t *testing.T) {
	root := t.TempDir()
	if got := Fence(root, "dispatch", testLedger); got != "" {
		t.Fatalf("undeclared fence = %q", got)
	}
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	record := Record{Schema: 1, Ledger: testLedger, Machine: "brain", DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z"}
	data, _ := json.Marshal(record)
	if err := os.WriteFile(Path(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, act := range []string{"dispatch", "follow-up", "cancel", "close", "reap", "land", "claim"} {
		if got := Fence(root, act, testLedger); got == "" {
			t.Fatalf("%s was not fenced", act)
		}
	}
}
