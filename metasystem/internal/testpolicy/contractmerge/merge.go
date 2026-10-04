// Package contractmerge owns semantic edits to the testing contract.
package contractmerge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const (
	MergeConflictCode   = "TESTING_MERGE_CONFLICT"
	InvalidContractCode = "TESTING_CONTRACT_INVALID"
	AddTestsCode        = "TESTING_ADD_TESTS_REFUSED"
)

// Refusal is a semantic refusal rather than an input/output failure. Entity
// and Field identify the smallest contract location that could not be merged.
// Its text is the two lines a person reads: where and why, then the command
// that resolves it when one does (Run); the code is its detail.
type Refusal struct {
	Code, Entity, Field, Detail string
	Run                         string
}

func (r *Refusal) location() string {
	if r.Field != "" {
		return r.Entity + " field " + r.Field
	}
	return r.Entity
}

// Words are the refusal's line 1: where, and why.
func (r *Refusal) Words() string { return r.location() + ": " + r.Detail }

func (r *Refusal) Error() string {
	text := r.Words()
	switch {
	case strings.HasPrefix(r.Run, nothingToDo):
		text += "\n" + r.Run
	case r.Run != "":
		text += "\nrun: " + r.Run
	}
	return text
}

// withRun sets the refusal's line 2: the command that resolves or shows it,
// or, beginning "nothing to do; ", why no command does.
func (r *Refusal) withRun(command string) *Refusal {
	r.Run = command
	return r
}

// nothingToDo begins a line 2 that names no command.
const nothingToDo = "nothing to do; "

// The line 2 of a merge refusal: git's merge holds the three sides as
// index stages 1 (base), 2 (ours) and 3 (theirs) of the contract.
const (
	contractStagePath = "metasystem/testing.json"
	// mergeConflictRun shows what their side changed, to redo on ours.
	mergeConflictRun = "git diff :1:" + contractStagePath + " :3:" + contractStagePath
)

// The line 2 of an edit refusal: the action's forms, with an example.
const (
	addHelp    = "metasystem test add --help"
	removeHelp = "metasystem test remove --help"
)

// RefusalCode and RefusalDetail are the code and the code-first line
// --verbose and the refusal records keep.
func (r *Refusal) RefusalCode() string   { return r.Code }
func (r *Refusal) RefusalDetail() string { return r.Code + ": " + r.location() + ": " + r.Detail }

func conflict(entity, field, detail string) error {
	return &Refusal{Code: MergeConflictCode, Entity: entity, Field: field, Detail: detail, Run: mergeConflictRun}
}

func invalid(detail string) *Refusal {
	return &Refusal{Code: InvalidContractCode, Entity: "contract", Detail: detail}
}

// Merge performs a three-way merge by surface name, group id, and list value.
// The order from ours is retained; additions found only in theirs follow it.
func Merge(base, ours, theirs testpolicy.Contract) (testpolicy.Contract, error) {
	merged, err := mergeStruct(reflect.ValueOf(base), reflect.ValueOf(ours), reflect.ValueOf(theirs), "contract", map[string]func() (reflect.Value, error){
		"Surfaces": func() (reflect.Value, error) {
			value, err := mergeSurfaces(base.Surfaces, ours.Surfaces, theirs.Surfaces)
			return reflect.ValueOf(value), err
		},
		"Groups": func() (reflect.Value, error) {
			value, err := mergeGroups(base.Groups, ours.Groups, theirs.Groups)
			return reflect.ValueOf(value), err
		},
		"Generated": func() (reflect.Value, error) {
			value, err := mergeGenerated(base.Generated, ours.Generated, theirs.Generated)
			return reflect.ValueOf(value), err
		},
	})
	if err != nil {
		return testpolicy.Contract{}, err
	}
	result := merged.Interface().(testpolicy.Contract)
	recomputeDerived(&result)
	if err := result.Validate(); err != nil {
		return testpolicy.Contract{}, invalid(err.Error())
	}
	rendered, err := Render(result)
	if err != nil {
		return testpolicy.Contract{}, invalid(err.Error())
	}
	loaded, err := testpolicy.Decode(rendered)
	if err != nil {
		return testpolicy.Contract{}, invalid(err.Error())
	}
	return loaded, nil
}

