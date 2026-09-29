package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// A design critique's rounds are answered one decisions file each, beside the
// round's return (g1-s66 D2, D4). Whatever file the author answered with, the
// validated decisions are kept as the round's own, so the close reads every
// answered round from one place: the register learns each finding's latest
// decision, and the design gains its Dispositions section, one row per
// finding per round, when the chain closes.

// designRoundLimit is the round limit the dispatch owner froze on the chain's
// root; a root that carries none is a design critique's two rounds.
func (inv *intentInvocation) designRoundLimit(root string) int64 {
	if record, err := inv.jobRecord(root); err == nil {
		if limit, ok := record["reviewRoundLimit"].(float64); ok && limit >= 1 {
			return int64(limit)
		}
	}
	return designCritiqueRounds
}

// roundDecisionsPath is the round's own decisions file, the template the
// engine writes beside the return.
func roundDecisionsPath(returnPath string) string {
	return filepath.Join(filepath.Dir(returnPath), "decisions.md")
}

// retainRoundDecisions keeps validated decisions as the round's own file.
func retainRoundDecisions(returnPath, answered string, content []byte) error {
	own := roundDecisionsPath(returnPath)
	if existing, err := os.ReadFile(own); err == nil && string(existing) == string(content) {
		return nil
	}
	if same, err := filepath.EvalSymlinks(answered); err == nil {
		if ownResolved, ownErr := filepath.EvalSymlinks(own); ownErr == nil && same == ownResolved {
			return nil
		}
	}
	return os.WriteFile(own, content, 0o600)
}

// closedByTheseDecisions reports whether the decisions file this call names
// answers the closed chain's final examination: the same answer again.
func (inv *intentInvocation) closedByTheseDecisions(chain dispatchcore.DesignCritiqueChain) bool {
	content, err := os.ReadFile(inv.flagPath("dispositions"))
	if err != nil {
		return false
	}
	bound, err := readReviewBinding(content)
	return err == nil && bound.Examination == chain.Root && bound.Round == chain.NewestRound
}

// answeredRound is one round as the Dispositions section reads it: the
// findings its return carried and the decisions its own file records.
type answeredRound struct {
	round    int64
	findings map[string]string // id -> claim
	rows     []validate.DispositionRow
}

// answeredDesignRounds reads every round of the chain that has a complete
// decisions file of its own, oldest first. A round whose file is still a
// template, or does not join its return, is not an answered round.
func (inv *intentInvocation) answeredDesignRounds(root string) []answeredRound {
	var rounds []answeredRound
	for round := int64(1); ; round++ {
		returnPath := inv.returnPathAt(inv.layout.InstallationRoot, root, round)
		if _, err := os.Stat(returnPath); err != nil {
			return rounds
		}
		own := roundDecisionsPath(returnPath)
		if violations := validate.CritiqueClosed(returnPath, own); len(violations) > 0 {
			continue
		}
		rows, violations := validate.DispositionRows(own)
		if len(violations) > 0 {
			continue
		}
		rounds = append(rounds, answeredRound{round: round, findings: roundClaims(returnPath), rows: rows})
	}
}

// roundClaims is each finding's claim, first line, from the round's return.
func roundClaims(returnPath string) map[string]string {
	claims := map[string]string{}
	data, err := os.ReadFile(returnPath)
	if err != nil {
		return claims
	}
	var result struct {
		Findings []struct {
			ID    string `json:"id"`
			Claim string `json:"claim"`
			Title string `json:"title"`
		} `json:"findings"`
	}
	if json.Unmarshal(data, &result) != nil {
		return claims
	}
	for _, finding := range result.Findings {
		claim := finding.Claim
		if claim == "" {
			claim = finding.Title
		}
		claims[finding.ID] = strings.TrimSpace(strings.Split(strings.ReplaceAll(claim, "\r\n", "\n"), "\n")[0])
	}
	return claims
}

// registerDecisions is what the register learns before the close of the
// answered round: each finding's latest decision across every answered round.
// A refutation and an out-of-scope stand from any round. An acceptance stands
// only from an earlier round, whose amendment the follow-up examined without
// raising the finding again; the answered round's own acceptances are left
// open, so the close classifies them.
func (inv *intentInvocation) registerDecisions(root string, answering int64) map[string]string {
	latest := map[string]string{}
	from := map[string]int64{}
	for _, round := range inv.answeredDesignRounds(root) {
		if round.round > answering {
			break
		}
		for _, row := range round.rows {
			latest[row.Finding], from[row.Finding] = row.Disposition, round.round
		}
	}
	decisions := map[string]string{}
	for id, disposition := range latest {
		switch {
		case disposition == "refuted" || disposition == "out-of-scope":
			decisions[id] = disposition
		case disposition == "accepted" && from[id] < answering:
			decisions[id] = disposition
		}
	}
	return decisions
}

// deferredObligations are the findings the close deferred into review
// obligations on the goal, each with the fixture that discharges it.
func (inv *intentInvocation) deferredObligations(root string) []string {
	record, err := inv.jobRecord(root)
	if err != nil {
		return nil
	}
	register, _ := record["findingRegister"].([]any)
	var obligations []string
	for _, raw := range register {
		entry, _ := raw.(map[string]any)
		if entry["status"] != "deferred" {
			continue
		}
		fixture, _ := entry["fixture"].(string)
		if fixture == "" {
			fixture, _ = entry["title"].(string)
		}
		obligations = append(obligations, fmt.Sprintf("%v: %s", entry["findingId"], fixture))
	}
	return obligations
}

// dispositionsHeading names the section a closed chain appends, so a design
// carries each chain's section once.
func dispositionsHeading(root string) string {
	return "## Dispositions (critique " + root + ")"
}

// appendDesignDispositions appends the closed chain's Dispositions section to
// the design, composed from every answered round: one row per finding per
// round, with the round, the reasoning and the amendment, so a recurring id
// shows each adjudication. A design already carrying the chain's section is
// left as it stands.
func appendDesignDispositions(design, root string, rounds []answeredRound) (bool, error) {
	content, err := os.ReadFile(design)
	if err != nil {
		return false, err
	}
	heading := dispositionsHeading(root)
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == heading {
			return false, nil
		}
	}
	var section strings.Builder
	section.WriteString(heading + "\n\n")
	section.WriteString("Written by metasystem design review when critique " + root + " closed: every answered round's decisions, as the author made them.\n\n")
	section.WriteString("| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |\n")
	section.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, round := range rounds {
		for _, row := range round.rows {
			section.WriteString("| " + strings.Join([]string{strconv.FormatInt(round.round, 10), row.Finding, tableCell(round.findings[row.Finding]),
				row.Disposition, row.Reasoning, row.Amendment}, " | ") + " |\n")
		}
	}
	text := strings.TrimRight(string(content), "\n") + "\n\n" + section.String()
	_, err = atomicfile.WriteText(design, text, filepath.Dir(design))
	return err == nil, err
}

// tableCell keeps a claim on its one table row.
func tableCell(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\n", " "), "|", `\|`)
}
