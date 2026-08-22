package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
)

// ReturnComplete validates a canonical agent return against the shipped
// schema for its role. In job mode it reads the job record, walks the
// parentJob chain to the round return, and also checks jobId, round, runtime,
// and sessionId identity between the return and the record. It returns the
// violations found (empty means pass).

var returnJobIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var returnAllowedRoles = map[string]bool{
	"orchestrator": true, "design-critic": true, "implementer": true,
	"code-critic": true, "verifier": true, "investigator": true, "behavior-judge": true,
	"steward-continuation": true,
}

var returnVersionedRoles = map[string]bool{
	"design-critic": true, "implementer": true, "code-critic": true,
	"verifier": true, "investigator": true, "behavior-judge": true,
	"steward-continuation": true,
}

type returnChecker struct {
	root       string
	violations []string
}

func (c *returnChecker) violation(format string, args ...any) {
	c.violations = append(c.violations, fmt.Sprintf(format, args...))
}

func (c *returnChecker) loadJSON(path, label string) any {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c.violation("%s does not exist: %s", label, path)
		return nil
	}
	if err != nil {
		c.violation("%s could not be read: %s: %v", label, path, err)
		return nil
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		c.violation("%s is not valid JSON: %v", label, err)
		return nil
	}
	return value
}

// ReturnCompleteRole validates a return file directly against a role schema.
func ReturnCompleteRole(root, role, file string) []string {
	c := &returnChecker{root: root}
	c.checkReturn(role, file, nil, "")
	return c.violations
}

// ReturnCompleteJobFile validates an EXPLICIT candidate file with the
// full job flow — record, chain walk, schema, and identity — instead of
// the round's conventional return path. The delivery collector's
// per-candidate selection needs exactly this: schema-only
// validation cannot catch a schema-valid return for the wrong job.
func ReturnCompleteJobFile(root, job, file string) []string {
	return returnCompleteJobAt(root, job, file)
}

// ReturnCompleteJob validates a job's round return: the record, the chain
// walk to the root job, the schema, and the identity fields.
func ReturnCompleteJob(root, job string) []string {
	return returnCompleteJobAt(root, job, "")
}

// returnCompleteJobAt is the shared job flow; an empty overridePath means
// the round's conventional return location.
func returnCompleteJobAt(root, job, overridePath string) []string {
	c := &returnChecker{root: root}
	recordPath := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
	raw := c.loadJSON(recordPath, "job record")
	if raw == nil {
		return c.violations
	}
	record, ok := raw.(map[string]any)
	if !ok {
		c.violation("job record must be a JSON object")
		return c.violations
	}
	if id, _ := record["jobId"].(string); id != job {
		c.violation("job record jobId must equal requested job id %q", job)
	}

	rootJobID := job
	seen := map[string]bool{job: true}
	current := record
	for current != nil && current["parentJob"] != nil {
		parent, isStr := current["parentJob"].(string)
		if !isStr || parent == "" {
			c.violation("job record parentJob must be a non-empty string or null")
			break
		}
		if !returnJobIDRe.MatchString(parent) {
			c.violation("job record parentJob is not a valid job id: %q", parent)
			break
		}
		if seen[parent] {
			c.violation("job record parentJob chain contains a cycle at %q", parent)
			break
		}
		seen[parent] = true
		parentRaw := c.loadJSON(filepath.Join(root, "artifacts", "agents", "jobs", parent+".json"),
			fmt.Sprintf("parent job record %q", parent))
		if parentRaw == nil {
			break
		}
		parentRecord, isObj := parentRaw.(map[string]any)
		if !isObj {
			c.violation("parent job record %q must be a JSON object", parent)
			break
		}
		if id, _ := parentRecord["jobId"].(string); id != parent {
			c.violation("parent job record %q has a different jobId", parent)
			break
		}
		rootJobID = parent
		current = parentRecord
	}

	role, _ := record["role"].(string)
	round, roundOK := jsonInteger(record["round"])
	var returnPath string
	if !roundOK {
		c.violation("job record round must be an integer")
	} else if overridePath != "" {
		returnPath = overridePath
	} else {
		returnPath = filepath.Join(root, "artifacts", "agents", rootJobID,
			"rounds", strconv.FormatInt(round, 10), "return.json")
	}
	if !returnAllowedRoles[role] {
		c.violation("job record role is not dispatchable: %q", role)
	}
	c.checkReturn(role, returnPath, record, "job")
	return c.violations
}

