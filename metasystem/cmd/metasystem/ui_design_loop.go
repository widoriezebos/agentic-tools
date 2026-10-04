package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The loop from the room's engine half (g1-s66 D1, D2, D4): the design page
// sends a design to critique and answers its rounds through `design review`
// itself, run in this process exactly as the public verb runs; it reads a
// round's findings from that round's retained return and never from the
// findings register, which keeps only material findings and merges a
// recurring id across rounds; and it writes each press as one row of the
// engine's own decisions file for that round, under the binding the engine
// wrote, in the engine's four columns and four values. Nothing here writes
// into the design: its digest stays the reviewed one until a fold changes it.

// designReviewRun is Send to critique, and Answer the round when After names
// the examination the round's own decisions file answers.
func designReviewRun(roots lifecycle.Roots, id string, asked httpd.DesignAsked) (httpd.DesignAnswer, error) {
	return designReviewRunWith(roots, id, asked, defaultIntentOwners())
}

func designReviewRunWith(roots lifecycle.Roots, id string, asked httpd.DesignAsked, owners intentOwners) (httpd.DesignAnswer, error) {
	design, record, err := designOf(roots, id)
	if err != nil {
		return httpd.DesignAnswer{}, err
	}
	args := []string{design, "--tool-calls", strconv.Itoa(asked.ToolCalls), "--json", "--repo", roots.Checkout}
	if asked.ToolCalls < 1 {
		// The engine refuses a review without a reader budget and never
		// invents one; so does this act, by passing none on.
		args = []string{design, "--json", "--repo", roots.Checkout}
	}
	if asked.Goal != "" {
		args = append(args, "--goal", asked.Goal)
	}
	if asked.After > 0 {
		chain, found := designChainOf(roots, design)
		if !found {
			return httpd.DesignAnswer{}, &httpd.DesignRefusal{Code: "no-chain", Message: "this design has no critique to answer"}
		}
		returnPath := roots.Installation.Path("artifacts", "agents", chain.Root, "rounds", strconv.FormatInt(asked.After, 10), "return.json")
		// A round with no findings has nothing to press, so nothing wrote its
		// file; the engine's template is its whole answer.
		if _, statErr := os.Stat(roundDecisionsPath(returnPath)); statErr != nil {
			findings, _, readErr := readIntentFindings(returnPath)
			if readErr == nil && len(findings) == 0 {
				if problem := writeRoundTemplate(roots, chain, record.ID, asked.After, returnPath, findings); problem != nil {
					return httpd.DesignAnswer{}, problem
				}
			}
		}
		args = append(args, "--dispositions", roundDecisionsPath(returnPath), "--after", strconv.FormatInt(asked.After, 10))
	}
	command, _ := findIntentCommand("design review")
	var stdout, stderr bytes.Buffer
	runIntentIn(command, args, &stdout, &stderr, roots.Checkout, owners)
	var result struct {
		Outcome  string         `json:"outcome"`
		Summary  string         `json:"summary"`
		Decision string         `json:"decision"`
		Next     *intentNext    `json:"next"`
		Data     map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return httpd.DesignAnswer{}, &httpd.DesignRefusal{Code: "unreadable",
			Message: "design review answered nothing the page can read: " + strings.TrimSpace(stderr.String())}
	}
	answer := httpd.DesignAnswer{Outcome: result.Outcome, Summary: result.Summary, Decision: result.Decision}
	if answer.Decision == "" && result.Next != nil && result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		// A refusal's resolving command is the page's decision line.
		answer.Decision = "run: " + shellCommand(result.Next.Argv)
		if result.Next.Reason != "" {
			answer.Decision += "  (" + result.Next.Reason + ")"
		}
	}
	for _, key := range []string{"ownerMessage", "obligations"} {
		lines, _ := result.Data[key].([]any)
		for _, line := range lines {
			answer.Lines = append(answer.Lines, fmt.Sprint(line))
		}
	}
	if chain, found := designChainOf(roots, design); found {
		answer.Chain = chain.Root
	}
	return answer, nil
}

