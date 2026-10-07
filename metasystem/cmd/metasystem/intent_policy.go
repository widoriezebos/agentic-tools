package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

func (inv *intentInvocation) policyReaders() config.PolicyReaders {
	readers := inv.owners.policies
	if readers.Registry == nil {
		readers.Registry = func(checkout string) (result config.PolicyRegistry, err error) {
			var laneProblem error
			defer func() { err = errors.Join(laneProblem, err) }()
			home, err := inv.boardHome()
			if err != nil {
				return result, err
			}
			record, present, _, err := lane.ReadGuarded(home)
			if err != nil {
				laneProblem = &config.PolicyReadError{Source: "lane", Err: err}
			}
			if present {
				result.Lane = record.Root
			}
			layout, err := inv.owners.resolver.ResolveLayout(checkout)
			if err != nil {
				return result, err
			}
			// Coordinator declarations are checkout records; the template
			// keeps its own records inside the installation.
			rootPath := checkoutAuthorityRoot(layout)
			ledger := ""
			if endpoint := inv.owners.dependencies.endpoint; endpoint != nil {
				resolved, err := endpoint(rootPath)
				if err != nil {
					return result, err
				}
				ledger = goal.ExistingLedgerIdentityAtEndpoint(resolved)
			} else {
				ledger = goal.ExistingLedgerIdentity(rootPath)
			}
			state := brain.Read(rootPath, ledger)
			if state.State == brain.Corrupt {
				return result, &config.PolicyReadError{Source: "coordinator", Checkout: rootPath, Err: fmt.Errorf("coordinator declaration %s is unreadable: %s", brain.Path(rootPath), state.Reason)}
			}
			if state.State == brain.Declared {
				result.Coordinator = checkout
				return result, nil
			}
			if ledger == "" {
				return result, nil
			}
			registryHome := filepath.Dir(home)
			data, err := os.ReadFile(brain.PointerPath(registryHome, ledger))
			if os.IsNotExist(err) {
				return result, nil
			}
			if err != nil {
				return result, err
			}
			target := strings.TrimSpace(string(data))
			if target == "" {
				// A blank pointer names no coordinator, as brain.Declare reads it.
				return result, nil
			}
			state = brain.Read(target, ledger)
			if state.State == brain.Corrupt {
				return result, &config.PolicyReadError{Source: "coordinator", Checkout: target, Err: fmt.Errorf("coordinator declaration %s is unreadable: %s", brain.Path(target), state.Reason)}
			}
			if state.State == brain.Declared {
				seat, err := helm.Locate(target)
				if err != nil {
					return result, err
				}
				result.Coordinator = seat.Checkout
			}
			return result, nil
		}
	}
	if readers.ConfPath == nil {
		readers.ConfPath = func(checkout string) (string, error) {
			layout, err := inv.owners.resolver.ResolveLayout(checkout)
			if err != nil {
				return "", err
			}
			return intentConfPath(layout), nil
		}
	}
	return readers
}

func (inv *intentInvocation) policyCheckout(layout stateroot.Layout) (string, error) {
	seat, err := helm.Locate(layout.GitRoot)
	if err != nil {
		return "", err
	}
	return seat.Checkout, nil
}

func (inv *intentInvocation) policyParams(key string) (config.GetParams, error) {
	checkout, err := inv.policyCheckout(inv.layout)
	if err != nil {
		return config.GetParams{}, err
	}
	calling := ""
	if seat, err := helm.Locate(inv.cwd); err == nil {
		calling = seat.Checkout
	}
	if calling == "" {
		calling = checkout
	}
	readers := inv.policyReaders()
	scope := config.PolicyScope(key)
	if scope == "lane" || scope == "coordinator" {
		registry, err := readers.Registry(checkout)
		if err != nil {
			return config.GetParams{}, err
		}
		owner := registry.Lane
		if scope == "coordinator" {
			owner = registry.Coordinator
		}
		if owner != "" {
			seat, err := helm.Locate(owner)
			if err != nil {
				return config.GetParams{}, err
			}
			checkout = seat.Checkout
		}
	}
	conf, err := readers.ConfPath(checkout)
	if err != nil {
		return config.GetParams{}, err
	}
	return config.GetParams{Key: key, ConfPath: conf, LookupEnv: inv.owners.lookupEnv, Policy: &config.PolicyContext{Checkout: checkout, CallingCheckout: calling, Readers: readers}}, nil
}

