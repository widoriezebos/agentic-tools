package plain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func TestReadImpactPlanHashTracksContentAcrossRuns(t *testing.T) {
	t.Parallel()
	selection := "internal/a=TestA"
	pretty := false
	seams := ProveSeams{
		Executable: func() (string, error) { return "/fixture/engine", nil },
		Command: func(command *exec.Cmd) error {
			if !reflect.DeepEqual(command.Args[1:], []string{"test", "impact", "--plan", "--json", "--base", "base"}) || command.Dir != "fixture" {
				t.Fatalf("plan command: %v dir=%q", command.Args, command.Dir)
			}
			plan := ImpactPlan{Base: "base", Subject: "batch base", Selections: []string{selection}}
			result := verbresult.FromError("test impact", 0, nil, plan)
			result.Summary = fmt.Sprintf("format %v", pretty)
			data, err := json.Marshal(result)
			if pretty {
				// Both data and envelope whitespace vary, as does envelope metadata.
				var indented bytes.Buffer
				err = json.Indent(&indented, data, "", "  ")
				data = indented.Bytes()
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := command.Stdout.Write(data); err != nil {
				t.Fatal(err)
			}
			ended := exec.Command("/usr/bin/true")
			if err := ended.Run(); err != nil {
				t.Fatal(err)
			}
			command.ProcessState = ended.ProcessState
			return nil
		},
	}
	hash := func() string {
		t.Helper()
		decision := scopeDecision{scopeRecord: scopeRecord{BaseCommit: "base"}}
		if err := decision.impactPlan(seams, "fixture"); err != nil || decision.PlanHash == "" {
			t.Fatalf("plan hash=%q error=%v", decision.PlanHash, err)
		}
		return decision.PlanHash
	}
	first := hash()
	if second := hash(); second != first {
		t.Fatalf("same plan changed hash: %s / %s", first, second)
	}
	pretty = true
	if formatted := hash(); formatted != first {
		t.Fatalf("formatting or summary changed hash: %s / %s", first, formatted)
	}
	selection = "internal/a=TestB"
	if changed := hash(); changed == first {
		t.Fatalf("changed selection kept hash %s", first)
	}
}

func TestReadImpactPlanRejectsUnreadableEnvelope(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, text string }{
		{"human text", "plan: base base\nselection: internal/a\n"},
		{"missing envelope", ""},
		{"null plan", `{"schemaVersion":1,"verb":"test impact","outcome":"confirmed","data":null}`},
		{"unknown plan field", `{"schemaVersion":1,"verb":"test impact","outcome":"confirmed","data":{"base":"base","subject":"","selections":[],"unknown":true}}`},
		{"outcome contradicts exit", `{"schemaVersion":1,"verb":"test impact","outcome":"refused"}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			seams := ProveSeams{
				Executable: func() (string, error) { return "/fixture/engine", nil },
				Command: func(command *exec.Cmd) error {
					fmt.Fprint(command.Stdout, row.text)
					ended := exec.Command("/usr/bin/true")
					if err := ended.Run(); err != nil {
						t.Fatal(err)
					}
					command.ProcessState = ended.ProcessState
					return nil
				},
			}
			decision := scopeDecision{scopeRecord: scopeRecord{BaseCommit: "base"}}
			if err := decision.impactPlan(seams, "fixture"); err == nil || decision.PlanHash != "" || !strings.Contains(err.Error(), "impact") {
				t.Fatalf("invalid plan authorized hash=%q error=%v", decision.PlanHash, err)
			}
		})
	}
}
