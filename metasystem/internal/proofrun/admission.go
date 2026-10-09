package proofrun

import (
	"fmt"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// AdmissionCapKey is the host-wide top-level proof admission setting.
const AdmissionCapKey = "proof.admission.top-level-max"

// AdmissionCap is the resolved host-wide limit and where its value came from.
type AdmissionCap struct {
	Max         int
	Key, Source string
}

// ResolveAdmissionCap resolves the host-wide limit, defaulting to one proof.
func ResolveAdmissionCap(confPath string, _ int) (AdmissionCap, error) {
	defaultMax := config.MustIntDefault(AdmissionCapKey)
	admission := AdmissionCap{Max: defaultMax, Key: AdmissionCapKey, Source: "default"}
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
	source, err := config.KeyOrigin(config.GetParams{Key: AdmissionCapKey, ConfPath: confPath})
	if err != nil {
		return AdmissionCap{}, fmt.Errorf("%s: %w", AdmissionCapKey, err)
	}
	if source == "default" {
		return admission, nil
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
		return fmt.Sprintf("ADMISSION_OVERLAP_UNKNOWN rank=host-load key=%s admitted=%d source=%s retry=retry-when-host-census-is-readable",
			admission.Key, admission.Max, admission.Source)
	}
	if !nestedKnown {
		return fmt.Sprintf("ADMISSION_NESTING_UNKNOWN rank=host-load key=%s admitted=%d observed=%d source=%s retry=retry-when-host-census-is-readable",
			admission.Key, admission.Max, sample.OverlappingHost, admission.Source)
	}
	return fmt.Sprintf("ADMISSION_REFUSED rank=host-load key=%s admitted=%d observed=%d source=%s retry=retry-when-a-launcher-ends",
		admission.Key, admission.Max, sample.OverlappingHost, admission.Source)
}
