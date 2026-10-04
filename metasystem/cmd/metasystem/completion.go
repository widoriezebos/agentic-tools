package main

// Shell Tab completion (design shell-tab-completion). The shell's glue, which
// system completion prints, starts this binary as the one-word entrypoint
// __complete with the words typed so far; every answer comes from the
// router's own tables and, for goal ids, from one read of the ledger
// directory of the checkout the command acts on. A Tab runs no git, takes no
// lock, and writes no journal, receipt or record.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// completeEntryUsage is the entry's usage line, as help and refusals print it.
const completeEntryUsage = "usage: metasystem internal __complete -- WORD..."

// completionZshScript is the zsh glue. It calls back through the command as
// typed, unquoted, with every word up to the cursor, the empty cursor word
// included; it guesses nothing about a word's shape and moves the first N
// characters out of the completed word only on the engine's :prefix N.
const completionZshScript = `_metasystem() {
  local -a lines specs; local line
  lines=(${(f)"$("${(Q)words[1]}" __complete -- "${(@Q)words[2,CURRENT]}" 2>/dev/null)"})
  [[ $lines[1] == ':prefix '* ]] && { compset -p ${lines[1]#:prefix }; shift lines }
  [[ $lines[1] == ':files' ]] && { _files; return }
  for line in $lines; specs+=("${line%%$'\t'*}:${line#*$'\t'}")
  _describe -t metasystem 'metasystem' specs
}
compdef _metasystem metasystem
`

// completionBashScript is the bash glue, for bash 3.2 and later (no
// compopt). It registers with -o default, not -o filenames, so a word the
// engine offers is inserted as it is even where a directory of that name
// exists. On :files it offers nothing and readline's own file-name
// completion takes over, which handles spaces, quotes and directories. The
// accepted side effect, bash only: a position where the engine offers nothing
// (a --reason value, say) also falls back to file names. bash splits a joined
// --name=value at the =, so the joined form is not completed.
const completionBashScript = `_metasystem() {
  local line; local -a lines
  COMPREPLY=()
  while IFS= read -r line; do lines[${#lines[@]}]=$line; done < <("${COMP_WORDS[0]}" __complete -- "${COMP_WORDS[@]:1:COMP_CWORD}" 2>/dev/null)
  case ${lines[0]} in
    ':prefix '*|':files') return 0 ;;
  esac
  for line in "${lines[@]}"; do COMPREPLY[${#COMPREPLY[@]}]=${line%%$'\t'*}; done
}
complete -o default -F _metasystem metasystem
`

// completionScripts are the shells system completion prints glue for.
var completionScripts = map[string]string{"zsh": completionZshScript, "bash": completionBashScript}

// runIntentSystemCompletion prints one shell's glue; it reads nothing.
func runIntentSystemCompletion(inv *intentInvocation) int {
	shell := ""
	if len(inv.input.args) == 1 {
		shell = inv.input.args[0]
	}
	script, ok := completionScripts[shell]
	if !ok {
		summary := "system completion needs the shell: zsh or bash; nothing was done"
		if shell != "" {
			summary = fmt.Sprintf("system completion knows zsh and bash, not %s; nothing was done", shellCommand([]string{shell}))
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: summary,
			next: inv.publicArgv("system", "completion", "zsh"), nextReason: "the glue for zsh; bash is the other shell"})
	}
	if inv.input.switched("json") {
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: "the " + shell + " completion script",
			Data: map[string]string{"shell": shell, "script": script}})
	}
	_, _ = io.WriteString(inv.stdout, script)
	return 0
}

// completeOwners are what a completion reads besides the tables: the
// directory the shell runs in and the resolver that finds the command's
// checkout from it.
type completeOwners struct {
	cwd      string
	resolver stateroot.Resolver
}

