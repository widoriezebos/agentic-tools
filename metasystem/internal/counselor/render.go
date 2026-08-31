package counselor

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// Render writes the charter's narrative form: sentences and explicit cost and
// limitation lines, with no verdict or dashboard projection.
func Render(writer io.Writer, brief Brief) error {
	if _, err := fmt.Fprintf(writer, "Counselor drift brief generated at %s.\n", brief.GeneratedAt.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "This brief reads durable records only, offers advice, and makes no decision or refusal."); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Window rule: %s\n\n", brief.WindowRule); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "Spend versus outcome compares completed governed and tracked run time with current-branch Git landings in the same window."); err != nil {
		return err
	}
	for _, window := range brief.SpendVsOutcome.Windows {
		period := renderWindow(window.Window)
		if _, err := fmt.Fprintf(writer,
			"Cost: During %s, the governed run count was %d with %s spent, and the retained tracked run count was %d with %s spent.\n",
			period, window.Governed.Runs, renderMinutes(window.Governed.Duration), window.Tracked.Runs, renderMinutes(window.Tracked.Duration)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer,
			"Run outcomes during that window were governed green %d, red %d, ended unknown %d, and launch failed %d; retained tracked green %d, red %d, ended unknown %d, and launch failed %d.\n",
			window.Governed.Outcomes.Green, window.Governed.Outcomes.Red, window.Governed.Outcomes.EndedUnknown, window.Governed.Outcomes.LaunchFailed,
			window.Tracked.Outcomes.Green, window.Tracked.Outcomes.Red, window.Tracked.Outcomes.EndedUnknown, window.Tracked.Outcomes.LaunchFailed); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer,
			"Git landing count was %d, touching %d files with %d text insertions; %d binary files had no insertion count.\n",
			window.Landings.Commits, window.Landings.Files, window.Landings.Insertions, window.Landings.BinaryFiles); err != nil {
			return err
		}
	}
	if err := renderLimitations(writer, brief.SpendVsOutcome.Limitations); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "\nProcess versus product compares retained goal-ledger operations and non-carrier record-only landings with concluded goals and product landings."); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Classification rule: %s\n", brief.ProcessVsProduct.ClassificationRule); err != nil {
		return err
	}
	for _, window := range brief.ProcessVsProduct.Windows {
		period := renderWindow(window.Window)
		if _, err := fmt.Fprintf(writer,
			"During %s, the retained goal history recorded %d open, %d edit, %d claim, %d explicit budget, and %d done operations; Git landing classes were non-carrier record-only %d, matched goal-operation carrier %d, product %d, and unclassified %d.\n",
			period, window.GoalEvents.Open, window.GoalEvents.Edit, window.GoalEvents.Claim, window.GoalEvents.Budget, window.GoalEvents.Done,
			window.RecordOnlyLandings, window.GoalCarrierLandings, window.ProductLandings, window.UnclassifiedLandings); err != nil {
			return err
		}
		if window.Ratio.Value == nil {
			if _, err := fmt.Fprintf(writer,
				"Cost: The process-versus-product ratio is undefined because the process activity count is %d and the product outcome count is zero.\n",
				window.Ratio.ProcessActivities); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(writer,
			"Cost: The process-versus-product activity ratio is %s, from %s divided by %s.\n",
			renderRatio(*window.Ratio.Value), renderCount(window.Ratio.ProcessActivities, "process activity", "process activities"), renderCount(window.Ratio.ProductOutcomes, "product outcome", "product outcomes")); err != nil {
			return err
		}
	}
	if err := renderLimitations(writer, brief.ProcessVsProduct.Limitations); err != nil {
		return err
	}

	return renderAcceptedRiskRegister(writer, brief.AcceptedRiskRegister)
}

