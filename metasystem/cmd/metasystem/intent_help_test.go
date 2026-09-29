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
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		words := strings.Fields(line)
		for index := 1; index < len(words) && index <= 2; index++ {
			if strings.HasPrefix(words[index], "[") {
				listed[strings.Join(words[:index], " ")]++
			}
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
	for _, wanted := range []string{"goal approve", "goal budget", "goal accept-risk", "help human", "help work review FORM", "--json", "--check", "OBJECT ACTION", "j2:ID"} {
		if !strings.Contains(page, wanted) {
			t.Errorf("agent help omits %q", wanted)
		}
	}
	for _, forbidden := range []string{"metasystem internal", "--fixture-human-authority", "--lineage", "goal fetch", "run-loop", "test worker"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("agent help exposes %s", forbidden)
		}
	}
}

// TestIntentHelpSurfacesArePublicForms is the witness that every help
// surface, the JSON help and the Partner catalogue are written from the one
// table: each usage line is an OBJECT ACTION form of a public pair (or the
// top-level status) and no hidden entry appears anywhere.
func TestIntentHelpSurfacesArePublicForms(t *testing.T) {
	t.Parallel()
	registered := families()
	pages := map[string]string{}
	for _, args := range [][]string{{"help"}, {"help", "all"}, {"help", "human"}, {"help", "agent"}} {
		_, page, _ := runCLIHelp(args, registered)
		pages[strings.Join(args, " ")] = page
	}
	for _, object := range intentObjects() {
		_, page, _ := runCLIHelp([]string{object}, registered)
		pages[object] = page
		_, page, _ = runCLIHelp([]string{"help", object, "--json"}, registered)
		pages["json "+object] = page
	}
	for _, command := range publicIntentCommands() {
		_, page, _ := runCLIHelp(append(command.words(), "--help"), registered)
		pages[command.name] = page
		_, page, _ = runCLIHelp(append(append([]string{"help"}, command.words()...), "--json"), registered)
		pages["json "+command.name] = page
		for _, form := range command.helpForms {
			_, page, _ = runCLIHelp(append(append([]string{"help"}, command.words()...), form.name), registered)
			pages[command.name+" "+form.name] = page
		}
	}
	_, index, _ := runCLIHelp([]string{"help", "--json"}, registered)
	pages["json index"] = index
	catalogue, _ := json.Marshal(commandCatalogue())
	pages["partner"] = string(catalogue)
	for label, page := range pages {
		if page == "" {
			t.Errorf("%s is empty", label)
		}
		for _, command := range intentCommands() {
			if command.hidden && (strings.Contains(page, "metasystem "+command.name) || strings.Contains(page, `"`+command.name+`"`)) {
				t.Errorf("%s names the entry %s", label, command.name)
			}
		}
		for _, match := range textCommandPattern.FindAllStringSubmatch(page, -1) {
			first, second := match[2], match[3]
			switch {
			case first == "help" || first == "internal" || first == "status":
			case isIntentObject(first):
				if command, ok := findIntentAction(first, second); ok && command.hidden || !ok && familyHasVerb(registered, first, second) {
					t.Errorf("%s names %s %s, which is not a public pair", label, first, second)
				}
			case slices.Contains(removedFirstWords, first) || familyHasVerb(registered, first, second):
				t.Errorf("%s names %s %s, which is not a public form", label, first, second)
			}
		}
	}
	for _, command := range publicIntentCommands() {
		for _, usage := range command.allUsage() {
			if !strings.HasPrefix(usage, "metasystem "+command.name) {
				t.Errorf("%s has the usage %q", command.name, usage)
			}
		}
	}
}