func (inv *intentInvocation) runPolicyShow(key string) int {
	params, err := inv.policyParams(key)
	var resolved config.PolicyResolution
	if err == nil {
		resolved, err = config.ResolvePolicy(params)
	}
	if err != nil {
		result := intentResult{Outcome: intentRefused, code: 1, Summary: "the policy cannot be read: " + err.Error()}
		if key == "settings.apply" {
			result.Decision = settingsDeclarationRemedy(key)
		} else {
			checkout := "PATH"
			if params.Policy != nil {
				checkout = params.Policy.Checkout
			}
			result.next = []string{"metasystem", "settings", "set", key, "auto", "--repo", checkout}
			result.nextReason = "a person at an enrolled terminal replaces the policy at its owner checkout"
			var problem *config.PolicyReadError
			if errors.As(err, &problem) {
				switch problem.Source {
				case "env":
					result.next = []string{"unset", config.EnvName(key)}
					result.nextReason = "remove the invalid environment override, then repeat the read"
				case "coordinator":
					result.next = []string{"metasystem", "settings", "coordinator", "--withdraw", "--by", inv.knownPerson(), "--repo", problem.Checkout}
					result.nextReason = "a person withdraws the unreadable coordinator declaration; settings coordinator --declare can declare it again"
				case "lane":
					result.next = []string{"metasystem", "landing", "set", "PATH"}
					result.nextReason = "a person registers the landing checkout again, replacing the unreadable lane record"
				case "helm":
					result.next = []string{"metasystem", "helm", "return", "--repo", problem.Checkout}
					result.nextReason = humanauthority.PersonActRemedy(shellCommand(result.next))
				}
			}
		}
		return inv.render(result)
	}
	meaning := ""
	for _, setting := range config.CompiledSettings() {
		if setting.Key == key {
			meaning = setting.Meaning
			break
		}
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: key + " is " + resolved.Value + " at " + resolved.Checkout, Data: resolved,
		view: func(page *textui.Page) {
			page.Headline(key+" is "+resolved.Value, resolved.Source)
			page.Section("", "").Text("checkout: " + resolved.Checkout)
			page.Section("", "").Text("setter: " + resolved.SetBy)
			at := "unknown"
			if !resolved.At.IsZero() {
				at = page.Env().Time(resolved.At)
			}
			page.Section("", "").Text("time: " + at)
			if resolved.Previous != nil {
				page.Section("", "").Text("helm holder: " + resolved.HelmHolder + "; underlying value: " + resolved.Previous.Value + " (" + resolved.Previous.Source + ")")
			}
			page.Section("", "").Text(meaning)
		}})
}

// settingsPerson tries the calling worktree, its primary checkout, then the
// destination. Roster writes retain their separate, destination-only gate.
func (inv *intentInvocation) settingsPerson(destination stateroot.Layout, act string, observed ...*humanauthority.Proof) (string, time.Time, *intentResult) {
	layouts := []stateroot.Layout{}
	if calling, err := inv.owners.resolver.ResolveLayout(inv.cwd); err == nil {
		layouts = append(layouts, calling)
	}
	if seat, err := helm.Locate(inv.cwd); err == nil {
		if calling, err := inv.owners.resolver.ResolveLayout(seat.Checkout); err == nil {
			layouts = append(layouts, calling)
		}
	}
	layouts = append(layouts, destination)
	seen := make(map[string]bool)
	for _, layout := range layouts {
		// Enrollment belongs to the checkout; the template enrolls in
		// its installation, as the terminal enrollment verb does.
		rootPath := checkoutAuthorityRoot(layout)
		if seen[rootPath] {
			continue
		}
		seen[rootPath] = true
		if inv.owners.prove == nil || inv.owners.commandNow == nil {
			continue
		}
		now, err := inv.owners.commandNow(rootPath)
		if err != nil {
			continue
		}
		proof, err := inv.owners.prove(rootPath, int64(os.Getppid()), nil, "", "", now)
		if err != nil {
			continue
		}
		if len(observed) > 0 {
			*observed[0] = proof
		}
		if proof.Helm != nil || !proof.EnrolledTerminalFor(rootPath) {
			_ = humanauthority.RecordAttorneyRefusal(rootPath, proof, act, "set only by the person's own proof", now)
			continue
		}
		name := "author unknown"
		enrollment, err := humanauthority.ReadEnrollment(rootPath)
		if err == nil && enrollment.Generation == proof.TerminalGeneration && enrollment.TerminalRef == proof.TerminalRef && strings.TrimSpace(enrollment.Human) != "" {
			name = enrollment.Human
		}
		return name, now, nil
	}
	failure := &intentResult{Outcome: intentRefused, code: 1, Summary: "only you set " + act + " at your enrolled terminal; neither the calling checkout nor the destination proved that person; nothing was done",
		next: inv.typedArgv(), nextReason: "run this at a person's terminal enrolled at the calling checkout or the destination, without a helm or grant"}
	return "", time.Time{}, failure
}

var errPolicyOwnerUndeclared = errors.New("the policy has no declared owner")

