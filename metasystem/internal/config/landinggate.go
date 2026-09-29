package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The landing gate's two settings (g1-s70 D1), read through the layered
// resolution every other setting uses — the environment, then the .local
// overlay, then the committed file, then the compiled default — so a
// threshold set in .local binds the gate, the clock and the page alike.
const (
	LandingHumanFromTierKey = "landing.review.human-from-tier"
	LandingAutoAfterKey     = "landing.review.auto-after"
)

// LandingGateFact is one setting as the resolution answered it, with the
// source it came from: env, conf-local, conf or default.
type LandingGateFact struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// LandingGate is both settings, parsed, with their facts.
type LandingGate struct {
	HumanFromTier uint8
	AutoAfter     time.Duration
	Tier          LandingGateFact
	After         LandingGateFact
}

// ResolveLandingGate resolves both settings for the installation whose
// metasystem.conf is confPath. A malformed value is refused, never replaced by
// the default: a gate that silently ran on another threshold is the failure
// the layered read exists to prevent.
func ResolveLandingGate(confPath string) (LandingGate, error) {
	read := func(key string) (LandingGateFact, error) {
		params := GetParams{Key: key, ConfPath: confPath}
		value, _, err := Get(params)
		if err != nil {
			return LandingGateFact{}, err
		}
		source, err := KeyOrigin(params)
		if err != nil {
			return LandingGateFact{}, err
		}
		return LandingGateFact{Key: key, Value: strings.TrimSpace(value), Source: source}, nil
	}
	tier, err := read(LandingHumanFromTierKey)
	if err != nil {
		return LandingGate{}, err
	}
	after, err := read(LandingAutoAfterKey)
	if err != nil {
		return LandingGate{}, err
	}
	threshold, err := landingTier(tier.Value)
	if err != nil {
		return LandingGate{}, err
	}
	grace, err := landingAutoAfter(after.Value)
	if err != nil {
		return LandingGate{}, err
	}
	return LandingGate{HumanFromTier: threshold, AutoAfter: grace, Tier: tier, After: after}, nil
}

// landingTier reads the threshold: 1 to 3 wait from that tier, 4 lets every
// tier land by itself.
func landingTier(raw string) (uint8, error) {
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 1 || parsed > 4 {
		return 0, fmt.Errorf("%s must be a tier from 1 through 4 (4: no tier waits for a person), got %s", LandingHumanFromTierKey, pyRepr(raw))
	}
	return uint8(parsed), nil
}

func landingAutoAfter(raw string) (time.Duration, error) {
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 4h or 30m, got %s", LandingAutoAfterKey, pyRepr(raw))
	}
	return parsed, nil
}
