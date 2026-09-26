package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func readHelpJSON(t *testing.T, words ...string) (intentResult, intentHelpDocument) {
	t.Helper()
	args := append([]string{"help"}, words...)
	code, output, problem := runCLIHelp(args, families())
	if code != 0 || problem != "" {
		t.Fatalf("%v = %d, %s", args, code, problem)
	}
	var envelope intentResult
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != 1 || envelope.Verb != "help" || envelope.Outcome != intentConfirmed || envelope.Targets == nil {
		t.Fatalf("bad help envelope: %+v", envelope)
	}
	data, err := json.Marshal(envelope.Data)
	if err != nil {
		t.Fatal(err)
	}
	var document intentHelpDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return envelope, document
}

func TestIntentAgentHelpCatalogue(t *testing.T) {
	t.Parallel()
	code, page, problem := runCLIHelp([]string{"help", "agent"}, families())
	if code != 0 || problem != "" {
		t.Fatalf("agent help = %d %s", code, problem)
	}
	listed := map[string]int{}
	for _, line := range strings.Split(page, "\n") {
		words := strings.Fields(line)
		if strings.HasPrefix(line, "  ") && len(words) >= 2 && strings.HasPrefix(words[1], "[") {
			listed[words[0]]++
		}
	}
	for _, command := range publicIntentCommands() {
		if listed[command.name] != 1 {
			t.Errorf("public %s listed %d times", command.name, listed[command.name])
		}
		delete(listed, command.name)
	}
	if len(listed) != 0 {
		t.Errorf("nonpublic commands: %v", listed)
	}
	for _, wanted := range []string{"approve", "budget", "accept-risk", "help human", "help review FORM", "--json", "--check"} {
		if !strings.Contains(page, wanted) {
			t.Errorf("agent help omits %q", wanted)
		}
	}
	for _, forbidden := range []string{"metasystem internal", "--fixture-human-authority", "--lineage", "metasystem goal list"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("agent help exposes %s", forbidden)
		}
	}
}

func TestIntentAdministrationHelp(t *testing.T) {
	t.Parallel()
	_, index := readHelpJSON(t, "administration", "--json")
	listed := map[string]bool{}
	for _, entry := range index.Commands {
		listed[entry.Name] = true
		if entry.Scope != "administration" && entry.Scope != "mixed" {
			t.Errorf("administration index includes workflow-only %s", entry.Name)
		}
	}
	for _, name := range []string{"enroll", "settings", "restart", "start", "stop", "status", "check", "repair", "land"} {
		if !listed[name] {
			t.Errorf("administration index omits %s", name)
		}
	}
	for _, name := range []string{"build", "review", "wait", "incidents"} {
		if listed[name] {
			t.Errorf("application work %s is misclassified as administration", name)
		}
	}
	for _, words := range [][]string{{"help", "all"}, {"help", "human"}, {"help", "administration"}} {
		code, page, problem := runCLIHelp(words, families())
		heading := strings.Index(page, "MetaSystem administration (tool setup and maintenance)")
		if code != 0 || problem != "" || heading < 0 {
			t.Fatalf("%v does not separate administration: %d %s", words, code, problem)
		}
		for _, name := range []string{"enroll", "settings", "restart"} {
			if at := strings.Index(page, "metasystem "+name+" "); at < heading {
				t.Errorf("%v lists %s outside administration", words, name)
			}
		}
	}
	for _, row := range []struct{ name, work, admin string }{
		{"start", "metasystem start session", "metasystem start [checkout]"},
		{"stop", "metasystem stop job J", "metasystem stop [checkout]"},
		{"status", "metasystem status G", "metasystem status ui"},
		{"repair", "metasystem repair review G", "metasystem repair goals --upgrade"},
	} {
		_, doc := readHelpJSON(t, row.name, "--json")
		if doc.Command.Scope != "mixed" || !strings.Contains(strings.Join(doc.Command.Usage, "\n"), row.work) ||
			!strings.Contains(strings.Join(doc.Command.AdministrationUsage, "\n"), row.admin) {
			t.Errorf("%s loses the workflow/administration distinction: %+v", row.name, doc.Command)
		}
		code, page, problem := runCLIHelp([]string{"help", row.name}, families())
		heading := strings.Index(page, "MetaSystem administration:")
		work, admin := strings.Index(page, row.work), strings.Index(page, row.admin)
		if code != 0 || problem != "" || work < 0 || work >= heading || admin <= heading {
			t.Errorf("%s text does not separate forms: %d %d %d / %s", row.name, work, heading, admin, problem)
		}
	}
	for _, command := range publicIntentCommands() {
		seen := map[string]bool{}
		for _, usage := range command.allUsage() {
			if seen[usage] {
				t.Errorf("%s repeats a form across sections: %s", command.name, usage)
			}
			seen[usage] = true
		}
	}
}

