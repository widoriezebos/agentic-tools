package dispatch

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const budgetExtensionEvidenceWindow = 2 * time.Hour

// BudgetExtensionTuple is the full before-or-after budget named by an offer.
type BudgetExtensionTuple struct {
	ElapsedLimit            string `json:"elapsedLimit"`
	AttemptLimit            uint64 `json:"attemptLimit"`
	ReservedJobMinutesLimit uint64 `json:"reservedJobMinutesLimit"`
	ActiveJobLimit          uint64 `json:"activeJobLimit"`
	ReviewRoundLimit        int64  `json:"reviewRoundLimit"`
}

// BudgetExtensionOffer is a read-only proposal from the exact revision seam.
// The goal verb journals these coordinates before publishing the one marker.
type BudgetExtensionOffer struct {
	EvidenceKind string               `json:"evidenceKind"`
	EvidenceID   string               `json:"evidenceId"`
	EvidenceAt   string               `json:"evidenceAt"`
	From         BudgetExtensionTuple `json:"from"`
	To           BudgetExtensionTuple `json:"to"`
}

type advancementEvidence struct {
	kind string
	id   string
	at   time.Time
}

func budgetExtensionOffer(repoRoot string, file *goal.GoalFile, tier uint8, now time.Time) (*BudgetExtensionOffer, error) {
	if file == nil || file.Budget == nil || file.BudgetExtension != nil {
		return nil, nil
	}
	box, err := config.TierBox(filepath.Join(repoRoot, "metasystem.conf"), tier)
	if err != nil {
		return nil, err
	}
	if file.Budget.AttemptLimit > ^uint64(0)-box.AttemptLimit ||
		file.Budget.ReservedJobMinutesLimit > ^uint64(0)-box.ReservedJobMinutesLimit {
		return nil, nil
	}
	evidence, err := latestAdvancementEvidence(repoRoot, file.Id, now)
	if err != nil || evidence == nil {
		return nil, err
	}
	from := budgetExtensionTuple(*file.Budget)
	to := from
	to.AttemptLimit += box.AttemptLimit
	to.ReservedJobMinutesLimit += box.ReservedJobMinutesLimit
	return &BudgetExtensionOffer{
		EvidenceKind: evidence.kind, EvidenceID: evidence.id,
		EvidenceAt: evidence.at.UTC().Format(time.RFC3339Nano), From: from, To: to,
	}, nil
}

func budgetExtensionTuple(b goal.Budget) BudgetExtensionTuple {
	return BudgetExtensionTuple{
		ElapsedLimit: b.ElapsedLimit, AttemptLimit: b.AttemptLimit,
		ReservedJobMinutesLimit: b.ReservedJobMinutesLimit, ActiveJobLimit: b.ActiveJobLimit,
		ReviewRoundLimit: b.ReviewRoundLimit,
	}
}

func latestAdvancementEvidence(repoRoot, goalID string, now time.Time) (*advancementEvidence, error) {
	var candidates []advancementEvidence
	candidates = append(candidates, reviewAdvancementEvidence(repoRoot, goalID, now)...)
	landingEvidence, err := landingAdvancementEvidence(repoRoot, goalID, now)
	if err != nil {
		return nil, fmt.Errorf("budget extension landing evidence: %w", err)
	}
	candidates = append(candidates, landingEvidence...)
	proofEvidence, err := proofAdvancementEvidence(repoRoot, goalID, now)
	if err != nil {
		return nil, fmt.Errorf("budget extension proof evidence: %w", err)
	}
	candidates = append(candidates, proofEvidence...)
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].at.Equal(candidates[j].at) {
			return candidates[i].at.After(candidates[j].at)
		}
		if candidates[i].kind != candidates[j].kind {
			return candidates[i].kind < candidates[j].kind
		}
		return candidates[i].id < candidates[j].id
	})
	return &candidates[0], nil
}

func evidenceInWindow(at, now time.Time) bool {
	at, now = at.UTC(), now.UTC()
	return !at.After(now) && !at.Before(now.Add(-budgetExtensionEvidenceWindow))
}

