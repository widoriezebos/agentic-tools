package proofrun

import (
	"fmt"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

const (
	// AdmissionCapKey is the host-wide top-level proof admission setting.
	AdmissionCapKey = "proof.admission.top-level-max"
	// AdmissionCapRuling names the ruling that introduced the temporary cap.
	AdmissionCapRuling = "R-111-m1e"
	// AdmissionCapExpiry names the landings that retire this temporary cap (ruling R-111-m1e, 2026-09-15, as the
	// seat's 2026-09-16 ruling narrows it): units 1e (gate on, initial manifest), 2 (protection) and 3b (the judge)
	// of goal tests-never-wait-on-wall-time, and units U4a (progress deadline, steward side) and U4b (progress
	// deadline and typed facts, command side) of goal engine-policy-binding-survives-drift-and-load. Those five
	// units remove the wall-time waits and completion deadlines that made concurrent batteries unsafe; the rest of
	// both pages lands at its own pace and does not hold this cap open.
	AdmissionCapExpiry = "tests-never-wait-on-wall-time:1e,2,3b+engine-policy-binding-survives-drift-and-load:U4a,U4b"
)

// AdmissionCap is the resolved host-wide limit and where its value came from.
type AdmissionCap struct {
	Max         int
	Key, Source string
}

// ResolveAdmissionCap resolves a configured limit or derives one from the host's cores.
func ResolveAdmissionCap(confPath string, cores int) (AdmissionCap, error) {
	admission := AdmissionCap{Max: max(1, cores/6), Key: AdmissionCapKey, Source: "cores"}
	if confPath == "" {
		return admission, nil
	}
	value, _, err := config.Get(config.GetParams{Key: AdmissionCapKey, ConfPath: confPath, Default: "", DefaultSet: true})
	if err != nil {
		return AdmissionCap{}, fmt.Errorf("%s: %w", AdmissionCapKey, err)
	}
	if value == "" {
		return admission, nil
	}
	configured, err := strconv.Atoi(value)
	if err != nil || configured < 0 || configured > 64 {
		return AdmissionCap{}, fmt.Errorf("%s must be an integer from 0 through 64", AdmissionCapKey)
	}
	return AdmissionCap{Max: configured, Key: AdmissionCapKey, Source: "configured"}, nil
}

// Refuses reports whether this cap refuses a top-level reserve that observed sample, given whether the
// reserving process is nested under a live proof launcher. An enabled cap
// refuses an unknown host overlap rather than admitting work without a census.
// A nested receipt whose parent already holds the slot remains admitted because
// refusing it would deadlock its battery.
func (admission AdmissionCap) Refuses(sample LoadSample, nested, nestedKnown bool) bool {
	if admission.Max <= 0 {
		return false
	}
	if nestedKnown && nested {
		return false
	}
	if !sample.OverlapKnown {
		return true
	}
	if sample.OverlappingHost < admission.Max {
		return false
	}
	return true
}

// RefusalReason renders the complete retryable admission refusal.
func (admission AdmissionCap) RefusalReason(sample LoadSample, nestedKnown bool) string {
	if !sample.OverlapKnown {
		return fmt.Sprintf("ADMISSION_OVERLAP_UNKNOWN rank=host-load key=%s admitted=%d source=%s retry=retry-when-host-census-is-readable temporary=yes ruling=%s expires-when=%s land",
			admission.Key, admission.Max, admission.Source, AdmissionCapRuling, AdmissionCapExpiry)
	}
	if !nestedKnown {
		return fmt.Sprintf("ADMISSION_NESTING_UNKNOWN rank=host-load key=%s admitted=%d observed=%d source=%s retry=retry-when-host-census-is-readable temporary=yes ruling=%s expires-when=%s land",
			admission.Key, admission.Max, sample.OverlappingHost, admission.Source, AdmissionCapRuling, AdmissionCapExpiry)
	}
	return fmt.Sprintf("ADMISSION_REFUSED rank=host-load key=%s admitted=%d observed=%d source=%s retry=retry-when-a-launcher-ends temporary=yes ruling=%s expires-when=%s land",
		admission.Key, admission.Max, sample.OverlappingHost, admission.Source, AdmissionCapRuling, AdmissionCapExpiry)
}
