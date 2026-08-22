package acp

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Envelope is the five-field permission contract, already expanded
// to absolute roots by the dispatch owner. Ordinals follow the
// shipped scales: network and approvals run deny < ask < allow,
// tools runs read-only < runtime-default.
type Envelope struct {
	ReadRoots  []string
	WriteRoots []string
	Network    string
	Approvals  string
	Tools      string
}

// PreflightACP reports why an envelope cannot ride the ACP
// transport in v1, or "" when it can. v1 has no escalation
// lifecycle, so approvals other than deny and network=ask fail
// preflight loudly instead of being silently answered as denials
// (spec: the permission decision, preflight narrowing).
func PreflightACP(envelope Envelope) string {
	// Unknown ordinal values fail CLOSED here: the shipped
	// comparator treats unknown effective values as wider-than-
	// anything, and a bogus grade must never reach Decide where an
	// exact-match check could read it as permissive.
	if envelope.Tools != "read-only" && envelope.Tools != "runtime-default" {
		return fmt.Sprintf("tools=%s is not on the ordinal scale", envelope.Tools)
	}
	switch envelope.Network {
	case "deny", "ask", "allow":
	default:
		return fmt.Sprintf("network=%s is not on the ordinal scale", envelope.Network)
	}
	switch envelope.Approvals {
	case "deny", "ask", "allow":
	default:
		return fmt.Sprintf("approvals=%s is not on the ordinal scale", envelope.Approvals)
	}
	if envelope.Approvals != "deny" {
		return fmt.Sprintf("approvals=%s is unsupported on ACP v1 (no escalation lifecycle)", envelope.Approvals)
	}
	if envelope.Network == "ask" {
		return "network=ask is unsupported on ACP v1 (no escalation lifecycle)"
	}
	return ""
}

// EffectClass is the dialect-neutral shape the request normalizer
// produces. Live wire captures show the permission
// machinery is idle in every tested mode, so Decide is the
// fail-closed BACKSTOP for requests that do fire, not the primary
// enforcement lever — that lever is the mode the adapter selects
// from the tools grade at session setup.
type EffectClass string

const (
	EffectRead    EffectClass = "read"
	EffectWrite   EffectClass = "write"
	EffectExecute EffectClass = "execute"
	EffectNetwork EffectClass = "network"
	EffectUnknown EffectClass = "unknown"
)

// Effect is one normalized effect. Paths are canonical (the
// normalizer resolves symlinks and relative segments before the
// pure decision).
type Effect struct {
	Class   EffectClass
	Paths   []string
	Command string
	Target  string
}

// Verdict is the pure decision outcome for a request.
type Verdict int

const (
	VerdictDeny Verdict = iota
	VerdictAllow
	VerdictUnclassifiable
)

// Decide is the pure, total decision over normalized effects and
// the envelope (spec: the Decide matrix). A request with multiple
// effects allows only when every constituent allows.
func Decide(effects []Effect, envelope Envelope) Verdict {
	if len(effects) == 0 {
		return VerdictUnclassifiable
	}
	if len(effects) == 1 {
		return decideOne(effects[0], envelope)
	}
	// The multi-effect row: allow only when EVERY constituent
	// allows; anything else — deny or unclassifiable — denies the
	// whole request (the matrix's "else the whole request denies").
	for _, effect := range effects {
		if decideOne(effect, envelope) != VerdictAllow {
			return VerdictDeny
		}
	}
	return VerdictAllow
}

func decideOne(effect Effect, envelope Envelope) Verdict {
	switch effect.Class {
	case EffectRead:
		if len(effect.Paths) == 0 {
			return VerdictUnclassifiable
		}
		roots := append(append([]string{}, envelope.ReadRoots...), envelope.WriteRoots...)
		if allInside(effect.Paths, roots) {
			return VerdictAllow
		}
		return VerdictDeny
	case EffectWrite:
		if len(effect.Paths) == 0 {
			return VerdictUnclassifiable
		}
		if envelope.Tools != "runtime-default" {
			return VerdictDeny
		}
		if allInside(effect.Paths, envelope.WriteRoots) {
			return VerdictAllow
		}
		return VerdictDeny
	case EffectExecute:
		// Admission, not containment: the command string is opaque
		// and roots are NOT verified for it; the raw command is
		// recorded as evidence by the caller. An EMPTY command is a
		// missing required fact.
		if effect.Command == "" {
			return VerdictUnclassifiable
		}
		if envelope.Tools != "runtime-default" {
			return VerdictDeny
		}
		return VerdictAllow
	case EffectNetwork:
		// network=ask cannot occur here — preflight refused it. An
		// empty target is a missing required fact.
		if effect.Target == "" {
			return VerdictUnclassifiable
		}
		if envelope.Network == "allow" {
			return VerdictAllow
		}
		return VerdictDeny
	}
	return VerdictUnclassifiable
}

// allInside reports whether every path sits inside at least one
// root. Paths and roots are canonical absolute paths.
func allInside(paths, roots []string) bool {
	for _, path := range paths {
		inside := false
		for _, root := range roots {
			if root == "" {
				continue
			}
			if path == root || strings.HasPrefix(path, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator)) {
				inside = true
				break
			}
		}
		if !inside {
			return false
		}
	}
	return len(paths) > 0
}

// PermissionOption is one offered option from a
// session/request_permission request. The response must return an
// exact offered option ID — the wire has no abstract allow/deny.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
}

// OutcomeSelected and OutcomeCancelled are the two response shapes.
const (
	outcomeSelected  = "selected"
	outcomeCancelled = "cancelled"
)

// PermissionAnswer is the response payload. WireResult produces
// the exact nested shape the pinned schema requires for the
// session/request_permission result.
type PermissionAnswer struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId,omitempty"`
}

// WireResult wraps the answer in the schema's result envelope:
// {"outcome":{"outcome":"selected","optionId":...}} or
// {"outcome":{"outcome":"cancelled"}}.
func (a PermissionAnswer) WireResult() map[string]PermissionAnswer {
	return map[string]PermissionAnswer{"outcome": a}
}

// MapVerdict turns a verdict into the wire answer under the
// exactly-one rule: the matching one-shot kind must appear exactly
// once among the offered options; zero, multiple, or duplicate-ID
// matches return cancelled. Persistent grants (allow_always,
// reject_always) are never selected — reject_always is remembered
// server-side and would poison a loaded repair session (spec:
// option mapping).
func MapVerdict(verdict Verdict, options []PermissionOption) PermissionAnswer {
	want := "reject_once"
	if verdict == VerdictAllow {
		want = "allow_once"
	}
	var ids []string
	for _, option := range options {
		if option.Kind == want {
			ids = append(ids, option.OptionID)
		}
	}
	if len(ids) != 1 || ids[0] == "" {
		return PermissionAnswer{Outcome: outcomeCancelled}
	}
	// The selected ID must be unambiguous across ALL offered
	// options, not merely within its kind — a server offering the
	// same ID under two kinds gets cancelled, never a guess.
	seen := 0
	for _, option := range options {
		if option.OptionID == ids[0] {
			seen++
		}
	}
	if seen != 1 {
		return PermissionAnswer{Outcome: outcomeCancelled}
	}
	return PermissionAnswer{Outcome: outcomeSelected, OptionID: ids[0]}
}

// StrictAnswer is the pre-dialect defensive mode: every request
// takes the deny branch regardless of content (spec: strict-refusal
// mode — a failure posture, not supported behavior).
func StrictAnswer(options []PermissionOption) PermissionAnswer {
	return MapVerdict(VerdictDeny, options)
}