func reviewAdvancementEvidence(repoRoot, goalID string, now time.Time) []advancementEvidence {
	state := loadCritiqueState(repoRoot)
	var evidence []advancementEvidence
	for reviewedID, reviewed := range state.records {
		if state.chainRoot(reviewedID) != reviewedID || asString(reviewed["goalId"]) != goalID {
			continue
		}
		criticID := asString(reviewed[independentCritiqueReferenceField])
		critic, present := state.records[criticID]
		if criticID == "" || !present || state.chainRoot(criticID) != criticID ||
			asString(critic["role"]) != "code-critic" || asString(critic["goalId"]) != goalID {
			continue
		}
		closed, ok := critic["chainClosed"].(bool)
		if !ok || !closed {
			continue
		}
		register, present, err := critiqueFindingRegister(critic)
		if err != nil || !present || len(openRegisterFindingIDs(register)) != 0 {
			continue
		}
		latest := state.latestMember(criticID)
		if latest == nil || asString(latest["status"]) != "completed" {
			continue
		}
		latestRound, latestOK := numInt(latest["round"])
		foldedRound, foldedErr := findingRegisterRound(critic, len(register))
		if !latestOK || latestRound < 1 || foldedErr != nil || foldedRound != latestRound {
			continue
		}
		endedAt, err := time.Parse(time.RFC3339Nano, asString(latest["endedAt"]))
		if err == nil && evidenceInWindow(endedAt, now) {
			evidence = append(evidence, advancementEvidence{kind: "review", id: criticID, at: endedAt})
		}
	}
	return evidence
}

type correctedReceipt struct {
	epoch  string
	id     string
	at     time.Time
	fields map[string]string
}

type receiptCorrection struct {
	identity string
	field    string
	value    string
}

func landingAdvancementEvidence(repoRoot, goalID string, now time.Time) ([]advancementEvidence, error) {
	tip, exists, err := goal.AcceptedLedgerTip(repoRoot)
	if err != nil || !exists {
		return nil, err
	}
	workspace := gittree.Workspace{Dir: repoRoot}
	receiptPath, err := budgetExtensionReceiptPath(workspace, repoRoot)
	if err != nil {
		return nil, err
	}
	data, present, err := workspace.FileAt(tip, receiptPath)
	if err != nil || !present {
		return nil, err
	}
	originals := map[string][]*correctedReceipt{}
	var records []*correctedReceipt
	var corrections []receiptCorrection
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		fields := map[string]string{}
		for _, field := range parts[3:] {
			if key, value, found := strings.Cut(field, "="); found && key != "" {
				fields[key] = value
			}
		}
		switch parts[2] {
		case "RECEIPT":
			epoch, parseErr := strconv.ParseInt(parts[0], 10, 64)
			if parseErr != nil || epoch < 0 {
				continue
			}
			digest := sha1.Sum([]byte(line))
			sha := hex.EncodeToString(digest[:])
			record := &correctedReceipt{epoch: parts[0], id: parts[0] + "-" + sha, at: time.Unix(epoch, 0).UTC(), fields: fields}
			records = append(records, record)
			key := parts[0] + "|" + sha
			originals[key] = append(originals[key], record)
		case "CORRECTION":
			if fields["field"] != "" {
				corrections = append(corrections, receiptCorrection{
					identity: fields["ref_epoch"] + "|" + fields["ref_sha1"],
					field:    fields["field"], value: fields["now"],
				})
			}
		}
	}
	for _, correction := range corrections {
		for _, record := range originals[correction.identity] {
			record.fields[correction.field] = correction.value
		}
	}
	var evidence []advancementEvidence
	for _, record := range records {
		if record.fields["goal"] == goalID && record.fields["type"] == "implement" &&
			record.fields["outcome"] == "shipped" && evidenceInWindow(record.at, now) {
			evidence = append(evidence, advancementEvidence{kind: "landing", id: record.id, at: record.at})
		}
	}
	return evidence, nil
}

