package launch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

type UnitSize struct {
	Name  string
	Lines int64

	Production *int64
	Row        string
}
type ReadChoice struct {
	Mode        string
	Lines       int64
	Directories map[string]int64
	Files       map[string]int64
}

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
	if code := ErrorCode(err); strings.HasPrefix(code, "LAUNCH_") {
		now := time.Now
		if m.Now != nil {
			now = m.Now
		}
		_ = m.Store.AppendRefusal(Refusal{Time: now().UTC().Format(time.RFC3339Nano), Code: code, Kind: spec.Kind, Goal: spec.Goal, Tag: spec.Tag, Numbers: refusalNumbers(ErrorDetail(err))})
	}
	return err
}

func (m *Manager) admit(spec StartSpec, settings Settings) error {
	if spec.Kind == "critique" && (spec.Tag == "" || len(spec.Inputs) < 2) {
		return fmt.Errorf("critique requires --tag and two --input files")
	}
	if spec.Kind == "read" && spec.DiffFile == "" {
		return coded("LAUNCH_READ_UNSIZED", "missing=diff-file", errors.New("a read needs the diff it reads, and none was given"))
	}
	if _, err := m.CheckPack(spec); err != nil {
		return err
	}
	paths := append([]string{spec.Brief}, spec.Inputs...)
	if spec.Page != "" {
		paths = append(paths, spec.Page)
	}
	if spec.UnitsPage != "" && spec.UnitsPage != spec.Page {
		paths = append(paths, spec.UnitsPage)
	}
	type measured struct {
		path   string
		tokens int64
	}
	var total int64
	var values []measured
	if spec.Kind == "critique" {
		switch m.Adapters["codex-exec"].(type) {
		case CodexExec, *CodexExec:
			common, err := protocol.Template(designCommonTemplate)
			if err != nil {
				return err
			}
			tokens := EstimateTokens(int64(len(common)))
			total += tokens
			values = append(values, measured{protocol.ReferencePrefix + "templates/" + designCommonTemplate, tokens})
		}
	}
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
		var lines []string
		for _, value := range values {
			lines = append(lines, fmt.Sprintf("input=%s tokens=%d", value.path, value.tokens))
		}
		return &CodedError{Code: "LAUNCH_BRIEF_OVERSIZE", Facts: fmt.Sprintf("total=%d cap=%d", total, settings.BriefCap),
			Reason: fmt.Errorf("the brief and its inputs come to about %d tokens, over the cap of %d; the largest is %s",
				total, settings.BriefCap, values[0].path),
			Background: strings.Join(lines, "\n")}
	}
	if spec.Kind == "build" {
		units, size, sizeErr := buildSize(spec)
		if sizeErr != nil {
			return sizeErr
		}
		if size > settings.BuildLinesCap {
			return &CodedError{Code: "LAUNCH_BUILD_OVERSIZE", Facts: fmt.Sprintf("size=%d cap=%d", size, settings.BuildLinesCap),
				Reason:     fmt.Errorf("the build is about %d changed lines, over the cap of %d; split it into smaller builds", size, settings.BuildLinesCap),
				Background: strings.Join(serialSplit(units, settings.BuildLinesCap), "\n")}
		}
	}
	if spec.Kind == "read" {
		choice, choiceErr := ChooseReadMode(spec.DiffFile, settings.ReadSplitLines)
		if choiceErr != nil {
			return choiceErr
		}
		_, packageExists := choice.Directories[spec.Package]
		_, fileExists := choice.Files[spec.File]
		follows := choice.Mode == "whole" && spec.Package == "" && !spec.Wide || choice.Mode == "package" && spec.Package != "" && packageExists && !spec.Wide || choice.Mode == "wide" && spec.Wide && spec.Package == ""
		if spec.readMode == "package" {
			follows = spec.Package != "" && packageExists && !spec.Wide
		}
		if spec.readMode == "wide" {
			follows = spec.Package != "" && packageExists && spec.Wide
		}
		if spec.readMode == "file" {
			follows = spec.Package != "" && spec.File != "" && filepath.Dir(spec.File) == spec.Package && fileExists && !spec.Wide
		}
		if !follows {
			var lines []string
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
			return &CodedError{Code: "LAUNCH_READ_UNSPLIT", Facts: "choice=" + choice.Mode,
				Reason:     fmt.Errorf("this diff's size calls for a %s read, which the launch does not ask for", choice.Mode),
				Background: strings.Join(lines, "\n")}
		}
	}
	return nil
}

func buildSize(spec StartSpec) ([]UnitSize, int64, error) {
	path := spec.Brief
	if spec.UnitsPage != "" {
		path = spec.UnitsPage
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if spec.UnitsPage != "" {
			return nil, 0, coded("LAUNCH_BUILD_UNSIZED", "missing=units-page", fmt.Errorf("the units page %s cannot be read, so the build has no size: %v", spec.UnitsPage, err))
		}
		return nil, 0, err
	}
	units, size, err := sizesFromTable(string(data), spec.Units)
	if spec.UnitsPage == "" && UnsizedMissing(err) == "units-table" {
		return nil, 0, coded("LAUNCH_BUILD_UNSIZED", "missing=declared-size", errors.New("the brief declares no size for this build: give it a units table with a size column"))
	}
	return units, size, err
}

// UnsizedError is a LAUNCH_BUILD_UNSIZED refusal's reason: Missing names
// what the page lacks (units-table, size-column, row, integer).
type UnsizedError struct {
	Missing string
	Err     error
}

