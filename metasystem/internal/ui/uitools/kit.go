package uitools

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The kit's own knowledge, answered from the kit's own owners.
//
// The interface's manifest says what this interface is; the project's records
// say what this project decided; this says what the metasystem itself means.
// Four owners, each already written down for the humans and agents who work
// here, and none of them copied into anything written for the Partner:
//
//   - the glossary, which AGENTS.md points at, defines the system's terms;
//   - the command catalogue in cmd/metasystem owns the verbs and what each
//     one does, in the same table the binary routes with;
//   - the rulings register owns every standing human ruling;
//   - wow.md owns the routes, and a routed skill is named and summarised from
//     its own front matter, never quoted whole.
//
// A workflow is READ here so it can be explained. Undertaking one is a
// different act, and not one this Partner can perform.

// Kit is where this kit's knowledge lives, for one installation.
type Kit struct {
	// Root is the metasystem installation this checkout carries. Empty is a
	// build that cannot reach the kit, which the result says.
	Root string
	// Commands is the engine's command catalogue, given by the process that
	// owns it so that this package never restates a verb or its description.
	Commands func() []CommandFamily
}

// CommandFamily is one family of the engine's verbs, as the binary routes it.
// An empty Name is the public command table, whose commands follow the
// executable name directly.
type CommandFamily struct {
	Name    string
	Summary string
	Verbs   []Command
}

// Command is one verb and what it does, in the catalogue's own words.
type Command struct {
	Name    string
	Summary string
	Scope   string
	// Usage is the command's accepted forms, each a complete command line.
	Usage []string
	// AdministrationUsage identifies forms that manage MetaSystem itself.
	AdministrationUsage []string
}

// The kit files this tool reads that are named rather than pointed at.
// AGENTS.md points at the glossary; wow.md and the rulings register are the
// kit's own fixed names, which AGENTS.md and wow.md themselves rely on.
const (
	contractFile = "AGENTS.md"
	routesFile   = "wow.md"
	rulingsFile  = "memory/rulings.md"
)

// entry is one thing this tool can answer with: where it came from, what it is
// called, and what it says.
type entry struct {
	owner string
	from  string
	name  string
	text  string
}

func (e entry) line() string {
	return "- " + e.owner + " · " + e.name + ": " + e.text + " (" + e.from + ")"
}

// kit answers one topic from the kit's own owners.
func (r Readers) kit(topic, cursor string) Result {
	root := strings.TrimSpace(r.Kit.Root)
	if root == "" && r.Kit.Commands == nil {
		return Result{Problem: "this build cannot reach the kit's own documents"}
	}
	entries, sources, problems := r.kitEntries(root)
	source := "this kit as it stands: " + join(sources)
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return bounded(source, kitIndex(entries, sources, problems), 1, 1)
	}
	lines := make([]string, 0, 16)
	for _, one := range entries {
		if !about(one.name+" "+one.text+" "+one.owner, topic) {
			continue
		}
		lines = append(lines, one.line())
	}
	answered := paged(source, lines, cursor)
	if len(lines) == 0 {
		answered.Body = "Nothing in the kit's glossary, command catalogue, rulings register or routes names " +
			topic + ". Call this tool with no topic to see what it can answer from.\n"
	}
	for _, problem := range problems {
		answered.Body += "- This answer is short one source: " + problem + "\n"
	}
	return answered
}

// about reports whether one entry answers a topic. Every word of the topic has
// to appear somewhere in the entry, in any order: a human asks about a "lease
// epoch" and the glossary writes the two words in one sentence rather than in
// that phrase, and an entry matched on the whole phrase would be missed.
func about(entry, topic string) bool {
	for _, word := range strings.Fields(topic) {
		if !contains(entry, word) {
			return false
		}
	}
	return true
}

