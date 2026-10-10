package main

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingKeeperStartsOriginOnlyHandInInOneTick(t *testing.T) {
	t.Parallel()
	bed, keeper, now, starts, _ := landingRestartBed(t)
	if _, _, err := plain.HandIn(bed.landingA, plain.Line{Goal: "next", SHA: "origin-only"}); err != nil {
		t.Fatal(err)
	}
	original := bed.plainProve.Git
	fetched, fetches := false, 0
	bed.plainProve.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" && slices.Contains(args, "refs/heads/goal/next") {
			fetched = true
			fetches++
			return "", nil
		}
		if args[0] == "cat-file" && args[2] == "origin-only^{commit}" && !fetched {
			return "", errors.New("not a valid object name")
		}
		return original(dir, args...)
	}
	keeper.Sources.Reasons = func(string) ([]string, error) {
		return plain.WakeReasons(bed.landingA, bed.landingA, time.Time{}, *now, bed.plainProve)
	}
	// Even a prepared selection must fetch a hand-in that is absent locally.
	fetched = true
	record, _, err := lane.Read(bed.home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.SelectBatch(bed.landingA, bed.landingA, record, bed.plainProve); err != nil {
		t.Fatal(err)
	}
	fetched = false
	line := keeper.Step()
	if *starts != 1 || fetches != 1 {
		t.Fatalf("first tick: starts=%d branch fetches=%d line=%s", *starts, fetches, line)
	}
}
