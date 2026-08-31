package counselor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	windowRule         = "Each observation is a completed half-open Coordinated Universal Time calendar week that carries retained evidence, plus the most recently completed week; the active week is excluded."
	classificationRule = "Goal events are deduplicated by operation identifier. Exact open, edit, claim, and set-budget events plus non-carrier record-only landings are process activities; exact done events plus product landings are product outcomes. A Git landing whose Goal-Transaction trailer matches a counted goal operation is that operation's carrier, is shown separately, and is not counted again. Every other landing is record-only when it changes at least one path and every changed path is under memory/, plans/, or records/. A landing with any other changed path is product, and a landing with no numstat path is unclassified."
)

// Build reads the checkout's durable evidence and computes both advisory
// signals. Unreadable or incomplete evidence becomes typed limitations rather
// than a refusal to produce counsel.
func Build(options Options) Brief {
	root := options.Root
	if root == "" {
		root = "."
	}
	return Compute(loadRecordSet(root), options.now())
}

// Compute derives both signals from a fixture-ready durable record projection.
func Compute(records RecordSet, now time.Time) Brief {
	now = now.UTC().Truncate(time.Second)
	windows := observedWindows(records, now)
	brief := Brief{
		GeneratedAt: now,
		WindowRule:  windowRule,
		SpendVsOutcome: SpendVsOutcomeTrend{
			Windows:     make([]SpendVsOutcomeWindow, len(windows)),
			Limitations: append([]Limitation(nil), records.SpendLimitations...),
		},
		ProcessVsProduct: ProcessVsProductTrend{
			Windows:            make([]ProcessVsProductWindow, len(windows)),
			ClassificationRule: classificationRule,
			Limitations:        append([]Limitation(nil), records.ActivityLimitations...),
		},
	}
	for index, window := range windows {
		brief.SpendVsOutcome.Windows[index].Window = window
		brief.ProcessVsProduct.Windows[index].Window = window
	}

	for _, record := range records.Runs {
		index := windowIndex(windows, record.CompletedAt)
		if index < 0 {
			continue
		}
		target := &brief.SpendVsOutcome.Windows[index].Tracked
		if record.Kind == RunGoverned {
			target = &brief.SpendVsOutcome.Windows[index].Governed
		}
		target.Runs++
		target.Duration += record.Duration
		target.Outcomes.add(record.Outcome)
	}

	countedGoalOperations := make(map[string]bool, len(records.GoalEvents))
	for _, event := range records.GoalEvents {
		countedGoalOperations[event.OperationID] = true
	}
	unmatchedGoalCarriers := 0
	for _, record := range records.Landings {
		index := windowIndex(windows, record.CompletedAt)
		if index < 0 {
			continue
		}
		landings := &brief.SpendVsOutcome.Windows[index].Landings
		landings.Commits++
		landings.Files += record.Files
		landings.Insertions += record.Insertions
		landings.BinaryFiles += record.BinaryFiles

		activity := &brief.ProcessVsProduct.Windows[index]
		if record.GoalOperationID != "" && countedGoalOperations[record.GoalOperationID] {
			activity.GoalCarrierLandings++
			continue
		}
		if record.GoalOperationID != "" {
			unmatchedGoalCarriers++
		}
		switch classifyLanding(record.Paths) {
		case landingRecordOnly:
			activity.RecordOnlyLandings++
		case landingProduct:
			activity.ProductLandings++
		default:
			activity.UnclassifiedLandings++
		}
	}

	for _, event := range records.GoalEvents {
		index := windowIndex(windows, event.At)
		if index < 0 {
			continue
		}
		counts := &brief.ProcessVsProduct.Windows[index].GoalEvents
		switch event.Class {
		case GoalOpen:
			counts.Open++
		case GoalEdit:
			counts.Edit++
		case GoalClaim:
			counts.Claim++
		case GoalBudget:
			counts.Budget++
		case GoalDone:
			counts.Done++
		}
	}

	for index := range brief.ProcessVsProduct.Windows {
		window := &brief.ProcessVsProduct.Windows[index]
		process := window.GoalEvents.Open + window.GoalEvents.Edit + window.GoalEvents.Claim + window.GoalEvents.Budget + window.RecordOnlyLandings
		product := window.GoalEvents.Done + window.ProductLandings
		window.Ratio = ActivityRatio{ProcessActivities: process, ProductOutcomes: product}
		if product > 0 {
			value := float64(process) / float64(product)
			window.Ratio.Value = &value
		}
	}

	brief.SpendVsOutcome.Limitations = appendUniqueLimitations(brief.SpendVsOutcome.Limitations,
		Limitation{
			Name:       "Risk-class evidence",
			Detail:     "Current durable records carry landing counts and size but no landing risk class, so this signal reports spend versus outcome and makes no risk verdict.",
			Enrichment: "Record a risk class with each landing.",
		},
		Limitation{
			Name:       "Threshold history",
			Detail:     "Configuration exposes a current scalar validation-weight threshold, but durable run and landing records do not preserve which threshold was effective in each window, so the brief makes no threshold comparison.",
			Enrichment: "Record the effective threshold value and its source with each weight generation.",
		},
		Limitation{
			Name:       "Causal attribution",
			Detail:     "Run spend and Git landings are aggregated by completion time in the same week; records do not identify which proof caused or validated which commit.",
			Enrichment: "Carry the landing commit identifier on governed attempts and retained tracked-run outcomes.",
		},
		Limitation{
			Name:       "Tracked-run retention",
			Detail:     "Ordinary tracked runs are represented only while their run records are retained; acknowledged terminal records can be pruned and ungoverned identifiers can be reused.",
			Enrichment: "Add a non-prunable terminal ledger for ordinary tracked runs.",
		},
		Limitation{
			Name:       "Window resolution",
			Detail:     "Delivery follows the repository's counselor cadence, while the signal windows remain completed Coordinated Universal Time calendar weeks and omit the active partial week and empty older weeks.",
			Enrichment: "Record finer-grained durable evidence before producing windows shorter than one week.",
		},
		Limitation{
			Name:       "Landing size",
			Detail:     "Git numstat supplies insertion counts for text paths; binary paths have no insertion count and merge commits may carry no numstat payload.",
			Enrichment: "Record byte sizes and an explicit merge-payload attribution rule.",
		},
	)

	brief.ProcessVsProduct.Limitations = appendUniqueLimitations(brief.ProcessVsProduct.Limitations,
		Limitation{
			Name:       "Path classification",
			Detail:     "The product count is a path proxy: every landing with a path outside memory/, plans/, and records/ is product, so non-code files outside those roots are included.",
			Enrichment: "Publish a repository-owned code-path projection and carry it with landing records.",
		},
		Limitation{
			Name:       "Goal-verb coverage",
			Detail:     "Only exact open, edit, claim, set-budget, and done history verbs are classified. Combined open-claim and every other verb are excluded because history has no closed activity-class field; a claim line also does not reveal an implicit budget.",
			Enrichment: "Add a closed activity class to each goal-history event, including combined operations.",
		},
		Limitation{
			Name:       "Goal-history retention",
			Detail:     "The signal reads retained live, concluded, and legacy goal files; pruning can remove concluded records, so this is not guaranteed all-time activity.",
			Enrichment: "Retain an append-only goal-event journal independent of pruned goal files.",
		},
		Limitation{
			Name:       "Register-edit classification",
			Detail:     "Registers and receipts both resolve under the broad memory/ record root, so register edits cannot be separated from other record-only landings and remain inside that count.",
			Enrichment: "Publish a dedicated register path or a durable typed landing class that distinguishes register edits from other records.",
		},
		Limitation{
			Name:       "Cross-source attribution",
			Detail:     "A matching Goal-Transaction trailer prevents a counted goal operation's carrier commit from being counted twice. A goal-record change without a matching trailer cannot be joined, so a mixed product commit can still contribute both a goal event and a product landing.",
			Enrichment: "Require every goal-record mutation to retain its goal operation identifier in the landing commit.",
		},
		Limitation{
			Name:       "Window resolution",
			Detail:     "Delivery follows the repository's counselor cadence, while the signal windows remain completed Coordinated Universal Time calendar weeks and omit the active partial week and empty older weeks.",
			Enrichment: "Record finer-grained durable evidence before producing windows shorter than one week.",
		},
	)
	if unmatchedGoalCarriers > 0 {
		brief.ProcessVsProduct.Limitations = appendUniqueLimitations(brief.ProcessVsProduct.Limitations, Limitation{
			Name:       "Goal-carrier reconciliation",
			Detail:     pluralCount(unmatchedGoalCarriers, "Git landing carried a Goal-Transaction identifier", "Git landings carried Goal-Transaction identifiers") + " that did not match a classified retained goal operation; those landings were classified by path instead of being silently discarded.",
			Enrichment: "Restore the matching retained goal history or add a closed activity class for the operation's verb.",
		})
	}
	return brief
}

func pluralCount(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func windowIndex(windows []Window, stamp time.Time) int {
	for index, window := range windows {
		if window.contains(stamp) {
			return index
		}
	}
	return -1
}

type landingClass int

const (
	landingUnclassified landingClass = iota
	landingRecordOnly
	landingProduct
)

func classifyLanding(paths []string) landingClass {
	if len(paths) == 0 {
		return landingUnclassified
	}
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(path))
		root, _, _ := strings.Cut(path, "/")
		if root != "memory" && root != "plans" && root != "records" {
			return landingProduct
		}
	}
	return landingRecordOnly
}

func appendUniqueLimitations(existing []Limitation, additions ...Limitation) []Limitation {
	seen := make(map[string]bool, len(existing)+len(additions))
	result := make([]Limitation, 0, len(existing)+len(additions))
	for _, limitation := range append(existing, additions...) {
		if seen[limitation.Name] {
			continue
		}
		seen[limitation.Name] = true
		result = append(result, limitation)
	}
	return result
}
