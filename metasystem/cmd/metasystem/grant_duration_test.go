package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestADelegatedOnlyGrantMayRunThirtyDays(t *testing.T) {
	t.Parallel()
	b := newDecisionAgentBed(t)
	lineage := b.lineage
	b.lineage = ""
	now, _ := b.commandNow(b.root())
	personOwners := b.grantEverythingBed.owners()
	code, got := b.runJSON(personOwners, "grant", "add", "--acts", "everything", "--classes", "run-release", "--lineage", "seat", "--for", "720h")
	if code != 0 || got.Outcome != intentConfirmed || len(got.Targets) != 1 || got.Targets[0].ID == "" {
		t.Fatalf("thirty-day grant: %d %+v", code, got)
	}
	endpoint, err := b.dependencies().endpoint(b.root())
	helmMust(t, err)
	_, entries, err := goal.GeneralGrantsAt(endpoint)
	helmMust(t, err)
	index := slices.IndexFunc(entries, func(entry goal.PowerOfAttorneyEntry) bool { return entry.ID == got.Targets[0].ID })
	if index < 0 {
		t.Fatalf("grant %s is missing from the ledger", got.Targets[0].ID)
	}
	entry := entries[index]
	if !slices.Equal(entry.Classes, []string{"run-release"}) || entry.Lineage != lineage ||
		entry.Until != now.Add(720*time.Hour).Truncate(time.Minute).UTC().Format(time.RFC3339) {
		t.Fatalf("recorded grant: %+v", entry)
	}
	if live, why := entry.LiveAt(now); !live {
		t.Fatalf("recorded grant is not live: %s", why)
	}
	for _, row := range []struct{ classes, duration string }{
		{"run-release", "744h"}, {"run-release,ordering", "192h"},
	} {
		code, got := b.runJSON(personOwners, "grant", "add", "--acts", "everything", "--classes", row.classes, "--for", row.duration)
		classes, err := goal.ParseAttorneyClasses(row.classes)
		helmMust(t, err)
		duration, err := time.ParseDuration(row.duration)
		helmMust(t, err)
		proof, err := b.prove(b.root(), 0, nil, "", "", now)
		helmMust(t, err)
		_, creationErr := goal.GrantGeneral(goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Human: "Wido"}, Now: now}, &proof,
			goal.GeneralGrant{Machine: entry.For, Checkout: entry.Checkout, Lineage: entry.Lineage, Classes: classes, Until: now.Add(duration).Truncate(time.Minute)})
		bad := entry
		bad.Classes, bad.Until = classes, now.Add(duration).Truncate(time.Minute).UTC().Format(time.RFC3339)
		within, why := bad.WithinBounds()
		if code != 2 || got.Outcome != intentRefused || creationErr == nil || within {
			t.Fatalf("%s for %s: command=%d %+v creation=%v recorded=%t %s", row.classes, row.duration, code, got, creationErr, within, why)
		}
		for _, refusal := range []string{got.Summary, creationErr.Error(), why} {
			if !strings.Contains(refusal, "30 days") || !strings.Contains(refusal, "7 days") {
				t.Fatalf("duration refusal lacks both limits: %s", refusal)
			}
		}
	}
	day29 := now.Add(29 * 24 * time.Hour)
	b.lineage = lineage
	b.owners.commandNow = func(string) (time.Time, error) { return day29, nil }
	var admitted *humanauthority.HelmGrant
	prove := b.owners.prove
	b.owners.prove = func(root string, pid int64, reader humanauthority.Reader, by, tty string, at time.Time) (humanauthority.Proof, error) {
		proof, err := prove(root, pid, reader, by, tty, at)
		admitted = proof.Helm
		return proof, err
	}
	code, got = b.record([]string{"--class", "run-release"}, []string{"work", "stop", "run:R"})
	decisions, bad, err := humanauthority.ReadDecisions(b.home)
	if err != nil || bad != 0 || len(decisions) != 1 || b.calls != 1 || admitted == nil || admitted.Grant != entry.ID ||
		admitted.Act != "run-release" || admitted.Decision != decisions[0].ID || admitted.Session != b.holder.MainId ||
		code != 1 || got.Outcome != intentRefused || !strings.Contains(got.Summary, "this shell acts under the helm or a grant") {
		t.Fatalf("day 29: %d %+v proof=%+v calls=%d decisions=%+v bad=%d err=%v", code, got, admitted, b.calls, decisions, bad, err)
	}
	t.Logf("720h grant read back; day 29 decision %s admitted run-release; run cancellation refused at the person gate", decisions[0].ID)
}