// runCompleteEntry is the __complete entrypoint as the shell starts it: the
// working directory and a resolver whose repository walk is pure and whose
// executable reader is never consulted.
func runCompleteEntry(args []string, stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	return runComplete(args, stdout, stderr, completeOwners{cwd: cwd, resolver: stateroot.NewResolver(nearestGitTop, noCompletionExecutable)})
}

// runComplete answers one completion request: -- then the shell words after
// the command name, the last being the word under the cursor. It prints
// directives (:prefix N, :files) and then one candidate per line, WORD, a tab
// and a description. Everything after -- exits 0 with nothing on stderr.
func runComplete(args []string, stdout, stderr io.Writer, owners completeOwners) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, completeEntryUsage)
		return 2
	}
	switch {
	case isHelpWord(args[0]):
		fmt.Fprintln(stdout, completeEntryUsage)
		fmt.Fprintln(stdout, "Your shell's completion script calls it; metasystem system completion SHELL prints that script.")
		return 0
	case args[0] != "--":
		fmt.Fprintf(stderr, "__complete does not take %s; nothing was done\n", args[0])
		fmt.Fprintln(stderr, completeEntryUsage)
		return 2
	}
	words := args[1:]
	if len(words) == 0 {
		words = []string{""}
	}
	answer := completeWords(words, owners)
	var out strings.Builder
	for _, directive := range answer.directives {
		out.WriteString(directive + "\n")
	}
	for _, candidate := range answer.candidates {
		out.WriteString(candidate.word + "\t" + strings.Join(strings.Fields(candidate.description), " ") + "\n")
	}
	_, _ = io.WriteString(stdout, out.String())
	return 0
}

type completionCandidate struct{ word, description string }

type completionAnswer struct {
	directives []string
	candidates []completionCandidate
}

// offer keeps the candidates the cursor word is a prefix of, each word once.
func (answer *completionAnswer) offer(current string, candidates ...completionCandidate) {
	for _, candidate := range candidates {
		if !strings.HasPrefix(candidate.word, current) {
			continue
		}
		if slices.ContainsFunc(answer.candidates, func(offered completionCandidate) bool { return offered.word == candidate.word }) {
			continue
		}
		answer.candidates = append(answer.candidates, candidate)
	}
}

// completeWords decides one position from the router's tables: word 1, an
// object's actions, help's selectors, the internal entrypoints, or a row's
// options and values.
func completeWords(words []string, owners completeOwners) completionAnswer {
	var answer completionAnswer
	prior, current := words[:len(words)-1], words[len(words)-1]
	if len(prior) == 0 {
		for _, object := range intentObjects() {
			answer.offer(current, completionCandidate{object, intentObjectSummaries[object]})
		}
		answer.offer(current,
			completionCandidate{"status", intentObjectSummaries["status"]},
			completionCandidate{"help", "the help pages: metasystem help [OBJECT [ACTION]]"},
			completionCandidate{"internal", "maintainer reference: the process entrypoints and the programs that start them"})
		return answer
	}
	switch first := prior[0]; {
	case first == "help":
		completeHelp(&answer, prior[1:], current)
	case first == "internal":
		completeInternal(&answer, prior[1:], current)
	case first == "status":
		command, _ := findIntentCommand("status")
		completeRow(&answer, command, prior[1:], current, owners)
	case isIntentObject(first):
		if len(prior) == 1 {
			for _, command := range objectActions(first) {
				answer.offer(current, completionCandidate{command.action, command.summary})
			}
			return answer
		}
		if command, ok := findIntentAction(first, prior[1]); ok && !command.hidden {
			completeRow(&answer, command, prior[2:], current, owners)
		}
	}
	return answer
}

// completeHelp offers help's selectors: an object, then its actions.
func completeHelp(answer *completionAnswer, selectors []string, current string) {
	selectors = slices.DeleteFunc(slices.Clone(selectors), func(word string) bool { return word == "--json" })
	switch len(selectors) {
	case 0:
		for _, object := range intentObjects() {
			answer.offer(current, completionCandidate{object, intentObjectSummaries[object]})
		}
	case 1:
		for _, command := range objectActions(selectors[0]) {
			answer.offer(current, completionCandidate{command.action, command.summary})
		}
	}
}