// designOf resolves a checkout-relative design id to its absolute path and
// record, refusing anything that is not a design record inside the checkout.
func designOf(roots lifecycle.Roots, id string) (string, intentDesignRecord, error) {
	clean := filepath.Clean(filepath.FromSlash(id))
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", intentDesignRecord{}, &httpd.DesignRefusal{Code: "not-found", Message: id + " is not a path inside this checkout"}
	}
	path := filepath.Join(roots.Checkout, clean)
	record, _, err := readIntentDesignRecord(path)
	if err != nil {
		if _, statErr := os.Stat(path); statErr != nil {
			return "", record, &httpd.DesignRefusal{Code: "not-found", Message: id + " is not in this checkout"}
		}
		return "", record, &httpd.DesignRefusal{Code: "not-a-design", Message: err.Error()}
	}
	return path, record, nil
}

// designChainOf is the design's one critique chain, of whichever goal.
func designChainOf(roots lifecycle.Roots, design string) (dispatchcore.DesignCritiqueChain, bool) {
	chains := dispatchcore.DesignCritiqueChains(roots.Installation.Path(), "", design)
	if len(chains) != 1 {
		return dispatchcore.DesignCritiqueChain{}, false
	}
	return chains[0], true
}

// designLoopRead is the design's critique as the page reads it.
func designLoopRead(roots lifecycle.Roots, id string) (httpd.DesignLoop, error) {
	design, _, err := designOf(roots, id)
	if err != nil {
		return httpd.DesignLoop{}, err
	}
	loop := httpd.DesignLoop{Design: id, State: "none", Rounds: []httpd.DesignRound{},
		ToolCalls: designToolCalls(roots)}
	chains := dispatchcore.DesignCritiqueChains(roots.Installation.Path(), "", design)
	if len(chains) > 1 {
		loop.Chains = len(chains)
		return loop, nil
	}
	if len(chains) == 0 {
		return loop, nil
	}
	chain := chains[0]
	loop.Chain, loop.Goal, loop.Round = chain.Root, chain.Goal, chain.NewestRound
	loop.Status = recordText(chain.Newest, "status")
	loop.Critic = roundModel(roots, chain.Root, chain.NewestRound)
	loop.Limit = reviewRoundCeiling(roots.Installation.Path())
	if root, err := readJobRecord(roots.Installation.Path(), chain.Root); err == nil {
		if limit, ok := root["reviewRoundLimit"].(float64); ok && limit >= 1 {
			loop.Limit = int64(limit)
		}
	}
	for round := int64(1); round <= chain.NewestRound; round++ {
		loop.Rounds = append(loop.Rounds, designRoundRead(roots, chain.Root, round))
	}
	switch {
	case chain.Closed:
		loop.State = "closed"
	case !dispatchcore.TerminalStatus(loop.Status):
		loop.State = "reading"
	case loop.Status != "completed":
		loop.State = "ended"
	default:
		loop.State = "deciding"
		newest := &loop.Rounds[len(loop.Rounds)-1]
		returnPath := roots.Installation.Path("artifacts", "agents", chain.Root, "rounds", strconv.FormatInt(chain.NewestRound, 10), "return.json")
		_, templated := os.Stat(roundDecisionsPath(returnPath))
		switch {
		case newest.Prose != "":
		case len(newest.Findings) == 0:
			newest.Answerable = true
		default:
			newest.Answerable = templated == nil && len(validate.CritiqueClosed(returnPath, roundDecisionsPath(returnPath))) == 0
		}
		if newest.Answerable {
			loop.State = "answered"
		}
	}
	return loop, nil
}

// roundModel is the model an examination runs, as its own composition record
// beside the round's return names it, or "" where it names none.
func roundModel(roots lifecycle.Roots, root string, round int64) string {
	var composition struct {
		Model string `json:"model"`
	}
	data, err := os.ReadFile(roots.Installation.Path("artifacts", "agents", root, "rounds", strconv.FormatInt(round, 10), "composition.json"))
	if err != nil || json.Unmarshal(data, &composition) != nil {
		return ""
	}
	return composition.Model
}

// designToolCalls is the reader budget the sheet prefills.
func designToolCalls(roots lifecycle.Roots) int {
	value := config.ConfValue(roots.Installation.Path("metasystem.conf"), config.ReviewDesignToolCallsKey, "")
	if calls, err := strconv.Atoi(value); err == nil && calls > 0 {
		return calls
	}
	return config.DefaultReviewDesignToolCalls
}

func readJobRecord(installation, job string) (map[string]any, error) {
	var record map[string]any
	data, err := os.ReadFile(filepath.Join(installation, "artifacts", "agents", "jobs", job+".json"))
	if err != nil {
		return nil, err
	}
	return record, json.Unmarshal(data, &record)
}

