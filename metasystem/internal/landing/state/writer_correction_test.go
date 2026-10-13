package state

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPersistedStateFieldsHaveJSONNames(t *testing.T) {
	t.Parallel()
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || typ == reflect.TypeOf(time.Time{}) {
			return
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if _, ok := field.Tag.Lookup("json"); !ok {
				t.Errorf("%s.%s has no JSON name", typ, field.Name)
			}
			walk(field.Type)
		}
	}
	walk(reflect.TypeOf(Record{}))
	walk(reflect.TypeOf(Event{}))
}

func TestDiffNamesBatchClocksAndDereferencesPayloads(t *testing.T) {
	t.Parallel()
	old := Record{}
	next := Record{Batch: Batch{SealedAt: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC), MainMove: "external"}, Stop: &Stop{Subject: "lane", Kind: "barren"}}
	diff := strings.Join(Diff(old, next), "\n")
	for _, name := range []string{"batch.sealed_at:", "batch.main_move:", "stop:"} {
		if !strings.Contains(diff, name) {
			t.Errorf("diff omitted %s: %s", name, diff)
		}
	}
	if strings.Contains(diff, "batch.:") || strings.Contains(diff, "0x") {
		t.Fatalf("unreadable payload: %s", diff)
	}
}