// completeInternal offers every family and one-word entrypoint, then a
// family's verbs; an entrypoint's own options are its parser's.
func completeInternal(answer *completionAnswer, words []string, current string) {
	registered := families()
	switch len(words) {
	case 0:
		for _, fam := range registered {
			answer.offer(current, completionCandidate{fam.name, fam.summary})
		}
		for _, entry := range topLevelEntries() {
			answer.offer(current, completionCandidate{entry.name, "process entrypoint started by " + entry.launcher})
		}
	case 1:
		for _, fam := range registered {
			if fam.name != words[0] {
				continue
			}
			for _, v := range fam.verbs {
				answer.offer(current, completionCandidate{v.name, v.summary})
			}
		}
	}
}

// completionFlag finds a row's option by any spelling, as the row's parser
// does: every option of a parsed row, a passthrough row's documented ones.
func completionFlag(command intentCommand, name string) (intentFlag, bool) {
	for _, candidate := range command.helpFlags() {
		if candidate.name == name || slices.Contains(candidate.aliases, name) {
			return candidate, true
		}
	}
	return intentFlag{}, false
}

// completionWalk is what the walk over a row's earlier words found.
type completionWalk struct {
	positionals int
	// valueOf is the option whose separate value the cursor word is.
	valueOf *intentFlag
	// dashes: `--` ended the options; every later word is positional.
	dashes bool
	// done: a rest option took every later word, or an unknown option makes
	// the line one the parser refuses.
	done bool
	// repo is the --repo (or --root) value typed earlier on the line.
	repo string
}

// walkCompletion walks a row's words before the cursor as parseIntentArgs
// does: `--` ends the options, an option with a value consumes the next word
// whatever its spelling, a joined --name=value consumes none, a rest option
// takes everything after it, and the remaining words are positionals.
func walkCompletion(command intentCommand, words []string) completionWalk {
	var walk completionWalk
	for index := 0; index < len(words); index++ {
		token := words[index]
		switch {
		case walk.dashes:
			walk.positionals++
			continue
		case token == "--":
			walk.dashes = true
			continue
		case token == "-h" || token == "--help" || token == "-help":
			continue
		case len(token) < 2 || token[0] != '-' || strings.HasPrefix(token, "---"):
			walk.positionals++
			continue
		}
		name, value, joined := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(token, "-"), "-"), "=")
		definition, known := completionFlag(command, name)
		if !known && command.passthrough != nil {
			// The owner's parser is its own; an option its row does not
			// document is read as a switch.
			continue
		}
		if !known || definition.rest {
			walk.done = true
			return walk
		}
		if definition.value == "" {
			continue
		}
		if !joined {
			if index+1 == len(words) {
				walk.valueOf = &definition
				return walk
			}
			index++
			value = words[index]
		}
		if definition.name == "repo" && walk.repo == "" {
			walk.repo = value
		}
	}
	return walk
}

// completeRow answers the cursor word after a row's action: a separate
// option value, a joined option's value behind a :prefix directive, an option
// name, or the next positional.
func completeRow(answer *completionAnswer, command intentCommand, words []string, current string, owners completeOwners) {
	walk := walkCompletion(command, words)
	switch {
	case walk.done:
		return
	case walk.valueOf != nil:
		completeValue(answer, walk.valueOf.value, current, walk.repo, owners)
		return
	case walk.dashes, !strings.HasPrefix(current, "-"), strings.HasPrefix(current, "---"):
		completePositional(answer, command, walk.positionals+1, current, walk.repo, owners)
		return
	}
	if spelled, part, joined := strings.Cut(current, "="); joined {
		definition, known := completionFlag(command, strings.TrimPrefix(strings.TrimPrefix(spelled, "-"), "-"))
		if !known {
			return
		}
		answer.directives = append(answer.directives, fmt.Sprintf(":prefix %d", len(spelled)+1))
		if !definition.rest {
			completeValue(answer, definition.value, part, walk.repo, owners)
		}
		return
	}
	for _, definition := range command.helpFlags() {
		if !definition.hidden {
			answer.offer(current, completionCandidate{"--" + definition.name, definition.usage})
		}
	}
}

