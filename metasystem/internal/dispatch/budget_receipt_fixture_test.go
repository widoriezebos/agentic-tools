package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type budgetReceiptBed struct {
	*goalAdmissionBed
	receipt *strictBudgetReceipt
}

type strictBudgetReceipt struct {
	t       *testing.T
	root    string
	top     string
	path    string
	data    []byte
	present bool
	want    int
	tips    int
	tops    int
	files   int
}

func newStrictBudgetReceipt(t *testing.T, root, top, path string) *strictBudgetReceipt {
	t.Helper()
	r := &strictBudgetReceipt{t: t, root: root, top: top, path: path}
	t.Cleanup(func() {
		if r.tips != r.want || r.tops != r.want || r.files != r.want {
			t.Errorf("receipt calls tip/top/file = %d/%d/%d, want %d each", r.tips, r.tops, r.files, r.want)
		}
	})
	return r
}

func (r *strictBudgetReceipt) reads() receiptAdmissionReads {
	return receiptAdmissionReads{
		AcceptedLedgerTip: func(root string) (string, bool, error) {
			r.t.Helper()
			r.tips++
			if root != r.root || r.tips > r.want {
				r.t.Fatalf("unexpected accepted tip read root=%q count=%d", root, r.tips)
			}
			return admissionFixtureTip, true, nil
		},
		TopLevel: func(root string) (string, error) {
			r.t.Helper()
			r.tops++
			if root != r.root || r.tops > r.want {
				return "", fmt.Errorf("unexpected repository top read root=%q count=%d", root, r.tops)
			}
			return r.top, nil
		},
		FileAt: func(root, tip, path string) ([]byte, bool, error) {
			r.t.Helper()
			r.files++
			if root != r.root || tip != admissionFixtureTip || path != r.path || r.files > r.want {
				return nil, false, fmt.Errorf("unexpected receipt read root=%q tip=%q path=%q count=%d", root, tip, path, r.files)
			}
			return append([]byte(nil), r.data...), r.present, nil
		},
	}
}

func newBudgetReceiptBed(t *testing.T, attempts, minutes, active uint64) *budgetReceiptBed {
	t.Helper()
	bed := newGoalAdmissionBed(t, 2)
	// A template installation makes RootForInstallation use its real layout
	// rule while the accepted goal remains an immutable in-memory snapshot.
	parent := filepath.Dir(bed.root)
	if err := os.MkdirAll(filepath.Join(parent, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "development", "metasystem-design.md"), []byte("# template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "metasystem")
	if err := os.Rename(bed.root, root); err != nil {
		t.Fatal(err)
	}
	bed.root = root
	bed.reads.NewWorld = func(got string) bool {
		if got != root {
			t.Fatalf("goal world root = %q, want %q", got, root)
		}
		return true
	}
	bed.reads.ResolveEndpoint = func(got string) (goal.Endpoint, error) {
		if got != root {
			return goal.Endpoint{}, fmt.Errorf("goal endpoint root = %q, want %q", got, root)
		}
		return goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: bed.repository}, nil
	}
	bed.reads.ResolveMachine = func(got string) (string, error) {
		if got != root {
			return "", fmt.Errorf("goal machine root = %q, want %q", got, root)
		}
		return "bed-m1", nil
	}
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse budget fixture: %v", problems)
	}
	file.Claimed.Revision = 3
	file.Claimed.At = file.History[2].At
	file.Claimed.AccountingRevision = 3
	file.StopCapability.Generation = 3
	file.StopCapability.Revision = 3
	file.Budget.AttemptLimit = attempts
	file.Budget.ReservedJobMinutesLimit = minutes
	file.Budget.ActiveJobLimit = active
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
	receipt := newStrictBudgetReceipt(t, root, root, "memory/receipts.log")
	bed.reads.Receipt = receipt.reads()
	return &budgetReceiptBed{goalAdmissionBed: bed, receipt: receipt}
}

func newReviewReceiptBed(t *testing.T) *budgetReceiptBed {
	bed := newBudgetReceiptBed(t, 20, 1000, 10)
	path := filepath.Join(bed.root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse review fixture: %v", problems)
	}
	file.Claimed.Revision = 2
	file.Claimed.At = file.History[1].At
	file.Claimed.AccountingRevision = 0
	file.StopCapability.Generation = 2
	file.StopCapability.Revision = 2
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.reviewChain(t)
	return bed
}

func (bed *budgetReceiptBed) setReceipt(t *testing.T, content string, calls int) {
	t.Helper()
	bed.receipt.want = calls
	if content == "" {
		bed.receipt.present = false
		bed.receipt.data = nil
		return
	}
	path := filepath.Join(bed.root, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bed.receipt.data = append([]byte(nil), data...)
	bed.receipt.present = true
}
