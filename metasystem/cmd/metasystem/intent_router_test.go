package main

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

// sentinelFamilies mirrors the registered families with handlers that only
// record their argument vector, so a routing test proves which handler a
// command line reaches without running it.
type sentinelFamilies struct {
	mu    sync.Mutex
	calls []string
}

func (s *sentinelFamilies) registry() []family {
	var registered []family
	for _, fam := range families() {
		copied := family{name: fam.name, summary: fam.summary}
		for _, v := range fam.verbs {
			name, verbName := fam.name, v.name
			copied.verbs = append(copied.verbs, verb{name: v.name, summary: v.summary, run: func(args []string) int {
				s.mu.Lock()
				defer s.mu.Unlock()
				s.calls = append(s.calls, strings.TrimSpace(name+" "+verbName+" "+strings.Join(args, " ")))
				return 0
			}})
		}
		registered = append(registered, copied)
	}
	return registered
}

func (s *sentinelFamilies) taken() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	calls := s.calls
	s.calls = nil
	return calls
}

func routeWith(registered []family, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamiliesAndRepositoryTop(args, &stdout, &stderr, registered, func(string) (string, error) {
		panic("routing resolved a repository")
	})
	return code, stdout.String(), stderr.String()
}

// TestIntentRouterEveryPairRoutes: every public pair reaches its own
// descriptor (its help page proves the route without running an owner), a
// pair a family verb shares goes to the public action, and the family form
// stays reachable only through the explicit internal entry.
func TestIntentRouterEveryPairRoutes(t *testing.T) {
	t.Parallel()
	sentinel := &sentinelFamilies{}
	registered := sentinel.registry()
	for _, command := range publicIntentCommands() {
		var want bytes.Buffer
		writeIntentHelp(&want, command)
		code, page, problem := routeWith(registered, append(command.words(), "--help")...)
		if code != 0 || problem != "" || page != want.String() {
			t.Errorf("%s --help = %d %q; it did not route to its own descriptor", command.name, code, problem)
		}
	}
	collisions := 0
	for _, fam := range families() {
		for _, v := range fam.verbs {
			command, public := findIntentAction(fam.name, v.name)
			if !public || command.hidden {
				continue
			}
			collisions++
			var want bytes.Buffer
			writeIntentHelp(&want, command)
			if code, page, _ := routeWith(registered, fam.name, v.name, "--help"); code != 0 || page != want.String() {
				t.Errorf("%s %s --help did not reach the public action", fam.name, v.name)
			}
			if code, _, problem := routeWith(registered, "internal", fam.name, v.name, "--root", "R"); code != 0 || problem != "" {
				t.Errorf("internal %s %s = %d %q", fam.name, v.name, code, problem)
			}
			if calls := sentinel.taken(); !slices.Equal(calls, []string{fam.name + " " + v.name + " --root R"}) {
				t.Errorf("internal %s %s reached %v, want the family verb with unchanged words", fam.name, v.name, calls)
			}
		}
	}
	// goal approve and the rest of section 6.5: the colliding pairs exist.
	if collisions < 20 {
		t.Errorf("only %d family pairs collide with public actions; the collision check did not run", collisions)
	}
	if calls := sentinel.taken(); len(calls) != 0 {
		t.Errorf("public help ran family handlers: %v", calls)
	}
}

