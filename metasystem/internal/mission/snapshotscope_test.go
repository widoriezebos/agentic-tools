package mission

import (
	"strings"
	"testing"
)

func goodPosture() map[string]any {
	return map[string]any{
		"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
		"stagedTreePost": strings.Repeat("a", 40), "topTreePost": nil, "topStagedPost": nil,
		"worktreeCensusPost": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
	}
}

func TestValidateStagedPosture(t *testing.T) {
	if err := ValidateStagedPosture(nil); err != nil {
		t.Fatalf("nil posture is lawful: %v", err)
	}
	good := map[string]any{"tree": strings.Repeat("a", 40), "unmerged": []any{"100644 x 1\tp"}}
	if err := ValidateStagedPosture(good); err != nil {
		t.Fatalf("good posture: %v", err)
	}
	bad := []any{
		"not-an-object",
		map[string]any{"tree": strings.Repeat("a", 40)},                  // missing unmerged
		map[string]any{"tree": "nothex", "unmerged": []any{}},            // bad tree
		map[string]any{"tree": strings.Repeat("a", 40), "unmerged": "x"}, // unmerged not array
		map[string]any{"tree": strings.Repeat("a", 40), "unmerged": []any{123}},
		map[string]any{"tree": strings.Repeat("a", 40), "unmerged": []any{""}},
	}
	for i, b := range bad {
		if err := ValidateStagedPosture(b); err == nil {
			t.Fatalf("bad posture %d accepted", i)
		}
	}
}

func TestValidateRefMap(t *testing.T) {
	if err := ValidateRefMap(map[string]any{"refs/heads/main": strings.Repeat("a", 40)}); err != nil {
		t.Fatalf("good ref map: %v", err)
	}
	for i, b := range []any{
		"not-object",
		map[string]any{"": strings.Repeat("a", 40)},
		map[string]any{"refs/x": "nothex"},
	} {
		if err := ValidateRefMap(b); err == nil {
			t.Fatalf("bad ref map %d accepted", i)
		}
	}
}

func TestValidateWorktreeCensus(t *testing.T) {
	good := []any{map[string]any{
		"path": "/w", "headOid": strings.Repeat("a", 40), "branch": "refs/heads/main",
		"detached": false, "bare": false, "prunable": false, "postureReadable": true,
		"pseudorefs": []any{map[string]any{"name": "ORIG_HEAD", "oids": []any{strings.Repeat("a", 40)}, "parseable": true}},
		"staged":     map[string]any{"tree": strings.Repeat("a", 40), "unmerged": []any{}},
	}}
	if err := ValidateWorktreeCensus(good); err != nil {
		t.Fatalf("good census: %v", err)
	}
	base := func() map[string]any {
		return map[string]any{
			"path": "/w", "headOid": strings.Repeat("a", 40), "branch": "refs/heads/main",
			"detached": false, "bare": false, "prunable": false, "postureReadable": true,
			"pseudorefs": []any{}, "staged": nil,
		}
	}
	mutators := []func(map[string]any){
		func(m map[string]any) { m["path"] = "" },
		func(m map[string]any) { m["headOid"] = "nothex" },
		func(m map[string]any) { delete(m, "branch") },
		func(m map[string]any) { m["detached"] = "no" },
		func(m map[string]any) { m["pseudorefs"] = "x" },
		func(m map[string]any) {
			m["pseudorefs"] = []any{map[string]any{"name": "", "oids": []any{}, "parseable": true}}
		},
		func(m map[string]any) {
			m["pseudorefs"] = []any{map[string]any{"name": "X", "oids": []any{"bad"}, "parseable": true}}
		},
		func(m map[string]any) { m["staged"] = map[string]any{"tree": "bad", "unmerged": []any{}} },
	}
	for i, mut := range mutators {
		rec := base()
		mut(rec)
		if err := ValidateWorktreeCensus([]any{rec}); err == nil {
			t.Fatalf("bad census %d accepted", i)
		}
	}
	if err := ValidateWorktreeCensus("not-array"); err == nil {
		t.Fatal("non-array census accepted")
	}
}

func TestValidateRecordedPosture(t *testing.T) {
	if err := ValidateRecordedPosture(goodPosture(), "x"); err != nil {
		t.Fatalf("good posture: %v", err)
	}
	if err := ValidateRecordedPosture(nil, "x"); err == nil {
		t.Fatal("nil posture accepted")
	}
	for name, mut := range map[string]func(map[string]any){
		"headCommitPost": func(m map[string]any) { m["headCommitPost"] = "bad" },
		"stagedTreePost": func(m map[string]any) { m["stagedTreePost"] = "bad" },
		"topTreePost":    func(m map[string]any) { m["topTreePost"] = "bad" },
		"refMapPost":     func(m map[string]any) { m["refMapPost"] = "bad" },
		"capturedAt":     func(m map[string]any) { m["capturedAt"] = "not-a-time" },
		"extra":          func(m map[string]any) { m["surprise"] = true },
	} {
		p := goodPosture()
		mut(p)
		if err := ValidateRecordedPosture(p, "x"); err == nil {
			t.Fatalf("bad posture (%s) accepted", name)
		}
	}
}

func TestUnverifiedAcceptance(t *testing.T) {
	acc := func(turn string) map[string]any {
		return map[string]any{"turnId": turn, "wall": map[string]any{"verdict": "passed"}}
	}
	ver := func(turn string) map[string]any {
		return map[string]any{"turnId": turn, "kind": WallVerificationKind}
	}
	cases := []struct {
		name string
		log  []any
		want string
	}{
		{"empty", []any{}, ""},
		{"pending", []any{acc("t1")}, "t1"},
		{"verified", []any{acc("t1"), ver("t1")}, ""},
		{"verification-before-acceptance-is-not-a-conclusion", []any{ver("t1"), acc("t1")}, "t1"},
	}
	for _, tc := range cases {
		if got := UnverifiedAcceptance(map[string]any{"turnLog": tc.log}); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestRecordableRefMapOmitsMissionNamespace(t *testing.T) {
	refs := map[string]string{
		"refs/heads/main": strings.Repeat("a", 40),
		"refs/metasystem/missions/demo/state-anchors": strings.Repeat("b", 40),
	}
	doc := RecordableRefMap(refs, "demo")
	if _, ok := doc["refs/heads/main"]; !ok {
		t.Fatal("ordinary ref dropped")
	}
	if _, ok := doc["refs/metasystem/missions/demo/state-anchors"]; ok {
		t.Fatal("mission-namespace ref must be omitted")
	}
}
