package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

func writeRulingRegister(t *testing.T, root string, rows []string) {
	t.Helper()
	path := filepath.Join(root, "memory", "rulings.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "| ID | Date | Decision | Evidence | Owner | Review |\n|---|---|---|---|---|---|\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRulingSweepRotatesOverflowAcrossRealDigests(t *testing.T) {
	root := t.TempDir()
	var rows []string
	for index := 0; index < 7; index++ {
		id := "R-due-" + string(rune('a'+index))
		rows = append(rows, "| "+id+" | 2026-08-29 | decision | evidence | Wido | class=temporary due=2026-08-29 |")
	}
	writeRulingRegister(t, root, rows)
	first := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	if err := sweepRulingReviews(root, first); err != nil {
		t.Fatal(err)
	}
	if err := sweepRulingReviews(root, first.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("two daily sweeps did not create two digest entries: %s", data)
	}
	seen := map[string]bool{}
	for _, line := range lines {
		if strings.Count(line, "owner=Wido") > rulingDigestCeiling {
			t.Fatalf("digest exceeded the hard attention ceiling: %s", line)
		}
		for index := 0; index < 7; index++ {
			id := "R-due-" + string(rune('a'+index))
			if strings.Contains(line, id) {
				seen[id] = true
			}
		}
	}
	if len(seen) != 7 {
		t.Fatalf("rotation starved overdue rows; seen=%v digest=%s", seen, data)
	}
}

func TestRulingSweepObservesNamedEventAndFlagsUnobservableEvent(t *testing.T) {
	root := t.TempDir()
	writeRulingRegister(t, root, []string{
		"| R-event | 2026-08-30 | decision | evidence | Wido | class=assumption-dependent event=first-measured-report-exists |",
		"| R-22-m1 | 2026-08-30 | decision | evidence | Wido | class=assumption-dependent event=exhaustion-window-roster-correlation-or-policy-change |",
	})
	report := filepath.Join(root, "artifacts", "agents", "governance", "correlation-policy-a-report.json")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := sweepRulingReviews(root, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "R-event") || !strings.Contains(text, "event-observed=first-measured-report-exists") ||
		!strings.Contains(text, "R-22-m1") || !strings.Contains(text, "needs-attention=event-condition-unobservable") {
		t.Fatalf("event-only reviews did not enter the real digest with evidence: %s", text)
	}
}

func TestRulingSweepSurfacesRegisterDefectsBeforeDueReviews(t *testing.T) {
	root := t.TempDir()
	writeRulingRegister(t, root, []string{
		"| R-ownerless | 2026-08-30 | decision | evidence | | class=temporary due=2026-08-29 |",
		"| R-malformed | 2026-08-30 | decision | evidence | Wido | class=temporary whenever=soon |",
		"| R-due | 2026-08-30 | decision | evidence | Wido | class=temporary due=2026-08-29 |",
	})
	if err := sweepRulingReviews(root, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("register content defects degraded the sweep: %v", err)
	}
	data, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	ownerless := strings.Index(text, "R-ownerless defect=no accountable owner choice=adopt|withdraw")
	malformed := strings.Index(text, "R-malformed defect=unknown review condition key")
	due := strings.Index(text, "R-due owner=Wido")
	if ownerless < 0 || malformed < 0 || due < 0 || ownerless > due || malformed > due {
		t.Fatalf("defects were not named before the due review with the prescribed ownerless choice: %s", text)
	}
}

func TestRulingSweepDoesNotSurfaceUnobservableEventBeforeFutureDueDate(t *testing.T) {
	root := t.TempDir()
	writeRulingRegister(t, root, []string{
		"| R-future | 2026-08-30 | decision | evidence | Wido | class=assumption-dependent due=2026-09-06 event=unknown-event |",
	})
	if err := sweepRulingReviews(root, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(narratordigest.Path(root)); !os.IsNotExist(err) {
		t.Fatalf("future review with an unobservable event entered the digest: data=%s err=%v", data, err)
	}
}

func TestRulingSweepSurfacesObservedEventBeforeFutureDueDate(t *testing.T) {
	root := t.TempDir()
	writeRulingRegister(t, root, []string{
		"| R-future | 2026-08-30 | decision | evidence | Wido | class=assumption-dependent due=2026-09-06 event=superseded-by-r22-m1 |",
		"| R-22-m1 | 2026-08-30 | decision | evidence | Wido | class=temporary due=2026-09-07 |",
	})
	if err := sweepRulingReviews(root, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "R-future") || !strings.Contains(text, "event-observed=superseded-by-r22-m1") {
		t.Fatalf("mechanically observed event did not override the future due date: %s", text)
	}
	if strings.Contains(text, "R-22-m1 owner=") {
		t.Fatalf("the future backstop row itself was surfaced early: %s", text)
	}
}

func TestRulingSweepLabelsMalformedColumnByRegisterRowPosition(t *testing.T) {
	root := t.TempDir()
	writeRulingRegister(t, root, []string{
		"| R-good | 2026-08-30 | decision | evidence | Wido | class=temporary due=2026-09-07 |",
		"| R-untrusted | too | few | columns |",
	})
	if err := sweepRulingReviews(root, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("malformed column content degraded the sweep: %v", err)
	}
	data, err := os.ReadFile(narratordigest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(data); !strings.Contains(text, "row=2 defect=wrong column count: got 4, want 6") {
		t.Fatalf("malformed row did not use its register-row position and column count: %s", text)
	}
}

func TestRulingRegisterHasOwnersAndOnlyTypedScheduledReviews(t *testing.T) {
	reviews, err := readRulingReviews(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) == 0 {
		t.Fatal("the register has no scheduled review conditions")
	}
}

func TestRulingSweepIsOptionalInAnAdoptedRepository(t *testing.T) {
	reviews, err := readRulingReviews(t.TempDir())
	if err != nil || reviews != nil {
		t.Fatalf("an adopted repository without the source register degraded its steward: %+v %v", reviews, err)
	}
}