// TestIntentRouterHiddenEntries: an entry keeps its argument vector and
// reaches its family verb exactly as the internal form does.
func TestIntentRouterHiddenEntries(t *testing.T) {
	t.Parallel()
	sentinel := &sentinelFamilies{}
	registered := sentinel.registry()
	for _, pair := range hiddenPairs {
		words := strings.Fields(pair)
		args := append(slices.Clone(words), "--root", "/r", "--help-not", "x y")
		code, stdout, stderr := routeWith(registered, args...)
		viaEntry := sentinel.taken()
		internalCode, internalStdout, internalStderr := routeWith(registered, append([]string{"internal"}, args...)...)
		viaInternal := sentinel.taken()
		want := []string{strings.Join(append(slices.Clone(words), "--root", "/r", "--help-not", "x y"), " ")}
		if code != 0 || !slices.Equal(viaEntry, want) || !slices.Equal(viaEntry, viaInternal) || code != internalCode || stdout != internalStdout || stderr != internalStderr {
			t.Errorf("%s = %d %v (internal %d %v)", pair, code, viaEntry, internalCode, viaInternal)
		}
		// An entry's own help is its handler's, never a public page.
		if _, page, _ := routeWith(registered, append(slices.Clone(words), "--help")...); page != "" || !slices.Equal(sentinel.taken(), []string{pair + " --help"}) {
			t.Errorf("%s --help did not reach the entry's own handler", pair)
		}
	}
	code, page, problem := routeWith(families(), "internal")
	if code != 0 || problem != "" {
		t.Fatalf("internal = %d %q", code, problem)
	}
	for _, command := range intentCommands() {
		if command.hidden && !strings.Contains(page, command.name) || command.hidden && !strings.Contains(page, command.launcher) {
			t.Errorf("metasystem internal does not list the entry %s and its launcher", command.name)
		}
	}
}

// TestIntentRouterTransitionalFallthrough: a family pair that is neither a
// public action nor an entry still reaches its family verb, and the number of
// such pairs only falls.
func TestIntentRouterTransitionalFallthrough(t *testing.T) {
	t.Parallel()
	sentinel := &sentinelFamilies{}
	registered := sentinel.registry()
	if code, _, problem := routeWith(registered, "json", "object", "a=b"); code != 0 || problem != "" || !slices.Equal(sentinel.taken(), []string{"json object a=b"}) {
		t.Errorf("json object did not fall through to its family: %d %q", code, problem)
	}
	if code, _, problem := routeWith(registered, "goal", "set-next", "--id", "g"); code != 0 || problem != "" || !slices.Equal(sentinel.taken(), []string{"goal set-next --id g"}) {
		t.Errorf("goal set-next did not fall through to its family: %d %q", code, problem)
	}
	fallthroughPairs := 0
	for _, fam := range families() {
		for _, v := range fam.verbs {
			if _, taken := findIntentAction(fam.name, v.name); !taken {
				fallthroughPairs++
			}
		}
	}
	// The ceiling ratchets down as scripts are ported and verbs deleted; it
	// reaches zero when the family registry goes.
	const fallthroughCeiling = 231
	t.Logf("family pairs falling through: measured %d, ceiling %d", fallthroughPairs, fallthroughCeiling)
	if fallthroughPairs > fallthroughCeiling {
		t.Errorf("%d family pairs fall through without internal; the ceiling is %d", fallthroughPairs, fallthroughCeiling)
	}
}

