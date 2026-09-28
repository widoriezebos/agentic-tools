package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// publicPairs is the object-action table of the design page's section 3.1
// as revision 7's section 3.4 leaves it (R12: every public action states an
// intent), with the top-level status: the complete public surface.
var publicPairs = []string{
	"goal list", "goal show", "goal approve", "goal budget", "goal pause", "goal resume", "goal done", "goal open", "goal edit",
	"goal claim", "goal release", "goal accept-risk", "goal pin", "goal prioritize", "goal reopen", "goal abandon", "goal block",
	"goal unblock", "goal unapprove", "goal split", "goal group", "goal ungroup", "goal notes", "goal sync", "goal allow", "goal disallow",
	"grant add", "grant revoke", "grant list",
	"decision list", "decision show",
	"design write", "design show", "design list", "design review", "design stop",
	"work brief", "work build", "work wait", "work review", "work revise", "work land", "work finish", "work status", "work stop",
	"test run", "test wait", "test plan", "test list", "test status",
	"question ask", "question retry", "question withdraw", "question answer", "question show", "question list", "question wait",
	"incident list", "incident claim", "incident close",
	"status",
	"helm take", "helm return",
	"session start", "session stop", "session status", "session handoff", "session isolate",
	"mission start", "mission status", "mission resume", "mission repair",
	"system start", "system stop", "system restart", "system status", "system check", "system enroll", "system setup", "system adopt",
	"machine list", "machine start",
	"app start", "app stop", "app restart", "app status", "app log", "app reset", "app check",
	"ui start", "ui stop", "ui restart", "ui status",
	"settings show", "settings keys", "settings check", "settings coordinator",
	"receipt add", "receipt status", "receipt retro",
	"experiment record", "experiment challenge", "experiment status", "experiment check",
}

// revisionSevenObjects are the objects section 3.4 leaves, in help order,
// with helm (human-control design, section 3.1) in the Run group.
var revisionSevenObjects = []string{"goal", "design", "decision", "grant", "work", "test", "question", "incident",
	"helm", "session", "mission", "system", "machine", "app", "ui", "settings", "receipt", "experiment"}

// hiddenPairs are the process entrypoints whose first word is an object.
var hiddenPairs = []string{"goal fetch", "goal next", "test worker", "test worker-capabilities", "mission run-loop", "app serve", "ui serve", "ui tools"}

