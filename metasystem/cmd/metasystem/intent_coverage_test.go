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

// TestIntentPublicCoverage is structural: it checks the public command table,
// its grammar, its help and its catalogue against the engine families, and
// never runs a substantive owner.
func TestIntentPublicCoverage(t *testing.T) {
	t.Parallel()
	publicNames := []string{
		"abandon", "accept-risk", "answer", "approve", "ask", "block", "brief", "budget", "build", "check", "claim", "close", "design",
		"decide", "doctor", "done", "edit", "enroll", "fleet", "fold", "goals", "grant", "group",
		"incidents", "land", "notes", "open", "pause", "pin", "prioritize", "ready", "recover", "red", "release", "repair",
		"reopen", "resolve", "restart", "resume", "review", "revise", "revoke", "settings", "show", "split", "start",
		"status", "stop", "test", "ui", "unapprove", "unblock", "ungroup", "wait",
	}
	commands := intentCommands()
	registered := families()

	t.Run("public table", func(t *testing.T) {
		var got []string
		seen := map[string]bool{}
		for _, command := range commands {
			if seen[command.name] {
				t.Errorf("command %s is registered twice", command.name)
			}
			seen[command.name] = true
			got = append(got, command.name)
			if command.run == nil {
				t.Errorf("%s has no run handler", command.name)
			}
			if command.summary == "" || len(command.usage) == 0 || len(command.examples) == 0 {
				t.Errorf("%s: summary %q, %d usage lines, %d examples; all are required", command.name, command.summary, len(command.usage), len(command.examples))
			}
			for _, line := range append(append([]string(nil), command.usage...), command.examples...) {
				if strings.TrimSpace(line) == "" {
					t.Errorf("%s has an empty usage or example line", command.name)
				}
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
		want := slices.Clone(publicNames)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("public commands differ:\n got  %v\n want %v", got, want)
		}
	})

	t.Run("goal acts", func(t *testing.T) {
		// Every goal-family verb has a public home or a stated reason to stay
		// internal. kind is a word the target's usage must carry.
		type disposition struct{ verb, flag, kind, internal string }
		acts := map[string]disposition{
			"branch":                      {verb: "build", internal: "diagnostics stay internal; build, review and land own the branch"},
			"handover":                    {internal: "delivery workflow step"},
			"open":                        {verb: "open"},
			"abandon":                     {verb: "abandon"},
			"carry":                       {verb: "abandon", flag: "successor"},
			"engine-floor":                {internal: "installation maintenance"},
			"set-next":                    {verb: "edit", flag: "next"},
			"read-items":                  {verb: "notes"},
			"promote":                     {internal: "legacy-ledger maintenance"},
			"park":                        {verb: "pause"},
			"unpark":                      {verb: "resume"},
			"block":                       {verb: "block"},
			"unblock":                     {verb: "unblock"},
			"done":                        {verb: "done"},
			"reopen":                      {verb: "reopen"},
			"declare-free":                {internal: "legacy"},
			"prune":                       {internal: "legacy"},
			"claim":                       {verb: "claim"},
			"restamp":                     {internal: "startup step"},
			"approve":                     {verb: "approve"},
			"budget":                      {verb: "budget"},
			"classify-sweep":              {internal: "installation maintenance"},
			"tier-probe":                  {verb: "goals", flag: "tiers"},
			"unapprove":                   {verb: "unapprove"},
			"set-budget":                  {verb: "budget", kind: "budget G BOX"},
			"extend-budget":               {internal: "dispatch admission"},
			"grant":                       {verb: "grant"},
			"revoke":                      {verb: "revoke"},
			"carrying":                    {internal: "exceptional landing"},
			"carried":                     {internal: "exceptional landing"},
			"accept-risk":                 {verb: "accept-risk"},
			"discharge-review-obligation": {verb: "resolve"},
			"split":                       {verb: "split"},
			"set-obligation":              {verb: "edit", flag: "obligation"},
			"enroll-terminal":             {verb: "enroll"},
			"resume":                      {verb: "resume"},
			"release":                     {verb: "release"},
			"steal":                       {verb: "claim", flag: "take-over"},
			"trunk-red":                   {verb: "red", kind: "red own"},
			"land-ready":                  {verb: "ready"},
			"edit":                        {verb: "edit"},
			"set-arc":                     {verb: "group"},
			"set-pin":                     {verb: "pin"},
			"set-priority":                {verb: "prioritize", flag: "sequence"},
			"detach":                      {verb: "ungroup"},
			"list":                        {verb: "goals"},
			"show":                        {verb: "show"},
			"next":                        {verb: "goals", flag: "ready"},
			"reconcile":                   {internal: "reviewed recovery"},
			"migrate":                     {internal: "installation cutover"},
			"fetch":                       {verb: "goals", flag: "fetch", internal: "diagnostic read-side advance"},
			"repair":                      {internal: "authority recovery"},
			"source-digest":               {internal: "migration support"},
			"recover":                     {verb: "recover"},
		}
		if len(acts) != 54 {
			t.Fatalf("the disposition table has %d rows, want 54", len(acts))
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
			if row.verb == "" {
				if row.internal == "" {
					t.Errorf("goal %s is internal without a reason", act)
				}
				continue
			}
			target, ok := findIntentCommand(row.verb)
			if !ok {
				t.Errorf("goal %s maps to %s, which is not a public command", act, row.verb)
				continue
			}
			if row.flag != "" {
				if _, ok := target.lookupFlag(row.flag); !ok {
					t.Errorf("goal %s maps to %s --%s, which %s does not accept", act, row.verb, row.flag, row.verb)
				}
			}
			if row.kind != "" && !strings.Contains(strings.Join(target.usage, "\n"), row.kind) {
				t.Errorf("goal %s maps to %q, which %s usage does not show", act, row.kind, row.verb)
			}
		}
	})

	t.Run("ordinary grammar", func(t *testing.T) {
		forms := map[string][]string{
			"start":     {"start session"},
			"stop":      {"stop job", "stop session"},
			"restart":   {"restart checkout", "restart ui"},
			"status":    {"status G", "status job", "status work"},
			"review":    {"review G", "review design", "review job", "review commit"},
			"revise":    {"revise G", "--after N", "--brief FILE"},
			"incidents": {"incidents claim", "incidents close"},
			"repair":    {"repair review G"},
			"fold":      {"fold review", "fold unit"},
			"land":      {"land job", "land G --queue-only"},
			"wait":      {"wait G", "wait G --for landing|human-act", "--since TIP", "wait job"},
			"ui":        {"start|stop|status"},
			"notes":     {"--read", "--add", "--close", "--fixed", "--moved", "--accepted"},
			"pin":       {"--clear"},
			"claim":     {"--take-over"},
			"red":       {"red own", "red close"},
			"answer":    {"answer Q [TEXT]", "answer M/Q TEXT"},
		}
		for name, wants := range forms {
			command, ok := findIntentCommand(name)
			if !ok {
				t.Errorf("no public %s", name)
				continue
			}
			usage := strings.Join(command.usage, "\n")
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
		}{
			{prefix: "metasystem open ", needs: []string{"--risk", "--basis"}},
			{prefix: "metasystem review design ", needs: []string{"--tool-calls"}},
			{prefix: "metasystem review job ", needs: []string{"--tool-calls"}},
			{prefix: "metasystem review commit ", needs: []string{"--goal"}},
			{prefix: "metasystem notes ", needs: []string{"--read"}, when: "--add"},
		}
		for _, command := range commands {
			for _, example := range command.examples {
				words := shellWords(example)
				if len(words) < 2 || words[0] != "metasystem" || words[1] != command.name {
					t.Errorf("%s example %q does not start with metasystem %s", command.name, example, command.name)
					continue
				}
				// The router hands a family verb after a public name (ui start)
				// to that family; only the rest reaches the intent parser.
				if intentYieldsToLegacy(words[1:], registered) {
					if len(words) < 3 || !isFamilyName(registered, command.name) {
						t.Errorf("%s example %q yields to a legacy form that is not a family verb", command.name, example)
					}
				} else if _, problem := parseIntentArgs(command, words[2:]); problem != nil {
					t.Errorf("%s example %q does not parse: %s", command.name, example, problem.summary)
				}
				for _, rule := range mandatory {
					if !strings.HasPrefix(example, rule.prefix) || (rule.when != "" && !slices.Contains(words, rule.when)) {
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
		build, _ := findIntentCommand("build")
		input, problem := parseIntentArgs(build, []string{"g", "u", "--brief", "b", "--check", "go", "test", "--json", "-run", "X"})
		if problem != nil {
			t.Fatalf("build --check parse: %s", problem.summary)
		}
		if got := input.values["check"]; !slices.Equal(got, []string{"go", "test", "--json", "-run", "X"}) || input.has("json") {
			t.Errorf("build --check = %q json %t; want the rest literally and no --json", got, input.has("json"))
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
		// The root page is an orientation; help all lists every public
		// command and none of the compatibility spellings, which stay routed.
		for _, name := range publicNames {
			command, _ := findIntentCommand(name)
			listed := strings.Contains(pages["help all"], "\n  metasystem "+name+" ") || strings.Contains(pages["help all"], "\n  metasystem "+name+"\n")
			if listed == command.compatibility {
				t.Errorf("help all lists %s = %t; compatibility %t", name, listed, command.compatibility)
			}
			if command.primary && !strings.Contains(pages["help"], "  "+name) {
				t.Errorf("root help does not mention the common command %s", name)
			}
		}
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		failing := intentOwners{resolver: stateroot.NewResolver(func(string) (string, error) { return "", errors.New("no repository") }, noExecutable)}
		commandPages := map[string]string{}
		for _, command := range commands {
			var direct, problem bytes.Buffer
			if code := runIntentIn(command, []string{"--help"}, &direct, &problem, missing, failing); code != 0 || problem.Len() != 0 {
				t.Errorf("%s --help without a repository = code %d stderr %q", command.name, code, problem.String())
			}
			for _, args := range [][]string{{"help", command.name}, {command.name, "--help"}} {
				code, page, stderr := runCLIHelp(args, registered)
				label := strings.Join(args, " ")
				if code != 0 || stderr != "" {
					t.Errorf("%s = code %d stderr %q", label, code, stderr)
				}
				for _, line := range append(append([]string(nil), command.usage...), command.examples...) {
					if !strings.Contains(page, line) {
						t.Errorf("%s lacks %q", label, line)
					}
				}
				if !strings.HasPrefix(page, direct.String()) {
					t.Errorf("%s does not begin with the command's own help", label)
				}
				checkStale(label, page)
				commandPages[command.name] = page
			}
		}
		t.Run("compatibility help", func(t *testing.T) {
			compatibility := map[string][]string{
				"ui":   {"ui start", "ui stop", "ui status"},
				"test": {"test plan", "test verify", "test report"},
				"wait": {"wait --job"},
			}
			for name, forms := range compatibility {
				page := commandPages[name]
				for _, form := range forms {
					if !strings.Contains(page, form) && !strings.Contains(page, "internal "+form) {
						t.Errorf("help %s does not mention the compatibility form %q", name, form)
					}
				}
			}
		})
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
		for _, name := range publicNames {
			command, _ := findIntentCommand(name)
			if want := map[bool]int{false: 1, true: 0}[command.compatibility]; counts[name] != want {
				t.Errorf("Partner catalogue lists %s %d times, want %d", name, counts[name], want)
			}
		}
		for _, fam := range registered {
			if _, public := findIntentCommand(fam.name); !public && counts[fam.name] != 0 {
				t.Errorf("Partner catalogue lists the engine family %s", fam.name)
			}
		}
	})
}