// MergeBytes loads every input with the contract's authoritative loader, then
// validates the rendered merge through that loader once more.
func MergeBytes(base, ours, theirs []byte) ([]byte, error) {
	values := make([]testpolicy.Contract, 3)
	for i, input := range [][]byte{base, ours, theirs} {
		value, err := testpolicy.Decode(input)
		if err != nil {
			return nil, invalid(fmt.Sprintf("the %s side is not a readable testing contract: %v", []string{"base", "our", "their"}[i], err)).
				withRun(fmt.Sprintf("git show :%d:%s", i+1, contractStagePath))
		}
		values[i] = value
	}
	merged, err := Merge(values[0], values[1], values[2])
	if err != nil {
		return nil, err
	}
	return Render(merged)
}

func mergeSurfaces(base, ours, theirs []testpolicy.Surface) ([]testpolicy.Surface, error) {
	return mergeEntities(base, ours, theirs, "surface", true, func(value testpolicy.Surface) string { return value.ID })
}

func mergeGroups(base, ours, theirs []testpolicy.Group) ([]testpolicy.Group, error) {
	return mergeEntities(base, ours, theirs, "group", false, func(value testpolicy.Group) string { return value.ID })
}

// A recipe is atomic: argv order and cwd together describe one command.
func mergeGenerated(base, ours, theirs []testpolicy.Generated) ([]testpolicy.Generated, error) {
	id := func(set testpolicy.Generated) string { return set.Identity() }
	bm, om, tm := entityMap(base, id), entityMap(ours, id), entityMap(theirs, id)
	seen := map[string]bool{}
	var result []testpolicy.Generated
	equal := func(a, b testpolicy.Generated, presentA, presentB bool) bool {
		a.Paths, b.Paths = nil, nil
		return presentA == presentB && reflect.DeepEqual(a, b)
	}
	for _, side := range [][]testpolicy.Generated{ours, theirs} {
		for _, set := range side {
			key := id(set)
			if seen[key] {
				continue
			}
			seen[key] = true
			b, bp := bm[key]
			o, op := om[key]
			t, tp := tm[key]
			switch {
			case equal(o, t, op, tp), equal(b, t, bp, tp):
				if op {
					result = append(result, o)
				}
			case equal(b, o, bp, op):
				if tp {
					result = append(result, t)
				}
			default:
				return nil, conflict("generated "+key, "recipe", "both sides changed the same generated set differently")
			}
		}
	}
	return result, nil
}

func mergeEntities[T any](base, ours, theirs []T, kind string, mergeNew bool, id func(T) string) ([]T, error) {
	baseMap, oursMap, theirsMap := entityMap(base, id), entityMap(ours, id), entityMap(theirs, id)
	order := make([]string, 0, len(ours)+len(theirs))
	seen := map[string]bool{}
	for _, values := range [][]T{ours, theirs} {
		for _, value := range values {
			if key := id(value); !seen[key] {
				seen[key] = true
				order = append(order, key)
			}
		}
	}
	result := make([]T, 0, len(order))
	for _, key := range order {
		b, inBase := baseMap[key]
		o, inOurs := oursMap[key]
		t, inTheirs := theirsMap[key]
		entity := fmt.Sprintf("%s %q", kind, key)
		if !inBase {
			switch {
			case inOurs && inTheirs:
				if mergeNew {
					merged, err := mergeStruct(reflect.Zero(reflect.TypeOf(o)), reflect.ValueOf(o), reflect.ValueOf(t), entity, nil)
					if err != nil {
						return nil, err
					}
					result = append(result, merged.Interface().(T))
					continue
				}
				if !semanticEqual(o, t) {
					return nil, conflict(entity, firstChangedField(reflect.ValueOf(o), reflect.ValueOf(t)), "both sides define the same new identity differently")
				}
				result = append(result, o)
			case inOurs:
				result = append(result, o)
			case inTheirs:
				result = append(result, t)
			}
			continue
		}
		if !inOurs && !inTheirs {
			continue
		}
		if !inOurs {
			if semanticEqual(b, t) {
				continue
			}
			return nil, conflict(entity, firstChangedField(reflect.ValueOf(b), reflect.ValueOf(t)), "ours removed the identity while theirs changed it")
		}
		if !inTheirs {
			if semanticEqual(b, o) {
				continue
			}
			return nil, conflict(entity, firstChangedField(reflect.ValueOf(b), reflect.ValueOf(o)), "theirs removed the identity while ours changed it")
		}
		merged, err := mergeStruct(reflect.ValueOf(b), reflect.ValueOf(o), reflect.ValueOf(t), entity, nil)
		if err != nil {
			return nil, err
		}
		result = append(result, merged.Interface().(T))
	}
	return result, nil
}

