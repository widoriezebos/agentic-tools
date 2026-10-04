package designgate

import (
	"errors"
	"testing"
)

func TestDesignGateAllowanceAndMode(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"warn", "refuse"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			f := Facts{Goal: "G", Tier: 2, Mode: mode}
			if got := Check(f); got.Mode != mode || !got.WouldRefuse {
				t.Fatalf("unallowed: %+v", got)
			}
			f.Allowed, f.Error = true, errors.New("unreadable design")
			if got := Check(f); got.Verdict != "allowed" || got.WouldRefuse || got.Mode != mode || got.Warning != [2]string{} {
				t.Fatalf("a standing allowance must precede the design check: %+v", got)
			}
			f.Tier = 1
			if got := Check(f); got.Verdict != "not-design-bearing" || got.WouldRefuse {
				t.Fatalf("tier one precedes the allowance: %+v", got)
			}
		})
	}
}