func TestIntentHelpJSON(t *testing.T) {
	t.Parallel()
	_, root := readHelpJSON(t, "--json")
	for _, words := range [][]string{{"agent", "--json"}, {"--json", "all"}, {"--json", "agent", "--json"}} {
		_, page := readHelpJSON(t, words...)
		if !reflect.DeepEqual(page.Commands, root.Commands) || !reflect.DeepEqual(page.Protocol, root.Protocol) {
			t.Errorf("%v is an incomplete root catalogue", words)
		}
	}
	if len(root.Commands) != len(publicIntentCommands()) {
		t.Fatalf("index has %d commands", len(root.Commands))
	}
	for _, entry := range root.Commands {
		_, page := readHelpJSON(t, entry.HelpArgv[2:]...)
		command, ok := findIntentCommand(entry.Name)
		if !ok || page.Command == nil {
			t.Fatalf("invalid index entry %+v", entry)
		}
		if !reflect.DeepEqual(page.Command.Usage, command.usage) || !reflect.DeepEqual(page.Command.AdministrationUsage, command.administrationUsage) || !reflect.DeepEqual(page.Command.Details, command.details) {
			t.Errorf("%s differs from its command definition", entry.Name)
		}
		seen := map[string]bool{}
		for _, option := range page.Command.Options {
			definition, ok := command.lookupFlag(option.Name)
			if !ok || definition.hidden || seen[option.Name] {
				t.Errorf("%s exposes invalid option %+v", entry.Name, option)
			}
			if option.Value != definition.value || option.Rest != definition.rest || option.Repeat != definition.repeat || option.Description != definition.usage || !reflect.DeepEqual(option.Aliases, definition.aliases) {
				t.Errorf("%s --%s differs from parser definition", entry.Name, option.Name)
			}
			seen[option.Name] = true
		}
		for _, definition := range command.allFlags() {
			if seen[definition.name] == definition.hidden {
				t.Errorf("%s --%s visibility mismatch", entry.Name, definition.name)
			}
		}
	}
	for _, topic := range []string{"work", "questions", "operations", "human"} {
		_, page := readHelpJSON(t, topic, "--json")
		if len(page.Commands) == 0 {
			t.Errorf("empty topic %s", topic)
		}
		for _, entry := range page.Commands {
			if (topic == "human" && entry.Audience == "agent") || (topic != "human" && entry.Group != topic) {
				t.Errorf("wrong topic entry: %s %+v", topic, entry)
			}
		}
	}
	_, goals := readHelpJSON(t, "goals", "--json")
	if goals.Command == nil || goals.Command.Name != "goals" {
		t.Fatal("command/topic collision no longer prefers the command")
	}
}

func TestIntentHelpRefusals(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"help", "--bad", "--json"}, {"help", "--json", "internal"},
		{"help", "launch", "--json"}, {"help", "close", "--json"},
		{"help", "review", "unknown", "--json"}, {"help", "review", "design", "extra", "--json"},
		{"help", "review", "--changes", "--json"}, {"help", "all", "extra", "--json"},
		{"help", "no-such-command", "--json"}, {"help", "review", "design", "extra"},
		{"help", "review", "unknown"}, {"help", "--bad"},
	} {
		code, output, problem := runCLIHelp(args, families())
		if code != 2 {
			t.Fatalf("%v = %d", args, code)
		}
		if slices.Contains(args, "--json") {
			var result intentResult
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("%v: %v: %s", args, err, output)
			}
			if result.Outcome != intentRefused || result.Verb != "help" || result.Next == nil || problem != "" {
				t.Errorf("%v has invalid refusal: %+v / %s", args, result, problem)
			}
		} else if output != "" || !strings.Contains(problem, "usage: metasystem help") {
			t.Errorf("text refusal %v = %q %q", args, output, problem)
		}
	}
}

func TestIntentHelpNeverExecutes(t *testing.T) {
	t.Parallel()
	registered := []family{{name: "sentinel", summary: "read help safely", verbs: []verb{{name: "mutate", summary: "must not run", run: func([]string) int {
		panic("help executed a command")
	}}}}}
	for _, args := range [][]string{
		{"help", "--json"}, {"help", "agent"}, {"help", "review", "submit"},
		{"help", "review", "design", "--json"}, {"help", "open", "--intent", "mutate"},
		{"help", "sentinel"}, {"help", "sentinel", "mutate"}, {"help", "sentinel", "--json"},
	} {
		var output, problem bytes.Buffer
		dispatchWithFamiliesAndRepositoryTop(args, &output, &problem, registered, func(string) (string, error) {
			panic("help resolved a repository")
		})
	}
}

