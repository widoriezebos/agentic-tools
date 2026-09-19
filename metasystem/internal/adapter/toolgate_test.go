package adapter

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

func TestToolGateAllowlist(t *testing.T) {
	t.Parallel()
	budget := config.Budget{Trigger: 105000, Ceiling: 250000}
	memoryDir := filepath.Join(t.TempDir(), "memory")
	type allowCase struct {
		name        string
		call        Call
		wantPattern string
		wantKind    ClassificationKind
		allow       bool
	}
	var cases []allowCase
	for index := range toolGateRows {
		row := &toolGateRows[index]
		cases = append(cases, allowCase{
			name:        "row-" + row.pattern,
			call:        toolGateCallForRow(row, memoryDir),
			wantPattern: row.pattern,
			wantKind:    row.kind,
			allow:       true,
		})
	}
	cases = append(cases,
		allowCase{name: "basename", call: bashToolGateCall("metasystem wait --restore job"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "absolute-path-prefix", call: bashToolGateCall("/usr/local/bin/metasystem context handoff --root /repo"), wantPattern: "metasystem context handoff", wantKind: AllowedAtTrigger, allow: true},
		allowCase{name: "relative-path-prefix", call: bashToolGateCall("bin/metasystem context resume"), wantPattern: "metasystem context resume", wantKind: AllowedAtTrigger, allow: true},
		allowCase{name: "dot-path-prefix", call: bashToolGateCall("./bin/metasystem context verify"), wantPattern: "metasystem context verify", wantKind: AllowedAtTrigger, allow: true},
		allowCase{name: "cd-prefix", call: bashToolGateCall("cd /tmp && metasystem context status --root /repo"), wantPattern: "metasystem context status", wantKind: AllowedAtTrigger, allow: true},
		allowCase{name: "assignment-prefix", call: bashToolGateCall("A=one B='two words' metasystem delegate --root /repo"), wantPattern: "metasystem delegate", wantKind: AllowedAtTrigger, allow: true},
		allowCase{name: "bash-script-wrapper", call: bashToolGateCall("bash /repo/scripts/agents/land.sh goal"), wantPattern: "scripts/agents/land.sh", wantKind: NeverDenied, allow: true},
		allowCase{name: "sh-script-wrapper", call: bashToolGateCall("sh ./scripts/agents/land.sh goal"), wantPattern: "scripts/agents/land.sh", wantKind: NeverDenied, allow: true},
		allowCase{name: "bash-c-wrapper", call: bashToolGateCall("bash -c 'metasystem wait --restore goal'"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "bash-lc-wrapper", call: bashToolGateCall("bash -lc 'metasystem steward status --root /repo'"), wantPattern: "metasystem steward status", wantKind: AllowedAtTrigger, allow: true},
		allowCase{name: "pipe-filter", call: bashToolGateCall("metasystem wait | head -1"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "or-filter", call: bashToolGateCall("metasystem wait || tail -1"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "semicolon-filter", call: bashToolGateCall("metasystem wait; grep ready status.txt"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "newline-filter", call: bashToolGateCall("metasystem wait\nwc -l"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "multiple-trailing-filters", call: bashToolGateCall("metasystem wait | grep ready | tail -1 | tee /tmp/status"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "compound-with-another-command", call: bashToolGateCall("metasystem wait && echo done"), wantKind: Other, allow: false},
		allowCase{name: "quoted-separators", call: bashToolGateCall("metasystem wait --label 'a && b || c; d | e\nf'"), wantPattern: "metasystem wait", wantKind: NeverDenied, allow: true},
		allowCase{name: "unparseable-input", call: Call{Tool: "Bash", Input: json.RawMessage(`{"command":`)}, wantKind: Other, allow: false},
	)

	covered := make(map[*toolGateRow]Classification)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			class := Classify(test.call, memoryDir)
			if class.Kind != test.wantKind {
				t.Fatalf("classification kind = %q, want %q", class.Kind, test.wantKind)
			}
			if test.wantPattern == "" {
				if class.Row != nil {
					t.Fatalf("unexpected row %q", class.Row.pattern)
				}
			} else if class.Row == nil || class.Row.pattern != test.wantPattern {
				t.Fatalf("classification row = %#v, want %q", class.Row, test.wantPattern)
			}
			if test.call.Tool == "Bash" && test.allow && class.Command == "" {
				t.Fatal("Bash classification did not retain its matched simple command")
			}
			for _, tokens := range []int64{budget.Trigger, budget.Ceiling} {
				decision := Decide(class, tokens, budget, ReserveReading{Size: 1}, "/repo")
				if decision.Deny == test.allow {
					t.Fatalf("tokens=%d decision=%#v, allow=%t", tokens, decision, test.allow)
				}
			}
			if class.Row != nil && class.Kind != MemoryUnresolved {
				covered[class.Row] = class
			}
		})
	}

	for index := range toolGateRows {
		row := &toolGateRows[index]
		class, ok := covered[row]
		if !ok {
			t.Fatalf("table row %q has no decision-changing case", row.pattern)
		}
		changed := *row
		changed.trigger, changed.ceiling = false, ceilingDeny
		class.Row = &changed
		atTrigger := Decide(class, budget.Trigger, budget, ReserveReading{}, "/repo")
		atCeiling := Decide(class, budget.Ceiling, budget, ReserveReading{}, "/repo")
		if !atTrigger.Deny || !atCeiling.Deny {
			t.Fatalf("flipping row %q did not change both decisions: trigger=%#v ceiling=%#v", row.pattern, atTrigger, atCeiling)
		}
	}
}

func TestToolGateNeverDeniesLandingWaitOrAgent(t *testing.T) {
	t.Parallel()
	budget := config.Budget{Trigger: 105000, Ceiling: 250000}
	for index := range toolGateRows {
		row := &toolGateRows[index]
		t.Run(row.pattern, func(t *testing.T) {
			class := Classify(toolGateCallForRow(row, filepath.Join(t.TempDir(), "memory")), "")
			if row.ceiling == ceilingDenyReserve {
				for _, reserve := range []ReserveReading{{}, {Size: 2, Used: 1}} {
					if decision := Decide(class, budget.Ceiling+500000, budget, reserve, "/repo"); decision.Deny {
						t.Fatalf("reserve decision above ceiling = %#v", decision)
					}
				}
				return
			}
			if row.trigger && row.ceiling == ceilingAllow {
				decision := Decide(class, budget.Ceiling+500000, budget, ReserveReading{Size: 1, Used: 1}, "/repo")
				if decision.Deny {
					t.Fatalf("non-reserve decision above ceiling = %#v", decision)
				}
			}
		})
	}
}

func TestToolGateAllowEmitsNothing(t *testing.T) {
	t.Parallel()
	budget := config.Budget{Trigger: 105000, Ceiling: 250000}
	memoryDir := filepath.Join(t.TempDir(), "memory")
	allows := []Decision{
		Decide(Classify(bashToolGateCall("rm scratch"), memoryDir), budget.Trigger-1, budget, ReserveReading{}, "/repo"),
		Decide(Classify(Call{Tool: "Agent"}, memoryDir), budget.Ceiling, budget, ReserveReading{}, "/repo"),
		Decide(Classify(toolGatePathCall("Write", filepath.Join(memoryDir, "note.md")), memoryDir), budget.Trigger, budget, ReserveReading{}, "/repo"),
		Decide(Classify(Call{Tool: "Edit", Input: json.RawMessage(`{"bad":true}`)}, ""), budget.Ceiling, budget, ReserveReading{}, "/repo"),
	}
	for index, decision := range allows {
		if decision.Deny || decision.Output() != nil {
			t.Fatalf("allow %d emitted %#v from %#v", index, decision.Output(), decision)
		}
	}

	deny := Decide(Classify(bashToolGateCall("rm scratch"), memoryDir), 123456, budget, ReserveReading{}, "/repo")
	wantReason := "CONTEXT AT 123K (trigger 105K): this call is denied; run metasystem context handoff --root /repo alone, or launch a delegate"
	wantOutput := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"` + wantReason + `"}}`
	if !deny.Deny || deny.Reason != wantReason || string(deny.Output()) != wantOutput {
		t.Fatalf("deny = %#v output=%s", deny, deny.Output())
	}
}

func TestToolGateCeilingColumnDiffersOnlyOnReserveRows(t *testing.T) {
	t.Parallel()
	wantReserve := map[string]bool{
		"Agent": true, "Task*": true, "metasystem landing <verb>": true,
		"metasystem goal land-ready": true, "metasystem wait": true,
		"metasystem job watch": true, "scripts/agents/land.sh": true,
		"metasystem delegate": true,
	}
	for _, row := range toolGateRows {
		if got := row.ceiling == ceilingDenyReserve; got != wantReserve[row.pattern] {
			t.Fatalf("row %q reserve cell = %t, want %t", row.pattern, got, wantReserve[row.pattern])
		}
		if !wantReserve[row.pattern] && (row.ceiling == ceilingAllow) != row.trigger {
			t.Fatalf("row %q has trigger=%t ceiling=%d", row.pattern, row.trigger, row.ceiling)
		}
	}
}

func TestSuccessorStartupSequenceAllowedAtEverySize(t *testing.T) {
	t.Parallel()
	budget := config.Budget{Trigger: 105000, Ceiling: 250000, Reserve: 1}
	memoryDir := filepath.Join(t.TempDir(), "memory")
	calls := []Call{
		bashToolGateCall("metasystem context resume --root X"),
		bashToolGateCall("metasystem goal next"),
		bashToolGateCall("metasystem goal claim --root X --id Y --continue-handoff"),
		bashToolGateCall("metasystem context handoff --root X"),
		{Tool: "SendMessage"}, {Tool: "Monitor"}, {Tool: "TaskStop"},
		toolGatePathCall("Write", filepath.Join(memoryDir, "note.md")),
	}
	readings := []ReserveReading{
		{Size: 1, Used: 1},
		{Size: 1, Used: 1, Fault: "reserve-unrecordable"},
		{Size: 1, Used: 1, Fault: "reserve-busy"},
	}
	for _, call := range calls {
		class := Classify(call, memoryDir)
		for _, tokens := range []int64{budget.Trigger, budget.Ceiling, budget.Ceiling + 500000} {
			for _, reserve := range readings {
				if decision := Decide(class, tokens, budget, reserve, "/repo"); decision.Deny {
					t.Fatalf("call=%s tokens=%d reserve=%+v decision=%#v", call.Tool, tokens, reserve, decision)
				}
			}
		}
	}
	plain := Classify(bashToolGateCall("metasystem goal claim --root X --id Y"), memoryDir)
	if plain.Kind != Other || plain.Row != nil {
		t.Fatalf("plain goal claim classification = %#v", plain)
	}
}

func TestToolGateMemoryNoteRule(t *testing.T) {
	t.Parallel()
	budget := config.Budget{Trigger: 105000, Ceiling: 250000}
	root := t.TempDir()
	memoryDir := filepath.Join(root, "memory", ".")
	for _, tool := range []string{"Write", "Edit"} {
		t.Run(tool+"-inside", func(t *testing.T) {
			class := Classify(toolGatePathCall(tool, filepath.Join(memoryDir, "handoff", "note.md")), memoryDir)
			if class.Kind != MemoryNote || class.Row == nil {
				t.Fatalf("inside classification = %#v", class)
			}
			for _, tokens := range []int64{budget.Trigger, budget.Ceiling} {
				decision := Decide(class, tokens, budget, ReserveReading{}, "/repo")
				if decision.Deny || decision.Cause != "memory-note" {
					t.Fatalf("tokens=%d inside decision = %#v", tokens, decision)
				}
			}
		})

		for name, path := range map[string]string{
			"outside": filepath.Join(root, "elsewhere", "note.md"),
			"escape":  filepath.Clean(memoryDir) + string(filepath.Separator) + ".." + string(filepath.Separator) + "escaped.md",
			"sibling": filepath.Clean(memoryDir) + "-extra" + string(filepath.Separator) + "note.md",
		} {
			t.Run(tool+"-"+name, func(t *testing.T) {
				class := Classify(toolGatePathCall(tool, path), memoryDir)
				decision := Decide(class, budget.Trigger, budget, ReserveReading{}, "/repo")
				if class.Kind != Other || !decision.Deny {
					t.Fatalf("outside classification=%#v decision=%#v", class, decision)
				}
			})
		}

		t.Run(tool+"-unresolved", func(t *testing.T) {
			class := Classify(Call{Tool: tool, Input: json.RawMessage(`not-json`)}, "")
			if class.Kind != MemoryUnresolved || class.Row == nil {
				t.Fatalf("unresolved classification = %#v", class)
			}
			for _, tokens := range []int64{budget.Trigger, budget.Ceiling} {
				decision := Decide(class, tokens, budget, ReserveReading{}, "/repo")
				if decision.Deny || decision.Cause != "memory-unresolved" {
					t.Fatalf("tokens=%d unresolved decision = %#v", tokens, decision)
				}
			}
		})
	}
}

func toolGateCallForRow(row *toolGateRow, memoryDir string) Call {
	switch row.match {
	case matchTool:
		return Call{Tool: row.words[0]}
	case matchToolPrefix:
		return Call{Tool: row.words[0] + "Create"}
	case matchCommand, matchCommandFlag:
		command := strings.ReplaceAll(strings.Join(row.words, " "), "<verb>", "finish")
		if row.match == matchCommandFlag {
			command += " " + row.flag
		}
		return bashToolGateCall(command)
	case matchLandingScript:
		return bashToolGateCall("./scripts/agents/land.sh goal")
	case matchMemoryPath:
		return toolGatePathCall(row.words[0], filepath.Join(memoryDir, "note.md"))
	default:
		return Call{}
	}
}

func bashToolGateCall(command string) Call {
	encoded, _ := json.Marshal(map[string]string{"command": command})
	return Call{Tool: "Bash", Input: encoded}
}

func toolGatePathCall(tool, path string) Call {
	encoded, _ := json.Marshal(map[string]string{"file_path": path})
	return Call{Tool: tool, Input: encoded}
}
