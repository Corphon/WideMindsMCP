package utils

import (
	"encoding/json"
	"fmt"
)

// ToJSON marshals the provided value into indented JSON bytes.
func ToJSON(v interface{}) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

// MustToJSON marshals the value into JSON and panics on failure.
func MustToJSON(v interface{}) []byte {
	data, err := ToJSON(v)
	if err != nil {
		panic(err)
	}
	return data
}

// FromJSON unmarshals the provided JSON bytes into target.
func FromJSON(data []byte, target interface{}) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return nil
}
