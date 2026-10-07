package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
)

// helmPolicyKeys includes seat policies everywhere and the declared owner's
// policies at that owner. Recorded keys keep an existing helm's scope visible
// when an advisory registry can no longer be read.
func (inv *intentInvocation) helmPolicyKeys(checkout string, state helm.State) []string {
	keys := map[string]bool{"seat.driver": true, "review.stop": true, "goal.raise": true}
	registry, _ := inv.policyReaders().Registry(checkout)
	same := func(path string) bool {
		if path == "" {
			return false
		}
		seat, err := helm.Locate(path)
		return path != "" && err == nil && seat.Checkout == checkout
	}
	if same(registry.Lane) {
		for _, key := range []string{"landing.batch", "landing.proof", "landing.on-red", "landing.trunk-red"} {
			keys[key] = true
		}
	}
	if same(registry.Coordinator) {
		keys["question.route"] = true
	}
	for key := range state.Policies {
		if config.PolicyScope(key) != "" && key != "settings.apply" {
			keys[key] = true
		}
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func (inv *intentInvocation) helmPolicyParams(checkout, key string) (config.GetParams, error) {
	readers := inv.policyReaders()
	conf, err := readers.ConfPath(checkout)
	calling := ""
	if seat, err := helm.Locate(inv.cwd); err == nil {
		calling = seat.Checkout
	}
	return config.GetParams{Key: key, ConfPath: conf, LookupEnv: inv.owners.lookupEnv, Policy: &config.PolicyContext{Checkout: checkout, CallingCheckout: calling, Readers: readers}}, err
}

func (inv *intentInvocation) helmPolicySnapshot(checkout, by string, now time.Time) map[string]helm.Policy {
	policies := map[string]helm.Policy{}
	for _, key := range inv.helmPolicyKeys(checkout, helm.State{}) {
		previous := config.PolicyValue{Checkout: checkout, Value: "unknown", Source: "unreadable", SetBy: "author unknown"}
		params, err := inv.helmPolicyParams(checkout, key)
		if err == nil {
			// Taking a corrupt signature must not read it as previous configuration.
			params.Policy.Readers.Helm = func(string) helm.State { return helm.State{} }
			resolved, readErr := config.ResolvePolicy(params)
			err = readErr
			if err == nil {
				previous = resolved.PolicyValue
			}
		}
		if err != nil {
			previous.Source = "unreadable: " + err.Error()
		}
		policies[key] = helm.Policy{Name: key, Checkout: checkout, Value: "person", SetBy: by, At: now, Previous: previous}
	}
	return policies
}

func (inv *intentInvocation) helmPolicyStatus(checkout string, state helm.State, lines []string) ([]config.PolicyResolution, []string) {
	var policies []config.PolicyResolution
	for _, key := range inv.helmPolicyKeys(checkout, state) {
		params, err := inv.helmPolicyParams(checkout, key)
		var policy config.PolicyResolution
		if err == nil {
			policy, err = config.ResolvePolicy(params)
		}
		if err != nil {
			policy = config.PolicyResolution{Name: key, PolicyValue: config.PolicyValue{Checkout: checkout, Value: "person", Source: "helm", SetBy: state.By, At: state.Since}, HelmHolder: state.By,
				Previous: &config.PolicyValue{Checkout: checkout, Value: "unknown", Source: "unreadable: " + err.Error(), SetBy: "author unknown"}}
		}
		policies = append(policies, policy)
		line := key + " = " + policy.Value + " (helm held by " + state.By + ")"
		if policy.Previous != nil {
			line += "; underlying " + policy.Previous.Value + " from " + policy.Previous.Source + ", set by " + policy.Previous.SetBy
		}
		lines = append(lines, line)
	}
	return policies, lines
}

type helmHostResult struct {
	Checkout string       `json:"checkout"`
	Exit     int          `json:"exit"`
	Result   intentResult `json:"result"`
	Recovery []string     `json:"recovery,omitempty"`
}

// runHelmAll shares the machine verbs' host discovery without their machine
// filter. All local registered checkouts belong to the act, even when stopped.
// A failure leaves successful targets in place and names the direct retry.
func (inv *intentInvocation) runHelmAll(act string) int {
	var actor *helmActor
	if act != "status" {
		var problem *intentResult
		actor, problem = inv.helmActorAt(act)
		if problem != nil {
			return inv.render(*problem)
		}
	}
	if seat, err := helm.Locate(inv.helmPath()); err == nil {
		inv.layout.GitRoot = seat.Checkout
	}
	candidates, reading := inv.discoverHostCheckouts()
	problems := []string{}
	if reading.RegistryProblem != "" {
		problems = append(problems, reading.RegistryProblem)
	}
	if reading.LaneProblem != "" {
		problems = append(problems, reading.LaneProblem)
	}
	// Coordinator pointers are local host records. Read each checkout's ledger
	// identity; fleet presence from other computers is deliberately not used.
	registryChecked := map[string]bool{}
	for index := 0; index < len(candidates); index++ {
		path := candidates[index].path
		seat, err := helm.Locate(path)
		if err != nil || registryChecked[seat.CommonDir] {
			continue
		}
		registryChecked[seat.CommonDir] = true
		owners, err := inv.policyReaders().Registry(seat.Checkout)
		if err != nil {
			var problem *config.PolicyReadError
			if errors.As(err, &problem) && problem.Checkout != "" {
				candidates = append(candidates, hostCheckout{problem.Checkout, "unreadable declaration"})
			}
			problems = append(problems, err.Error())
		}
		if owners.Lane != "" {
			candidates = append(candidates, hostCheckout{owners.Lane, "landing lane"})
		}
		if owners.Coordinator != "" {
			candidates = append(candidates, hostCheckout{owners.Coordinator, "coordinator"})
		}
	}
	var results []helmHostResult
	var recovery []string
	var lines []string
	seen := map[string]bool{}
	code := 0
	command := func(path string) string {
		argv := []string{"metasystem", "helm", act}
		if act == "take" {
			argv = append(argv, "--reason", inv.input.text("reason"))
		}
		return shellCommand(append(argv, "--repo", path))
	}
	for _, target := range candidates {
		path, key := target.path, filepath.Clean(target.path)
		if seat, err := helm.Locate(path); err == nil {
			path, key = seat.Checkout, seat.CommonDir
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		child := *inv
		child.input = intentInput{values: map[string][]string{"repo": {path}, "json": {"true"}}}
		for _, flag := range []string{"reason", "name"} {
			if inv.input.has(flag) {
				child.input.values[flag] = inv.input.values[flag]
			}
		}
		child.owners.helm.actor = actor
		var stdout, stderr bytes.Buffer
		child.stdout, child.stderr = &stdout, &stderr
		child.owners.helm.stdinTerminal = func() bool { return false }
		var exit int
		switch act {
		case "take":
			exit = runIntentHelmTake(&child)
		case "return":
			exit = runIntentHelmReturn(&child)
		default:
			exit = runIntentHelmStatus(&child)
		}
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			exit = 1
			result = intentResult{Outcome: intentFailed, Summary: "the checkout result could not be read: " + err.Error()}
		}
		row := helmHostResult{Checkout: path, Exit: exit, Result: result}
		if act == "status" && helm.Active(path).Malformed != "" {
			row.Exit = 1
			row.Result.Outcome = intentFailed
			exit = 1
		}
		if exit != 0 {
			row.Recovery = []string{command(path)}
			recovery = append(recovery, command(path))
		}
		results = append(results, row)
		lines = append(lines, path+": "+result.Summary)
		if exit != 0 {
			code = 1
			lines = append(lines, "at the person's enrolled terminal: "+command(path))
		}
		if act == "status" {
			if data, ok := result.Data.(map[string]any); ok {
				if policies, ok := data["policies"]; ok {
					raw, _ := json.Marshal(policies)
					var values []config.PolicyResolution
					if json.Unmarshal(raw, &values) == nil {
						for _, policy := range values {
							line := policy.Name + " = " + policy.Value + " (helm held by " + policy.HelmHolder + ")"
							if policy.Previous != nil {
								line += "; underlying " + policy.Previous.Value + " from " + policy.Previous.Source + ", set by " + policy.Previous.SetBy
							}
							lines = append(lines, line)
						}
					}
				}
			}
		}
	}
	if len(problems) > 0 {
		code = 1
		recovery = append(recovery, command("PATH"))
		for _, problem := range problems {
			lines = append(lines, problem+"; for each missing local checkout, run: "+command("PATH"))
		}
	}
	outcome, summary := intentConfirmed, fmt.Sprintf("helm %s reported %d local checkouts", act, len(results))
	if code != 0 {
		outcome, summary = intentFailed, fmt.Sprintf("helm %s is partial; %d local checkouts were reported", act, len(results))
	}
	return inv.render(intentResult{Outcome: outcome, code: code, Summary: summary, text: lines, Data: map[string]any{"checkouts": results, "problems": problems, "recovery": recovery}})
}
