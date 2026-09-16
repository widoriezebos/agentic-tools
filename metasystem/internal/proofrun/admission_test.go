package proofrun

import (
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
)

func TestAdmissionCapResolvesFromConfigurationAndCores(t *testing.T) {
	tests := []struct {
		name, value, source string
		cores, want         int
		wantErr             bool
	}{
		{"absent-18", "", "cores", 18, 3, false},
		{"absent-8", "", "cores", 8, 1, false},
		{"absent-3", "", "cores", 3, 1, false},
		{"absent-negative", "", "cores", -1, 1, false},
		{"absent-key-in-a-populated-conf", "unrelated.value=1", "cores", 18, 3, false},
		{"configured", "5", "configured", 18, 5, false},
		{"disabled", "0", "configured", 18, 0, false},
		{"not-an-integer", "x", "", 18, 0, true},
		{"negative", "-1", "", 18, 0, true},
		{"too-large", "65", "", 18, 0, true},
		{"duplicate-key", AdmissionCapKey + "=1\n" + AdmissionCapKey + "=2", "", 18, 0, true},
		{"empty-path", "derived", "cores", 0, 1, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conf := ""
			if test.value != "derived" {
				conf = filepath.Join(t.TempDir(), "metasystem.conf")
				content := test.value
				if content != "" && !strings.Contains(content, "=") {
					content = AdmissionCapKey + "=" + content
				}
				writeTestFile(t, conf, []byte(content+"\n"), 0o600)
			}
			admission, err := ResolveAdmissionCap(conf, test.cores)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), AdmissionCapKey) || test.name != "duplicate-key" && !strings.Contains(err.Error(), "0 through 64") {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || admission.Max != test.want || admission.Key != AdmissionCapKey || admission.Source != test.source {
				t.Fatalf("admission = %+v, error = %v", admission, err)
			}
		})
	}
}
func admissionRequest(t *testing.T, admissionMax, overlaps int, known, nested bool) AdmissionRequest {
	t.Helper()
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18}, overlaps, known)
	loadSeams.nested = func(int64) (bool, bool) { return nested, true }
	root, identity := proofAttemptFixture(t, "admission")
	conf := filepath.Join(root, "admission.conf")
	writeTestFile(t, conf, []byte(AdmissionCapKey+"="+strconv.Itoa(admissionMax)+"\n"), 0o600)
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	return AdmissionRequest{ControlRoot: root, ExecutionRoot: root, ConfPath: conf, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)}
}
func TestAdmissionCapExemptsNestedReceipts(t *testing.T) {
	request := admissionRequest(t, 1, 3, true, true)
	attempt, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("nested reserve = %+v, %+v, %v", attempt, decision, err)
	}
	stored, err := ReadAttempt(request.ControlRoot, attempt.AttemptID)
	if err != nil || stored.AttemptID != attempt.AttemptID {
		t.Fatalf("nested attempt was not written: %+v, %v", stored, err)
	}
	joined, _, err := JoinedComponentDecisionLocked(request)
	if err != nil || joined.Disposition == DispositionAdmissionRefused {
		t.Fatalf("joined decision consulted admission cap: %+v, %v", joined, err)
	}
}
func TestAdmissionCapRefusesAtTheCap(t *testing.T) {
	for _, test := range []struct {
		name, want string
		overlaps   int
	}{{"at-cap", DispositionAdmissionRefused, 2}, {"below-cap", DispositionExecuted, 1}} {
		t.Run(test.name, func(t *testing.T) {
			request := admissionRequest(t, 2, test.overlaps, true, false)
			attempt, decision, err := ReserveLocked(request)
			if err != nil || decision.Disposition != test.want {
				t.Fatalf("reserve = %+v, %+v, %v", attempt, decision, err)
			}
			attempts, readErr := ReadAttempts(request.ControlRoot)
			if test.want == DispositionAdmissionRefused && (!reflect.DeepEqual(attempt, Attempt{}) || decision.ExitStatus != ExitAdmissionRefused || readErr != nil || len(attempts) != 0) {
				t.Fatalf("refusal left state: attempt=%+v decision=%+v retained=%d error=%v", attempt, decision, len(attempts), readErr)
			}
		})
	}
}
func TestAdmissionRefusalNamesItsExpiryAndRuling(t *testing.T) {
	reason := (AdmissionCap{Max: 2, Key: AdmissionCapKey, Source: "configured"}).RefusalReason(3)
	for _, part := range []string{AdmissionCapKey, "admitted=2", "observed=3", "ruling=R-111-m1e", "tests-never-wait-on-wall-time:1e,2,3b", "engine-policy-binding-survives-drift-and-load:U4a,U4b"} {
		if !strings.Contains(reason, part) {
			t.Fatalf("reason %q does not contain %q", reason, part)
		}
	}
}
func TestAdmissionCapAdmitsWhenOverlapIsUnknown(t *testing.T) {
	request := admissionRequest(t, 1, 99, false, false)
	attempt, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionExecuted || attempt.Load == nil || attempt.Load.Start.OverlapKnown {
		t.Fatalf("unknown overlap reserve = %+v, %+v, %v", attempt, decision, err)
	}
	stored, err := ReadAttempt(request.ControlRoot, attempt.AttemptID)
	if err != nil || stored.Load == nil || stored.Load.Start.OverlapKnown {
		t.Fatalf("unknown overlap was not recorded: %+v, %v", stored.Load, err)
	}
}
func TestAdmissionCapRefusalPredicate(t *testing.T) {
	check := func(name string, max, observed int, known, nested, nestedKnown, want bool) {
		t.Run(name, func(t *testing.T) {
			if got := (AdmissionCap{Max: max}).Refuses(LoadSample{OverlappingHost: observed, OverlapKnown: known}, nested, nestedKnown); got != want {
				t.Fatalf("refuses = %v, want %v", got, want)
			}
		})
	}
	check("unknown-host", 1, 99, false, false, true, false)
	check("disabled", 0, 99, true, false, true, false)
	check("nested", 1, 99, true, true, true, false)
	check("nested-unknown", 1, 99, true, false, false, false)
	check("at-cap", 2, 2, true, false, true, true)
	check("below-cap", 2, 1, true, false, true, false)
}
func TestAdmissionCapUsesTheRecordedStartSample(t *testing.T) {
	request := admissionRequest(t, 2, 0, true, false)
	calls := 0
	loadSeams.launchers = func(int64) (int, bool) {
		calls++
		if calls == 1 {
			return 1, true
		}
		return 99, true
	}
	attempt, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionExecuted || calls != 1 || attempt.Load.Start.OverlappingHost != 1 {
		t.Fatalf("reserve sampled %d times: attempt=%+v decision=%+v error=%v", calls, attempt, decision, err)
	}
}
