package plain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResultClassificationPolicyJSON(t *testing.T) {
	t.Parallel()
	for _, policy := range []PolicyValue{{}, {Value: "auto", Source: "default", At: time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)}} {
		name := "absent"
		if policy.Value != "" {
			name = "present"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result := Result{Requested: "head", Result: "none", ClassificationPolicy: policy}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if _, present := fields["classification-policy"]; present != (policy != (PolicyValue{})) {
				t.Fatalf("classification policy presence: %s", data)
			}
			var decoded Result
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.ClassificationPolicy != policy || decoded.Requested != result.Requested || decoded.Result != result.Result {
				t.Fatalf("result changed in JSON: %+v", decoded)
			}
		})
	}
}
