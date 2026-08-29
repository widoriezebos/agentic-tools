package main

import "testing"

func TestBreachStopHasNoCallerAssertedLockFlag(t *testing.T) {
	if code := runDispatchBreachStop([]string{"--lock-held"}); code != 2 {
		t.Fatalf("caller-asserted lock metadata remained accepted: exit=%d", code)
	}
}
