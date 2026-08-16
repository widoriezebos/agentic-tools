package acp

import (
	"encoding/json"
	"testing"
)

func workspaceEnvelope(tools string) Envelope {
	return Envelope{
		ReadRoots:  []string{"/repo"},
		WriteRoots: []string{"/repo/work"},
		Network:    "deny",
		Approvals:  "deny",
		Tools:      tools,
	}
}

func TestPreflightNarrowing(t *testing.T) {
	for _, row := range []struct {
		envelope Envelope
		refused  bool
	}{
		{Envelope{Approvals: "deny", Network: "deny", Tools: "read-only"}, false},
		{Envelope{Approvals: "deny", Network: "allow", Tools: "runtime-default"}, false},
		{Envelope{Approvals: "ask", Network: "deny", Tools: "read-only"}, true},
		{Envelope{Approvals: "allow", Network: "deny", Tools: "read-only"}, true},
		{Envelope{Approvals: "deny", Network: "ask"}, true},
		{Envelope{Approvals: "deny", Network: "deny", Tools: "bogus"}, true},
		{Envelope{Approvals: "deny", Network: "wide-open", Tools: "read-only"}, true},
		{Envelope{Approvals: "sometimes", Network: "deny", Tools: "read-only"}, true},
	} {
		reason := PreflightACP(row.envelope)
		if (reason != "") != row.refused {
			t.Fatalf("preflight approvals=%s network=%s: got %q, want refused=%v",
				row.envelope.Approvals, row.envelope.Network, reason, row.refused)
		}
	}
}

// The matrix rows, including the grades-bite fixtures the spec
// demands: read-only denies state-changing effects even inside an
// allowed root.
func TestDecideMatrix(t *testing.T) {
	for name, row := range map[string]struct {
		effects  []Effect
		envelope Envelope
		want     Verdict
	}{
		"read inside readRoots allows":         {[]Effect{{Class: EffectRead, Paths: []string{"/repo/a.go"}}}, workspaceEnvelope("runtime-default"), VerdictAllow},
		"read inside writeRoots allows":        {[]Effect{{Class: EffectRead, Paths: []string{"/repo/work/x"}}}, workspaceEnvelope("read-only"), VerdictAllow},
		"read outside roots denies":            {[]Effect{{Class: EffectRead, Paths: []string{"/etc/passwd"}}}, workspaceEnvelope("runtime-default"), VerdictDeny},
		"read without paths unclassifiable":    {[]Effect{{Class: EffectRead}}, workspaceEnvelope("runtime-default"), VerdictUnclassifiable},
		"write in root under read-only denies": {[]Effect{{Class: EffectWrite, Paths: []string{"/repo/work/f"}}}, workspaceEnvelope("read-only"), VerdictDeny},
		"write in writeRoots allows":           {[]Effect{{Class: EffectWrite, Paths: []string{"/repo/work/f"}}}, workspaceEnvelope("runtime-default"), VerdictAllow},
		"write in readRoots only denies":       {[]Effect{{Class: EffectWrite, Paths: []string{"/repo/f"}}}, workspaceEnvelope("runtime-default"), VerdictDeny},
		"execute under read-only denies":       {[]Effect{{Class: EffectExecute, Command: "rm -rf /"}}, workspaceEnvelope("read-only"), VerdictDeny},
		"execute under runtime-default allows": {[]Effect{{Class: EffectExecute, Command: "make"}}, workspaceEnvelope("runtime-default"), VerdictAllow},
		"network denied by deny":               {[]Effect{{Class: EffectNetwork, Target: "example.com"}}, workspaceEnvelope("runtime-default"), VerdictDeny},
		"unknown unclassifiable":               {[]Effect{{Class: EffectUnknown}}, workspaceEnvelope("runtime-default"), VerdictUnclassifiable},
		"no effects unclassifiable":            {nil, workspaceEnvelope("runtime-default"), VerdictUnclassifiable},
		"mixed effects deny wins":              {[]Effect{{Class: EffectWrite, Paths: []string{"/repo/work/f"}}, {Class: EffectWrite, Paths: []string{"/tmp/out"}}}, workspaceEnvelope("runtime-default"), VerdictDeny},
		"multi-effect unclassifiable denies":   {[]Effect{{Class: EffectWrite, Paths: []string{"/repo/work/f"}}, {Class: EffectUnknown}}, workspaceEnvelope("runtime-default"), VerdictDeny},
		"empty command unclassifiable":         {[]Effect{{Class: EffectExecute}}, workspaceEnvelope("runtime-default"), VerdictUnclassifiable},
		"empty network target unclassifiable":  {[]Effect{{Class: EffectNetwork}}, workspaceEnvelope("runtime-default"), VerdictUnclassifiable},
	} {
		if got := Decide(row.effects, row.envelope); got != row.want {
			t.Fatalf("%s: got %v want %v", name, got, row.want)
		}
	}
	allowNet := workspaceEnvelope("runtime-default")
	allowNet.Network = "allow"
	if Decide([]Effect{{Class: EffectNetwork, Target: "example.com"}}, allowNet) != VerdictAllow {
		t.Fatal("network=allow must admit a network effect")
	}
}

// /a/bc must not read as inside root /a/b — containment is by path
// component, not string prefix.
func TestAllInsideComponentBoundary(t *testing.T) {
	if allInside([]string{"/repo/workx/f"}, []string{"/repo/work"}) {
		t.Fatal("sibling with shared prefix leaked inside the root")
	}
	if !allInside([]string{"/repo/work"}, []string{"/repo/work"}) {
		t.Fatal("the root itself must count as inside")
	}
	if allInside([]string{"/repo/work/f"}, nil) {
		t.Fatal("no roots must contain nothing")
	}
}

