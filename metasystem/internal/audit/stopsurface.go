package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const (
	stopMoveDirectory = "docs/stop-decision-moves"
)

var (
	stopGoPattern   = regexp.MustCompile(`ShouldBlock|BlockSource|IdleRefusal|CountSpent|\\?"decision\\?"\s*:\s*\\?"(?:block|allow)\\?"|\[\s*"decision"\s*\]\s*\)*\s*(?:==|!=)\s*"(?:block|allow)"`)
	stopWirePattern = regexp.MustCompile(`\\?"decision\\?"\s*:\s*\\?"(?:block|allow)\\?"`)
	stopBedPattern  = regexp.MustCompile(`"decision":"(block|allow)"|stop response outcome=|stop response decision=`)
	stopGoalPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	stopDigestName  = regexp.MustCompile(`^(.+)-([0-9a-f]{12})\.txt$`)
)

// StopSurfaceGoalRecord carries the canonical goal fields the Stop surface
// is permitted to trust.
type StopSurfaceGoalRecord struct {
	State            string
	StopSurfaceMoves bool
}

// StopSurfaceGoalReader validates a goal record and projects only the fields
// that authorize a Stop-surface move.
type StopSurfaceGoalReader func(goalID string, content []byte) (StopSurfaceGoalRecord, []string)

// StopSurfaceOptions selects the historical tree compared with the current
// installation and supplies the canonical goal-record reader. An empty Base
// follows the repository's normal landing base.
type StopSurfaceOptions struct {
	Base       string
	GoalRecord StopSurfaceGoalReader
}

// StopSurfaceLine is one normalized assertion occurrence. Equal entries may
// appear more than once because the surface is a multiset within each file.
type StopSurfaceLine struct {
	File string `json:"file"`
	Line string `json:"line"`
}

// StopSurfaceMove is a removed assertion admitted by a new declaration.
type StopSurfaceMove struct {
	File string `json:"file"`
	Line string `json:"line"`
	Goal string `json:"goal"`
}

// StopSurfaceResult describes the complete comparison and every declaration
// problem. Removed contains only assertion occurrences that were not admitted.
type StopSurfaceResult struct {
	Base     string            `json:"base"`
	Added    []StopSurfaceLine `json:"added"`
	Moved    []StopSurfaceMove `json:"moved"`
	Removed  []StopSurfaceLine `json:"removed"`
	Problems []string          `json:"problems"`
}

// Refused reports whether the candidate changes the protected surface without
// an exact, current declaration.
func (r StopSurfaceResult) Refused() bool {
	return len(r.Removed) != 0 || len(r.Problems) != 0
}

// Summary is the stable one-line result consumed by the fast gate.
func (r StopSurfaceResult) Summary() string {
	return fmt.Sprintf("stop decision surface: base %s; added %d, moved %d, removed %d",
		r.Base, len(r.Added), len(r.Moved), len(r.Removed))
}

type stopSurfaceFile struct {
	Kind string
	Path string
}

type stopSurfaceKey struct {
	File string
	Line string
}

type stopSurfaceInspection struct {
	workspace stopSurfaceWorkspace
	base      string
	baseTree  string
	added     []StopSurfaceLine
	removed   []StopSurfaceLine
}

type stopSurfaceWorkspace interface {
	ResolveCommit(string) (string, error)
	HeadCommit() (string, bool, error)
	RefMap() (map[string]string, error)
	MergeBases(string, string) ([]string, error)
	TreeOf(string) (string, error)
	FileAt(string, string) ([]byte, bool, error)
}

type stopSurfaceDependencies struct {
	workspace          stopSurfaceWorkspace
	candidateInventory func(string) ([]byte, error)
	treeInventory      func(string, string) ([]byte, error)
}

func gitStopSurfaceDependencies(root string) stopSurfaceDependencies {
	return stopSurfaceDependencies{
		workspace:          gittree.Workspace{Dir: root},
		candidateInventory: gitStopSurfaceCandidateInventory,
		treeInventory:      gitStopSurfaceTreeInventory,
	}
}

type stopMoveDeclaration struct {
	path  string
	goal  string
	moves []StopSurfaceLine
}

// AuditStopDecisionSurface compares the current files on disk with the
// selected committed base and validates declarations added by this candidate.
func AuditStopDecisionSurface(root string, options StopSurfaceOptions) (StopSurfaceResult, error) {
	return auditStopDecisionSurface(root, options, gitStopSurfaceDependencies(root))
}

