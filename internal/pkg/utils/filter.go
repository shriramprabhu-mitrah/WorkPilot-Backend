package utils

import (
	"encoding/json"
	"strings"
)

// FilterFields serializes the input data to JSON and unmarshals it into map structures
// to selectively retain only the keys listed in the comma-separated fieldName parameter.
func FilterFields(data any, fieldName string) (any, error) {
	if fieldName == "" {
		return data, nil
	}

	fields := strings.Split(fieldName, ",")
	fieldSet := make(map[string]bool)
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			fieldSet[f] = true
		}
	}

	if len(fieldSet) == 0 {
		return data, nil
	}

	// Marshal to JSON
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Try unmarshaling as a slice of maps
	var sliceOfMaps []map[string]any
	if err := json.Unmarshal(bytes, &sliceOfMaps); err == nil {
		filteredSlice := make([]map[string]any, 0, len(sliceOfMaps))
		for _, m := range sliceOfMaps {
			filteredMap := make(map[string]any)
			for k, v := range m {
				if fieldSet[k] {
					filteredMap[k] = v
				}
			}
			filteredSlice = append(filteredSlice, filteredMap)
		}
		return filteredSlice, nil
	}

	// Try unmarshaling as a single map
	var singleMap map[string]any
	if err := json.Unmarshal(bytes, &singleMap); err == nil {
		filteredMap := make(map[string]any)
		for k, v := range singleMap {
			if fieldSet[k] {
				filteredMap[k] = v
			}
		}
		return filteredMap, nil
	}

	return data, nil
}