func entityMap[T any](values []T, id func(T) string) map[string]T {
	result := make(map[string]T, len(values))
	for _, value := range values {
		result[id(value)] = value
	}
	return result
}

func mergeStruct(base, ours, theirs reflect.Value, entity string, special map[string]func() (reflect.Value, error)) (reflect.Value, error) {
	typ := base.Type()
	result := reflect.New(typ).Elem()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if merge, ok := special[field.Name]; ok {
			value, err := merge()
			if err != nil {
				return reflect.Value{}, err
			}
			result.Field(i).Set(value)
			continue
		}
		name := jsonFieldName(field)
		value, err := mergeValue(base.Field(i), ours.Field(i), theirs.Field(i), entity, name)
		if err != nil {
			return reflect.Value{}, err
		}
		result.Field(i).Set(value)
	}
	return result, nil
}

func mergeValue(base, ours, theirs reflect.Value, entity, field string) (reflect.Value, error) {
	if base.Type() == reflect.TypeOf(json.RawMessage{}) {
		return mergeRawJSON(base, ours, theirs, entity, field)
	}
	if base.Kind() == reflect.Struct {
		return mergeStruct(base, ours, theirs, entity, nil)
	}
	if base.Kind() == reflect.Ptr && base.Type().Elem().Kind() == reflect.Struct && !ours.IsNil() && !theirs.IsNil() {
		baseValue := base
		if base.IsNil() {
			baseValue = reflect.New(base.Type().Elem())
		}
		value, err := mergeStruct(baseValue.Elem(), ours.Elem(), theirs.Elem(), entity, nil)
		if err != nil {
			return reflect.Value{}, err
		}
		pointer := reflect.New(base.Type().Elem())
		pointer.Elem().Set(value)
		return pointer, nil
	}
	if base.Kind() == reflect.Slice {
		return mergeSet(base, ours, theirs), nil
	}
	if reflect.DeepEqual(ours.Interface(), theirs.Interface()) {
		return ours, nil
	}
	if reflect.DeepEqual(ours.Interface(), base.Interface()) {
		return theirs, nil
	}
	if reflect.DeepEqual(theirs.Interface(), base.Interface()) {
		return ours, nil
	}
	return reflect.Value{}, conflict(entity, field, "both sides changed the same scalar differently")
}

func mergeSet(base, ours, theirs reflect.Value) reflect.Value {
	if reflect.DeepEqual(base.Interface(), ours.Interface()) && reflect.DeepEqual(base.Interface(), theirs.Interface()) {
		return ours
	}
	baseKeys, theirsKeys := sliceKeys(base), sliceKeys(theirs)
	result := reflect.MakeSlice(ours.Type(), 0, ours.Len()+theirs.Len())
	seen := map[string]bool{}
	for i := 0; i < ours.Len(); i++ {
		key := valueKey(ours.Index(i).Interface())
		if baseKeys[key] && !theirsKeys[key] {
			continue
		}
		if !seen[key] {
			result = reflect.Append(result, ours.Index(i))
			seen[key] = true
		}
	}
	for i := 0; i < theirs.Len(); i++ {
		key := valueKey(theirs.Index(i).Interface())
		if baseKeys[key] || seen[key] {
			continue
		}
		result = reflect.Append(result, theirs.Index(i))
		seen[key] = true
	}
	return result
}

func sliceKeys(value reflect.Value) map[string]bool {
	result := make(map[string]bool, value.Len())
	for i := 0; i < value.Len(); i++ {
		result[valueKey(value.Index(i).Interface())] = true
	}
	return result
}