func auditStopDecisionSurface(root string, options StopSurfaceOptions, dependencies stopSurfaceDependencies) (StopSurfaceResult, error) {
	inspection, err := inspectStopDecisionSurfaceWithDependencies(root, options, dependencies)
	if err != nil {
		return StopSurfaceResult{}, err
	}
	result := StopSurfaceResult{
		Base:     inspection.base,
		Added:    inspection.added,
		Moved:    []StopSurfaceMove{},
		Removed:  []StopSurfaceLine{},
		Problems: []string{},
	}
	declarations, problems, err := newStopMoveDeclarations(root, inspection.workspace, inspection.baseTree, options.GoalRecord)
	if err != nil {
		return StopSurfaceResult{}, err
	}
	result.Problems = append(result.Problems, problems...)

	remaining := lineCounts(inspection.removed)
	for _, declaration := range declarations {
		for _, move := range declaration.moves {
			key := stopSurfaceKey(move)
			if remaining[key] == 0 {
				result.Problems = append(result.Problems,
					fmt.Sprintf("%s declares a line that was not removed: %s: %s", declaration.path, move.File, move.Line))
				continue
			}
			remaining[key]--
			result.Moved = append(result.Moved, StopSurfaceMove{File: move.File, Line: move.Line, Goal: declaration.goal})
		}
	}
	for _, removed := range inspection.removed {
		key := stopSurfaceKey(removed)
		if remaining[key] == 0 {
			continue
		}
		result.Removed = append(result.Removed, removed)
		remaining[key]--
	}
	sort.Slice(result.Moved, func(i, j int) bool {
		left, right := result.Moved[i], result.Moved[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Goal < right.Goal
	})
	sort.Strings(result.Problems)
	return result, nil
}

// DeclareStopDecisionSurface writes one declaration for the complete current
// removed set. The audit remains the authority that accepts the new file.
func DeclareStopDecisionSurface(root string, options StopSurfaceOptions, goalID, reason string) (string, error) {
	return declareStopDecisionSurface(root, options, goalID, reason, gitStopSurfaceDependencies(root))
}

func declareStopDecisionSurface(root string, options StopSurfaceOptions, goalID, reason string, dependencies stopSurfaceDependencies) (string, error) {
	inspection, err := inspectStopDecisionSurfaceWithDependencies(root, options, dependencies)
	if err != nil {
		return "", err
	}
	if len(inspection.removed) == 0 {
		return "", fmt.Errorf("stop decision surface declaration refused: no assertions were removed")
	}
	if !stopGoalPattern.MatchString(goalID) {
		return "", fmt.Errorf("stop decision surface declaration refused: goal %q is not a valid ledger id", goalID)
	}
	if reason != strings.TrimSpace(reason) || reason == "" || strings.ContainsAny(reason, "\r\n") {
		return "", fmt.Errorf("stop decision surface declaration refused: reason must be one non-empty line")
	}
	if err := requireStopSurfaceGoalPermission(root, goalID, options.GoalRecord); err != nil {
		return "", fmt.Errorf("stop decision surface declaration refused: %w", err)
	}

	rows := declarationRows(inspection.removed)
	digest := stopMoveDigest(rows)
	relative := filepath.ToSlash(filepath.Join(stopMoveDirectory, goalID+"-"+digest[:12]+".txt"))
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", fmt.Errorf("create stop decision move directory: %w", err)
	}
	content := "goal: " + goalID + "\nreason: " + reason + "\nmoved:\n" + strings.Join(rows, "\n") + "\n"
	file, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("write stop decision move declaration: %w", err)
	}
	if _, err = file.WriteString(content); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write stop decision move declaration: %w", err)
	}
	if err = file.Close(); err != nil {
		return "", fmt.Errorf("write stop decision move declaration: %w", err)
	}
	return relative, nil
}

