package gaterun

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
