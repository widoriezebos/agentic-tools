package authority

import "testing"

func cls(class string, holder bool, jobID string) map[string]any {
	m := map[string]any{"class": class, "holder": holder}
	if jobID != "" {
		m["jobId"] = jobID
	}
	return m
}

func TestAuthorize(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		class   map[string]any
		job     string
		allowed bool
	}{
		{"human always", "holder-only", cls("HUMAN", false, ""), "", true},
		{"holder writes", "holder-only", cls("MAIN", true, ""), "", true},
		{"holder cannot standing-reap", "supervision-only", cls("MAIN", true, ""), "", false},
		{"non-holder main refused holder-only", "holder-only", cls("MAIN", false, ""), "", false},
		{"supervision writes record", "record-writer", cls("SUPERVISION", false, ""), "", true},
		{"supervision writes its own", "supervision-only", cls("SUPERVISION", false, ""), "", true},
		{"supervision refused adapter-writer", "adapter-writer", cls("SUPERVISION", false, ""), "", false},
		{"adapter writes its own job", "adapter-writer", cls("ADAPTER-SUPERVISOR", false, "job-7"), "job-7", true},
		{"adapter refused other job", "adapter-writer", cls("ADAPTER-SUPERVISOR", false, "job-7"), "job-9", false},
		{"adapter refused holder-only", "holder-only", cls("ADAPTER-SUPERVISOR", false, "job-7"), "job-7", false},
		{"delegate refused", "record-writer", cls("DELEGATE", false, ""), "", false},
	}
	for _, c := range cases {
		err := Authorize(c.mode, c.class, c.job)
		if (err == nil) != c.allowed {
			t.Errorf("%s: Authorize(%s) allowed=%v, err=%v", c.name, c.mode, err == nil, err)
		}
	}
}

func TestValidMode(t *testing.T) {
	for _, mode := range []string{"holder-only", "record-writer", "adapter-writer", "supervision-only"} {
		if !ValidMode(mode) {
			t.Errorf("%s must be a valid control-plane mode", mode)
		}
	}
	for _, mode := range []string{"", "holder", "HOLDER-ONLY", "everything"} {
		if ValidMode(mode) {
			t.Errorf("%q must not be a valid control-plane mode", mode)
		}
	}
}

// Genesis admits the human and the root's lease holder outright, and every
// other caller — a main without the lease, a delegate, a helper — only when
// the ledger it would baseline is adoption-shaped (goal-free on a checkout
// whose history carries none); the verb layer carries that flag in the
// classification.
func TestGenesisMode(t *testing.T) {
	if !ValidMode("genesis") {
		t.Fatal("genesis must be a known mode")
	}
	for _, tc := range []struct {
		class   string
		holder  bool
		shaped  bool
		allowed bool
	}{
		{"HUMAN", false, false, true},
		{"MAIN", true, false, true},
		{"MAIN", false, false, false},
		{"MAIN", false, true, true},
		{"DELEGATE", false, false, false},
		{"DELEGATE", false, true, true},
		{"SUPERVISION", false, false, false},
		{"SUPERVISION", false, true, true},
		{"ADAPTER-SUPERVISOR", false, false, false},
		{"ADAPTER-SUPERVISOR", false, true, true},
	} {
		err := Authorize("genesis", map[string]any{"class": tc.class, "holder": tc.holder, "adoptionShaped": tc.shaped}, "")
		if (err == nil) != tc.allowed {
			t.Fatalf("genesis class %s holder=%v shaped=%v: err=%v want allowed=%v", tc.class, tc.holder, tc.shaped, err, tc.allowed)
		}
	}
	// The flag is genesis-only: it never raises a holder-only write.
	if err := Authorize("holder-only", map[string]any{"class": "DELEGATE", "holder": false, "adoptionShaped": true}, ""); err == nil {
		t.Fatal("adoptionShaped must not raise a holder-only write")
	}
	if err := Authorize("holder-only", map[string]any{"class": "MAIN", "holder": false}, ""); err == nil {
		t.Fatal("holder-only must still refuse a non-holder main")
	}
}

func TestStewardIsAdmittedToExactlyItsOwnContinuationJob(t *testing.T) {
	own := map[string]any{"class": "STEWARD", "stewardJob": "job-7"}
	if err := Authorize("holder-only", own, "job-7"); err != nil {
		t.Fatalf("the steward launches the job its authorization names: %v", err)
	}
	if err := Authorize("holder-only", own, "job-8"); err == nil {
		t.Fatal("another job must refuse")
	}
	if err := Authorize("holder-only", map[string]any{"class": "STEWARD"}, "job-7"); err == nil {
		t.Fatal("a steward without a named continuation job must refuse")
	}
	if err := Authorize("record-writer", own, "job-7"); err != nil {
		t.Fatalf("the steward patches its own continuation's record: %v", err)
	}
	if err := Authorize("record-writer", own, "job-8"); err == nil {
		t.Fatal("another job's record must refuse")
	}
	for _, mode := range []string{"adapter-writer", "supervision-only", "genesis"} {
		if err := Authorize(mode, own, "job-7"); err == nil {
			t.Fatalf("mode %s must refuse the steward", mode)
		}
	}
}

func TestUntrustedRefusesEveryMode(t *testing.T) {
	caller := map[string]any{"class": "UNTRUSTED"}
	for _, mode := range []string{"holder-only", "record-writer", "adapter-writer", "supervision-only", "genesis"} {
		if err := Authorize(mode, caller, ""); err == nil {
			t.Fatalf("mode %s must refuse an untrusted caller", mode)
		}
	}
	// The adoption shape widens genesis to working callers only: an
	// unrecognized headless process stays out even when the ledger
	// it would baseline looks adoptable.
	shaped := map[string]any{"class": "UNTRUSTED", "adoptionShaped": true}
	if err := Authorize("genesis", shaped, ""); err == nil {
		t.Fatal("adoption shape must not admit an untrusted caller to genesis")
	}
	stewardShaped := map[string]any{"class": "STEWARD", "adoptionShaped": true, "stewardJob": "job-1"}
	if err := Authorize("genesis", stewardShaped, ""); err == nil {
		t.Fatal("the steward's one action is continuation dispatch, never genesis")
	}
}
