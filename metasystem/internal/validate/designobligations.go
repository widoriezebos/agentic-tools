package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The design-obligation gate: a declared obligation status is only as
// trustworthy as the proof behind it, so CRITICAL/HIGH rows demand code-shaped owners and concrete
// proof cells before DONE or READY_FOR_RUNTIME stands. Matrices inside
// fenced code blocks are documentation, not declarations.

var (
	fenceLine      = regexp.MustCompile("^[[:space:]]*```")
	headerStart    = regexp.MustCompile(`^\|[[:space:]]*Obligation id[[:space:]]*\|[[:space:]]*Severity[[:space:]]*\|`)
	headerEnd      = regexp.MustCompile(`\|[[:space:]]*Status[[:space:]]*\|[[:space:]]*Next action[[:space:]]*\|[[:space:]]*$`)
	separatorRow   = regexp.MustCompile(`^\|[[:space:]-]+\|`)
	backtickToken  = regexp.MustCompile("`[^`]+`")
	filenameToken  = regexp.MustCompile(`[[:alpha:]][[:alnum:]_.-]*\.[[:alnum:]][[:alnum:]]+`)
	shortExtToken  = regexp.MustCompile(`[[:alnum:]_/.-]+\.(c|h|m|r|R)([^[:alnum:]]|$)`)
	slashToken     = regexp.MustCompile(`[[:alnum:]_.-]+/[[:alnum:]_/.-]+`)
	naPhrase       = regexp.MustCompile(`(not applicable|no runtime proof required|runtime proof not required)`)
	naWithReason   = regexp.MustCompile(`(not applicable|no runtime proof required|runtime proof not required)[^a-z0-9]+[a-z0-9]`)
	dottedToken    = regexp.MustCompile(`[[:alnum:]_]+\.[[:alnum:]_]+`)
	camelCaseToken = regexp.MustCompile(`[A-Z][a-z0-9]+[A-Z]`)
)

func obligationWeak(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "NONE", "MISSING", "TBD", "TODO", "UNKNOWN":
		return true
	}
	return false
}

// obligationConcrete accepts a backticked token, a path-shaped token, or a
// "Not applicable" carrying a reason; keyword-only prose fails.
func obligationConcrete(value string) bool {
	original := strings.TrimSpace(value)
	normalized := strings.ToLower(original)
	if obligationWeak(original) {
		return false
	}
	if naPhrase.MatchString(normalized) {
		return naWithReason.MatchString(normalized)
	}
	return backtickToken.MatchString(original) ||
		filenameToken.MatchString(original) ||
		shortExtToken.MatchString(original) ||
		slashToken.MatchString(original)
}

// obligationOwner accepts a backticked, dotted, slashed, double-colon, or
// CamelCase code token; plain prose fails.
func obligationOwner(value string) bool {
	original := strings.TrimSpace(value)
	if obligationWeak(original) {
		return false
	}
	return backtickToken.MatchString(original) ||
		dottedToken.MatchString(original) ||
		slashToken.MatchString(original) ||
		strings.Contains(original, "::") ||
		camelCaseToken.MatchString(original)
}

// checkObligationFile validates one matrix file and returns the failure
// lines, each prefixed file:line: with the caller's original file argument.
func checkObligationFile(name, content string, runtimeRequired bool) []string {
	var failures []string
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	add := func(lineNumber int, message string) {
		failures = append(failures, fmt.Sprintf("%s:%d: %s", name, lineNumber, message))
	}
	cell := func(columns []string, index int) string {
		// awk's split indexes from 1; Go's slice from 0.
		if index-1 < len(columns) {
			return strings.TrimSpace(columns[index-1])
		}
		return ""
	}
	inTable, inCode := false, false
	rows := 0
	for i, line := range lines {
		lineNumber := i + 1
		if fenceLine.MatchString(line) {
			inCode = !inCode
			inTable = false
			continue
		}
		if inCode {
			continue
		}
		if headerStart.MatchString(line) {
			if headerEnd.MatchString(line) {
				inTable = true
			}
			continue
		}
		if inTable && separatorRow.MatchString(line) {
			continue
		}
		if inTable && !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}
		if !inTable {
			continue
		}
		columns := strings.Split(line, "|")
		obligationID := cell(columns, 2)
		severity := strings.ToUpper(cell(columns, 3))
		owner := cell(columns, 6)
		codeProof := cell(columns, 7)
		testProof := cell(columns, 8)
		runtimeProof := cell(columns, 9)
		status := strings.ToUpper(cell(columns, 10))
		if obligationID == "" || obligationID == "Obligation id" {
			continue
		}
		rows++
		switch severity {
		case "CRITICAL", "HIGH", "MEDIUM", "LOW":
		default:
			add(lineNumber, obligationID+" has invalid severity: "+severity)
		}
		switch status {
		case "DONE", "READY_FOR_RUNTIME", "PARTIAL", "MISSING", "CONTRADICTED", "BLOCKED":
		default:
			add(lineNumber, obligationID+" has invalid status: "+status)
		}
		if severity != "CRITICAL" && severity != "HIGH" {
			continue
		}
		if status == "MISSING" || status == "PARTIAL" || status == "CONTRADICTED" {
			add(lineNumber, obligationID+" blocks completion: "+severity+" "+status)
		}
		if status == "READY_FOR_RUNTIME" {
			if runtimeRequired {
				add(lineNumber, obligationID+" still needs runtime proof before this gate: "+status)
			}
			if !obligationOwner(owner) || !obligationConcrete(codeProof) || !obligationConcrete(testProof) {
				add(lineNumber, obligationID+" cannot be READY_FOR_RUNTIME without an owner, concrete code proof, and concrete test proof")
			}
		}
		if status == "DONE" && (!obligationOwner(owner) || !obligationConcrete(codeProof) ||
			!obligationConcrete(testProof) || !obligationConcrete(runtimeProof)) {
			add(lineNumber, obligationID+" cannot be DONE without an owner and concrete code, test, and runtime proof")
		}
		if status == "BLOCKED" {
			add(lineNumber, obligationID+" is BLOCKED and needs the named external decision before this gate")
		}
	}
	if rows == 0 {
		add(len(lines), "no design-obligation rows found")
	}
	return failures
}

// DesignObligations validates every matrix file and returns the lines for
// stdout, the lines for stderr, and the process exit code, matching the
// shell gate's contract: 0 passed, 1 failed, with a relative path that is
// unreadable from the working directory retried against root.
func DesignObligations(root string, files []string, runtimeRequired bool) (out, errs []string, code int) {
	failedFiles := 0
	for _, file := range files {
		path := file
		if !filepath.IsAbs(file) && !readable(path) {
			path = filepath.Join(root, file)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, "missing or unreadable obligation file: "+file)
			failedFiles++
			continue
		}
		if failures := checkObligationFile(file, string(content), runtimeRequired); len(failures) > 0 {
			errs = append(errs, failures...)
			failedFiles++
		}
	}
	if failedFiles > 0 {
		errs = append(errs, "design obligation gate failed")
		return out, errs, 1
	}
	runtimeFlag := 0
	if runtimeRequired {
		runtimeFlag = 1
	}
	out = append(out, fmt.Sprintf("design obligation gate passed (%d file(s), runtime_required=%d)", len(files), runtimeFlag))
	return out, errs, 0
}

func readable(path string) bool {
	handle, err := os.Open(path)
	if err != nil {
		return false
	}
	handle.Close()
	return true
}