// TestIntentPublicCoverage is structural: it checks the object-action table,
// its grammar, its help and its catalogue against the engine families, and
// never runs a substantive owner.
func TestIntentPublicCoverage(t *testing.T) {
	t.Parallel()
	commands := publicIntentCommands()
	registered := families()

	t.Run("public table", func(t *testing.T) {
		var got []string
		seen := map[string]bool{}
		for _, command := range commands {
			if seen[command.name] {
				t.Errorf("%s is registered twice", command.name)
			}
			seen[command.name] = true
			got = append(got, command.name)
			if command.name != "status" && (command.object == "" || command.action == "" || !isIntentObject(command.object)) {
				t.Errorf("%s is not an object and an action", command.name)
			}
			if command.group == "" || intentObjectGroup(command.object) != command.group {
				t.Errorf("%s is in group %q, not its object's", command.name, command.group)
			}
			if (command.run == nil) == (command.passthrough == nil) {
				t.Errorf("%s needs exactly one of a parsed handler and a passthrough handler", command.name)
			}
			if command.summary == "" || len(command.allUsage()) == 0 || len(command.examples) == 0 {
				t.Errorf("%s: summary %q, %d usage lines, %d examples; all are required", command.name, command.summary, len(command.allUsage()), len(command.examples))
			}
			for _, line := range append(append([]string(nil), command.allUsage()...), command.examples...) {
				if !strings.HasPrefix(line, "metasystem "+command.name) || strings.TrimSpace(line) != line {
					t.Errorf("%s: %q does not start with metasystem %s", command.name, line, command.name)
				}
			}
			if command.passthrough != nil {
				continue
			}
			spellings := map[string]string{}
			for _, definition := range command.allFlags() {
				for _, spelling := range append([]string{definition.name}, definition.aliases...) {
					if owner, dup := spellings[spelling]; dup {
						t.Errorf("%s: --%s is spelled by both --%s and --%s", command.name, spelling, owner, definition.name)
					}
					spellings[spelling] = definition.name
				}
			}
			for spelling, want := range map[string]string{"repo": "repo", "root": "repo", "json": "json"} {
				if definition, ok := command.lookupFlag(spelling); !ok || definition.name != want {
					t.Errorf("%s: --%s does not resolve to --%s", command.name, spelling, want)
				}
			}
		}
		sort.Strings(got)
		want := slices.Clone(publicPairs)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("public pairs differ:\n got  %v\n want %v", got, want)
		}
		var hidden []string
		for _, command := range intentCommands() {
			if command.hidden {
				hidden = append(hidden, command.name)
				if command.launcher == "" || !familyHasVerb(registered, command.object, command.action) {
					t.Errorf("hidden entry %s has no launcher or no family verb that serves it", command.name)
				}
				if seen[command.name] {
					t.Errorf("%s is both public and an entry", command.name)
				}
			}
		}
		if !slices.Equal(hidden, hiddenPairs) {
			t.Errorf("hidden entries = %v, want %v", hidden, hiddenPairs)
		}
		if !slices.Equal(intentObjects(), revisionSevenObjects) {
			t.Errorf("objects = %v, want revision 7's %v", intentObjects(), revisionSevenObjects)
		}
		for _, object := range intentObjects() {
			if len(objectActions(object)) == 0 {
				t.Errorf("object %s has no public action", object)
			}
			if intentObjectSummaries[object] == "" {
				t.Errorf("object %s has no summary", object)
			}
		}
	})

	t.Run("goal acts", func(t *testing.T) {
		// Every goal-family verb has a public home or a stated reason to stay
		// internal. kind is a word the target's usage must carry.
		type disposition struct{ action, flag, kind, internal string }
		acts := map[string]disposition{
			"branch":          {action: "work build", internal: "diagnostics stay internal; build, review and land own the branch"},
			"handover":        {internal: "delivery workflow step"},
			"open":            {action: "goal open"},
			"carry":           {action: "goal abandon", flag: "successor"},
			"set-next":        {action: "goal edit", flag: "next"},
			"park":            {action: "goal pause"},
			"done":            {action: "goal done"},
			"claim":           {action: "goal claim"},
			"restamp":         {internal: "startup step"},
			"approve":         {action: "goal approve"},
			"tier-probe":      {action: "goal list", flag: "tiers"},
			"set-budget":      {action: "goal budget", kind: "goal budget G BOX"},
			"extend-budget":   {internal: "dispatch admission"},
			"accept-risk":     {action: "goal accept-risk"},
			"enroll-terminal": {action: "system enroll"},
			"release":         {action: "goal release"},
			"trunk-red":       {action: "incident claim"},
			"land-ready":      {action: "work land", flag: "queue-only", kind: "work land G --queue-only"},
			"edit":            {action: "goal edit"},
			"list":            {action: "goal list"},
			"show":            {action: "goal show"},
			"next":            {action: "goal list", flag: "ready", internal: "seat launch entry"},
			"reconcile":       {action: "goal sync", flag: "publish", internal: "reviewed recovery"},
			"migrate":         {internal: "installation cutover"},
			"fetch":           {action: "goal list", flag: "fetch", internal: "seat launch entry"},
			"repair":          {internal: "authority recovery (goal sync --accept-remote-history is its public form)"},
		}
		var goalFamily *family
		for index := range registered {
			if registered[index].name == "goal" {
				goalFamily = &registered[index]
			}
		}
		if goalFamily == nil {
			t.Fatal("no goal family is registered")
		}
		live := map[string]bool{}
		for _, v := range goalFamily.verbs {
			live[v.name] = true
			if _, ok := acts[v.name]; !ok {
				t.Errorf("goal %s has no public or internal disposition", v.name)
			}
		}
		for act, row := range acts {
			if !live[act] {
				t.Errorf("disposition for goal %s, which the goal family no longer registers", act)
			}
			if row.action == "" {
				if row.internal == "" {
					t.Errorf("goal %s is internal without a reason", act)
				}
				continue
			}
			target, ok := findIntentCommand(row.action)
			if !ok || target.hidden {
				t.Errorf("goal %s maps to %s, which is not a public action", act, row.action)
				continue
			}
			if row.flag != "" {
				if _, ok := target.lookupFlag(row.flag); !ok {
					t.Errorf("goal %s maps to %s --%s, which %s does not accept", act, row.action, row.flag, row.action)
				}
			}
			if row.kind != "" && !strings.Contains(strings.Join(target.allUsage(), "\n"), row.kind) {
				t.Errorf("goal %s maps to %q, which %s usage does not show", act, row.kind, row.action)
			}
		}
	})

	t.Run("ordinary grammar", func(t *testing.T) {
		forms := map[string][]string{
			"status":          {"metasystem status", "metasystem status G [--work NAME]"},
			"work review":     {"work review G", "work review j2:J", "work review --commit SHA --goal G", "work review run:RUN", "work review G --finding F --test NAME", "work review --changes", "work review --patch PATCH"},
			"work revise":     {"work revise G", "--after N", "--brief FILE", "work revise j2:R --dispositions FILE --brief FILE", "work revise run:RUN --brief FILE"},
			"work finish":     {"work finish j2:J", "work finish j2:J --evidence R"},
			"work land":       {"work land j2:J", "work land G --queue-only"},
			"work wait":       {"work wait G", "work wait G --for landing|human-act", "--since TIP", "work wait REF", "work wait --path PATH --until present|absent", "work wait j2:J --exit-code", "work wait --run ID --exit-code", "work wait --list"},
			"work build":      {"work build G [--work NAME] --brief FILE --check COMMAND...", "work build run:RUN"},
			"goal done":       {"goal done G --reason TEXT"},
			"goal notes":      {"--read", "--add", "--close", "--fixed", "--moved", "--accepted"},
			"goal pin":        {"--clear"},
			"goal claim":      {"--take-over"},
			"goal sync":       {"goal sync --recover", "goal sync --refresh", "goal sync --publish --goal G... --by NAME", "goal sync --upgrade"},
			"work status":     {"work status [--all]", "work status G", "work status REF", "work status [G | j1:ID] --history [--since RFC3339]"},
			"session handoff": {"session handoff --status", "session handoff --verify NONCE"},
			"receipt add":     {"receipt add --corrects EPOCH:SHA1"},
			"test status":     {"test status --tree TREE", "test status --result FILE"},
			"question answer": {"question answer Q [TEXT]", "question answer M/Q TEXT"},
			"test wait":       {"test wait proof:ID"},
			"design review":   {"design review FILE", "design review FILE --check-only"},
			"system start":    {"system start --if-down"},
			"system status":   {"system status --steward"},
			"incident claim":  {"incident claim I --goal G"},
		}
		for name, wants := range forms {
			command, ok := findIntentCommand(name)
			if !ok {
				t.Errorf("no public %s", name)
				continue
			}
			usage := strings.Join(command.allUsage(), "\n")
			for _, want := range wants {
				if !strings.Contains(usage, want) {
					t.Errorf("%s usage lacks %q: %q", name, want, usage)
				}
			}
		}
		// Mandatory options the examples must show, by example prefix.
		mandatory := []struct {
			prefix string
			needs  []string
			when   string
			except string
		}{
			{prefix: "metasystem goal open ", needs: []string{"--risk", "--basis"}},
			{prefix: "metasystem design review ", needs: []string{"--tool-calls"}, except: "--check-only"},
			{prefix: "metasystem work review j2:", needs: []string{"--tool-calls"}},
			{prefix: "metasystem work review --commit ", needs: []string{"--goal"}},
			{prefix: "metasystem goal notes ", needs: []string{"--read"}, when: "--add"},
		}
		for _, command := range commands {
			for _, example := range command.examples {
				words := shellWords(example)
				rest := words[1+len(command.words()):]
				if command.passthrough == nil {
					if _, problem := parseIntentArgs(command, rest); problem != nil {
						t.Errorf("%s example %q does not parse: %s", command.name, example, problem.summary)
					}
				}
				for _, rule := range mandatory {
					if !strings.HasPrefix(example, rule.prefix) || (rule.when != "" && !slices.Contains(words, rule.when)) || (rule.except != "" && slices.Contains(words, rule.except)) {
						continue
					}
					for _, need := range rule.needs {
						if !slices.ContainsFunc(words, func(w string) bool { return w == need || strings.HasPrefix(w, need+"=") }) {
							t.Errorf("example %q lacks the mandatory %s", example, need)
						}
					}
				}
			}
		}
		build, _ := findIntentCommand("work build")
		input, problem := parseIntentArgs(build, []string{"g", "u", "--brief", "b", "--check", "go", "test", "--json", "-run", "X"})
		if problem != nil {
			t.Fatalf("work build --check parse: %s", problem.summary)
		}
		if got := input.values["check"]; !slices.Equal(got, []string{"go", "test", "--json", "-run", "X"}) || input.has("json") {
			t.Errorf("work build --check = %q json %t; want the rest literally and no --json", got, input.has("json"))
		}
	})

	t.Run("help", func(t *testing.T) {
		stale := []string{"has not moved", "have not moved", "not moved to this surface", "not public commands yet", "not public yet"}
		checkStale := func(label, page string) {
			t.Helper()
			for _, phrase := range stale {
				if strings.Contains(page, phrase) {
					t.Errorf("%s still says %q", label, phrase)
				}
			}
		}
		pages := map[string]string{}
		for _, args := range [][]string{{"help"}, {"help", "human"}, {"help", "agent"}, {"help", "all"}} {
			code, page, problem := runCLIHelp(args, registered)
			if code != 0 || problem != "" || page == "" {
				t.Errorf("%v = code %d stderr %q", args, code, problem)
			}
			checkStale(strings.Join(args, " "), page)
			pages[strings.Join(args, " ")] = page
		}
		for _, object := range intentObjects() {
			if !strings.Contains(pages["help"], "\n  "+object+" ") {
				t.Errorf("root help does not list the object %s", object)
			}
		}
		for _, command := range commands {
			listed := false
			for _, usage := range command.allUsage() {
				listed = listed || strings.Contains(pages["help all"], "\n  "+usage+"\n")
			}
			if !listed {
				t.Errorf("help all does not list %s", command.name)
			}
		}
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		failing := intentOwners{resolver: stateroot.NewResolver(func(string) (string, error) { return "", errors.New("no repository") }, noExecutable)}
		for _, command := range commands {
			var direct bytes.Buffer
			writeIntentHelp(&direct, command)
			if command.passthrough == nil {
				var parsed, problem bytes.Buffer
				if code := runIntentIn(command, []string{"--help"}, &parsed, &problem, missing, failing); code != 0 || problem.Len() != 0 || parsed.String() != direct.String() {
					t.Errorf("%s --help without a repository = code %d stderr %q", command.name, code, problem.String())
				}
			}
			for _, args := range [][]string{append([]string{"help"}, command.words()...), append(command.words(), "--help")} {
				code, page, stderr := runCLIHelp(args, registered)
				label := strings.Join(args, " ")
				if code != 0 || stderr != "" || page != direct.String() {
					t.Errorf("%s = code %d stderr %q; it is not the action's own help", label, code, stderr)
				}
				for _, line := range append(append([]string(nil), command.allUsage()...), command.examples...) {
					if !strings.Contains(page, line) {
						t.Errorf("%s lacks %q", label, line)
					}
				}
				checkStale(label, page)
			}
		}
	})

	t.Run("partner union", func(t *testing.T) {
		catalogue := commandCatalogue()
		if len(catalogue) != 1 || catalogue[0].Name != "" {
			t.Fatalf("catalogue has %d entries, want the public table alone", len(catalogue))
		}
		counts := map[string]int{}
		for _, one := range catalogue[0].Verbs {
			counts[one.Name]++
		}
		for _, name := range publicPairs {
			if counts[name] != 1 {
				t.Errorf("Partner catalogue lists %s %d times, want 1", name, counts[name])
			}
		}
		for _, name := range hiddenPairs {
			if counts[name] != 0 {
				t.Errorf("Partner catalogue lists the entry %s", name)
			}
		}
		for _, fam := range registered {
			if counts[fam.name] != 0 {
				t.Errorf("Partner catalogue lists the engine family %s", fam.name)
			}
		}
	})
}
