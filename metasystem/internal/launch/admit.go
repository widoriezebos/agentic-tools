package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type UnitSize struct {
	Name  string
	Lines int64
}
type ReadChoice struct {
	Mode        string
	Lines       int64
	Directories map[string]int64
}

var declaredSizePattern = regexp.MustCompile(`(?mi)^Declared size:\s*([0-9]+)\s+changed lines\s*$`)
var firstInteger = regexp.MustCompile(`[0-9]+`)
var witnessInteger = regexp.MustCompile(`(?i)witness[^0-9]*([0-9]+)`)

func EstimateTokens(bytes int64) int64 {
	if bytes <= 0 {
		return 0
	}
	return (bytes + 3) / 4
}

func (m *Manager) Admit(spec StartSpec) error {
	settings, err := m.resolvedSettings()
	if err == nil {
		err = m.admit(spec, settings)
	}
	if err != nil && strings.HasPrefix(err.Error(), "LAUNCH_") {
		now := time.Now
		if m.Now != nil {
			now = m.Now
		}
		_ = m.Store.AppendRefusal(Refusal{Time: now().UTC().Format(time.RFC3339Nano), Code: strings.Fields(err.Error())[0], Kind: spec.Kind, Goal: spec.Goal, Tag: spec.Tag, Numbers: refusalNumbers(err.Error())})
	}
	return err
}

func (m *Manager) admit(spec StartSpec, settings Settings) error {
	paths := append([]string{spec.Brief}, spec.Inputs...)
	if spec.Page != "" {
		paths = append(paths, spec.Page)
	}
	if spec.Kind == "critique" {
		var common string
		codexAdapter := false
		switch adapter := m.Adapters["codex-exec"].(type) {
		case CodexExec:
			common = adapter.CommonTemplate
			codexAdapter = true
		case *CodexExec:
			common = adapter.CommonTemplate
			codexAdapter = true
		}
		if codexAdapter {
			if common == "" {
				common = filepath.Join("scripts", "agents", "templates", "design-common.md")
			}
			paths = append(paths, common)
		}
	}
	type measured struct {
		path   string
		tokens int64
	}
	var total int64
	var values []measured
	for _, path := range paths {
		if path == "" {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			return statErr
		}
		tokens := EstimateTokens(info.Size())
		total += tokens
		values = append(values, measured{path, tokens})
	}
	if total > settings.BriefCap {
		sort.SliceStable(values, func(i, j int) bool { return values[i].tokens > values[j].tokens })
		lines := []string{fmt.Sprintf("LAUNCH_BRIEF_OVERSIZE total=%d cap=%d", total, settings.BriefCap)}
		for _, value := range values {
			lines = append(lines, fmt.Sprintf("input=%s tokens=%d", value.path, value.tokens))
		}
		return fmt.Errorf("%s", strings.Join(lines, "\n"))
	}
	if spec.Kind == "build" {
		units, size, sizeErr := buildSize(spec)
		if sizeErr != nil {
			return sizeErr
		}
		if size > settings.BuildLinesCap {
			lines := []string{fmt.Sprintf("LAUNCH_BUILD_OVERSIZE size=%d cap=%d", size, settings.BuildLinesCap)}
			if len(units) > 0 {
				lines = append(lines, serialSplit(units, settings.BuildLinesCap)...)
			}
			return fmt.Errorf("%s", strings.Join(lines, "\n"))
		}
	}
	if spec.Kind == "read" && spec.DiffFile != "" {
		choice, choiceErr := ChooseReadMode(spec.DiffFile, settings.ReadSplitLines)
		if choiceErr != nil {
			return choiceErr
		}
		_, packageExists := choice.Directories[spec.Package]
		follows := choice.Mode == "whole" && spec.Package == "" && !spec.Wide || choice.Mode == "package" && spec.Package != "" && packageExists && !spec.Wide || choice.Mode == "wide" && spec.Wide && spec.Package == ""
		if !follows {
			lines := []string{fmt.Sprintf("LAUNCH_READ_UNSPLIT choice=%s", choice.Mode)}
			type pair struct {
				name  string
				lines int64
			}
			var pairs []pair
			for name, count := range choice.Directories {
				pairs = append(pairs, pair{name, count})
			}
			sort.Slice(pairs, func(i, j int) bool {
				if pairs[i].lines == pairs[j].lines {
					return pairs[i].name < pairs[j].name
				}
				return pairs[i].lines > pairs[j].lines
			})
			for _, pair := range pairs {
				lines = append(lines, fmt.Sprintf("directory=%s lines=%d", pair.name, pair.lines))
			}
			return fmt.Errorf("%s", strings.Join(lines, "\n"))
		}
	}
	return nil
}

func buildSize(spec StartSpec) ([]UnitSize, int64, error) {
	if spec.UnitsPage != "" || len(spec.Units) > 0 {
		if spec.UnitsPage == "" {
			return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED missing=units-page")
		}
		if len(spec.Units) == 0 {
			return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED missing=unit")
		}
		data, err := os.ReadFile(spec.UnitsPage)
		if err != nil {
			return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED missing=units-page: %v", err)
		}
		return sizesFromTable(string(data), spec.Units)
	}
	data, err := os.ReadFile(spec.Brief)
	if err != nil {
		return nil, 0, err
	}
	match := declaredSizePattern.FindSubmatch(data)
	if len(match) != 2 {
		return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED missing=declared-size")
	}
	size, _ := strconv.ParseInt(string(match[1]), 10, 64)
	return nil, size, nil
}

