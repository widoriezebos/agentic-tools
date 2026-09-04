package landing

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReceiptCommandBoundResolution(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if got := receiptCommandBound(conf); got.Limit != 40*time.Minute || got.Key != "landing.receipt-bound-min" {
		t.Fatalf("absent receipt bound = %+v", got)
	}
	if err := os.WriteFile(conf, []byte("landing.receipt-bound-min=17\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := receiptCommandBound(conf); got.Limit != 17*time.Minute || got.Key != "landing.receipt-bound-min" {
		t.Fatalf("configured receipt bound = %+v", got)
	}
	for _, malformed := range []string{"0", "nonsense"} {
		if err := os.WriteFile(conf, []byte("landing.receipt-bound-min="+malformed+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := receiptCommandBound(conf); got.Limit != 40*time.Minute {
			t.Fatalf("malformed receipt bound %q did not fall back: %+v", malformed, got)
		}
	}
}