func inspectStopDecisionSurfaceWithDependencies(root string, options StopSurfaceOptions, dependencies stopSurfaceDependencies) (stopSurfaceInspection, error) {
	workspace := dependencies.workspace
	base, err := resolveStopSurfaceBaseWithWorkspace(workspace, options.Base)
	if err != nil {
		return stopSurfaceInspection{}, err
	}
	baseTree, err := workspace.TreeOf(base)
	if err != nil {
		return stopSurfaceInspection{}, fmt.Errorf("stop decision surface base tree: %w", err)
	}
	baseFiles, err := discoverStopSurfaceFilesAtTreeWithInventory(root, baseTree, dependencies.treeInventory)
	if err != nil {
		return stopSurfaceInspection{}, fmt.Errorf("discover base stop decision surface: %w", err)
	}
	candidateFiles, err := discoverStopSurfaceFilesWithInventory(root, dependencies.candidateInventory)
	if err != nil {
		return stopSurfaceInspection{}, fmt.Errorf("discover candidate stop decision surface: %w", err)
	}
	baseSurface, err := extractStopSurface(baseFiles, func(path string) ([]byte, bool, error) {
		return workspace.FileAt(baseTree, path)
	})
	if err != nil {
		return stopSurfaceInspection{}, fmt.Errorf("base stop decision surface: %w", err)
	}
	candidateSurface, err := extractStopSurface(candidateFiles, func(path string) ([]byte, bool, error) {
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if os.IsNotExist(readErr) {
			return nil, false, nil
		}
		return content, readErr == nil, readErr
	})
	if err != nil {
		return stopSurfaceInspection{}, fmt.Errorf("candidate stop decision surface: %w", err)
	}
	added, removed := stopSurfaceDifference(baseSurface, candidateSurface)
	return stopSurfaceInspection{
		workspace: workspace,
		base:      base,
		baseTree:  baseTree,
		added:     added,
		removed:   removed,
	}, nil
}

func resolveStopSurfaceBaseWithWorkspace(workspace stopSurfaceWorkspace, requested string) (string, error) {
	if requested != "" {
		base, err := workspace.ResolveCommit(requested)
		if err != nil {
			return "", fmt.Errorf("stop decision surface base: %w", err)
		}
		return base, nil
	}
	head, unborn, err := workspace.HeadCommit()
	if err != nil {
		return "", fmt.Errorf("stop decision surface HEAD: %w", err)
	}
	if unborn {
		return "", fmt.Errorf("stop decision surface HEAD has no commit")
	}
	refs, err := workspace.RefMap()
	if err != nil {
		return "", fmt.Errorf("stop decision surface refs: %w", err)
	}
	main, ok := refs["refs/remotes/origin/main"]
	if !ok {
		return head, nil
	}
	bases, err := workspace.MergeBases(head, main)
	if err != nil {
		return "", fmt.Errorf("stop decision surface merge base: %w", err)
	}
	if len(bases) != 1 {
		return "", fmt.Errorf("stop decision surface merge base: got %d commits, want one", len(bases))
	}
	return bases[0], nil
}

func discoverStopSurfaceFiles(root string) ([]stopSurfaceFile, error) {
	return discoverStopSurfaceFilesWithInventory(root, gitStopSurfaceCandidateInventory)
}

func gitStopSurfaceCandidateInventory(root string) ([]byte, error) {
	command := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--")
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return nil, fmt.Errorf("list candidate stop decision surface files: %w", err)
		}
		return nil, fmt.Errorf("list candidate stop decision surface files: %s: %w", detail, err)
	}
	return output, nil
}

func discoverStopSurfaceFilesWithInventory(root string, inventory func(string) ([]byte, error)) ([]stopSurfaceFile, error) {
	output, err := inventory(root)
	if err != nil {
		return nil, err
	}

	files := []stopSurfaceFile{}
	seen := map[stopSurfaceFile]bool{}
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		path := filepath.ToSlash(string(raw))
		// An installed dependency tree is content on disk, never a stop
		// surface, whether or not the checkout ignores it (g1-s8 revision 5).
		if isDependencyTreePath(path) {
			continue
		}
		kind, selected := stopSurfaceFileKind(path)
		if !selected {
			continue
		}
		if !validStopSurfacePath(path) {
			return nil, fmt.Errorf("candidate inventory contains non-canonical path %q", path)
		}
		file := stopSurfaceFile{Kind: kind, Path: path}
		if !seen[file] {
			files = append(files, file)
			seen[file] = true
		}
	}
	sortStopSurfaceFiles(files)
	return files, nil
}

func gitStopSurfaceTreeInventory(root, tree string) ([]byte, error) {
	command := exec.Command("git", "-C", root, "ls-tree", "-r", "-z", "--name-only", "--full-tree", tree)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tree %s: %w", tree, err)
	}
	return output, nil
}