// completionChoices is a lowercase choice list such as human|main or 1|2|3.
var completionChoices = regexp.MustCompile(`^[a-z0-9-]+(\|[a-z0-9-]+)+$`)

// completeValue offers what a value spelling names, and nothing for any
// other spelling: goal ids for G and G2, file names for FILE, PATH and DIR,
// the words of a lowercase choice list.
func completeValue(answer *completionAnswer, spelling, current, repo string, owners completeOwners) {
	switch kind := strings.TrimSuffix(spelling, "..."); {
	case kind == "G" || kind == "G2":
		completeGoalIDs(answer, current, repo, owners)
	case kind == "FILE" || kind == "PATH" || kind == "DIR":
		if !slices.Contains(answer.directives, ":files") {
			answer.directives = append(answer.directives, ":files")
		}
	case completionChoices.MatchString(kind):
		for _, word := range strings.Split(kind, "|") {
			answer.offer(current, completionCandidate{word, ""})
		}
	}
}

// completePositional offers the k-th positional by the k-th placeholder
// after the action across the row's usage lines; a trailing G... or FILE...
// covers every later position.
func completePositional(answer *completionAnswer, command intentCommand, k int, current, repo string, owners completeOwners) {
	var spellings []string
	for _, line := range command.allUsage() {
		placeholders := usagePlaceholders(command, line)
		switch {
		case k <= len(placeholders):
			spellings = append(spellings, placeholders[k-1].spelling)
		case len(placeholders) > 0 && placeholders[len(placeholders)-1].variadic:
			spellings = append(spellings, placeholders[len(placeholders)-1].spelling)
		}
	}
	slices.Sort(spellings)
	for _, spelling := range slices.Compact(spellings) {
		completeValue(answer, spelling, current, repo, owners)
	}
}

type usagePlaceholder struct {
	spelling string
	variadic bool
}

// usagePlaceholders are one usage line's positional placeholders after the
// action, in order, brackets removed: option words and their value
// placeholders are skipped, and an alternative after a lone | shares the
// position before it.
func usagePlaceholders(command intentCommand, line string) []usagePlaceholder {
	rest, ok := strings.CutPrefix(line, "metasystem "+command.name)
	if !ok {
		return nil
	}
	tokens := usageTokens(rest)
	var placeholders []usagePlaceholder
	alternative := false
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if token == "|" {
			alternative = true
			continue
		}
		if token == "..." {
			continue
		}
		bare := strings.TrimLeft(token, "[(")
		if strings.HasPrefix(bare, "-") || strings.Contains(bare, "|-") {
			parts := strings.Split(bare, "|")
			option := strings.TrimRight(parts[len(parts)-1], "]).")
			name, _, joined := strings.Cut(strings.TrimLeft(option, "-"), "=")
			definition, known := completionFlag(command, name)
			switch {
			case known && definition.value != "" && !joined:
				index++
			case !known && !joined && !strings.HasSuffix(token, "]") && !strings.HasSuffix(token, ")") &&
				index+1 < len(tokens) && !strings.HasPrefix(strings.TrimLeft(tokens[index+1], "[("), "-") && tokens[index+1] != "|":
				// An option the row does not document takes the placeholder
				// that follows it inside the same bracket.
				index++
			}
			alternative = false
			continue
		}
		if alternative && len(placeholders) > 0 {
			alternative = false
			continue
		}
		spelling := strings.TrimRight(bare, "])")
		if strings.Trim(spelling, ".") == "" {
			continue
		}
		variadic := strings.HasSuffix(spelling, "...")
		spelling = strings.TrimRight(strings.TrimSuffix(spelling, "..."), "])")
		placeholders = append(placeholders, usagePlaceholder{spelling: spelling, variadic: variadic})
	}
	return placeholders
}