func (e *UnsizedError) Error() string { return e.Err.Error() }
func (e *UnsizedError) Unwrap() error { return e.Err }

// UnsizedMissing is what an unsized refusal says the page lacks, or "".
func UnsizedMissing(err error) string {
	var unsized *UnsizedError
	if errors.As(err, &unsized) {
		return unsized.Missing
	}
	return ""
}

func sizesFromTable(page string, wanted []string) ([]UnitSize, int64, error) {
	rows := parseUnitSizes(page, true)
	if len(rows) == 0 {
		return nil, 0, coded("LAUNCH_BUILD_UNSIZED", "missing=units-table", &UnsizedError{Missing: "units-table", Err: errors.New("the page has no sized units table")})
	}
	wants := map[string]bool{}
	for _, name := range wanted {
		wants[name] = true
	}
	seen := map[string]bool{}
	var result []UnitSize
	for _, row := range rows {
		name := row.Name
		matched := name
		if len(wants) > 0 {
			matched = ""
			for wanted := range wants {
				if name == wanted || strings.HasPrefix(name, wanted+".") || strings.HasPrefix(name, wanted+" ") {
					matched = wanted
					break
				}
			}
		}
		if matched == "" || seen[matched] {
			continue
		}
		if row.Lines < 0 {
			return nil, 0, coded("LAUNCH_BUILD_UNSIZED", "unit="+matched+" missing=integer", &UnsizedError{Missing: "integer", Err: fmt.Errorf("unit %s has no number in its total size column", matched)})
		}
		row.Name = matched
		row.Row = ""
		result = append(result, row)
		seen[matched] = true
	}
	for _, name := range wanted {
		if !seen[name] {
			return nil, 0, coded("LAUNCH_BUILD_UNSIZED", "unit="+name+" missing=row", &UnsizedError{Missing: "row", Err: fmt.Errorf("the units table has no row for unit %s, so the build has no size", name)})
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
	groups := serialGroups(units, cap)
	result := make([]string, 0, len(groups))
	for index, group := range groups {
		suffix := ""
		if group.OverCap {
			suffix = " over-cap"
		}
		result = append(result, fmt.Sprintf("job=%d units=%s size=%d%s", index+1, strings.Join(group.Units, ","), group.Size, suffix))
	}
	return result
}

type buildGroup struct {
	Units   []string
	Size    int64
	OverCap bool
}

func serialGroups(units []UnitSize, cap int64) []buildGroup {
	var result []buildGroup
	var group buildGroup
	flush := func() {
		if len(group.Units) == 0 {
			return
		}
		result = append(result, group)
		group = buildGroup{}
	}
	for _, unit := range units {
		if unit.Lines > cap {
			flush()
			result = append(result, buildGroup{Units: []string{unit.Name}, Size: unit.Lines, OverCap: true})
			continue
		}
		if group.Size > 0 && group.Size+unit.Lines > cap {
			flush()
		}
		group.Units = append(group.Units, unit.Name)
		group.Size += unit.Lines
	}
	flush()
	return result
}

func ChooseReadMode(path string, splitLines int64) (ReadChoice, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReadChoice{}, err
	}
	blocks := parseDiff(data)
	choice := ReadChoice{Mode: "whole", Directories: map[string]int64{}, Files: map[string]int64{}}
	for _, block := range blocks {
		if block.lines == 0 || block.directory == "" {
			continue
		}
		choice.Lines += block.lines
		choice.Directories[block.directory] += block.lines
		choice.Files[block.file] += block.lines
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
	file      string
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
				current.file = path
				current.directory = filepath.Dir(path)
			}
		}
		if (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")) && !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
			current.lines++
		}
	}
	return blocks
}

func readDiff(path, mode, share string) ([]byte, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if mode != "package" && mode != "file" {
		choice, err := ChooseReadMode(path, 1<<62)
		return data, choice.Lines, err
	}
	var lines []string
	var count int64
	for _, block := range parseDiff(data) {
		if mode == "package" && block.directory == share || mode == "file" && block.file == share {
			lines = append(lines, block.text...)
			count += block.lines
		}
	}
	if len(lines) == 0 {
		return nil, 0, coded("LAUNCH_READ_UNSPLIT", "choice="+mode+" missing="+share, fmt.Errorf("the diff has no changes in %s, so there is nothing to read by %s", share, mode))
	}
	return []byte(strings.Join(lines, "\n") + "\n"), count, nil
}

func refusalNumbers(message string) map[string]int64 {
	result := map[string]int64{}
	for _, field := range strings.Fields(strings.Split(message, "\n")[0]) {
		key, raw, ok := strings.Cut(strings.TrimSuffix(field, ":"), "=")
		if !ok {
			continue
		}
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
			result[key] = value
		}
	}
	return result
}

// DeclaredUnitLines reads one unit's changed-line estimate from the units
// table of a page, the same row build admission reads for that unit.
func DeclaredUnitLines(page, unit string) (int64, error) {
	data, err := os.ReadFile(page)
	if err != nil {
		return 0, err
	}
	_, lines, err := sizesFromTable(string(data), []string{unit})
	return lines, err
}

// DeclaredUnits reads every row of a page's units table with its estimate.
func DeclaredUnits(page string) ([]UnitSize, error) {
	data, err := os.ReadFile(page)
	if err != nil {
		return nil, err
	}
	units, _, err := sizesFromTable(string(data), nil)
	return units, err
}
