package proofrun

import (
	"fmt"
	"reflect"
	"strings"
)

// attemptFill fills a record type the way a writer could: every exported
// field allocated (a pointer set, a slice and a map given one entry), and
// every string field whose name or JSON tag says attempt or reuse, and
// every string in a slice or map such a field holds, given a fresh attempt
// id (a struct's fields are judged by their own names). The ids it
// hands out are what a reader of that record kind must name.
type attemptFill struct {
	ids   []string
	where []string
	skip  map[string]bool
}

func (f *attemptFill) fill(value reflect.Value, path string, marked bool, depth int) {
	if depth > 10 {
		return
	}
	switch value.Kind() {
	case reflect.String:
		if marked && value.CanSet() {
			id := fmt.Sprintf("proof-f-%016x", len(f.ids)+1)
			f.ids, f.where = append(f.ids, id), append(f.where, path)
			value.SetString(id)
		}
	case reflect.Ptr:
		if value.IsNil() && value.CanSet() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		if !value.IsNil() {
			f.fill(value.Elem(), path, marked, depth+1)
		}
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 || !value.CanSet() {
			return
		}
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
		f.fill(value.Index(0), path+"[]", marked, depth+1)
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String || !value.CanSet() {
			return
		}
		entry := reflect.New(value.Type().Elem()).Elem()
		f.fill(entry, path+"{}", marked, depth+1)
		value.Set(reflect.MakeMap(value.Type()))
		value.SetMapIndex(reflect.ValueOf("k").Convert(value.Type().Key()), entry)
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			field := value.Type().Field(index)
			if !field.IsExported() || f.skip[field.Name] {
				continue
			}
			words := strings.ToLower(field.Name + " " + field.Tag.Get("json"))
			f.fill(value.Field(index), path+"."+field.Name, strings.Contains(words, "attempt") || strings.Contains(words, "reuse"), depth+1)
		}
	}
}

// uncovered are the fields whose handed-out ids a reader did not name.
func (f *attemptFill) uncovered(named []string) []string {
	seen := map[string]bool{}
	for _, id := range named {
		seen[id] = true
	}
	var missing []string
	for index, id := range f.ids {
		if !seen[id] {
			missing = append(missing, f.where[index])
		}
	}
	return missing
}
