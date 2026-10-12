package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingHealthTextJSONAndReadOnly(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	b.cwd = b.checkout
	for _, args := range [][]string{{"landing", "health"}, {"landing", "health", "--json"}, {"landing", "status", "--json"}} {
		before := idemTreeDigest(t, b.installation)
		code, text := b.run(t, args...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, text)
		}
		if args[len(args)-1] != "--json" {
			for _, group := range []string{"lane idle; nothing fires", "goals:", "batch:", "agent:", "questions:", "incidents:", "engine:", "proofs:", "load:"} {
				if !strings.Contains(text, group) {
					t.Fatalf("missing %s: %s", group, text)
				}
			}
		} else {
			var result struct{ Data json.RawMessage }
			helmMust(t, json.Unmarshal([]byte(text), &result))
			if strings.Contains(string(result.Data), `"running_load"`) {
				t.Fatal("health includes an unrecorded running load")
			}
			var h plain.Health
			if args[1] == "status" {
				var status plain.Status
				helmMust(t, json.Unmarshal(result.Data, &status))
				if status.Health == nil {
					t.Fatal("status omitted health")
				}
				h = *status.Health
			} else {
				helmMust(t, json.Unmarshal(result.Data, &h))
			}
			if h.State != "idle" || h.Load.AtStart != nil || h.Load.AtEnd != nil {
				t.Fatalf("health: %+v", h)
			}
		}
		idemSameTree(t, "read health", before, idemTreeDigest(t, b.installation))
	}
	fix := plain.Fix{Attempt: "resolution", Goal: "g", Commit: b.main, Tip: "missing", State: "resolving", Units: []string{"unit"}, Paths: []conflict.Path{{Path: "x"}}}
	helmMust(t, plain.WriteFix(b.installation, &fix))
	data, err := json.Marshal(plain.Line{Goal: "g", SHA: strings.Repeat("a", 40), At: laneTestNow.Add(-95 * time.Minute).Format(time.RFC3339)})
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "queue.jsonl"), append(data, '\n'), 0600))
	for _, args := range [][]string{{"landing", "health"}, {"landing", "health", "--json"}} {
		before, home := idemTreeDigest(t, b.checkout), idemTreeDigest(t, b.home)
		code, out := b.run(t, args...)
		if code != 0 || !strings.Contains(out, "abandoned-resolution") || !strings.Contains(out, "overdue-goal") {
			t.Fatalf("abandoned health: %d %s", code, out)
		}
		idemSameTree(t, "abandoned resolution health", before, idemTreeDigest(t, b.checkout))
		idemSameTree(t, "abandoned resolution health home", home, idemTreeDigest(t, b.home))
	}
	command, ok := findIntentAction("landing", "health")
	if !ok || !slices.Contains(command.examples, "metasystem landing health --json") {
		t.Fatal("missing health coverage example")
	}
}

func TestLandingHealthCLIRealEntrypoint(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	b.cwd = b.checkout
	interventions, err := os.ReadFile("testdata/landing-steward-interventions.jsonl")
	helmMust(t, err)
	helmMust(t, os.MkdirAll(plain.Dir(b.installation), 0755))
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "interventions.jsonl"), interventions, 0600))
	engine := os.Getenv("METASYSTEM_WAIT_BINARY")
	if engine == "" {
		t.Fatal("the package did not build the CLI")
	}
	for _, args := range [][]string{{"landing", "health"}, {"landing", "health", "--json"}, {"landing", "health", "unexpected"}} {
		cmd := exec.CommandContext(t.Context(), engine, args...)
		cmd.Dir = b.checkout
		cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
		out, err := cmd.CombinedOutput()
		if args[len(args)-1] == "unexpected" {
			if err == nil {
				t.Fatalf("invalid arguments accepted: %s", out)
			}
			continue
		}
		if err != nil || !strings.Contains(string(out), "nothing fires") {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		if !strings.Contains(string(out), "redeploy the engine from main") || !strings.Contains(string(out), "deployed") {
			t.Fatalf("CLI omitted the intervention: %s", out)
		}
	}
	now := time.Now()
	idle := struct {
		EndedAt string           `json:"ended_at"`
		Step    *plain.AgentStep `json:"step"`
	}{EndedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Step: &plain.AgentStep{Verb: "work review", StartedAt: now.Add(-50 * time.Minute).Format(time.RFC3339Nano)}}
	data, err := json.Marshal(idle)
	helmMust(t, err)
	helmMust(t, os.MkdirAll(plain.Dir(b.installation), 0755))
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "agent-idle.json"), data, 0600))
	cmd := exec.CommandContext(t.Context(), engine, "landing", "health", "--json")
	cmd.Dir = b.checkout
	cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
	out, err := cmd.CombinedOutput()
	helmMust(t, err)
	var result struct{ Data plain.Health }
	helmMust(t, json.Unmarshal(out, &result))
	sampled, err := time.Parse(time.RFC3339Nano, result.Data.Load.Now.At)
	helmMust(t, err)
	ended, err := time.Parse(time.RFC3339Nano, idle.EndedAt)
	helmMust(t, err)
	if result.Data.Agent.Minutes == nil || *result.Data.Agent.Minutes != sampled.Sub(ended).Minutes() || result.Data.Agent.Verb != "work review" {
		t.Fatalf("CLI used the finished step's start for inactivity: %s", out)
	}
	helmMust(t, plain.WriteFix(b.installation, &plain.Fix{Attempt: "resolution", Goal: "g", Commit: b.main, Tip: "missing", State: "resolving", Units: []string{"unit"}, Paths: []conflict.Path{{Path: "x"}}}))
	for _, args := range [][]string{{"landing", "health"}, {"landing", "health", "--json"}} {
		before, home := idemTreeDigest(t, b.checkout), idemTreeDigest(t, b.home)
		cmd := exec.CommandContext(t.Context(), engine, args...)
		cmd.Dir = b.checkout
		cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "abandoned-resolution") {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		idemSameTree(t, "CLI health checkout", before, idemTreeDigest(t, b.checkout))
		idemSameTree(t, "CLI health home", home, idemTreeDigest(t, b.home))
	}
}

func TestLandingHealthRealContractAndPlan(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	b.cwd = b.checkout
	now := laneTestNow
	duration := 40.
	rows := []plain.Result{}
	for i := 0; i < 5; i++ {
		rows = append(rows, plain.Result{Attempt: time.Duration(i).String(), Commit: b.main, Scope: "full", Result: plain.Green, Minutes: &duration})
	}
	for _, r := range rows {
		data, err := json.Marshal(r)
		helmMust(t, err)
		f, err := os.OpenFile(filepath.Join(plain.Dir(b.installation), "results.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if os.IsNotExist(err) {
			helmMust(t, os.MkdirAll(plain.Dir(b.installation), 0755))
			f, err = os.OpenFile(filepath.Join(plain.Dir(b.installation), "results.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		}
		helmMust(t, err)
		_, err = f.Write(append(data, '\n'))
		helmMust(t, err)
		helmMust(t, f.Close())
	}
	b.owners.landing.plainProve.Now = func() time.Time { return now }
	code, out := b.run(t, "landing", "health", "--json")
	if code != 0 {
		t.Fatal(out)
	}
	var result struct{ Data plain.Health }
	helmMust(t, json.Unmarshal([]byte(out), &result))
	if result.Data.Proofs.Median == nil || *result.Data.Proofs.Median != 40 {
		t.Fatal(out)
	}
}
