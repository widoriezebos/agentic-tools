package missionrunner

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// A repeated resolution is recognized by its substance: variant, tree for a
// restore, the waived claims for an adoption. Who ruled and why do not make
// another resolution; anything else does (R-129-ui).
func TestSameTaintResolution(t *testing.T) {
	t.Parallel()
	tree := strings.Repeat("a", 40)
	restore := map[string]any{"variant": "restore", "treeId": tree, "resolvedBy": "Wido"}
	adopt := map[string]any{"variant": "adopt-disputed-tree", "treeId": strings.Repeat("b", 40), "waivedClaims": []any{"one", "two"}}
	for _, test := range []struct {
		name     string
		recorded map[string]any
		variant  string
		tree     string
		waived   []string
		want     bool
	}{
		{"the same restore", restore, "restore", tree, nil, true},
		{"a restore to another tree", restore, "restore", strings.Repeat("c", 40), nil, false},
		{"an adoption of a restored taint", restore, "adopt-disputed-tree", "", []string{"one"}, false},
		{"the same adoption in another order", adopt, "adopt-disputed-tree", "", []string{"two", " one "}, true},
		{"an adoption waiving other claims", adopt, "adopt-disputed-tree", "", []string{"one", "three"}, false},
		{"an adoption waiving fewer claims", adopt, "adopt-disputed-tree", "", []string{"one"}, false},
		{"a restore of an adopted taint", adopt, "restore", strings.Repeat("b", 40), nil, false},
	} {
		if got := sameTaintResolution(test.recorded, test.variant, test.tree, test.waived); got != test.want {
			t.Errorf("%s: sameTaintResolution = %v, want %v", test.name, got, test.want)
		}
	}
}

// A start or resume of a mission whose runner is live is success naming it,
// and it writes, arms and spawns nothing (R-129-ui). The live runner is this
// test process, with a tag its own command line carries, exactly as
// TestCleanupStaleLease proves a live holder.
func TestLaunchOverALiveRunnerIsAlreadyRunning(t *testing.T) {
	t.Parallel()
	self := os.Getpid()
	selfCommand := processCommand(self, fixtureauth.CommandProbe{})
	tag := selfCommand[strings.LastIndex(selfCommand, "/")+1:]
	if index := strings.Index(tag, " "); index > 0 {
		tag = tag[:index]
	}
	if tag == "" {
		t.Skip("own argv unreadable on this host")
	}
	engine := &Engine{Root: t.TempDir(), Mission: "mr-live"}
	dir := engine.missionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(dir, "state.json"), `{"status":"running"}`)
	writeText(t, filepath.Join(dir, "lease.json"), `{"missionId":"mr-live","pid":`+strconv.Itoa(self)+`,"pgid":1,"instanceTag":"`+tag+`","startedAt":"x","renewedAt":"x"}`)
	armed := 0
	engine.ArmSupervision = func([]string) (verbresult.Result, error) {
		armed++
		return verbresult.Result{Outcome: verbresult.Unknown}, errors.New("up printed no readable result")
	}
	for _, mode := range []string{"start", "resume", "resume"} {
		err := engine.launch(mode, false)
		var running *alreadyRunning
		if !errors.As(err, &running) || !strings.HasPrefix(err.Error(), AlreadyRunningPrefix+"mr-live") {
			t.Fatalf("%s over a live runner = %v, want already running", mode, err)
		}
	}
	if armed != 0 {
		t.Fatalf("an already running mission was armed %d times", armed)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if strings.Join(names, ",") != "lease.json,lease.lock,state.json" {
		t.Fatalf("an already running mission's directory changed: %v", names)
	}
}