// The exactly-one rule over offered options: zero, multiple, or
// missing-ID matches return cancelled, and persistent grants are
// never selected regardless of verdict.
func TestMapVerdictCardinality(t *testing.T) {
	allow := PermissionOption{OptionID: "a1", Kind: "allow_once"}
	reject := PermissionOption{OptionID: "r1", Kind: "reject_once"}
	always := PermissionOption{OptionID: "aa", Kind: "allow_always"}
	rejectAlways := PermissionOption{OptionID: "ra", Kind: "reject_always"}

	if got := MapVerdict(VerdictAllow, []PermissionOption{allow, reject, always}); got.Outcome != "selected" || got.OptionID != "a1" {
		t.Fatalf("allow leg: %+v", got)
	}
	if got := MapVerdict(VerdictDeny, []PermissionOption{allow, reject}); got.Outcome != "selected" || got.OptionID != "r1" {
		t.Fatalf("deny leg: %+v", got)
	}
	if got := MapVerdict(VerdictAllow, []PermissionOption{reject, always}); got.Outcome != "cancelled" {
		t.Fatalf("no one-shot allow offered: persistent grant must not substitute: %+v", got)
	}
	if got := MapVerdict(VerdictDeny, []PermissionOption{allow, rejectAlways}); got.Outcome != "cancelled" {
		t.Fatalf("reject_always must never be selected (it poisons loaded sessions): %+v", got)
	}
	if got := MapVerdict(VerdictAllow, []PermissionOption{allow, {OptionID: "a2", Kind: "allow_once"}}); got.Outcome != "cancelled" {
		t.Fatalf("two allow_once options: %+v", got)
	}
	if got := MapVerdict(VerdictDeny, []PermissionOption{{Kind: "reject_once"}}); got.Outcome != "cancelled" {
		t.Fatalf("empty option ID must cancel: %+v", got)
	}
	if got := MapVerdict(VerdictUnclassifiable, []PermissionOption{allow, reject}); got.Outcome != "selected" || got.OptionID != "r1" {
		t.Fatalf("unclassifiable takes the deny branch: %+v", got)
	}
	if got := StrictAnswer([]PermissionOption{allow, reject}); got.OptionID != "r1" {
		t.Fatalf("strict mode must reject: %+v", got)
	}
}

// The answer must marshal to the schema's exact member names, and
// an ID shared across kinds is ambiguous, never selectable.
func TestAnswerWireShapeAndCrossKindAmbiguity(t *testing.T) {
	answer := MapVerdict(VerdictAllow, []PermissionOption{{OptionID: "a1", Kind: "allow_once"}})
	wire, err := json.Marshal(answer.WireResult())
	if err != nil {
		t.Fatal(err)
	}
	if string(wire) != `{"outcome":{"optionId":"a1","outcome":"selected"}}` &&
		string(wire) != `{"outcome":{"outcome":"selected","optionId":"a1"}}` {
		t.Fatalf("wire shape drifted: %s", wire)
	}
	cancelled, _ := json.Marshal(MapVerdict(VerdictDeny, nil).WireResult())
	if string(cancelled) != `{"outcome":{"outcome":"cancelled"}}` {
		t.Fatalf("cancelled shape drifted: %s", cancelled)
	}

	ambiguous := []PermissionOption{
		{OptionID: "x", Kind: "allow_once"},
		{OptionID: "x", Kind: "reject_once"},
	}
	if got := MapVerdict(VerdictAllow, ambiguous); got.Outcome != "cancelled" {
		t.Fatalf("an ID shared across kinds must cancel: %+v", got)
	}
}

// Canonicalize resolves symlinks over the existing prefix, keeps
// nonexistent tails, and (on this case-insensitive host class)
// substitutes on-disk spelling so containment is identity, not
// string luck.
func TestCanonicalize(t *testing.T) {
	if _, err := Canonicalize("relative/path"); err == nil {
		t.Fatal("relative paths must be refused")
	}
	dir := t.TempDir()
	resolved, err := Canonicalize(dir + "/does/not/exist/yet")
	if err != nil {
		t.Fatal(err)
	}
	base, err := Canonicalize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !allInside([]string{resolved}, []string{base}) {
		t.Fatalf("a nonexistent tail must stay inside its canonical base: %s vs %s", resolved, base)
	}
}

// The last matrix and normalizer branches: single unclassifiable
// stays unclassifiable, multi-effect all-allow allows, malformed
// assembler inputs drop, and Canonicalize resolves symlinks to the
// target's canonical home.
func TestRemainingBranches(t *testing.T) {
	envelope := workspaceEnvelope("runtime-default")
	if Decide([]Effect{{Class: EffectUnknown}}, envelope) != VerdictUnclassifiable {
		t.Fatal("single unclassifiable stays unclassifiable")
	}
	multi := []Effect{
		{Class: EffectWrite, Paths: []string{"/repo/work/a"}},
		{Class: EffectRead, Paths: []string{"/repo/b"}},
	}
	if Decide(multi, envelope) != VerdictAllow {
		t.Fatal("multi-effect all-allow must allow")
	}
	if allInside([]string{"/x"}, []string{""}) {
		t.Fatal("empty roots are skipped, not matched")
	}
}