func TestIntentReviewHelpForms(t *testing.T) {
	t.Parallel()
	_, review := readHelpJSON(t, "review", "--json")
	command, _ := findIntentCommand("review")
	expected := map[string][]string{
		"goal": {reviewGoalUsage, reviewExplicitGoalUsage}, "submit": {reviewSubmitUsage}, "finding": {reviewFindingUsage},
		"design": {reviewDesignUsage}, "job": {reviewJobUsage}, "commit": {reviewCommitUsage}, "run": {reviewRunUsage}, "changes": {reviewChangesUsage}, "diff": {reviewDiffUsage},
	}
	if len(review.Command.Forms) != len(expected) {
		t.Fatalf("forms: %v", review.Command.Forms)
	}
	for _, entry := range review.Command.Forms {
		_, page := readHelpJSON(t, entry.HelpArgv[2:]...)
		form := page.Form
		if form == nil || !reflect.DeepEqual(form.Usage, expected[entry.Name]) {
			t.Fatalf("wrong usage for %s: %+v", entry.Name, form)
		}
		code, text, problem := runCLIHelp([]string{"help", "review", entry.Name}, families())
		if code != 0 || problem != "" {
			t.Fatalf("focused text %s failed: %d %s", entry.Name, code, problem)
		}
		for _, section := range []string{form.Purpose, form.Effects, form.Authority, form.Repetition, "inputs:", "options:", "example:"} {
			if section == "" || !strings.Contains(text, section) {
				t.Errorf("focused text %s omits %q", entry.Name, section)
			}
		}
		input, err := parseIntentArgs(command, form.ExampleArgv[2:])
		if err != nil {
			t.Fatalf("example cannot parse: %v: %+v", form.ExampleArgv, err)
		}
		options := map[string]bool{}
		descriptions := map[string]string{}
		valueNames := map[string]string{}
		for _, option := range form.Options {
			definition, ok := command.lookupFlag(option.Name)
			if !ok || definition.hidden {
				t.Fatalf("invalid form option %s", option.Name)
			}
			options[option.Name] = true
			descriptions[option.Name] = option.Description
			valueNames[option.Name] = option.Value
		}
		if !options["repo"] || !options["json"] {
			t.Errorf("%s lacks common options", entry.Name)
		}
		switch entry.Name {
		case "changes", "diff":
			if input.args[0] != entry.Name || input.has("changes") || input.has("patch") {
				t.Errorf("diagnostic example submits: %v", form.ExampleArgv)
			}
			for _, flag := range []string{"changes", "patch", "work", "dispositions", "finding", "test", "model", "tool-calls", "after"} {
				if options[flag] {
					t.Errorf("diagnostic %s advertises --%s", entry.Name, flag)
				}
			}
			if !strings.Contains(form.Effects, "Does not commit, publish work") || !options["brief"] || !options["retry"] {
				t.Errorf("diagnostic boundary missing for %s", entry.Name)
			}
			if strings.Contains(descriptions["brief"], "commit review") || !strings.Contains(descriptions["retry"], "feedback attempt") {
				t.Errorf("diagnostic option descriptions refer to another mode: %v", descriptions)
			}
		case "submit":
			if valueNames["after"] != "COMMIT" {
				t.Errorf("submission correction names a commit, got --after %s", valueNames["after"])
			}
			if !reflect.DeepEqual(input.args, []string{"goal", "G"}) || !input.switched("changes") || !input.has("brief") || options["retry"] {
				t.Errorf("submission example/options: %+v %+v", input, options)
			}
			if !strings.Contains(form.Effects, "commits") || !strings.Contains(form.Effects, "publishes") {
				t.Error("submission does not disclose its effects")
			}
		case "design":
			if options["model"] || !options["after"] || !options["retry"] {
				t.Error("design help misstates model or retry options")
			}
		}
	}
}

func TestIntentAgentResultProtocol(t *testing.T) {
	t.Parallel()
	_, page := readHelpJSON(t, "agent", "--json")
	if page.Protocol == nil {
		t.Fatal("no result protocol")
	}
	code, text, _ := runCLIHelp([]string{"help", "agent"}, families())
	if code != 0 {
		t.Fatal(code)
	}
	for _, instruction := range page.Protocol.Instructions {
		if !strings.Contains(text, instruction) {
			t.Errorf("text and structured protocol disagree: %s", instruction)
		}
	}
	for _, wanted := range []string{"decision describes a prerequisite", "supply known missing input", "human-only acts", "not permission", "do not shell-evaluate", "not delivery or goal conclusion", "before --check"} {
		if !strings.Contains(text, wanted) {
			t.Errorf("missing execution guidance: %q", wanted)
		}
	}
	for _, outcome := range page.Protocol.Outcomes {
		var output bytes.Buffer
		inv := &intentInvocation{command: intentCommand{name: "status"}, stdout: &output, stderr: &output,
			input: intentInput{values: map[string][]string{"json": {"true"}}}}
		code := inv.render(intentResult{Outcome: outcome.Name})
		var actual intentResult
		if err := json.Unmarshal(output.Bytes(), &actual); err != nil {
			t.Fatal(err)
		}
		if actual.Outcome != outcome.Name || (code == 0) != slices.Contains([]string{intentConfirmed, intentUnchanged}, outcome.Name) {
			t.Errorf("documented outcome disagrees with real renderer: %s exit %d", outcome.Name, code)
		}
	}
	if len(page.Protocol.Outcomes) != 6 {
		t.Errorf("incomplete outcome protocol: %+v", page.Protocol.Outcomes)
	}
}