func TestIntentAdministrationHelp(t *testing.T) {
	t.Parallel()
	for _, command := range publicIntentCommands() {
		administration := slices.Contains(intentAdministrationObjects, command.object)
		if administration && command.helpScope() != "administration" || !administration && command.helpScope() == "administration" {
			t.Errorf("%s has scope %s", command.name, command.helpScope())
		}
		seen := map[string]bool{}
		for _, usage := range command.allUsage() {
			if seen[usage] {
				t.Errorf("%s repeats a form across sections: %s", command.name, usage)
			}
			seen[usage] = true
		}
	}
	for _, row := range []struct{ name, work, admin string }{
		{"work land", "metasystem work land G --queue-only", "metasystem work land G --exception CODE --reason TEXT --by NAME --upgrade-goals"},
		{"goal sync", "metasystem goal sync --recover", "metasystem goal sync --upgrade"},
	} {
		_, doc := readHelpJSON(t, append(strings.Fields(row.name), "--json")...)
		if doc.Command.Scope != "mixed" || !strings.Contains(strings.Join(doc.Command.Usage, "\n"), row.work) ||
			!strings.Contains(strings.Join(doc.Command.AdministrationUsage, "\n"), row.admin) {
			t.Errorf("%s loses the workflow/administration distinction: %+v", row.name, doc.Command)
		}
		code, page, problem := runCLIHelp(append([]string{"help"}, strings.Fields(row.name)...), families())
		heading := strings.Index(page, "MetaSystem administration:")
		work, admin := strings.Index(page, row.work), strings.Index(page, row.admin)
		if code != 0 || problem != "" || work < 0 || work >= heading || admin <= heading {
			t.Errorf("%s text does not separate forms: %d %d %d / %s", row.name, work, heading, admin, problem)
		}
	}
	_, page, _ := runCLIHelp([]string{"system", "stop", "--help"}, families())
	if !strings.Contains(page, "MetaSystem administration: this action manages the work system itself.") {
		t.Errorf("system stop does not say it administers MetaSystem: %q", page)
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
		if command.passthrough != nil {
			continue
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
	for _, object := range intentObjects() {
		// help agent is the agents' page, the whole catalogue (above); the
		// agent object's own actions are each reachable as help agent ACTION.
		if intentTopic(object) {
			continue
		}
		_, page := readHelpJSON(t, object, "--json")
		if len(page.Commands) != len(objectActions(object)) {
			t.Errorf("help %s --json lists %d actions, want %d", object, len(page.Commands), len(objectActions(object)))
		}
		for _, entry := range page.Commands {
			if !strings.HasPrefix(entry.Name, object+" ") {
				t.Errorf("help %s --json lists %s", object, entry.Name)
			}
		}
	}
	_, human := readHelpJSON(t, "human", "--json")
	for _, entry := range human.Commands {
		if entry.Audience == "agent" {
			t.Errorf("help human lists the agent action %s", entry.Name)
		}
	}
}

func TestIntentHelpRefusals(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"help", "--bad", "--json"}, {"help", "--json", "internal"},
		{"help", "launch", "--json"}, {"help", "close", "--json"},
		{"help", "work", "review", "unknown", "--json"}, {"help", "work", "review", "design", "extra", "--json"},
		{"help", "work", "review", "--changes", "--json"}, {"help", "all", "extra", "--json"},
		{"help", "no-such-command", "--json"}, {"help", "work", "review", "goal", "extra"},
		{"help", "work", "review", "unknown"}, {"help", "--bad"}, {"help", "goal", "fetch", "--json"},
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
		} else if output != "" || !strings.Contains(problem, "metasystem") {
			t.Errorf("text refusal %v = %q %q", args, output, problem)
		}
	}
	for _, args := range [][]string{{"help", "goal", "fetch"}, {"help", "no-such-object"}, {"help", "goals"}} {
		if code, output, problem := runCLIHelp(args, families()); code != 2 || output != "" || !strings.Contains(problem, "nothing was done") {
			t.Errorf("%v = %d %q %q", args, code, output, problem)
		}
	}
}

