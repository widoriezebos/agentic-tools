package authority

import "testing"

func cls(class string, holder bool, jobID string) map[string]any {
	m := map[string]any{"class": class, "holder": holder}
	if jobID != "" {
		m["jobId"] = jobID
	}
	return m
}

func TestAuthorize(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		class   map[string]any
		job     string
		allowed bool
	}{
		{"human always", "holder-only", cls("HUMAN", false, ""), "", true},
		{"holder writes", "holder-only", cls("MAIN", true, ""), "", true},
		{"holder cannot standing-reap", "supervision-only", cls("MAIN", true, ""), "", false},
		{"non-holder main refused holder-only", "holder-only", cls("MAIN", false, ""), "", false},
		{"supervision writes record", "record-writer", cls("SUPERVISION", false, ""), "", true},
		{"supervision writes its own", "supervision-only", cls("SUPERVISION", false, ""), "", true},
		{"supervision refused adapter-writer", "adapter-writer", cls("SUPERVISION", false, ""), "", false},
		{"adapter writes its own job", "adapter-writer", cls("ADAPTER-SUPERVISOR", false, "job-7"), "job-7", true},
		{"adapter refused other job", "adapter-writer", cls("ADAPTER-SUPERVISOR", false, "job-7"), "job-9", false},
		{"adapter refused holder-only", "holder-only", cls("ADAPTER-SUPERVISOR", false, "job-7"), "job-7", false},
		{"delegate refused", "record-writer", cls("DELEGATE", false, ""), "", false},
	}
	for _, c := range cases {
		err := Authorize(c.mode, c.class, c.job)
		if (err == nil) != c.allowed {
			t.Errorf("%s: Authorize(%s) allowed=%v, err=%v", c.name, c.mode, err == nil, err)
		}
	}
}