func discoverStopSurfaceFilesAtTreeWithInventory(root, tree string, inventory func(string, string) ([]byte, error)) ([]stopSurfaceFile, error) {
	output, err := inventory(root, tree)
	if err != nil {
		return nil, err
	}
	files := []stopSurfaceFile{}
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		path := string(raw)
		if isDependencyTreePath(path) {
			continue
		}
		kind, selected := stopSurfaceFileKind(path)
		if !selected {
			continue
		}
		if !validStopSurfacePath(path) {
			return nil, fmt.Errorf("tree contains non-canonical path %q", path)
		}
		files = append(files, stopSurfaceFile{Kind: kind, Path: path})
	}
	sortStopSurfaceFiles(files)
	return files, nil
}

// isDependencyTreePath reports a path inside a node_modules tree at any depth.
func isDependencyTreePath(path string) bool {
	return strings.HasPrefix(path, "node_modules/") || strings.Contains(path, "/node_modules/")
}

func stopSurfaceFileKind(path string) (string, bool) {
	if strings.HasSuffix(path, "_test.go") {
		return "go", true
	}
	if strings.HasPrefix(path, "scripts/agents/") && strings.HasSuffix(path, "-fixtures.sh") {
		return "bed", true
	}
	return "", false
}

func sortStopSurfaceFiles(files []stopSurfaceFile) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].Path != files[j].Path {
			return files[i].Path < files[j].Path
		}
		return files[i].Kind < files[j].Kind
	})
}

func validStopSurfacePath(path string) bool {
	return path != "" && path == filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))) && path != "." &&
		!filepath.IsAbs(path) && !strings.HasPrefix(path, "../") && !strings.ContainsAny(path, "\x00\r\n\\")
}

func extractStopSurface(files []stopSurfaceFile, read func(string) ([]byte, bool, error)) (map[stopSurfaceKey]int, error) {
	surface := map[stopSurfaceKey]int{}
	for _, file := range files {
		content, exists, err := read(file.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Path, err)
		}
		if !exists {
			continue
		}
		if file.Kind == "go" {
			if err := extractGoStopSurface(surface, file.Path, content); err != nil {
				return nil, err
			}
			continue
		}
		for _, raw := range bytes.Split(content, []byte{'\n'}) {
			if !stopBedPattern.Match(raw) {
				continue
			}
			line := strings.Join(strings.Fields(string(raw)), " ")
			surface[stopSurfaceKey{File: file.Path, Line: line}]++
		}
	}
	return surface, nil
}

func extractGoStopSurface(surface map[stopSurfaceKey]int, path string, content []byte) error {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, path, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	masked := bytes.Clone(content)
	executableLines := map[int]bool{}
	keptStrings, syntaxAssertions := goStopDecisionSyntax(parsed)
	for _, assertion := range syntaxAssertions {
		var normalized bytes.Buffer
		if err := format.Node(&normalized, files, assertion); err != nil {
			return fmt.Errorf("normalize Stop assertion in %s: %w", path, err)
		}
		line := strings.Join(strings.Fields(normalized.String()), " ")
		surface[stopSurfaceKey{File: path, Line: line}]++
		maskStopSurfaceSpan(masked, files, assertion.Pos(), assertion.End())
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncDecl:
			if value.Body != nil {
				first := files.Position(value.Body.Lbrace).Line
				last := files.Position(value.Body.Rbrace).Line
				for line := first; line <= last; line++ {
					executableLines[line] = true
				}
			}
		case *ast.BasicLit:
			if value.Kind == token.STRING && !keptStrings[value] {
				maskStopSurfaceSpan(masked, files, value.Pos(), value.End())
			}
		}
		return true
	})
	for _, commentGroup := range parsed.Comments {
		maskStopSurfaceSpan(masked, files, commentGroup.Pos(), commentGroup.End())
	}
	originalLines := bytes.Split(content, []byte{'\n'})
	maskedLines := bytes.Split(masked, []byte{'\n'})
	for index, raw := range maskedLines {
		if !executableLines[index+1] || !stopGoPattern.Match(raw) {
			continue
		}
		line := strings.Join(strings.Fields(string(originalLines[index])), " ")
		surface[stopSurfaceKey{File: path, Line: line}]++
	}
	return nil
}