// kitIndex is what a reader is given when it names no topic: the owners, where
// each one lives, and how much each holds.
func kitIndex(entries []entry, sources, problems []string) string {
	counts := map[string]int{}
	order := []string{}
	for _, one := range entries {
		if _, seen := counts[one.owner]; !seen {
			order = append(order, one.owner)
		}
		counts[one.owner]++
	}
	var built strings.Builder
	built.WriteString("This tool answers about the metasystem itself, from the kit's own owners:\n")
	for _, owner := range order {
		built.WriteString("- " + owner + ": " + itoa(counts[owner]) + " entries\n")
	}
	built.WriteString("Read from: " + join(sources) + "\n")
	for _, problem := range problems {
		built.WriteString("- Could not be read: " + problem + "\n")
	}
	built.WriteString("Call it again with a topic — a term, a verb, a ruling id, a workflow — to be answered from them.\n")
	built.WriteString("Observations of the ledger are not here: which goals exist and where they stand come from the board, goal and search tools, and are never explanations of the rules.\n")
	return built.String()
}

// kitEntries reads the four owners, and says which of them it could not read
// rather than answering as if they were empty.
func (r Readers) kitEntries(root string) (entries []entry, sources, problems []string) {
	if r.Kit.Commands != nil {
		for _, family := range r.Kit.Commands() {
			for _, verb := range family.Verbs {
				name, text := "metasystem "+verb.Name, verb.Summary
				if verb.Scope == "administration" {
					text = "MetaSystem administration: " + text
				}
				if family.Name != "" {
					name = "metasystem " + family.Name + " " + verb.Name
					text += " (" + family.Name + ": " + family.Summary + ")"
				}
				if len(verb.Usage) > 0 {
					text += "; usage: " + strings.Join(verb.Usage, "; ")
				}
				if len(verb.AdministrationUsage) > 0 {
					text += "; MetaSystem administration forms: " + strings.Join(verb.AdministrationUsage, "; ")
				}
				entries = append(entries, entry{
					owner: "command", from: "the engine's own command catalogue",
					name: name, text: text,
				})
			}
		}
		sources = append(sources, "the engine's command catalogue")
	} else {
		problems = append(problems, "this process carries no command catalogue")
	}
	if root == "" {
		problems = append(problems, "this build cannot reach the kit's own documents")
		return entries, sources, problems
	}

	glossary, pointer := glossaryPath(root)
	switch {
	case glossary == "":
		problems = append(problems, "AGENTS.md does not point at a glossary")
	default:
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(glossary)))
		if err != nil {
			problems = append(problems, glossary+" could not be read: "+err.Error())
			break
		}
		entries = append(entries, glossaryEntries(glossary, string(body))...)
		sources = append(sources, glossary+" (named by "+contractFile+": "+pointer+")")
	}

	if body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rulingsFile))); err != nil {
		problems = append(problems, rulingsFile+" could not be read: "+err.Error())
	} else {
		entries = append(entries, rulingEntries(rulingsFile, string(body))...)
		sources = append(sources, rulingsFile)
	}

	if body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(routesFile))); err != nil {
		problems = append(problems, routesFile+" could not be read: "+err.Error())
	} else {
		entries = append(entries, routeEntries(root, routesFile, string(body))...)
		sources = append(sources, routesFile)
	}
	return entries, sources, problems
}

// backticked names a path a document points at, in the only way this kit
// writes one: between back quotes, ending in .md.
var backticked = regexp.MustCompile("`([^`]+\\.md)`")

// glossaryPath is where the contract says the system's terms are defined. The
// path is read out of AGENTS.md rather than written here, because the contract
// is what a maintainer edits when the glossary moves.
func glossaryPath(root string) (path, pointer string) {
	body, err := os.ReadFile(filepath.Join(root, contractFile))
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.Contains(strings.ToLower(line), "glossary") {
			continue
		}
		for _, found := range backticked.FindAllStringSubmatch(line, -1) {
			if strings.Contains(strings.ToLower(found[1]), "glossary") {
				return found[1], oneLine(strings.TrimLeft(line, "- "))
			}
		}
	}
	return "", ""
}

// definition is one glossary entry: a bullet opening with a bold term, with
// anything in brackets after it, and the definition after a dash.
var definition = regexp.MustCompile(`^\s*[-*]\s+\*\*(.+?)\*\*\s*(.*)$`)