func (inv *intentInvocation) policyDestination(key string) (stateroot.Layout, string, error) {
	checkout, err := inv.policyCheckout(inv.layout)
	if err != nil {
		return inv.layout, "", err
	}
	scope := config.PolicyScope(key)
	canonical, err := inv.owners.resolver.ResolveLayout(checkout)
	if err != nil {
		return inv.layout, "", err
	}
	if scope == "seat" {
		return canonical, checkout, nil
	}
	registry, err := inv.policyReaders().Registry(checkout)
	if err != nil {
		// An explicit person's destination is usable despite advisory registry
		// damage. The proof is checked before any write at that destination.
		if inv.input.has("repo") {
			return canonical, checkout, nil
		}
		return inv.layout, "", err
	}
	target := registry.Lane
	if scope == "coordinator" {
		target = registry.Coordinator
	}
	if target == "" {
		return inv.layout, "", fmt.Errorf("%w: no %s is declared", errPolicyOwnerUndeclared, scope)
	}
	seat, err := helm.Locate(target)
	if err != nil {
		return inv.layout, "", err
	}
	layout, err := inv.owners.resolver.ResolveLayout(seat.Checkout)
	return layout, seat.Checkout, err
}

func (inv *intentInvocation) runPolicySet(key, value string) int {
	destination, checkout, err := inv.policyDestination(key)
	if err != nil {
		remedy := inv.publicArgv("landing", "set", "PATH")
		reason := "a person declares the policy owner, then repeats the write"
		if !errors.Is(err, errPolicyOwnerUndeclared) {
			remedy = []string{"metasystem", "settings", "set", key, value, "--repo", "PATH"}
			reason = "a person at an enrolled terminal supplies the exact destination"
		}
		if key == "question.route" && errors.Is(err, errPolicyOwnerUndeclared) {
			remedy = []string{"metasystem", "settings", "coordinator", "--declare", "--by", inv.knownPerson(), "--repo", inv.layout.GitRoot}
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "the policy destination cannot be resolved: " + err.Error() + "; nothing was done", next: remedy, nextReason: reason})
	}
	by, at, problem := inv.settingsPerson(destination, "settings set "+key)
	if problem != nil {
		problem.next = []string{"metasystem", "settings", "set", key, value, "--repo", checkout}
		return inv.render(*problem)
	}
	conf := intentConfPath(destination)
	local := conf + ".local"
	changed := false
	err = config.WithPolicyLock(checkout, func() error {
		_, fresh, err := inv.policyDestination(key)
		if err != nil {
			return err
		}
		if fresh != checkout {
			return fmt.Errorf("the policy owner changed from %s to %s; repeat the write", checkout, fresh)
		}
		before, err := os.ReadFile(local)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s cannot be preserved: %w", local, err)
		}
		if err == nil {
			if existing, found, readErr := config.ConfLookup(local, key); readErr == nil && found && existing == value {
				return nil
			}
		}
		if os.IsNotExist(err) {
			if err := os.WriteFile(local, nil, 0600); err != nil {
				return err
			}
		}
		if err := validate.SetConfKeys(local, []validate.ConfSetting{{Key: key, Value: value}}); err != nil {
			return err
		}
		after, err := os.ReadFile(local)
		if err != nil {
			return err
		}
		changed = !bytes.Equal(before, after)
		if !changed {
			return nil
		}
		return config.RecordPolicy(checkout, conf, key, value, by, at)
	})
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the policy write at " + local + " is incomplete: " + err.Error(), next: inv.sameCommand(), nextReason: "retry after correcting the write failure; unmatched provenance is shown as unknown"})
	}
	outcome, summary := intentConfirmed, key+" is set to "+value+" at "+checkout
	if !changed {
		outcome, summary = intentUnchanged, key+" already holds "+value+" at "+checkout+"; nothing was changed"
	}
	state := helm.Active(checkout)
	if state.Active {
		summary += "; it applies when the helm is returned"
	}
	return inv.render(intentResult{Outcome: outcome, Summary: summary, Data: map[string]any{"key": key, "value": value, "checkout": checkout, "set-by": by, "at": at, "file": local}, view: settingSetView(summary)})
}

func settingsDeclarationRemedy(key string) string {
	if key == "settings.apply" {
		return "edit settings.apply in committed metasystem.conf to boundary or now"
	}
	return proofDeclarationRemedy(key)
}

// Checkout authority follows the terminal enrollment's committed template
// declaration. An adopted checkout enrolls at its repository root.
func checkoutAuthorityRoot(layout stateroot.Layout) string {
	installation := layout.InstallationRoot.Path()
	if filepath.Base(installation) == "metasystem" && config.TemplateMode(installation) {
		return installation
	}
	return layout.GitRoot
}
