package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// readerSpec selects only the Decision and Readers row needed for textual
// evidence. The accepted unit row, rather than a similar name, owns selection.
func readerSpec(data []byte, unit string) (decision, readers string) {
	var headings []string
	readerColumn := -1
	for _, line := range strings.Split(string(data), "\n") {
		cells := strings.Split(strings.Trim(line, " |\t"), "|")
		if len(cells) > 1 && strings.EqualFold(strings.TrimSpace(cells[0]), "Unit") {
			readerColumn = -1
			for column, cell := range cells {
				if strings.EqualFold(strings.TrimSpace(cell), "Readers") {
					readerColumn = column
				}
			}
		}
		if len(cells) > 1 && strings.Trim(strings.TrimSpace(cells[0]), "`") == unit {
			if readerColumn >= 0 && readerColumn < len(cells) {
				readers += cells[readerColumn] + "\n"
			}
			for _, ref := range regexp.MustCompile(`(?i)Decision\s+[0-9]+`).FindAllString(line, -1) {
				headings = append(headings, strings.ToLower(ref))
			}
		}
	}
	var current string
	selected := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") {
			current = strings.TrimSpace(strings.TrimLeft(line, "#"))
			words := strings.Fields(current)
			selected = len(words) > 0 && (words[0] == unit || strings.HasPrefix(current, unit+" —"))
			for _, heading := range headings {
				selected = selected || strings.EqualFold(current, heading) || strings.HasPrefix(strings.ToLower(current), heading+" ") || strings.HasPrefix(strings.ToLower(current), heading+" —")
			}
		}
		if selected {
			decision += line + "\n"
		}
		if strings.EqualFold(current, "Readers") {
			cells := strings.Split(strings.Trim(line, " |\t"), "|")
			if len(cells) > 1 && strings.Trim(strings.TrimSpace(cells[0]), "`") == unit {
				readers += strings.Join(cells[1:], ",") + "\n"
			}
		}
	}
	return decision, readers
}

func (inv *intentInvocation) briefReaderSections(decision, readers, constraints, base string) string {
	var out strings.Builder
	out.WriteString("\n# Readers\n\n")
	if base == "" {
		out.WriteString("Coverage incomplete: citation validation could not resolve the base. Restore tree access and regenerate the brief.\n")
	} else {
		out.WriteString(project.BriefReaders(decision, readers, inv.layout.GitRoot, base, inv.work().git, inv.work().gitInput))
	}
	out.WriteString("\n# Deletion rules\n\n")
	var declarations []string
	for _, line := range strings.Split(decision+"\n"+constraints, "\n") {
		if regexp.MustCompile(`(?i)^\s*(?:Deletes:\s*|\|\s*Deletion\s*\|)`).MatchString(line) {
			declarations = append(declarations, line)
		}
	}
	if intent := strings.Join(declarations, "\n"); intent != "" {
		fmt.Fprintf(&out, "Declared intent (no deletion is performed by composition):\n\n%s\n", strings.TrimSpace(intent))
		scope := regexp.MustCompile(`\.\s+|[;\n]`).Split(intent, 2)[0]
		if !strings.Contains(scope, "/") && !strings.Contains(scope, "`") {
			out.WriteString(intentMissingDecision + " the deletion scope; clarify the exact ids or paths in the accepted design before deleting.\n")
		}
		out.WriteString("Refuse empty ids. Report errors; never discard them. Never widen declared paths. Name protected paths explicitly and preserve them. A dry run must precede deletion.\n")
	} else {
		out.WriteString("This unit declares no deletion intent.\n")
	}
	out.WriteString("\n# Effort\n\nComposer effort: empty; launch.build.effort decides. An authorized explicit override remains owned by work build.\n")
	return out.String()
}