func goStopDecisionSyntax(parsed *ast.File) (map[*ast.BasicLit]bool, []ast.Node) {
	kept := map[*ast.BasicLit]bool{}
	var assertions []ast.Node
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		var ancestors []ast.Node
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if node == nil {
				ancestors = ancestors[:len(ancestors)-1]
				return true
			}
			switch value := node.(type) {
			case *ast.BasicLit:
				if value.Kind == token.STRING && !insideStopSurfaceMetadata(ancestors) &&
					!embeddedSourceLiteral(value.Value) && stopWirePattern.MatchString(unquotedString(value.Value)) {
					kept[value] = true
				}
			case *ast.KeyValueExpr:
				if !insideStopSurfaceMetadata(ancestors) {
					if key, keyOK := stringLiteral(value.Key); keyOK && key == "decision" {
						if decision, decisionOK := stringLiteral(value.Value); decisionOK && (decision == "block" || decision == "allow") {
							assertions = append(assertions, value)
						}
					}
				}
			case *ast.BinaryExpr:
				if !insideStopSurfaceMetadata(ancestors) && (value.Op == token.EQL || value.Op == token.NEQ) {
					if decisionIndexComparedWithControl(value.X, value.Y) || decisionIndexComparedWithControl(value.Y, value.X) {
						assertions = append(assertions, value)
					}
				}
			}
			ancestors = append(ancestors, node)
			return true
		})
	}
	return kept, assertions
}

func insideStopSurfaceMetadata(ancestors []ast.Node) bool {
	for _, ancestor := range ancestors {
		literal, ok := ancestor.(*ast.CompositeLit)
		if ok && stopSurfaceMetadataType(literal.Type) {
			return true
		}
	}
	return false
}

func stopSurfaceMetadataType(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "StopSurfaceLine"
	case *ast.SelectorExpr:
		return value.Sel.Name == "StopSurfaceLine"
	case *ast.ArrayType:
		return stopSurfaceMetadataType(value.Elt)
	default:
		return false
	}
}

func embeddedSourceLiteral(raw string) bool {
	return strings.Contains(raw, `\n`) || strings.Contains(unquotedString(raw), "package ")
}

func unquotedString(raw string) string {
	value, err := strconv.Unquote(raw)
	if err != nil {
		return raw
	}
	return value
}

func stringLiteral(expression ast.Expr) (string, bool) {
	expression = unwrapParentheses(expression)
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	return unquotedString(literal.Value), true
}

func unwrapParentheses(expression ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			return expression
		}
		expression = parenthesized.X
	}
}

func decisionIndexComparedWithControl(indexed, control ast.Expr) bool {
	indexed = unwrapParentheses(indexed)
	control = unwrapParentheses(control)
	index, ok := indexed.(*ast.IndexExpr)
	if !ok {
		return false
	}
	key, keyOK := stringLiteral(index.Index)
	decision, decisionOK := stringLiteral(control)
	return keyOK && key == "decision" && decisionOK && (decision == "block" || decision == "allow")
}

func maskStopSurfaceSpan(content []byte, files *token.FileSet, start, end token.Pos) {
	first := files.Position(start).Offset
	last := files.Position(end).Offset
	for index := first; index < last && index < len(content); index++ {
		if content[index] != '\n' {
			content[index] = ' '
		}
	}
}

func stopSurfaceDifference(base, candidate map[stopSurfaceKey]int) ([]StopSurfaceLine, []StopSurfaceLine) {
	added := expandStopSurfaceDifference(candidate, base)
	removed := expandStopSurfaceDifference(base, candidate)
	return added, removed
}

func expandStopSurfaceDifference(left, right map[stopSurfaceKey]int) []StopSurfaceLine {
	keys := make([]stopSurfaceKey, 0, len(left))
	for key := range left {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].File != keys[j].File {
			return keys[i].File < keys[j].File
		}
		return keys[i].Line < keys[j].Line
	})
	var lines []StopSurfaceLine
	for _, key := range keys {
		for count := left[key] - right[key]; count > 0; count-- {
			lines = append(lines, StopSurfaceLine(key))
		}
	}
	return lines
}

