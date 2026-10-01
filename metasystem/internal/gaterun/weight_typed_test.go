package gaterun

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// landing validate's Finalize tells a discharge the run or the policy
// refused (typed, settled without a reset) from a read that failed (left
// pending): each refusal carries its WEIGHT_* code, a read failure none.
func TestWeightDischargeRefusalsAreTyped(t *testing.T) {
	typed := func(t *testing.T, err error, code string) {
		t.Helper()
		var refusal *WeightRefusal
		if !errors.As(err, &refusal) || refusal.Code != code {
			t.Fatalf("discharge error %v; want a %s refusal", err, code)
		}
	}
	t.Run("not green", func(t *testing.T) {
		bed, now := weightAuthorityBed(t)
		completeWeightProof(t, bed, "red-proof", now, 1, nil)
		_, err := bed.discharge("bounded", 3, "red-proof", *now)
		typed(t, err, WeightRunNotGreen)
	})
	t.Run("obligation revision", func(t *testing.T) {
		bed, now := weightAuthorityBed(t)
		_, err := bed.discharge("bounded", 4, "wrong-revision", *now)
		typed(t, err, WeightResetNotAllowed)
	})
	t.Run("already reset", func(t *testing.T) {
		bed, now := weightAuthorityBed(t)
		completeWeightProof(t, bed, "green-proof", now, 0, nil)
		if _, err := bed.discharge("bounded", 3, "green-proof", *now); err != nil {
			t.Fatal(err)
		}
		_, err := bed.discharge("bounded", 3, "green-proof", *now)
		typed(t, err, WeightAlreadyReset)
	})
	// F-2 (serial: the weight bed sets the package clock).
	t.Run("read failures keep the green run pending", dischargeReadFailuresKeepTheGreenRunPending)
	t.Run("unreadable state", func(t *testing.T) {
		bed, now := weightAuthorityBed(t)
		if err := os.MkdirAll(filepath.Dir(weightPath(bed.root)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(weightPath(bed.root), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := bed.discharge("bounded", 3, "any-proof", *now)
		var refusal *WeightRefusal
		if err == nil || errors.As(err, &refusal) {
			t.Fatalf("an unreadable weight state = %v; want an untyped read failure", err)
		}
	})
}

// F-2: a read that fails while a green run's discharge is judged (its run
// record, the correlation policy) is not a refusal. Through landing
// validate's Finalize with the real weight code, the green run stays
// pending with its reservation kept, and its reset happens once the read
// works again.
func dischargeReadFailuresKeepTheGreenRunPending(t *testing.T) {
	for _, tc := range []struct {
		name   string
		break_ func(t *testing.T, root string) (restore func())
	}{
		{"run record unreadable", func(t *testing.T, root string) func() {
			path := run.RecordPath(root, "green-proof")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
			return func() {
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{"correlation policy unreadable", func(t *testing.T, root string) func() {
			path := filepath.Join(root, "metasystem.conf")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(append([]byte(nil), data...), []byte("metasystem.governance.correlation-policy=Z\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
			return func() {
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bed, now := weightAuthorityBed(t)
			completeWeightProof(t, bed, "green-proof", now, 0, nil)
			state, err := loadWeight(bed.root, *now)
			if err != nil {
				t.Fatal(err)
			}
			restore := tc.break_(t, bed.root)
			validate := newValidateBed(t)
			validate.reserved("green-proof", true)
			validate.reservation.Key.WeightGeneration = state.Generation
			validate.reservation.Authority = CadenceAuthority{GoalID: "bounded", ObligationRevision: 3}
			validate.outcome["green-proof"] = RunOutcome{Usable: true, Result: greenResult("attempt-green")}
			seams := validate.seams()
			seams.Weight = func() (WeightState, error) { return loadWeight(bed.root, *now) }
			seams.Discharge = func(authority CadenceAuthority, runID string, at time.Time) error {
				_, err := bed.discharge(authority.GoalID, authority.ObligationRevision, runID, *now)
				return err
			}
			outcome, err := Validate(false, seams)
			if err != nil || outcome.Result != ValidatePending || validate.reservation == nil || len(validate.published) != 0 {
				t.Fatalf("a read that failed = %+v %v, reservation %+v, published %d; want pending", outcome, err, validate.reservation, len(validate.published))
			}
			restore()
			outcome, err = Validate(false, seams)
			if err != nil || outcome.Result != ValidateFinalized || !outcome.Discharged {
				t.Fatalf("after the read works again = %+v %v; want finalized with the reset", outcome, err)
			}
		})
	}
}
