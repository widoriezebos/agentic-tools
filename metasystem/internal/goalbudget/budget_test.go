package goalbudget

import (
	"strings"
	"testing"
	"time"
)

func TestParseWorkingDurationReadsCompoundsAndRefusesMalformed(t *testing.T) {
	for _, test := range []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"4h", 4 * time.Hour, true},
		{"90m", 90 * time.Minute, true},
		{"1d", 8 * time.Hour, true},
		{"1d2h30m", 8*time.Hour + 2*time.Hour + 30*time.Minute, true},
		{"24h", 24 * time.Hour, true},
		{"", 0, false},
		{"h", 0, false},
		{"4", 0, false},
		{"0h", 0, false},
		{"-2h", 0, false},
		{"4x", 0, false},
		{"4h5", 0, false},
	} {
		got, ok := ParseWorkingDuration(test.in)
		if ok != test.ok || got != test.want {
			t.Fatalf("ParseWorkingDuration(%q) = %v, %v; want %v, %v", test.in, got, ok, test.want, test.ok)
		}
	}
}

func TestFormatWorkingDurationUsesEightHourDaysAndRoundTrips(t *testing.T) {
	if got := FormatWorkingDuration(24 * time.Hour); got != "3d" {
		t.Fatalf("24 clock hours must render as 3 working days, got %q", got)
	}
	if got := FormatWorkingDuration(0); got != "" {
		t.Fatalf("a non-positive duration renders empty, got %q", got)
	}
	for _, in := range []string{"3d", "1d2h30m", "45m", "7h"} {
		parsed, ok := ParseWorkingDuration(in)
		if !ok {
			t.Fatalf("canonical %q must parse", in)
		}
		if got := FormatWorkingDuration(parsed); got != in {
			t.Fatalf("canonical form must round-trip: %q -> %q", in, got)
		}
	}
}

func TestNewNormalizesAndRefusesIncompleteTuples(t *testing.T) {
	b, err := New("24h", 6, 960, 2, 3)
	if err != nil {
		t.Fatalf("a complete tuple was refused: %v", err)
	}
	if b.ElapsedLimit != "3d" || b.AttemptLimit != 6 || b.ReservedJobMinutesLimit != 960 || b.ActiveJobLimit != 2 || b.ReviewRoundLimit != 3 {
		t.Fatalf("the tuple did not normalize as recorded: %+v", b)
	}
	if _, err := New("nonsense", 1, 1, 1, 0); err == nil || !strings.Contains(err.Error(), "elapsedLimit") {
		t.Fatalf("a malformed elapsed limit must refuse naming the field, got %v", err)
	}
	for _, bad := range [][3]int64{{0, 1, 1}, {1, 0, 1}, {1, 1, 0}, {-1, 1, 1}} {
		if _, err := New("4h", bad[0], bad[1], bad[2], 0); err == nil {
			t.Fatalf("a non-positive limit %v must refuse", bad)
		}
	}
	if _, err := New("4h", 1, 1, 1, -1); err == nil || !strings.Contains(err.Error(), "reviewRoundLimit") {
		t.Fatalf("a negative review limit must refuse, got %v", err)
	}
}

func TestValidateMirrorsConstructionRules(t *testing.T) {
	good := Budget{ElapsedLimit: "4h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1}
	if err := good.Validate(); err != nil {
		t.Fatalf("a valid record was refused: %v", err)
	}
	for _, bad := range []Budget{
		{ElapsedLimit: "", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1},
		{ElapsedLimit: "4h", AttemptLimit: 0, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1},
		{ElapsedLimit: "4h", AttemptLimit: 1, ReservedJobMinutesLimit: 0, ActiveJobLimit: 1},
		{ElapsedLimit: "4h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 0},
	} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("an incomplete record passed validation: %+v", bad)
		}
	}
}

func TestReviewRoundValidation(t *testing.T) {
	budget := Budget{ElapsedLimit: "4h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1, ReviewRoundLimit: 4}
	negative := budget
	negative.ReviewRoundLimit = -1
	if err := negative.Validate(); err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("a negative review-round limit passed validation: %v", err)
	}
	if err := budget.Validate(3); err == nil || !strings.Contains(err.Error(), "maximum 3") {
		t.Fatalf("a fourth round escaped the configured ceiling: %v", err)
	}
	tierOne := budget
	tierOne.ReviewRoundLimit = 0
	if err := tierOne.Validate(3); err != nil {
		t.Fatalf("the tier-one zero-round limit was refused: %v", err)
	}
	if err := budget.Validate(0); err != nil {
		t.Fatalf("the configured zero ceiling must admit every non-negative limit: %v", err)
	}
	if err := budget.Validate(3, 4); err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Fatalf("multiple configured ceilings passed validation: %v", err)
	}
}

func TestElapsedBreachDurationAppliesGraceAndGuardsOverflow(t *testing.T) {
	b := Budget{ElapsedLimit: "10h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1}
	if got := b.ElapsedDuration(); got != 10*time.Hour {
		t.Fatalf("elapsed duration misread: %v", got)
	}
	breach, err := b.ElapsedBreachDuration(50)
	if err != nil || breach != 15*time.Hour {
		t.Fatalf("10h with 50 percent grace must breach at 15h, got %v, %v", breach, err)
	}
	breach, err = b.ElapsedBreachDuration(0)
	if err != nil || breach != 10*time.Hour {
		t.Fatalf("zero grace must breach at the limit, got %v, %v", breach, err)
	}
	if _, err := (Budget{ElapsedLimit: "bad"}).ElapsedBreachDuration(10); err == nil {
		t.Fatal("a malformed limit must refuse a breach computation")
	}
	huge := Budget{ElapsedLimit: "1000000000d"}
	if _, ok := ParseWorkingDuration(huge.ElapsedLimit); ok {
		if _, err := huge.ElapsedBreachDuration(1000000000); err == nil {
			t.Fatal("an overflowing grace computation must refuse")
		}
	}
}