// TestIntentRouterSuggestsTheCurrentCommand (rule C1): an unknown command
// is refused before any handler runs and names the current command that was
// probably meant, computed from the current table alone. A first word that
// is a current action suggests OBJECT ACTION with the remaining words, one
// line per object that has it; any other word is replaced by the nearest
// current object or action by spelling.
func TestIntentRouterSuggestsTheCurrentCommand(t *testing.T) {
	t.Parallel()
	sentinel := &sentinelFamilies{}
	registered := sentinel.registry()
	for _, row := range []struct {
		args []string
		want []string
	}{
		// Rule 1: a current action as the first word.
		{[]string{"approve", "x"}, []string{"metasystem goal approve x"}},
		{[]string{"sync", "--recover"}, []string{"metasystem goal sync --recover"}},
		{[]string{"finish", "j2:inv-1"}, []string{"metasystem work finish j2:inv-1"}},
		{[]string{"enroll", "--name", "Wido"}, []string{"metasystem system enroll --name Wido"}},
		{[]string{"review", "g"}, []string{"metasystem work review g", "metasystem design review g"}},
		{[]string{"list"}, []string{"metasystem goal list", "metasystem design list", "metasystem decision list", "metasystem grant list",
			"metasystem test list", "metasystem question list", "metasystem incident list", "metasystem machine list"}},
		{[]string{"show", "g", "--json"}, []string{"metasystem goal show g --json", "metasystem design show g --json", "metasystem decision show g --json", "metasystem question show g --json", "metasystem settings show g --json"}},
		// Rule 2: the nearest current object or action by spelling.
		{[]string{"goel", "list"}, []string{"metasystem goal list"}},
		{[]string{"wrk", "land", "g"}, []string{"metasystem work land g"}},
		{[]string{"statsu"}, []string{"metasystem status"}},
		{[]string{"aprove", "g"}, []string{"metasystem goal approve g"}},
	} {
		code, stdout, stderr := routeWith(registered, row.args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "nothing was done") || !strings.Contains(stderr, "metasystem lists the objects") {
			t.Errorf("%v = %d %q %q; want a refusal before any effect", row.args, code, stdout, stderr)
		}
		if got := suggestedLine(stderr); !slices.Equal(got, sortedCopy(row.want)) {
			t.Errorf("%v suggests %q, want %q", row.args, got, row.want)
		}
		if calls := sentinel.taken(); len(calls) != 0 {
			t.Errorf("%v reached a handler: %v", row.args, calls)
		}
	}
	// An unknown action of a current object: its nearest actions by
	// spelling, else the objects that have that exact action.
	for _, row := range []struct {
		args []string
		want string
	}{
		{[]string{"goal", "aprove", "g"}, "did you mean: metasystem goal approve g\n"},
		{[]string{"work", "finsh", "j2:x"}, "did you mean: metasystem work finish j2:x\n"},
		{[]string{"session", "enroll"}, "did you mean: metasystem system enroll\n"},
	} {
		code, stdout, stderr := routeWith(registered, row.args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "nothing was done") || !strings.Contains(stderr, row.want) {
			t.Errorf("%v = %d %q %q; want %q", row.args, code, stdout, stderr, row.want)
		}
	}
	// Every suggestion is a current public command: it routes to its own
	// descriptor.
	for _, word := range []string{"approve", "review", "list", "goel", "wrk", "sync", "finsh"} {
		for _, suggestion := range suggestIntent(word, nil) {
			words := strings.Fields(suggestion)[1:]
			if _, _, ok := resolveIntentArgv(words); !ok && !(len(words) == 1 && isIntentObject(words[0])) {
				t.Errorf("%s suggests %q, which is not a current command", word, suggestion)
			}
		}
	}
	if calls := sentinel.taken(); len(calls) != 0 {
		t.Errorf("a refusal reached a handler: %v", calls)
	}
}

// suggestedLine is the sorted suggestions of a refusal's "did you mean"
// line.
func suggestedLine(stderr string) []string {
	for _, line := range strings.Split(stderr, "\n") {
		if rest, found := strings.CutPrefix(line, "did you mean: "); found {
			return sortedCopy(strings.Split(rest, " | "))
		}
	}
	return nil
}

func sortedCopy(values []string) []string {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted
}

// TestIntentRouterObjectPages: an object alone lists its actions; an
// unknown action is refused and points at the object's page.
func TestIntentRouterObjectPages(t *testing.T) {
	t.Parallel()
	for _, object := range intentObjects() {
		for _, args := range [][]string{{object}, {object, "--help"}, {"help", object}} {
			code, page, problem := routeWith(families(), args...)
			if code != 0 || problem != "" {
				t.Fatalf("%v = %d %q", args, code, problem)
			}
			for _, command := range objectActions(object) {
				if !strings.Contains(page, "\n  "+command.action+" ") {
					t.Errorf("%v does not list %s", args, command.name)
				}
			}
			for _, command := range intentCommands() {
				if command.hidden && command.object == object && strings.Contains(page, "\n  "+command.action+" ") {
					t.Errorf("%v lists the entry %s", args, command.name)
				}
			}
		}
		code, _, problem := routeWith(families(), object, "no-such-action")
		if code != 2 || !strings.Contains(problem, fmt.Sprintf("metasystem %s: unknown action", object)) || !strings.Contains(problem, "metasystem "+object+" lists its actions") {
			t.Errorf("%s no-such-action = %d %q", object, code, problem)
		}
	}
}
