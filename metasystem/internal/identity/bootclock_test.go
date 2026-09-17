package identity

import (
	"errors"
	"testing"
	"time"
)

const testDarwinSessionUUID = "16CD714C-2D75-4544-8B28-69576F9570B9"

func TestDarwinBootIdentitySurvivesBootTimeAdjustment(t *testing.T) {
	first, err := DarwinBootIdentity(DarwinBootReadings{
		SessionUUID:  testDarwinSessionUUID,
		BootTimeSec:  1788592681,
		BootTimeUsec: 131526,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := DarwinBootIdentity(DarwinBootReadings{
		SessionUUID:  testDarwinSessionUUID,
		BootTimeSec:  1788592681,
		BootTimeUsec: 76372,
	})
	if err != nil || first != testDarwinSessionUUID || second != first {
		t.Fatalf("identities after clock adjustment = %q, %q, err=%v", first, second, err)
	}
}

func TestDarwinBootIdentityFallsBackFromUnavailableSession(t *testing.T) {
	want := "1788592681.131526"
	readings := []DarwinBootReadings{
		{
			SessionUUID:  testDarwinSessionUUID,
			SessionErr:   errors.New("unavailable"),
			BootTimeSec:  1788592681,
			BootTimeUsec: 131526,
		},
		{
			SessionUUID:  " \x00  \x00",
			BootTimeSec:  1788592681,
			BootTimeUsec: 131526,
		},
	}
	for _, reading := range readings {
		got, err := DarwinBootIdentity(reading)
		if err != nil || got != want {
			t.Fatalf("fallback identity = %q, %v; want %q", got, err, want)
		}
	}
}

func TestDarwinBootIdentityEstimatesToTheMinute(t *testing.T) {
	utcNow := time.Date(2026, 9, 16, 23, 20, 5, 0, time.UTC)
	boot := time.Date(2026, 9, 5, 9, 18, 1, 0, time.UTC)
	elapsed := utcNow.Sub(boot)
	for _, now := range []time.Time{
		utcNow,
		time.Date(2026, 9, 17, 1, 20, 5, 0, time.FixedZone("CEST", 2*60*60)),
	} {
		got, err := DarwinBootIdentity(DarwinBootReadings{
			SessionErr:   errors.New("unavailable"),
			BootTimeSec:  1788592681,
			BootTimeUsec: 131526,
			BootTimeErr:  errors.New("unavailable"),
			Now:          now,
			Elapsed:      elapsed,
		})
		if err != nil || got != "estimated-20260905T0918Z" {
			t.Fatalf("estimated identity at %s = %q, %v", now, got, err)
		}
	}
}

func TestDarwinBootIdentityRejectsImplausibleReadings(t *testing.T) {
	now := time.Date(2026, 9, 16, 23, 20, 5, 0, time.UTC)
	want := "estimated-20260916T2320Z"
	for _, reading := range []DarwinBootReadings{
		{BootTimeSec: 0, BootTimeUsec: 131526},
		{BootTimeSec: 1788592681, BootTimeUsec: -1},
		{BootTimeSec: 1788592681, BootTimeUsec: 1000000},
	} {
		reading.SessionErr = errors.New("unavailable")
		reading.Now = now
		got, err := DarwinBootIdentity(reading)
		if err != nil || got != want {
			t.Fatalf("implausible boot time identity = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := DarwinBootIdentity(DarwinBootReadings{
		SessionUUID: testDarwinSessionUUID,
		Elapsed:     -1,
	}); err == nil {
		t.Fatal("negative elapsed time was accepted")
	}
	for _, elapsed := range []time.Duration{
		time.Duration(now.Unix()) * time.Second,
		time.Duration(now.Unix()+1) * time.Second,
		time.Duration(now.Unix()-30) * time.Second,
	} {
		if _, err := DarwinBootIdentity(DarwinBootReadings{
			SessionErr:  errors.New("unavailable"),
			BootTimeErr: errors.New("unavailable"),
			Now:         now,
			Elapsed:     elapsed,
		}); err == nil {
			t.Fatalf("elapsed time %s did not reject an estimate in or before the epoch minute", elapsed)
		}
	}
}