// budgetExtensionReceiptPath names the application receipt ledger inside a
// whole-repository tree. A template installation is a repository subtree;
// an adopted nested installation keeps the ledger at its application root.
func budgetExtensionReceiptPath(workspace gittree.Workspace, repoRoot string) (string, error) {
	top, err := workspace.TopLevel()
	if err != nil {
		return "", err
	}
	appRoot, err := stateroot.RootForInstallation(repoRoot)
	if err != nil {
		return "", err
	}
	top = resolvedBudgetExtensionPath(top)
	appRoot = resolvedBudgetExtensionPath(appRoot)
	relativeApp, err := filepath.Rel(top, appRoot)
	if err != nil || relativeApp == ".." || strings.HasPrefix(relativeApp, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("budget extension receipt root %s is outside repository %s", appRoot, top)
	}
	receiptRoot, err := stateroot.RelativeRoot(stateroot.Receipts)
	if err != nil {
		return "", err
	}
	if relativeApp == "." {
		relativeApp = ""
	}
	return path.Join(filepath.ToSlash(relativeApp), receiptRoot, "receipts.log"), nil
}

func resolvedBudgetExtensionPath(value string) string {
	absolute, err := filepath.Abs(value)
	if err == nil {
		value = absolute
	}
	if resolved, resolveErr := filepath.EvalSymlinks(value); resolveErr == nil {
		value = resolved
	}
	return filepath.Clean(value)
}

func proofAdvancementEvidence(repoRoot, goalID string, now time.Time) ([]advancementEvidence, error) {
	attempts, err := proofrun.ReadAttempts(repoRoot)
	if err != nil {
		return nil, err
	}
	var evidence []advancementEvidence
	for _, attempt := range attempts {
		if candidate := proofAttemptAdvancement(attempt, goalID, now); candidate != nil {
			evidence = append(evidence, *candidate)
		}
	}
	return evidence, nil
}

func proofAttemptAdvancement(attempt proofrun.Attempt, goalID string, now time.Time) *advancementEvidence {
	if attempt.GoalID != goalID || attempt.TestResult == nil || attempt.Terminal == nil ||
		attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult.Purpose != testpolicy.PurposeDelivery ||
		attempt.EndedAt == "" || attempt.Terminal.At != attempt.EndedAt ||
		!attempt.TestResult.Delivery.Sufficient || proofrun.ValidateTestResult(*attempt.TestResult) != nil {
		return nil
	}
	endedAt, err := time.Parse(time.RFC3339Nano, attempt.EndedAt)
	if err != nil || !evidenceInWindow(endedAt, now) {
		return nil
	}
	return &advancementEvidence{kind: "receipt", id: attempt.AttemptID, at: endedAt}
}

func (o BudgetExtensionOffer) GoalOffer() goal.BudgetExtensionOffer {
	return goal.BudgetExtensionOffer{
		EvidenceKind: o.EvidenceKind, EvidenceID: o.EvidenceID, EvidenceAt: o.EvidenceAt,
		AttemptLimitFrom: o.From.AttemptLimit, AttemptLimitTo: o.To.AttemptLimit,
		ReservedJobMinutesFrom: o.From.ReservedJobMinutesLimit, ReservedJobMinutesTo: o.To.ReservedJobMinutesLimit,
	}
}

func (o BudgetExtensionOffer) Validate() error {
	if o.EvidenceKind == "" || o.EvidenceID == "" || o.EvidenceAt == "" ||
		o.From.ElapsedLimit != o.To.ElapsedLimit || o.From.ActiveJobLimit != o.To.ActiveJobLimit ||
		o.From.ReviewRoundLimit != o.To.ReviewRoundLimit || o.To.AttemptLimit <= o.From.AttemptLimit ||
		o.To.ReservedJobMinutesLimit <= o.From.ReservedJobMinutesLimit {
		return fmt.Errorf("budget extension offer is incomplete or changes an unextendable member")
	}
	return nil
}