func renderAcceptedRiskRegister(writer io.Writer, register AcceptedRiskRegister) error {
	if register.Source == "" && register.CountingRule == "" && len(register.Classes) == 0 && len(register.Entries) == 0 && len(register.Limitations) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(writer, "\nAccepted risk and near-miss register reads %s and aggregates valid entries by class.\n", register.Source); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Counting rule: %s\n", register.CountingRule); err != nil {
		return err
	}
	if len(register.Classes) == 0 {
		if _, err := fmt.Fprintln(writer, "No accepted risks or near misses were loaded from the durable register."); err != nil {
			return err
		}
	} else {
		entriesByClass := map[string][]RegisterEntry{}
		for _, entry := range register.Entries {
			entriesByClass[entry.Class] = append(entriesByClass[entry.Class], entry)
		}
		for _, class := range register.Classes {
			if _, err := fmt.Fprintf(writer, "Class %s has %s: %s and %s.\n",
				class.Class, renderCount(len(class.EntryIDs), "valid entry", "valid entries"),
				renderCount(class.AcceptedRisks, "accepted risk", "accepted risks"),
				renderCount(class.NearMisses, "near miss", "near misses")); err != nil {
				return err
			}
			for _, entry := range entriesByClass[class.Class] {
				if err := renderAcceptedRiskEntry(writer, entry); err != nil {
					return err
				}
			}
		}
	}
	return renderLimitations(writer, register.Limitations)
}

func renderAcceptedRiskEntry(writer io.Writer, entry RegisterEntry) error {
	if _, err := fmt.Fprintf(writer, "Entry %s is %s recorded at %s: %s. Acceptance status: %s. Acceptance reason: %s\n",
		entry.ID, renderRegisterKindWithArticle(entry.Kind), entry.RecordedAt.UTC().Format(time.RFC3339), entry.Title, entry.AcceptanceStatus, entry.AcceptanceReason); err != nil {
		return err
	}
	for _, fact := range entry.SpecimenFacts {
		if _, err := fmt.Fprintf(writer, "Specimen fact: %s Citations: %s.\n", fact.Fact, renderRegisterCitations(fact.Citations)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "Review linkage: %s.\n", renderRegisterReviewLinks(entry.ReviewLinks)); err != nil {
		return err
	}
	return nil
}

func renderRegisterKindWithArticle(kind RegisterEntryKind) string {
	switch kind {
	case RegisterAcceptedRisk:
		return "an accepted risk"
	case RegisterNearMiss:
		return "a near miss"
	default:
		return string(kind)
	}
}

func renderRegisterCitations(citations []RegisterCitation) string {
	parts := make([]string, 0, len(citations))
	for _, citation := range citations {
		parts = append(parts, renderRegisterReference(citation.Kind, citation.Target, citation.Detail))
	}
	return strings.Join(parts, "; ")
}

func renderRegisterReviewLinks(links []RegisterReviewLink) string {
	parts := make([]string, 0, len(links))
	for _, link := range links {
		parts = append(parts, renderRegisterReference(link.Kind, link.Target, link.Detail))
	}
	return strings.Join(parts, "; ")
}

func renderRegisterReference(kind, target, detail string) string {
	if detail == "" {
		return kind + " " + target
	}
	return kind + " " + target + " (" + detail + ")"
}

func renderLimitations(writer io.Writer, limitations []Limitation) error {
	for _, limitation := range limitations {
		if _, err := fmt.Fprintf(writer, "Limitation — %s: %s Enrichment: %s\n", limitation.Name, limitation.Detail, limitation.Enrichment); err != nil {
			return err
		}
	}
	return nil
}

func renderWindow(window Window) string {
	return window.Start.UTC().Format("2006-01-02") + " through " + window.End.UTC().Format("2006-01-02") + " (end excluded)"
}

func renderMinutes(duration time.Duration) string {
	minutes := duration.Minutes()
	if math.Abs(minutes-math.Round(minutes)) < 0.0000001 {
		return strconv.FormatInt(int64(math.Round(minutes)), 10) + " minutes"
	}
	return strconv.FormatFloat(minutes, 'f', 1, 64) + " minutes"
}

func renderRatio(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func renderCount(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(count) + " " + plural
}
