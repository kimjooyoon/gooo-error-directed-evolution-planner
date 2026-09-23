package planner

import "testing"

func TestKeyValuesRejectsEmptyAndDuplicateKeys(t *testing.T) {
	for _, tokens := range [][]string{
		{"id="},
		{"id=", "id=second"},
		{"id=first", "id=second"},
	} {
		if _, err := keyValues(tokens); err == nil {
			t.Fatalf("expected key/value input %v to be rejected", tokens)
		}
	}
}