func newStopMoveDeclarations(root string, workspace stopSurfaceWorkspace, baseTree string, reader StopSurfaceGoalReader) ([]stopMoveDeclaration, []string, error) {
	directory := filepath.Join(root, filepath.FromSlash(stopMoveDirectory))
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read stop decision move declarations: %w", err)
	}
	var declarations []stopMoveDeclaration
	var problems []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		relative := filepath.ToSlash(filepath.Join(stopMoveDirectory, entry.Name()))
		_, existed, err := workspace.FileAt(baseTree, relative)
		if err != nil {
			return nil, nil, fmt.Errorf("read base declaration %s: %w", relative, err)
		}
		if existed {
			continue
		}
		content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, nil, fmt.Errorf("read declaration %s: %w", relative, err)
		}
		declaration, err := parseStopMoveDeclaration(relative, content)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if err := requireStopSurfaceGoalPermission(root, declaration.goal, reader); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", relative, err))
			continue
		}
		declarations = append(declarations, declaration)
	}
	sort.Slice(declarations, func(i, j int) bool { return declarations[i].path < declarations[j].path })
	return declarations, problems, nil
}

func requireStopSurfaceGoalPermission(root, goalID string, reader StopSurfaceGoalReader) error {
	if reader == nil {
		return stopSurfaceGoalRefusal("no goal record reader")
	}
	goalPath := filepath.Join(root, "plans", "goals", goalID+".md")
	info, err := os.Stat(goalPath)
	if err != nil || !info.Mode().IsRegular() {
		return stopSurfaceGoalRefusal("goal %s has no live ledger record", goalID)
	}
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return stopSurfaceGoalRefusal("goal %s record is unreadable: %v", goalID, err)
	}
	record, problems := reader(goalID, content)
	if len(problems) != 0 {
		return stopSurfaceGoalRefusal("goal %s record is invalid: %s", goalID, problems[0])
	}
	if record.State == "done" || record.State == "abandoned" {
		return stopSurfaceGoalRefusal("goal %s is %s", goalID, record.State)
	}
	if !record.StopSurfaceMoves {
		return stopSurfaceGoalRefusal("goal %s does not permit Stop-surface moves", goalID)
	}
	return nil
}

func stopSurfaceGoalRefusal(format string, args ...any) error {
	return fmt.Errorf("STOP_SURFACE_GOAL_REFUSED "+format, args...)
}

func parseStopMoveDeclaration(path string, content []byte) (stopMoveDeclaration, error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) < 4 || !strings.HasPrefix(lines[0], "goal: ") || !strings.HasPrefix(lines[1], "reason: ") || lines[2] != "moved:" {
		return stopMoveDeclaration{}, fmt.Errorf("%s has an invalid header or missing moved: section", path)
	}
	goal := strings.TrimPrefix(lines[0], "goal: ")
	reason := strings.TrimPrefix(lines[1], "reason: ")
	if !stopGoalPattern.MatchString(goal) {
		return stopMoveDeclaration{}, fmt.Errorf("%s has an invalid goal", path)
	}
	if reason == "" || reason != strings.TrimSpace(reason) {
		return stopMoveDeclaration{}, fmt.Errorf("%s has an empty or non-canonical reason", path)
	}
	match := stopDigestName.FindStringSubmatch(filepath.Base(path))
	if len(match) != 3 || match[1] != goal {
		return stopMoveDeclaration{}, fmt.Errorf("%s does not name its goal and digest", path)
	}
	moves := make([]StopSurfaceLine, 0, len(lines)-3)
	for index, line := range lines[3:] {
		file, normalized, ok := strings.Cut(line, "\t")
		if !ok || strings.Contains(normalized, "\t") || !validStopSurfacePath(file) || normalized == "" || normalized != strings.Join(strings.Fields(normalized), " ") {
			return stopMoveDeclaration{}, fmt.Errorf("%s has an invalid moved line at %d", path, index+4)
		}
		moves = append(moves, StopSurfaceLine{File: file, Line: normalized})
	}
	rows := declarationRows(moves)
	digest := stopMoveDigest(rows)
	if match[2] != digest[:12] {
		return stopMoveDeclaration{}, fmt.Errorf("%s digest does not match its moved lines", path)
	}
	return stopMoveDeclaration{path: path, goal: goal, moves: moves}, nil
}

func declarationRows(lines []StopSurfaceLine) []string {
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, line.File+"\t"+line.Line)
	}
	sort.Strings(rows)
	return rows
}

func stopMoveDigest(rows []string) string {
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:])
}

func lineCounts(lines []StopSurfaceLine) map[stopSurfaceKey]int {
	counts := make(map[stopSurfaceKey]int, len(lines))
	for _, line := range lines {
		counts[stopSurfaceKey(line)]++
	}
	return counts
}
