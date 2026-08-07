package utils_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ms-kanban-server/internal/pkg/utils"
)

type testStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func TestFilterFields(t *testing.T) {
	t.Run("returns original data if fieldName is empty", func(t *testing.T) {
		input := []testStruct{
			{ID: 1, Name: "Alice", Role: "Admin"},
		}
		result, err := utils.FilterFields(input, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(result, input) {
			t.Errorf("expected %+v, got %+v", input, result)
		}
	})

	t.Run("filters a slice of structs to specific fields", func(t *testing.T) {
		input := []testStruct{
			{ID: 1, Name: "Alice", Role: "Admin"},
			{ID: 2, Name: "Bob", Role: "User"},
		}
		result, err := utils.FilterFields(input, "id,name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Marshall/unmarshall results to check fields
		bytes, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal result: %v", err)
		}

		var filteredMaps []map[string]any
		if err := json.Unmarshal(bytes, &filteredMaps); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		if len(filteredMaps) != 2 {
			t.Fatalf("expected 2 items, got %d", len(filteredMaps))
		}

		for _, m := range filteredMaps {
			if len(m) != 2 {
				t.Errorf("expected map to have exactly 2 keys, got %d (keys: %+v)", len(m), m)
			}
			if _, ok := m["id"]; !ok {
				t.Error("expected 'id' field to exist")
			}
			if _, ok := m["name"]; !ok {
				t.Error("expected 'name' field to exist")
			}
			if _, ok := m["role"]; ok {
				t.Error("expected 'role' field to be filtered out")
			}
		}
	})

	t.Run("filters a single struct to specific fields", func(t *testing.T) {
		input := testStruct{ID: 1, Name: "Alice", Role: "Admin"}
		result, err := utils.FilterFields(input, "name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		bytes, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal result: %v", err)
		}

		var filteredMap map[string]any
		if err := json.Unmarshal(bytes, &filteredMap); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		if len(filteredMap) != 1 {
			t.Errorf("expected map to have exactly 1 key, got %d (keys: %+v)", len(filteredMap), filteredMap)
		}
		if _, ok := filteredMap["name"]; !ok {
			t.Error("expected 'name' field to exist")
		}
		if _, ok := filteredMap["id"]; ok {
			t.Error("expected 'id' field to be filtered out")
		}
	})
}