// usageTokens splits a usage line at spaces, keeping a quoted example such as
// 'LABEL: CONSEQUENCE' one token.
func usageTokens(line string) []string {
	var tokens []string
	var token strings.Builder
	quoted := false
	for _, r := range line {
		switch {
		case r == '\'':
			quoted = !quoted
			token.WriteRune(r)
		case r == ' ' && !quoted:
			if token.Len() > 0 {
				tokens = append(tokens, token.String())
				token.Reset()
			}
		default:
			token.WriteRune(r)
		}
	}
	if token.Len() > 0 {
		tokens = append(tokens, token.String())
	}
	return tokens
}

// completeGoalIDs offers the open goals of the checkout the command acts on,
// each with its state, sorted by id.
func completeGoalIDs(answer *completionAnswer, current, repo string, owners completeOwners) {
	dir, ok := completionGoalDir(owners, repo)
	if !ok {
		return
	}
	answer.offer(current, readCompletionGoals(dir)...)
}

// completionGoalDir is the goal directory of the checkout the command acts
// on, found as selectRoot finds it: --repo (joined to the working directory
// when relative), else the working directory, through ResolveLayout and then
// RootForInstallation.
func completionGoalDir(owners completeOwners, repo string) (string, bool) {
	path := owners.cwd
	if repo != "" {
		path = repo
		if !filepath.IsAbs(path) {
			path = filepath.Join(owners.cwd, path)
		}
	}
	layout, err := owners.resolver.ResolveLayout(path)
	if err != nil {
		return "", false
	}
	root, err := owners.resolver.RootForInstallation(layout.InstallationRoot)
	if err != nil {
		return "", false
	}
	relative, err := stateroot.RelativeRoot(stateroot.Goals)
	if err != nil {
		return "", false
	}
	return root.Path(filepath.FromSlash(relative)), true
}

// completionGoalHead is how much of a goal file is read for its State line.
const completionGoalHead = 4096

// readCompletionGoals reads one directory and, per goal file, one read of at
// most 4 KiB for the State line; the id is the file's stem. A file without a
// State line there (the ledger's root record) or that cannot be read is
// skipped.
func readCompletionGoals(dir string) []completionCandidate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	head := make([]byte, completionGoalHead)
	var goals []completionCandidate
	for _, entry := range entries {
		id, isGoal := strings.CutSuffix(entry.Name(), ".md")
		if !isGoal || entry.IsDir() {
			continue
		}
		if state, ok := completionGoalState(filepath.Join(dir, entry.Name()), head); ok {
			goals = append(goals, completionCandidate{id, state})
		}
	}
	sort.Slice(goals, func(i, j int) bool { return goals[i].word < goals[j].word })
	return goals
}

func completionGoalState(path string, head []byte) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	n, err := file.Read(head)
	if err != nil && n == 0 {
		return "", false
	}
	for _, line := range bytes.Split(head[:n], []byte("\n")) {
		if state, ok := bytes.CutPrefix(line, []byte("- State: ")); ok {
			return strings.TrimSpace(string(state)), true
		}
	}
	return "", false
}

// nearestGitTop is the repository top without git: the nearest ancestor
// holding a .git entry, file or directory, which is what git rev-parse
// --show-toplevel answers once the steering variables are scrubbed.
func nearestGitTop(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for dir := absolute; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			return "", fmt.Errorf("state root: installation %w: %s", stateroot.ErrNotInRepository, absolute)
		}
	}
}

// noCompletionExecutable is the resolver's executable reader for a
// completion: the command's checkout is never the executable's installation.
func noCompletionExecutable() (string, error) {
	return "", errors.New("completion reads the checkout the command acts on, never the executable's installation")
}