// checkReturn loads the return and schema, applies versioning, validates, and
// runs the role-specific checks; record is non-nil in job mode.
func (c *returnChecker) checkReturn(role, returnPath string, record map[string]any, mode string) {
	if mode != "job" && !returnAllowedRoles[role] {
		c.violation("unknown role: %q", role)
	}
	var result any
	if returnPath != "" {
		result = c.loadJSON(returnPath, "return file")
	}
	var schema map[string]any
	if returnAllowedRoles[role] {
		raw := c.loadJSON(filepath.Join(c.root, "scripts", "agents", "schemas", role+".schema.json"), "role schema")
		schema, _ = raw.(map[string]any)
	}

	if resultObj, isObj := result.(map[string]any); isObj {
		if version, present := resultObj["schemaVersion"]; present {
			v, vOK := jsonInteger(version)
			if !returnVersionedRoles[role] || !vOK || v != 2 {
				c.violation("unknown return schema version for role %q: %v", role, version)
			} else if schema != nil {
				upgraded, err := returnschema.VersionTwo(schema)
				if err != nil {
					c.violation("role schema cannot version: %v", err)
				} else {
					schema = upgraded
				}
			}
		}
	}

	if result == nil || schema == nil {
		return
	}
	before := len(c.violations)
	c.validateSchemaShape(schema, "$schema")
	if len(c.violations) == before {
		c.validateValue(result, schema, "$")
	}

	resultObj, _ := result.(map[string]any)
	if (role == "design-critic" || role == "code-critic") && resultObj != nil {
		c.checkMaterialCount(resultObj)
	}
	if role == "behavior-judge" && resultObj != nil {
		c.checkJudgeAnchors(resultObj)
	}
	if mode == "job" && resultObj != nil && record != nil {
		for _, name := range []string{"jobId", "round", "runtime", "sessionId"} {
			if !jsonSame(resultObj[name], record[name]) {
				c.violation("$.%s identity mismatch: return has %s, job record has %s",
					name, jsonRepr(resultObj[name]), jsonRepr(record[name]))
			}
		}
	}
}

var supportedSchemaKeywords = map[string]bool{
	"$schema": true, "$comment": true, "title": true, "description": true,
	"type": true, "enum": true, "const": true, "properties": true,
	"required": true, "additionalProperties": true, "items": true,
}

func (c *returnChecker) validateSchemaShape(schema any, path string) {
	obj, ok := schema.(map[string]any)
	if !ok {
		c.violation("%s must be a JSON object", path)
		return
	}
	for keyword := range obj {
		if !supportedSchemaKeywords[keyword] {
			c.violation("%s uses unsupported schema keyword %q", path, keyword)
		}
	}
	if properties, present := obj["properties"]; present {
		propObj, isObj := properties.(map[string]any)
		if !isObj {
			c.violation("%s.properties must be an object", path)
		} else {
			for name, child := range propObj {
				c.validateSchemaShape(child, path+".properties."+name)
			}
		}
	}
	if items, present := obj["items"]; present {
		c.validateSchemaShape(items, path+".items")
	}
	if required, present := obj["required"]; present {
		list, isList := required.([]any)
		valid := isList
		if isList {
			for _, name := range list {
				if _, isStr := name.(string); !isStr {
					valid = false
				}
			}
		}
		if !valid {
			c.violation("%s.required must be an array of strings", path)
		}
	}
	if ap, present := obj["additionalProperties"]; present {
		if _, isBool := ap.(bool); !isBool {
			c.violation("%s.additionalProperties must be boolean", path)
		}
	}
	if enum, present := obj["enum"]; present {
		if _, isList := enum.([]any); !isList {
			c.violation("%s.enum must be an array", path)
		}
	}
}