func valueKey(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func mergeRawJSON(base, ours, theirs reflect.Value, entity, field string) (reflect.Value, error) {
	b, ba := rawArray(base.Bytes())
	o, oa := rawArray(ours.Bytes())
	t, ta := rawArray(theirs.Bytes())
	if ba && oa && ta {
		merged := mergeSet(reflect.ValueOf(b), reflect.ValueOf(o), reflect.ValueOf(t))
		data, _ := json.Marshal(merged.Interface())
		return reflect.ValueOf(json.RawMessage(data)), nil
	}
	return mergeScalarSemantic(base, ours, theirs, entity, field)
}

func rawArray(data []byte) ([]any, bool) {
	if len(data) == 0 {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	array, ok := value.([]any)
	return array, ok
}

func mergeScalarSemantic(base, ours, theirs reflect.Value, entity, field string) (reflect.Value, error) {
	if semanticEqual(ours.Interface(), theirs.Interface()) {
		return ours, nil
	}
	if semanticEqual(ours.Interface(), base.Interface()) {
		return theirs, nil
	}
	if semanticEqual(theirs.Interface(), base.Interface()) {
		return ours, nil
	}
	return reflect.Value{}, conflict(entity, field, "both sides changed the same scalar differently")
}

func semanticEqual(left, right any) bool {
	return valueKey(semanticValue(reflect.ValueOf(left))) == valueKey(semanticValue(reflect.ValueOf(right)))
}

func semanticValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	if value.Type() == reflect.TypeOf(json.RawMessage{}) {
		decoded, ok := rawJSONValue(value.Bytes())
		if !ok {
			return string(value.Bytes())
		}
		return semanticValue(reflect.ValueOf(decoded))
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		result := make(map[string]any, value.NumField())
		for i := 0; i < value.NumField(); i++ {
			result[value.Type().Field(i).Name] = semanticValue(value.Field(i))
		}
		return result
	case reflect.Slice:
		keys := map[string]bool{}
		for i := 0; i < value.Len(); i++ {
			keys[valueKey(semanticValue(value.Index(i)))] = true
		}
		result := make([]string, 0, len(keys))
		for key := range keys {
			result = append(result, key)
		}
		sort.Strings(result)
		return result
	case reflect.Map:
		result := map[string]any{}
		for _, key := range value.MapKeys() {
			result[fmt.Sprint(key.Interface())] = semanticValue(value.MapIndex(key))
		}
		return result
	default:
		return value.Interface()
	}
}

func rawJSONValue(data []byte) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	return value, true
}

func firstChangedField(left, right reflect.Value) string {
	if left.Kind() != reflect.Struct || right.Kind() != reflect.Struct {
		return "definition"
	}
	for i := 0; i < left.NumField(); i++ {
		if !semanticEqual(left.Field(i).Interface(), right.Field(i).Interface()) {
			return jsonFieldName(left.Type().Field(i))
		}
	}
	return "definition"
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "-" {
		return strings.ToLower(strings.TrimSuffix(field.Name, "Set"))
	}
	if name == "" {
		return field.Name
	}
	return name
}

func recomputeDerived(contract *testpolicy.Contract) {
	if contract.Fallback == "" {
		return
	}
	fallback := -1
	for i := range contract.Surfaces {
		if contract.Surfaces[i].ID == contract.Fallback {
			fallback = i
			break
		}
	}
	if fallback < 0 {
		return
	}
	standard := contract.Surfaces[fallback].Standard[:0:0]
	deep := contract.Surfaces[fallback].Deep[:0:0]
	critical := contract.Surfaces[fallback].Critical[:0:0]
	for _, surface := range contract.Surfaces {
		if surface.ID == contract.Fallback {
			continue
		}
		if surface.ID == "context-budget" {
			standard = appendUnique(standard, contract.Surfaces[fallback].Standard...)
			deep = appendUnique(deep, contract.Surfaces[fallback].Deep...)
			critical = appendUnique(critical, contract.Surfaces[fallback].Critical...)
		} else {
			standard = appendUnique(standard, surface.Standard...)
			deep = appendUnique(deep, surface.Deep...)
			critical = appendUnique(critical, surface.Critical...)
		}
	}
	contract.Surfaces[fallback].Standard = standard
	contract.Surfaces[fallback].Deep = deep
	contract.Surfaces[fallback].Critical = critical
	for i := range contract.Surfaces {
		if contract.Surfaces[i].ID == "context-budget" {
			contract.Surfaces[i].Standard = append(standard[:0:0], standard...)
			contract.Surfaces[i].Deep = append(deep[:0:0], deep...)
			contract.Surfaces[i].Critical = append(critical[:0:0], critical...)
		}
	}
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}
