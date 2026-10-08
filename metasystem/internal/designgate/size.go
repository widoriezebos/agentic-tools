package designgate

import "fmt"

type SizeResult struct {
	Verdict     string    `json:"verdict"`
	WouldRefuse bool      `json:"wouldRefuse"`
	Count       int       `json:"count"`
	Limits      [2]int64  `json:"limits"`
	Declaration string    `json:"declaration"`
	Designs     []Design  `json:"designs"`
	Warning     [2]string `json:"warning"`
}

func CheckDesignSize(f Facts) *SizeResult {
	// Repeated tables may add test allowances to the same production estimate.
	type unitEstimate struct {
		lines, production int64
		known             bool
	}
	r := &SizeResult{Verdict: "ok", Limits: f.Limits, Declaration: f.Declaration, Designs: f.Designs}
	designs, units := map[string]bool{}, map[string]bool{}
	for _, d := range f.Designs {
		if d.SizeExempt || d.Status != "accepted" && d.Status != "draft" || designs[d.ID] {
			continue
		}
		designs[d.ID] = true
		estimates := map[string]unitEstimate{}
		for _, u := range d.Units {
			estimate := unitEstimate{lines: u.Lines}
			if u.Production != nil {
				estimate = unitEstimate{production: *u.Production, known: true}
			}
			if prior, ok := estimates[u.Name]; ok && prior == estimate {
				continue
			}
			estimates[u.Name] = estimate
			r.Count++
			problem := ""
			switch {
			case u.Name == "":
				problem = "has no unit identity"
			case units[u.Name]:
				problem = "has a duplicate unit identity"
			case u.Production == nil:
				problem = "has no unambiguous nonnegative production estimate"
			case *u.Production > f.Limits[0]:
				problem = fmt.Sprintf("has %d production lines, exceeding %d", *u.Production, f.Limits[0])
			}
			units[u.Name] = true
			if problem != "" && r.Verdict == "ok" {
				r.Verdict, r.WouldRefuse = "design-size-invalid", true
				r.Warning = [2]string{fmt.Sprintf("warning: unit %s in %s %s; row: %s", u.Name, d.Path, problem, u.Row), fmt.Sprintf("retain a draft of %s; add or repair its Production lines column, divide oversized units, then obtain normal human acceptance and retry", d.Path)}
			}
		}
		if len(d.Units) == 0 && !d.SizeExempt && r.Verdict == "ok" {
			r.Verdict, r.WouldRefuse = "design-size-invalid", true
			r.Warning = [2]string{"warning: " + d.Path + " has no sized Units table", "retain a draft of " + d.Path + "; add Unit, Lines and Production lines columns, obtain normal human acceptance and retry"}
		}
	}
	if int64(r.Count) > f.Limits[1] {
		r.Verdict, r.WouldRefuse = "design-goal-too-large", true
		r.Warning = [2]string{fmt.Sprintf("warning: goal %s has %d units across its designs, exceeding %d", f.Goal, r.Count, f.Limits[1]), fmt.Sprintf("preserve split-off requirements in a draft of %s; a person runs metasystem goal open NEW-GOAL --intent 'Preserve split-off requirements from %s' --next 'Review the preserved draft' --risk ANSWERS --basis TEXT; assign that draft's preserved work to the new goal, obtain normal human acceptance and retry", f.Designs[0].Path, f.Goal)}
	}
	return r
}