// glossaryEntries reads the glossary's own bullets. A definition runs to the
// next bullet or blank line, so a term explained in three lines is answered in
// three lines rather than one.
func glossaryEntries(from, body string) []entry {
	lines := splitLines(body)
	entries := []entry{}
	for index := 0; index < len(lines); index++ {
		found := definition.FindStringSubmatch(lines[index])
		if found == nil {
			continue
		}
		text := found[2]
		for next := index + 1; next < len(lines); next++ {
			trimmed := strings.TrimSpace(lines[next])
			if trimmed == "" || definition.MatchString(lines[next]) || strings.HasPrefix(trimmed, "#") {
				break
			}
			text += " " + trimmed
			index = next
		}
		// The glossary writes a term, an optional qualifier naming the field or
		// script that carries it, then a dash and the definition. The
		// qualifier belongs to the name — it is how the term is spelled in the
		// code — and the definition is what the entry says.
		name := oneLine(found[1])
		if at := strings.Index(text, "—"); at >= 0 {
			if qualifier := oneLine(text[:at]); qualifier != "" {
				name += " " + qualifier
			}
			text = text[at+len("—"):]
		}
		entries = append(entries, entry{
			owner: "glossary", from: from,
			name: name, text: oneLine(strings.TrimLeft(text, "- ")),
		})
	}
	return entries
}

// rulingEntries reads the register's own table: one row per standing human
// ruling. The prose above the table is the register's law about itself and is
// not a ruling, so it is not read as one.
func rulingEntries(from, body string) []entry {
	entries := []entry{}
	for _, line := range splitLines(body) {
		cells := tableRow(line)
		if len(cells) < 4 || strings.EqualFold(cells[0], "id") || isRule(cells[0]) {
			continue
		}
		text := cells[2]
		if cells[3] != "" {
			text += " Context: " + cells[3]
		}
		if len(cells) > 4 && cells[4] != "" {
			text += " Owner: " + cells[4] + "."
		}
		entries = append(entries, entry{
			owner: "ruling", from: from,
			name: cells[0] + " (" + cells[1] + ")", text: oneLine(text),
		})
	}
	return entries
}

// routeEntries reads wow.md's own table, and names the skill a route points at
// from the skill's own front matter. A skill's body is never carried here: a
// route says where to look, and reading a workflow in order to explain it is
// the tool call after this one.
func routeEntries(root, from, body string) []entry {
	entries := []entry{}
	for _, line := range splitLines(body) {
		cells := tableRow(line)
		if len(cells) < 3 || strings.EqualFold(cells[0], "need") || isRule(cells[0]) {
			continue
		}
		text := "Owned by " + cells[1] + ". Load it when: " + cells[2] + "."
		if named, said := skillSummary(root, cells[1]); named != "" {
			text += " The skill is " + named + ": " + said
		}
		entries = append(entries, entry{
			owner: "route", from: from, name: oneLine(cells[0]), text: oneLine(text),
		})
	}
	return entries
}

// skillSummary is a routed skill's own name and description line, from its
// front matter and nowhere else.
func skillSummary(root, owner string) (string, string) {
	found := backticked.FindStringSubmatch(owner)
	if found == nil || !strings.HasSuffix(found[1], "SKILL.md") {
		return "", ""
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(found[1])))
	if err != nil {
		return "", ""
	}
	name, description := "", ""
	for _, line := range splitLines(string(body)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && name != "" {
			break
		}
		if rest, is := strings.CutPrefix(trimmed, "name:"); is {
			name = strings.TrimSpace(rest)
		}
		if rest, is := strings.CutPrefix(trimmed, "description:"); is {
			description = strings.TrimSpace(rest)
		}
	}
	return name, description
}

// tableRow splits one Markdown table row into its cells, or nothing for a line
// that is not one.
func tableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return nil
	}
	parts := strings.Split(strings.Trim(trimmed, "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// isRule reports the separator row under a table's head.
func isRule(cell string) bool {
	return strings.Trim(cell, "-: ") == "" && cell != ""
}

func splitLines(body string) []string {
	return strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
}

func join(values []string) string {
	if len(values) == 0 {
		return "nothing this process could read"
	}
	return strings.Join(values, "; ")
}

func itoa(count int) string {
	return strconv.Itoa(count)
}