// designRoundRead is one examination: its findings from its own return, and
// its decisions file as it stands. A return that carries no findings the page
// can read is shown as its words, and offers no presses.
func designRoundRead(roots lifecycle.Roots, root string, round int64) httpd.DesignRound {
	read := httpd.DesignRound{Round: round, Findings: []httpd.DesignFinding{}, Decisions: []httpd.DesignRow{}}
	returnPath := roots.Installation.Path("artifacts", "agents", root, "rounds", strconv.FormatInt(round, 10), "return.json")
	data, err := os.ReadFile(returnPath)
	if err != nil {
		return read
	}
	var result struct {
		Findings []httpd.DesignFinding `json:"findings"`
	}
	if json.Unmarshal(data, &result) != nil || result.Findings == nil {
		read.Prose = strings.TrimSpace(string(data))
		return read
	}
	for _, finding := range result.Findings {
		finding.Change, finding.Tests = proseLine(finding.Claim+"\n"+finding.Evidence, "change"), proseLine(finding.Claim+"\n"+finding.Evidence, "test")
		read.Findings = append(read.Findings, finding)
	}
	if rows, _ := validate.DispositionRows(roundDecisionsPath(returnPath)); rows != nil {
		for _, row := range rows {
			if decidedValue(row.Disposition) {
				read.Decisions = append(read.Decisions, httpd.DesignRow{Finding: row.Finding, Disposition: row.Disposition,
					Reasoning: row.Reasoning, Amendment: row.Amendment})
			}
		}
	}
	return read
}

