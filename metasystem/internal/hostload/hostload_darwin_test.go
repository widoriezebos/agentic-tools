package hostload

import (
	"encoding/hex"
	"testing"
)

func TestParseDarwinLoadavg(t *testing.T) {
	// A reading taken on the eighteen-core box on 2026-09-12: 5.04, 7.68 and
	// 7.71 at a scale of 2048.
	raw, err := hex.DecodeString("5b280000753d0000b73d0000000000000008000000000000")
	if err != nil {
		t.Fatal(err)
	}
	one, five, fifteen, err := parseDarwinLoadavg(raw)
	if err != nil || one < 5.04 || one > 5.05 || five < 7.68 || five > 7.69 || fifteen < 7.71 || fifteen > 7.72 {
		t.Fatalf("parsed %v %v %v, %v", one, five, fifteen, err)
	}
	if _, _, _, err := parseDarwinLoadavg(raw[:20]); err == nil {
		t.Fatal("a short buffer parsed")
	}
	zeroScale := append([]byte(nil), raw...)
	copy(zeroScale[16:24], make([]byte, 8))
	if _, _, _, err := parseDarwinLoadavg(zeroScale); err == nil {
		t.Fatal("a zero scale parsed")
	}
}