func sizesFromTable(page string, wanted []string) ([]UnitSize, int64, error) {
	lines := strings.Split(page, "\n")
	header, sizeColumn := -1, -1
	tableFound := false
	for index, line := range lines {
		cells := tableCells(line)
		if len(cells) == 0 || !strings.EqualFold(strings.TrimSpace(cells[0]), "unit") {
			continue
		}
		tableFound = true
		for column, cell := range cells {
			name := strings.ToLower(strings.TrimSpace(cell))
			if strings.Contains(name, "line") || name == "size" || name == "alloc" || name == "cap" {
				sizeColumn = column
				break
			}
		}
		if sizeColumn >= 0 {
			header = index
			break
		}
	}
	if header < 0 {
		if tableFound {
			return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED missing=size-column")
		}
		return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED missing=units-table")
	}
	wants := map[string]bool{}
	for _, name := range wanted {
		wants[name] = true
	}
	seen := map[string]bool{}
	var result []UnitSize
	for _, line := range lines[header+1:] {
		cells := tableCells(line)
		if len(cells) <= sizeColumn || separatorRow(cells) {
			continue
		}
		name := strings.TrimSpace(cells[0])
		matched := ""
		for wanted := range wants {
			if name == wanted || strings.HasPrefix(name, wanted+".") || strings.HasPrefix(name, wanted+" ") {
				matched = wanted
				break
			}
		}
		if matched == "" || seen[matched] {
			continue
		}
		cell := cells[sizeColumn]
		first := firstInteger.FindString(cell)
		if first == "" {
			return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED unit=%s missing=integer", matched)
		}
		lines, _ := strconv.ParseInt(first, 10, 64)
		if witness := witnessInteger.FindStringSubmatch(cell); len(witness) == 2 {
			extra, _ := strconv.ParseInt(witness[1], 10, 64)
			lines += extra
		}
		result = append(result, UnitSize{Name: matched, Lines: lines})
		seen[matched] = true
	}
	for _, name := range wanted {
		if !seen[name] {
			return nil, 0, fmt.Errorf("LAUNCH_BUILD_UNSIZED unit=%s missing=row", name)
		}
	}
	var total int64
	for _, unit := range result {
		total += unit.Lines
	}
	return result, total, nil
}

func tableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "|") {
		return nil
	}
	trimmed = strings.Trim(trimmed, "|")
	return strings.Split(trimmed, "|")
}
func separatorRow(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(strings.TrimSpace(cell), ":-") != "" {
			return false
		}
	}
	return true
}
func serialSplit(units []UnitSize, cap int64) []string {
	var result []string
	var names []string
	var size int64
	flush := func(over bool) {
		if len(names) == 0 {
			return
		}
		suffix := ""
		if over {
			suffix = " over-cap"
		}
		result = append(result, fmt.Sprintf("job=%d units=%s size=%d%s", len(result)+1, strings.Join(names, ","), size, suffix))
		names, size = nil, 0
	}
	for _, unit := range units {
		if unit.Lines > cap {
			flush(false)
			names, size = []string{unit.Name}, unit.Lines
			flush(true)
			continue
		}
		if size > 0 && size+unit.Lines > cap {
			flush(false)
		}
		names, size = append(names, unit.Name), size+unit.Lines
	}
	flush(false)
	return result
}

func ChooseReadMode(path string, splitLines int64) (ReadChoice, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReadChoice{}, err
	}
	blocks := parseDiff(data)
	choice := ReadChoice{Mode: "whole", Directories: map[string]int64{}}
	for _, block := range blocks {
		if block.lines == 0 || block.directory == "" {
			continue
		}
		choice.Lines += block.lines
		choice.Directories[block.directory] += block.lines
	}
	if choice.Lines <= splitLines {
		return choice, nil
	}
	choice.Mode = "package"
	for _, count := range choice.Directories {
		if count > splitLines {
			choice.Mode = "wide"
			break
		}
	}
	return choice, nil
}

type diffBlock struct {
	text      []string
	directory string
	lines     int64
}

func parseDiff(data []byte) []diffBlock {
	var blocks []diffBlock
	var current *diffBlock
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			blocks = append(blocks, diffBlock{})
			current = &blocks[len(blocks)-1]
		}
		if current == nil {
			blocks = append(blocks, diffBlock{})
			current = &blocks[len(blocks)-1]
		}
		current.text = append(current.text, line)
		if strings.HasPrefix(line, "+++ ") || current.directory == "" && strings.HasPrefix(line, "--- ") {
			path := strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "--- ")
			if path != "/dev/null" {
				path = strings.TrimPrefix(strings.TrimPrefix(path, "a/"), "b/")
				current.directory = filepath.Dir(path)
			}
		}
		if (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")) && !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
			current.lines++
		}
	}
	return blocks
}

func readDiff(path, mode, directory string) ([]byte, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if mode != "package" {
		choice, err := ChooseReadMode(path, 1<<62)
		return data, choice.Lines, err
	}
	var lines []string
	var count int64
	for _, block := range parseDiff(data) {
		if block.directory == directory {
			lines = append(lines, block.text...)
			count += block.lines
		}
	}
	if len(lines) == 0 {
		return nil, 0, fmt.Errorf("LAUNCH_READ_UNSPLIT choice=package missing=%s", directory)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), count, nil
}

func refusalNumbers(message string) map[string]int64 {
	result := map[string]int64{}
	for _, field := range strings.Fields(strings.Split(message, "\n")[0]) {
		key, raw, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
			result[key] = value
		}
	}
	return result
}