// proseLine is what a finding's own words say under a label: the change it
// asks for ("Change: …") or its tests ("Test: …", "Tests: …"). A finding that
// says neither shows neither.
func proseLine(text, label string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		key, value, found := strings.Cut(trimmed, ":")
		key = strings.ToLower(strings.TrimSpace(key))
		if found && (key == label || key == label+"s" || key == label+" asked" || key == "the "+label+" asked for") && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func decidedValue(disposition string) bool {
	return disposition == "accepted" || disposition == "refuted" || disposition == "noted" || disposition == "out-of-scope"
}

// designDecide writes one row into the round's decisions file: the engine's
// template beside the return, written first under the binding the engine
// writes when the page is the first to decide. A finding the round does not
// carry, a value outside the four, a material finding noted, a refutation or
// an out-of-scope without its reason, an acceptance without its amendment, and
// a finding already decided are refused in words; the row is written once.
func designDecide(roots lifecycle.Roots, id string, round int64, row httpd.DesignRow) (httpd.DesignLoop, error) {
	design, record, err := designOf(roots, id)
	if err != nil {
		return httpd.DesignLoop{}, err
	}
	refuse := func(code, message string) (httpd.DesignLoop, error) {
		return httpd.DesignLoop{}, &httpd.DesignRefusal{Code: code, Message: message}
	}
	chain, found := designChainOf(roots, design)
	switch {
	case !found:
		return refuse("no-chain", "this design has no critique whose findings could be decided")
	case chain.Closed:
		return refuse("closed", fmt.Sprintf("critique %s is closed; its decisions are part of the design now", chain.Root))
	case round != chain.NewestRound || recordText(chain.Newest, "status") != "completed":
		return refuse("round", fmt.Sprintf("round %d is not the critique's newest finished examination; only that round is decided", round))
	}
	returnPath := roots.Installation.Path("artifacts", "agents", chain.Root, "rounds", strconv.FormatInt(round, 10), "return.json")
	findings, _, err := readIntentFindings(returnPath)
	if err != nil {
		return refuse("return", "round "+strconv.FormatInt(round, 10)+"'s return cannot be read: "+err.Error())
	}
	var material, known bool
	for _, finding := range findings {
		if finding.ID == row.Finding {
			known, material = true, finding.Material
		}
	}
	switch {
	case !known:
		return refuse("finding", fmt.Sprintf("round %d carries no finding %s", round, row.Finding))
	case !decidedValue(row.Disposition):
		return refuse("disposition", fmt.Sprintf("%q is not a decision; a finding is accepted, refuted, noted or out-of-scope", row.Disposition))
	case material && row.Disposition == "noted":
		return refuse("noted", fmt.Sprintf("material finding %s cannot be noted; defer it as out-of-scope with the evidence that it is outside the brief", row.Finding))
	case row.Disposition == "refuted" && row.Reasoning == "":
		return refuse("reason", fmt.Sprintf("refuting %s needs your reason: the check you made and what it showed", row.Finding))
	case row.Disposition == "out-of-scope" && row.Reasoning == "":
		return refuse("reason", fmt.Sprintf("deferring material finding %s needs the evidence that it is outside the brief's scope", row.Finding))
	case row.Disposition == "accepted" && row.Amendment == "":
		return refuse("amendment", fmt.Sprintf("accepting %s needs the fold's one-line amendment", row.Finding))
	}
	own := roundDecisionsPath(returnPath)
	if _, statErr := os.Stat(own); statErr != nil {
		if problem := writeRoundTemplate(roots, chain, record.ID, round, returnPath, findings); problem != nil {
			return httpd.DesignLoop{}, problem
		}
	}
	content, err := os.ReadFile(own)
	if err != nil {
		return httpd.DesignLoop{}, err
	}
	lines := strings.Split(string(content), "\n")
	at := -1
	for index, line := range lines {
		cells := dispositionCells(line)
		if len(cells) == 4 && cells[0] == row.Finding {
			if decidedValue(cells[1]) {
				return refuse("decided", fmt.Sprintf("finding %s of round %d is already decided: %s", row.Finding, round, cells[1]))
			}
			at = index
		}
	}
	written := "| " + strings.Join([]string{row.Finding, row.Disposition, tableCell(row.Reasoning), tableCell(row.Amendment)}, " | ") + " |"
	if at < 0 {
		return refuse("template", fmt.Sprintf("round %d's decisions file has no row for %s; the engine writes one per finding", round, row.Finding))
	}
	lines[at] = written
	candidate := strings.Join(lines, "\n")
	// The engine's own join judges the row as it would at the close: a row
	// it would refuse is not written.
	probe, err := os.CreateTemp(filepath.Dir(own), ".decisions-*.md")
	if err != nil {
		return httpd.DesignLoop{}, err
	}
	probe.WriteString(candidate)
	probe.Close()
	defer os.Remove(probe.Name())
	for _, violation := range validate.CritiqueClosed(returnPath, probe.Name()) {
		if strings.Contains(violation, "'"+row.Finding+"'") {
			return refuse("join", violation)
		}
	}
	if _, err := atomicfile.WriteText(own, candidate, filepath.Dir(own)); err != nil {
		return httpd.DesignLoop{}, err
	}
	return designLoopRead(roots, id)
}

// dispositionCells is a table row's cells, or nil for a line that is not one.
func dispositionCells(line string) []string {
	return validate.TableCells(line)
}

// writeRoundTemplate writes the engine's template as the round's own file.
func writeRoundTemplate(roots lifecycle.Roots, chain dispatchcore.DesignCritiqueChain, recordID string, round int64, returnPath string, findings []intentFinding) error {
	template, problem := designTemplate(roots, chain, recordID, round, returnPath, findings)
	if problem != nil {
		return problem
	}
	return os.WriteFile(roundDecisionsPath(returnPath), []byte(template), 0o600)
}

// designTemplate is the decisions file the engine writes for this round,
// under the same binding: the goal, the design, the examination, its round
// and the subject and return digests.
func designTemplate(roots lifecycle.Roots, chain dispatchcore.DesignCritiqueChain, recordID string, round int64, returnPath string, findings []intentFinding) (string, error) {
	var entry designReviewEntry
	data, err := os.ReadFile(roots.Installation.Path("artifacts", "agents", "intent-review", "design-"+strings.ToLower(recordID), "chain.json"))
	if err == nil {
		err = json.Unmarshal(data, &entry)
	}
	subject := entry.Subjects[strconv.FormatInt(round, 10)]
	if err != nil || subject == "" {
		return "", &httpd.DesignRefusal{Code: "subject", Message: fmt.Sprintf(
			"the design version examination %d read is not recorded here, so its decisions cannot be bound to it", round)}
	}
	digest, _, err := reviewReturnDigest(returnPath)
	if err != nil {
		return "", err
	}
	goal := chain.Goal
	if goal == "" {
		goal = entry.Goal
	}
	return decisionsDocument(reviewBinding{Goal: goal, Work: "design:" + recordID, Attempt: int(round), Subject: subject,
		Examination: chain.Root, Round: round, Return: digest}, findings), nil
}