func TestIntentHelpNeverExecutes(t *testing.T) {
	t.Parallel()
	registered := []family{{name: "sentinel", summary: "read help safely", verbs: []verb{{name: "mutate", summary: "must not run", run: func([]string) int {
		panic("help executed a command")
	}}}}}
	for _, args := range [][]string{
		{"help", "--json"}, {"help", "agent"}, {"help", "work", "review", "submit"},
		{"help", "design", "review", "design", "--json"}, {"help", "goal", "open", "--intent", "mutate"},
		{"help", "sentinel"}, {"help", "sentinel", "mutate"}, {"help", "sentinel", "--json"},
		{"goal"}, {"goal", "--help"}, {"goal", "approve", "--help"}, {"status", "--help"}, {"test", "plan", "--help"},
	} {
		var output, problem bytes.Buffer
		dispatchWithFamiliesAndRepositoryTop(args, &output, &problem, registered, func(string) (string, error) {
			panic("help resolved a repository")
		})
	}
}

func TestIntentReviewHelpForms(t *testing.T) {
	t.Parallel()
	expected := map[string]map[string][]string{
		"work review": {
			"goal": {reviewGoalUsage}, "submit": {reviewSubmitUsage}, "finding": {reviewFindingUsage},
			"job": {reviewJobUsage}, "commit": {reviewCommitUsage}, "run": {reviewRunUsage}, "changes": {reviewChangesUsage}, "diff": {reviewDiffUsage},
			"check": {reviewCheckUsage},
		},
		"design review": {"design": {reviewDesignUsage}, "check": {reviewDesignCheck}},
	}
	for name, forms := range expected {
		_, review := readHelpJSON(t, append(strings.Fields(name), "--json")...)
		command, _ := findIntentCommand(name)
		if len(review.Command.Forms) != len(forms) {
			t.Fatalf("%s forms: %v", name, review.Command.Forms)
		}
		for _, entry := range review.Command.Forms {
			_, page := readHelpJSON(t, entry.HelpArgv[2:]...)
			form := page.Form
			if form == nil || !reflect.DeepEqual(form.Usage, forms[entry.Name]) {
				t.Fatalf("wrong usage for %s %s: %+v", name, entry.Name, form)
			}
			code, text, problem := runCLIHelp(append(append([]string{"help"}, strings.Fields(name)...), entry.Name), families())
			if code != 0 || problem != "" || !strings.HasPrefix(text, name+" "+entry.Name+" - ") {
				t.Fatalf("focused text %s %s failed: %d %s %q", name, entry.Name, code, problem, text)
			}
			for _, section := range []string{form.Purpose, form.Effects, form.Authority, form.Repetition, "inputs:", "options:", "example:"} {
				if section == "" || !strings.Contains(text, section) {
					t.Errorf("focused text %s omits %q", entry.Name, section)
				}
			}
			if !slices.Equal(form.ExampleArgv[:1+len(command.words())], append([]string{"metasystem"}, command.words()...)) {
				t.Fatalf("%s example is not a %s form: %v", entry.Name, name, form.ExampleArgv)
			}
			input, err := parseIntentArgs(command, form.ExampleArgv[1+len(command.words()):])
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
				if len(input.args) != 0 || !input.has("changes") && !input.has("patch") {
					t.Errorf("diagnostic example names a goal: %v", form.ExampleArgv)
				}
				for _, flag := range []string{"work", "dispositions", "finding", "test", "model", "tool-calls", "after"} {
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
				if !reflect.DeepEqual(input.args, []string{"G"}) || !input.switched("changes") || !input.has("brief") || options["retry"] {
					t.Errorf("submission example/options: %+v %+v", input, options)
				}
				if !strings.Contains(form.Effects, "commits") || !strings.Contains(form.Effects, "publishes") {
					t.Error("submission does not disclose its effects")
				}
			case "design":
				if options["model"] || !options["after"] || !options["retry"] {
					t.Error("design help misstates model or retry options")
				}
			case "commit":
				if !input.has("commit") || !input.has("goal") {
					t.Errorf("commit example = %+v", input)
				}
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