func (c *returnChecker) validateValue(value any, schema map[string]any, path string) {
	if expected, present := schema["type"]; present {
		choices := []any{expected}
		if list, isList := expected.([]any); isList {
			choices = list
		}
		matched := false
		for _, choice := range choices {
			if name, _ := choice.(string); typeMatches(value, name) {
				matched = true
				break
			}
		}
		if !matched {
			c.violation("%s must be %s", path, describeTypes(expected))
			return
		}
	}
	if enum, present := schema["enum"]; present {
		list, _ := enum.([]any)
		found := false
		for _, item := range list {
			if jsonSame(value, item) {
				found = true
				break
			}
		}
		if !found {
			allowed := ""
			for i, item := range list {
				if i > 0 {
					allowed += ", "
				}
				allowed += jsonRepr(item)
			}
			c.violation("%s must be one of: %s", path, allowed)
		}
	}
	if constant, present := schema["const"]; present && !jsonSame(value, constant) {
		c.violation("%s must equal %s", path, jsonRepr(constant))
	}

	if obj, isObj := value.(map[string]any); isObj {
		properties, _ := schema["properties"].(map[string]any)
		if required, _ := schema["required"].([]any); required != nil {
			for _, name := range required {
				nameStr, _ := name.(string)
				if _, present := obj[nameStr]; !present {
					c.violation("%s.%s is required", path, nameStr)
				}
			}
		}
		if ap, present := schema["additionalProperties"]; present {
			if allowed, isBool := ap.(bool); isBool && !allowed {
				for name := range obj {
					if _, known := properties[name]; !known {
						c.violation("%s.%s is not allowed by this role schema", path, name)
					}
				}
			}
		}
		for name, child := range obj {
			if childSchema, known := properties[name].(map[string]any); known {
				c.validateValue(child, childSchema, path+"."+name)
			}
		}
	}
	if list, isList := value.([]any); isList {
		if items, present := schema["items"].(map[string]any); present {
			for index, item := range list {
				c.validateValue(item, items, fmt.Sprintf("%s[%d]", path, index))
			}
		}
	}
}

func (c *returnChecker) checkMaterialCount(result map[string]any) {
	findings, findingsOK := result["findings"].([]any)
	verdict, verdictOK := jsonInteger(result["verdictMaterialCount"])
	if !findingsOK || !verdictOK {
		return
	}
	materialCount := int64(0)
	for _, raw := range findings {
		finding, isObj := raw.(map[string]any)
		if isObj && finding["material"] == true {
			materialCount++
		}
	}
	if verdict != materialCount {
		c.violation("$.verdictMaterialCount must equal the count of findings with material=true (expected %d, got %d)",
			materialCount, verdict)
	}
}

func (c *returnChecker) checkJudgeAnchors(result map[string]any) {
	requireAnchors := func(anchors any, path string) {
		list, isList := anchors.([]any)
		if !isList || len(list) == 0 {
			c.violation("%s must contain at least one file-and-line anchor", path)
			return
		}
		for index, raw := range list {
			anchor, isObj := raw.(map[string]any)
			if !isObj {
				continue
			}
			if file, _ := anchor["file"].(string); file == "" {
				c.violation("%s[%d].file must be a non-empty string", path, index)
			}
			if line, ok := jsonInteger(anchor["line"]); !ok || line < 1 {
				c.violation("%s[%d].line must be a positive one-based line number", path, index)
			}
		}
	}

	if dimensions, isList := result["dimensions"].([]any); isList {
		ids := []string{}
		unique := map[string]bool{}
		for _, raw := range dimensions {
			dimension, isObj := raw.(map[string]any)
			if !isObj {
				continue
			}
			id, _ := dimension["id"].(string)
			ids = append(ids, id)
			unique[id] = true
		}
		if len(ids) == 0 || len(ids) != len(unique) {
			c.violation("$.dimensions must contain at least one requested judged dimension with no duplicate ids")
		}
		for di, raw := range dimensions {
			dimension, isObj := raw.(map[string]any)
			if !isObj {
				continue
			}
			requireAnchors(dimension["anchors"], fmt.Sprintf("$.dimensions[%d].anchors", di))
			if findings, isList := dimension["findings"].([]any); isList {
				for fi, findingRaw := range findings {
					if finding, isObj := findingRaw.(map[string]any); isObj {
						requireAnchors(finding["anchors"], fmt.Sprintf("$.dimensions[%d].findings[%d].anchors", di, fi))
					}
				}
			}
		}
	}
	if watches, isList := result["reliabilityWatch"].([]any); isList {
		for wi, raw := range watches {
			if watch, isObj := raw.(map[string]any); isObj {
				requireAnchors(watch["anchors"], fmt.Sprintf("$.reliabilityWatch[%d].anchors", wi))
			}
		}
	}
}

func typeMatches(value any, expected string) bool {
	switch expected {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		_, ok := jsonInteger(value)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	}
	return false
}

func describeTypes(expected any) string {
	if list, isList := expected.([]any); isList {
		out := ""
		for i, item := range list {
			if i > 0 {
				out += " or "
			}
			out += fmt.Sprint(item)
		}
		return out
	}
	return fmt.Sprint(expected)
}

func jsonInteger(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}

func jsonSame(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// jsonRepr renders the shared dialect core with quoted strings; everything
// outside the core (bools, non-integral floats, composites) renders as raw
// JSON bytes — this gate's deliberate difference from the conformance one.
func jsonRepr(v any) string {
	return reprValue(v, true, func(rest any) string {
		data, _ := json.Marshal(rest)
		return string(data)
	})
}
