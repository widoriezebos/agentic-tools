// Package designgate evaluates the design evidence handed in by a caller.
package designgate

import (
	"fmt"
	"strings"
)

type Chain struct {
	Closed bool
	Round  int64
}

type Design struct {
	ID       string  `json:"id"`
	Path     string  `json:"path"`
	SHA256   string  `json:"sha256"`
	Critique string  `json:"critique"`
	Name     string  `json:"-"`
	Status   string  `json:"-"`
	Chains   []Chain `json:"-"`
}

type Facts struct {
	Goal    string
	Tier    uint8
	Mode    string
	Allowed bool
	Designs []Design
	Error   error
}

type Result struct {
	Verdict     string    `json:"verdict"`
	WouldRefuse bool      `json:"wouldRefuse"`
	Mode        string    `json:"mode"`
	Warning     [2]string `json:"-"`
}

// Check reads no files or local state. A ruled critique stands on the
// person's acceptance; only a closed critique can be contradicted by a chain.
func Check(f Facts) Result {
	r := Result{Verdict: "not-design-bearing", Mode: "warn"}
	if f.Mode == "refuse" {
		r.Mode = f.Mode
	}
	if f.Tier == 1 {
		return r
	}
	if f.Allowed {
		r.Verdict = "allowed"
		return r
	}
	if f.Error != nil {
		r.Verdict = "unchecked"
		r.Warning = [2]string{fmt.Sprintf("warning: the design check could not run (%s); this build was not checked", f.Error), "metasystem design list --goal " + f.Goal}
		return r
	}
	var accepted []Design
	var draft string
	for _, d := range f.Designs {
		if d.Status == "accepted" {
			accepted = append(accepted, d)
		} else if d.Status == "draft" && draft == "" {
			draft = d.Path
		}
	}
	r.Verdict, r.WouldRefuse = "no-accepted-design", true
	r.Warning = [2]string{fmt.Sprintf("warning: goal %s has no accepted design; this build runs on its brief alone", f.Goal), "metasystem design write " + f.Goal + " --brief FILE"}
	if len(accepted) == 0 {
		if draft != "" {
			r.Warning[1] = "metasystem design review " + draft
		}
		return r
	}
	for _, d := range accepted {
		words := strings.Fields(d.Critique)
		if len(words) == 0 || words[0] != "closed" && words[0] != "ruled" {
			r.Verdict = "critique-not-recorded"
			r.Warning = [2]string{fmt.Sprintf("warning: the accepted design %s does not say its critique closed; the build goes on", d.Name), fmt.Sprintf("edit %s: add \"- Critique: closed at round N on 0 material findings (WHO)\" under its Goals line", d.Path)}
			return r
		}
	}
	for _, d := range accepted {
		if strings.Fields(d.Critique)[0] != "closed" {
			continue
		}
		for _, c := range d.Chains {
			if !c.Closed {
				r.Verdict = "critique-open"
				r.Warning = [2]string{fmt.Sprintf("warning: the accepted design %s says its critique closed, but its review here is open at round %d", d.Name, c.Round), fmt.Sprintf("metasystem design review %s --dispositions FILE --after %d", d.Path, c.Round)}
				return r
			}
		}
	}
	r.Verdict, r.WouldRefuse, r.Warning = "ok", false, [2]string{}
	return r
}
